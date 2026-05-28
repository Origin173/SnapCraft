package webui

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	JobIdle      = "idle"
	JobRunning   = "running"
	JobSucceeded = "succeeded"
	JobFailed    = "failed"
)

// Job describes a long-running WebUI operation.
type Job struct {
	Status      string     `json:"status"`
	Operation   string     `json:"operation"`
	Message     string     `json:"message,omitempty"`
	SnapshotID  string     `json:"snapshot_id,omitempty"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

type JobManager struct {
	mu  sync.Mutex
	job Job
}

func NewJobManager() *JobManager {
	return &JobManager{job: Job{Status: JobIdle}}
}

func (m *JobManager) Current() Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.job
}

func (m *JobManager) Run(ctx context.Context, operation string, fn func(context.Context) (string, error)) error {
	m.mu.Lock()
	if m.job.Status == JobRunning {
		err := fmt.Errorf("operation %q already running", m.job.Operation)
		m.mu.Unlock()
		return err
	}
	now := time.Now().UTC()
	m.job = Job{
		Status:    JobRunning,
		Operation: operation,
		StartedAt: &now,
	}
	m.mu.Unlock()

	msg, err := fn(ctx)
	completed := time.Now().UTC()

	m.mu.Lock()
	defer m.mu.Unlock()
	m.job.CompletedAt = &completed
	m.job.Message = msg
	if err != nil {
		m.job.Status = JobFailed
		m.job.Message = err.Error()
		return err
	}
	m.job.Status = JobSucceeded
	if msg != "" {
		m.job.Message = msg
	}
	return nil
}

func (m *JobManager) RunAsync(operation string, fn func(context.Context) (string, error)) error {
	m.mu.Lock()
	if m.job.Status == JobRunning {
		err := fmt.Errorf("operation %q already running", m.job.Operation)
		m.mu.Unlock()
		return err
	}
	now := time.Now().UTC()
	m.job = Job{
		Status:    JobRunning,
		Operation: operation,
		StartedAt: &now,
	}
	m.mu.Unlock()

	go func() {
		msg, err := fn(context.Background())
		completed := time.Now().UTC()
		m.mu.Lock()
		defer m.mu.Unlock()
		m.job.CompletedAt = &completed
		if err != nil {
			m.job.Status = JobFailed
			m.job.Message = err.Error()
			return
		}
		m.job.Status = JobSucceeded
		m.job.Message = msg
	}()

	return nil
}

func (m *JobManager) SetSnapshotID(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.job.SnapshotID = id
}
