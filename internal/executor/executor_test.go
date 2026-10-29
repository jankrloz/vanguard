package executor

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

func TestMPMPrimaryWithSilentNativeFallback(t *testing.T) {
	stateDir := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("VANGUARD_STATE_DIR", stateDir)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Create fake mpm that fails on custom tap brew formula (exiting 0 with "not recognized" error)
	mpmScript := filepath.Join(binDir, "mpm")
	if err := os.WriteFile(mpmScript, []byte("#!/bin/sh\necho \"error: anomalyco/tap/opencode-v2 is not recognized by any of the selected managers. Skip it.\"\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write fake mpm: %v", err)
	}

	// Create fake brew that succeeds natively
	brewScript := filepath.Join(binDir, "brew")
	if err := os.WriteFile(brewScript, []byte("#!/bin/sh\necho \"==> Upgrading anomalyco/tap/opencode-v2 2.2.0 -> 2.2.1\"\nexit 0\n"), 0755); err != nil {
		t.Fatalf("write fake brew: %v", err)
	}

	execEngine := New()
	pkgs := []domain.Package{
		{
			ID:             "brew:anomalyco/tap/opencode-v2",
			Name:           "anomalyco/tap/opencode-v2",
			Manager:        "brew",
			CurrentVersion: "2.2.0",
			LatestVersion:  "2.2.1",
		},
	}

	var logs []string
	var doneStatus string
	err := execEngine.ExecuteUpgrades(context.Background(), pkgs, func(ev domain.UpgradeEvent) {
		if ev.Type == "log" {
			logs = append(logs, ev.Message)
		}
		if ev.Type == "package_done" {
			doneStatus = ev.Status
		}
	})
	if err != nil {
		t.Fatalf("expected silent fallback to succeed, got error: %v", err)
	}
	if doneStatus != "ok" {
		t.Fatalf("expected package_done status 'ok', got %q", doneStatus)
	}

	joinedLogs := strings.Join(logs, "\n")
	if strings.Contains(joinedLogs, "is not recognized by any of the selected managers") {
		t.Fatalf("expected failed mpm log to be suppressed during silent fallback, got: %s", joinedLogs)
	}
	if !strings.Contains(joinedLogs, "==> Upgrading anomalyco/tap/opencode-v2 2.2.0 -> 2.2.1") {
		t.Fatalf("expected native brew fallback log to be emitted, got: %s", joinedLogs)
	}
}

func TestPerPackageFaultIsolation(t *testing.T) {
	stateDir := t.TempDir()
	binDir := t.TempDir()
	t.Setenv("VANGUARD_STATE_DIR", stateDir)
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Fake mpm always fails
	mpmScript := filepath.Join(binDir, "mpm")
	_ = os.WriteFile(mpmScript, []byte("#!/bin/sh\necho \"critical: Could not upgrade\"\nexit 1\n"), 0755)

	// Fake pnpm succeeds for "good-pkg@latest" and fails if "bad-pkg@latest" is present
	pnpmScript := filepath.Join(binDir, "pnpm")
	_ = os.WriteFile(pnpmScript, []byte(`#!/bin/sh
case "$*" in
  *bad-pkg*)
    echo "ERR_PNPM_FETCH_404 bad-pkg"
    exit 1
    ;;
  *)
    echo "Upgraded $*"
    exit 0
    ;;
esac
`), 0755)

	execEngine := New()
	pkgs := []domain.Package{
		{
			ID:             "pnpm:good-pkg",
			Name:           "good-pkg",
			Manager:        "pnpm",
			CurrentVersion: "1.0.0",
			LatestVersion:  "1.1.0",
		},
		{
			ID:             "pnpm:bad-pkg",
			Name:           "bad-pkg",
			Manager:        "pnpm",
			CurrentVersion: "1.0.0",
			LatestVersion:  "2.0.0",
		},
	}

	_ = execEngine.ExecuteUpgrades(context.Background(), pkgs, func(ev domain.UpgradeEvent) {})

	entries, err := history.ReadRecent(10)
	if err != nil {
		t.Fatalf("ReadRecent: %v", err)
	}
	statusByPkg := make(map[string]string)
	for _, e := range entries {
		statusByPkg[e.PackageID] = e.Status
	}
	if statusByPkg["pnpm:good-pkg"] != "ok" {
		t.Fatalf("expected pnpm:good-pkg to succeed via fault isolation, got %q", statusByPkg["pnpm:good-pkg"])
	}
	if statusByPkg["pnpm:bad-pkg"] != "failed" {
		t.Fatalf("expected pnpm:bad-pkg to be recorded as failed, got %q", statusByPkg["pnpm:bad-pkg"])
	}
}

func TestBuildNativeFallbackCommandsMatrix(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		mgr      string
		names    []string
		wantArgs []string
	}{
		{"brew", []string{"jq"}, []string{"brew", "upgrade", "jq"}},
		{"cask", []string{"codex"}, []string{"brew", "upgrade", "--cask", "codex"}},
		{"gcloud", []string{"bq", "core"}, []string{"gcloud", "components", "update", "--quiet"}},
		{"npm", []string{"vercel"}, []string{"npm", "install", "-g", "vercel@latest"}},
		{"pnpm", []string{"neonctl"}, []string{"pnpm", "add", "-g", "neonctl@latest"}},
		{"yarn", []string{"serve"}, []string{"yarn", "global", "upgrade", "serve"}},
		{"bun", []string{"bunx-pkg"}, []string{"bun", "update", "-g", "bunx-pkg"}},
		{"pip", []string{"websockets"}, []string{"pip3", "install", "--upgrade", "websockets"}},
		{"pipx", []string{"httpie"}, []string{"pipx", "upgrade", "httpie"}},
		{"uv", []string{"pip"}, []string{"uv", "pip", "install", "--system", "--upgrade", "pip"}},
		{"uvx", []string{"ruff"}, []string{"uv", "tool", "upgrade", "ruff"}},
		{"cargo", []string{"ripgrep"}, []string{"cargo", "install", "ripgrep"}},
		{"rustup", []string{"stable"}, []string{"rustup", "update"}},
		{"mas", []string{"497799835"}, []string{"mas", "upgrade", "497799835"}},
		{"composer", []string{"laravel/installer"}, []string{"composer", "global", "update", "laravel/installer"}},
	}

	for _, tc := range cases {
		cmds := buildNativeFallbackCommands(ctx, tc.mgr, tc.names)
		if len(cmds) == 0 {
			t.Fatalf("expected native fallback commands for %s, got 0", tc.mgr)
		}
		got := cmds[0].Args
		if len(got) < len(tc.wantArgs) {
			t.Fatalf("mgr %s: got args %v, want prefix %v", tc.mgr, got, tc.wantArgs)
		}
		for i := 1; i < len(tc.wantArgs); i++ {
			if got[i] != tc.wantArgs[i] {
				t.Fatalf("mgr %s arg[%d]: got %q, want %q (full: %v)", tc.mgr, i, got[i], tc.wantArgs[i], got)
			}
		}
	}
}
