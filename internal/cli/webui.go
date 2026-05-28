package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/webui"
	"github.com/spf13/cobra"
)

var (
	webuiFlag bool
	webuiAddr string
)

func init() {
	RootCmd.PersistentFlags().BoolVar(&webuiFlag, "webui", false, "start the WebUI server")
	RootCmd.PersistentFlags().StringVar(&webuiAddr, "webui-addr", "", "WebUI listen address (overrides config)")

	RootCmd.RunE = runRoot
}

func runRoot(cmd *cobra.Command, args []string) error {
	if !webuiFlag {
		return cmd.Help()
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	cfg.WebUI.Enabled = true
	if webuiAddr != "" {
		cfg.WebUI.Addr = webuiAddr
	}

	s, err := webui.NewServer(cfg, configPath)
	if err != nil {
		return err
	}

	fmt.Printf("SnapCraft WebUI: %s\n", webuiHint(cfg.WebUI.Addr))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return s.Run(ctx)
}

func webuiHint(addr string) string {
	return fmt.Sprintf("http://%s", addr)
}
