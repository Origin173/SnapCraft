package rclone

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/rclone/rclone/fs/fspath"
)

// Runner executes rclone operations via the embedded library.
type Runner interface {
	Copy(ctx context.Context, localPath, remotePath string) error
	CopyToLocal(ctx context.Context, remotePath, localPath string) error
	Sync(ctx context.Context, localPath, remotePath string, backupDir string) error
	Check(ctx context.Context, localPath, remotePath string) error
	ListJSON(ctx context.Context, remotePath string) ([]ListEntry, error)
	DeleteFile(ctx context.Context, remotePath string) error
	Purge(ctx context.Context, remotePath string) error
	LastOutput() string
}

// ListEntry represents an rclone lsjson entry.
type ListEntry struct {
	Name  string `json:"Name"`
	Path  string `json:"Path"`
	Size  int64  `json:"Size"`
	IsDir bool   `json:"IsDir"`
}

// EmbeddedRunner runs rclone through the embedded librclone RPC layer.
type EmbeddedRunner struct {
	cfg    *config.Config
	rpc    rpcCaller
	lastOut string
}

func NewEmbeddedRunner(cfg *config.Config) *EmbeddedRunner {
	return &EmbeddedRunner{cfg: cfg, rpc: librcloneCaller{}}
}

// NewRunner returns the default embedded rclone runner.
func NewRunner(cfg *config.Config) Runner {
	return NewEmbeddedRunner(cfg)
}

func (r *EmbeddedRunner) Copy(ctx context.Context, localPath, remotePath string) error {
	return r.copyPaths(ctx, localPath, remotePath)
}

func (r *EmbeddedRunner) CopyToLocal(ctx context.Context, remotePath, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	return r.copyPaths(ctx, remotePath, localPath)
}

func (r *EmbeddedRunner) Sync(ctx context.Context, localPath, remotePath, backupDir string) error {
	params := map[string]any{
		"srcFs": toFsString(localPath),
		"dstFs": toFsString(remotePath),
	}
	if cfg := r.transferConfig(backupDir); len(cfg) > 0 {
		params["_config"] = cfg
	}
	out, err := r.rpc.call("sync/sync", params)
	r.storeOutput(out, err)
	return err
}

func (r *EmbeddedRunner) Check(ctx context.Context, localPath, remotePath string) error {
	params := map[string]any{
		"srcFs": toFsString(localPath),
		"dstFs": toFsString(remotePath),
	}
	if cfg := r.transferConfig(""); len(cfg) > 0 {
		params["_config"] = cfg
	}
	out, err := r.rpc.call("operations/check", params)
	r.storeOutput(out, err)
	if err != nil {
		return err
	}
	if success, ok := out["success"].(bool); ok && !success {
		status, _ := out["status"].(string)
		if status == "" {
			status = "files differ"
		}
		return fmt.Errorf("rclone check: %s", status)
	}
	return nil
}

func (r *EmbeddedRunner) ListJSON(ctx context.Context, remotePath string) ([]ListEntry, error) {
	fsName, remote, err := splitRemotePath(remotePath)
	if err != nil {
		return nil, err
	}
	params := map[string]any{
		"fs":     fsName,
		"remote": remote,
	}
	out, err := r.rpc.call("operations/list", params)
	r.storeOutput(out, err)
	if err != nil {
		if strings.Contains(err.Error(), "directory not found") {
			return nil, nil
		}
		return nil, err
	}

	rawList, ok := out["list"].([]any)
	if !ok || len(rawList) == 0 {
		return nil, nil
	}

	data, err := json.Marshal(rawList)
	if err != nil {
		return nil, fmt.Errorf("marshal list entries: %w", err)
	}
	var entries []ListEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("parse list entries: %w", err)
	}
	return entries, nil
}

func (r *EmbeddedRunner) DeleteFile(ctx context.Context, remotePath string) error {
	fsName, remote, err := splitRemotePath(remotePath)
	if err != nil {
		return err
	}
	params := map[string]any{
		"fs":     fsName,
		"remote": remote,
	}
	out, err := r.rpc.call("operations/deletefile", params)
	r.storeOutput(out, err)
	return err
}

func (r *EmbeddedRunner) Purge(ctx context.Context, remotePath string) error {
	fsName, remote, err := splitRemotePath(remotePath)
	if err != nil {
		return err
	}
	params := map[string]any{
		"fs":     fsName,
		"remote": remote,
	}
	out, err := r.rpc.call("operations/purge", params)
	r.storeOutput(out, err)
	return err
}

func (r *EmbeddedRunner) LastOutput() string {
	return r.lastOut
}

