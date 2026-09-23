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

func TestRoyalMailSummaryIgnoresHelpAndHistory(t *testing.T) {
	piece := mailPiece{
		Summary: &summary{
			StatusDescription: "We've got it",
			StatusHelpText:    "My item is shown as delivered but it hasn't been",
			LastEventCode:     "EVIMC",
		},
		Events: []event{
			{EventName: "We have your item"},
			{EventName: "Your item was delivered"},
		},
	}
	res := normalizePiece("TRACKING_NUMBER", piece, nil)
	if res.Status != model.StatusInTransit || res.Delivered || res.Terminal {
		t.Fatalf("status=%s delivered=%v terminal=%v", res.Status, res.Delivered, res.Terminal)
	}
}
