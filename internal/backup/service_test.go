package backup

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/minecraft"
	"github.com/Origin173/SnapCraft/internal/notify"
	"github.com/Origin173/SnapCraft/internal/rclone"
)

func testConfig(t *testing.T, worldPath, staging string) *config.Config {
	t.Helper()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:      "test",
			WorldPath: worldPath,
			Control:   config.ControlConfig{Type: config.ControlRCON},
		},
		Backup: config.BackupConfig{
			Mode:        config.BackupModeArchive,
			Compression: config.CompressionNone,
			StagingDir:  staging,
			LockFile:    filepath.Join(staging, "test.lock"),
		},
		Repository: config.RepositoryConfig{
			LocalPath: filepath.Join(staging, "repo"),
		},
		Upload: config.UploadConfig{Enabled: false},
		Rclone: config.RcloneConfig{
			Remote:     "fake",
			RemotePath: "snapcraft/test",
		},
	}
	config.ApplyDefaultsForTest(cfg)
	return cfg
}

func TestBackupSaveOnOnFailure(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	os.MkdirAll(world, 0o755)
	os.WriteFile(filepath.Join(world, "level.dat"), []byte("data"), 0o644)

	cfg := testConfig(t, world, dir)
	mc := minecraft.NewFakeController()
	mc.SetFail("save-all flush", errors.New("save-all failed"))

	svc, err := NewService(cfg, mc, &rclone.FakeRunner{}, notify.NoopNotifier{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	_, err = svc.Run(context.Background())
	if err == nil {
		t.Fatal("expected backup error")
	}

	calls := mc.CommandSequence()
	hasSaveOff := false
	hasSaveOn := false
	for _, c := range calls {
		if c == "save-off" {
			hasSaveOff = true
		}
		if c == "save-on" {
			hasSaveOn = true
		}
	}
	if !hasSaveOff {
		t.Error("expected save-off to be called")
	}
	if !hasSaveOn {
		t.Error("expected save-on to be called on failure path")
	}
}

func TestBackupLocalSuccessWhenUploadFails(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	os.MkdirAll(world, 0o755)
	os.WriteFile(filepath.Join(world, "level.dat"), []byte("data"), 0o644)

	cfg := testConfig(t, world, dir)
	cfg.Upload.Enabled = true
	mc := minecraft.NewFakeController()
	runner := &rclone.FakeRunner{FailNext: true}

	svc, err := NewService(cfg, mc, runner, notify.NoopNotifier{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	result, err := svc.Run(context.Background())
	if err != nil {
		t.Fatalf("local backup should succeed: %v", err)
	}
	if result.Manifest.Error == "" {
		t.Error("expected remote sync error on manifest")
	}
}

func TestBackupOrder(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	os.MkdirAll(world, 0o755)
	os.WriteFile(filepath.Join(world, "level.dat"), []byte("data"), 0o644)

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
		t.Fatalf("Run() error = %v", err)
	}
	if result.Manifest.Status != "completed" {
		t.Errorf("status = %q, want completed", result.Manifest.Status)
	}

	wantPrefix := []string{"save-off", "save-all flush", "save-on"}
	calls := mc.CommandSequence()
	for i, w := range wantPrefix {
		found := false
		for j, c := range calls {
			if c == w && j >= i {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %q in calls, got %v", w, calls)
		}
	}
}

func TestBackupSaveOffFailure(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	os.MkdirAll(world, 0o755)

	cfg := testConfig(t, world, dir)
	mc := minecraft.NewFakeController()
	mc.SetFail("save-off", errors.New("rcon down"))

	svc, err := NewService(cfg, mc, &rclone.FakeRunner{}, notify.NoopNotifier{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	_, err = svc.Run(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	for _, c := range mc.CommandSequence() {
		if c == "save-on" {
			t.Error("save-on should not be called if save-off failed")
		}
	}
}
