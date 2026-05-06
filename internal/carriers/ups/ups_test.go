package ups

import (
	"testing"

	"github.com/cavit99/parcelcli/internal/model"
)

func TestUPSClassifyDelivered(t *testing.T) {
	status, delivered, delayed := classify("Delivered Left at front door")
	if status != model.StatusDelivered || !delivered || delayed {
		t.Fatalf("status=%s delivered=%v delayed=%v", status, delivered, delayed)
	}
}
