package cli

import (
	"fmt"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/repository"
	"github.com/spf13/cobra"
)

var repoCmd = &cobra.Command{
	Use:   "repo",
	Short: "Local repository commands",
}

var repoInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize local backup repository",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		if err := repository.Init(cfg); err != nil {
			return err
		}
		fmt.Printf("Repository initialized at %s\n", cfg.Repository.LocalPath)
		return nil
	},
}

var repoVerifyCmd = &cobra.Command{
	Use:   "verify [snapshot-id]",
	Short: "Verify local repository or a specific snapshot",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		repo, err := repository.Open(cfg)
		if err != nil {
			return err
		}
		defer repo.Close()

		if len(args) == 0 {
			snaps, err := repo.ListSnapshots()
			if err != nil {
				return err
			}
			for _, s := range snaps {
				if err := repo.VerifySnapshotLocal(s.ID); err != nil {
					return fmt.Errorf("snapshot %s: %w", s.ID, err)
				}
			}
			fmt.Printf("Verified %d snapshot(s).\n", len(snaps))
			return nil
		}

		if err := repo.VerifySnapshotLocal(args[0]); err != nil {
			return err
		}
		fmt.Printf("Snapshot %s verified.\n", args[0])
		return nil
	},
}

func init() {
	repoCmd.AddCommand(repoInitCmd)
	repoCmd.AddCommand(repoVerifyCmd)
}
