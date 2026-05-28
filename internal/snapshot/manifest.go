package snapshot

import (
	"encoding/json"
	"time"

	"github.com/Origin173/SnapCraft/internal/config"
)

const (
	StatusPending   = "pending"
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Manifest describes a backup snapshot stored remotely.
type Manifest struct {
	ID              string    `json:"id"`
	ServerName      string    `json:"server_name"`
	WorldPath       string    `json:"world_path"`
	Mode            string    `json:"mode"`
	Compression     string    `json:"compression"`
	Remote          string    `json:"remote"`
	RemotePath      string    `json:"remote_path"`
	ArchivePath     string    `json:"archive_path,omitempty"`
	DirectoryPath   string    `json:"directory_path,omitempty"`
	HistoryPath     string    `json:"history_path,omitempty"`
	FileCount       int64     `json:"file_count"`
	TotalBytes      int64     `json:"total_bytes"`
	StartedAt       time.Time `json:"started_at"`
	CompletedAt     time.Time `json:"completed_at,omitempty"`
	Status          string    `json:"status"`
	ControlType     string    `json:"control_type"`
	RcloneSummary   string    `json:"rclone_summary,omitempty"`
	Error           string    `json:"error,omitempty"`
	Restorable      bool      `json:"restorable"`
}

func NewManifest(cfg *config.Config, id string) *Manifest {
	return &Manifest{
		ID:          id,
		ServerName:  cfg.Server.Name,
		WorldPath:   cfg.Server.WorldPath,
		Mode:        cfg.Backup.Mode,
		Compression: cfg.Backup.Compression,
		Remote:      cfg.Rclone.Remote,
		RemotePath:  cfg.Rclone.RemotePath,
		StartedAt:   time.Now().UTC(),
		Status:      StatusRunning,
		ControlType: cfg.Server.Control.Type,
		Restorable:  false,
	}
}

func (m *Manifest) MarkCompleted() {
	m.Status = StatusCompleted
	m.CompletedAt = time.Now().UTC()
	m.Restorable = true
}

func (m *Manifest) MarkFailed(err error) {
	m.Status = StatusFailed
	m.CompletedAt = time.Now().UTC()
	if err != nil {
		m.Error = err.Error()
	}
}

func (m *Manifest) JSON() ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}

func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// RemoteLayout returns standard remote paths for a snapshot.
type RemoteLayout struct {
	Base         string
	Manifests    string
	Archives     string
	DirCurrent   string
	DirHistory   string
	Logs         string
}

func RemoteLayoutFor(cfg *config.Config) RemoteLayout {
	base := cfg.Rclone.RemotePath
	return RemoteLayout{
		Base:       base,
		Manifests:  base + "/manifests",
		Archives:   base + "/archives",
		DirCurrent: base + "/directories/current",
		DirHistory: base + "/directories/history",
		Logs:       base + "/logs",
	}
}

func (l RemoteLayout) ManifestPath(id string) string {
	return l.Manifests + "/" + id + ".json"
}

func (l RemoteLayout) ArchivePath(id string, ext string) string {
	return l.Archives + "/" + id + ext
}

func (l RemoteLayout) HistoryPath(id string) string {
	return l.DirHistory + "/" + id
}

func (l RemoteLayout) LogPath(id string) string {
	return l.Logs + "/" + id + ".log"
}
