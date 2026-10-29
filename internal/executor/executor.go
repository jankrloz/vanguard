package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jankrloz/vanguard/internal/auth"
	"github.com/jankrloz/vanguard/internal/domain"
	"github.com/jankrloz/vanguard/internal/history"
)

type EventCallback func(event domain.UpgradeEvent)

type Executor struct {
	mu sync.Mutex
}

func New() *Executor {
	return &Executor{}
}

// requiresPrivilegeEscalation inspects whether upgrading a package manager requires
// root escalation, strictly guarding user-level environments from root ownership pollution.
func requiresPrivilegeEscalation(mgr string) bool {
	// 1. User-scoped managers must NEVER run with sudo (prevents root-polluted ~/.npm, ~/.cargo, Homebrew aborts)
	switch strings.ToLower(mgr) {
	case "brew", "cask", "npm", "pnpm", "yarn", "cargo", "pip", "pipx", "uv", "uvx", "gcloud", "mise", "asdf",
		"fisher", "antidote", "antigen", "zinit", "zplug", "rustup", "go", "composer":
		return false
	}

	// 2. Linux native system package managers always require root
	switch strings.ToLower(mgr) {
	case "apt", "aptitude", "dnf", "dnf5", "pacman", "zypper", "apk", "emerge":
		return true
	}

	// 3. For hybrid managers like Ruby gems: inspect the active gem directory
	if strings.ToLower(mgr) == "gem" {
		home, _ := os.UserHomeDir()
		out, err := exec.Command("gem", "environment", "gemdir").Output()
		if err == nil {
			gemDir := strings.TrimSpace(string(out))
			// Safety rule: if inside user's $HOME (e.g. ~/.rbenv, ~/.asdf, ~/.gem), NEVER use sudo
			if home != "" && strings.HasPrefix(gemDir, home) {
				return false
			}
			// Check if writable by the current user
			if isDirWritable(gemDir) {
				return false
			}
			return true // System directory (e.g. /Library/Ruby/Gems/2.6.0) owned by root
		}
	}

	return false
}

func isDirWritable(path string) bool {
	if path == "" {
		return false
	}
	testFile := filepath.Join(path, ".vanguard_write_probe")
	f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0600)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(testFile)
	return true
}

// isSafeIdentifier rejects CLI flag injection (leading hyphen) or control characters
// in package or manager identifiers before passing them to subprocesses.
func isSafeIdentifier(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	return !strings.ContainsAny(s, "\x00\r\n\t;&|`$")
}

// buildUpgradeCommand constructs the authoritative upgrade command for a given manager.
// Uses native CLIs for managers where mpm lacks support or mishandles arguments (e.g. brew custom taps,
// gcloud components, uv system pip, uvx tools), falling back to mpm for remaining ecosystems.
func buildUpgradeCommand(ctx context.Context, mgr string, names []string, onEvent EventCallback) *exec.Cmd {
	lowerMgr := strings.ToLower(mgr)
	switch lowerMgr {
	case "brew":
		if _, err := exec.LookPath("brew"); err == nil {
			args := append([]string{"upgrade"}, names...)
			cmd := exec.CommandContext(ctx, "brew", args...)
			cmd.Env = append(os.Environ(), "HOMEBREW_NO_ENV_HINTS=1")
			return cmd
		}
	case "cask":
		if _, err := exec.LookPath("brew"); err == nil {
			args := append([]string{"upgrade", "--cask"}, names...)
			cmd := exec.CommandContext(ctx, "brew", args...)
			cmd.Env = append(os.Environ(), "HOMEBREW_NO_ENV_HINTS=1")
			return cmd
		}
	case "gcloud":
		if _, err := exec.LookPath("gcloud"); err == nil {
			return exec.CommandContext(ctx, "gcloud", "components", "update", "--quiet")
		}
	case "uv":
		if _, err := exec.LookPath("uv"); err == nil {
			args := append([]string{"pip", "install", "--system", "--upgrade"}, names...)
			return exec.CommandContext(ctx, "uv", args...)
		}
	case "uvx":
		if _, err := exec.LookPath("uv"); err == nil {
			args := append([]string{"tool", "upgrade"}, names...)
			return exec.CommandContext(ctx, "uv", args...)
		}
	case "pipx":
		if _, err := exec.LookPath("pipx"); err == nil {
			args := append([]string{"upgrade"}, names...)
			return exec.CommandContext(ctx, "pipx", args...)
		}
	}

	var args []string
	if mgr != "" && mgr != "all" {
		args = append(args, "--"+mgr)
		if requiresPrivilegeEscalation(mgr) {
			args = append(args, "--sudo")
			if err := auth.EnsureSudo(); err != nil {
				onEvent(domain.UpgradeEvent{
					Type:    "log",
					Message: fmt.Sprintf("⚠️ [%s] requires administrator privileges: %v. Please prime credentials via 'sudo -v' or terminal 'vg'", mgr, err),
				})
			}
		}
	}
	args = append(args, "upgrade")
	args = append(args, names...)
	return exec.CommandContext(ctx, "mpm", args...)
}

