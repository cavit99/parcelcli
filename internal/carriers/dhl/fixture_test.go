package dhl

import (
	"testing"

	"github.com/cavit99/parcelcli/internal/model"
)

func TestResultFromRenderedFixture(t *testing.T) {
	body := `Shipment details
Shipment number: TRACKING_NUMBER
Customs clearance process started
Destination country/region:
COUNTRY
Detailed tracking history
Mo, 20.04.2026, 11:56, COUNTRY
The customs clearance process for import into the destination country/region has started.`
	res := resultFromRendered("TRACKING_NUMBER", body, nil)
	if res.Status != model.StatusInTransit || res.Delivered || res.Delayed {
		t.Fatalf("status=%s delivered=%v delayed=%v", res.Status, res.Delivered, res.Delayed)
	}
	if res.StatusText != "Customs clearance process started" {
		t.Fatalf("status_text=%q", res.StatusText)
	}
	if res.LastEvent == nil || res.LastEvent.Location != "COUNTRY" {
		t.Fatalf("last_event=%#v", res.LastEvent)
	}
	if res.Raw["destination_country_region"] != "COUNTRY" {
		t.Fatalf("raw=%#v", res.Raw)
	}
}
