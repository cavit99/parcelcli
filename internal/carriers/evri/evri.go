package evri

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/cavit99/parcelcli/internal/browser"
	"github.com/cavit99/parcelcli/internal/model"
	"github.com/cavit99/parcelcli/internal/textutil"
)

var eventRE = regexp.MustCompile(`(?im)^\d{1,2}:\d{2}\s*-\s*[a-z][^\n]*`)

type Tracker struct{}

func (Tracker) Track(ctx context.Context, req model.TrackRequest) (*model.Result, error) {
	postcode := strings.ToUpper(strings.ReplaceAll(req.Postcode, " ", ""))
	u := fmt.Sprintf("https://www.evri.com/track/parcel/%s/details", url.PathEscape(req.TrackingNumber))
	if postcode != "" {
		u = fmt.Sprintf("%s?postcode=%s", u, url.QueryEscape(postcode))
	}
	page, err := browser.FetchText(ctx, browser.Options{ChromePath: req.ChromePath, URL: u, Timeout: req.Timeout, Debug: req.Debug, WaitFor: []string{"Your parcel from", "Update on your parcel", "Barcode number", "Out for delivery", "delayed", "delivered"}})
	if err != nil {
		return nil, err
	}
	statusText, sender, lastTime, lastEvent, eta := extract(page.Body, req.TrackingNumber)
	status, delivered, delayed := classify(statusText + "\n" + lastEvent + "\n" + page.Body)
	events := extractEvents(page.Body)
	var le *model.Event
	if lastTime != "" || lastEvent != "" {
		le = &model.Event{Time: lastTime, Text: lastEvent}
	}
	return &model.Result{
		Carrier: "evri", TrackingNumber: req.TrackingNumber, Postcode: postcode,
		Status: status, StatusText: statusText, Terminal: delivered || status == model.StatusReturned, Delivered: delivered, Delayed: delayed,
		SenderLine: sender, EstimatedDelivery: eta, LastEvent: le, Events: events,
		Source: model.Source{Method: "browser", URL: u, FetchedAt: time.Now().UTC().Format(time.RFC3339)},
		Raw:    map[string]any{"api_observations": page.APIObservations},
	}, nil
}

func extract(body, trackingNumber string) (status, sender, lastTime, lastEvent, eta string) {
	lines := textutil.CleanLines(body)
	number := strings.ToUpper(strings.ReplaceAll(trackingNumber, " ", ""))
	for i, l := range lines {
		lower := strings.ToLower(l)
		if strings.Contains(lower, "update on your parcel") && i+1 < len(lines) {
			status = lines[i+1]
			if eventRE.MatchString(status) && i+2 < len(lines) {
				status = lines[i+2]
			}
		}
		if strings.HasPrefix(lower, "your parcel from ") {
			sender = l
		}
		if strings.ToUpper(strings.ReplaceAll(l, " ", "")) == number && status == "" && i+1 < len(lines) {
			status = lines[i+1]
		}
		if strings.Contains(lower, "will deliver your parcel") && eta == "" {
			eta = l
		}
		if lower == "delivery time" && i+1 < len(lines) && eta == "" {
			eta = lines[i+1]
		}
		if eventRE.MatchString(l) && lastTime == "" {
			lastTime = l
			if i+1 < len(lines) {
				lastEvent = lines[i+1]
			}
		}
	}
	return
}
func extractEvents(body string) []model.Event {
	lines := textutil.CleanLines(body)
	var out []model.Event
	for i, l := range lines {
		if eventRE.MatchString(l) {
			text := ""
			if i+1 < len(lines) {
				text = lines[i+1]
			}
			out = append(out, model.Event{Time: l, Text: text})
		}
	}
	return out
}
func classify(s string) (model.Status, bool, bool) {
	l := strings.ToLower(s)
	delayed := strings.Contains(l, "delayed")
	switch {
	case strings.Contains(l, "delivered") && !strings.Contains(l, "out for delivery"):
		return model.StatusDelivered, true, delayed
	case delayed:
		return model.StatusDelayed, false, true
	case strings.Contains(l, "out for delivery") || strings.Contains(l, "driver en route"):
		return model.StatusOutForDelivery, false, delayed
	case strings.Contains(l, "we've got it") || strings.Contains(l, "on its way") || strings.Contains(l, "in transit"):
		return model.StatusInTransit, false, delayed
	case strings.Contains(l, "not found"):
		return model.StatusNotFound, false, delayed
	default:
		return model.StatusUnknown, false, delayed
	}
}
