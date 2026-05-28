package rclone

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Origin173/SnapCraft/internal/config"
)

const redactedPlaceholder = "••••••••"

// RemoteBaseName returns the rclone remote name from a remote spec such as "myremote:crypt".
func RemoteBaseName(remoteSpec string) string {
	name := strings.SplitN(strings.TrimSpace(remoteSpec), ":", 2)[0]
	return strings.TrimSpace(name)
}

// RemoteSubpath returns the path portion of a remote spec such as "crypt" from "myremote:crypt".
func RemoteSubpath(remoteSpec string) string {
	parts := strings.SplitN(strings.TrimSpace(remoteSpec), ":", 2)
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// JoinRemoteSpec builds a remote spec from a base name and optional in-remote subpath.
func JoinRemoteSpec(name, subpath string) string {
	name = strings.TrimSpace(name)
	subpath = strings.TrimSpace(subpath)
	if subpath == "" {
		return name
	}
	return name + ":" + subpath
}

// BuildUploadFS combines configured rclone.remote and rclone.remote_path into one fs string.
func BuildUploadFS(remote, remotePath string) string {
	remote = strings.TrimSpace(remote)
	remotePath = strings.TrimSpace(filepath.ToSlash(remotePath))
	if remotePath == "" {
		if strings.Contains(remote, ":") {
			return remote
		}
		return remote + ":"
	}
	return remote + ":" + remotePath
}

// FilterCreateParameters removes empty and placeholder values before creating a remote.
func FilterCreateParameters(parameters map[string]string) map[string]string {
	out := make(map[string]string, len(parameters))
	for key, value := range parameters {
		value = strings.TrimSpace(value)
		if value == "" || value == redactedPlaceholder {
			continue
		}
		out[key] = value
	}
	return out
}

// NormalizeRemoteParameters applies provider-specific normalizations.
func NormalizeRemoteParameters(parameters map[string]string) map[string]string {
	out := make(map[string]string, len(parameters))
	for key, value := range parameters {
		out[key] = value
	}
	if out["vendor"] == "openlist" {
		out["vendor"] = "other"
	}
	return out
}

// ValidateRemoteParameters checks required fields for a remote type.
func ValidateRemoteParameters(remoteType string, parameters map[string]string, isCreate bool) error {
	if remoteType != "webdav" {
		return nil
	}
	url := strings.TrimSpace(parameters["url"])
	if url == "" {
		if isCreate {
			return fmt.Errorf("webdav url 为必填项（如 http://127.0.0.1:5244/dav/）")
		}
		return fmt.Errorf("webdav url 未配置，请编辑远程并填写 url")
	}
	lower := strings.ToLower(url)
	if !strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return fmt.Errorf("webdav url 必须以 http:// 或 https:// 开头")
	}
	return nil
}

// PrepareRemoteParameters filters, normalizes, and validates remote parameters.
func PrepareRemoteParameters(remoteType string, parameters map[string]string, isCreate bool) (map[string]string, error) {
	params := NormalizeRemoteParameters(FilterCreateParameters(parameters))
	if err := ValidateRemoteParameters(remoteType, params, isCreate); err != nil {
		return nil, err
	}
	return params, nil
}

// RemoteConfiguredURL returns the configured url for a remote, if any.
func RemoteConfiguredURL(name string) (string, error) {
	cfg, err := ShowRemote(name)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(cfg["url"]), nil
}

// UploadRemoteStatus describes whether the configured upload remote is usable.
type UploadRemoteStatus struct {
	Enabled      bool     `json:"enabled"`
	Remote       string   `json:"remote"`
	RemoteBase   string   `json:"remote_base"`
	RemoteSubpath string  `json:"remote_subpath"`
	RemotePath   string   `json:"remote_path"`
	FullFS       string   `json:"full_fs"`
	RemoteExists bool     `json:"remote_exists"`
	Configured   bool     `json:"configured"`
	Available    []string `json:"available,omitempty"`
}

// UploadRemoteStatusFor returns upload remote validation details for cfg.
func UploadRemoteStatusFor(cfg *config.Config) (UploadRemoteStatus, error) {
	status := UploadRemoteStatus{
		Enabled:       cfg.Upload.Enabled,
		Remote:        cfg.Rclone.Remote,
		RemoteBase:    RemoteBaseName(cfg.Rclone.Remote),
		RemoteSubpath: RemoteSubpath(cfg.Rclone.Remote),
		RemotePath:    cfg.Rclone.RemotePath,
		FullFS:        BuildUploadFS(cfg.Rclone.Remote, cfg.Rclone.RemotePath),
	}
	if !cfg.Upload.Enabled {
		return status, nil
	}
	status.Configured = strings.TrimSpace(status.RemoteBase) != "" && strings.TrimSpace(cfg.Rclone.RemotePath) != ""
	if status.RemoteBase == "" {
		return status, nil
	}
	remotes, err := ListRemotes()
	if err != nil {
		return status, err
	}
	status.Available = remotes
	for _, name := range remotes {
		if name == status.RemoteBase {
			status.RemoteExists = true
			break
		}
	}
	return status, nil
}
