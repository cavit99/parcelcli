package inpost

import (
	"testing"

	"github.com/cavit99/parcelcli/internal/model"
)

func TestResultFromJSONNotFound(t *testing.T) {
	body := []byte(`{"trackingNumber":"JJD0002219933896965","message":"Tracking information about JJD0002219933896965 shipment has not been found."}`)
	res, err := resultFromJSON("JJD0002219933896965", baseURL+"JJD0002219933896965", 404, body)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != model.StatusNotFound || res.Delivered || res.Terminal {
		t.Fatalf("status=%s delivered=%v terminal=%v", res.Status, res.Delivered, res.Terminal)
	}
	if res.StatusText == "" {
		t.Fatal("status_text is empty")
	}
}

func TestResultFromJSONDelivered(t *testing.T) {
	body := []byte(`{
		"tracking_number": "JJD0000000000000000",
		"service": "inpost_locker_standard",
		"type": "inpost_locker_standard",
		"status": "delivered",
		"tracking_details": [
			{"status":"confirmed","origin_status":"confirmed","agency":"InPost","datetime":"2026-06-01T10:00:00.000+01:00"},
			{"status":"delivered","origin_status":"delivered","agency":"InPost","datetime":"2026-06-02T12:30:00.000+01:00","location":"London"}
		]
	}`)
	res, err := resultFromJSON("JJD0000000000000000", baseURL+"JJD0000000000000000", 200, body)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != model.StatusDelivered || !res.Delivered || !res.Terminal {
		t.Fatalf("status=%s delivered=%v terminal=%v", res.Status, res.Delivered, res.Terminal)
	}
	if res.LastEvent == nil || res.LastEvent.RawCode != "delivered" || res.LastEvent.Location != "London" {
		t.Fatalf("last_event=%#v", res.LastEvent)
	}
	if res.Raw["service"] != "inpost_locker_standard" {
		t.Fatalf("raw=%#v", res.Raw)
	}
}

func TestClassifyInPostStatuses(t *testing.T) {
	tests := []struct {
		input     string
		status    model.Status
		delivered bool
		delayed   bool
	}{
		{"ready_to_pickup", model.StatusReadyForPickup, false, false},
		{"out_for_delivery_to_address", model.StatusOutForDelivery, false, false},
		{"delay_in_delivery", model.StatusDelayed, false, true},
		{"returned_to_sender", model.StatusReturned, false, false},
		{"undelivered_wrong_address", model.StatusDeliveryAttempted, false, false},
	}
	for _, tt := range tests {
		status, delivered, delayed := classify(tt.input)
		if status != tt.status || delivered != tt.delivered || delayed != tt.delayed {
			t.Fatalf("%q => %s delivered=%v delayed=%v", tt.input, status, delivered, delayed)
		}
	}
}
