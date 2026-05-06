package fedex

import (
	"testing"

	"github.com/cavit99/parcelcli/internal/model"
)

func TestFedExClassifyDelayBeatsOutForDelivery(t *testing.T) {
	status, delivered, delayed := classify("Delivery updated. Your package is still on the way and we are working to get you a new delivery date. Out for delivery")
	if status != model.StatusDelayed || delivered || !delayed {
		t.Fatalf("status=%s delivered=%v delayed=%v", status, delivered, delayed)
	}
}

func TestFedExDetailedResultAndNotFoundDetection(t *testing.T) {
	if !hasDetailedResult("DELIVERY DETAILS\nDelivery updated\nTRACKING_NUMBER\nDATE TIME", "TRACKING_NUMBER") {
		t.Fatal("expected detailed result")
	}
	if hasDetailedResult("We can’t find that tracking number TRACKING_NUMBER", "TRACKING_NUMBER") {
		t.Fatal("not-found page is not a detailed result")
	}
	if !hasNotFoundText("We can’t find that tracking number TRACKING_NUMBER", "TRACKING_NUMBER") {
		t.Fatal("expected not-found text")
	}
}
