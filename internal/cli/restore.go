package cli

import (
	"context"
	"fmt"

	"github.com/Origin173/SnapCraft/internal/backup"
	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/minecraft"
	"github.com/Origin173/SnapCraft/internal/notify"
	"github.com/Origin173/SnapCraft/internal/rclone"
	"github.com/Origin173/SnapCraft/internal/restore"
	"github.com/spf13/cobra"
)

var (
	forceOnline  bool
	restoreRemote bool
	restoreSource string
)

var restoreCmd = &cobra.Command{
	Use:   "restore <snapshot-id>",
	Short: "Restore a snapshot",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}

		source := restore.SourceLocal
		if restoreRemote {
			source = restore.SourceRemote
		}
		if restoreSource != "" {
			source = restoreSource
		}

		if source == restore.SourceRemote {
			if err := rclone.EnsureRemoteConfigured(cfg); err != nil {
				return err
			}
		}

		mc, err := minecraft.NewController(cfg)
		if err != nil {
			return err
		}
		defer mc.Close()

		runner := rclone.NewRunner(cfg)
		notifier := notify.BuildFromConfig(cfg)
		backupSvc, err := backup.NewService(cfg, mc, runner, notifier)
		if err != nil {
			return err
		}
		defer backupSvc.Close()

		svc, err := restore.NewService(cfg, mc, runner, backupSvc, notifier)
		if err != nil {
			return err
		}
		defer svc.Close()

		opts := restore.Options{ForceOnline: forceOnline, Source: source}
		if err := svc.Run(context.Background(), args[0], opts); err != nil {
			return err
		}
		fmt.Printf("Restored snapshot %s successfully from %s.\n", args[0], source)
		return nil
	},
}

func init() {
	restoreCmd.Flags().BoolVar(&forceOnline, "force-online", false, "allow restore while server is running (creates safety backup first)")
	restoreCmd.Flags().BoolVar(&restoreRemote, "remote", false, "restore from remote storage (download missing objects first)")
	restoreCmd.Flags().StringVar(&restoreSource, "source", "", "restore source: local or remote")
}
