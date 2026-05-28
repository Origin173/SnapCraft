package restore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Origin173/SnapCraft/internal/archive"
	"github.com/Origin173/SnapCraft/internal/backup"
	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/minecraft"
	"github.com/Origin173/SnapCraft/internal/notify"
	"github.com/Origin173/SnapCraft/internal/rclone"
	"github.com/Origin173/SnapCraft/internal/repository"
	"github.com/Origin173/SnapCraft/internal/snapshot"
)

const (
	SourceLocal  = "local"
	SourceRemote = "remote"
)

// Options controls restore behavior.
type Options struct {
	ForceOnline bool
	Source      string
}

// Service orchestrates restore operations.
type Service struct {
	cfg      *config.Config
	mc       minecraft.Controller
	rclone   rclone.Runner
	repo     *repository.Repository
	syncer   *repository.Syncer
	backup   *backup.Service
	notifier notify.Notifier
}

func NewService(cfg *config.Config, mc minecraft.Controller, runner rclone.Runner, backupSvc *backup.Service, notifier notify.Notifier) (*Service, error) {
	repo, err := repository.Open(cfg)
	if err != nil {
		return nil, err
	}
	return &Service{
		cfg:      cfg,
		mc:       mc,
		rclone:   runner,
		repo:     repo,
		syncer:   repository.NewSyncer(repo, runner),
		backup:   backupSvc,
		notifier: notifier,
	}, nil
}

func (s *Service) Close() error {
	if s.repo != nil {
		return s.repo.Close()
	}
	return nil
}

// Run restores a snapshot by ID.
func (s *Service) Run(ctx context.Context, snapshotID string, opts Options) error {
	if opts.Source == "" {
		opts.Source = SourceLocal
	}

	rec, err := s.repo.GetSnapshot(snapshotID)
	if err != nil {
		return err
	}
	if !rec.Restorable {
		return fmt.Errorf("snapshot %s is not restorable (status: %s)", snapshotID, rec.Status)
	}

	if !opts.ForceOnline && s.cfg.NeedsServerControl() {
		return fmt.Errorf("restore requires server to be stopped; use --force-online to restore while running (not recommended)")
	}

	if s.cfg.Backup.SafetyBackupLocal && s.backup != nil {
		if _, err := s.backup.Run(ctx); err != nil {
			return fmt.Errorf("safety backup failed: %w", err)
		}
	}

	if opts.Source == SourceRemote {
		if !s.cfg.Upload.Enabled {
			return fmt.Errorf("remote restore requested but upload.enabled is false")
		}
		if err := s.syncer.FetchSnapshot(ctx, snapshotID); err != nil {
			return fmt.Errorf("fetch remote snapshot: %w", err)
		}
	}

	switch rec.Mode {
	case config.BackupModeArchive:
		return s.restoreArchive(ctx, rec)
	case config.BackupModeIncremental:
		return s.restoreIncremental(ctx, rec)
	case config.BackupModeDirectory:
		return s.restoreDirectoryLegacy(ctx, snapshotID)
	default:
		return fmt.Errorf("unknown backup mode: %s", rec.Mode)
	}
}

func (s *Service) restoreArchive(ctx context.Context, rec *repository.SnapshotRecord) error {
	if rec.ArchivePath == "" {
		return fmt.Errorf("snapshot %s has no local archive", rec.ID)
	}
	if _, err := os.Stat(rec.ArchivePath); err != nil {
		return fmt.Errorf("local archive missing for %s: %w (try --remote)", rec.ID, err)
	}

	extractDir, err := os.MkdirTemp(s.cfg.Backup.StagingDir, "snapcraft-restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(extractDir)

	if err := archive.Extract(rec.ArchivePath, extractDir); err != nil {
		return fmt.Errorf("extract archive: %w", err)
	}

	worldName := filepath.Base(s.cfg.Server.WorldPath)
	extractedWorld := filepath.Join(extractDir, worldName)
	if _, err := os.Stat(extractedWorld); err != nil {
		entries, _ := os.ReadDir(extractDir)
		if len(entries) == 1 && entries[0].IsDir() {
			extractedWorld = filepath.Join(extractDir, entries[0].Name())
		} else {
			return fmt.Errorf("world directory %q not found in archive", worldName)
		}
	}

	if err := s.atomicReplaceWorld(extractedWorld); err != nil {
		return err
	}

	manifest := &snapshot.Manifest{ID: rec.ID, ServerName: rec.ServerName}
	s.notifier.NotifyRestore(manifest)
	return nil
}

func (s *Service) restoreIncremental(ctx context.Context, rec *repository.SnapshotRecord) error {
	tmpWorld, err := os.MkdirTemp(s.cfg.Backup.StagingDir, "snapcraft-inc-restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpWorld)

	if err := s.repo.RestoreIncremental(rec.ID, tmpWorld); err != nil {
		return err
	}
	return s.atomicReplaceWorld(tmpWorld)
}

func (s *Service) restoreDirectoryLegacy(ctx context.Context, snapshotID string) error {
	tmpWorld, err := os.MkdirTemp(s.cfg.Backup.StagingDir, "snapcraft-dir-restore-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpWorld)

	layout := snapshot.RemoteLayoutFor(s.cfg)
	remoteCurrent := s.cfg.Rclone.Remote + ":" + filepath.ToSlash(layout.DirCurrent)
	if err := s.rclone.CopyToLocal(ctx, remoteCurrent, tmpWorld); err != nil {
		return fmt.Errorf("download directory snapshot: %w", err)
	}

	entries, err := os.ReadDir(tmpWorld)
	if err != nil {
		return err
	}
	source := tmpWorld
	if len(entries) == 1 && entries[0].IsDir() {
		source = filepath.Join(tmpWorld, entries[0].Name())
	}
	return s.atomicReplaceWorld(source)
}

func (s *Service) atomicReplaceWorld(source string) error {
	worldPath := s.cfg.Server.WorldPath
	parent := filepath.Dir(worldPath)
	base := filepath.Base(worldPath)
	backupPath := filepath.Join(parent, base+".snapcraft-old-"+time.Now().Format("20060102-150405"))

	if err := os.Rename(worldPath, backupPath); err != nil {
		return fmt.Errorf("move current world aside: %w", err)
	}

	if err := copyTree(source, worldPath); err != nil {
		_ = os.Rename(backupPath, worldPath)
		return fmt.Errorf("copy restored world: %w", err)
	}
	return nil
}

func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
