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
	"github.com/jankrloz/vanguard/internal/executor"
	"github.com/jankrloz/vanguard/internal/scanner"
	"github.com/jankrloz/vanguard/internal/server"
)

const Version = "0.1.0"

func main() {
	devMode := flag.Bool("dev", false, "Enable development mode (reverse proxy to Vite dev server)")
	port := flag.Int("port", 9099, "HTTP server port (0 for random open port)")
	noOpen := flag.Bool("no-open", false, "Do not automatically open browser on startup")
	noSudo := flag.Bool("no-sudo", false, "Skip privilege pre-flight priming")
	showVersion := flag.Bool("version", false, "Print Vanguard version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("Vanguard v%s (%s/%s)\n", Version, runtime.GOOS, runtime.GOARCH)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("\n  ▲ VANGUARD — Vulnerability Shield & Package Orchestrator")
	fmt.Println("  ────────────────────────────────────────────────────────")

	// 1. Privilege pre-flight & Keepalive
	if !*noSudo {
		fmt.Print("  [1/3] Initializing privilege keepalive... ")
		if err := auth.EnsureSudo(); err != nil {
			fmt.Printf("⚠️  Warning: privilege priming failed: %v\n", err)
		} else {
			fmt.Println("✓ Active")
			auth.StartKeepAlive(ctx, 40*time.Second)
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
		DevMode:    *devMode,
		ViteDevURL: "http://127.0.0.1:5173",
		StaticFS:   embeddedFS,
		Scanner:    pkgScanner,
		Executor:   pkgExecutor,
	})

	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		// Fallback to random free port
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
	if !*noOpen {
		fmt.Printf("  [3/3] Opening dashboard in your default browser...\n\n")
		go openBrowser(appURL)
	} else {
		fmt.Printf("  [3/3] Dashboard ready at %s\n\n", appURL)
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
