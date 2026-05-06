package royalmail

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/cavit99/parcelcli/internal/browser"
	"github.com/cavit99/parcelcli/internal/model"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const (
	trackURL = "https://www.royalmail.com/track-your-item"
)

type Tracker struct{}

type apiObservation struct {
	Status int    `json:"status"`
	Method string `json:"method,omitempty"`
	URL    string `json:"url"`
	Body   string `json:"body,omitempty"`
}

type apiSummary struct {
	MailPieces any `json:"mailPieces"`
	Errors     any `json:"errors"`
}

type mailPiece struct {
	CarrierFullName  string   `json:"carrierFullName"`
	CarrierShortName string   `json:"carrierShortName"`
	Error            *rmError `json:"error"`
	Summary          *summary `json:"summary"`
	Events           []event  `json:"events"`
}

type rmError struct {
	ErrorCode        string `json:"errorCode"`
	ErrorCause       string `json:"errorCause"`
	ErrorDescription string `json:"errorDescription"`
	ErrorResolution  string `json:"errorResolution"`
}

type summary struct {
	OneDBarcode             string `json:"oneDBarcode"`
	ProductName             string `json:"productName"`
	StatusCategory          string `json:"statusCategory"`
	StatusDescription       string `json:"statusDescription"`
	StatusHelpText          string `json:"statusHelpText"`
	SummaryLine             string `json:"summaryLine"`
	LastEventCode           string `json:"lastEventCode"`
	LastEventName           string `json:"lastEventName"`
	LastEventDateTime       string `json:"lastEventDateTime"`
	LastEventLocationName   string `json:"lastEventLocationName"`
	EstimatedDeliveryDate   string `json:"estimatedDeliveryDate"`
	ExpectedDeliveryDate    string `json:"expectedDeliveryDate"`
	EstimatedDeliveryWindow string `json:"estimatedDeliveryWindow"`
}

type event struct {
	EventCode     string `json:"eventCode"`
	EventName     string `json:"eventName"`
	EventDateTime string `json:"eventDateTime"`
	LocationName  string `json:"locationName"`
}

func (Tracker) Track(ctx context.Context, req model.TrackRequest) (*model.Result, error) {
	number := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(req.TrackingNumber), " ", ""))
	if number == "" {
		return nil, fmt.Errorf("royalmail requires a tracking number")
	}
	page, observations, err := fetch(ctx, req.ChromePath, number, req.Timeout)
	if err != nil {
		return nil, err
	}

	for _, obs := range observations {
		if obs.Body == "" || !strings.Contains(strings.ToLower(obs.URL), "/summary/") {
			continue
		}
		if res, ok := resultFromJSON(number, obs.Body, observations); ok {
			return res, nil
		}
	}
	return resultFromRendered(number, page, observations), nil
}

func fetch(ctx context.Context, chromePath, number string, timeout time.Duration) (string, []apiObservation, error) {
	if timeout == 0 {
		timeout = 45 * time.Second
	}
	if chromePath == "" {
		chromePath = browser.DefaultChromePath()
	}
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath),
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	runCtx, cancelRun := context.WithTimeout(browserCtx, timeout+15*time.Second)
	defer cancelRun()

	var mu sync.Mutex
	methods := map[network.RequestID]string{}
	interesting := map[network.RequestID]int{}
	var observations []apiObservation

	chromedp.ListenTarget(runCtx, func(ev any) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			methods[e.RequestID] = e.Request.Method
		case *network.EventResponseReceived:
			u := e.Response.URL
			if isAPIURL(u) {
				mu.Lock()
				idx := len(observations)
				observations = append(observations, apiObservation{Status: int(e.Response.Status), Method: methods[e.RequestID], URL: u})
				interesting[e.RequestID] = idx
				mu.Unlock()
			}
		case *network.EventLoadingFinished:
			mu.Lock()
			idx, ok := interesting[e.RequestID]
			mu.Unlock()
			if !ok {
				return
			}
			go func(id network.RequestID, idx int) {
				body, err := network.GetResponseBody(id).Do(runCtx)
				if err != nil || len(body) == 0 {
					return
				}
				mu.Lock()
				if idx < len(observations) {
					observations[idx].Body = string(body)
				}
				mu.Unlock()
			}(e.RequestID, idx)
		}
	})

	var body string
	if err := chromedp.Run(runCtx,
		network.Enable(),
		chromedp.Navigate(trackURL),
		chromedp.Sleep(5*time.Second),
		chromedp.Evaluate(`[...document.querySelectorAll('button')].find(b=>/Accept all|Decline all/i.test(b.textContent||''))?.click()`, nil),
		chromedp.WaitVisible(`#barcode-input`, chromedp.ByID),
		chromedp.Click(`#barcode-input`, chromedp.ByID),
		chromedp.SendKeys(`#barcode-input`, number, chromedp.ByID),
		chromedp.Sleep(500*time.Millisecond),
		chromedp.Click(`#submit`, chromedp.ByID),
	); err != nil {
		return "", nil, err
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := chromedp.Run(runCtx, chromedp.Text("body", &body, chromedp.ByQuery)); err != nil {
			return "", nil, err
		}
		lower := strings.ToLower(body)
		if hasResultText(lower, number) || hasSummaryObservation(observations) {
			chromedp.Run(runCtx, chromedp.Sleep(1*time.Second))
			mu.Lock()
			out := append([]apiObservation(nil), observations...)
			mu.Unlock()
			return body, out, nil
		}
		time.Sleep(1500 * time.Millisecond)
	}
	mu.Lock()
	out := append([]apiObservation(nil), observations...)
	mu.Unlock()
	if strings.TrimSpace(body) == "" {
		return body, out, fmt.Errorf("royalmail page did not render tracking text before timeout")
	}
	return body, out, nil
}

