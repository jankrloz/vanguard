package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/jankrloz/vanguard/internal/auth"
	"github.com/jankrloz/vanguard/internal/domain"
	"github.com/jankrloz/vanguard/internal/executor"
	"github.com/jankrloz/vanguard/internal/history"
	"github.com/jankrloz/vanguard/internal/scanner"
)

func runCheck(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	secOnly := fs.Bool("security-only", false, "Display only packages with known CVEs")
	mgrFilter := fs.String("manager", "", "Filter display to a specific package manager")
	limit := fs.Int("limit", 0, "Limit number of packages to print (0 for all)")
	_ = fs.Parse(args)

	fmt.Println("\n  ▲ VANGUARD — Security & Outdated Package Check")
	fmt.Println("  ───────────────────────────────────────────────")

	s := scanner.New()
	res, err := s.Scan(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Scan failed: %v\n", err)
		return 1
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "  MANAGER\tPACKAGE\tCURRENT\tLATEST\tCHANGE\tSECURITY ADVISORY")
	fmt.Fprintln(w, "  ───────\t───────\t───────\t──────\t──────\t─────────────────")

	count := 0
	for _, p := range res.Packages {
		if *secOnly && !p.HasVulnerability {
			continue
		}
		if *mgrFilter != "" && !strings.EqualFold(p.Manager, *mgrFilter) {
			continue
		}
		count++
		if *limit > 0 && count > *limit {
			continue
		}

		secText := "None"
		if p.HasVulnerability {
			var ids []string
			for _, v := range p.Vulnerabilities {
				ids = append(ids, v.ID)
			}
			secText = fmt.Sprintf("🚨 %s [%s]", p.MaxSeverity, strings.Join(ids, ", "))
		}

		pinMark := ""
		if p.Pinned {
			pinMark = " (pinned)"
		}

		fmt.Fprintf(w, "  %s\t%s%s\t%s\t%s\t%s\t%s\n",
			p.Manager,
			p.Name,
			pinMark,
			p.CurrentVersion,
			p.LatestVersion,
			p.ChangeType,
			secText,
		)
	}
	_ = w.Flush()

	if *limit > 0 && count > *limit {
		fmt.Printf("  ... and %d more packages (use --limit 0 to print all)\n", count-*limit)
	}

	fmt.Println("  ───────────────────────────────────────────────")
	fmt.Printf("  Total Outdated: %d | Safe Minor/Patch: %d | Major Breaking: %d | Urgent CVEs: %d\n\n",
		res.Summary.TotalPackages,
		res.Summary.SafeCount,
		res.Summary.MajorCount,
		res.Summary.SecurityFixCount,
	)

	if res.Summary.SecurityFixCount > 0 {
		return 1
	}
	return 0
}

func runApply(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	secOnly := fs.Bool("security-only", false, "Upgrade only packages fixing known CVEs")
	all := fs.Bool("all", false, "Upgrade all outdated packages (including major breaking)")
	mgrFilter := fs.String("manager", "", "Target a specific package manager only")
	autoYes := fs.Bool("yes", false, "Auto-confirm upgrade prompt without interactive input")
	autoYesShort := fs.Bool("y", false, "Auto-confirm upgrade prompt")
	_ = fs.Parse(args)

	yes := *autoYes || *autoYesShort

	fmt.Println("\n  ▲ VANGUARD — Headless Package Upgrade")
	fmt.Println("  ──────────────────────────────────────")

	_ = auth.EnsureSudo()

	s := scanner.New()
	res, err := s.Scan(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Scan failed: %v\n", err)
		return 1
	}

	var candidates []domain.Package
	for _, p := range res.Packages {
		if p.Pinned {
			continue
		}
		if *mgrFilter != "" && !strings.EqualFold(p.Manager, *mgrFilter) {
			continue
		}
		if *secOnly {
			if p.HasVulnerability {
				candidates = append(candidates, p)
			}
			continue
		}
		if *all {
			candidates = append(candidates, p)
			continue
		}
		// Default safe mode: minor, patch, or security fix
		if p.Safe || p.HasVulnerability {
			candidates = append(candidates, p)
		}
	}

	if len(candidates) == 0 {
		fmt.Println("  ✓ All packages are up to date and protected.")
		return 0
	}

	fmt.Printf("  Targeting %d packages for upgrade:\n", len(candidates))
	for _, p := range candidates {
		tag := string(p.ChangeType)
		if p.HasVulnerability {
			tag = "🚨 CVE FIX"
		}
		fmt.Printf("    • [%s] %s %s -> %s (%s)\n", p.Manager, p.Name, p.CurrentVersion, p.LatestVersion, tag)
	}

	if !yes {
		fmt.Print("\n  Proceed with upgrade? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Println("  Aborted by user.")
			return 0
		}
	}

	fmt.Println("\n  Executing upgrades...")
	execEngine := executor.New()

	err = execEngine.ExecuteUpgrades(ctx, candidates, func(ev domain.UpgradeEvent) {
		switch ev.Type {
		case "start":
			fmt.Printf("  ▶ %s\n", ev.Message)
		case "package_start":
			fmt.Printf("\n  ==> %s\n", ev.Message)
		case "log":
			fmt.Printf("    %s\n", ev.Message)
		case "package_done":
			if ev.Status == "ok" {
				fmt.Printf("  ✓ %s\n", ev.Message)
			} else {
				fmt.Printf("  ✗ %s\n", ev.Message)
			}
		case "complete":
			fmt.Printf("\n  ✓ %s\n", ev.Message)
		case "error":
			fmt.Printf("\n  ✗ %s\n", ev.Message)
		}
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Upgrade run failed: %v\n", err)
		return 1
	}

	return 0
}

func runHistory(args []string) int {
	fs := flag.NewFlagSet("history", flag.ExitOnError)
	limit := fs.Int("limit", 25, "Maximum number of history records to display")
	_ = fs.Parse(args)

	fmt.Println("\n  ▲ VANGUARD — Audit History Log")
	fmt.Println("  ───────────────────────────────")

	entries, err := history.ReadRecent(*limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Failed to read audit log: %v\n", err)
		return 1
	}

	if len(entries) == 0 {
		fmt.Printf("  No upgrade history recorded yet in %s\n\n", history.LogPath())
		return 0
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "  TIMESTAMP\tPACKAGE\tMANAGER\tFROM\tTO\tSTATUS")
	fmt.Fprintln(w, "  ─────────\t───────\t───────\t────\t──\t──────")

	for _, e := range entries {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\t%s\n",
			e.Timestamp.Format("2006-01-02 15:04:05"),
			e.PackageID,
			e.Manager,
			e.FromVersion,
			e.ToVersion,
			e.Status,
		)
	}
	_ = w.Flush()
	fmt.Printf("\n  Showing %d most recent records from %s\n\n", len(entries), history.LogPath())
	return 0
}

