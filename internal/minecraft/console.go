package minecraft

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Origin173/SnapCraft/internal/config"
)

// ConsoleController writes commands to a server console input pipe.
type ConsoleController struct {
	cfg *config.Config
}

func NewConsoleController(cfg *config.Config) (*ConsoleController, error) {
	return &ConsoleController{cfg: cfg}, nil
}

func (c *ConsoleController) sendCommand(ctx context.Context, cmd string) error {
	inputPath := c.cfg.Server.Control.Console.InputPath
	f, err := os.OpenFile(inputPath, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open console input: %w", err)
	}
	defer f.Close()

	line := cmd
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	if _, err := f.WriteString(line); err != nil {
		return fmt.Errorf("write console command: %w", err)
	}

	// Allow server time to process command.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		return nil
	}
}

func (c *ConsoleController) SaveOff(ctx context.Context) error {
	return c.sendCommand(ctx, "save-off")
}

func (c *ConsoleController) SaveAll(ctx context.Context) error {
	return c.sendCommand(ctx, "save-all flush")
}

func (c *ConsoleController) SaveOn(ctx context.Context) error {
	return c.sendCommand(ctx, "save-on")
}

func (c *ConsoleController) Say(ctx context.Context, message string) error {
	return c.sendCommand(ctx, "say "+message)
}

func (c *ConsoleController) Close() error {
	return nil
}
