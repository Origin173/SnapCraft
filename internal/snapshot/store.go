package snapshot

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/rclone"
)

// Store manages snapshot manifests on remote storage via rclone.
type Store struct {
	cfg    *config.Config
	rclone rclone.Runner
	layout RemoteLayout
}

func NewStore(cfg *config.Config, runner rclone.Runner) *Store {
	return &Store{
		cfg:    cfg,
		rclone: runner,
		layout: RemoteLayoutFor(cfg),
	}
}

func (s *Store) UploadManifest(ctx context.Context, manifest *Manifest) error {
	data, err := manifest.JSON()
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.cfg.Backup.StagingDir, "manifest-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	remote := s.remotePath(s.layout.ManifestPath(manifest.ID))
	return s.rclone.Copy(ctx, tmpPath, remote)
}

func (s *Store) List(ctx context.Context) ([]*Manifest, error) {
	remote := s.remotePath(s.layout.Manifests)
	entries, err := s.rclone.ListJSON(ctx, remote)
	if err != nil {
		return nil, err
	}

	var manifests []*Manifest
	for _, e := range entries {
		if e.IsDir || !strings.HasSuffix(e.Name, ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name, ".json")
		m, err := s.Get(ctx, id)
		if err != nil {
			continue
		}
		manifests = append(manifests, m)
	}

	sort.Slice(manifests, func(i, j int) bool {
		return manifests[i].StartedAt.After(manifests[j].StartedAt)
	})
	return manifests, nil
}

func (s *Store) Get(ctx context.Context, id string) (*Manifest, error) {
	remote := s.remotePath(s.layout.ManifestPath(id))
	tmp, err := os.CreateTemp(s.cfg.Backup.StagingDir, "manifest-dl-*.json")
	if err != nil {
		return nil, err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := s.rclone.CopyToLocal(ctx, remote, tmpPath); err != nil {
		return nil, fmt.Errorf("download manifest %s: %w", id, err)
	}
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, err
	}
	return ParseManifest(data)
}

func (s *Store) Delete(ctx context.Context, m *Manifest) error {
	remoteManifest := s.remotePath(s.layout.ManifestPath(m.ID))
	if err := s.rclone.DeleteFile(ctx, remoteManifest); err != nil {
		return err
	}
	if m.ArchivePath != "" {
		if err := s.rclone.DeleteFile(ctx, s.remotePath(m.ArchivePath)); err != nil {
			return err
		}
	}
	if m.HistoryPath != "" {
		if err := s.rclone.Purge(ctx, s.remotePath(m.HistoryPath)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) remotePath(rel string) string {
	return s.cfg.Rclone.Remote + ":" + filepath.ToSlash(rel)
}

func (s *Store) Layout() RemoteLayout {
	return s.layout
}
