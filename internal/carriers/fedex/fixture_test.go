package fedex

import (
	"testing"

	"github.com/cavit99/parcelcli/internal/model"
)

func TestResultFromRenderedFixture(t *testing.T) {
	body := `FedEx Tracking
TRACKING_NUMBER
DELIVERY DETAILS
Delivery updated
FROM
ORIGIN CITY, REGION
TO
DESTINATION CITY, REGION
Delivery updated
CITY, REGION
5/6/26 2:11 AM`
	res := resultFromRendered("TRACKING_NUMBER", body, nil)
	if res.Status != model.StatusDelayed || !res.Delayed || res.Delivered {
		t.Fatalf("status=%s delayed=%v delivered=%v", res.Status, res.Delayed, res.Delivered)
	}
	if res.StatusText != "Delivery updated" {
		t.Fatalf("status_text=%q", res.StatusText)
	}
	if res.LastEvent == nil || res.LastEvent.Time != "5/6/26 2:11 AM" || res.LastEvent.Text != "Delivery updated" || res.LastEvent.Location != "CITY, REGION" {
		t.Fatalf("last_event=%#v", res.LastEvent)
	}
	if res.Raw["from"] != "ORIGIN CITY, REGION" || res.Raw["to"] != "DESTINATION CITY, REGION" {
		t.Fatalf("raw=%#v", res.Raw)
	}
}
