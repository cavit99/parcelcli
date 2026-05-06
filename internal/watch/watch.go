package watch

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

type Item struct {
	ID             string `json:"id"`
	Carrier        string `json:"carrier"`
	TrackingNumber string `json:"tracking_number"`
	Postcode       string `json:"postcode,omitempty"`
	Label          string `json:"label,omitempty"`
	AddedAt        string `json:"added_at"`
	LastHash       string `json:"last_hash,omitempty"`
}
type State struct {
	Items []Item `json:"items"`
}

func ConfigDir() string {
	if runtime.GOOS == "darwin" {
		h, _ := os.UserHomeDir()
		return filepath.Join(h, "Library", "Application Support", "parcelcli")
	}
	d, _ := os.UserConfigDir()
	return filepath.Join(d, "parcelcli")
}
func Path() string { return filepath.Join(ConfigDir(), "watch.json") }
func Load() (*State, error) {
	b, err := os.ReadFile(Path())
	if errors.Is(err, os.ErrNotExist) {
		return &State{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s State
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	return &s, nil
}
func Save(s *State) error {
	if err := os.MkdirAll(ConfigDir(), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(Path(), b, 0600)
}
func NewID(carrier, number string) string {
	h := sha1.Sum([]byte(carrier + ":" + number + ":" + time.Now().Format(time.RFC3339Nano)))
	return hex.EncodeToString(h[:])[:10]
}
func Hash(v any) string { b, _ := json.Marshal(v); h := sha1.Sum(b); return hex.EncodeToString(h[:]) }
