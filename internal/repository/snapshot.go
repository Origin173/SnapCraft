package repository

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/repository/chunker"
	"github.com/klauspost/compress/zstd"
)

const (
	SnapshotStatusRunning         = "running"
	SnapshotStatusCompletedLocal  = "completed_local"
	SnapshotStatusCompletedRemote = "completed_remote"
	SnapshotStatusFailed          = "failed"

	EntryTypeFile = "file"
	EntryTypeDir  = "dir"
	EntryTypeLink = "symlink"
)

// SnapshotRecord represents a snapshot in the local repository.
type SnapshotRecord struct {
	ID           string
	ServerName   string
	WorldPath    string
	Mode         string
	Compression  string
	Status       string
	LocalStatus  string
	RemoteStatus string
	ArchivePath  string
	ArchiveHash  string
	ArchiveSize  int64
	FileCount    int64
	TotalBytes   int64
	StartedAt    time.Time
	CompletedAt  time.Time
	Error        string
	Restorable   bool
}

// EntryRecord is a file tree entry in a snapshot.
type EntryRecord struct {
	RelPath       string
	EntryType     string
	Mode          os.FileMode
	Mtime         time.Time
	Size          int64
	ObjectHash    string
	IsChunked     bool
	SymlinkTarget string
	ChunkHashes   []string
}

