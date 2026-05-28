package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	BackupModeArchive     = "archive"
	BackupModeDirectory   = "directory"
	BackupModeIncremental = "incremental"

	CompressionZstd = "zstd"
	CompressionGzip = "gzip"
	CompressionNone = "none"

	ControlRCON    = "rcon"
	ControlConsole = "console"
	ControlNone    = "none"

	HashBlake3 = "blake3"
	HashSHA256 = "sha256"
)

// Config holds all SnapCraft settings.
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Backup     BackupConfig     `yaml:"backup"`
	Repository RepositoryConfig `yaml:"repository"`
	Upload     UploadConfig     `yaml:"upload"`
	Rclone     RcloneConfig     `yaml:"rclone"`
	Retention  RetentionConfig  `yaml:"retention"`
	Schedule   ScheduleConfig   `yaml:"schedule"`
	Notify     NotifyConfig     `yaml:"notify"`
	Log        LogConfig        `yaml:"log"`
}

type ServerConfig struct {
	Name      string        `yaml:"name"`
	WorldPath string        `yaml:"world_path"`
	Control   ControlConfig `yaml:"control"`
}

type ControlConfig struct {
	Type    string        `yaml:"type"`
	RCON    RCONConfig    `yaml:"rcon"`
	Console ConsoleConfig `yaml:"console"`
}

type RCONConfig struct {
	Host     string        `yaml:"host"`
	Port     int           `yaml:"port"`
	Password string        `yaml:"password"`
	Timeout  time.Duration `yaml:"timeout"`
}

type ConsoleConfig struct {
	InputPath  string `yaml:"input_path"`
	OutputPath string `yaml:"output_path"`
}

type BackupConfig struct {
	Mode              string        `yaml:"mode"`
	Compression       string        `yaml:"compression"`
	HashMethod        string        `yaml:"hash_method"`
	StagingDir        string        `yaml:"staging_dir"`
	LockFile          string        `yaml:"lock_file"`
	SafetyBackupLocal bool          `yaml:"safety_backup_local"`
	ExcludePatterns   []string      `yaml:"exclude_patterns"`
	Archive           ArchiveConfig `yaml:"archive"`
	CDC               CDCConfig     `yaml:"cdc"`
}

type ArchiveConfig struct {
	IncludePaths []string `yaml:"include_paths"`
}

type CDCConfig struct {
	Enabled     bool  `yaml:"enabled"`
	MinSize     int64 `yaml:"min_size"`
	AvgSize     int64 `yaml:"avg_size"`
	MaxSize     int64 `yaml:"max_size"`
	MinFileSize int64 `yaml:"min_file_size"`
}

type RepositoryConfig struct {
	LocalPath                  string `yaml:"local_path"`
	CleanupAfterVerifiedUpload bool   `yaml:"cleanup_after_verified_upload"`
	KeepLocalManifests         bool   `yaml:"keep_local_manifests"`
	VerifyAfterBackup          bool   `yaml:"verify_after_backup"`
	VerifyAfterUpload          bool   `yaml:"verify_after_upload"`
}

type UploadConfig struct {
	Enabled bool `yaml:"enabled"`
}

type RcloneConfig struct {
	Remote     string        `yaml:"remote"`
	RemotePath string        `yaml:"remote_path"`
	BwLimit    string        `yaml:"bwlimit"`
	Transfers  int           `yaml:"transfers"`
	Checkers   int           `yaml:"checkers"`
	Timeout    time.Duration `yaml:"timeout"`
	Retries    int           `yaml:"retries"`
	ExtraArgs  []string      `yaml:"extra_args"`
}

type RetentionConfig struct {
	Daily   int `yaml:"daily"`
	Weekly  int `yaml:"weekly"`
	Monthly int `yaml:"monthly"`
}

type ScheduleConfig struct {
	Enabled bool   `yaml:"enabled"`
	Cron    string `yaml:"cron"`
}

type NotifyConfig struct {
	Webhook WebhookConfig `yaml:"webhook"`
	Email   EmailConfig   `yaml:"email"`
}

type WebhookConfig struct {
	Enabled bool   `yaml:"enabled"`
	URL     string `yaml:"url"`
}

