package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Origin173/SnapCraft/internal/backup"
	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/minecraft"
	"github.com/Origin173/SnapCraft/internal/notify"
	"github.com/Origin173/SnapCraft/internal/rclone"
	"github.com/Origin173/SnapCraft/internal/scheduler"
	"github.com/spf13/cobra"
)

var scheduleCmd = &cobra.Command{
	Use:   "schedule",
	Short: "Scheduled backup commands",
}

var scheduleRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the backup scheduler daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		if !cfg.Schedule.Enabled {
			return fmt.Errorf("schedule.enabled is false in config")
		}

		mc, err := minecraft.NewController(cfg)
		if err != nil {
			return err
		}
		defer mc.Close()

		if err := rclone.EnsureRemoteConfigured(cfg); err != nil {
			return err
		}

		runner := rclone.NewRunner(cfg)
		notifier := notify.BuildFromConfig(cfg)
		backupSvc, err := backup.NewService(cfg, mc, runner, notifier)
		if err != nil {
			return err
		}
		defer backupSvc.Close()
		sched := scheduler.New(cfg, backupSvc)

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		slog.Info("scheduler started", "cron", cfg.Schedule.Cron)
		return sched.Run(ctx)
	},
}

func init() {
	scheduleCmd.AddCommand(scheduleRunCmd)
}
