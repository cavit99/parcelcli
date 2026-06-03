package inpost

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/cavit99/parcelcli/internal/browser"
	"github.com/cavit99/parcelcli/internal/model"
)

const baseURL = "https://tracking.inpost.co.uk/api/v2.0/"

type Tracker struct{}

type trackingResponse struct {
	Events []trackingEvent
	Error  string
}

type trackingEvent struct {
	Time        string `json:"ts"`
	Code        string `json:"code"`
	Description string `json:"description"`
	Remark      string `json:"remark"`
}

func (Tracker) Track(ctx context.Context, req model.TrackRequest) (*model.Result, error) {
	number := normalizeNumber(req.TrackingNumber)
	if number == "" {
		return nil, fmt.Errorf("inpost requires a tracking number")
	}
	timeout := req.Timeout
	if timeout == 0 {
		timeout = browser.DefaultTimeout
	}

	u := baseURL + url.PathEscape(number)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "parcelcli/0 (+https://github.com/cavit99/parcelcli)")

	resp, err := (&http.Client{Timeout: timeout}).Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("inpost tracking returned HTTP %d", resp.StatusCode)
	}
	return resultFromJSON(number, u, resp.StatusCode, body)
}

func resultFromJSON(number, sourceURL string, httpStatus int, body []byte) (*model.Result, error) {
	tr, err := parseTrackingResponse(number, body)
	if err != nil {
		return nil, fmt.Errorf("inpost tracking returned invalid JSON: %w", err)
	}
	if isNotFound(httpStatus, tr) {
		message := firstNonEmpty(tr.Error, fmt.Sprintf("No events found for consignment %s", number))
		return &model.Result{
			Carrier:        "inpost",
			TrackingNumber: number,
			Status:         model.StatusNotFound,
			StatusText:     message,
			Terminal:       false,
			Delivered:      false,
			Delayed:        false,
			Source:         source(sourceURL),
			Raw:            map[string]any{"http_status": httpStatus},
		}, nil
	}
	if httpStatus < 200 || httpStatus > 299 {
		return nil, fmt.Errorf("inpost tracking returned HTTP %d", httpStatus)
	}

	events := eventsFromDetails(tr.Events)
	last := latestEvent(events)
	statusText := ""
	if last != nil && statusText == "" {
		statusText = last.Text
	}
	status, delivered, delayed := classify(eventCodes(events) + "\n" + statusText)
	raw := map[string]any{"http_status": httpStatus}
	return &model.Result{
		Carrier:        "inpost",
		TrackingNumber: number,
		Status:         status,
		StatusText:     statusText,
		Terminal:       delivered || status == model.StatusReturned || status == model.StatusException,
		Delivered:      delivered,
		Delayed:        delayed,
		LastEvent:      last,
		Events:         events,
		Source:         source(sourceURL),
		Raw:            raw,
	}, nil
}

func parseTrackingResponse(number string, body []byte) (trackingResponse, error) {
	var wrapped map[string]json.RawMessage
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return trackingResponse{}, err
	}
	raw, ok := wrapped[number]
	if !ok {
		for k, v := range wrapped {
			if strings.EqualFold(k, number) {
				raw = v
				ok = true
				break
			}
		}
	}
	if !ok {
		return trackingResponse{Error: "No tracking data returned for consignment " + number}, nil
	}
	var events []trackingEvent
	if err := json.Unmarshal(raw, &events); err == nil {
		return trackingResponse{Events: events}, nil
	}
	var errObj struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &errObj); err != nil {
		return trackingResponse{}, err
	}
	return trackingResponse{Error: errObj.Error}, nil
}

func isNotFound(httpStatus int, tr trackingResponse) bool {
	if httpStatus == http.StatusNotFound {
		return true
	}
	if len(tr.Events) == 0 {
		return true
	}
	l := strings.ToLower(tr.Error)
	return strings.Contains(l, "no events found") || strings.Contains(l, "not found")
}

func eventsFromDetails(details []trackingEvent) []model.Event {
	var out []model.Event
	for _, d := range details {
		text := firstNonEmpty(d.Description, d.Remark, humanStatus(d.Code))
		location := ""
		rawCode := d.Code
		if text == "" && d.Time == "" && location == "" && rawCode == "" {
			continue
		}
		out = append(out, model.Event{Time: d.Time, Text: text, Location: location, RawCode: rawCode})
	}
	return out
}

func latestEvent(events []model.Event) *model.Event {
	if len(events) == 0 {
		return nil
	}
	latestIdx := 0
	latestTime := parseTime(events[0].Time)
	for i := 1; i < len(events); i++ {
		t := parseTime(events[i].Time)
		if latestTime.IsZero() || (!t.IsZero() && t.After(latestTime)) {
			latestIdx = i
			latestTime = t
		}
	}
	return &events[latestIdx]
}

