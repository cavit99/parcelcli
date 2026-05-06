package ups

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cavit99/parcelcli/internal/browser"
	"github.com/cavit99/parcelcli/internal/model"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const baseURL = "https://www.ups.com/track"

var deliveredEventRE = regexp.MustCompile(`(?i)^([A-Za-z]+,\s+[A-Za-z]+\s+\d{1,2})\s+(.+)$`)

type Tracker struct{}

type apiObservation struct {
	Status int    `json:"status"`
	Method string `json:"method,omitempty"`
	URL    string `json:"url"`
}

func (Tracker) Track(ctx context.Context, req model.TrackRequest) (*model.Result, error) {
	number := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(req.TrackingNumber), " ", ""))
	if number == "" {
		return nil, fmt.Errorf("ups requires a tracking number")
	}
	body, observations, err := fetch(ctx, req.ChromePath, number, req.Timeout)
	if err != nil {
		return nil, err
	}
	return resultFromRendered(number, body, observations), nil
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
	seen := map[network.RequestID]bool{}
	var observations []apiObservation

	chromedp.ListenTarget(runCtx, func(ev any) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			methods[e.RequestID] = e.Request.Method
		case *network.EventResponseReceived:
			u := e.Response.URL
			if isTrackAPIURL(u) {
				mu.Lock()
				if !seen[e.RequestID] {
					observations = append(observations, apiObservation{Status: int(e.Response.Status), Method: methods[e.RequestID], URL: u})
					seen[e.RequestID] = true
				}
				mu.Unlock()
			}
		}
	})

	u := baseURL + "?loc=en_US&tracknum=" + url.QueryEscape(number) + "&requester=ST/trackdetails"
	var body string
	if err := chromedp.Run(runCtx,
		network.Enable(),
		chromedp.Navigate(u),
		chromedp.Sleep(8*time.Second),
		chromedp.Evaluate(`[...document.querySelectorAll('button')].find(b=>/Essential Cookies Only|Accept|Allow All/i.test(b.textContent||''))?.click()`, nil),
	); err != nil {
		return "", nil, err
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := chromedp.Run(runCtx, chromedp.Text("body", &body, chromedp.ByQuery)); err != nil {
			return "", nil, err
		}
		if hasResultText(body, number) {
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
		return body, out, fmt.Errorf("ups page did not render tracking text before timeout")
	}
	return body, out, nil
}

func isTrackAPIURL(u string) bool {
	return strings.Contains(strings.ToLower(u), "webapis.ups.com/track/api/track/getstatus")
}

func hasResultText(body, number string) bool {
	l := strings.ToLower(body)
	return strings.Contains(strings.ToUpper(body), number) && (strings.Contains(l, "tracking details") || strings.Contains(l, "delivered") || strings.Contains(l, "on the way") || strings.Contains(l, "out for delivery") || strings.Contains(l, "could not locate") || strings.Contains(l, "invalid"))
}

func resultFromRendered(number, body string, observations []apiObservation) *model.Result {
	lines := cleanLines(body)
	statusText := firstAfter(lines, "Latest Update")
	if statusText == "" {
		statusText = firstEventLine(lines)
	}
	statusLine := statusAfterTrackingDetails(lines)
	if statusText == "" && statusLine != "" {
		statusText = statusLine
	}
	deliveredTo := firstAfter(lines, "Delivered To")
	receivedBy := firstAfter(lines, "Received By")
	status, delivered, delayed := classify(statusLine + "\n" + statusText + "\n" + body)
	last := lastEvent(statusText, deliveredTo)
	raw := map[string]any{"api_observations": observations}
	if deliveredTo != "" {
		raw["delivered_to"] = deliveredTo
	}
	if receivedBy != "" {
		raw["received_by"] = receivedBy
	}
	return &model.Result{
		Carrier: "ups", TrackingNumber: number, Status: status, StatusText: statusText,
		Terminal: delivered || status == model.StatusReturned, Delivered: delivered, Delayed: delayed,
		LastEvent: last,
		Source:    model.Source{Method: "browser", URL: baseURL + "?loc=en_US&tracknum=" + url.QueryEscape(number) + "&requester=ST/trackdetails", FetchedAt: time.Now().UTC().Format(time.RFC3339)},
		Raw:       raw,
	}
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

func firstAfter(lines []string, marker string) string {
	for i, l := range lines {
		if strings.EqualFold(l, marker) && i+1 < len(lines) {
			for _, next := range lines[i+1:] {
				if !skipLine(next) {
					return next
				}
			}
		}
	}
	return ""
}

func firstEventLine(lines []string) string {
	for _, l := range lines {
		if deliveredEventRE.MatchString(l) {
			return l
		}
	}
	return ""
}

func statusAfterTrackingDetails(lines []string) string {
	for i, l := range lines {
		if strings.EqualFold(l, "Tracking Details") && i+1 < len(lines) {
			for _, next := range lines[i+1:] {
				if !skipLine(next) && !strings.Contains(strings.ToLower(next), "content_copy") {
					return strings.TrimSpace(strings.TrimSuffix(next, " check_circle"))
				}
			}
		}
	}
	return ""
}

func skipLine(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return true
	}
	lower := strings.ToLower(t)
	return lower == "check_circle" || lower == "expand_more" || lower == "keyboard_arrow_down" || lower == "content_copy" || lower == "copy tracking number" || lower == "check" || lower == "close"
}

func lastEvent(statusText, location string) *model.Event {
	if statusText == "" {
		return nil
	}
	if m := deliveredEventRE.FindStringSubmatch(statusText); len(m) == 3 {
		return &model.Event{Time: strings.TrimSpace(m[1]), Text: strings.TrimSpace(m[2]), Location: location}
	}
	return &model.Event{Text: statusText, Location: location}
}

func classify(s string) (model.Status, bool, bool) {
	l := strings.ToLower(s)
	delayed := strings.Contains(l, "delay") || strings.Contains(l, "delayed")
	switch {
	case strings.Contains(l, "delivered") || strings.Contains(l, "left at") || strings.Contains(l, "proof of delivery"):
		return model.StatusDelivered, true, delayed
	case strings.Contains(l, "out for delivery"):
		return model.StatusOutForDelivery, false, delayed
	case strings.Contains(l, "on the way") || strings.Contains(l, "in transit") || strings.Contains(l, "departed") || strings.Contains(l, "arrived") || strings.Contains(l, "processing at ups facility"):
		return model.StatusInTransit, false, delayed
	case strings.Contains(l, "label created") || strings.Contains(l, "shipment ready for ups") || strings.Contains(l, "has not received the package"):
		return model.StatusPreAdvice, false, delayed
	case strings.Contains(l, "delivery attempted") || strings.Contains(l, "we missed you") || strings.Contains(l, "attempted"):
		return model.StatusDeliveryAttempted, false, delayed
	case strings.Contains(l, "ready for pickup") || strings.Contains(l, "available for pickup") || strings.Contains(l, "access point"):
		return model.StatusReadyForPickup, false, delayed
	case strings.Contains(l, "exception") || strings.Contains(l, "clearance") || strings.Contains(l, "address information required") || strings.Contains(l, "action required"):
		return model.StatusException, false, delayed
	case strings.Contains(l, "return to sender") || strings.Contains(l, "returned"):
		return model.StatusReturned, false, delayed
	case strings.Contains(l, "could not locate") || strings.Contains(l, "tracking information not found") || strings.Contains(l, "invalid tracking"):
		return model.StatusNotFound, false, delayed
	case delayed:
		return model.StatusDelayed, false, true
	default:
		return model.StatusUnknown, false, false
	}
}
