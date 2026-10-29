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
	"strings"
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

const Version = "0.3.1"

func printUsage() {
	fmt.Print(ui.Banner(Version, runtime.GOOS+"/"+runtime.GOARCH))
	fmt.Printf("  %s\n", ui.StyleWhiteBold.Render("USAGE"))
	fmt.Printf("    %s %s\n\n", ui.StyleCyanBold.Render("vg"), ui.StyleZinc.Render("[command] [flags]"))

	fmt.Printf("  %s\n", ui.StyleWhiteBold.Render("COMMANDS"))
	cmds := [][2]string{
		{"check", "Scan packages and print security status table (exits 1 on urgent CVEs)"},
		{"apply", "Execute CLI package upgrades in terminal (--all, --security-only, --major, --force, -y)"},
		{"history", "View persistent upgrade audit trail (~/.local/state/vanguard/history.jsonl)"},
		{"sweep", "Run deep system-wide maintenance sweep via Topgrade (git, toolchains, firmware)"},
		{"monitor", "Run background security monitor daemon with desktop alert notifications"},
		{"dashboard", "Launch interactive Web Control Center (default)"},
		{"version", "Print Vanguard version and platform information"},
	}
	for _, c := range cmds {
		pad := strings.Repeat(" ", max(0, 12-len(c[0])))
		fmt.Printf("    %s%s%s\n", ui.StyleCyanBold.Render(c[0]), pad, ui.StyleZinc.Render(c[1]))
	}

	fmt.Printf("\n  %s\n", ui.StyleWhiteBold.Render("DASHBOARD FLAGS"))
	flags := [][2]string{
		{"-port int", "HTTP server port (default 9099)"},
		{"-no-web", "Run server in headless background mode without opening a browser"},
		{"-no-sudo", "Skip privilege pre-flight priming"},
		{"-dev", "Run in development mode (reverse proxy to Vite dev server)"},
	}
	for _, f := range flags {
		pad := strings.Repeat(" ", max(0, 12-len(f[0])))
		fmt.Printf("    %s%s%s\n", ui.StyleWhiteBold.Render(f[0]), pad, ui.StyleZinc.Render(f[1]))
	}
	fmt.Println()
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
