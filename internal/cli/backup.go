package cli

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Origin173/SnapCraft/internal/backup"
	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/minecraft"
	"github.com/Origin173/SnapCraft/internal/notify"
	"github.com/Origin173/SnapCraft/internal/rclone"
	"github.com/spf13/cobra"
)

var (
	backupWorld string
	backupName  string
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup commands",
}

var backupRunCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a backup now",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		if backupWorld != "" {
			cfg.Server.WorldPath = backupWorld
			cfg.Server.Control.Type = config.ControlNone
		}
		if backupName != "" {
			cfg.Server.Name = backupName
		}
		if err := rclone.EnsureRemoteConfigured(cfg); err != nil {
			return err
		}

		mc, err := minecraft.NewController(cfg)
		if err != nil {
			return err
		}
		defer mc.Close()

		runner := rclone.NewRunner(cfg)
		notifier := notify.BuildFromConfig(cfg)
		svc, err := backup.NewService(cfg, mc, runner, notifier)
		if err != nil {
			return err
		}
		defer svc.Close()

		ctx := context.Background()
		result, err := svc.Run(ctx)
		if err != nil {
			return err
		}
		slog.Info("backup completed", "snapshot_id", result.Manifest.ID, "bytes", result.Manifest.TotalBytes)
		fmt.Printf("Backup completed: %s (%d bytes)\n", result.Manifest.ID, result.Manifest.TotalBytes)
		if result.Manifest.Error != "" {
			fmt.Printf("Remote sync note: %s\n", result.Manifest.Error)
		}
		return nil
	},
}

func init() {
	backupRunCmd.Flags().StringVar(&backupWorld, "world", "", "world path override (enables offline/singleplayer mode)")
	backupRunCmd.Flags().StringVar(&backupName, "name", "", "server/world name override")
	backupCmd.AddCommand(backupRunCmd)
}
