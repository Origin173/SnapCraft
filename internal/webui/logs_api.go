package webui

import (
	"net/http"
	"strconv"
)

func (s *Server) handleListLogs(w http.ResponseWriter, r *http.Request) {
	level := r.URL.Query().Get("level")
	source := r.URL.Query().Get("source")
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	entries := s.logs.List(level, source, limit)
	writeJSON(w, http.StatusOK, map[string]any{"logs": entries})
}

func (s *Server) handleClearLogs(w http.ResponseWriter, r *http.Request) {
	s.logs.Clear()
	s.logInfo("api", "日志已清空", nil)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