type EmailConfig struct {
	Enabled  bool   `yaml:"enabled"`
	SMTPHost string `yaml:"smtp_host"`
	SMTPPort int    `yaml:"smtp_port"`
	From     string `yaml:"from"`
	To       string `yaml:"to"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	File   string `yaml:"file"`
	Format string `yaml:"format"`
}

// NeedsServerControl returns true when save-off/save-on commands are required.
func (c *Config) NeedsServerControl() bool {
	return c.Server.Control.Type != ControlNone
}

// Load reads configuration from a YAML file and applies environment overrides.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	applyDefaults(&cfg)
	applyEnvOverrides(&cfg)

	if err := Validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Name == "" {
		cfg.Server.Name = "default"
	}
	if cfg.Server.Control.Type == "" {
		cfg.Server.Control.Type = ControlRCON
	}
	if cfg.Server.Control.RCON.Host == "" {
		cfg.Server.Control.RCON.Host = "127.0.0.1"
	}
	if cfg.Server.Control.RCON.Port == 0 {
		cfg.Server.Control.RCON.Port = 25575
	}
	if cfg.Server.Control.RCON.Timeout == 0 {
		cfg.Server.Control.RCON.Timeout = 10 * time.Second
	}
	if cfg.Backup.Mode == "" {
		cfg.Backup.Mode = BackupModeArchive
	}
	if cfg.Backup.Compression == "" {
		cfg.Backup.Compression = CompressionZstd
	}
	if cfg.Backup.HashMethod == "" {
		cfg.Backup.HashMethod = HashBlake3
	}
	if cfg.Backup.StagingDir == "" {
		cfg.Backup.StagingDir = os.TempDir()
	}
	if cfg.Backup.LockFile == "" {
		cfg.Backup.LockFile = "snapcraft.lock"
	}
	if cfg.Backup.CDC.MinSize == 0 {
		cfg.Backup.CDC.MinSize = 65536
	}
	if cfg.Backup.CDC.AvgSize == 0 {
		cfg.Backup.CDC.AvgSize = 1048576
	}
	if cfg.Backup.CDC.MaxSize == 0 {
		cfg.Backup.CDC.MaxSize = 4194304
	}
	if cfg.Backup.CDC.MinFileSize == 0 {
		cfg.Backup.CDC.MinFileSize = 8388608
	}
	if cfg.Repository.LocalPath == "" {
		cfg.Repository.LocalPath = "./snapcraft-repo"
	}
	if cfg.Repository.KeepLocalManifests == false && !cfg.Repository.CleanupAfterVerifiedUpload {
		cfg.Repository.KeepLocalManifests = true
	}
	if cfg.Rclone.Transfers == 0 {
		cfg.Rclone.Transfers = 4
	}
	if cfg.Rclone.Checkers == 0 {
		cfg.Rclone.Checkers = 8
	}
	if cfg.Rclone.Timeout == 0 {
		cfg.Rclone.Timeout = 30 * time.Minute
	}
	if cfg.Rclone.Retries == 0 {
		cfg.Rclone.Retries = 3
	}
	if cfg.Retention.Daily == 0 {
		cfg.Retention.Daily = 7
	}
	if cfg.Retention.Weekly == 0 {
		cfg.Retention.Weekly = 4
	}
	if cfg.Log.Level == "" {
		cfg.Log.Level = "info"
	}
	if cfg.Log.Format == "" {
		cfg.Log.Format = "text"
	}
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("SNAPCRAFT_SERVER_NAME"); v != "" {
		cfg.Server.Name = v
	}
	if v := os.Getenv("SNAPCRAFT_WORLD_PATH"); v != "" {
		cfg.Server.WorldPath = v
	}
	if v := os.Getenv("SNAPCRAFT_RCON_HOST"); v != "" {
		cfg.Server.Control.RCON.Host = v
	}
	if v := os.Getenv("SNAPCRAFT_RCON_PORT"); v != "" {
		if port, err := parseInt(v); err == nil {
			cfg.Server.Control.RCON.Port = port
		}
	}
	if v := os.Getenv("SNAPCRAFT_RCON_PASSWORD"); v != "" {
		cfg.Server.Control.RCON.Password = v
	}
	if v := os.Getenv("SNAPCRAFT_RCLONE_REMOTE"); v != "" {
		cfg.Rclone.Remote = v
	}
	if v := os.Getenv("SNAPCRAFT_RCLONE_PATH"); v != "" {
		cfg.Rclone.RemotePath = v
	}
	if v := os.Getenv("SNAPCRAFT_REPO_PATH"); v != "" {
		cfg.Repository.LocalPath = v
	}
	if v := os.Getenv("SNAPCRAFT_WEBHOOK_URL"); v != "" {
		cfg.Notify.Webhook.URL = v
		cfg.Notify.Webhook.Enabled = true
	}
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	return n, err
}
