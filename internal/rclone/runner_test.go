package rclone

import (
	"testing"

	"github.com/Origin173/SnapCraft/internal/config"
)

func TestTransferConfig(t *testing.T) {
	cfg := &config.Config{
		Rclone: config.RcloneConfig{
			BwLimit:   "10M",
			Transfers: 4,
			Checkers:  8,
			Retries:   3,
		},
	}
	r := NewEmbeddedRunner(cfg)
	got := r.transferConfig("remote:backup")
	if got["BwLimit"] != "10M" {
		t.Fatalf("BwLimit = %v", got["BwLimit"])
	}
	if got["Transfers"] != 4 {
		t.Fatalf("Transfers = %v", got["Transfers"])
	}
	if got["Checkers"] != 8 {
		t.Fatalf("Checkers = %v", got["Checkers"])
	}
	if got["LowLevelRetries"] != 3 {
		t.Fatalf("LowLevelRetries = %v", got["LowLevelRetries"])
	}
	if got["BackupDir"] != "remote:backup" {
		t.Fatalf("BackupDir = %v", got["BackupDir"])
	}
}

func TestParseKeyValues(t *testing.T) {
	got, err := ParseKeyValues([]string{"url=https://example.com", "vendor=other"})
	if err != nil {
		t.Fatal(err)
	}
	if got["url"] != "https://example.com" || got["vendor"] != "other" {
		t.Fatalf("unexpected map: %#v", got)
	}
	_, err = ParseKeyValues([]string{"invalid"})
	if err == nil {
		t.Fatal("expected error for invalid key=value")
	}
}

func TestParseRPCError(t *testing.T) {
	err := parseRPCError("sync/copy", `{"error":"boom","status":500}`, 500)
	if err == nil || err.Error() == "" {
		t.Fatalf("expected error, got %v", err)
	}
}

type fakeRPC struct {
	lastMethod string
	lastParams map[string]any
	response   map[string]any
	err        error
}

func (f *fakeRPC) call(method string, params map[string]any) (map[string]any, error) {
	f.lastMethod = method
	f.lastParams = params
	return f.response, f.err
}

func TestEmbeddedRunnerCopyUsesSyncCopy(t *testing.T) {
	rpc := &fakeRPC{response: map[string]any{"ok": true}}
	r := &EmbeddedRunner{
		cfg: &config.Config{Rclone: config.RcloneConfig{Transfers: 2}},
		rpc: rpc,
	}
	if err := r.Copy(t.Context(), "/tmp/a", "remote:path/a"); err != nil {
		t.Fatal(err)
	}
	if rpc.lastMethod != "sync/copy" {
		t.Fatalf("method = %q, want sync/copy", rpc.lastMethod)
	}
	if rpc.lastParams["srcFs"] != "/tmp/a" || rpc.lastParams["dstFs"] != "remote:path/a" {
		t.Fatalf("params = %#v", rpc.lastParams)
	}
}

func TestEnsureRemoteConfigured(t *testing.T) {
	cfg := &config.Config{
		Upload: config.UploadConfig{Enabled: true},
		Rclone: config.RcloneConfig{Remote: "missing"},
	}
	if err := EnsureRemoteConfigured(cfg); err == nil {
		t.Fatal("expected error for missing remote")
	}
}
