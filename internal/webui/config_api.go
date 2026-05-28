package webui

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Origin173/SnapCraft/internal/config"
)

const redactedSecret = "••••••••"

// ConfigDTO is a JSON-friendly view of SnapCraft configuration for the WebUI editor.
type ConfigDTO struct {
	Server     ServerDTO     `json:"server"`
	Backup     BackupDTO     `json:"backup"`
	Repository RepositoryDTO `json:"repository"`
	Upload     UploadDTO     `json:"upload"`
	Rclone     RcloneDTO     `json:"rclone"`
	Retention  RetentionDTO  `json:"retention"`
	Schedule   ScheduleDTO   `json:"schedule"`
	Notify     NotifyDTO     `json:"notify"`
	Log        LogDTO        `json:"log"`
	WebUI      WebUIDTO      `json:"webui"`
}

type ServerDTO struct {
	Name      string      `json:"name"`
	WorldPath string      `json:"world_path"`
	Control   ControlDTO  `json:"control"`
}

type ControlDTO struct {
	Type    string       `json:"type"`
	RCON    RCONDTO      `json:"rcon"`
	Console ConsoleDTO   `json:"console"`
}

type RCONDTO struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Password string `json:"password"`
	Timeout  string `json:"timeout"`
}

type ConsoleDTO struct {
	InputPath  string `json:"input_path"`
	OutputPath string `json:"output_path"`
}

type BackupDTO struct {
	Mode              string      `json:"mode"`
	Compression       string      `json:"compression"`
	HashMethod        string      `json:"hash_method"`
	StagingDir        string      `json:"staging_dir"`
	LockFile          string      `json:"lock_file"`
	SafetyBackupLocal bool        `json:"safety_backup_local"`
	ExcludePatterns   []string    `json:"exclude_patterns"`
	Archive           ArchiveDTO  `json:"archive"`
	CDC               CDCDTO      `json:"cdc"`
}

type ArchiveDTO struct {
	IncludePaths []string `json:"include_paths"`
}

type CDCDTO struct {
	Enabled     bool  `json:"enabled"`
	MinSize     int64 `json:"min_size"`
	AvgSize     int64 `json:"avg_size"`
	MaxSize     int64 `json:"max_size"`
	MinFileSize int64 `json:"min_file_size"`
}

type RepositoryDTO struct {
	LocalPath                  string `json:"local_path"`
	CleanupAfterVerifiedUpload bool   `json:"cleanup_after_verified_upload"`
	KeepLocalManifests         bool   `json:"keep_local_manifests"`
	VerifyAfterBackup          bool   `json:"verify_after_backup"`
	VerifyAfterUpload          bool   `json:"verify_after_upload"`
}

type UploadDTO struct {
	Enabled bool `json:"enabled"`
}

type RcloneDTO struct {
	Remote     string   `json:"remote"`
	RemotePath string   `json:"remote_path"`
	BwLimit    string   `json:"bwlimit"`
	Transfers  int      `json:"transfers"`
	Checkers   int      `json:"checkers"`
	Timeout    string   `json:"timeout"`
	Retries    int      `json:"retries"`
	ExtraArgs  []string `json:"extra_args"`
}

type RetentionDTO struct {
	Daily   int `json:"daily"`
	Weekly  int `json:"weekly"`
	Monthly int `json:"monthly"`
}

type ScheduleDTO struct {
	Enabled bool   `json:"enabled"`
	Cron    string `json:"cron"`
}

type NotifyDTO struct {
	Webhook WebhookDTO `json:"webhook"`
	Email   EmailDTO   `json:"email"`
}

type WebhookDTO struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url"`
}

