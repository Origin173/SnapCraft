package cli

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/Origin173/SnapCraft/internal/rclone"
	"github.com/spf13/cobra"
)

var rcloneCmd = &cobra.Command{
	Use:   "rclone",
	Short: "Manage embedded rclone remotes",
}

var rcloneListCmd = &cobra.Command{
	Use:   "list",
	Short: "List configured rclone remotes",
	RunE: func(cmd *cobra.Command, args []string) error {
		remotes, err := rclone.ListRemotes()
		if err != nil {
			return err
		}
		if len(remotes) == 0 {
			fmt.Println("No rclone remotes configured.")
			return nil
		}
		for _, name := range remotes {
			fmt.Println(name)
		}
		return nil
	},
}

var rcloneShowCmd = &cobra.Command{
	Use:   "show <name>",
	Short: "Show configuration for a remote",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := rclone.ShowRemote(args[0])
		if err != nil {
			return err
		}
		out, err := rclone.FormatRemoteConfig(cfg)
		if err != nil {
			return err
		}
		fmt.Println(out)
		return nil
	},
}

var rcloneCreateCmd = &cobra.Command{
	Use:   "create <name> <type> [key=value...]",
	Short: "Create a new rclone remote",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		params, err := rclone.ParseKeyValues(args[2:])
		if err != nil {
			return err
		}
		if err := rclone.CreateRemote(args[0], args[1], params); err != nil {
			return err
		}
		fmt.Printf("Created remote %q (%s).\n", args[0], args[1])
		return nil
	},
}

var rcloneUpdateCmd = &cobra.Command{
	Use:   "update <name> [key=value...]",
	Short: "Update an existing rclone remote",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		params, err := rclone.ParseKeyValues(args[1:])
		if err != nil {
			return err
		}
		if len(params) == 0 {
			return fmt.Errorf("at least one key=value parameter is required")
		}
		if err := rclone.UpdateRemote(args[0], params); err != nil {
			return err
		}
		fmt.Printf("Updated remote %q.\n", args[0])
		return nil
	},
}

var rcloneDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a configured rclone remote",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !rcloneDeleteConfirm {
			fmt.Printf("Dry run: remote %q would be deleted. Use --yes to confirm.\n", args[0])
			return nil
		}
		if err := rclone.DeleteRemote(args[0]); err != nil {
			return err
		}
		fmt.Printf("Deleted remote %q.\n", args[0])
		return nil
	},
}

var rcloneDeleteConfirm bool

var rcloneProvidersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List supported rclone storage providers",
	RunE: func(cmd *cobra.Command, args []string) error {
		providers, err := rclone.ListProviders()
		if err != nil {
			return err
		}
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TYPE\tDESCRIPTION")
		for _, p := range providers {
			fmt.Fprintf(w, "%s\t%s\n", p.Name, strings.TrimSpace(p.Description))
		}
		return w.Flush()
	},
}

func init() {
	rcloneDeleteCmd.Flags().BoolVar(&rcloneDeleteConfirm, "yes", false, "confirm deletion")
	rcloneCmd.AddCommand(rcloneListCmd)
	rcloneCmd.AddCommand(rcloneShowCmd)
	rcloneCmd.AddCommand(rcloneCreateCmd)
	rcloneCmd.AddCommand(rcloneUpdateCmd)
	rcloneCmd.AddCommand(rcloneDeleteCmd)
	rcloneCmd.AddCommand(rcloneProvidersCmd)
}