func isAPIURL(u string) bool {
	l := strings.ToLower(u)
	return strings.Contains(l, "api-web.royalmail.com/mailpieces") || strings.Contains(l, "api.royalmail.net/mailpieces")
}

func hasSummaryObservation(obs []apiObservation) bool {
	for _, o := range obs {
		if strings.Contains(strings.ToLower(o.URL), "/summary/") && (o.Body != "" || o.Status >= 400) {
			return true
		}
	}
	return false
}

func hasResultText(lower, number string) bool {
	return strings.Contains(lower, strings.ToLower(number)) || strings.Contains(lower, "unable to confirm") || strings.Contains(lower, "don't recognise") || strings.Contains(lower, "not recognise") || strings.Contains(lower, "we've got it") || strings.Contains(lower, "we have your item") || strings.Contains(lower, "your item was delivered") || strings.Contains(lower, "we've delivered") || strings.Contains(lower, "out for delivery") || strings.Contains(lower, "ready for delivery") || strings.Contains(lower, "in transit")
}

func resultFromJSON(number, body string, observations []apiObservation) (*model.Result, bool) {
	var root any
	if err := json.Unmarshal([]byte(body), &root); err != nil {
		return nil, false
	}
	piece, ok := firstPiece(root)
	if !ok {
		return nil, false
	}
	return normalizePiece(number, piece, observations), true
}

func firstPiece(v any) (mailPiece, bool) {
	b, _ := json.Marshal(v)
	var direct mailPiece
	if json.Unmarshal(b, &direct) == nil && (direct.Summary != nil || direct.Error != nil || len(direct.Events) > 0) {
		return direct, true
	}
	var wrapped struct {
		MailPieces json.RawMessage `json:"mailPieces"`
	}
	if json.Unmarshal(b, &wrapped) != nil || len(wrapped.MailPieces) == 0 {
		return mailPiece{}, false
	}
	var one mailPiece
	if json.Unmarshal(wrapped.MailPieces, &one) == nil && (one.Summary != nil || one.Error != nil || len(one.Events) > 0) {
		return one, true
	}
	var many []mailPiece
	if json.Unmarshal(wrapped.MailPieces, &many) == nil && len(many) > 0 {
		return many[0], true
	}
	return mailPiece{}, false
}

func normalizePiece(number string, p mailPiece, observations []apiObservation) *model.Result {
	var statusText, eta string
	var events []model.Event
	var last *model.Event
	if p.Summary != nil {
		statusText = firstNonEmpty(p.Summary.StatusDescription, p.Summary.SummaryLine, p.Summary.StatusHelpText, p.Summary.StatusCategory)
		eta = firstNonEmpty(p.Summary.EstimatedDeliveryWindow, p.Summary.ExpectedDeliveryDate, p.Summary.EstimatedDeliveryDate)
		if p.Summary.LastEventName != "" || p.Summary.LastEventDateTime != "" {
			last = &model.Event{Time: p.Summary.LastEventDateTime, Text: p.Summary.LastEventName, Location: p.Summary.LastEventLocationName, RawCode: p.Summary.LastEventCode}
		}
	}
	for _, e := range p.Events {
		events = append(events, model.Event{Time: e.EventDateTime, Text: e.EventName, Location: e.LocationName, RawCode: e.EventCode})
	}
	if len(events) > 0 {
		last = &events[0]
	}
	if p.Error != nil {
		statusText = firstNonEmpty(p.Error.ErrorCause, p.Error.ErrorDescription, p.Error.ErrorResolution, p.Error.ErrorCode)
	}
	s, delivered, delayed := classify(pieceText(p), eventCode(p))
	return &model.Result{
		Carrier: "royalmail", TrackingNumber: number, Status: s, StatusText: statusText,
		Terminal: delivered || s == model.StatusReturned, Delivered: delivered, Delayed: delayed,
		EstimatedDelivery: eta, LastEvent: last, Events: events,
		Source: model.Source{Method: "browser", URL: trackURL + "#/results?trackNumber=" + url.QueryEscape(number), FetchedAt: time.Now().UTC().Format(time.RFC3339)},
		Raw:    map[string]any{"api_observations": observations},
	}
}

