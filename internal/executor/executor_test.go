package executor

import (
	"context"
	"testing"
	"time"

	"github.com/jankrloz/vanguard/internal/domain"
	"github.com/jankrloz/vanguard/internal/history"
)

func TestExecutorContextCancellation(t *testing.T) {
	t.Setenv("VANGUARD_STATE_DIR", t.TempDir())

	exec := New()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	pkgs := []domain.Package{
		{
			ID:             "npm:test",
			Name:           "test",
			Manager:        "npm",
			CurrentVersion: "1.0.0",
			LatestVersion:  "1.0.1",
		},
	}

	var events []domain.UpgradeEvent
	err := exec.ExecuteUpgrades(ctx, pkgs, func(ev domain.UpgradeEvent) {
		events = append(events, ev)
	})

	if err == nil {
		t.Fatalf("expected context cancellation error, got nil")
	}

	if len(events) == 0 {
		t.Fatalf("expected at least start event")
	}
}

func TestExecutorHistoryIntegration(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("VANGUARD_STATE_DIR", stateDir)

	entry := history.Entry{
		Timestamp:   time.Now(),
		PackageID:   "npm:sample",
		Manager:     "npm",
		FromVersion: "1.0.0",
		ToVersion:   "1.0.1",
		Status:      "ok",
	}

	if err := history.Append(entry); err != nil {
		t.Fatalf("Append history: %v", err)
	}

	entries, err := history.ReadRecent(10)
	if err != nil {
		t.Fatalf("ReadRecent: %v", err)
	}

	if len(entries) != 1 || entries[0].PackageID != "npm:sample" {
		t.Fatalf("unexpected history entries: %+v", entries)
	}
}
