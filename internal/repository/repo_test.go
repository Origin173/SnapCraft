package repository

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/repository/chunker"
)

func testRepoConfig(t *testing.T, dir string) *config.Config {
	t.Helper()
	cfg := &config.Config{
		Server: config.ServerConfig{Name: "test", WorldPath: filepath.Join(dir, "world")},
		Backup: config.BackupConfig{
			Mode:        config.BackupModeIncremental,
			Compression: config.CompressionNone,
			HashMethod:  config.HashBlake3,
			StagingDir:  dir,
			CDC: config.CDCConfig{
				Enabled:     true,
				MinSize:     64,
				AvgSize:     256,
				MaxSize:     1024,
				MinFileSize: 512,
			},
		},
		Repository: config.RepositoryConfig{
			LocalPath:         filepath.Join(dir, "repo"),
			VerifyAfterBackup: false,
		},
		Upload: config.UploadConfig{Enabled: false},
	}
	config.ApplyDefaultsForTest(cfg)
	return cfg
}

func TestRepoInitAndIncrementalBackup(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	os.MkdirAll(world, 0o755)
	os.WriteFile(filepath.Join(world, "level.dat"), []byte("hello world"), 0o644)

	cfg := testRepoConfig(t, dir)
	repo, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	id := "test-snapshot-1"
	if _, err := repo.CreateSnapshot(id, config.BackupModeIncremental); err != nil {
		t.Fatal(err)
	}
	fileCount, totalBytes, err := repo.ScanAndStoreIncremental(context.Background(), id, world)
	if err != nil {
		t.Fatal(err)
	}
	if fileCount != 1 {
		t.Errorf("fileCount = %d, want 1", fileCount)
	}
	if totalBytes == 0 {
		t.Error("expected totalBytes > 0")
	}

	dest := filepath.Join(dir, "restored")
	if err := repo.RestoreIncremental(id, dest); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dest, "level.dat"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("restored = %q", data)
	}
}

func TestChunkerSplit(t *testing.T) {
	data := make([]byte, 2048)
	for i := range data {
		data[i] = byte(i % 251)
	}
	chunks := chunker.SplitBytes(data, chunker.Config{MinSize: 64, AvgSize: 256, MaxSize: 1024}, "blake3")
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
}

func TestVerifyArchive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.dat")
	os.WriteFile(path, []byte("abc"), 0o644)
	cfg := testRepoConfig(t, dir)
	repo, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	hasher := NewHasher(cfg.Backup.HashMethod)
	hash, size, err := hasher.SumFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.VerifyArchive(path, hash, size); err != nil {
		t.Fatal(err)
	}
}
