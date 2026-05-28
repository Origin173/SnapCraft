package webui

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRcloneStatusAPIRequiresAuth(t *testing.T) {
	cfg := testConfig(t)
	s, err := NewServer(cfg, "")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/rclone/status", nil)
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestRcloneStatusAPIAuthenticated(t *testing.T) {
	cfg := testConfig(t)
	cfg.Upload.Enabled = true
	cfg.Rclone.Remote = "myremote:crypt"
	cfg.Rclone.RemotePath = "snapcraft/test"

	s, err := NewServer(cfg, "")
	if err != nil {
		t.Fatal(err)
	}

	loginBody, _ := json.Marshal(map[string]string{"token": "secret-token"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	s.handler.ServeHTTP(loginRec, loginReq)

	req := httptest.NewRequest(http.MethodGet, "/api/rclone/status", nil)
	for _, c := range loginRec.Result().Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Upload struct {
			Remote     string `json:"remote"`
			RemoteBase string `json:"remote_base"`
			FullFS     string `json:"full_fs"`
		} `json:"upload"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Upload.RemoteBase != "myremote" || resp.Upload.FullFS != "myremote:crypt:snapcraft/test" {
		t.Fatalf("upload status = %#v", resp.Upload)
	}
}

func TestRcloneTestUploadAPIAuthenticated(t *testing.T) {
	cfg := testConfig(t)
	cfg.Upload.Enabled = false

	s, err := NewServer(cfg, "")
	if err != nil {
		t.Fatal(err)
	}

	loginBody, _ := json.Marshal(map[string]string{"token": "secret-token"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/login", bytes.NewReader(loginBody))
	loginRec := httptest.NewRecorder()
	s.handler.ServeHTTP(loginRec, loginReq)

	req := httptest.NewRequest(http.MethodPost, "/api/rclone/test-upload", bytes.NewReader([]byte("{}")))
	for _, c := range loginRec.Result().Cookies() {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	s.handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Result struct {
			OK bool `json:"ok"`
		} `json:"result"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Result.OK {
		t.Fatal("expected failed test when upload disabled")
	}
}