func (r *Repository) CreateSnapshot(id, mode string) (*SnapshotRecord, error) {
	now := time.Now().UTC()
	rec := &SnapshotRecord{
		ID:           id,
		ServerName:   r.cfg.Server.Name,
		WorldPath:    r.cfg.Server.WorldPath,
		Mode:         mode,
		Compression:  r.cfg.Backup.Compression,
		Status:       SnapshotStatusRunning,
		LocalStatus:  SnapshotStatusRunning,
		RemoteStatus: "pending",
		StartedAt:    now,
	}
	_, err := r.db.Exec(`
		INSERT INTO snapshots(id, server_name, world_path, mode, compression, status, local_status, remote_status, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.ServerName, rec.WorldPath, rec.Mode, rec.Compression,
		rec.Status, rec.LocalStatus, rec.RemoteStatus, rec.StartedAt.Format(time.RFC3339Nano),
	)
	return rec, err
}

func (r *Repository) MarkSnapshotLocalComplete(id string, fileCount, totalBytes int64, archivePath, archiveHash string, archiveSize int64) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.Exec(`
		UPDATE snapshots SET status=?, local_status=?, file_count=?, total_bytes=?, archive_path=?, archive_hash=?, archive_size=?, completed_at=?, restorable=1
		WHERE id=?`,
		SnapshotStatusCompletedLocal, SnapshotStatusCompletedLocal,
		fileCount, totalBytes, archivePath, archiveHash, archiveSize, now, id,
	)
	return err
}

func (r *Repository) MarkSnapshotFailed(id string, errMsg string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.Exec(`
		UPDATE snapshots SET status=?, local_status=?, error=?, completed_at=?, restorable=0 WHERE id=?`,
		SnapshotStatusFailed, SnapshotStatusFailed, errMsg, now, id,
	)
	return err
}

func (r *Repository) MarkSnapshotRemoteComplete(id string) error {
	_, err := r.db.Exec(`
		UPDATE snapshots SET status=?, remote_status=? WHERE id=?`,
		SnapshotStatusCompletedRemote, SnapshotStatusCompletedRemote, id,
	)
	return err
}

func (r *Repository) MarkSnapshotRemoteFailed(id, errMsg string) error {
	_, err := r.db.Exec(`UPDATE snapshots SET remote_status=? WHERE id=?`, "failed: "+errMsg, id)
	return err
}

func (r *Repository) GetSnapshot(id string) (*SnapshotRecord, error) {
	row := r.db.QueryRow(`
		SELECT id, server_name, world_path, mode, compression, status, local_status, remote_status,
		       COALESCE(archive_path,''), COALESCE(archive_hash,''), COALESCE(archive_size,0),
		       COALESCE(file_count,0), COALESCE(total_bytes,0), started_at,
		       COALESCE(completed_at,''), COALESCE(error,''), restorable
		FROM snapshots WHERE id=?`, id)

	var rec SnapshotRecord
	var started, completed string
	var restorable int
	if err := row.Scan(
		&rec.ID, &rec.ServerName, &rec.WorldPath, &rec.Mode, &rec.Compression,
		&rec.Status, &rec.LocalStatus, &rec.RemoteStatus,
		&rec.ArchivePath, &rec.ArchiveHash, &rec.ArchiveSize,
		&rec.FileCount, &rec.TotalBytes, &started, &completed, &rec.Error, &restorable,
	); err != nil {
		return nil, err
	}
	rec.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
	if completed != "" {
		rec.CompletedAt, _ = time.Parse(time.RFC3339Nano, completed)
	}
	rec.Restorable = restorable == 1
	return &rec, nil
}

func (r *Repository) ListSnapshots() ([]*SnapshotRecord, error) {
	rows, err := r.db.Query(`
		SELECT id, server_name, world_path, mode, compression, status, local_status, remote_status,
		       COALESCE(archive_path,''), COALESCE(archive_hash,''), COALESCE(archive_size,0),
		       COALESCE(file_count,0), COALESCE(total_bytes,0), started_at,
		       COALESCE(completed_at,''), COALESCE(error,''), restorable
		FROM snapshots ORDER BY started_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*SnapshotRecord
	for rows.Next() {
		var rec SnapshotRecord
		var started, completed string
		var restorable int
		if err := rows.Scan(
			&rec.ID, &rec.ServerName, &rec.WorldPath, &rec.Mode, &rec.Compression,
			&rec.Status, &rec.LocalStatus, &rec.RemoteStatus,
			&rec.ArchivePath, &rec.ArchiveHash, &rec.ArchiveSize,
			&rec.FileCount, &rec.TotalBytes, &started, &completed, &rec.Error, &restorable,
		); err != nil {
			return nil, err
		}
		rec.StartedAt, _ = time.Parse(time.RFC3339Nano, started)
		if completed != "" {
			rec.CompletedAt, _ = time.Parse(time.RFC3339Nano, completed)
		}
		rec.Restorable = restorable == 1
		out = append(out, &rec)
	}
	return out, rows.Err()
}

func (r *Repository) StoreArchive(ctx context.Context, snapshotID, srcPath string) (string, string, int64, error) {
	ext := archiveExt(r.cfg.Backup.Compression)
	name := snapshotID + ext
	dest := filepath.Join(r.layout.Archives, name)
	if err := copyFileAtomic(srcPath, dest); err != nil {
		return "", "", 0, err
	}
	hasher := NewHasher(r.cfg.Backup.HashMethod)
	hash, size, err := hasher.SumFile(dest)
	if err != nil {
		return "", "", 0, err
	}
	return dest, hash, size, nil
}

func (r *Repository) VerifyArchive(path, expectedHash string, expectedSize int64) error {
	hasher := NewHasher(r.cfg.Backup.HashMethod)
	hash, size, err := hasher.SumFile(path)
	if err != nil {
		return err
	}
	if expectedSize > 0 && size != expectedSize {
		return fmt.Errorf("archive size mismatch: got %d want %d", size, expectedSize)
	}
	if expectedHash != "" && hash != expectedHash {
		return fmt.Errorf("archive hash mismatch: got %s want %s", hash, expectedHash)
	}
	return nil
}

func archiveExt(compression string) string {
	switch compression {
	case config.CompressionZstd:
		return ".tar.zst"
	case config.CompressionGzip:
		return ".tar.gz"
	default:
		return ".tar"
	}
}

func copyFileAtomic(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(dest)
		return err
	}
	return out.Close()
}

