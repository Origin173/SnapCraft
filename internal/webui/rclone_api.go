package webui

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/rclone"
)

func (s *Server) handleRcloneStatus(w http.ResponseWriter, r *http.Request) {
	status, err := rclone.UploadRemoteStatusFor(s.getConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	remotes, err := rclone.ListRemotes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	details := make([]map[string]any, 0, len(remotes))
	for _, name := range remotes {
		item := map[string]any{"name": name}
		if cfg, showErr := rclone.ShowRemote(name); showErr == nil {
			item["type"] = cfg["type"]
		}
		details = append(details, item)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"upload":  status,
		"remotes": details,
	})
}

func (s *Server) handleRcloneTestRemote(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	if strings.TrimSpace(name) == "" {
		writeError(w, http.StatusBadRequest, errRemoteNameRequired)
		return
	}

	var body struct {
		Path string `json:"path"`
	}
	_ = decodeJSON(r, &body)

	fs := rclone.JoinRemoteSpec(name, body.Path)
	ctx, cancel := context.WithTimeout(r.Context(), testTimeout(s.getConfig()))
	defer cancel()

	result := rclone.TestList(ctx, s.getConfig(), fs)
	if result.OK {
		s.logInfo("rclone", "远程连接测试成功", map[string]any{"remote": name, "path": fs, "items": result.ItemCount})
	} else {
		s.logError("rclone", "远程连接测试失败", map[string]any{"remote": name, "path": fs, "error": result.Message})
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func (s *Server) handleRcloneTestUpload(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), testTimeout(s.getConfig()))
	defer cancel()

	result, err := rclone.TestUploadPath(ctx, s.getConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if result.OK {
		s.logInfo("rclone", "上传路径测试成功", map[string]any{"path": result.Path, "items": result.ItemCount})
	} else {
		s.logError("rclone", "上传路径测试失败", map[string]any{"path": result.Path, "error": result.Message})
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

var errRemoteNameRequired = errorString("remote name is required")

type errorString string

func (e errorString) Error() string { return string(e) }

func testTimeout(cfg *config.Config) time.Duration {
	if cfg.Rclone.Timeout > 0 {
		return cfg.Rclone.Timeout
	}
	return 30 * time.Second
}
