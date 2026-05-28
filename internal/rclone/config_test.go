package rclone

import "testing"

func TestToAnyMap(t *testing.T) {
	in := map[string]string{"url": "https://example.com", "vendor": "other"}
	got := toAnyMap(in)
	if got["url"] != "https://example.com" {
		t.Fatalf("url = %v", got["url"])
	}
}

func TestCreateRemotePayload(t *testing.T) {
	rpc := &fakeRPC{response: map[string]any{}}
	old := configRPC
	configRPC = rpc
	t.Cleanup(func() { configRPC = old })

	if err := CreateRemote("webdav", "webdav", map[string]string{"url": "https://example.com"}); err != nil {
		t.Fatal(err)
	}
	if rpc.lastMethod != "config/create" {
		t.Fatalf("method = %q", rpc.lastMethod)
	}
	if rpc.lastParams["name"] != "webdav" || rpc.lastParams["type"] != "webdav" {
		t.Fatalf("params = %#v", rpc.lastParams)
	}
}
