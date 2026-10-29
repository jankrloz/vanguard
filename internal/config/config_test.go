package config

import (
	"testing"
)

func TestConfigTogglePin(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.IsPinned("foo") {
		t.Fatalf("expected foo not to be pinned")
	}

	// Toggle pin ON
	cfg.Packages.Pinned = append(cfg.Packages.Pinned, "foo")
	if !cfg.IsPinned("foo") {
		t.Fatalf("expected foo to be pinned")
	}

	// Filter out
	var updated []string
	for _, p := range cfg.Packages.Pinned {
		if p != "foo" {
			updated = append(updated, p)
		}
	}
	cfg.Packages.Pinned = updated

	if cfg.IsPinned("foo") {
		t.Fatalf("expected foo not to be pinned")
	}
}
