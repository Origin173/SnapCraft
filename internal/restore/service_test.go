package restore

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Origin173/SnapCraft/internal/archive"
	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/minecraft"
	"github.com/Origin173/SnapCraft/internal/notify"
	"github.com/Origin173/SnapCraft/internal/rclone"
)

func testRestoreConfig(t *testing.T, dir, world string) *config.Config {
	t.Helper()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Name:      "test",
			WorldPath: world,
			Control:   config.ControlConfig{Type: config.ControlRCON},
		},
		Backup: config.BackupConfig{
			StagingDir:  dir,
			Mode:        config.BackupModeArchive,
			Compression: config.CompressionNone,
		},
		Repository: config.RepositoryConfig{LocalPath: filepath.Join(dir, "repo")},
		Upload:     config.UploadConfig{Enabled: false},
	}
	config.ApplyDefaultsForTest(cfg)
	return cfg
}

func TestRestoreRequiresForceOnline(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	os.MkdirAll(world, 0o755)

	cfg := testRestoreConfig(t, dir, world)
	mc := minecraft.NewFakeController()
	svc, err := NewService(cfg, mc, &rclone.FakeRunner{}, nil, notify.NoopNotifier{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	err = svc.Run(context.Background(), "missing-id", Options{ForceOnline: false})
	if err == nil {
		t.Fatal("expected error without --force-online")
	}
}

func TestAtomicReplaceWorld(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	src := filepath.Join(dir, "source")
	os.MkdirAll(world, 0o755)
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(world, "old.dat"), []byte("old"), 0o644)
	os.WriteFile(filepath.Join(src, "new.dat"), []byte("new"), 0o644)

	cfg := &config.Config{
		Server: config.ServerConfig{WorldPath: world},
		Backup: config.BackupConfig{StagingDir: dir},
	}
	svc := &Service{cfg: cfg}

	if err := svc.atomicReplaceWorld(src); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(world, "new.dat")); err != nil {
		t.Error("expected new.dat in world")
	}
}

func TestExtractAndRestoreFlow(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	os.MkdirAll(world, 0o755)
	os.WriteFile(filepath.Join(world, "level.dat"), []byte("original"), 0o644)

	archivePath := filepath.Join(dir, "backup.tar")
	cfg := &config.Config{Backup: config.BackupConfig{Compression: config.CompressionNone}}
	if _, err := archive.Create(cfg, archivePath, []string{world}); err != nil {
		t.Fatal(err)
	}

	os.WriteFile(filepath.Join(world, "level.dat"), []byte("modified"), 0o644)

	extractDir := filepath.Join(dir, "extract")
	if err := archive.Extract(archivePath, extractDir); err != nil {
		t.Fatal(err)
	}
	svc := &Service{cfg: &config.Config{Server: config.ServerConfig{WorldPath: world}}}
	extractedWorld := filepath.Join(extractDir, "world")
	if err := svc.atomicReplaceWorld(extractedWorld); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(filepath.Join(world, "level.dat"))
	if string(data) != "original" {
		t.Errorf("restored = %q, want original", data)
	}
}
