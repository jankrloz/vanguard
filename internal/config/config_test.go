package config

import (
	"testing"
)

func TestConfigTogglePin(t *testing.T) {
	t.Setenv("VANGUARD_CONFIG_DIR", t.TempDir())

	cfg, err := Load()
	if err != nil {
		t.Fatalf("failed to load initial config: %v", err)
	}

	if cfg.IsPinned("foo") {
		t.Fatalf("expected foo not to be pinned")
	}

	// Toggle pin ON via real TogglePin method
	pinned, err := cfg.TogglePin("foo")
	if err != nil || !pinned {
		t.Fatalf("expected foo to be pinned: %v, pinned=%v", err, pinned)
	}

	// Reload from disk to verify persistence
	reloaded, err := Load()
	if err != nil {
		t.Fatalf("failed to reload config: %v", err)
	}
	if !reloaded.IsPinned("foo") {
		t.Fatalf("expected foo to be persisted as pinned")
	}

	// Toggle pin OFF
	pinned, err = reloaded.TogglePin("foo")
	if err != nil || pinned {
		t.Fatalf("expected foo to be unpinned: %v, pinned=%v", err, pinned)
	}
}
