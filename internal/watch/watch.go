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
	ID, Carrier, TrackingNumber, Postcode, Label string `json:",omitempty"`
	AddedAt                                      string `json:"added_at"`
	LastHash                                     string `json:"last_hash,omitempty"`
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
	return &s, json.Unmarshal(b, &s)
}
func Save(s *State) error {
	if err := os.MkdirAll(ConfigDir(), 0700); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return os.WriteFile(Path(), b, 0600)
}
func NewID(carrier, number string) string {
	h := sha1.Sum([]byte(carrier + ":" + number + ":" + time.Now().Format(time.RFC3339Nano)))
	return hex.EncodeToString(h[:])[:10]
}
func Hash(v any) string { b, _ := json.Marshal(v); h := sha1.Sum(b); return hex.EncodeToString(h[:]) }
