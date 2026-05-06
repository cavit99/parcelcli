package model

import "time"

type Status string

const (
	StatusUnknown           Status = "unknown"
	StatusPreAdvice         Status = "pre_advice"
	StatusAccepted          Status = "accepted"
	StatusInTransit         Status = "in_transit"
	StatusOutForDelivery    Status = "out_for_delivery"
	StatusDelayed           Status = "delayed"
	StatusDeliveryAttempted Status = "delivery_attempted"
	StatusReadyForPickup    Status = "ready_for_pickup"
	StatusDelivered         Status = "delivered"
	StatusReturned          Status = "returned"
	StatusException         Status = "exception"
	StatusNotFound          Status = "not_found"
)

type Event struct {
	Time     string `json:"time,omitempty"`
	Text     string `json:"text,omitempty"`
	Location string `json:"location,omitempty"`
	RawCode  string `json:"raw_code,omitempty"`
}

type Source struct {
	Method    string `json:"method"`
	URL       string `json:"url,omitempty"`
	FetchedAt string `json:"fetched_at"`
}

type Result struct {
	Carrier           string         `json:"carrier"`
	TrackingNumber    string         `json:"tracking_number"`
	Postcode          string         `json:"postcode,omitempty"`
	Status            Status         `json:"status"`
	StatusText        string         `json:"status_text,omitempty"`
	Terminal          bool           `json:"terminal"`
	Delivered         bool           `json:"delivered"`
	Delayed           bool           `json:"delayed"`
	SenderLine        string         `json:"sender_line,omitempty"`
	EstimatedDelivery string         `json:"estimated_delivery,omitempty"`
	LastEvent         *Event         `json:"last_event,omitempty"`
	Events            []Event        `json:"events,omitempty"`
	Requires          []string       `json:"requires,omitempty"`
	Source            Source         `json:"source"`
	Raw               map[string]any `json:"raw,omitempty"`
}

type TrackRequest struct {
	TrackingNumber string
	Postcode       string
	ChromePath     string
	Timeout        time.Duration
	Debug          bool
}
