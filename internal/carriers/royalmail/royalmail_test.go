package royalmail

import (
	"testing"

	"github.com/cavit99/parcelcli/internal/model"
)

func TestRoyalMailClassifyDelivered(t *testing.T) {
	status, delivered, delayed := classify("Your item was delivered", "")
	if status != model.StatusDelivered || !delivered || delayed {
		t.Fatalf("status=%s delivered=%v delayed=%v", status, delivered, delayed)
	}
}
