package webui

import "testing"

func TestLogStoreAppendAndList(t *testing.T) {
	store := NewLogStore(3)
	store.Append("info", "server", "started", nil)
	store.Append("warn", "auth", "failed login", map[string]any{"ip": "127.0.0.1"})
	store.Append("error", "job", "backup failed", nil)
	store.Append("info", "job", "newest", nil)

	all := store.List("", "", 0)
	if len(all) != 3 {
		t.Fatalf("capacity trim = %d", len(all))
	}
	if all[0].Message != "newest" {
		t.Fatalf("newest first = %q", all[0].Message)
	}

	jobs := store.List("", "job", 0)
	if len(jobs) != 2 {
		t.Fatalf("job filter = %d", len(jobs))
	}

	errs := store.List("error", "", 0)
	if len(errs) != 1 || errs[0].Message != "backup failed" {
		t.Fatalf("level filter = %#v", errs)
	}
}

func TestLogStoreClear(t *testing.T) {
	store := NewLogStore(10)
	store.Append("info", "api", "x", nil)
	store.Clear()
	if len(store.List("", "", 0)) != 0 {
		t.Fatal("expected empty store")
	}
}

func TestMergeRemoteParameters(t *testing.T) {
	existing := map[string]string{"pass": "secret", "url": "https://old"}
	incoming := map[string]string{"pass": redactedSecret, "url": "https://new"}
	got := mergeRemoteParameters(incoming, existing)
	if got["pass"] != "secret" {
		t.Fatalf("pass = %q", got["pass"])
	}
	if got["url"] != "https://new" {
		t.Fatalf("url = %q", got["url"])
	}
}
