package auth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// EnsureSudo checks if sudo privileges are active.
// If not, it executes `sudo -v` attached to /dev/tty or os.Stdin,
// which prompts for Touch ID or biometric auth on macOS (via pam_tid.so).
func EnsureSudo() error {
	// 1. Fast path: check if sudo credentials are valid without prompting
	if exec.Command("sudo", "-n", "true").Run() == nil {
		return nil
	}

	// 2. Check if stdin is a character device (terminal)
	stat, err := os.Stdin.Stat()
	if err != nil || (stat.Mode()&os.ModeCharDevice) == 0 {
		return fmt.Errorf("non-interactive environment (no TTY attached)")
	}

	// 3. Prompt Touch ID with a 15-second safety timeout
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("biometric/sudo authentication declined or failed: %w", err)
	}

	return nil
}

// StartKeepAlive runs a background goroutine that refreshes the sudo
// timeout every interval (default 40 seconds) until ctx is canceled.
func StartKeepAlive(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 40 * time.Second
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_ = exec.Command("sudo", "-n", "true").Run()
			}
		}
	}()
}
