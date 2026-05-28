package archive

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Origin173/SnapCraft/internal/config"
)

func TestCreateAndExtract(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "world")
	os.MkdirAll(src, 0o755)
	os.WriteFile(filepath.Join(src, "level.dat"), []byte("test-world-data"), 0o644)

	cfg := &config.Config{
		Backup: config.BackupConfig{Compression: config.CompressionNone},
	}
	archivePath := filepath.Join(dir, "backup.tar")
	result, err := Create(cfg, archivePath, []string{src})
	if err != nil {
		t.Fatal(err)
	}
	if result.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1", result.FileCount)
	}

	extractDir := filepath.Join(dir, "restored")
	if err := Extract(archivePath, extractDir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(extractDir, "world", "level.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "test-world-data" {
		t.Errorf("restored data = %q", data)
	}
}

func TestArchiveExtension(t *testing.T) {
	tests := []struct {
		comp string
		want string
	}{
		{config.CompressionZstd, ".tar.zst"},
		{config.CompressionGzip, ".tar.gz"},
		{config.CompressionNone, ".tar"},
	}
	for _, tt := range tests {
		cfg := &config.Config{Backup: config.BackupConfig{Compression: tt.comp}}
		if got := ArchiveExtension(cfg); got != tt.want {
			t.Errorf("ArchiveExtension(%s) = %q, want %q", tt.comp, got, tt.want)
		}
	}
}
