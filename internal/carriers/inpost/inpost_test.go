package inpost

import (
	"testing"

	"github.com/cavit99/parcelcli/internal/model"
)

func TestResultFromJSONNotFound(t *testing.T) {
	body := []byte(`{"JJD0002219933896965":{"error":"No events found for consignment JJD0002219933896965"}}`)
	res, err := resultFromJSON("JJD0002219933896965", baseURL+"JJD0002219933896965", 200, body)
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

func TestResultFromJSONAccepted(t *testing.T) {
	body := []byte(`{
		"JJD0002219933896965": [
			{"ts":"02\/06\/2026 14:56:21","code":"PRD","description":"Ready For Dispatch","remark":""},
			{"ts":"03\/06\/2026 09:00:42","code":"PSC","description":"Parcel Stored by Customer","remark":""}
		]
	}`)
	res, err := resultFromJSON("JJD0002219933896965", baseURL+"JJD0002219933896965", 200, body)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != model.StatusAccepted || res.Delivered || res.Terminal {
		t.Fatalf("status=%s delivered=%v terminal=%v", res.Status, res.Delivered, res.Terminal)
	}
	if res.StatusText != "Your parcel is about to hit the road" {
		t.Fatalf("status_text=%q", res.StatusText)
	}
	if res.LastEvent == nil || res.LastEvent.RawCode != "PSC" || res.LastEvent.Text != "Parcel Stored by Customer" {
		t.Fatalf("last_event=%#v", res.LastEvent)
	}
	if res.Raw["carrier_status_text"] != "Parcel Stored by Customer" {
		t.Fatalf("raw=%#v", res.Raw)
	}
	if res.Raw["http_status"] != 200 {
		t.Fatalf("raw=%#v", res.Raw)
	}
}

func TestResultFromJSONDelivered(t *testing.T) {
	body := []byte(`{
		"JJD0000000000000000": [
			{"ts":"01\/06\/2026 10:00:00","code":"PSC","description":"Parcel Stored by Customer","remark":""},
			{"ts":"02\/06\/2026 12:30:00","code":"EOL.1001","description":"Delivered","remark":""}
		]
	}`)
	res, err := resultFromJSON("JJD0000000000000000", baseURL+"JJD0000000000000000", 200, body)
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != model.StatusDelivered || !res.Delivered || !res.Terminal {
		t.Fatalf("status=%s delivered=%v terminal=%v", res.Status, res.Delivered, res.Terminal)
	}
	if res.LastEvent == nil || res.LastEvent.RawCode != "EOL.1001" {
		t.Fatalf("last_event=%#v", res.LastEvent)
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
		{"PRD\nPSC\nParcel Stored by Customer", model.StatusAccepted, false, false},
	}
	for _, tt := range tests {
		status, delivered, delayed := classify(tt.input)
		if status != tt.status || delivered != tt.delivered || delayed != tt.delayed {
			t.Fatalf("%q => %s delivered=%v delayed=%v", tt.input, status, delivered, delayed)
		}
	}
}
