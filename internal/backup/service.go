package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/Origin173/SnapCraft/internal/archive"
	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/lockfile"
	"github.com/Origin173/SnapCraft/internal/minecraft"
	"github.com/Origin173/SnapCraft/internal/notify"
	"github.com/Origin173/SnapCraft/internal/rclone"
	"github.com/Origin173/SnapCraft/internal/repository"
	"github.com/Origin173/SnapCraft/internal/snapshot"
)

// Service orchestrates backup operations.
type Service struct {
	cfg      *config.Config
	mc       minecraft.Controller
	rclone   rclone.Runner
	repo     *repository.Repository
	syncer   *repository.Syncer
	notifier notify.Notifier
	lockPath string
}

// NewService creates a backup service.
func NewService(cfg *config.Config, mc minecraft.Controller, runner rclone.Runner, notifier notify.Notifier) (*Service, error) {
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
		notifier: notifier,
		lockPath: cfg.Backup.LockFile,
	}, nil
}

func (s *Service) Close() error {
	if s.repo != nil {
		return s.repo.Close()
	}
	return nil
}

// Result describes a completed backup run.
type Result struct {
	Manifest *snapshot.Manifest
	Snapshot *repository.SnapshotRecord
}

// Run executes a full backup cycle.
func (s *Service) Run(ctx context.Context) (*Result, error) {
	if err := config.EnsureBackupDirs(s.cfg); err != nil {
		return nil, err
	}
	lock, err := lockfile.Acquire(s.lockPath)
	if err != nil {
		return nil, err
	}
	defer lock.Release()

	id := snapshot.NewID()
	manifest := snapshot.NewManifest(s.cfg, id)
	rec, err := s.repo.CreateSnapshot(id, s.cfg.Backup.Mode)
	if err != nil {
		return nil, err
	}

	var saveOffDone bool
	if s.cfg.NeedsServerControl() {
		defer func() {
			if saveOffDone {
				if err := s.mc.SaveOn(context.Background()); err != nil {
					s.notifier.NotifyFailure(manifest, fmt.Errorf("save-on after failure: %w", err))
				}
			}
		}()

		_ = s.mc.Say(ctx, "[SnapCraft] Backup starting...")
		if err := s.mc.SaveOff(ctx); err != nil {
			_ = s.repo.MarkSnapshotFailed(id, err.Error())
			s.notifier.NotifyFailure(manifest, err)
			return nil, fmt.Errorf("save-off: %w", err)
		}
		saveOffDone = true

		if err := s.mc.SaveAll(ctx); err != nil {
			_ = s.repo.MarkSnapshotFailed(id, err.Error())
			s.notifier.NotifyFailure(manifest, err)
			return nil, fmt.Errorf("save-all: %w", err)
		}
	}

	stagingWorld, err := s.stageWorld(ctx)
	if err != nil {
		_ = s.repo.MarkSnapshotFailed(id, err.Error())
		s.notifier.NotifyFailure(manifest, err)
		return nil, err
	}
	defer os.RemoveAll(stagingWorld)

	if s.cfg.NeedsServerControl() {
		if err := s.mc.SaveOn(ctx); err != nil {
			_ = s.repo.MarkSnapshotFailed(id, err.Error())
			s.notifier.NotifyFailure(manifest, err)
			return nil, fmt.Errorf("save-on: %w", err)
		}
		saveOffDone = false
	}

	var backupErr error
	switch s.cfg.Backup.Mode {
	case config.BackupModeArchive:
		backupErr = s.runArchiveBackup(ctx, id, manifest, stagingWorld)
	case config.BackupModeDirectory:
		backupErr = s.runDirectoryBackup(ctx, manifest, stagingWorld)
	case config.BackupModeIncremental:
		backupErr = s.runIncrementalBackup(ctx, id, stagingWorld)
	default:
		backupErr = fmt.Errorf("unknown backup mode: %s", s.cfg.Backup.Mode)
	}

	if backupErr != nil {
		_ = s.repo.MarkSnapshotFailed(id, backupErr.Error())
		manifest.MarkFailed(backupErr)
		s.notifier.NotifyFailure(manifest, backupErr)
		return nil, backupErr
	}

	if s.cfg.Repository.VerifyAfterBackup {
		if err := s.repo.VerifySnapshotLocal(id); err != nil {
			_ = s.repo.MarkSnapshotFailed(id, err.Error())
			manifest.MarkFailed(err)
			s.notifier.NotifyFailure(manifest, err)
			return nil, fmt.Errorf("local verify: %w", err)
		}
	}

	rec, _ = s.repo.GetSnapshot(id)
	manifest.FileCount = rec.FileCount
	manifest.TotalBytes = rec.TotalBytes
	manifest.ArchivePath = rec.ArchivePath
	manifest.MarkCompleted()

	if s.cfg.Upload.Enabled {
		if err := s.syncer.SyncSnapshot(ctx, id); err != nil {
			// local backup succeeded; remote failed
			manifest.Error = err.Error()
		} else {
			manifest.RcloneSummary = s.rclone.LastOutput()
		}
	}

	s.notifier.NotifySuccess(manifest)
	return &Result{Manifest: manifest, Snapshot: rec}, nil
}

