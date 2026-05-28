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

func TestApplyControlProfile(t *testing.T) {
	cfg := &config.Config{Server: config.ServerConfig{Control: config.ControlConfig{Type: config.ControlRCON}}}
	if err := ApplyControlProfile(cfg, "singleplayer"); err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Control.Type != config.ControlNone {
		t.Fatalf("type = %q", cfg.Server.Control.Type)
	}

	cfg.Server.Control.Type = config.ControlNone
	if err := ApplyControlProfile(cfg, "server"); err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Control.Type != config.ControlRCON {
		t.Fatalf("type = %q", cfg.Server.Control.Type)
	}
}

func TestControlModeAPI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := testConfig(t)
	cfg.Server.Control.Type = config.ControlRCON
	if err := config.Save(path, cfg); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(cfg, path)
	if err != nil {
		t.Fatal(err)
	}

	loginBody, _ := json.Marshal(map[string]string{"token": "secret-token"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	s.handler.ServeHTTP(loginRec, loginReq)

	body, _ := json.Marshal(map[string]string{"profile": "singleplayer"})
	req := httptest.NewRequest(http.MethodPost, "/api/config/control-mode", bytes.NewReader(body))
	for _, c := range loginRec.Result().Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Server.Control.Type != config.ControlNone {
		t.Fatalf("saved type = %q", reloaded.Server.Control.Type)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) == 0 {
		t.Fatal("config file empty")
	}
}
