package royalmail

import (
	"testing"

	"github.com/cavit99/parcelcli/internal/model"
)

func TestResultFromRenderedFixture(t *testing.T) {
	body := `Royal Mail
TRACKING_NUMBER
Your item was delivered
Signed for by RECIPIENT`
	res := resultFromRendered("TRACKING_NUMBER", body, nil)
	if res.Status != model.StatusDelivered || !res.Delivered || !res.Terminal {
		t.Fatalf("status=%s delivered=%v terminal=%v", res.Status, res.Delivered, res.Terminal)
	}
	if res.StatusText != "Your item was delivered" {
		t.Fatalf("status_text=%q", res.StatusText)
	}
}

func TestResultFromRenderedIgnoresHelpLink(t *testing.T) {
	body := `Track your item
We've got it
We expect to deliver it tomorrow between 10:00am - 2:00pm
We have your item at Manchester MC and it's on its way.
Tracking number:
TRACKING_NUMBER
Need help?
My item is shown as delivered but it hasn't been`
	if !hasResultText(body, "TRACKING_NUMBER") {
		t.Fatal("actual tracking result was not recognized")
	}
	res := resultFromRendered("TRACKING_NUMBER", body, nil)
	if res.Status != model.StatusInTransit || res.Delivered || res.Terminal || res.StatusText != "We've got it" {
		t.Fatalf("status=%s delivered=%v terminal=%v status_text=%q", res.Status, res.Delivered, res.Terminal, res.StatusText)
	}
}

func TestHelpLinkIsNotTrackingResult(t *testing.T) {
	body := `Track your item
Need help?
My item is shown as delivered but it hasn't been`
	if hasResultText(body, "TRACKING_NUMBER") {
		t.Fatal("help link was mistaken for a tracking result")
	}
	res := resultFromRendered("TRACKING_NUMBER", body, nil)
	if res.Status != model.StatusUnknown || res.Delivered || res.Terminal {
		t.Fatalf("status=%s delivered=%v terminal=%v", res.Status, res.Delivered, res.Terminal)
	}
}
