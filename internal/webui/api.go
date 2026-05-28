package webui

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Origin173/SnapCraft/internal/backup"
	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/minecraft"
	"github.com/Origin173/SnapCraft/internal/notify"
	"github.com/Origin173/SnapCraft/internal/rclone"
	"github.com/Origin173/SnapCraft/internal/repository"
	"github.com/Origin173/SnapCraft/internal/restore"
	"github.com/Origin173/SnapCraft/internal/retention"
	"github.com/Origin173/SnapCraft/internal/snapshot"
)

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	repoExists := false
	snapCount := 0
	var latest *repository.SnapshotRecord
	if repo, err := repository.Open(s.getConfig()); err == nil {
		repoExists = true
		if snaps, err := repo.ListSnapshots(); err == nil {
			snapCount = len(snaps)
			if len(snaps) > 0 {
				latest = snaps[0]
			}
		}
		repo.Close()
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"server": map[string]any{
			"name":         s.getConfig().Server.Name,
			"world_path":   s.getConfig().Server.WorldPath,
			"control_type": s.getConfig().Server.Control.Type,
		},
		"backup": map[string]any{
			"mode":        s.getConfig().Backup.Mode,
			"compression": s.getConfig().Backup.Compression,
		},
		"repository": map[string]any{
			"local_path": s.getConfig().Repository.LocalPath,
			"exists":     repoExists,
			"snapshots":  snapCount,
		},
		"upload": map[string]any{
			"enabled":     s.getConfig().Upload.Enabled,
			"remote":      s.getConfig().Rclone.Remote,
			"remote_path": s.getConfig().Rclone.RemotePath,
		},
		"schedule": map[string]any{
			"enabled": s.getConfig().Schedule.Enabled,
			"cron":    s.getConfig().Schedule.Cron,
		},
		"webui": map[string]any{
			"addr": s.getConfig().WebUI.Addr,
		},
		"latest_snapshot": snapshotFromRecord(latest),
		"job":             s.jobs.Current(),
		"config_summary":  configSummary(s.getConfig()),
	})
}

func configSummary(cfg *config.Config) map[string]any {
	return map[string]any{
		"server": map[string]any{
			"name":         cfg.Server.Name,
			"world_path":   cfg.Server.WorldPath,
			"control_type": cfg.Server.Control.Type,
			"rcon_host":    cfg.Server.Control.RCON.Host,
			"rcon_port":    cfg.Server.Control.RCON.Port,
			"rcon_password": RedactValue("password", cfg.Server.Control.RCON.Password),
		},
		"backup": map[string]any{
			"mode":        cfg.Backup.Mode,
			"compression": cfg.Backup.Compression,
			"hash_method": cfg.Backup.HashMethod,
			"staging_dir": cfg.Backup.StagingDir,
		},
		"repository": cfg.Repository,
		"upload":     cfg.Upload,
		"rclone": map[string]any{
			"remote":      cfg.Rclone.Remote,
			"remote_path": cfg.Rclone.RemotePath,
			"bwlimit":     cfg.Rclone.BwLimit,
			"transfers":   cfg.Rclone.Transfers,
			"checkers":    cfg.Rclone.Checkers,
		},
		"retention": cfg.Retention,
		"schedule":  cfg.Schedule,
		"notify": map[string]any{
			"webhook_enabled": cfg.Notify.Webhook.Enabled,
			"email_enabled":   cfg.Notify.Email.Enabled,
		},
	}
}

func (s *Server) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	repo, err := repository.Open(s.getConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer repo.Close()

	snaps, err := repo.ListSnapshots()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]any, 0, len(snaps))
	for _, snap := range snaps {
		out = append(out, snapshotFromRecord(snap))
	}
	writeJSON(w, http.StatusOK, map[string]any{"snapshots": out})
}

func (s *Server) handleGetSnapshot(w http.ResponseWriter, r *http.Request) {
	id := pathParam(r, "id")
	repo, err := repository.Open(s.getConfig())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer repo.Close()

	rec, err := repo.GetSnapshot(id)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Errorf("snapshot not found"))
		return
	}
	writeJSON(w, http.StatusOK, snapshotFromRecord(rec))
}

