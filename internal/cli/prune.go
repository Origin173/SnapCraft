package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/rclone"
	"github.com/Origin173/SnapCraft/internal/retention"
	"github.com/Origin173/SnapCraft/internal/snapshot"
	"github.com/spf13/cobra"
)

var pruneApply bool

var pruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Apply retention policy to remove old snapshots",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath)
		if err != nil {
			return err
		}
		store := snapshot.NewStore(cfg, rclone.NewExecRunner(cfg))
		manifests, err := store.List(context.Background())
		if err != nil {
			return err
		}

		plan := retention.Compute(cfg, manifests)
		if len(plan.Delete) == 0 {
			fmt.Println("No snapshots to prune.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		action := "would delete"
		if pruneApply {
			action = "deleting"
		}
		fmt.Fprintf(w, "%s:\n", action)
		for _, m := range plan.Delete {
			fmt.Fprintf(w, "  %s\t%s\t%d bytes\n", m.ID, m.StartedAt.Format("2006-01-02"), m.TotalBytes)
		}
		w.Flush()

		if !pruneApply {
			fmt.Printf("\nDry run: %d snapshot(s) would be deleted. Use --apply to execute.\n", len(plan.Delete))
			return nil
		}

		ctx := context.Background()
		for _, m := range plan.Delete {
			if err := store.Delete(ctx, m); err != nil {
				return fmt.Errorf("delete %s: %w", m.ID, err)
			}
		}
		fmt.Printf("Pruned %d snapshot(s).\n", len(plan.Delete))
		return nil
	},
}

func init() {
	pruneCmd.Flags().BoolVar(&pruneApply, "apply", false, "actually delete snapshots (default is dry-run)")
}
