package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	if err := os.MkdirAll(world, 0o755); err != nil {
		t.Fatal(err)
	}

	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
server:
  name: test-server
  world_path: ` + world + `
  control:
    type: rcon
    rcon:
      host: 127.0.0.1
      port: 25575
      password: secret
backup:
  mode: archive
  compression: zstd
rclone:
  remote: myremote
  remote_path: snapcraft/test-server
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Server.Name != "test-server" {
		t.Errorf("Server.Name = %q, want test-server", cfg.Server.Name)
	}
	if cfg.Backup.Mode != BackupModeArchive {
		t.Errorf("Backup.Mode = %q, want archive", cfg.Backup.Mode)
	}
}

func TestValidateMissingRemoteWhenUploadEnabled(t *testing.T) {
	cfg := &Config{
		Server: ServerConfig{
			Name:      "test",
			WorldPath: "/tmp/world",
			Control: ControlConfig{
				Type: ControlRCON,
				RCON: RCONConfig{Password: "x", Port: 25575},
			},
		},
		Backup:     BackupConfig{Mode: BackupModeArchive, Compression: CompressionZstd},
		Repository: RepositoryConfig{LocalPath: "/tmp/repo"},
		Upload:     UploadConfig{Enabled: true},
	}
	applyDefaults(cfg)
	if err := Validate(cfg); err == nil {
		t.Fatal("expected validation error for missing rclone.remote when upload enabled")
	}
}

func TestEnvOverrides(t *testing.T) {
	dir := t.TempDir()
	world := filepath.Join(dir, "world")
	os.MkdirAll(world, 0o755)

	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
server:
  name: base
  world_path: ` + world + `
  control:
    type: rcon
    rcon:
      password: secret
rclone:
  remote: base-remote
  remote_path: snapcraft/base
`
	os.WriteFile(cfgPath, []byte(content), 0o644)

	t.Setenv("SNAPCRAFT_SERVER_NAME", "env-server")
	t.Setenv("SNAPCRAFT_RCLONE_REMOTE", "env-remote")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Name != "env-server" {
		t.Errorf("Server.Name = %q, want env-server", cfg.Server.Name)
	}
	if cfg.Rclone.Remote != "env-remote" {
		t.Errorf("Rclone.Remote = %q, want env-remote", cfg.Rclone.Remote)
	}
}
