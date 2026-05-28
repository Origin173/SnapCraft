package repository

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Origin173/SnapCraft/internal/config"
	_ "modernc.org/sqlite"
)

const schemaVersion = 1

// Layout defines on-disk repository paths.
type Layout struct {
	Root      string
	DB        string
	Objects   string
	Chunks    string
	Archives  string
	Manifests string
	Staging   string
}

// Repository is the local backup store.
type Repository struct {
	cfg    *config.Config
	layout Layout
	db     *sql.DB
}

// Open opens or creates a local repository.
func Open(cfg *config.Config) (*Repository, error) {
	root, err := filepath.Abs(cfg.Repository.LocalPath)
	if err != nil {
		return nil, err
	}
	layout := Layout{
		Root:      root,
		DB:        filepath.Join(root, "snapcraft.db"),
		Objects:   filepath.Join(root, "objects"),
		Chunks:    filepath.Join(root, "chunks"),
		Archives:  filepath.Join(root, "archives"),
		Manifests: filepath.Join(root, "manifests"),
		Staging:   filepath.Join(root, "staging"),
	}
	for _, dir := range []string{layout.Root, layout.Objects, layout.Chunks, layout.Archives, layout.Manifests, layout.Staging} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create repo dir %s: %w", dir, err)
		}
	}

	db, err := sql.Open("sqlite", layout.DB+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	repo := &Repository{cfg: cfg, layout: layout, db: db}
	if err := repo.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return repo, nil
}

func (r *Repository) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

func (r *Repository) Layout() Layout {
	return r.layout
}

func (r *Repository) DB() *sql.DB {
	return r.db
}

func (r *Repository) Config() *config.Config {
	return r.cfg
}

// Init creates repository directories and database without opening a long-lived handle.
func Init(cfg *config.Config) error {
	repo, err := Open(cfg)
	if err != nil {
		return err
	}
	return repo.Close()
}
