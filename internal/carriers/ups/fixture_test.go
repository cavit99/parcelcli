package ups

import (
	"testing"

	"github.com/cavit99/parcelcli/internal/model"
)

func TestResultFromRenderedFixture(t *testing.T) {
	body := `Tracking Details
Delivered check_circle
TRACKING_NUMBER content_copy
DATE Delivered
Delivered To
CITY, REGION
Received By
RECIPIENT`
	res := resultFromRendered("TRACKING_NUMBER", body, nil)
	if res.Status != model.StatusDelivered || !res.Delivered || !res.Terminal {
		t.Fatalf("status=%s delivered=%v terminal=%v", res.Status, res.Delivered, res.Terminal)
	}
	if res.LastEvent == nil || res.LastEvent.Text != "Delivered" || res.LastEvent.Location != "CITY, REGION" {
		t.Fatalf("last_event=%#v", res.LastEvent)
	}
	if got := res.Raw["received_by"]; got != "RECIPIENT" {
		t.Fatalf("received_by=%v", got)
	}
}