func resultFromRendered(number, body string, observations []apiObservation) *model.Result {
	statusText := extractRenderedStatus(body, number)
	status, delivered, delayed := classify(statusText+"\n"+body, "")
	return &model.Result{
		Carrier: "royalmail", TrackingNumber: number, Status: status, StatusText: statusText,
		Terminal: delivered || status == model.StatusReturned, Delivered: delivered, Delayed: delayed,
		Source: model.Source{Method: "browser", URL: trackURL + "#/results?trackNumber=" + url.QueryEscape(number), FetchedAt: time.Now().UTC().Format(time.RFC3339)},
		Raw:    map[string]any{"api_observations": observations},
	}
}

func extractRenderedStatus(body, number string) string {
	lines := cleanLines(body)
	for _, l := range lines {
		ll := strings.ToLower(l)
		if strings.Contains(l, number) && (strings.Contains(ll, "sorry") || strings.Contains(ll, "delivered") || strings.Contains(ll, "status")) {
			return l
		}
		if strings.Contains(ll, "sorry - we don't recognise") || strings.Contains(ll, "unable to confirm") || strings.Contains(ll, "your item was delivered") || strings.Contains(ll, "we've delivered") || strings.Contains(ll, "we've got it") || strings.Contains(ll, "we have your item") || strings.Contains(ll, "in transit") || strings.Contains(ll, "out for delivery") || strings.Contains(ll, "ready for delivery") {
			return l
		}
	}
	return ""
}

func cleanLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(l); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func pieceText(p mailPiece) string {
	var parts []string
	if p.Summary != nil {
		parts = append(parts, p.Summary.StatusCategory, p.Summary.StatusDescription, p.Summary.StatusHelpText, p.Summary.SummaryLine, p.Summary.LastEventName)
	}
	if p.Error != nil {
		parts = append(parts, p.Error.ErrorCode, p.Error.ErrorCause, p.Error.ErrorDescription, p.Error.ErrorResolution)
	}
	for _, e := range p.Events {
		parts = append(parts, e.EventCode, e.EventName)
	}
	return strings.Join(parts, "\n")
}

func eventCode(p mailPiece) string {
	if p.Summary != nil && p.Summary.LastEventCode != "" {
		return p.Summary.LastEventCode
	}
	if len(p.Events) > 0 {
		return p.Events[0].EventCode
	}
	return ""
}

func classify(text, code string) (model.Status, bool, bool) {
	c := strings.ToUpper(strings.TrimSpace(code))
	l := strings.ToLower(text)
	delayed := strings.Contains(l, "delay")
	switch {
	case containsCode(c, "EVKSP", "EVKOP", "EVKSF") || strings.Contains(l, "delivered"):
		return model.StatusDelivered, true, delayed
	case c == "EVPLC" || strings.Contains(l, "collected"):
		return model.StatusDelivered, true, delayed
	case c == "EVPLA" || strings.Contains(l, "available for collection"):
		return model.StatusReadyForPickup, false, delayed
	case c == "EVKNR" || strings.Contains(l, "delivery attempted") || strings.Contains(l, "unable to deliver"):
		return model.StatusDeliveryAttempted, false, delayed
	case c == "EVGPD" || strings.Contains(l, "out for delivery") || strings.Contains(l, "ready for delivery") || strings.Contains(l, "deliver it today"):
		return model.StatusOutForDelivery, false, delayed
	case containsCode(c, "EVNSR", "EVODO", "EVORI", "EVOAC", "EVAIE", "EVAIP", "EVPPA", "EVDAV", "EVIMC", "EVDAC", "EVNRT", "EVOCO", "RSRXS", "RORXS", "EVNDA", "EVBAV", "EVKLS", "EVIAV") || strings.Contains(l, "in transit"):
		return model.StatusInTransit, false, delayed
	case strings.Contains(l, "returned to sender"):
		return model.StatusReturned, false, delayed
	case strings.Contains(l, "restricted") || strings.Contains(l, "prohibited") || strings.Contains(l, "duplicate") || strings.Contains(l, "access denied") || strings.Contains(l, "e0015"):
		return model.StatusException, false, delayed
	case strings.Contains(l, "e1142") || strings.Contains(l, "don't recognise") || strings.Contains(l, "not recognise") || strings.Contains(l, "cannot be located"):
		return model.StatusNotFound, false, delayed
	case delayed:
		return model.StatusDelayed, false, true
	default:
		return model.StatusUnknown, false, false
	}
}

func containsCode(code string, codes ...string) bool {
	for _, c := range codes {
		if code == c {
			return true
		}
	}
	return false
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}
