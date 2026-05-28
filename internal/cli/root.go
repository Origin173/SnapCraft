package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var configPath string

// RootCmd is the top-level snapcraft command.
var RootCmd = &cobra.Command{
	Use:   "snapcraft",
	Short: "SnapCraft - Minecraft server backup tool with rclone",
	Long:  "SnapCraft coordinates Minecraft save states and backs up worlds to cloud storage via rclone.",
}

func init() {
	RootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "config.yaml", "path to configuration file")
	RootCmd.AddCommand(configCmd)
	RootCmd.AddCommand(backupCmd)
	RootCmd.AddCommand(snapshotsCmd)
	RootCmd.AddCommand(restoreCmd)
	RootCmd.AddCommand(scheduleCmd)
	RootCmd.AddCommand(pruneCmd)
	RootCmd.AddCommand(repoCmd)
}

func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