func runMonitor(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	interval := fs.Duration("interval", 1*time.Hour, "Check frequency interval (e.g. 30m, 1h, 4h)")
	_ = fs.Parse(args)

	fmt.Println("\n  ▲ VANGUARD — Background Security Monitor")
	fmt.Printf("  Active polling interval: %v\n", *interval)
	fmt.Println("  Press Ctrl+C to terminate.")

	s := scanner.New()

	runCheckOnce := func() {
		fmt.Printf("  [%s] Scanning for vulnerabilities...\n", time.Now().Format("15:04:05"))
		res, err := s.Scan(ctx)
		if err != nil {
			fmt.Printf("  ⚠️  Scan failed: %v\n", err)
			return
		}

		if res.Summary.SecurityFixCount > 0 {
			msg := fmt.Sprintf("%d urgent security CVE fixes available across your packages.", res.Summary.SecurityFixCount)
			fmt.Printf("  🚨 ALERT: %s\n", msg)

			if runtime.GOOS == "darwin" {
				script := fmt.Sprintf(`display notification %q with title "Vanguard Security Shield" sound name "Ping"`, msg)
				_ = exec.Command("osascript", "-e", script).Run()
			}
		} else {
			fmt.Printf("  ✓ System clean (%d outdated packages, 0 CVEs)\n", res.Summary.TotalPackages)
		}
	}

	runCheckOnce()

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\n  Monitor stopped.")
			return 0
		case <-ticker.C:
			runCheckOnce()
		}
	}
}

func runSweep(ctx context.Context) int {
	fmt.Println("\n  ▲ VANGUARD — Deep System Maintenance (Topgrade)")
	fmt.Println("  Running system-wide sweep across:")
	fmt.Println("  • Git repositories, submodules, and worktrees")
	fmt.Println("  • Firmware, language toolchains (rustup, pyenv, nvm)")
	fmt.Println("  • Shell frameworks, editor extensions, containers")
	fmt.Println("  • macOS App Store & system caches")
	fmt.Println("  ─────────────────────────────────────────────────")

	_ = auth.EnsureSudo()

	execEngine := executor.New()
	err := execEngine.ExecuteTopgrade(ctx, func(ev domain.UpgradeEvent) {
		switch ev.Type {
		case "start":
			fmt.Printf("  ▶ %s\n", ev.Message)
		case "log":
			fmt.Printf("    %s\n", ev.Message)
		case "package_done":
			fmt.Printf("  ==> %s\n", ev.Message)
		case "complete":
			fmt.Printf("  ✓ %s\n", ev.Message)
		case "error":
			fmt.Printf("  ✗ %s\n", ev.Message)
		}
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "  ✗ Sweep failed: %v\n", err)
		return 1
	}
	return 0
}
