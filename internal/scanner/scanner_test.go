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

func TestDeriveSourceURL(t *testing.T) {
	cases := []struct {
		mgr  string
		name string
		want string
	}{
		{"skills", "ruff", "https://skills.sh"},
		{"brew", "fzf", "https://formulae.brew.sh/formula/fzf"},
		{"cask", "codex", "https://formulae.brew.sh/cask/codex"},
		{"npm", "vercel", "https://www.npmjs.com/package/vercel"},
		{"pnpm", "neonctl", "https://www.npmjs.com/package/neonctl"},
		{"pip", "requests", "https://pypi.org/project/requests"},
		{"cargo", "ripgrep", "https://crates.io/crates/ripgrep"},
	}

	for _, tc := range cases {
		got := DeriveSourceURL(tc.mgr, tc.name)
		if got != tc.want {
			t.Errorf("DeriveSourceURL(%q, %q) = %q; want %q", tc.mgr, tc.name, got, tc.want)
		}
	}
}

func TestParseTopgradeDryRun(t *testing.T) {
	raw := `
―― 13:19:15 - macOS system update ――
Dry running: softwareupdate --install --all
―― 13:19:19 - Cargo ――
cargo-update isn't installed so Topgrade can't upgrade cargo packages.
―― 13:19:20 - Containers ――
―― 13:19:22 - Claude Code ――
Dry running: /Users/jc/.local/bin/claude update
―― 13:19:22 - uv ――
Dry running: /opt/homebrew/bin/uv self update
―― 13:19:23 - Brew ――
Dry running: /opt/homebrew/bin/brew update
―― 13:19:23 - Summary ――
System update: OK
Containers: FAILED
Claude Code: OK
uv: OK
Brew (ARM): OK
`
	pkgs := ParseTopgradeDryRun(strings.NewReader(raw), nil, false)
	got := make(map[string]domain.Package)
	for _, p := range pkgs {
		got[p.ID] = p
	}

	if _, ok := got["topgrade:claude_code"]; !ok {
		t.Fatalf("expected topgrade:claude_code, got %+v", got)
	}
	if _, ok := got["topgrade:uv"]; !ok {
		t.Fatalf("expected topgrade:uv, got %+v", got)
	}
	if _, ok := got["topgrade:system"]; ok {
		t.Fatalf("did not expect topgrade:system when hasMacOSUpdates=false")
	}
	if _, ok := got["topgrade:cargo"]; ok {
		t.Fatalf("did not expect topgrade:cargo without dry-run commands")
	}
	if _, ok := got["topgrade:containers"]; ok {
		t.Fatalf("did not expect topgrade:containers when FAILED")
	}
	if _, ok := got["topgrade:brew"]; ok {
		t.Fatalf("did not expect MPM-covered brew in Topgrade tier")
	}
}

func TestParseSkillsCheckOutput(t *testing.T) {
	raw := `
Checking for skill updates…
Fetching 2 sources…
Available updates:
  • impeccable (pbakaus/impeccable)
  • ponytail (dietrichgebert/ponytail)
Warning: The following skills appear to have been deleted upstream:
  • old-skill
`
	pkgs := ParseSkillsCheckOutput(strings.NewReader(raw))
	if len(pkgs) != 2 {
		t.Fatalf("expected 2 skill updates, got %d: %+v", len(pkgs), pkgs)
	}
	if pkgs[0].ID != "skills:impeccable" || pkgs[1].ID != "skills:ponytail" {
		t.Fatalf("unexpected skill IDs: %+v", pkgs)
	}
}

func TestParseAPMOutdatedOutput(t *testing.T) {
	raw := `
Package              Current  Latest
microsoft/spec-kit   0.1.0    0.2.0
`
	pkgs := ParseAPMOutdatedOutput(strings.NewReader(raw))
	if len(pkgs) != 1 || pkgs[0].ID != "apm:microsoft/spec-kit" || pkgs[0].LatestVersion != "0.2.0" {
		t.Fatalf("unexpected apm outdated parse: %+v", pkgs)
	}
}
