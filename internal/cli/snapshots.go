package cli

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/repository"
	"github.com/spf13/cobra"
)

var snapshotsCmd = &cobra.Command{
	Use:     "snapshots",
	Aliases: []string{"snapshot"},
	Short:   "Snapshot management",
}

var snapshotsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List backup snapshots",
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

		snaps, err := repo.ListSnapshots()
		if err != nil {
			return err
		}
		if len(snaps) == 0 {
			fmt.Println("No snapshots found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tMODE\tLOCAL\tREMOTE\tBYTES\tSTARTED")
		for _, m := range snaps {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%s\n",
				m.ID, m.Mode, m.LocalStatus, m.RemoteStatus, m.TotalBytes, m.StartedAt.Format(time.RFC3339))
		}
		return w.Flush()
	},
}

func init() {
	snapshotsCmd.AddCommand(snapshotsListCmd)
}
