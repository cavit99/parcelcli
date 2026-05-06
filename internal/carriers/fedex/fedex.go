package fedex

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cavit99/parcelcli/internal/model"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const baseURL = "https://www.fedex.com/fedextrack/"

var dateTimeRE = regexp.MustCompile(`^\d{1,2}/\d{1,2}/\d{2}\s+\d{1,2}:\d{2}\s+[AP]M$`)

type Tracker struct{}

type apiObservation struct {
	Status int    `json:"status"`
	Method string `json:"method,omitempty"`
	URL    string `json:"url"`
}

func (Tracker) Track(ctx context.Context, req model.TrackRequest) (*model.Result, error) {
	number := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(req.TrackingNumber), " ", ""))
	if number == "" {
		return nil, fmt.Errorf("fedex requires a tracking number")
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
		chromePath = defaultChromePath()
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
	runCtx, cancelRun := context.WithTimeout(browserCtx, timeout+45*time.Second)
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

	u := baseURL + "?trknbr=" + url.QueryEscape(number)
	var body string
	if err := chromedp.Run(runCtx,
		network.Enable(),
		chromedp.Navigate(u),
		chromedp.Sleep(20*time.Second),
	); err != nil {
		return "", nil, err
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := chromedp.Run(runCtx, chromedp.Text("body", &body, chromedp.ByQuery)); err != nil {
			return "", nil, err
		}
		if hasResultText(body, number) {
			chromedp.Run(runCtx, chromedp.Sleep(2*time.Second))
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
		return body, out, fmt.Errorf("fedex page did not render tracking text before timeout")
	}
	return body, out, nil
}

func defaultChromePath() string {
	if runtime.GOOS == "darwin" {
		return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	}
	for _, p := range []string{"google-chrome", "chromium", "chromium-browser"} {
		if found, err := exec.LookPath(p); err == nil {
			return found
		}
	}
	return "google-chrome"
}

func isTrackAPIURL(u string) bool {
	l := strings.ToLower(u)
	return strings.Contains(l, "api.fedex.com/track/") || strings.Contains(l, "api.fedex.com/auth/oauth")
}

func hasResultText(body, number string) bool {
	if !strings.Contains(strings.ToUpper(body), number) {
		return false
	}
	lines := cleanLines(body)
	if firstAfter(lines, "DELIVERY DETAILS") != "" || latestEvent(lines) != nil {
		return true
	}
	l := strings.ToLower(body)
	return strings.Contains(l, "no record of this tracking") || strings.Contains(l, "unable to retrieve") || strings.Contains(l, "can’t find that tracking number") || strings.Contains(l, "can't find that tracking number")
}

func resultFromRendered(number, body string, observations []apiObservation) *model.Result {
	lines := cleanLines(body)
	statusText := firstAfter(lines, "DELIVERY DETAILS")
	last := latestEvent(lines)
	if statusText == "" && last != nil {
		statusText = last.Text
	}
	from := firstAfter(lines, "FROM")
	to := firstAfter(lines, "TO")
	status, delivered, delayed := classify(statusText + "\n" + body)
	raw := map[string]any{"api_observations": observations}
	if from != "" {
		raw["from"] = from
	}
	if to != "" {
		raw["to"] = to
	}
	return &model.Result{
		Carrier: "fedex", TrackingNumber: number, Status: status, StatusText: statusText,
		Terminal: delivered || status == model.StatusReturned, Delivered: delivered, Delayed: delayed,
		LastEvent: last,
		Source:    model.Source{Method: "browser", URL: baseURL + "?trknbr=" + url.QueryEscape(number), FetchedAt: time.Now().UTC().Format(time.RFC3339)},
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
			return lines[i+1]
		}
	}
	return ""
}

func latestEvent(lines []string) *model.Event {
	for i := len(lines) - 1; i >= 0; i-- {
		if !dateTimeRE.MatchString(lines[i]) {
			continue
		}
		e := &model.Event{Time: lines[i]}
		if i-1 >= 0 {
			e.Location = lines[i-1]
		}
		if i-2 >= 0 {
			e.Text = lines[i-2]
		}
		return e
	}
	return nil
}

func classify(s string) (model.Status, bool, bool) {
	l := strings.ToLower(s)
	delayed := strings.Contains(l, "new delivery date") || strings.Contains(l, "delivery updated") || strings.Contains(l, "delay") || strings.Contains(l, "delayed")
	switch {
	case strings.Contains(l, "delivered") && !strings.Contains(l, "out for delivery"):
		return model.StatusDelivered, true, delayed
	case strings.Contains(l, "out for delivery") && !delayed:
		return model.StatusOutForDelivery, false, delayed
	case delayed:
		return model.StatusDelayed, false, true
	case strings.Contains(l, "we have your package") || strings.Contains(l, "on the way") || strings.Contains(l, "in transit") || strings.Contains(l, "arrived") || strings.Contains(l, "departed"):
		return model.StatusInTransit, false, delayed
	case strings.Contains(l, "label created") || strings.Contains(l, "shipment information sent"):
		return model.StatusPreAdvice, false, delayed
	case strings.Contains(l, "delivery exception") || strings.Contains(l, "exception") || strings.Contains(l, "clearance delay") || strings.Contains(l, "action required"):
		return model.StatusException, false, delayed
	case strings.Contains(l, "delivery attempted") || strings.Contains(l, "attempted delivery"):
		return model.StatusDeliveryAttempted, false, delayed
	case strings.Contains(l, "available for pickup") || strings.Contains(l, "hold at location"):
		return model.StatusReadyForPickup, false, delayed
	case strings.Contains(l, "return to shipper") || strings.Contains(l, "returned"):
		return model.StatusReturned, false, delayed
	case strings.Contains(l, "no record of this tracking") || strings.Contains(l, "not found") || strings.Contains(l, "unable to retrieve") || strings.Contains(l, "can’t find that tracking number") || strings.Contains(l, "can't find that tracking number"):
		return model.StatusNotFound, false, delayed
	default:
		return model.StatusUnknown, false, false
	}
}
