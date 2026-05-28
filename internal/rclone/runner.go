package rclone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Origin173/SnapCraft/internal/config"
)

// Runner executes rclone commands.
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

// ExecRunner runs real rclone binaries.
type ExecRunner struct {
	cfg    *config.Config
	lastOut string
}

func NewExecRunner(cfg *config.Config) *ExecRunner {
	return &ExecRunner{cfg: cfg}
}

func (r *ExecRunner) Copy(ctx context.Context, localPath, remotePath string) error {
	args := r.baseArgs("copy", localPath, remotePath)
	return r.run(ctx, args...)
}

func (r *ExecRunner) CopyToLocal(ctx context.Context, remotePath, localPath string) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	args := r.baseArgs("copy", remotePath, localPath)
	return r.run(ctx, args...)
}

func (r *ExecRunner) Sync(ctx context.Context, localPath, remotePath, backupDir string) error {
	args := r.baseArgs("sync", localPath, remotePath)
	if backupDir != "" {
		args = append(args, "--backup-dir", backupDir)
	}
	return r.run(ctx, args...)
}

func (r *ExecRunner) Check(ctx context.Context, localPath, remotePath string) error {
	args := r.baseArgs("check", localPath, remotePath)
	return r.run(ctx, args...)
}

func (r *ExecRunner) ListJSON(ctx context.Context, remotePath string) ([]ListEntry, error) {
	args := r.baseArgs("lsjson", remotePath)
	var out bytes.Buffer
	if err := r.runWithOutput(ctx, &out, args...); err != nil {
		// Empty remote may not exist yet.
		if strings.Contains(out.String(), "directory not found") {
			return nil, nil
		}
		return nil, err
	}
	if strings.TrimSpace(out.String()) == "" {
		return nil, nil
	}
	var entries []ListEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		return nil, fmt.Errorf("parse lsjson: %w", err)
	}
	return entries, nil
}

func (r *ExecRunner) DeleteFile(ctx context.Context, remotePath string) error {
	args := r.baseArgs("deletefile", remotePath)
	return r.run(ctx, args...)
}

func (r *ExecRunner) Purge(ctx context.Context, remotePath string) error {
	args := r.baseArgs("purge", remotePath)
	return r.run(ctx, args...)
}

func (r *ExecRunner) LastOutput() string {
	return r.lastOut
}

func (r *ExecRunner) baseArgs(subcmd string, paths ...string) []string {
	args := []string{subcmd}
	if r.cfg.Rclone.BwLimit != "" {
		args = append(args, "--bwlimit", r.cfg.Rclone.BwLimit)
	}
	if r.cfg.Rclone.Transfers > 0 {
		args = append(args, "--transfers", fmt.Sprintf("%d", r.cfg.Rclone.Transfers))
	}
	if r.cfg.Rclone.Checkers > 0 {
		args = append(args, "--checkers", fmt.Sprintf("%d", r.cfg.Rclone.Checkers))
	}
	if r.cfg.Rclone.Retries > 0 {
		args = append(args, "--retries", fmt.Sprintf("%d", r.cfg.Rclone.Retries))
	}
	args = append(args, r.cfg.Rclone.ExtraArgs...)
	args = append(args, paths...)
	return args
}

func (r *ExecRunner) run(ctx context.Context, args ...string) error {
	var out bytes.Buffer
	return r.runWithOutput(ctx, &out, args...)
}

func (r *ExecRunner) runWithOutput(ctx context.Context, out *bytes.Buffer, args ...string) error {
	if _, err := exec.LookPath(r.cfg.Rclone.Binary); err != nil {
		return fmt.Errorf("rclone not found at %q: %w", r.cfg.Rclone.Binary, err)
	}

	cmdCtx := ctx
	if _, ok := ctx.Deadline(); !ok && r.cfg.Rclone.Timeout > 0 {
		var cancel context.CancelFunc
		cmdCtx, cancel = context.WithTimeout(ctx, r.cfg.Rclone.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(cmdCtx, r.cfg.Rclone.Binary, args...)
	var stderr bytes.Buffer
	cmd.Stdout = out
	cmd.Stderr = &stderr

	err := cmd.Run()
	r.lastOut = out.String()
	if err != nil {
		return fmt.Errorf("rclone %s: %w\nstderr: %s", args[0], err, stderr.String())
	}
	return nil
}

// EnsureAvailable checks rclone is on PATH.
func EnsureAvailable(cfg *config.Config) error {
	_, err := exec.LookPath(cfg.Rclone.Binary)
	if err != nil {
		return fmt.Errorf("rclone binary %q not found: %w", cfg.Rclone.Binary, err)
	}
	return nil
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
