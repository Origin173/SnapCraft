package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// normalizeBackupPaths rewrites common Unix temp paths to OS-appropriate locations.
func normalizeBackupPaths(cfg *Config) {
	cfg.Backup.StagingDir = normalizeTempPath(cfg.Backup.StagingDir, "snapcraft")
	cfg.Backup.LockFile = normalizeLockFilePath(cfg.Backup.LockFile, cfg.Backup.StagingDir)
}

func normalizeTempPath(path, name string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return filepath.Join(os.TempDir(), name)
	}
	if rewritten := rewriteUnixTempPath(path, name); rewritten != path {
		return rewritten
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func normalizeLockFilePath(path, stagingDir string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return filepath.Join(stagingDir, "snapcraft.lock")
	}
	if rewritten := rewriteUnixTempPath(path, "snapcraft.lock"); rewritten != path {
		return rewritten
	}
	if !filepath.IsAbs(path) {
		return filepath.Join(stagingDir, path)
	}
	return path
}

// EnsureBackupDirs creates runtime directories required for backup operations.
func EnsureBackupDirs(cfg *Config) error {
	if err := os.MkdirAll(cfg.Backup.StagingDir, 0o755); err != nil {
		return fmt.Errorf("backup.staging_dir: %w", err)
	}
	lockDir := filepath.Dir(cfg.Backup.LockFile)
	if lockDir != "" && lockDir != "." {
		if err := os.MkdirAll(lockDir, 0o755); err != nil {
			return fmt.Errorf("backup.lock_file: %w", err)
		}
	}
	return nil
}

func rewriteUnixTempPath(path, fallbackName string) string {
	clean := filepath.ToSlash(strings.TrimSpace(path))
	switch clean {
	case "/tmp/snapcraft":
		return filepath.Join(os.TempDir(), "snapcraft")
	case "/tmp/snapcraft.lock":
		return filepath.Join(os.TempDir(), "snapcraft.lock")
	}
	if strings.HasPrefix(clean, "/tmp/snapcraft/") {
		suffix := strings.TrimPrefix(clean, "/tmp/snapcraft")
		return filepath.Join(os.TempDir(), "snapcraft"+filepath.FromSlash(suffix))
	}
	if runtime.GOOS == "windows" && strings.HasPrefix(clean, "/") && !strings.HasPrefix(clean, "//") {
		trimmed := strings.TrimPrefix(clean, "/")
		return filepath.Join(os.TempDir(), filepath.FromSlash(trimmed))
	}
	return path
}
