package repository

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/rclone"
)

// Syncer uploads local repository payloads to remote storage.
type Syncer struct {
	repo   *Repository
	runner rclone.Runner
	cfg    *config.Config
}

func NewSyncer(repo *Repository, runner rclone.Runner) *Syncer {
	return &Syncer{repo: repo, runner: runner, cfg: repo.cfg}
}

func (s *Syncer) SyncSnapshot(ctx context.Context, snapshotID string) error {
	if !s.cfg.Upload.Enabled {
		return nil
	}
	rec, err := s.repo.GetSnapshot(snapshotID)
	if err != nil {
		return err
	}

	remoteBase := s.cfg.Rclone.Remote + ":" + filepath.ToSlash(s.cfg.Rclone.RemotePath)

	switch rec.Mode {
	case config.BackupModeArchive:
		if rec.ArchivePath == "" {
			return fmt.Errorf("no archive to upload")
		}
		rel := "archives/" + filepath.Base(rec.ArchivePath)
		remote := remoteBase + "/" + filepath.ToSlash(rel)
		if err := s.runner.Copy(ctx, rec.ArchivePath, remote); err != nil {
			_ = s.repo.MarkSnapshotRemoteFailed(snapshotID, err.Error())
			return err
		}
		if s.cfg.Repository.VerifyAfterUpload {
			if err := s.runner.Check(ctx, rec.ArchivePath, remote); err != nil {
				_ = s.repo.MarkSnapshotRemoteFailed(snapshotID, err.Error())
				return fmt.Errorf("remote verify: %w", err)
			}
		}
	case config.BackupModeIncremental:
		if err := s.uploadIncrementalObjects(ctx, snapshotID, remoteBase); err != nil {
			_ = s.repo.MarkSnapshotRemoteFailed(snapshotID, err.Error())
			return err
		}
	default:
		return fmt.Errorf("unsupported mode for sync: %s", rec.Mode)
	}

	if err := s.uploadManifest(ctx, snapshotID, remoteBase); err != nil {
		_ = s.repo.MarkSnapshotRemoteFailed(snapshotID, err.Error())
		return err
	}

	if err := s.repo.MarkSnapshotRemoteComplete(snapshotID); err != nil {
		return err
	}

	if s.cfg.Repository.CleanupAfterVerifiedUpload {
		return s.repo.CleanupLocalPayload(snapshotID)
	}
	return nil
}

func (s *Syncer) uploadIncrementalObjects(ctx context.Context, snapshotID, remoteBase string) error {
	entries, err := s.repo.LoadEntries(snapshotID)
	if err != nil {
		return err
	}
	uploaded := map[string]bool{}
	for _, e := range entries {
		if e.IsChunked {
			for _, ch := range e.ChunkHashes {
				if uploaded[ch] {
					continue
				}
				if err := s.uploadChunk(ctx, ch, remoteBase); err != nil {
					return err
				}
				uploaded[ch] = true
			}
		} else if e.ObjectHash != "" {
			if uploaded[e.ObjectHash] {
				continue
			}
			if err := s.uploadObject(ctx, e.ObjectHash, remoteBase); err != nil {
				return err
			}
			uploaded[e.ObjectHash] = true
		}
	}
	return nil
}

func (s *Syncer) uploadObject(ctx context.Context, hash, remoteBase string) error {
	var localPath string
	if err := s.repo.db.QueryRow(`SELECT local_path FROM objects WHERE hash=?`, hash).Scan(&localPath); err != nil {
		return err
	}
	rel := "repo/objects/" + hash[0:2] + "/" + hash[2:4] + "/" + filepath.Base(localPath)
	remote := remoteBase + "/" + filepath.ToSlash(rel)
	if err := s.runner.Copy(ctx, localPath, remote); err != nil {
		return err
	}
	if s.cfg.Repository.VerifyAfterUpload {
		if err := s.runner.Check(ctx, localPath, remote); err != nil {
			return err
		}
	}
	_, err := s.repo.db.Exec(`UPDATE objects SET uploaded=1, verified=1, remote_path=? WHERE hash=?`, rel, hash)
	return err
}

func (s *Syncer) uploadChunk(ctx context.Context, hash, remoteBase string) error {
	var localPath string
	if err := s.repo.db.QueryRow(`SELECT local_path FROM chunks WHERE hash=?`, hash).Scan(&localPath); err != nil {
		return err
	}
	rel := "repo/chunks/" + hash[0:2] + "/" + hash[2:4] + "/" + filepath.Base(localPath)
	remote := remoteBase + "/" + filepath.ToSlash(rel)
	if err := s.runner.Copy(ctx, localPath, remote); err != nil {
		return err
	}
	if s.cfg.Repository.VerifyAfterUpload {
		if err := s.runner.Check(ctx, localPath, remote); err != nil {
			return err
		}
	}
	_, err := s.repo.db.Exec(`UPDATE chunks SET uploaded=1, verified=1, remote_path=? WHERE hash=?`, rel, hash)
	return err
}

