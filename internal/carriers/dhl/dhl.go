package dhl

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
	"github.com/cavit99/parcelcli/internal/textutil"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

const baseURL = "https://www.dhl.com/global-en/home/tracking/tracking-parcel.html"

var eventDateRE = regexp.MustCompile(`(?i)^(Mo|Tu|We|Th|Fr|Sa|Su),\s+\d{2}\.\d{2}\.\d{4},\s+\d{2}:\d{2}(?:,\s+.+)?$`)

type Tracker struct{}

type apiObservation struct {
	Status int    `json:"status"`
	Method string `json:"method,omitempty"`
	URL    string `json:"url"`
}

func (Tracker) Track(ctx context.Context, req model.TrackRequest) (*model.Result, error) {
	number := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(req.TrackingNumber), " ", ""))
	if number == "" {
		return nil, fmt.Errorf("dhl requires a tracking number")
	}
	body, observations, err := fetch(ctx, req.ChromePath, number, req.Timeout)
	if err != nil {
		return nil, err
	}
	return resultFromRendered(number, body, observations), nil
}

func fetch(ctx context.Context, chromePath, number string, timeout time.Duration) (string, []apiObservation, error) {
	if timeout == 0 {
		timeout = browser.DefaultTimeout
	}
	if chromePath == "" {
		chromePath = browser.DefaultChromePath()
	}
	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(chromePath), chromedp.Headless, chromedp.DisableGPU,
		chromedp.NoFirstRun, chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36"),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, allocOpts...)
	defer cancelAlloc()
	browserCtx, cancelBrowser := chromedp.NewContext(allocCtx)
	defer cancelBrowser()
	// DHL can spend extra time bouncing between global DHL and dhl.de tracking pages.
	runCtx, cancelRun := context.WithTimeout(browserCtx, timeout+25*time.Second)
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

	u := baseURL + "?submit=1&tracking-id=" + url.QueryEscape(number)
	var body string
	if err := chromedp.Run(runCtx,
		network.Enable(),
		chromedp.Navigate(u),
		chromedp.Sleep(12*time.Second),
		chromedp.Evaluate(`[...document.querySelectorAll('button')].find(b=>/Strictly Necessary Only|Accept All|Accept/i.test(b.textContent||''))?.click()`, nil),
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
		return body, out, fmt.Errorf("dhl page did not render tracking text before timeout")
	}
	return body, out, nil
}

func isTrackAPIURL(u string) bool {
	l := strings.ToLower(u)
	return strings.Contains(l, "dhl.com/utapi") || strings.Contains(l, "dhl.de/int-verfolgen/data/search")
}

func hasResultText(body, number string) bool {
	l := strings.ToLower(body)
	return strings.Contains(strings.ToUpper(body), number) && (strings.Contains(l, "shipment details") || strings.Contains(l, "detailed tracking history") || strings.Contains(l, "customs clearance") || strings.Contains(l, "delivered") || strings.Contains(l, "shipment number") || strings.Contains(l, "could not be found"))
}

func resultFromRendered(number, body string, observations []apiObservation) *model.Result {
	lines := textutil.CleanLines(body)
	statusText := firstStatus(lines)
	last := latestEvent(lines)
	destination := firstAfter(lines, "Destination country/region:")
	status, delivered, delayed := classify(statusText + "\n" + body)
	raw := map[string]any{"api_observations": observations}
	if destination != "" {
		raw["destination_country_region"] = destination
	}
	return &model.Result{
		Carrier: "dhl", TrackingNumber: number, Status: status, StatusText: statusText,
		Terminal: delivered || status == model.StatusReturned, Delivered: delivered, Delayed: delayed,
		LastEvent: last,
		Source:    model.Source{Method: "browser", URL: baseURL + "?submit=1&tracking-id=" + url.QueryEscape(number), FetchedAt: time.Now().UTC().Format(time.RFC3339)},
		Raw:       raw,
	}
}

func firstAfter(lines []string, marker string) string {
	for i, l := range lines {
		if strings.EqualFold(l, marker) && i+1 < len(lines) {
			return lines[i+1]
		}
	}
	return ""
}

func firstStatus(lines []string) string {
	for _, l := range lines {
		if isStatusLine(l) {
			return l
		}
	}
	return ""
}

func latestEvent(lines []string) *model.Event {
	for i, l := range lines {
		if eventDateRE.MatchString(l) {
			e := &model.Event{Time: l}
			if idx := strings.LastIndex(l, ", "); idx >= 0 && idx+2 < len(l) {
				loc := strings.TrimSpace(l[idx+2:])
				if !strings.Contains(loc, ".") && !strings.Contains(loc, ":") {
					e.Location = loc
				}
			}
			if i+1 < len(lines) {
				e.Text = lines[i+1]
			}
			return e
		}
	}
	return nil
}

func isStatusLine(s string) bool {
	l := strings.ToLower(s)
	return strings.Contains(l, "customs clearance") || strings.Contains(l, "delivered") || strings.Contains(l, "out for delivery") || strings.Contains(l, "shipment has arrived") || strings.Contains(l, "shipment is being transported") || strings.Contains(l, "could not be found")
}

func classify(s string) (model.Status, bool, bool) {
	l := strings.ToLower(s)
	delayed := strings.Contains(l, "delay") || strings.Contains(l, "delayed")
	switch {
	case strings.Contains(l, "delivered") && !strings.Contains(l, "out for delivery"):
		return model.StatusDelivered, true, delayed
	case strings.Contains(l, "out for delivery"):
		return model.StatusOutForDelivery, false, delayed
	case strings.Contains(l, "customs clearance") || strings.Contains(l, "clearance process"):
		if strings.Contains(l, "delay") || strings.Contains(l, "held") || strings.Contains(l, "problem") || strings.Contains(l, "action required") {
			return model.StatusException, false, delayed
		}
		return model.StatusInTransit, false, delayed
	case strings.Contains(l, "being transported") || strings.Contains(l, "has arrived") || strings.Contains(l, "departed") || strings.Contains(l, "on flight") || strings.Contains(l, "we have your package"):
		return model.StatusInTransit, false, delayed
	case strings.Contains(l, "label") || strings.Contains(l, "booked"):
		return model.StatusPreAdvice, false, delayed
	case strings.Contains(l, "ready for pickup") || strings.Contains(l, "available for pickup"):
		return model.StatusReadyForPickup, false, delayed
	case strings.Contains(l, "returned") || strings.Contains(l, "return to sender"):
		return model.StatusReturned, false, delayed
	case strings.Contains(l, "could not be found") || strings.Contains(l, "not found") || strings.Contains(l, "unknown shipment"):
		return model.StatusNotFound, false, delayed
	case delayed:
		return model.StatusDelayed, false, true
	default:
		return model.StatusUnknown, false, false
	}
}
