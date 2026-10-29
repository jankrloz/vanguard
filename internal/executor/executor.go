package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	case "brew", "cask", "npm", "pnpm", "yarn", "cargo", "pip", "pipx", "mise", "asdf",
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

// ExecuteUpgrades groups packages by manager and runs upgrades while streaming events.
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

	for mgr, pkgList := range byManager {
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

		var args []string
		if mgr != "" && mgr != "all" {
			args = append(args, "--"+mgr)
			if requiresPrivilegeEscalation(mgr) {
				args = append(args, "--sudo")
				// Prime administrator credentials via Touch ID / sudo before invoking MPM
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
		cmd := exec.CommandContext(ctx, "mpm", args...)

		err := e.runStreamingCommand(cmd, onEvent)
		status := "ok"
		if err != nil {
			status = "failed"
			onEvent(domain.UpgradeEvent{
				Type:    "package_done",
				Package: mgr,
				Status:  "failed",
				Message: fmt.Sprintf("[%s] failed: %v", mgr, err),
			})
		} else {
			onEvent(domain.UpgradeEvent{
				Type:    "package_done",
				Package: mgr,
				Status:  "ok",
				Message: fmt.Sprintf("[%s] successfully upgraded", mgr),
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

	onEvent(domain.UpgradeEvent{
		Type:    "complete",
		Message: "All requested upgrades finished",
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
	streamReader := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			line := scanner.Text()
			onEvent(domain.UpgradeEvent{
				Type:    "log",
				Message: line,
			})
			if strings.Contains(line, "FilePermissionError") || strings.Contains(line, "sudo: a password is required") {
				onEvent(domain.UpgradeEvent{
					Type:    "log",
					Message: "⚠️ Sudo authentication required: prime credentials via 'vg' terminal command or 'sudo -v'",
				})
			}
		}
	}

	wg.Add(2)
	go streamReader(stdout)
	go streamReader(stderr)

	wg.Wait()
	return cmd.Wait()
}