func (r *EmbeddedRunner) copyPaths(ctx context.Context, srcPath, dstPath string) error {
	params := map[string]any{
		"srcFs": toFsString(srcPath),
		"dstFs": toFsString(dstPath),
	}
	if cfg := r.transferConfig(""); len(cfg) > 0 {
		params["_config"] = cfg
	}
	out, err := r.rpc.call("sync/copy", params)
	r.storeOutput(out, err)
	return err
}

func (r *EmbeddedRunner) transferConfig(backupDir string) map[string]any {
	cfg := map[string]any{}
	if r.cfg.Rclone.BwLimit != "" {
		cfg["BwLimit"] = r.cfg.Rclone.BwLimit
	}
	if r.cfg.Rclone.Transfers > 0 {
		cfg["Transfers"] = r.cfg.Rclone.Transfers
	}
	if r.cfg.Rclone.Checkers > 0 {
		cfg["Checkers"] = r.cfg.Rclone.Checkers
	}
	if r.cfg.Rclone.Retries > 0 {
		cfg["LowLevelRetries"] = r.cfg.Rclone.Retries
	}
	if r.cfg.Rclone.Timeout > 0 {
		cfg["Timeout"] = r.cfg.Rclone.Timeout.String()
	}
	if backupDir != "" {
		cfg["BackupDir"] = toFsString(backupDir)
	}
	return cfg
}

func (r *EmbeddedRunner) storeOutput(out map[string]any, err error) {
	if err != nil {
		r.lastOut = err.Error()
		return
	}
	if out == nil {
		r.lastOut = ""
		return
	}
	data, marshalErr := json.Marshal(out)
	if marshalErr != nil {
		r.lastOut = fmt.Sprintf("%v", out)
		return
	}
	r.lastOut = string(data)
}

func toFsString(path string) string {
	path = filepath.ToSlash(path)
	if filepath.IsAbs(path) || strings.HasPrefix(path, "/") {
		return path
	}
	return path
}

func splitRemotePath(path string) (fsName, remote string, err error) {
	name, remotePath, err := fspath.SplitFs(filepath.ToSlash(path))
	if err != nil {
		return "", "", err
	}
	if name == "" {
		return "", remotePath, fmt.Errorf("path %q is not a remote path", path)
	}
	return name, remotePath, nil
}

// EnsureRemoteConfigured verifies the configured remote exists when upload is enabled.
func EnsureRemoteConfigured(cfg *config.Config) error {
	if !cfg.Upload.Enabled {
		return nil
	}
	remoteName := strings.SplitN(cfg.Rclone.Remote, ":", 2)[0]
	if strings.TrimSpace(remoteName) == "" {
		return fmt.Errorf("rclone remote is not configured")
	}
	remotes, err := ListRemotes()
	if err != nil {
		return err
	}
	for _, name := range remotes {
		if name == remoteName {
			return nil
		}
	}
	return fmt.Errorf("rclone remote %q not found; configure it with: snapcraft rclone create %s <type> key=value...", remoteName, remoteName)
}

// FakeRunner is a test double recording calls.
type FakeRunner struct {
	Calls    []string
	ListData map[string][]ListEntry
	FailNext bool
	lastOut  string
}

func (f *FakeRunner) Copy(ctx context.Context, localPath, remotePath string) error {
	f.Calls = append(f.Calls, fmt.Sprintf("copy %s %s", localPath, remotePath))
	if f.FailNext {
		f.FailNext = false
		return fmt.Errorf("fake rclone error")
	}
	return nil
}

func (f *FakeRunner) CopyToLocal(ctx context.Context, remotePath, localPath string) error {
	f.Calls = append(f.Calls, fmt.Sprintf("copy %s %s", remotePath, localPath))
	return nil
}

func (f *FakeRunner) Sync(ctx context.Context, localPath, remotePath, backupDir string) error {
	f.Calls = append(f.Calls, fmt.Sprintf("sync %s %s backup-dir=%s", localPath, remotePath, backupDir))
	return nil
}

func (f *FakeRunner) Check(ctx context.Context, localPath, remotePath string) error {
	f.Calls = append(f.Calls, fmt.Sprintf("check %s %s", localPath, remotePath))
	return nil
}

func (f *FakeRunner) ListJSON(ctx context.Context, remotePath string) ([]ListEntry, error) {
	if f.ListData != nil {
		if entries, ok := f.ListData[remotePath]; ok {
			return entries, nil
		}
	}
	return nil, nil
}

func (f *FakeRunner) DeleteFile(ctx context.Context, remotePath string) error {
	f.Calls = append(f.Calls, "deletefile "+remotePath)
	return nil
}

func (f *FakeRunner) Purge(ctx context.Context, remotePath string) error {
	f.Calls = append(f.Calls, "purge "+remotePath)
	return nil
}

func (f *FakeRunner) LastOutput() string {
	return f.lastOut
}
