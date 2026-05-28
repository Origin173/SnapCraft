package rclone

import (
	"testing"

	"github.com/Origin173/SnapCraft/internal/config"
)

func TestRemoteBaseName(t *testing.T) {
	if got := RemoteBaseName("myremote:crypt"); got != "myremote" {
		t.Fatalf("base = %q", got)
	}
	if got := RemoteSubpath("myremote:crypt"); got != "crypt" {
		t.Fatalf("subpath = %q", got)
	}
}

func TestJoinRemoteSpec(t *testing.T) {
	if got := JoinRemoteSpec("myremote", "crypt"); got != "myremote:crypt" {
		t.Fatalf("join = %q", got)
	}
	if got := JoinRemoteSpec("myremote", ""); got != "myremote" {
		t.Fatalf("join empty = %q", got)
	}
}

func TestBuildUploadFS(t *testing.T) {
	got := BuildUploadFS("myremote:crypt", "snapcraft/test")
	want := "myremote:crypt:snapcraft/test"
	if got != want {
		t.Fatalf("full fs = %q, want %q", got, want)
	}
}

func TestFilterCreateParameters(t *testing.T) {
	got := FilterCreateParameters(map[string]string{
		"url":  "https://example.com",
		"pass": "",
		"user": redactedPlaceholder,
		"vendor": "other",
	})
	if len(got) != 2 || got["url"] == "" || got["vendor"] == "" {
		t.Fatalf("filtered = %#v", got)
	}
}

func TestValidateRemoteParameters(t *testing.T) {
	if err := ValidateRemoteParameters("webdav", map[string]string{}, true); err == nil {
		t.Fatal("expected error for missing url")
	}
	if err := ValidateRemoteParameters("webdav", map[string]string{"url": "127.0.0.1/dav/"}, true); err == nil {
		t.Fatal("expected error for missing scheme")
	}
	if err := ValidateRemoteParameters("webdav", map[string]string{"url": "http://127.0.0.1:5244/dav/"}, true); err != nil {
		t.Fatal(err)
	}
}

func TestPrepareRemoteParameters(t *testing.T) {
	params, err := PrepareRemoteParameters("webdav", map[string]string{
		"vendor": "openlist",
		"url":    "http://127.0.0.1:5244/dav/",
		"user":   "admin",
	}, true)
	if err != nil {
		t.Fatal(err)
	}
	if params["vendor"] != "other" {
		t.Fatalf("vendor = %q", params["vendor"])
	}
}

func TestUploadRemoteStatusFor(t *testing.T) {
	old := configRPC
	configRPC = &fakeRPC{response: map[string]any{"remotes": []any{"myremote", "other"}}}
	t.Cleanup(func() { configRPC = old })

	cfg := testConfig()
	cfg.Upload.Enabled = true
	cfg.Rclone.Remote = "myremote:crypt"
	cfg.Rclone.RemotePath = "snapcraft/test"

	status, err := UploadRemoteStatusFor(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !status.RemoteExists || !status.Configured || status.FullFS != "myremote:crypt:snapcraft/test" {
		t.Fatalf("status = %#v", status)
	}
}

func testConfig() *config.Config {
	return &config.Config{}
}
