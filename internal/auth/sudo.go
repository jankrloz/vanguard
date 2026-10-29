package auth

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"regexp"
	"runtime"
	"time"
)

var validUsernameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func safeCurrentUsername() string {
	if u, err := user.Current(); err == nil && validUsernameRe.MatchString(u.Username) {
		return u.Username
	}
	if envUser := os.Getenv("USER"); validUsernameRe.MatchString(envUser) {
		return envUser
	}
	return "root"
}

// EnsureSudo checks if sudo privileges are active.
// If not:
// 1. If /dev/tty or os.Stdin is available, prompts via `sudo -v` (Touch ID / PAM).
// 2. On macOS without an interactive terminal (e.g. web daemon), prompts native macOS authorization dialog:
//    "Vanguard is trying to execute privileged package updates."
func EnsureSudo() error {
	// 1. Fast path: check if sudo credentials are valid without prompting
	if exec.Command("sudo", "-n", "true").Run() == nil {
		return nil
	}

	// 2. Try interactive TTY if available
	if tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0); err == nil {
		defer tty.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "sudo", "-v")
		cmd.Stdin = tty
		cmd.Stdout = tty
		cmd.Stderr = tty
		if err := cmd.Run(); err == nil {
			return nil
		}
	} else if stat, err := os.Stdin.Stat(); err == nil && (stat.Mode()&os.ModeCharDevice) != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "sudo", "-v")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
	}

	// 3. Fallback on macOS: Native macOS Authorization dialog with custom prompt
	if runtime.GOOS == "darwin" {
		user := safeCurrentUsername()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		script := fmt.Sprintf(`do shell script "sudo -u %s -v || sudo -v" with prompt "Vanguard is trying to execute privileged package updates." with administrator privileges`, user)
		cmd := exec.CommandContext(ctx, "osascript", "-e", script)
		if err := cmd.Run(); err == nil {
			if exec.Command("sudo", "-n", "true").Run() == nil {
				return nil
			}
		}
	}

	return fmt.Errorf("sudo authorization required but no interactive session or permission granted")
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
