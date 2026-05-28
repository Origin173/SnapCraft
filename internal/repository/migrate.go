package repository

import (
	"database/sql"
	"fmt"
)

const migrateSQL = `
CREATE TABLE IF NOT EXISTS schema_version (
  version INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS snapshots (
  id TEXT PRIMARY KEY,
  server_name TEXT NOT NULL,
  world_path TEXT NOT NULL,
  mode TEXT NOT NULL,
  compression TEXT NOT NULL,
  status TEXT NOT NULL,
  local_status TEXT NOT NULL DEFAULT 'pending',
  remote_status TEXT NOT NULL DEFAULT 'pending',
  archive_path TEXT,
  archive_hash TEXT,
  archive_size INTEGER DEFAULT 0,
  file_count INTEGER DEFAULT 0,
  total_bytes INTEGER DEFAULT 0,
  started_at TEXT NOT NULL,
  completed_at TEXT,
  error TEXT,
  restorable INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS entries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  snapshot_id TEXT NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
  rel_path TEXT NOT NULL,
  entry_type TEXT NOT NULL,
  mode INTEGER NOT NULL DEFAULT 0,
  mtime_ns INTEGER NOT NULL DEFAULT 0,
  size INTEGER NOT NULL DEFAULT 0,
  object_hash TEXT,
  is_chunked INTEGER NOT NULL DEFAULT 0,
  symlink_target TEXT,
  UNIQUE(snapshot_id, rel_path)
);

CREATE TABLE IF NOT EXISTS objects (
  hash TEXT PRIMARY KEY,
  size INTEGER NOT NULL,
  compression TEXT NOT NULL,
  local_path TEXT,
  remote_path TEXT,
  ref_count INTEGER NOT NULL DEFAULT 0,
  local_present INTEGER NOT NULL DEFAULT 1,
  uploaded INTEGER NOT NULL DEFAULT 0,
  verified INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS chunks (
  hash TEXT PRIMARY KEY,
  size INTEGER NOT NULL,
  compression TEXT NOT NULL,
  local_path TEXT,
  remote_path TEXT,
  ref_count INTEGER NOT NULL DEFAULT 0,
  local_present INTEGER NOT NULL DEFAULT 1,
  uploaded INTEGER NOT NULL DEFAULT 0,
  verified INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS file_chunks (
  snapshot_id TEXT NOT NULL,
  rel_path TEXT NOT NULL,
  chunk_index INTEGER NOT NULL,
  chunk_hash TEXT NOT NULL REFERENCES chunks(hash),
  PRIMARY KEY (snapshot_id, rel_path, chunk_index)
);

CREATE TABLE IF NOT EXISTS remote_sync (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  snapshot_id TEXT NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
  object_type TEXT NOT NULL,
  object_hash TEXT NOT NULL,
  uploaded INTEGER NOT NULL DEFAULT 0,
  verified INTEGER NOT NULL DEFAULT 0,
  error TEXT,
  UNIQUE(snapshot_id, object_type, object_hash)
);

CREATE INDEX IF NOT EXISTS idx_entries_snapshot ON entries(snapshot_id);
CREATE INDEX IF NOT EXISTS idx_remote_sync_snapshot ON remote_sync(snapshot_id);
`

func (r *Repository) migrate() error {
	if _, err := r.db.Exec(migrateSQL); err != nil {
		return fmt.Errorf("migrate schema: %w", err)
	}
	var version int
	row := r.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`)
	if err := row.Scan(&version); err != nil && err != sql.ErrNoRows {
		return err
	}
	if version == 0 {
		if _, err := r.db.Exec(`INSERT INTO schema_version(version) VALUES (?)`, schemaVersion); err != nil {
			return err
		}
	}
	return nil
}
