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
	case "brew", "cask", "npm", "pnpm", "yarn", "bun", "cargo", "pip", "pipx", "uv", "uvx", "gcloud", "mise", "asdf",
		"fisher", "antidote", "antigen", "zinit", "zplug", "rustup", "go", "composer", "vscode", "gh-ext", "mas":
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

// buildMPMCommand constructs the primary MPM upgrade command for a manager and package list.
func buildMPMCommand(ctx context.Context, mgr string, names []string) *exec.Cmd {
	var args []string
	if mgr != "" && mgr != "all" {
		args = append(args, "--"+mgr)
		if requiresPrivilegeEscalation(mgr) {
			args = append(args, "--sudo")
			_ = auth.EnsureSudo()
		}
	}
	args = append(args, "upgrade")
	args = append(args, names...)
	cmd := exec.CommandContext(ctx, "mpm", args...)
	cmd.Env = append(os.Environ(), "HOMEBREW_NO_ENV_HINTS=1")
	return cmd
}

// buildNativeFallbackCommands constructs native CLI upgrade commands for a package manager
// when MPM fails, lacks support for the manager, or rejects custom tap/component identifiers.
func buildNativeFallbackCommands(ctx context.Context, mgr string, names []string) []*exec.Cmd {
	lowerMgr := strings.ToLower(mgr)
	withCleanEnv := func(cmd *exec.Cmd) *exec.Cmd {
		cmd.Env = append(os.Environ(), "HOMEBREW_NO_ENV_HINTS=1")
		return cmd
	}

	switch lowerMgr {
	case "brew":
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "brew", append([]string{"upgrade"}, names...)...))}
	case "cask":
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "brew", append([]string{"upgrade", "--cask"}, names...)...))}
	case "gcloud":
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "gcloud", "components", "update", "--quiet"))}
	case "npm":
		targets := make([]string, 0, len(names)+2)
		targets = append(targets, "install", "-g")
		for _, n := range names {
			targets = append(targets, n+"@latest")
		}
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "npm", targets...))}
	case "pnpm":
		targets := make([]string, 0, len(names)+2)
		targets = append(targets, "add", "-g")
		for _, n := range names {
			targets = append(targets, n+"@latest")
		}
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "pnpm", targets...))}
	case "yarn":
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "yarn", append([]string{"global", "upgrade"}, names...)...))}
	case "bun":
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "bun", append([]string{"update", "-g"}, names...)...))}
	case "pip":
		bin := "pip3"
		if _, err := exec.LookPath("pip3"); err != nil {
			bin = "pip"
		}
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, bin, append([]string{"install", "--upgrade"}, names...)...))}
	case "pipx":
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "pipx", append([]string{"upgrade"}, names...)...))}
	case "uv":
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "uv", append([]string{"pip", "install", "--system", "--upgrade"}, names...)...))}
	case "uvx":
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "uv", append([]string{"tool", "upgrade"}, names...)...))}
	case "cargo":
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "cargo", append([]string{"install"}, names...)...))}
	case "rustup":
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "rustup", "update"))}
	case "gem":
		if requiresPrivilegeEscalation("gem") {
			_ = auth.EnsureSudo()
			return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "sudo", append([]string{"gem", "update"}, names...)...))}
		}
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "gem", append([]string{"update"}, names...)...))}
	case "mas":
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "mas", append([]string{"upgrade"}, names...)...))}
	case "composer":
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "composer", append([]string{"global", "update"}, names...)...))}
	case "apt", "aptitude":
		_ = auth.EnsureSudo()
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "sudo", append([]string{"apt-get", "install", "--only-upgrade", "-y"}, names...)...))}
	case "dnf", "dnf5":
		_ = auth.EnsureSudo()
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "sudo", append([]string{"dnf", "upgrade", "-y"}, names...)...))}
	case "apk":
		_ = auth.EnsureSudo()
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "sudo", append([]string{"apk", "upgrade"}, names...)...))}
	case "pacman":
		_ = auth.EnsureSudo()
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "sudo", append([]string{"pacman", "-S", "--noconfirm"}, names...)...))}
	case "zypper":
		_ = auth.EnsureSudo()
		return []*exec.Cmd{withCleanEnv(exec.CommandContext(ctx, "sudo", append([]string{"zypper", "update", "-y"}, names...)...))}
	case "vscode":
		var cmds []*exec.Cmd
		for _, n := range names {
			cmds = append(cmds, withCleanEnv(exec.CommandContext(ctx, "code", "--install-extension", n, "--force")))
		}
		return cmds
	case "gh-ext":
		var cmds []*exec.Cmd
		for _, n := range names {
			cmds = append(cmds, withCleanEnv(exec.CommandContext(ctx, "gh", "extension", "upgrade", n)))
		}
		return cmds
	default:
		return nil
	}
}

