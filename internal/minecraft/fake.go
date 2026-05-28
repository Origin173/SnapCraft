package minecraft

import (
	"context"
	"fmt"
	"sync"
)

// FakeController records command calls for testing.
type FakeController struct {
	mu     sync.Mutex
	Calls  []string
	FailOn map[string]error
}

func NewFakeController() *FakeController {
	return &FakeController{FailOn: make(map[string]error)}
}

func (f *FakeController) record(cmd string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.Calls = append(f.Calls, cmd)
	if err, ok := f.FailOn[cmd]; ok {
		return err
	}
	return nil
}

func (f *FakeController) SaveOff(ctx context.Context) error {
	return f.record("save-off")
}

func (f *FakeController) SaveAll(ctx context.Context) error {
	return f.record("save-all flush")
}

func (f *FakeController) SaveOn(ctx context.Context) error {
	return f.record("save-on")
}

func (f *FakeController) Say(ctx context.Context, message string) error {
	return f.record("say " + message)
}

func (f *FakeController) Close() error { return nil }

func (f *FakeController) CommandSequence() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, len(f.Calls))
	copy(out, f.Calls)
	return out
}

func (f *FakeController) SetFail(cmd string, err error) {
	f.FailOn[cmd] = err
}

func (f *FakeController) MustHaveSequence(want []string) error {
	got := f.CommandSequence()
	if len(got) < len(want) {
		return fmt.Errorf("expected at least %d calls, got %d: %v", len(want), len(got), got)
	}
	for i, w := range want {
		if got[i] != w {
			return fmt.Errorf("call[%d] = %q, want %q (full: %v)", i, got[i], w, got)
		}
	}
	return nil
}
