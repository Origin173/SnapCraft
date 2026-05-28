package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Validate checks that configuration is complete and consistent.
func Validate(cfg *Config) error {
	var errs []string

	if strings.TrimSpace(cfg.Server.Name) == "" {
		errs = append(errs, "server.name is required")
	}
	if strings.TrimSpace(cfg.Server.WorldPath) == "" {
		errs = append(errs, "server.world_path is required")
	} else if !filepath.IsAbs(cfg.Server.WorldPath) {
		abs, err := filepath.Abs(cfg.Server.WorldPath)
		if err != nil {
			errs = append(errs, fmt.Sprintf("server.world_path: %v", err))
		} else {
			cfg.Server.WorldPath = abs
		}
	}

	switch cfg.Server.Control.Type {
	case ControlRCON:
		if strings.TrimSpace(cfg.Server.Control.RCON.Password) == "" {
			errs = append(errs, "server.control.rcon.password is required when control type is rcon")
		}
		if cfg.Server.Control.RCON.Port <= 0 || cfg.Server.Control.RCON.Port > 65535 {
			errs = append(errs, "server.control.rcon.port must be between 1 and 65535")
		}
	case ControlConsole:
		if strings.TrimSpace(cfg.Server.Control.Console.InputPath) == "" {
			errs = append(errs, "server.control.console.input_path is required when control type is console")
		}
	case ControlNone:
		// offline/singleplayer mode: no server control required
	default:
		errs = append(errs, fmt.Sprintf("server.control.type must be %q, %q, or %q", ControlRCON, ControlConsole, ControlNone))
	}

	switch cfg.Backup.Mode {
	case BackupModeArchive, BackupModeDirectory, BackupModeIncremental:
	default:
		errs = append(errs, fmt.Sprintf("backup.mode must be %q, %q, or %q", BackupModeArchive, BackupModeDirectory, BackupModeIncremental))
	}

	switch cfg.Backup.Compression {
	case CompressionZstd, CompressionGzip, CompressionNone:
	default:
		errs = append(errs, fmt.Sprintf("backup.compression must be %q, %q, or %q", CompressionZstd, CompressionGzip, CompressionNone))
	}

	switch cfg.Backup.HashMethod {
	case HashBlake3, HashSHA256:
	default:
		errs = append(errs, fmt.Sprintf("backup.hash_method must be %q or %q", HashBlake3, HashSHA256))
	}

	if strings.TrimSpace(cfg.Repository.LocalPath) == "" {
		errs = append(errs, "repository.local_path is required")
	}

	if cfg.Upload.Enabled {
		if strings.TrimSpace(cfg.Rclone.Remote) == "" {
			errs = append(errs, "rclone.remote is required when upload.enabled is true")
		}
		if strings.TrimSpace(cfg.Rclone.RemotePath) == "" {
			errs = append(errs, "rclone.remote_path is required when upload.enabled is true")
		}
	}

	if cfg.Schedule.Enabled && strings.TrimSpace(cfg.Schedule.Cron) == "" {
		errs = append(errs, "schedule.cron is required when schedule.enabled is true")
	}

	if cfg.Notify.Webhook.Enabled && strings.TrimSpace(cfg.Notify.Webhook.URL) == "" {
		errs = append(errs, "notify.webhook.url is required when webhook is enabled")
	}

	if len(errs) > 0 {
		return fmt.Errorf("config validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}

	return nil
}

// ValidatePaths checks filesystem paths exist where applicable (optional pre-flight).
func ValidatePaths(cfg *Config) error {
	if _, err := os.Stat(cfg.Server.WorldPath); err != nil {
		return fmt.Errorf("server.world_path does not exist: %w", err)
	}
	if err := EnsureBackupDirs(cfg); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.Repository.LocalPath, 0o755); err != nil {
		return fmt.Errorf("repository.local_path: %w", err)
	}
	return nil
}
