package snapshot

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

const IDTimeLayout = "2006-01-02T15-04-05Z"

// NewID generates a snapshot ID: UTC timestamp + 6-char random suffix.
func NewID() string {
	ts := time.Now().UTC().Format(IDTimeLayout)
	suffix := randomSuffix(3)
	return fmt.Sprintf("%s-%s", ts, suffix)
}

func randomSuffix(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	return hex.EncodeToString(b)
}

// ParseTime extracts the UTC timestamp from a snapshot ID.
func ParseTime(id string) (time.Time, error) {
	// Format: 2026-05-28T08-30-00Z-a1b2c3
	if len(id) < len(IDTimeLayout) {
		return time.Time{}, fmt.Errorf("invalid snapshot id: %q", id)
	}
	ts := id[:len(IDTimeLayout)]
	return time.Parse(IDTimeLayout, ts)
}
