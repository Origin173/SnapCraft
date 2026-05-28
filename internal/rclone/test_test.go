package rclone

import (
	"strings"
	"testing"
)

func TestHumanizeTestError(t *testing.T) {
	raw := "rclone rpc operations/list: the remote url looks incorrect. Note that nextcloud chunked uploads require you to use the /dav/files/USER endpoint instead of /webdav."
	msg, hint := humanizeTestError(raw)
	if msg != "WebDAV vendor 与地址不匹配" {
		t.Fatalf("msg = %q", msg)
	}
	if hint == "" || !strings.Contains(hint, "OpenList") {
		t.Fatalf("hint = %q", hint)
	}
}

func TestHumanizeEmptyURL(t *testing.T) {
	msg, hint := humanizeTestError(`Propfind "/": unsupported protocol scheme ""`)
	if msg != "WebDAV url 未配置或格式错误" {
		t.Fatalf("msg = %q", msg)
	}
	if hint == "" {
		t.Fatal("expected hint")
	}
}
