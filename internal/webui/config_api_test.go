package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Origin173/SnapCraft/internal/config"
)

func TestConfigGetAndSave(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := testConfig(t)
	cfg.Server.WorldPath = filepath.Join(dir, "world")
	if err := os.MkdirAll(cfg.Server.WorldPath, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewServer(loaded, path)
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("get config without auth = %d", rec.Code)
	}

	loginBody, _ := json.Marshal(map[string]string{"token": "secret-token"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	s.handler.ServeHTTP(loginRec, loginReq)

	getReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	for _, c := range loginRec.Result().Cookies() {
		getReq.AddCookie(c)
	}
	getRec := httptest.NewRecorder()
	s.handler.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get config = %d", getRec.Code)
	}

	var getResp struct {
		Path   string    `json:"path"`
		Config ConfigDTO `json:"config"`
	}
	if err := json.NewDecoder(getRec.Body).Decode(&getResp); err != nil {
		t.Fatal(err)
	}
	if getResp.Config.Server.Name != "test" {
		t.Fatalf("server name = %q", getResp.Config.Server.Name)
	}
	if getResp.Config.WebUI.Token != redactedSecret {
		t.Fatalf("expected redacted webui token, got %q", getResp.Config.WebUI.Token)
	}

	getResp.Config.Server.Name = "updated-server"
	putBody, _ := json.Marshal(getResp.Config)
	putReq := httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(putBody))
	for _, c := range loginRec.Result().Cookies() {
		putReq.AddCookie(c)
	}
	putRec := httptest.NewRecorder()
	s.handler.ServeHTTP(putRec, putReq)
	if putRec.Code != http.StatusOK {
		t.Fatalf("put config = %d body=%s", putRec.Code, putRec.Body.String())
	}

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Server.Name != "updated-server" {
		t.Fatalf("saved name = %q", reloaded.Server.Name)
	}
}

func TestMergeSecret(t *testing.T) {
	if mergeSecret(redactedSecret, "keep-me") != "keep-me" {
		t.Fatal("expected keep existing on redacted")
	}
	if mergeSecret("new", "old") != "new" {
		t.Fatal("expected new value")
	}
}
