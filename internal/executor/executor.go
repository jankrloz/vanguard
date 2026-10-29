package executor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/jankrloz/vanguard/internal/domain"
)

type EventCallback func(event domain.UpgradeEvent)

type Executor struct {
	mu sync.Mutex
}

func New() *Executor {
	return &Executor{}
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

		var cmd *exec.Cmd
		switch mgr {
		case "brew":
			args := append([]string{"upgrade"}, names...)
			cmd = exec.CommandContext(ctx, "brew", args...)
		case "cask":
			args := append([]string{"upgrade", "--cask"}, names...)
			cmd = exec.CommandContext(ctx, "brew", args...)
		case "pnpm":
			args := append([]string{"update", "-g"}, names...)
			cmd = exec.CommandContext(ctx, "pnpm", args...)
		case "npm":
			args := append([]string{"update", "-g"}, names...)
			cmd = exec.CommandContext(ctx, "npm", args...)
		case "uv", "uvx":
			// If pip, uv self-update or tool update
			args := append([]string{"tool", "upgrade"}, names...)
			cmd = exec.CommandContext(ctx, "uv", args...)
		case "mise":
			args := append([]string{"upgrade"}, names...)
			cmd = exec.CommandContext(ctx, "mise", args...)
		default:
			// Fallback to mpm granular upgrade
			args := append([]string{"--" + mgr, "upgrade"}, names...)
			cmd = exec.CommandContext(ctx, "mpm", args...)
		}

		err := e.runStreamingCommand(cmd, onEvent)
		if err != nil {
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
	}

	onEvent(domain.UpgradeEvent{
		Type:    "complete",
		Message: "All requested upgrades finished",
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

	// Check if `mo` (Mole) is available
	if path, err := exec.LookPath("mo"); err == nil && path != "" {
		onEvent(domain.UpgradeEvent{
			Type:    "log",
			Message: "Running Mole cleanup (`mo clean`)...",
		})
		cmd := exec.CommandContext(ctx, "mo", "clean", "--dry-run=false")
		_ = e.runStreamingCommand(cmd, onEvent)
	} else {
		// Fallback to brew cleanup
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
		}
	}

	wg.Add(2)
	go streamReader(stdout)
	go streamReader(stderr)

	wg.Wait()
	return cmd.Wait()
}
