package backup

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/minecraft"
	"github.com/Origin173/SnapCraft/internal/notify"
	"github.com/Origin173/SnapCraft/internal/rclone"
	"github.com/Origin173/SnapCraft/internal/snapshot"
)

func TestDirectoryBackupMode(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	os.MkdirAll(world, 0o755)
	os.WriteFile(filepath.Join(world, "level.dat"), []byte("data"), 0o644)

	cfg := testConfig(t, world, dir)
	cfg.Backup.Mode = config.BackupModeDirectory
	cfg.Upload.Enabled = true

	mc := minecraft.NewFakeController()
	runner := &rclone.FakeRunner{}

	svc, err := NewService(cfg, mc, runner, notify.NoopNotifier{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Manifest.Mode != config.BackupModeDirectory {
		t.Errorf("mode = %q", result.Manifest.Mode)
	}
	if result.Manifest.DirectoryPath == "" {
		t.Error("expected directory_path in manifest")
	}

	foundSync := false
	for _, c := range runner.Calls {
		if len(c) >= 4 && c[:4] == "sync" {
			foundSync = true
		}
	}
	if !foundSync {
		t.Errorf("expected sync call, got %v", runner.Calls)
	}
}

func TestManifestUploadOnSuccess(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	os.MkdirAll(world, 0o755)
	os.WriteFile(filepath.Join(world, "level.dat"), []byte("x"), 0o644)

	cfg := testConfig(t, world, dir)
	mc := minecraft.NewFakeController()
	runner := &rclone.FakeRunner{}

	svc, err := NewService(cfg, mc, runner, notify.NoopNotifier{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Manifest.Status != snapshot.StatusCompleted {
		t.Errorf("status = %q", result.Manifest.Status)
	}
}

func TestIncrementalBackupDedup(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	os.MkdirAll(world, 0o755)
	os.WriteFile(filepath.Join(world, "level.dat"), []byte("data"), 0o644)

	cfg := testConfig(t, world, dir)
	cfg.Backup.Mode = config.BackupModeIncremental
	cfg.Backup.Compression = config.CompressionNone
	cfg.Repository.VerifyAfterBackup = false

	mc := minecraft.NewFakeController()
	svc, err := NewService(cfg, mc, &rclone.FakeRunner{}, notify.NoopNotifier{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	if _, err := svc.Run(context.Background()); err != nil {
		t.Fatalf("first backup: %v", err)
	}
	if _, err := svc.Run(context.Background()); err != nil {
		t.Fatalf("second backup: %v", err)
	}
}

func TestOfflineBackupNoServerControl(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	os.MkdirAll(world, 0o755)
	os.WriteFile(filepath.Join(world, "level.dat"), []byte("sp"), 0o644)

	cfg := testConfig(t, world, dir)
	cfg.Server.Control.Type = config.ControlNone

	mc := minecraft.NewNoopController()
	svc, err := NewService(cfg, mc, &rclone.FakeRunner{}, notify.NoopNotifier{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Manifest.Status != snapshot.StatusCompleted {
		t.Errorf("status = %q", result.Manifest.Status)
	}
}
