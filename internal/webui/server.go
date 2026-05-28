package webui

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Origin173/SnapCraft/internal/config"
)

//go:embed static/*
var staticFS embed.FS

// Server serves the SnapCraft WebUI and JSON API.
type Server struct {
	cfg        *config.Config
	configPath string
	configMu   sync.RWMutex
	auth       *Auth
	jobs       *JobManager
	logs       *LogStore
	handler    http.Handler
}

func NewServer(cfg *config.Config, configPath string) (*Server, error) {
	if err := ValidateStartupToken(cfg.WebUI.Token); err != nil {
		return nil, err
	}
	s := &Server{
		cfg:        cfg,
		configPath: configPath,
		auth:       NewAuth(cfg.WebUI.Token, cfg.WebUI.CookieName),
		jobs:       NewJobManager(),
		logs:       NewLogStore(defaultLogCapacity),
	}
	s.jobs.SetCallbacks(
		func(op string) { s.logInfo("job", op+" 已开始", map[string]any{"operation": op}) },
		func(op, status, msg string) {
			level := "info"
			if status == JobFailed {
				level = "error"
			}
			s.logs.Append(level, "job", msg, map[string]any{"operation": op, "status": status})
		},
	)
	s.handler = s.routes()
	s.logInfo("server", "WebUI 已启动", map[string]any{"addr": cfg.WebUI.Addr})
	return s, nil
}

func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.WebUI.Addr,
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("webui listening", "addr", s.cfg.WebUI.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil {
			return err
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

func Run(cfg *config.Config, configPath string) error {
	s, err := NewServer(cfg, configPath)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return s.Run(ctx)
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/session", s.handleSession)

	mux.Handle("GET /api/status", s.requireAuth(http.HandlerFunc(s.handleStatus)))
	mux.Handle("GET /api/snapshots", s.requireAuth(http.HandlerFunc(s.handleListSnapshots)))
	mux.Handle("GET /api/snapshots/{id}", s.requireAuth(http.HandlerFunc(s.handleGetSnapshot)))
	mux.Handle("POST /api/backup/run", s.requireAuth(http.HandlerFunc(s.handleBackupRun)))
	mux.Handle("GET /api/jobs/current", s.requireAuth(http.HandlerFunc(s.handleJobCurrent)))
	mux.Handle("POST /api/restore", s.requireAuth(http.HandlerFunc(s.handleRestore)))
	mux.Handle("POST /api/repo/init", s.requireAuth(http.HandlerFunc(s.handleRepoInit)))
	mux.Handle("POST /api/repo/verify", s.requireAuth(http.HandlerFunc(s.handleRepoVerify)))
	mux.Handle("GET /api/prune/preview", s.requireAuth(http.HandlerFunc(s.handlePrunePreview)))
	mux.Handle("POST /api/prune/apply", s.requireAuth(http.HandlerFunc(s.handlePruneApply)))
	mux.Handle("GET /api/rclone/remotes", s.requireAuth(http.HandlerFunc(s.handleRcloneList)))
	mux.Handle("GET /api/rclone/remotes/{name}", s.requireAuth(http.HandlerFunc(s.handleRcloneShow)))
	mux.Handle("POST /api/rclone/remotes", s.requireAuth(http.HandlerFunc(s.handleRcloneCreate)))
	mux.Handle("PATCH /api/rclone/remotes/{name}", s.requireAuth(http.HandlerFunc(s.handleRcloneUpdate)))
	mux.Handle("DELETE /api/rclone/remotes/{name}", s.requireAuth(http.HandlerFunc(s.handleRcloneDelete)))
	mux.Handle("GET /api/rclone/providers", s.requireAuth(http.HandlerFunc(s.handleRcloneProviders)))
	mux.Handle("GET /api/rclone/status", s.requireAuth(http.HandlerFunc(s.handleRcloneStatus)))
	mux.Handle("POST /api/rclone/remotes/{name}/test", s.requireAuth(http.HandlerFunc(s.handleRcloneTestRemote)))
	mux.Handle("POST /api/rclone/test-upload", s.requireAuth(http.HandlerFunc(s.handleRcloneTestUpload)))
	mux.Handle("GET /api/config", s.requireAuth(http.HandlerFunc(s.handleGetConfig)))
	mux.Handle("PUT /api/config", s.requireAuth(http.HandlerFunc(s.handlePutConfig)))
	mux.Handle("POST /api/config/validate", s.requireAuth(http.HandlerFunc(s.handleValidateConfig)))
	mux.Handle("GET /api/logs", s.requireAuth(http.HandlerFunc(s.handleListLogs)))
	mux.Handle("DELETE /api/logs", s.requireAuth(http.HandlerFunc(s.handleClearLogs)))

	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(static))
	mux.Handle("GET /{$}", fileServer)
	mux.Handle("GET /styles.css", fileServer)
	mux.Handle("GET /app.js", fileServer)

	return mux
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.Authenticated(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if !s.auth.ValidToken(body.Token) {
		s.logWarn("auth", "登录失败：无效令牌", nil)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
		return
	}
	s.auth.SetSession(w, r, body.Token)
	s.logInfo("auth", "用户已登录", nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s.auth.Authenticated(r) {
		s.logInfo("auth", "用户已退出", nil)
	}
	s.auth.ClearSession(w, r)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"authenticated": s.auth.Authenticated(r)})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func pathParam(r *http.Request, key string) string {
	return r.PathValue(key)
}

func decodeJSON(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

func fmtAddrHint(addr string) string {
	if strings.HasPrefix(addr, "127.0.0.1") || strings.HasPrefix(addr, "localhost") {
		return fmt.Sprintf("http://%s", addr)
	}
	return fmt.Sprintf("http://%s (bind carefully; use reverse proxy + TLS for remote access)", addr)
}