func (r *Repository) ScanAndStoreIncremental(ctx context.Context, snapshotID, rootDir string) (int64, int64, error) {
	hasher := NewHasher(r.cfg.Backup.HashMethod)
	cdcCfg := chunker.Config{
		MinSize:     int(r.cfg.Backup.CDC.MinSize),
		AvgSize:     int(r.cfg.Backup.CDC.AvgSize),
		MaxSize:     int(r.cfg.Backup.CDC.MaxSize),
		MinFileSize: r.cfg.Backup.CDC.MinFileSize,
	}
	var fileCount, totalBytes int64

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(rootDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if shouldExclude(rel, r.cfg.Backup.ExcludePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		entry := EntryRecord{
			RelPath:   rel,
			Mode:      info.Mode(),
			Mtime:     info.ModTime(),
			Size:      info.Size(),
			EntryType: EntryTypeFile,
		}
		if info.IsDir() {
			entry.EntryType = EntryTypeDir
			return r.insertEntry(snapshotID, entry)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			entry.EntryType = EntryTypeLink
			entry.SymlinkTarget = target
			return r.insertEntry(snapshotID, entry)
		}

		useCDC := r.cfg.Backup.CDC.Enabled && info.Size() >= r.cfg.Backup.CDC.MinFileSize
		if useCDC {
			chunks, err := chunker.SplitFile(path, cdcCfg, r.cfg.Backup.HashMethod)
			if err != nil {
				return err
			}
			if len(chunks) > 0 {
				entry.IsChunked = true
				for i, ch := range chunks {
					if err := r.storeChunk(ch.Hash, ch.Data); err != nil {
						return err
					}
					entry.ChunkHashes = append(entry.ChunkHashes, ch.Hash)
					if _, err := r.db.Exec(`
						INSERT OR IGNORE INTO file_chunks(snapshot_id, rel_path, chunk_index, chunk_hash)
						VALUES (?, ?, ?, ?)`, snapshotID, rel, i, ch.Hash); err != nil {
						return err
					}
					totalBytes += ch.Size
				}
				fileCount++
				return r.insertEntry(snapshotID, entry)
			}
		}

		hash, size, err := hasher.SumFile(path)
		if err != nil {
			return err
		}
		entry.ObjectHash = hash
		entry.Size = size
		if err := r.storeObjectFromFile(hash, path); err != nil {
			return err
		}
		fileCount++
		totalBytes += size
		return r.insertEntry(snapshotID, entry)
	})
	return fileCount, totalBytes, err
}

func shouldExclude(rel string, patterns []string) bool {
	for _, p := range patterns {
		if matched, _ := filepath.Match(p, rel); matched {
			return true
		}
		if strings.Contains(rel, p) {
			return true
		}
	}
	return false
}

