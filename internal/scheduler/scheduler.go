package scheduler

import (
	"context"
	"log"
	"log/slog"

	"github.com/robfig/cron/v3"
	"github.com/Origin173/SnapCraft/internal/backup"
	"github.com/Origin173/SnapCraft/internal/config"
)

// Scheduler runs backup jobs on a cron schedule.
type Scheduler struct {
	cfg    *config.Config
	backup *backup.Service
	cron   *cron.Cron
}

func New(cfg *config.Config, backupSvc *backup.Service) *Scheduler {
	return &Scheduler{
		cfg:    cfg,
		backup: backupSvc,
		cron:   cron.New(cron.WithLogger(cron.VerbosePrintfLogger(log.Default()))),
	}
}

// Run starts the scheduler and blocks until context is cancelled.
func (s *Scheduler) Run(ctx context.Context) error {
	if !s.cfg.Schedule.Enabled {
		return nil
	}
	_, err := s.cron.AddFunc(s.cfg.Schedule.Cron, func() {
		slog.Info("scheduled backup starting")
		if _, err := s.backup.Run(ctx); err != nil {
			slog.Error("scheduled backup failed", "error", err)
		} else {
			slog.Info("scheduled backup completed")
		}
	})
	if err != nil {
		return err
	}
	s.cron.Start()
	<-ctx.Done()
	s.cron.Stop()
	return ctx.Err()
}
