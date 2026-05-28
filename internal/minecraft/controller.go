package minecraft

import (
	"context"
	"fmt"

	"github.com/Origin173/SnapCraft/internal/config"
)

// Controller sends commands to a Minecraft server.
type Controller interface {
	SaveOff(ctx context.Context) error
	SaveAll(ctx context.Context) error
	SaveOn(ctx context.Context) error
	Say(ctx context.Context, message string) error
	Close() error
}

// NewController creates a controller based on config.
func NewController(cfg *config.Config) (Controller, error) {
	switch cfg.Server.Control.Type {
	case config.ControlRCON:
		return NewRCONController(cfg)
	case config.ControlConsole:
		return NewConsoleController(cfg)
	case config.ControlNone:
		return NewNoopController(), nil
	default:
		return nil, fmt.Errorf("unsupported control type: %s", cfg.Server.Control.Type)
	}
}
