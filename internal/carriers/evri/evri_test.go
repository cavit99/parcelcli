package evri

import (
	"testing"

	"github.com/cavit99/parcelcli/internal/model"
)

func TestClassifyDelayedBeforeOutForDeliveryPhrase(t *testing.T) {
	status, delivered, delayed := classify("We're sorry, your parcel has been delayed. We'll let you know once it's out for delivery")
	if status != model.StatusDelayed || delivered || !delayed {
		t.Fatalf("classify delayed = %s delivered=%v delayed=%v", status, delivered, delayed)
	}
}

func TestExtractEvriRenderedText(t *testing.T) {
	body := "Your parcel from AMAZON\nUpdate on your parcel\n10:58 - Tue May 05\nWe're sorry, your parcel has been delayed.\n10:58 - Tue May 05\nWe're sorry, your parcel has been delayed."
	status, sender, lastTime, lastEvent := extract(body)
	if sender != "Your parcel from AMAZON" {
		t.Fatalf("sender = %q", sender)
	}
	if status != "We're sorry, your parcel has been delayed." {
		t.Fatalf("status = %q", status)
	}
	if lastTime != "10:58 - Tue May 05" || lastEvent != "We're sorry, your parcel has been delayed." {
		t.Fatalf("event = %q / %q", lastTime, lastEvent)
	}
}
