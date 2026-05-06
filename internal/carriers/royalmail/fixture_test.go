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
