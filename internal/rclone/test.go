package rclone

import (
	"context"
	"fmt"
	"strings"

	"github.com/Origin173/SnapCraft/internal/config"
)

// TestResult describes the outcome of a remote connectivity test.
type TestResult struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message"`
	Hint      string `json:"hint,omitempty"`
	ItemCount int    `json:"item_count"`
	Path      string `json:"path"`
}

// TestList verifies that rclone can list the given remote path.
func TestList(ctx context.Context, cfg *config.Config, fs string) TestResult {
	fs = normalizeRemoteFS(fs)
	runner := NewEmbeddedRunner(cfg)
	entries, err := runner.ListJSON(ctx, fs)
	if err != nil {
		msg, hint := humanizeTestError(err.Error())
		return TestResult{
			OK:      false,
			Message: msg,
			Hint:    hint,
			Path:    fs,
		}
	}
	result := TestResult{
		OK:        true,
		ItemCount: len(entries),
		Path:      fs,
	}
	if len(entries) == 0 {
		result.Message = "连接成功，目标路径为空或尚不存在（上传时会自动创建）"
		return result
	}
	dirs, files := 0, 0
	for _, entry := range entries {
		if entry.IsDir {
			dirs++
		} else {
			files++
		}
	}
	result.Message = fmt.Sprintf("连接成功，列出 %d 项（%d 个文件夹，%d 个文件）", len(entries), dirs, files)
	return result
}

// TestUploadPath verifies the configured upload destination.
func TestUploadPath(ctx context.Context, cfg *config.Config) (TestResult, error) {
	status, err := UploadRemoteStatusFor(cfg)
	if err != nil {
		return TestResult{}, err
	}
	if !cfg.Upload.Enabled {
		return TestResult{OK: false, Message: "远程上传未启用", Path: status.FullFS}, nil
	}
	if status.RemoteBase == "" {
		return TestResult{OK: false, Message: "未配置 rclone.remote", Path: status.FullFS}, nil
	}
	if !status.RemoteExists {
		msg := fmt.Sprintf("远程 %q 不存在", status.RemoteBase)
		if len(status.Available) > 0 {
			msg += fmt.Sprintf("；已配置远程：%s", strings.Join(status.Available, ", "))
		}
		return TestResult{OK: false, Message: msg, Path: status.FullFS}, nil
	}
	if strings.TrimSpace(cfg.Rclone.RemotePath) == "" {
		return TestResult{OK: false, Message: "未配置 rclone.remote_path", Path: status.FullFS}, nil
	}
	return TestList(ctx, cfg, status.FullFS), nil
}

func normalizeRemoteFS(fs string) string {
	fs = strings.TrimSpace(fs)
	if fs == "" {
		return fs
	}
	if strings.Contains(fs, ":") {
		return fs
	}
	return fs + ":"
}

func humanizeTestError(raw string) (message, hint string) {
	lower := strings.ToLower(raw)
	switch {
	case strings.Contains(lower, "remote url looks incorrect") && strings.Contains(lower, "dav/files"):
		return "WebDAV vendor 与地址不匹配",
			"若使用 OpenList：vendor 选 other，url 填 http://地址:端口/dav/（如 http://127.0.0.1:5244/dav/），" +
				"并在 OpenList 用户权限中开启 Webdav Read 与 Webdav Manage。" +
				"若使用 Nextcloud：vendor 选 nextcloud，url 用 https://域名/remote.php/dav/files/用户名/。"
	case strings.Contains(lower, "401") || strings.Contains(lower, "unauthorized"):
		return "认证失败：用户名或密码错误", "请检查 user、pass 是否正确；Nextcloud 建议使用应用专用密码。"
	case strings.Contains(lower, "403") || strings.Contains(lower, "forbidden"):
		return "访问被拒绝", "请确认账户有 WebDAV 权限，且 url 指向该用户自己的文件目录。"
	case strings.Contains(lower, "connection refused") || strings.Contains(lower, "no such host"):
		return "无法连接到服务器", "请检查 url 域名是否正确，以及服务器是否可从本机访问。"
	case strings.Contains(lower, "certificate") || strings.Contains(lower, "x509"):
		return "TLS 证书校验失败", "若使用自签名证书，可在高级参数中添加 no_check_certificate=true。"
	default:
		if idx := strings.Index(raw, "rclone rpc "); idx >= 0 {
			raw = raw[idx+len("rclone rpc "):]
		}
		if idx := strings.Index(raw, ": "); idx >= 0 {
			return raw[idx+2:], ""
		}
		return raw, ""
	}
}