type EmailDTO struct {
	Enabled  bool   `json:"enabled"`
	SMTPHost string `json:"smtp_host"`
	SMTPPort int    `json:"smtp_port"`
	From     string `json:"from"`
	To       string `json:"to"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type LogDTO struct {
	Level  string `json:"level"`
	File   string `json:"file"`
	Format string `json:"format"`
}

type WebUIDTO struct {
	Enabled    bool   `json:"enabled"`
	Addr       string `json:"addr"`
	Token      string `json:"token"`
	CookieName string `json:"cookie_name"`
}

func durationString(d time.Duration) string {
	if d == 0 {
		return ""
	}
	return d.String()
}

func parseDuration(s string, fallback time.Duration) (time.Duration, error) {
	if s == "" {
		return fallback, nil
	}
	return time.ParseDuration(s)
}

func configToDTO(cfg *config.Config) ConfigDTO {
	return ConfigDTO{
		Server: ServerDTO{
			Name:      cfg.Server.Name,
			WorldPath: cfg.Server.WorldPath,
			Control: ControlDTO{
				Type: cfg.Server.Control.Type,
				RCON: RCONDTO{
					Host:     cfg.Server.Control.RCON.Host,
					Port:     cfg.Server.Control.RCON.Port,
					Password: RedactValue("password", cfg.Server.Control.RCON.Password),
					Timeout:  durationString(cfg.Server.Control.RCON.Timeout),
				},
				Console: ConsoleDTO{
					InputPath:  cfg.Server.Control.Console.InputPath,
					OutputPath: cfg.Server.Control.Console.OutputPath,
				},
			},
		},
		Backup: BackupDTO{
			Mode:              cfg.Backup.Mode,
			Compression:       cfg.Backup.Compression,
			HashMethod:        cfg.Backup.HashMethod,
			StagingDir:        cfg.Backup.StagingDir,
			LockFile:          cfg.Backup.LockFile,
			SafetyBackupLocal: cfg.Backup.SafetyBackupLocal,
			ExcludePatterns:   append([]string(nil), cfg.Backup.ExcludePatterns...),
			Archive: ArchiveDTO{
				IncludePaths: append([]string(nil), cfg.Backup.Archive.IncludePaths...),
			},
			CDC: CDCDTO{
				Enabled:     cfg.Backup.CDC.Enabled,
				MinSize:     cfg.Backup.CDC.MinSize,
				AvgSize:     cfg.Backup.CDC.AvgSize,
				MaxSize:     cfg.Backup.CDC.MaxSize,
				MinFileSize: cfg.Backup.CDC.MinFileSize,
			},
		},
		Repository: RepositoryDTO{
			LocalPath:                  cfg.Repository.LocalPath,
			CleanupAfterVerifiedUpload: cfg.Repository.CleanupAfterVerifiedUpload,
			KeepLocalManifests:         cfg.Repository.KeepLocalManifests,
			VerifyAfterBackup:          cfg.Repository.VerifyAfterBackup,
			VerifyAfterUpload:          cfg.Repository.VerifyAfterUpload,
		},
		Upload: UploadDTO{Enabled: cfg.Upload.Enabled},
		Rclone: RcloneDTO{
			Remote:     cfg.Rclone.Remote,
			RemotePath: cfg.Rclone.RemotePath,
			BwLimit:    cfg.Rclone.BwLimit,
			Transfers:  cfg.Rclone.Transfers,
			Checkers:   cfg.Rclone.Checkers,
			Timeout:    durationString(cfg.Rclone.Timeout),
			Retries:    cfg.Rclone.Retries,
			ExtraArgs:  append([]string(nil), cfg.Rclone.ExtraArgs...),
		},
		Retention: RetentionDTO{
			Daily:   cfg.Retention.Daily,
			Weekly:  cfg.Retention.Weekly,
			Monthly: cfg.Retention.Monthly,
		},
		Schedule: ScheduleDTO{
			Enabled: cfg.Schedule.Enabled,
			Cron:    cfg.Schedule.Cron,
		},
		Notify: NotifyDTO{
			Webhook: WebhookDTO{
				Enabled: cfg.Notify.Webhook.Enabled,
				URL:     cfg.Notify.Webhook.URL,
			},
			Email: EmailDTO{
				Enabled:  cfg.Notify.Email.Enabled,
				SMTPHost: cfg.Notify.Email.SMTPHost,
				SMTPPort: cfg.Notify.Email.SMTPPort,
				From:     cfg.Notify.Email.From,
				To:       cfg.Notify.Email.To,
				Username: cfg.Notify.Email.Username,
				Password: RedactValue("password", cfg.Notify.Email.Password),
			},
		},
		Log: LogDTO{
			Level:  cfg.Log.Level,
			File:   cfg.Log.File,
			Format: cfg.Log.Format,
		},
		WebUI: WebUIDTO{
			Enabled:    cfg.WebUI.Enabled,
			Addr:       cfg.WebUI.Addr,
			Token:      RedactValue("token", cfg.WebUI.Token),
			CookieName: cfg.WebUI.CookieName,
		},
	}
}

func mergeSecret(incoming, existing string) string {
	if incoming == "" || incoming == redactedSecret {
		return existing
	}
	return incoming
}

func dtoToConfig(dto ConfigDTO, existing *config.Config) (*config.Config, error) {
	rconTimeout, err := parseDuration(dto.Server.Control.RCON.Timeout, existing.Server.Control.RCON.Timeout)
	if err != nil {
		return nil, fmt.Errorf("server.control.rcon.timeout: %w", err)
	}
	rcloneTimeout, err := parseDuration(dto.Rclone.Timeout, existing.Rclone.Timeout)
	if err != nil {
		return nil, fmt.Errorf("rclone.timeout: %w", err)
	}

	cfg := *existing
	cfg.Server = config.ServerConfig{
		Name:      dto.Server.Name,
		WorldPath: dto.Server.WorldPath,
		Control: config.ControlConfig{
			Type: dto.Server.Control.Type,
			RCON: config.RCONConfig{
				Host:     dto.Server.Control.RCON.Host,
				Port:     dto.Server.Control.RCON.Port,
				Password: mergeSecret(dto.Server.Control.RCON.Password, existing.Server.Control.RCON.Password),
				Timeout:  rconTimeout,
			},
			Console: config.ConsoleConfig{
				InputPath:  dto.Server.Control.Console.InputPath,
				OutputPath: dto.Server.Control.Console.OutputPath,
			},
		},
	}
	cfg.Backup = config.BackupConfig{
		Mode:              dto.Backup.Mode,
		Compression:       dto.Backup.Compression,
		HashMethod:        dto.Backup.HashMethod,
		StagingDir:        dto.Backup.StagingDir,
		LockFile:          dto.Backup.LockFile,
		SafetyBackupLocal: dto.Backup.SafetyBackupLocal,
		ExcludePatterns:   append([]string(nil), dto.Backup.ExcludePatterns...),
		Archive: config.ArchiveConfig{
			IncludePaths: append([]string(nil), dto.Backup.Archive.IncludePaths...),
		},
		CDC: config.CDCConfig{
			Enabled:     dto.Backup.CDC.Enabled,
			MinSize:     dto.Backup.CDC.MinSize,
			AvgSize:     dto.Backup.CDC.AvgSize,
			MaxSize:     dto.Backup.CDC.MaxSize,
			MinFileSize: dto.Backup.CDC.MinFileSize,
		},
	}
	cfg.Repository = config.RepositoryConfig{
		LocalPath:                  dto.Repository.LocalPath,
		CleanupAfterVerifiedUpload: dto.Repository.CleanupAfterVerifiedUpload,
		KeepLocalManifests:         dto.Repository.KeepLocalManifests,
		VerifyAfterBackup:          dto.Repository.VerifyAfterBackup,
		VerifyAfterUpload:          dto.Repository.VerifyAfterUpload,
	}
	cfg.Upload = config.UploadConfig{Enabled: dto.Upload.Enabled}
	cfg.Rclone = config.RcloneConfig{
		Remote:     dto.Rclone.Remote,
		RemotePath: dto.Rclone.RemotePath,
		BwLimit:    dto.Rclone.BwLimit,
		Transfers:  dto.Rclone.Transfers,
		Checkers:   dto.Rclone.Checkers,
		Timeout:    rcloneTimeout,
		Retries:    dto.Rclone.Retries,
		ExtraArgs:  append([]string(nil), dto.Rclone.ExtraArgs...),
	}
	cfg.Retention = config.RetentionConfig{
		Daily:   dto.Retention.Daily,
		Weekly:  dto.Retention.Weekly,
		Monthly: dto.Retention.Monthly,
	}
	cfg.Schedule = config.ScheduleConfig{
		Enabled: dto.Schedule.Enabled,
		Cron:    dto.Schedule.Cron,
	}
	cfg.Notify = config.NotifyConfig{
		Webhook: config.WebhookConfig{
			Enabled: dto.Notify.Webhook.Enabled,
			URL:     dto.Notify.Webhook.URL,
		},
		Email: config.EmailConfig{
			Enabled:  dto.Notify.Email.Enabled,
			SMTPHost: dto.Notify.Email.SMTPHost,
			SMTPPort: dto.Notify.Email.SMTPPort,
			From:     dto.Notify.Email.From,
			To:       dto.Notify.Email.To,
			Username: dto.Notify.Email.Username,
			Password: mergeSecret(dto.Notify.Email.Password, existing.Notify.Email.Password),
		},
	}
	cfg.Log = config.LogConfig{
		Level:  dto.Log.Level,
		File:   dto.Log.File,
		Format: dto.Log.Format,
	}
	newToken := mergeSecret(dto.WebUI.Token, existing.WebUI.Token)
	cfg.WebUI = config.WebUIConfig{
		Enabled:    dto.WebUI.Enabled,
		Addr:       dto.WebUI.Addr,
		Token:      newToken,
		CookieName: dto.WebUI.CookieName,
	}
	return &cfg, nil
}

func (s *Server) getConfig() *config.Config {
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.cfg
}

func (s *Server) setConfig(cfg *config.Config) {
	s.configMu.Lock()
	defer s.configMu.Unlock()
	s.cfg = cfg
}

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"path":   s.configPath,
		"config": configToDTO(s.getConfig()),
	})
}

func (s *Server) handlePutConfig(w http.ResponseWriter, r *http.Request) {
	if s.configPath == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("config path unknown; restart with --config"))
		return
	}
	var dto ConfigDTO
	if err := decodeJSON(r, &dto); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := dtoToConfig(dto, s.getConfig())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := config.Save(s.configPath, cfg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.setConfig(cfg)
	if cfg.WebUI.Token != "" {
		s.auth.UpdateToken(cfg.WebUI.Token)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"message": "配置已保存",
		"config":  configToDTO(cfg),
	})
}

func (s *Server) handleValidateConfig(w http.ResponseWriter, r *http.Request) {
	var dto ConfigDTO
	if err := decodeJSON(r, &dto); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := dtoToConfig(dto, s.getConfig())
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := config.Validate(cfg); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true, "message": "配置校验通过"})
}