func (s *Syncer) uploadManifest(ctx context.Context, snapshotID, remoteBase string) error {
	rec, err := s.repo.GetSnapshot(snapshotID)
	if err != nil {
		return err
	}
	manifestPath := filepath.Join(s.repo.layout.Manifests, snapshotID+".json")
	data := fmt.Sprintf(`{"id":%q,"mode":%q,"archive_path":%q,"archive_hash":%q,"archive_size":%d,"status":%q}`,
		rec.ID, rec.Mode, rec.ArchivePath, rec.ArchiveHash, rec.ArchiveSize, rec.Status)
	if err := os.WriteFile(manifestPath, []byte(data), 0o644); err != nil {
		return err
	}
	remote := remoteBase + "/manifests/" + snapshotID + ".json"
	return s.runner.Copy(ctx, manifestPath, remote)
}

// FetchSnapshot downloads missing remote objects for restore.
func (s *Syncer) FetchSnapshot(ctx context.Context, snapshotID string) error {
	rec, err := s.repo.GetSnapshot(snapshotID)
	if err != nil {
		return err
	}
	remoteBase := s.cfg.Rclone.Remote + ":" + filepath.ToSlash(s.cfg.Rclone.RemotePath)

	switch rec.Mode {
	case config.BackupModeArchive:
		if rec.ArchivePath != "" {
			if _, err := os.Stat(rec.ArchivePath); err == nil {
				return nil
			}
		}
		rel := "archives/" + filepath.Base(rec.ArchivePath)
		if rec.ArchivePath == "" {
			rel = "archives/" + snapshotID + archiveExt(s.cfg.Backup.Compression)
		}
		local := filepath.Join(s.repo.layout.Archives, filepath.Base(rel))
		remote := remoteBase + "/" + filepath.ToSlash(rel)
		if err := s.runner.CopyToLocal(ctx, remote, local); err != nil {
			return err
		}
		_, err = s.repo.db.Exec(`UPDATE snapshots SET archive_path=? WHERE id=?`, local, snapshotID)
		return err
	case config.BackupModeIncremental:
		return s.fetchIncrementalObjects(ctx, snapshotID, remoteBase)
	}
	return fmt.Errorf("unsupported mode: %s", rec.Mode)
}

func (s *Syncer) fetchIncrementalObjects(ctx context.Context, snapshotID, remoteBase string) error {
	entries, err := s.repo.LoadEntries(snapshotID)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsChunked {
			for _, ch := range e.ChunkHashes {
				if err := s.fetchChunk(ctx, ch, remoteBase); err != nil {
					return err
				}
			}
		} else if e.ObjectHash != "" {
			if err := s.fetchObject(ctx, e.ObjectHash, remoteBase); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Syncer) fetchObject(ctx context.Context, hash, remoteBase string) error {
	var localPath string
	var localPresent int
	err := s.repo.db.QueryRow(`SELECT local_path, local_present FROM objects WHERE hash=?`, hash).Scan(&localPath, &localPresent)
	if err == nil && localPresent == 1 {
		if _, err := os.Stat(localPath); err == nil {
			return nil
		}
	}
	rel := "repo/objects/" + hash[0:2] + "/" + hash[2:4] + "/" + hash + objectExt(s.cfg.Backup.Compression)
	localPath = objectPath(s.repo.layout.Objects, hash) + objectExt(s.cfg.Backup.Compression)
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	remote := remoteBase + "/" + filepath.ToSlash(rel)
	if err := s.runner.CopyToLocal(ctx, remote, localPath); err != nil {
		// try alternate path with basename from DB
		var remotePath string
		_ = s.repo.db.QueryRow(`SELECT remote_path FROM objects WHERE hash=?`, hash).Scan(&remotePath)
		if remotePath != "" {
			remote = s.cfg.Rclone.Remote + ":" + filepath.ToSlash(remotePath)
			if err2 := s.runner.CopyToLocal(ctx, remote, localPath); err2 != nil {
				return err
			}
		} else {
			return err
		}
	}
	_, err = s.repo.db.Exec(`UPDATE objects SET local_path=?, local_present=1 WHERE hash=?`, localPath, hash)
	return err
}

func (s *Syncer) fetchChunk(ctx context.Context, hash, remoteBase string) error {
	var localPath string
	var localPresent int
	err := s.repo.db.QueryRow(`SELECT local_path, local_present FROM chunks WHERE hash=?`, hash).Scan(&localPath, &localPresent)
	if err == nil && localPresent == 1 {
		if _, err := os.Stat(localPath); err == nil {
			return nil
		}
	}
	rel := "repo/chunks/" + hash[0:2] + "/" + hash[2:4] + "/" + hash + objectExt(s.cfg.Backup.Compression)
	localPath = objectPath(s.repo.layout.Chunks, hash) + objectExt(s.cfg.Backup.Compression)
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	remote := remoteBase + "/" + filepath.ToSlash(rel)
	if err := s.runner.CopyToLocal(ctx, remote, localPath); err != nil {
		var remotePath string
		_ = s.repo.db.QueryRow(`SELECT remote_path FROM chunks WHERE hash=?`, hash).Scan(&remotePath)
		if remotePath != "" {
			remote = s.cfg.Rclone.Remote + ":" + filepath.ToSlash(strings.TrimPrefix(remotePath, "/"))
			if err2 := s.runner.CopyToLocal(ctx, remote, localPath); err2 != nil {
				return err
			}
		} else {
			return err
		}
	}
	_, err = s.repo.db.Exec(`UPDATE chunks SET local_path=?, local_present=1 WHERE hash=?`, localPath, hash)
	return err
}
