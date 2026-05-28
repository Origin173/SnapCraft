package retention

import (
	"fmt"
	"sort"
	"time"

	"github.com/Origin173/SnapCraft/internal/config"
	"github.com/Origin173/SnapCraft/internal/snapshot"
)

// Plan describes snapshots to keep and delete.
type Plan struct {
	Keep   []*snapshot.Manifest
	Delete []*snapshot.Manifest
}

// Compute determines which snapshots to retain based on policy.
func Compute(cfg *config.Config, manifests []*snapshot.Manifest) *Plan {
	if len(manifests) == 0 {
		return &Plan{}
	}

	sorted := make([]*snapshot.Manifest, len(manifests))
	copy(sorted, manifests)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartedAt.After(sorted[j].StartedAt)
	})

	keepSet := make(map[string]bool)

	// Keep all from last N days (daily retention).
	dailyCutoff := time.Now().UTC().AddDate(0, 0, -cfg.Retention.Daily)
	for _, m := range sorted {
		if m.StartedAt.After(dailyCutoff) {
			keepSet[m.ID] = true
		}
	}

	// Keep one per week for last N weeks.
	weeklyKeep := weeklySnapshots(sorted, cfg.Retention.Weekly)
	for _, m := range weeklyKeep {
		keepSet[m.ID] = true
	}

	// Keep one per month for last N months.
	if cfg.Retention.Monthly > 0 {
		monthlyKeep := monthlySnapshots(sorted, cfg.Retention.Monthly)
		for _, m := range monthlyKeep {
			keepSet[m.ID] = true
		}
	}

	plan := &Plan{}
	for _, m := range sorted {
		if m.Status != snapshot.StatusCompleted {
			keepSet[m.ID] = true
		}
		if keepSet[m.ID] {
			plan.Keep = append(plan.Keep, m)
		} else {
			plan.Delete = append(plan.Delete, m)
		}
	}
	return plan
}

func weeklySnapshots(manifests []*snapshot.Manifest, weeks int) []*snapshot.Manifest {
	if weeks <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -weeks*7)
	seen := make(map[string]bool)
	var result []*snapshot.Manifest
	for _, m := range manifests {
		if m.StartedAt.Before(cutoff) {
			continue
		}
		y, w := m.StartedAt.ISOWeek()
		key := fmtWeekKey(y, w)
		if !seen[key] {
			seen[key] = true
			result = append(result, m)
		}
	}
	return result
}

func monthlySnapshots(manifests []*snapshot.Manifest, months int) []*snapshot.Manifest {
	if months <= 0 {
		return nil
	}
	cutoff := time.Now().UTC().AddDate(0, -months, 0)
	seen := make(map[string]bool)
	var result []*snapshot.Manifest
	for _, m := range manifests {
		if m.StartedAt.Before(cutoff) {
			continue
		}
		key := m.StartedAt.Format("2006-01")
		if !seen[key] {
			seen[key] = true
			result = append(result, m)
		}
	}
	return result
}

func fmtWeekKey(year, week int) string {
	return fmt.Sprintf("%d-W%02d", year, week)
}
