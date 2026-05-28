package minecraft

import "context"

// NoopController skips all server commands for offline/singleplayer backups.
type NoopController struct{}

func NewNoopController() *NoopController {
	return &NoopController{}
}

func (n *NoopController) SaveOff(ctx context.Context) error  { return nil }
func (n *NoopController) SaveAll(ctx context.Context) error  { return nil }
func (n *NoopController) SaveOn(ctx context.Context) error   { return nil }
func (n *NoopController) Say(ctx context.Context, message string) error { return nil }
func (n *NoopController) Close() error                         { return nil }
