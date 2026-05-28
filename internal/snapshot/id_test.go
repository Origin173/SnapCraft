package snapshot

import (
	"testing"
	"time"
)

func TestNewIDFormat(t *testing.T) {
	id := NewID()
	if len(id) < len(IDTimeLayout)+2 {
		t.Fatalf("id too short: %q", id)
	}
	ts, err := ParseTime(id)
	if err != nil {
		t.Fatalf("ParseTime() error = %v", err)
	}
	if ts.After(time.Now().UTC().Add(time.Minute)) {
		t.Error("parsed time is in the future")
	}
}

func TestManifestJSON(t *testing.T) {
	m := &Manifest{
		ID:         "2026-05-28T08-30-00Z-a1b2c3",
		ServerName: "test",
		Status:     StatusCompleted,
	}
	data, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ID != m.ID {
		t.Errorf("ID = %q, want %q", parsed.ID, m.ID)
	}
}
