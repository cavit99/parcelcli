package browser

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

// DefaultTimeout is the page-render wait budget used when callers do not pass one.
// The CLI also defaults to this value; carrier fetchers may add a small CDP
// cleanup/redirect cushion around it without extending the semantic wait.
const DefaultTimeout = 35 * time.Second

type Options struct {
	ChromePath string
	URL        string
	WaitFor    []string
	Timeout    time.Duration
	Debug      bool
}

type PageResult struct {
	Href            string
	Title           string
	Body            string
	APIObservations []APIObservation
}

type APIObservation struct {
	Status int64  `json:"status"`
	URL    string `json:"url"`
}

func DefaultChromePath() string {
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

func FetchText(ctx context.Context, opts Options) (*PageResult, error) {
	if opts.Timeout == 0 {
		opts.Timeout = DefaultTimeout
	}
	if opts.ChromePath == "" {
		opts.ChromePath = DefaultChromePath()
	}

	allocOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(opts.ChromePath),
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
	runCtx, cancelRun := context.WithTimeout(browserCtx, opts.Timeout+10*time.Second)
	defer cancelRun()

	var mu sync.Mutex
	var api []APIObservation
	chromedp.ListenTarget(runCtx, func(ev any) {
		if e, ok := ev.(*network.EventResponseReceived); ok {
			u := e.Response.URL
			if strings.Contains(u, "tracking.platform-apis.evri.com") || strings.Contains(u, "protected/keys.json") {
				mu.Lock()
				api = append(api, APIObservation{Status: e.Response.Status, URL: u})
				mu.Unlock()
			}
		}
	})

	var body, title, href string
	if err := chromedp.Run(runCtx,
		network.Enable(),
		chromedp.Navigate(opts.URL),
		chromedp.Sleep(6*time.Second),
		chromedp.Evaluate(`[...document.querySelectorAll('button')].find(b=>/Accept all/i.test(b.textContent))?.click()`, nil),
	); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(opts.Timeout)
	for time.Now().Before(deadline) {
		if err := chromedp.Run(runCtx,
			chromedp.Location(&href),
			chromedp.Title(&title),
			chromedp.Text("body", &body, chromedp.ByQuery),
		); err != nil {
			return nil, err
		}
		lower := strings.ToLower(body)
		for _, marker := range opts.WaitFor {
			if strings.Contains(lower, strings.ToLower(marker)) {
				mu.Lock()
				outAPI := append([]APIObservation(nil), api...)
				mu.Unlock()
				return &PageResult{Href: href, Title: title, Body: body, APIObservations: outAPI}, nil
			}
		}
		time.Sleep(2 * time.Second)
	}
	mu.Lock()
	outAPI := append([]APIObservation(nil), api...)
	mu.Unlock()
	if strings.TrimSpace(body) == "" {
		return &PageResult{Href: href, Title: title, Body: body, APIObservations: outAPI}, fmt.Errorf("page did not render tracking text before timeout")
	}
	return &PageResult{Href: href, Title: title, Body: body, APIObservations: outAPI}, nil
}
