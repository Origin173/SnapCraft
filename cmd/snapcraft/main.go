package main

import (
	"log/slog"
	"os"

	"github.com/Origin173/SnapCraft/internal/cli"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cli.Execute()
}