func (s *Service) stageWorld(ctx context.Context) (string, error) {
	stagingDir, err := os.MkdirTemp(s.cfg.Backup.StagingDir, "snapcraft-stage-*")
	if err != nil {
		return "", err
	}
	dest := filepath.Join(stagingDir, filepath.Base(s.cfg.Server.WorldPath))
	if err := copyDir(s.cfg.Server.WorldPath, dest); err != nil {
		os.RemoveAll(stagingDir)
		return "", fmt.Errorf("stage world: %w", err)
	}
	return stagingDir, nil
}

func (s *Service) runArchiveBackup(ctx context.Context, snapshotID string, manifest *snapshot.Manifest, stagingDir string) error {
	ext := archive.ArchiveExtension(s.cfg)
	archiveName := snapshotID + ext
	tempArchive := filepath.Join(s.cfg.Backup.StagingDir, archiveName)
	defer os.Remove(tempArchive)

	sources := []string{filepath.Join(stagingDir, filepath.Base(s.cfg.Server.WorldPath))}
	result, err := archive.Create(s.cfg, tempArchive, sources)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}

	dest, hash, size, err := s.repo.StoreArchive(ctx, snapshotID, tempArchive)
	if err != nil {
		return fmt.Errorf("store archive: %w", err)
	}

	if err := s.repo.MarkSnapshotLocalComplete(snapshotID, result.FileCount, result.TotalBytes, dest, hash, size); err != nil {
		return err
	}
	manifest.ArchivePath = dest
	return nil
}

func (s *Service) runIncrementalBackup(ctx context.Context, snapshotID, stagingDir string) error {
	worldSrc := filepath.Join(stagingDir, filepath.Base(s.cfg.Server.WorldPath))
	fileCount, totalBytes, err := s.repo.ScanAndStoreIncremental(ctx, snapshotID, worldSrc)
	if err != nil {
		return fmt.Errorf("incremental scan: %w", err)
	}
	return s.repo.MarkSnapshotLocalComplete(snapshotID, fileCount, totalBytes, "", "", 0)
}

func (s *Service) runDirectoryBackup(ctx context.Context, manifest *snapshot.Manifest, stagingDir string) error {
	worldSrc := filepath.Join(stagingDir, filepath.Base(s.cfg.Server.WorldPath))
	layout := snapshot.RemoteLayoutFor(s.cfg)
	remoteCurrent := s.cfg.Rclone.Remote + ":" + filepath.ToSlash(layout.DirCurrent)
	remoteHistory := s.cfg.Rclone.Remote + ":" + filepath.ToSlash(layout.HistoryPath(manifest.ID))

	if err := s.rclone.Sync(ctx, worldSrc, remoteCurrent, remoteHistory); err != nil {
		return fmt.Errorf("directory sync: %w", err)
	}

	manifest.DirectoryPath = layout.DirCurrent
	manifest.HistoryPath = layout.HistoryPath(manifest.ID)
	manifest.Restorable = true
	return s.repo.MarkSnapshotLocalComplete(manifest.ID, 0, 0, "", "", 0)
}

func copyDir(src, dst string) error {
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
		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
