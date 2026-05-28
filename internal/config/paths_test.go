package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRewriteUnixTempPath(t *testing.T) {
	got := rewriteUnixTempPath("/tmp/snapcraft", "snapcraft")
	want := filepath.Join(os.TempDir(), "snapcraft")
	if got != want {
		t.Fatalf("staging = %q, want %q", got, want)
	}

	got = rewriteUnixTempPath("/tmp/snapcraft.lock", "snapcraft.lock")
	want = filepath.Join(os.TempDir(), "snapcraft.lock")
	if got != want {
		t.Fatalf("lock = %q, want %q", got, want)
	}
}

func TestNormalizeBackupPathsUnixDefaults(t *testing.T) {
	cfg := &Config{
		Backup: BackupConfig{
			StagingDir: "/tmp/snapcraft",
			LockFile:   "/tmp/snapcraft.lock",
		},
	}
	normalizeBackupPaths(cfg)
	if !strings.HasPrefix(cfg.Backup.StagingDir, os.TempDir()) {
		t.Fatalf("staging = %q", cfg.Backup.StagingDir)
	}
	if !strings.HasPrefix(cfg.Backup.LockFile, os.TempDir()) {
		t.Fatalf("lock = %q", cfg.Backup.LockFile)
	}
	if runtime.GOOS == "windows" {
		if strings.Contains(cfg.Backup.StagingDir, "/tmp/") {
			t.Fatalf("staging still unix path on windows: %q", cfg.Backup.StagingDir)
		}
	}
}

func TestEnsureBackupDirsCreatesStaging(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "snapcraft")
	cfg := &Config{
		Backup: BackupConfig{
			StagingDir: dir,
			LockFile:   filepath.Join(dir, "snapcraft.lock"),
		},
	}
	if err := EnsureBackupDirs(cfg); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("staging dir not created: %v", err)
	}
}

func TestNormalizeBackupPathsEmptyUsesTemp(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)
	if cfg.Backup.StagingDir == "" {
		t.Fatal("expected staging dir")
	}
	if !filepath.IsAbs(cfg.Backup.StagingDir) {
		t.Fatalf("staging should be absolute: %q", cfg.Backup.StagingDir)
	}
}
