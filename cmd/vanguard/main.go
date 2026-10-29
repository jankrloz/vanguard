package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/jankrloz/vanguard"
	"github.com/jankrloz/vanguard/internal/auth"
	"github.com/jankrloz/vanguard/internal/config"
	"github.com/jankrloz/vanguard/internal/executor"
	"github.com/jankrloz/vanguard/internal/scanner"
	"github.com/jankrloz/vanguard/internal/server"
	"github.com/jankrloz/vanguard/internal/ui"
)

const Version = "0.1.2"

func printUsage() {
	fmt.Print(ui.Banner(Version, runtime.GOOS+"/"+runtime.GOARCH))
	fmt.Printf(`Usage:
  vg [command] [flags]

Commands:
  check      Scan packages and print security status table (exits 1 on urgent CVEs)
  apply      Execute non-interactive CLI package upgrades directly in terminal
  history    View local persistent audit log (~/.local/state/vanguard/history.jsonl)
  sweep      Run deep system-wide maintenance sweep via Topgrade (git, toolchains, firmware)
  monitor    Run background security monitor daemon with macOS alerts
  dashboard  Launch interactive web control dashboard (default)
  version    Print Vanguard version and platform information

Dashboard Flags:
  -port int     HTTP server port (default 9099)
  -no-web       Do not automatically open dashboard in browser (headless daemon mode)
  -no-sudo      Skip privilege pre-flight priming
  -dev          Run in development mode (reverse proxy to Vite dev server)
`)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if len(os.Args) > 1 {
		cmd := os.Args[1]
		switch cmd {
		case "check":
			os.Exit(runCheck(ctx, os.Args[2:]))
		case "apply":
			os.Exit(runApply(ctx, os.Args[2:]))
		case "history":
			os.Exit(runHistory(os.Args[2:]))
		case "sweep", "topgrade":
			os.Exit(runSweep(ctx))
		case "monitor":
			os.Exit(runMonitor(ctx, os.Args[2:]))
		case "version", "-v", "--version":
			fmt.Printf("Vanguard v%s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
			return
		case "help", "-h", "--help":
			printUsage()
			return
		case "dashboard", "web", "ui":
			// Strip subcommand and parse dashboard flags
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
		}
	}

	runDashboard(ctx)
}

func runDashboard(ctx context.Context) {
	devMode := flag.Bool("dev", false, "Enable development mode")
	port := flag.Int("port", 9099, "HTTP server port (0 for random open port)")
	noWeb := flag.Bool("no-web", false, "Do not automatically open dashboard in browser")
	noOpen := flag.Bool("no-open", false, "Alias for -no-web")
	noSudo := flag.Bool("no-sudo", false, "Skip privilege pre-flight priming")
	flag.Parse()

	isHeadless := *noWeb || *noOpen

	fmt.Print(ui.Banner(Version, runtime.GOOS+"/"+runtime.GOARCH))

	// 1. Privilege pre-flight & Keepalive
	_ = config.EnsureMPMConfig()
	if !*noSudo {
		fmt.Print("  [1/3] Checking privilege cache... ")
		if exec.Command("sudo", "-n", "true").Run() == nil {
			fmt.Println("✓ Active (Touch ID / sudo cached)")
			auth.StartKeepAlive(ctx, 40*time.Second)
		} else {
			stat, err := os.Stdin.Stat()
			isTTY := err == nil && (stat.Mode()&os.ModeCharDevice) != 0
			if !isHeadless && isTTY {
				if err := auth.EnsureSudo(); err != nil {
					fmt.Printf("⚠️  Privilege priming skipped (%v)\n", err)
				} else {
					fmt.Println("✓ Active (Touch ID / sudo cached)")
					auth.StartKeepAlive(ctx, 40*time.Second)
				}
			} else {
				fmt.Println("⚡ On-demand (prompts only when privileged upgrades run)")
			}
		}
	} else {
		fmt.Println("  [1/3] Privilege priming skipped (--no-sudo)")
	}

	// 2. Load embedded static assets
	embeddedFS, err := vanguard.StaticFS()
	if err != nil {
		fmt.Printf("⚠️  Warning: embedded assets error: %v\n", err)
	}

	// 3. Initialize services
	pkgScanner := scanner.New()
	pkgExecutor := executor.New()

	srv := server.New(server.Config{
		Version:    Version,
		DevMode:    *devMode,
		ViteDevURL: "http://127.0.0.1:5173",
		StaticFS:   embeddedFS,
		Scanner:    pkgScanner,
		Executor:   pkgExecutor,
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ✗ Failed to bind listener: %v\n", err)
			os.Exit(1)
		}
	}

	assignedPort := listener.Addr().(*net.TCPAddr).Port
	appURL := fmt.Sprintf("http://127.0.0.1:%d", assignedPort)

	fmt.Printf("  [2/3] Server listening on %s\n", appURL)

	httpServer := &http.Server{
		Handler: srv,
	}

	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "  ✗ Server error: %v\n", err)
		}
	}()

	// 4. Open browser
	if !isHeadless {
		fmt.Printf("  [3/3] Opening dashboard in your default browser...\n\n")
		go openBrowser(appURL)
	} else {
		fmt.Printf("  [3/3] Dashboard ready at %s (headless mode)\n\n", appURL)
	}

	fmt.Printf("  Press Ctrl+C to stop the background server.\n\n")

	<-ctx.Done()
	fmt.Println("\n  Shutting down Vanguard server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	fmt.Println("  Goodbye.")
}

func openBrowser(url string) {
	time.Sleep(200 * time.Millisecond)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	if cmd != nil {
		_ = cmd.Start()
	}
}