// ExecuteUpgrades groups packages by manager in deterministic order and runs upgrades while streaming events.
func (e *Executor) ExecuteUpgrades(ctx context.Context, pkgs []domain.Package, onEvent EventCallback) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	onEvent(domain.UpgradeEvent{
		Type:    "start",
		Message: fmt.Sprintf("Starting upgrade for %d packages...", len(pkgs)),
	})

	// Group by manager
	byManager := make(map[string][]domain.Package)
	for _, p := range pkgs {
		if !isSafeIdentifier(p.Manager) || !isSafeIdentifier(p.Name) {
			onEvent(domain.UpgradeEvent{
				Type:    "error",
				Message: fmt.Sprintf("Rejected unsafe package identifier: %s:%s", p.Manager, p.Name),
			})
			continue
		}
		byManager[p.Manager] = append(byManager[p.Manager], p)
	}

	// Deterministic manager ordering: managers with vulnerable packages first, then alphabetical
	var managers []string
	for mgr := range byManager {
		managers = append(managers, mgr)
	}
	sort.Slice(managers, func(i, j int) bool {
		hasVulnI := false
		for _, p := range byManager[managers[i]] {
			if p.HasVulnerability {
				hasVulnI = true
				break
			}
		}
		hasVulnJ := false
		for _, p := range byManager[managers[j]] {
			if p.HasVulnerability {
				hasVulnJ = true
				break
			}
		}
		if hasVulnI != hasVulnJ {
			return hasVulnI
		}
		return managers[i] < managers[j]
	})

	succeededPackages := 0
	failedPackages := 0

	for _, mgr := range managers {
		pkgList := byManager[mgr]
		select {
		case <-ctx.Done():
			onEvent(domain.UpgradeEvent{
				Type:    "error",
				Message: "Upgrade canceled by user",
			})
			return ctx.Err()
		default:
		}

		var names []string
		for _, p := range pkgList {
			names = append(names, p.Name)
		}

		onEvent(domain.UpgradeEvent{
			Type:    "package_start",
			Package: mgr,
			Message: fmt.Sprintf("Upgrading [%s] packages: %s", mgr, strings.Join(names, ", ")),
		})

		cmd := buildUpgradeCommand(ctx, mgr, names, onEvent)
		err := e.runStreamingCommand(cmd, onEvent)
		status := "ok"
		if err != nil {
			status = "failed"
			failedPackages += len(pkgList)
			onEvent(domain.UpgradeEvent{
				Type:    "package_done",
				Package: mgr,
				Status:  "failed",
				Message: fmt.Sprintf("[%s] failed: %v", mgr, err),
			})
		} else {
			succeededPackages += len(pkgList)
			onEvent(domain.UpgradeEvent{
				Type:    "package_done",
				Package: mgr,
				Status:  "ok",
				Message: fmt.Sprintf("[%s] successfully upgraded (%d/%d)", mgr, len(pkgList), len(pkgList)),
			})
		}

		// Record in persistent audit history
		for _, p := range pkgList {
			_ = history.Append(history.Entry{
				Timestamp:   time.Now(),
				PackageID:   p.ID,
				Manager:     p.Manager,
				FromVersion: p.CurrentVersion,
				ToVersion:   p.LatestVersion,
				Status:      status,
			})
		}
	}

	if failedPackages > 0 {
		msg := fmt.Sprintf("Upgrade completed with errors: %d succeeded, %d failed", succeededPackages, failedPackages)
		onEvent(domain.UpgradeEvent{
			Type:    "error",
			Message: msg,
		})
		return fmt.Errorf("%d package(s) failed to upgrade", failedPackages)
	}

	onEvent(domain.UpgradeEvent{
		Type:    "complete",
		Message: fmt.Sprintf("All %d requested package(s) upgraded successfully", succeededPackages),
	})

	return nil
}