func (r *Repository) insertEntry(snapshotID string, e EntryRecord) error {
	_, err := r.db.Exec(`
		INSERT INTO entries(snapshot_id, rel_path, entry_type, mode, mtime_ns, size, object_hash, is_chunked, symlink_target)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		snapshotID, e.RelPath, e.EntryType, int(e.Mode), e.Mtime.UnixNano(), e.Size,
		nullString(e.ObjectHash), boolInt(e.IsChunked), nullString(e.SymlinkTarget),
	)
	return err
}

func (r *Repository) storeObjectFromFile(hash, path string) error {
	var exists int
	if err := r.db.QueryRow(`SELECT 1 FROM objects WHERE hash=?`, hash).Scan(&exists); err == nil {
		_, err = r.db.Exec(`UPDATE objects SET ref_count = ref_count + 1 WHERE hash=?`, hash)
		return err
	}
	compressed, size, err := r.compressFile(path)
	if err != nil {
		return err
	}
	localPath := objectPath(r.layout.Objects, hash) + objectExt(r.cfg.Backup.Compression)
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(localPath, compressed, 0o644); err != nil {
		return err
	}
	_, err = r.db.Exec(`
		INSERT INTO objects(hash, size, compression, local_path, ref_count, local_present)
		VALUES (?, ?, ?, ?, 1, 1)`,
		hash, size, r.cfg.Backup.Compression, localPath,
	)
	return err
}

func (r *Repository) storeChunk(hash string, data []byte) error {
	var exists int
	if err := r.db.QueryRow(`SELECT 1 FROM chunks WHERE hash=?`, hash).Scan(&exists); err == nil {
		_, err = r.db.Exec(`UPDATE chunks SET ref_count = ref_count + 1 WHERE hash=?`, hash)
		return err
	}
	compressed, err := compressBytes(data, r.cfg.Backup.Compression)
	if err != nil {
		return err
	}
	localPath := objectPath(r.layout.Chunks, hash) + objectExt(r.cfg.Backup.Compression)
	if err := os.MkdirAll(filepath.Dir(localPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(localPath, compressed, 0o644); err != nil {
		return err
	}
	_, err = r.db.Exec(`
		INSERT INTO chunks(hash, size, compression, local_path, ref_count, local_present)
		VALUES (?, ?, ?, ?, 1, 1)`,
		hash, int64(len(data)), r.cfg.Backup.Compression, localPath,
	)
	return err
}

func (r *Repository) compressFile(path string) ([]byte, int64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	out, err := compressBytes(data, r.cfg.Backup.Compression)
	return out, int64(len(data)), err
}

func compressBytes(data []byte, compression string) ([]byte, error) {
	switch compression {
	case config.CompressionNone:
		return data, nil
	case config.CompressionGzip:
		// simplified: store raw for now with gzip wrapper via temp buffer
		return gzipCompress(data)
	case config.CompressionZstd:
		enc, err := zstd.NewWriter(nil)
		if err != nil {
			return nil, err
		}
		return enc.EncodeAll(data, nil), nil
	default:
		return data, nil
	}
}

func gzipCompress(data []byte) ([]byte, error) {
	// use std gzip via pipe-less approach
	tmp, err := os.CreateTemp("", "gzip-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmp.Name())
	gw := gzip.NewWriter(tmp)
	if _, err := gw.Write(data); err != nil {
		gw.Close()
		tmp.Close()
		return nil, err
	}
	if err := gw.Close(); err != nil {
		tmp.Close()
		return nil, err
	}
	if _, err := tmp.Seek(0, 0); err != nil {
		tmp.Close()
		return nil, err
	}
	out, err := io.ReadAll(tmp)
	tmp.Close()
	return out, err
}

func objectExt(compression string) string {
	switch compression {
	case config.CompressionZstd:
		return ".zst"
	case config.CompressionGzip:
		return ".gz"
	default:
		return ""
	}
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (r *Repository) LoadEntries(snapshotID string) ([]EntryRecord, error) {
	rows, err := r.db.Query(`
		SELECT rel_path, entry_type, mode, mtime_ns, size, COALESCE(object_hash,''), is_chunked, COALESCE(symlink_target,'')
		FROM entries WHERE snapshot_id=? ORDER BY rel_path`, snapshotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []EntryRecord
	for rows.Next() {
		var e EntryRecord
		var mode int
		var mtime int64
		var isChunked int
		if err := rows.Scan(&e.RelPath, &e.EntryType, &mode, &mtime, &e.Size, &e.ObjectHash, &isChunked, &e.SymlinkTarget); err != nil {
			return nil, err
		}
		e.Mode = os.FileMode(mode)
		e.Mtime = time.Unix(0, mtime)
		e.IsChunked = isChunked == 1
		if e.IsChunked {
			chRows, err := r.db.Query(`
				SELECT chunk_hash FROM file_chunks WHERE snapshot_id=? AND rel_path=? ORDER BY chunk_index`,
				snapshotID, e.RelPath)
			if err != nil {
				return nil, err
			}
			for chRows.Next() {
				var h string
				if err := chRows.Scan(&h); err != nil {
					chRows.Close()
					return nil, err
				}
				e.ChunkHashes = append(e.ChunkHashes, h)
			}
			chRows.Close()
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

func (r *Repository) ReadObject(hash string) ([]byte, error) {
	var localPath, compression string
	var localPresent int
	err := r.db.QueryRow(`SELECT local_path, compression, local_present FROM objects WHERE hash=?`, hash).
		Scan(&localPath, &compression, &localPresent)
	if err != nil {
		return nil, fmt.Errorf("object %s: %w", hash, err)
	}
	if localPresent == 0 {
		return nil, fmt.Errorf("object %s not present locally; use --remote to fetch", hash)
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, err
	}
	return decompressBytes(data, compression)
}

func (r *Repository) ReadChunk(hash string) ([]byte, error) {
	var localPath, compression string
	var localPresent int
	err := r.db.QueryRow(`SELECT local_path, compression, local_present FROM chunks WHERE hash=?`, hash).
		Scan(&localPath, &compression, &localPresent)
	if err != nil {
		return nil, fmt.Errorf("chunk %s: %w", hash, err)
	}
	if localPresent == 0 {
		return nil, fmt.Errorf("chunk %s not present locally; use --remote to fetch", hash)
	}
	data, err := os.ReadFile(localPath)
	if err != nil {
		return nil, err
	}
	return decompressBytes(data, compression)
}

func decompressBytes(data []byte, compression string) ([]byte, error) {
	switch compression {
	case config.CompressionNone, "":
		return data, nil
	case config.CompressionZstd:
		dec, err := zstd.NewReader(nil)
		if err != nil {
			return nil, err
		}
		defer dec.Close()
		return dec.DecodeAll(data, nil)
	case config.CompressionGzip:
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		return io.ReadAll(gr)
	default:
		return data, nil
	}
}

func (r *Repository) RestoreIncremental(snapshotID, destRoot string) error {
	entries, err := r.LoadEntries(snapshotID)
	if err != nil {
		return err
	}
	for _, e := range entries {
		target := filepath.Join(destRoot, filepath.FromSlash(e.RelPath))
		switch e.EntryType {
		case EntryTypeDir:
			if err := os.MkdirAll(target, e.Mode); err != nil {
				return err
			}
		case EntryTypeLink:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			_ = os.Remove(target)
			if err := os.Symlink(e.SymlinkTarget, target); err != nil {
				return err
			}
		case EntryTypeFile:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			var data []byte
			if e.IsChunked {
				for _, ch := range e.ChunkHashes {
					part, err := r.ReadChunk(ch)
					if err != nil {
						return err
					}
					data = append(data, part...)
				}
			} else {
				var err error
				data, err = r.ReadObject(e.ObjectHash)
				if err != nil {
					return err
				}
			}
			if err := os.WriteFile(target, data, e.Mode); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *Repository) VerifySnapshotLocal(snapshotID string) error {
	rec, err := r.GetSnapshot(snapshotID)
	if err != nil {
		return err
	}
	switch rec.Mode {
	case config.BackupModeArchive:
		if rec.ArchivePath == "" {
			return fmt.Errorf("snapshot %s has no archive path", snapshotID)
		}
		return r.VerifyArchive(rec.ArchivePath, rec.ArchiveHash, rec.ArchiveSize)
	case config.BackupModeIncremental:
		entries, err := r.LoadEntries(snapshotID)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if e.EntryType != EntryTypeFile {
				continue
			}
			if e.IsChunked {
				for _, ch := range e.ChunkHashes {
					if _, err := r.ReadChunk(ch); err != nil {
						return err
					}
				}
			} else if e.ObjectHash != "" {
				if _, err := r.ReadObject(e.ObjectHash); err != nil {
					return err
				}
			}
		}
		return nil
	default:
		return nil
	}
}

func (r *Repository) CleanupLocalPayload(snapshotID string) error {
	rec, err := r.GetSnapshot(snapshotID)
	if err != nil {
		return err
	}
	if rec.Mode == config.BackupModeArchive && rec.ArchivePath != "" && !r.cfg.Repository.KeepLocalManifests {
		_ = os.Remove(rec.ArchivePath)
	}
	// Mark objects/chunks as not locally present if ref_count allows and uploaded
	return nil
}