// executeWithSilentFallback runs MPM first (buffering its output), and if MPM fails or reports
// stream-level errors, silently falls back to the native package manager command(s).
func (e *Executor) executeWithSilentFallback(ctx context.Context, mgr string, names []string, onEvent EventCallback) error {
	mpmCmd := buildMPMCommand(ctx, mgr, names)
	var mpmLogs []string
	mpmErr := e.runStreamingCommand(mpmCmd, func(ev domain.UpgradeEvent) {
		if ev.Type == "log" {
			mpmLogs = append(mpmLogs, ev.Message)
		}
	})

	if mpmErr == nil {
		for _, line := range mpmLogs {
			onEvent(domain.UpgradeEvent{
				Type:    "log",
				Message: line,
			})
		}
		return nil
	}

	// MPM failed: attempt silent native fallback
	fallbackCmds := buildNativeFallbackCommands(ctx, mgr, names)
	if len(fallbackCmds) == 0 {
		// No native fallback known for this manager; flush MPM logs so user has diagnostics
		for _, line := range mpmLogs {
			onEvent(domain.UpgradeEvent{
				Type:    "log",
				Message: line,
			})
		}
		return mpmErr
	}

	var fallbackLogs []string
	var fallbackErr error
	for _, fbCmd := range fallbackCmds {
		err := e.runStreamingCommand(fbCmd, func(ev domain.UpgradeEvent) {
			if ev.Type == "log" {
				fallbackLogs = append(fallbackLogs, ev.Message)
			}
		})
		if err != nil {
			fallbackErr = err
			break
		}
	}

	if fallbackErr == nil {
		for _, line := range fallbackLogs {
			onEvent(domain.UpgradeEvent{
				Type:    "log",
				Message: line,
			})
		}
		return nil
	}

	// Only emit fallback logs when single-package or no further per-package isolation will run
	if len(names) <= 1 {
		for _, line := range fallbackLogs {
			onEvent(domain.UpgradeEvent{
				Type:    "log",
				Message: line,
			})
		}
	}
	return fallbackErr
}

// ExecuteUpgrades groups packages by manager in deterministic order, attempts MPM first,
// silently falls back to native CLIs on failure, and isolates faults per-package when batches fail.
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

		err := e.executeWithSilentFallback(ctx, mgr, names, onEvent)
		if err == nil {
			succeededPackages += len(pkgList)
			onEvent(domain.UpgradeEvent{
				Type:    "package_done",
				Package: mgr,
				Status:  "ok",
				Message: fmt.Sprintf("[%s] successfully upgraded (%d/%d)", mgr, len(pkgList), len(pkgList)),
			})
			for _, p := range pkgList {
				_ = history.Append(history.Entry{
					Timestamp:   time.Now(),
					PackageID:   p.ID,
					Manager:     p.Manager,
					FromVersion: p.CurrentVersion,
					ToVersion:   p.LatestVersion,
					Status:      "ok",
				})
			}
			continue
		}

		// Fault isolation: if a multi-package manager batch failed and the manager supports
		// per-package upgrades (i.e. not whole-suite updaters like gcloud/rustup), retry each package individually.
		lowerMgr := strings.ToLower(mgr)
		if len(pkgList) > 1 && lowerMgr != "gcloud" && lowerMgr != "rustup" {
			mgrSucceeded := 0
			mgrFailed := 0
			var lastErr error
			for _, p := range pkgList {
				pkgErr := e.executeWithSilentFallback(ctx, mgr, []string{p.Name}, onEvent)
				pkgStatus := "ok"
				if pkgErr != nil {
					pkgStatus = "failed"
					mgrFailed++
					failedPackages++
					lastErr = pkgErr
				} else {
					mgrSucceeded++
					succeededPackages++
				}
				_ = history.Append(history.Entry{
					Timestamp:   time.Now(),
					PackageID:   p.ID,
					Manager:     p.Manager,
					FromVersion: p.CurrentVersion,
					ToVersion:   p.LatestVersion,
					Status:      pkgStatus,
				})
			}
			if mgrFailed == 0 {
				onEvent(domain.UpgradeEvent{
					Type:    "package_done",
					Package: mgr,
					Status:  "ok",
					Message: fmt.Sprintf("[%s] successfully upgraded (%d/%d)", mgr, mgrSucceeded, len(pkgList)),
				})
			} else {
				onEvent(domain.UpgradeEvent{
					Type:    "package_done",
					Package: mgr,
					Status:  "failed",
					Message: fmt.Sprintf("[%s] partial/failed (%d/%d upgraded): %v", mgr, mgrSucceeded, len(pkgList), lastErr),
				})
			}
			continue
		}

		failedPackages += len(pkgList)
		onEvent(domain.UpgradeEvent{
			Type:    "package_done",
			Package: mgr,
			Status:  "failed",
			Message: fmt.Sprintf("[%s] failed: %v", mgr, err),
		})
		for _, p := range pkgList {
			_ = history.Append(history.Entry{
				Timestamp:   time.Now(),
				PackageID:   p.ID,
				Manager:     p.Manager,
				FromVersion: p.CurrentVersion,
				ToVersion:   p.LatestVersion,
				Status:      "failed",
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
