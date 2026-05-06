package dhl

import (
	"testing"

	"github.com/cavit99/parcelcli/internal/model"
)

func TestDHLClassifyCustomsClearanceAsTransitUnlessProblem(t *testing.T) {
	status, delivered, delayed := classify("Customs clearance process started")
	if status != model.StatusInTransit || delivered || delayed {
		t.Fatalf("status=%s delivered=%v delayed=%v", status, delivered, delayed)
	}
	status, _, _ = classify("Customs clearance delay - action required")
	if status != model.StatusException {
		t.Fatalf("status=%s, want exception", status)
	}
}
