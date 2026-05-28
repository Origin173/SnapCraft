package webui

import (
	"context"
	"testing"
	"time"
)

func TestJobManagerSingleFlight(t *testing.T) {
	m := NewJobManager()
	done := make(chan struct{})
	err := m.RunAsync("backup", func(ctx context.Context) (string, error) {
		<-done
		return "ok", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := m.RunAsync("restore", func(ctx context.Context) (string, error) {
		return "", nil
	}); err == nil {
		t.Fatal("expected conflict")
	}
	close(done)
	time.Sleep(50 * time.Millisecond)
	if m.Current().Status != JobSucceeded {
		t.Fatalf("status = %q", m.Current().Status)
	}
}
