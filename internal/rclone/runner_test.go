package rclone

import (
	"testing"

	"github.com/Origin173/SnapCraft/internal/config"
)

func TestBaseArgs(t *testing.T) {
	cfg := &config.Config{
		Rclone: config.RcloneConfig{
			Binary:    "rclone",
			BwLimit:   "10M",
			Transfers: 4,
			Checkers:  8,
			Retries:   3,
		},
	}
	r := NewExecRunner(cfg)
	args := r.baseArgs("copy", "/local", "remote:path")
	want := []string{"copy", "--bwlimit", "10M", "--transfers", "4", "--checkers", "8", "--retries", "3", "/local", "remote:path"}
	if len(args) != len(want) {
		t.Fatalf("args = %v, want %v", args, want)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d] = %q, want %q", i, args[i], want[i])
		}
	}
}