func snapshotFromRecord(rec *repository.SnapshotRecord) map[string]any {
	if rec == nil {
		return nil
	}
	return map[string]any{
		"id":            rec.ID,
		"server_name":   rec.ServerName,
		"world_path":    rec.WorldPath,
		"mode":          rec.Mode,
		"compression":   rec.Compression,
		"status":        rec.Status,
		"local_status":  rec.LocalStatus,
		"remote_status": rec.RemoteStatus,
		"archive_path":  rec.ArchivePath,
		"file_count":    rec.FileCount,
		"total_bytes":   rec.TotalBytes,
		"started_at":    rec.StartedAt,
		"completed_at":  rec.CompletedAt,
		"error":         rec.Error,
		"restorable":    rec.Restorable,
	}
}

func (s *Server) handleBackupRun(w http.ResponseWriter, r *http.Request) {
	if err := s.jobs.RunAsync("backup", func(ctx context.Context) (string, error) {
		if err := rclone.EnsureRemoteConfigured(s.getConfig()); err != nil {
			return "", err
		}
		mc, err := minecraft.NewController(s.getConfig())
		if err != nil {
			return "", err
		}
		defer mc.Close()

		runner := rclone.NewRunner(s.getConfig())
		notifier := notify.BuildFromConfig(s.getConfig())
		svc, err := backup.NewService(s.getConfig(), mc, runner, notifier)
		if err != nil {
			return "", err
		}
		defer svc.Close()

		result, err := svc.Run(ctx)
		if err != nil {
			return "", err
		}
		s.jobs.SetSnapshotID(result.Manifest.ID)
		return fmt.Sprintf("backup completed: %s (%d bytes)", result.Manifest.ID, result.Manifest.TotalBytes), nil
	}); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": s.jobs.Current()})
}

func (s *Server) handleJobCurrent(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.jobs.Current())
}