// ExecuteTopgrade triggers a deep system sweep through Topgrade.
func (e *Executor) ExecuteTopgrade(ctx context.Context, onEvent EventCallback) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	onEvent(domain.UpgradeEvent{
		Type:    "start",
		Message: "Starting Topgrade system-wide sweep...",
	})

	topgradePath, err := exec.LookPath("topgrade")
	if err != nil {
		onEvent(domain.UpgradeEvent{
			Type:    "error",
			Message: "topgrade executable not found in PATH",
		})
		return fmt.Errorf("topgrade not installed: %w", err)
	}

	cmd := exec.CommandContext(ctx, topgradePath, "--yes")
	err = e.runStreamingCommand(cmd, onEvent)

	status := "ok"
	if err != nil {
		status = "failed"
		onEvent(domain.UpgradeEvent{
			Type:    "package_done",
			Package: "topgrade",
			Status:  "failed",
			Message: fmt.Sprintf("Topgrade sweep failed: %v", err),
		})
	} else {
		onEvent(domain.UpgradeEvent{
			Type:    "package_done",
			Package: "topgrade",
			Status:  "ok",
			Message: "Topgrade sweep successfully completed",
		})
	}

	_ = history.Append(history.Entry{
		Timestamp:   time.Now(),
		PackageID:   "system:topgrade",
		Manager:     "topgrade",
		FromVersion: "sweep",
		ToVersion:   "latest",
		Status:      status,
	})

	onEvent(domain.UpgradeEvent{
		Type:    "complete",
		Message: "Topgrade sweep complete",
	})
	return nil
}

// ExecuteClean runs system cleanup (Mole clean or brew cleanup) and streams output.
func (e *Executor) ExecuteClean(ctx context.Context, onEvent EventCallback) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	onEvent(domain.UpgradeEvent{
		Type:    "start",
		Message: "Starting system cleanup...",
	})

	if path, err := exec.LookPath("mo"); err == nil && path != "" {
		onEvent(domain.UpgradeEvent{
			Type:    "log",
			Message: "Running Mole cleanup (`mo clean`)...",
		})
		cmd := exec.CommandContext(ctx, "mo", "clean", "--dry-run=false")
		_ = e.runStreamingCommand(cmd, onEvent)
	} else {
		onEvent(domain.UpgradeEvent{
			Type:    "log",
			Message: "Running `brew cleanup -s`...",
		})
		cmd := exec.CommandContext(ctx, "brew", "cleanup", "-s")
		_ = e.runStreamingCommand(cmd, onEvent)
	}

	onEvent(domain.UpgradeEvent{
		Type:    "complete",
		Message: "Cleanup complete",
	})
	return nil
}

// isNoisyBoxLine filters out decorative ASCII/Unicode box borders and spinner frames
// from subprocess logs so CLI and Web logs remain clean and readable.
func isNoisyBoxLine(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return true
	}
	for _, prefix := range []string{"╭─", "├─", "╰─", "┌─", "└─", "╔═", "╠═", "╚═", "│ Package ID │"} {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func (e *Executor) runStreamingCommand(cmd *exec.Cmd, onEvent EventCallback) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	var streamErrMu sync.Mutex
	var streamErrMsg string

	streamReader := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			if idx := strings.LastIndex(line, "\r"); idx != -1 {
				line = line[idx+1:]
			}
			trimmed := strings.TrimSpace(line)

			if strings.Contains(trimmed, "FilePermissionError") ||
				strings.Contains(trimmed, "sudo: a password is required") ||
				strings.Contains(trimmed, "Needs administrator rights") {
				streamErrMu.Lock()
				if streamErrMsg == "" {
					streamErrMsg = "sudo authentication required or permission denied"
				}
				streamErrMu.Unlock()
				onEvent(domain.UpgradeEvent{
					Type:    "log",
					Message: "⚠️ Sudo authentication required: prime credentials via 'vg' terminal command or 'sudo -v'",
				})
			}

			if strings.Contains(trimmed, "critical: Could not upgrade:") ||
				strings.Contains(trimmed, "is not recognized by any of the selected managers") ||
				strings.Contains(trimmed, "critical: No manager selected") ||
				strings.Contains(trimmed, "ERROR:  While executing gem") ||
				strings.HasPrefix(trimmed, "✘ ") {
				streamErrMu.Lock()
				streamErrMsg = trimmed
				streamErrMu.Unlock()
			} else if strings.Contains(trimmed, "still outdated") {
				streamErrMu.Lock()
				if streamErrMsg == "" {
					streamErrMsg = "package remained at previous version after upgrade command"
				}
				streamErrMu.Unlock()
			}

			if !isNoisyBoxLine(trimmed) {
				onEvent(domain.UpgradeEvent{
					Type:    "log",
					Message: trimmed,
				})
			}
		}
	}

	wg.Add(2)
	go streamReader(stdout)
	go streamReader(stderr)

	wg.Wait()
	if err := cmd.Wait(); err != nil {
		if streamErrMsg != "" {
			return fmt.Errorf("%s", streamErrMsg)
		}
		return err
	}
	streamErrMu.Lock()
	defer streamErrMu.Unlock()
	if streamErrMsg != "" {
		return fmt.Errorf("%s", streamErrMsg)
	}
	return nil
}
