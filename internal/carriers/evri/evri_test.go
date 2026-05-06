package evri

import "testing"

func TestExtractRoughTrackingWithoutPostcode(t *testing.T) {
	body := `Track another parcel
Your parcel from Amazon MFN Tier 6
T0435A1584097005
Out for delivery
Your local courier will deliver your parcel between 12:30 and 14:30 today
Delivery date
Wed 6th May
Delivery time
12:30 - 14:30
Barcode number: T0435A1584097005
We're expecting it
We've got it
On its way
Out for delivery
Delivered`

	status, sender, _, _, eta := extract(body, "T0435A1584097005")
	if status != "Out for delivery" {
		t.Fatalf("status = %q, want Out for delivery", status)
	}
	if sender != "Your parcel from Amazon MFN Tier 6" {
		t.Fatalf("sender = %q", sender)
	}
	if eta != "Your local courier will deliver your parcel between 12:30 and 14:30 today" {
		t.Fatalf("eta = %q", eta)
	}

	got, delivered, delayed := classify(status)
	if got != "out_for_delivery" || delivered || delayed {
		t.Fatalf("classify = %q delivered=%v delayed=%v", got, delivered, delayed)
	}
}

func TestClassifyDelayedBeforeOutForDeliveryPhrase(t *testing.T) {
	status, delivered, delayed := classify("We're sorry, your parcel has been delayed. We'll let you know once it's out for delivery")
	if status != "delayed" || delivered || !delayed {
		t.Fatalf("classify delayed = %s delivered=%v delayed=%v", status, delivered, delayed)
	}
}

func TestExtractEvriRenderedText(t *testing.T) {
	body := "Your parcel from AMAZON\nUpdate on your parcel\n10:58 - Tue May 05\nWe're sorry, your parcel has been delayed.\n10:58 - Tue May 05\nWe're sorry, your parcel has been delayed."
	status, sender, lastTime, lastEvent, eta := extract(body, "T0435A1584097005")
	if sender != "Your parcel from AMAZON" {
		t.Fatalf("sender = %q", sender)
	}
	if status != "We're sorry, your parcel has been delayed." {
		t.Fatalf("status = %q", status)
	}
	if lastTime != "10:58 - Tue May 05" || lastEvent != "We're sorry, your parcel has been delayed." {
		t.Fatalf("event = %q / %q", lastTime, lastEvent)
	}
	if eta != "" {
		t.Fatalf("eta = %q", eta)
	}
}
