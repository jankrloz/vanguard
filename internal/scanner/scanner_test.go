package scanner

import (
	"strings"
	"testing"

	"github.com/jankrloz/vanguard/internal/domain"
)

func TestParseMPMOutput(t *testing.T) {
	rawOutput := `
warning:bun.outdated:
error: missing package.json, nothing outdated
Manager  Package ID     Installed version  Latest version
cask     codex          0.164.0            0.165.0
npm      npm            11.20.0            12.2.1
pnpm     neonctl        7.0.8              8.13.1
uv       pip            26.2.1             26.2.2
4 packages total (cask: 1, npm: 1, pnpm: 1, uv: 1).
`

	pkgs := ParseMPMOutput(strings.NewReader(rawOutput))

	if len(pkgs) != 4 {
		t.Fatalf("expected 4 packages, got %d", len(pkgs))
	}

	// Verify codex
	codex := pkgs[0]
	if codex.ID != "cask:codex" || codex.Name != "codex" || codex.Manager != "cask" {
		t.Errorf("unexpected codex fields: %+v", codex)
	}
	if codex.ChangeType != domain.ChangeMinor || !codex.Safe || !codex.Selected {
		t.Errorf("expected codex to be safe minor update, got %+v", codex)
	}

	// Verify npm
	npm := pkgs[1]
	if npm.ChangeType != domain.ChangeMajor || npm.Safe || npm.Selected {
		t.Errorf("expected npm to be unsafe major breaking update, got %+v", npm)
	}

	// Verify pip (patch)
	pip := pkgs[3]
	if pip.ChangeType != domain.ChangePatch || !pip.Safe {
		t.Errorf("expected pip to be safe patch, got %+v", pip)
	}
}
