package retention

import (
	"fmt"
	"testing"
	"time"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/snapshot"
)

func TestComputeDailyRetention(t *testing.T) {
	cfg := &config.Config{
		Retention: config.RetentionConfig{Daily: 7, Weekly: 4},
	}
	now := time.Now().UTC()
	var manifests []*snapshot.Manifest
	for i := 0; i < 10; i++ {
		manifests = append(manifests, &snapshot.Manifest{
			ID:        fmt.Sprintf("snap-%d", i),
			StartedAt: now.AddDate(0, 0, -i),
			Status:    snapshot.StatusCompleted,
		})
	}
	plan := Compute(cfg, manifests)
	if len(plan.Keep) == 0 {
		t.Fatal("expected some snapshots to keep")
	}
}

func TestComputeEmpty(t *testing.T) {
	cfg := &config.Config{Retention: config.RetentionConfig{Daily: 7, Weekly: 4}}
	plan := Compute(cfg, nil)
	if len(plan.Keep) != 0 || len(plan.Delete) != 0 {
		t.Errorf("expected empty plan, got keep=%d delete=%d", len(plan.Keep), len(plan.Delete))
	}
}
