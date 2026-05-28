package cli

import (
	"fmt"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Configuration commands",
}

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		if checkPaths, _ := cmd.Flags().GetBool("check-paths"); checkPaths {
			if err := config.ValidatePaths(cfg); err != nil {
				return err
			}
		}
		fmt.Printf("Configuration valid: server=%s mode=%s remote=%s\n",
			cfg.Server.Name, cfg.Backup.Mode, cfg.Rclone.Remote)
		return nil
	},
}

func init() {
	configValidateCmd.Flags().Bool("check-paths", false, "verify world path and staging dir exist")
	configCmd.AddCommand(configValidateCmd)
}