func (s *Server) handleRestore(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SnapshotID  string `json:"snapshot_id"`
		Source      string `json:"source"`
		ForceOnline bool   `json:"force_online"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.SnapshotID == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("snapshot_id is required"))
		return
	}
	source := body.Source
	if source == "" {
		source = restore.SourceLocal
	}
	if source == restore.SourceRemote {
		if err := rclone.EnsureRemoteConfigured(s.getConfig()); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	if err := s.jobs.RunAsync("restore", func(ctx context.Context) (string, error) {
		mc, err := minecraft.NewController(s.getConfig())
		if err != nil {
			return "", err
		}
		defer mc.Close()

		runner := rclone.NewRunner(s.getConfig())
		notifier := notify.BuildFromConfig(s.getConfig())
		backupSvc, err := backup.NewService(s.getConfig(), mc, runner, notifier)
		if err != nil {
			return "", err
		}
		defer backupSvc.Close()

		svc, err := restore.NewService(s.getConfig(), mc, runner, backupSvc, notifier)
		if err != nil {
			return "", err
		}
		defer svc.Close()

		opts := restore.Options{ForceOnline: body.ForceOnline, Source: source}
		if err := svc.Run(ctx, body.SnapshotID, opts); err != nil {
			return "", err
		}
		s.jobs.SetSnapshotID(body.SnapshotID)
		return fmt.Sprintf("restored snapshot %s from %s", body.SnapshotID, source), nil
	}); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": s.jobs.Current()})
}

func (s *Server) handleRepoInit(w http.ResponseWriter, r *http.Request) {
	if err := repository.Init(s.getConfig()); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("repository initialized at %s", s.getConfig().Repository.LocalPath)})
}

func (s *Server) handleRepoVerify(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SnapshotID string `json:"snapshot_id"`
	}
	_ = decodeJSON(r, &body)

	if err := s.jobs.RunAsync("repo_verify", func(ctx context.Context) (string, error) {
		repo, err := repository.Open(s.getConfig())
		if err != nil {
			return "", err
		}
		defer repo.Close()

		if body.SnapshotID != "" {
			if err := repo.VerifySnapshotLocal(body.SnapshotID); err != nil {
				return "", err
			}
			return fmt.Sprintf("snapshot %s verified", body.SnapshotID), nil
		}

		snaps, err := repo.ListSnapshots()
		if err != nil {
			return "", err
		}
		for _, snap := range snaps {
			if err := repo.VerifySnapshotLocal(snap.ID); err != nil {
				return "", fmt.Errorf("snapshot %s: %w", snap.ID, err)
			}
		}
		return fmt.Sprintf("verified %d snapshot(s)", len(snaps)), nil
	}); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": s.jobs.Current()})
}

func (s *Server) handlePrunePreview(w http.ResponseWriter, r *http.Request) {
	manifests, err := s.loadManifestsForPrune()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	plan := retention.Compute(s.getConfig(), manifests)
	writeJSON(w, http.StatusOK, map[string]any{
		"keep":   manifestList(plan.Keep),
		"delete": manifestList(plan.Delete),
	})
}

func (s *Server) handlePruneApply(w http.ResponseWriter, r *http.Request) {
	if err := s.jobs.RunAsync("prune", func(ctx context.Context) (string, error) {
		store := snapshot.NewStore(s.getConfig(), rclone.NewRunner(s.getConfig()))
		manifests, err := store.List(ctx)
		if err != nil {
			return "", err
		}
		plan := retention.Compute(s.getConfig(), manifests)
		for _, m := range plan.Delete {
			if err := store.Delete(ctx, m); err != nil {
				return "", fmt.Errorf("delete %s: %w", m.ID, err)
			}
		}
		return fmt.Sprintf("pruned %d snapshot(s)", len(plan.Delete)), nil
	}); err != nil {
		writeError(w, http.StatusConflict, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"job": s.jobs.Current()})
}

func (s *Server) loadManifestsForPrune() ([]*snapshot.Manifest, error) {
	store := snapshot.NewStore(s.getConfig(), rclone.NewRunner(s.getConfig()))
	return store.List(context.Background())
}

func manifestList(in []*snapshot.Manifest) []map[string]any {
	out := make([]map[string]any, 0, len(in))
	for _, m := range in {
		out = append(out, map[string]any{
			"id":          m.ID,
			"started_at":  m.StartedAt,
			"total_bytes": m.TotalBytes,
			"status":      m.Status,
		})
	}
	return out
}

func (s *Server) handleRcloneList(w http.ResponseWriter, r *http.Request) {
	remotes, err := rclone.ListRemotes()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"remotes": remotes})
}

func (s *Server) handleRcloneShow(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	cfg, err := rclone.ShowRemote(name)
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, RedactMap(cfg))
}

func (s *Server) handleRcloneCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       string            `json:"name"`
		Type       string            `json:"type"`
		Parameters map[string]string `json:"parameters"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if body.Name == "" || body.Type == "" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("name and type are required"))
		return
	}
	if err := rclone.CreateRemote(body.Name, body.Type, body.Parameters); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"message": fmt.Sprintf("created remote %q", body.Name)})
}

func (s *Server) handleRcloneUpdate(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	var body struct {
		Parameters map[string]string `json:"parameters"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(body.Parameters) == 0 {
		writeError(w, http.StatusBadRequest, fmt.Errorf("parameters are required"))
		return
	}
	if err := rclone.UpdateRemote(name, body.Parameters); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("updated remote %q", name)})
}

func (s *Server) handleRcloneDelete(w http.ResponseWriter, r *http.Request) {
	name := pathParam(r, "name")
	if err := rclone.DeleteRemote(name); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": fmt.Sprintf("deleted remote %q", name)})
}

func (s *Server) handleRcloneProviders(w http.ResponseWriter, r *http.Request) {
	providers, err := rclone.ListProviders()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	out := make([]map[string]string, 0, len(providers))
	for _, p := range providers {
		out = append(out, map[string]string{"name": p.Name, "description": p.Description})
	}
	writeJSON(w, http.StatusOK, map[string]any{"providers": out})
}