func parseTime(s string) time.Time {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05.000-07:00", "2006-01-02T15:04:05-07:00", "02/01/2006 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

func classify(s string) (model.Status, bool, bool) {
	l := strings.ToLower(s)
	delayed := strings.Contains(l, "delay")
	switch {
	case strings.Contains(l, "pickup_time_expired") || strings.Contains(l, "returned_to_sender") || strings.Contains(l, "return to sender") || strings.Contains(l, "rts."):
		return model.StatusReturned, false, delayed
	case strings.Contains(l, "undelivered") || strings.Contains(l, "avizo") || strings.Contains(l, "delivery attempted"):
		return model.StatusDeliveryAttempted, false, delayed
	case strings.Contains(l, "canceled") || strings.Contains(l, "claimed") || strings.Contains(l, "rejected") || strings.Contains(l, "missing") || strings.Contains(l, "wrong_address") || strings.Contains(l, "incomplete_address") || strings.Contains(l, "unknown_receiver") || strings.Contains(l, "oversized"):
		return model.StatusException, false, delayed
	case strings.Contains(l, "delivered") || strings.Contains(l, "eol.1001"):
		return model.StatusDelivered, true, delayed
	case strings.Contains(l, "ready_to_pickup") || strings.Contains(l, "awaiting_collection"):
		return model.StatusReadyForPickup, false, delayed
	case strings.Contains(l, "out_for_delivery"):
		return model.StatusOutForDelivery, false, delayed
	case delayed:
		return model.StatusDelayed, false, true
	case strings.Contains(l, "stored by customer") || strings.Contains(l, "psc"):
		return model.StatusAccepted, false, delayed
	case strings.Contains(l, "ready for dispatch") || strings.Contains(l, "prd") || strings.Contains(l, "confirmed") || strings.Contains(l, "created") || strings.Contains(l, "offer_"):
		return model.StatusPreAdvice, false, delayed
	case strings.Contains(l, "dispatched") || strings.Contains(l, "collected") || strings.Contains(l, "taken_by_courier") || strings.Contains(l, "adopted") || strings.Contains(l, "sent_from") || strings.Contains(l, "in transit") || strings.Contains(l, "redirect_to_box") || strings.Contains(l, "readdressed"):
		return model.StatusInTransit, false, delayed
	case strings.Contains(l, "not found"):
		return model.StatusNotFound, false, delayed
	default:
		return model.StatusUnknown, false, delayed
	}
}

func humanStatus(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	known := map[string]string{
		"prd":                                  "Ready For Dispatch",
		"psc":                                  "Parcel Stored by Customer",
		"created":                              "Shipment created",
		"confirmed":                            "Prepared by sender",
		"dispatched_by_sender":                 "Dispatched by sender",
		"collected_from_sender":                "Collected from sender",
		"taken_by_courier":                     "Collected by courier",
		"adopted_at_source_branch":             "Accepted at source branch",
		"sent_from_source_branch":              "Sent from source branch",
		"adopted_at_sorting_center":            "Accepted at sorting center",
		"sent_from_sorting_center":             "Sent from sorting center",
		"adopted_at_target_branch":             "Accepted at target branch",
		"out_for_delivery":                     "Out for delivery",
		"out_for_delivery_to_address":          "Out for delivery",
		"ready_to_pickup":                      "Ready for pickup",
		"ready_to_pickup_from_pok":             "Ready for pickup",
		"ready_to_pickup_from_branch":          "Ready for pickup",
		"delivered":                            "Delivered",
		"pickup_time_expired":                  "Pickup time expired",
		"returned_to_sender":                   "Returned to sender",
		"delay_in_delivery":                    "Delayed in delivery",
		"canceled":                             "Canceled",
		"missing":                              "Missing",
		"undelivered":                          "Undelivered",
		"undelivered_wrong_address":            "Undelivered: wrong address",
		"undelivered_incomplete_address":       "Undelivered: incomplete address",
		"undelivered_unknown_receiver":         "Undelivered: unknown receiver",
		"undelivered_cod_cash_receiver":        "Undelivered: cash on delivery issue",
		"undelivered_no_mailbox":               "Undelivered: no mailbox",
		"undelivered_not_live_address":         "Undelivered: recipient not at address",
		"undelivered_lack_of_access_letterbox": "Undelivered: lack of access",
	}
	if v, ok := known[strings.ToLower(s)]; ok {
		return v
	}
	parts := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(s, "_", " "), ".", " "))
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + strings.ToLower(p[1:])
	}
	return strings.Join(parts, " ")
}

func eventCodes(events []model.Event) string {
	var parts []string
	for _, e := range events {
		parts = append(parts, e.RawCode, e.Text)
	}
	sort.Strings(parts)
	return strings.Join(parts, "\n")
}

func source(sourceURL string) model.Source {
	return model.Source{Method: "api", URL: sourceURL, FetchedAt: time.Now().UTC().Format(time.RFC3339)}
}

func normalizeNumber(s string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(s), " ", ""))
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
