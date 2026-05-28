package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Origin173/SnapCraft/internal/config"
)

func testConfig(t *testing.T) *config.Config {
	return &config.Config{
		Server: config.ServerConfig{
			Name:      "test",
			WorldPath: "/tmp/world",
			Control:   config.ControlConfig{Type: config.ControlNone},
		},
		Backup:     config.BackupConfig{Mode: config.BackupModeIncremental, Compression: config.CompressionZstd},
		Repository: config.RepositoryConfig{LocalPath: t.TempDir()},
		WebUI: config.WebUIConfig{
			Token:      "secret-token",
			CookieName: "snapcraft_webui",
			Addr:       "127.0.0.1:7824",
		},
	}
}

func TestAuthValidToken(t *testing.T) {
	a := NewAuth("secret-token", "snapcraft_webui")
	if !a.ValidToken("secret-token") {
		t.Fatal("expected valid token")
	}
	if a.ValidToken("wrong") {
		t.Fatal("expected invalid token")
	}
}

func TestValidateStartupToken(t *testing.T) {
	if err := ValidateStartupToken(""); err == nil {
		t.Fatal("expected error for empty token")
	}
}

func TestLoginAndSession(t *testing.T) {
	cfg := testConfig(t)
	s, err := NewServer(cfg, "")
	if err != nil {
		t.Fatal(err)
	}

	loginBody, _ := json.Marshal(map[string]string{"token": "secret-token"})
	req := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d", rec.Code)
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	for _, c := range rec.Result().Cookies() {
		sessionReq.AddCookie(c)
	}
	sessionRec := httptest.NewRecorder()
	s.handler.ServeHTTP(sessionRec, sessionReq)
	if sessionRec.Code != http.StatusOK {
		t.Fatalf("session status = %d", sessionRec.Code)
	}
}

func TestProtectedRouteRequiresAuth(t *testing.T) {
	cfg := testConfig(t)
	s, err := NewServer(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestLogsAPIRequiresAuth(t *testing.T) {
	cfg := testConfig(t)
	s, err := NewServer(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("logs status = %d", rec.Code)
	}
}

func TestLogsAPIAuthenticated(t *testing.T) {
	cfg := testConfig(t)
	s, err := NewServer(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	s.logInfo("api", "test event", nil)

	loginBody, _ := json.Marshal(map[string]string{"token": "secret-token"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	s.handler.ServeHTTP(loginRec, loginReq)

	req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
	for _, c := range loginRec.Result().Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logs status = %d", rec.Code)
	}
	var resp struct {
		Logs []LogEntry `json:"logs"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Logs) == 0 {
		t.Fatal("expected logs")
	}
}

func TestRedactMap(t *testing.T) {
	out := RedactMap(map[string]string{"user": "alice", "pass": "secret"})
	if out["pass"] != "••••••••" {
		t.Fatalf("pass = %q", out["pass"])
	}
}
