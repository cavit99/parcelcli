package watch

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestItemJSONUsesSnakeCase(t *testing.T) {
	b, err := json.Marshal(Item{ID: "abc", Carrier: "ups", TrackingNumber: "TRACKING_NUMBER", AddedAt: "DATE"})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"id"`, `"carrier"`, `"tracking_number"`, `"added_at"`} {
		if !strings.Contains(s, want) {
			t.Fatalf("json %s missing %s", s, want)
		}
	}
	for _, bad := range []string{`"ID"`, `"Carrier"`, `"TrackingNumber"`} {
		if strings.Contains(s, bad) {
			t.Fatalf("json %s contains exported key %s", s, bad)
		}
	}
}

func TestLoadCorruptJSONReturnsNilState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if err := os.MkdirAll(ConfigDir(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(), []byte(`{"items":[`), 0600); err != nil {
		t.Fatal(err)
	}
	st, err := Load()
	if err == nil {
		t.Fatal("expected corrupt JSON error")
	}
	if st != nil {
		t.Fatalf("state = %#v, want nil", st)
	}
}
