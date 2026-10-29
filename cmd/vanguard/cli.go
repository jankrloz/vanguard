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
	"time"

	"github.com/jankrloz/vanguard/internal/auth"
	"github.com/jankrloz/vanguard/internal/config"
	"github.com/jankrloz/vanguard/internal/domain"
	"github.com/jankrloz/vanguard/internal/executor"
	"github.com/jankrloz/vanguard/internal/history"
	"github.com/jankrloz/vanguard/internal/scanner"
	"github.com/jankrloz/vanguard/internal/ui"
)

func runCheck(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	secOnly := fs.Bool("security-only", false, "Display only packages with known CVEs")
	mgrFilter := fs.String("manager", "", "Filter display to a specific package manager")
	limit := fs.Int("limit", 0, "Limit number of packages to print (0 for all)")
	_ = fs.Parse(args)

	fmt.Print(ui.Banner(Version, runtime.GOOS+"/"+runtime.GOARCH))

	stopSpinner := ui.StartSpinner("Scanning packages across system registries & querying OSV.dev advisories")
	s := scanner.New()
	res, err := s.Scan(ctx)
	if err != nil {
		stopSpinner("")
		fmt.Fprintf(os.Stderr, "  %s✗ Scan failed: %v%s\n", ui.Rose, err, ui.Reset)
		return 1
	}
	stopSpinner(fmt.Sprintf("Discovered %d outdated packages across active registries", len(res.Packages)))

	var candidates []domain.Package
	for _, p := range res.Packages {
		if *secOnly && !p.HasVulnerability {
			continue
		}
		if *mgrFilter != "" && !strings.EqualFold(p.Manager, *mgrFilter) {
			continue
		}
		candidates = append(candidates, p)
	}

	if len(candidates) == 0 {
		fmt.Printf("  %s✓ All packages are up to date and clean of known vulnerabilities.%s\n\n", ui.Emerald, ui.Reset)
		return 0
	}

	// Categorize
	var cves, safes, majors, others []domain.Package
	for _, p := range candidates {
		if p.HasVulnerability {
			cves = append(cves, p)
		} else if p.ChangeType == domain.ChangeMajor {
			majors = append(majors, p)
		} else if p.Safe {
			safes = append(safes, p)
		} else {
			others = append(others, p)
		}
	}

	printGroup := func(title string, pkgs []domain.Package, headerColor string) {
		if len(pkgs) == 0 {
			return
		}
		fmt.Println(ui.SectionHeader(title, len(pkgs), headerColor))
		for i, p := range pkgs {
			if *limit > 0 && i >= *limit {
				fmt.Printf("    %s... and %d more items%s\n", ui.DarkZinc, len(pkgs)-*limit, ui.Reset)
				break
			}
			badge := ui.BadgeSafe()
			if p.HasVulnerability {
				badge = ui.BadgeCVE(p.Vulnerabilities[0].ID, p.MaxSeverity)
			} else if p.ChangeType == domain.ChangeMajor {
				badge = ui.BadgeMajor()
			}
			pinBadge := ""
			if p.Pinned {
				pinBadge = fmt.Sprintf(" %s[PINNED]%s", ui.Amber, ui.Reset)
			}

			fmt.Printf("  %s %s%-20s%s %s%s → %s%s %s%s\n",
				ui.BadgeManager(p.Manager),
				ui.Bold+ui.White, p.Name, ui.Reset,
				ui.Zinc, p.CurrentVersion,
				ui.White, p.LatestVersion+ui.Reset,
				badge,
				pinBadge,
			)

			if p.HasVulnerability && len(p.Vulnerabilities) > 0 {
				for _, v := range p.Vulnerabilities {
					summary := v.Summary
					if summary == "" {
						summary = "Security advisory fix available in upstream release"
					}
					fmt.Printf("    %s🚨 %s: %s%s\n", ui.Rose, v.ID, summary, ui.Reset)
				}
			}

			if p.Description != "" {
				fmt.Printf("    %s└─ %s%s%s\n", ui.DarkZinc, ui.Zinc, p.Description, ui.Reset)
			}
		}
	}

	printGroup("CRITICAL & SECURITY ADVISORIES", cves, ui.Rose)
	printGroup("SAFE MINOR & PATCH UPDATES", safes, ui.Emerald)
	printGroup("MAJOR BREAKING UPDATES", majors, ui.Amber)
	printGroup("OTHER PACKAGES", others, ui.Zinc)

	summaryText := fmt.Sprintf(
		"Total: %s%d%s │ Critical: %s%d%s │ High: %s%d%s │ Med: %s%d%s │ Safe: %s%d%s │ Major: %s%d%s │ Pinned: %d",
		ui.White, res.Summary.TotalPackages, ui.Reset,
		ui.Rose+ui.Bold, res.Summary.CriticalCount, ui.Reset,
		ui.Amber, res.Summary.HighCount, ui.Reset,
		ui.Zinc, res.Summary.MediumCount, ui.Reset,
		ui.Emerald, res.Summary.SafeCount, ui.Reset,
		ui.Amber, res.Summary.MajorCount, ui.Reset,
		res.Summary.PinnedCount,
	)
	fmt.Printf("\n  %s\n  %s\n\n", ui.DarkZinc+strings.Repeat("─", 68)+ui.Reset, summaryText)

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
	_ = config.EnsureMPMConfig()

	stopSpinner := ui.StartSpinner("Scanning packages across registries for update candidates")
	s := scanner.New()
	res, err := s.Scan(ctx)
	if err != nil {
		stopSpinner("")
		fmt.Fprintf(os.Stderr, "  ✗ Scan failed: %v\n", err)
		return 1
	}
	stopSpinner(fmt.Sprintf("Scan completed (%d candidate packages found)", len(res.Packages)))

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
		fmt.Printf("  %s✓ All packages are up to date and protected.%s\n\n", ui.Emerald, ui.Reset)
		return 0
	}

	fmt.Printf("\n  %sTargeting %d packages for upgrade:%s\n", ui.Bold+ui.White, len(candidates), ui.Reset)
	for _, p := range candidates {
		badge := ui.BadgeSafe()
		if p.HasVulnerability {
			badge = ui.BadgeCVE(p.Vulnerabilities[0].ID, p.MaxSeverity)
		} else if p.ChangeType == domain.ChangeMajor {
			badge = ui.BadgeMajor()
		}

		fmt.Printf("    • %s %s%-18s%s %s%s → %s%s %s\n",
			ui.BadgeManager(p.Manager),
			ui.Bold+ui.White, p.Name, ui.Reset,
			ui.Zinc, p.CurrentVersion,
			ui.White, p.LatestVersion+ui.Reset,
			badge,
		)
		if p.HasVulnerability && len(p.Vulnerabilities) > 0 {
			for _, v := range p.Vulnerabilities {
				summary := v.Summary
				if summary == "" {
					summary = "Security advisory fix available in upstream release"
				}
				fmt.Printf("      %s🚨 %s: %s%s\n", ui.Rose, v.ID, summary, ui.Reset)
			}
		}
		if p.Description != "" {
			fmt.Printf("      %s└─ %s%s%s\n", ui.DarkZinc, ui.Zinc, p.Description, ui.Reset)
		}
	}

	if !yes {
		fmt.Printf("\n  %sProceed with upgrade? [y/N]:%s ", ui.Bold+ui.Cyan, ui.Reset)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Printf("  %sAborted by user.%s\n\n", ui.Zinc, ui.Reset)
			return 0
		}
	}

	fmt.Printf("\n  %sExecuting upgrades...%s\n", ui.Cyan, ui.Reset)
	execEngine := executor.New()

	err = execEngine.ExecuteUpgrades(ctx, candidates, func(ev domain.UpgradeEvent) {
		switch ev.Type {
		case "start":
			fmt.Printf("  %s▶%s %s\n", ui.Cyan, ui.Reset, ev.Message)
		case "package_start":
			fmt.Printf("\n  %s==>%s %s%s%s\n", ui.Cyan, ui.Reset, ui.Bold+ui.White, ev.Message, ui.Reset)
		case "log":
			fmt.Printf("    %s%s%s\n", ui.Zinc, ev.Message, ui.Reset)
		case "package_done":
			if ev.Status == "ok" {
				fmt.Printf("  %s✓ %s%s\n", ui.Emerald, ev.Message, ui.Reset)
			} else {
				fmt.Printf("  %s✗ %s%s\n", ui.Rose, ev.Message, ui.Reset)
			}
		case "complete":
			fmt.Printf("\n  %s✓ %s%s\n\n", ui.Emerald+ui.Bold, ev.Message, ui.Reset)
		case "error":
			fmt.Printf("\n  %s✗ %s%s\n\n", ui.Rose+ui.Bold, ev.Message, ui.Reset)
		}
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "  %s✗ Upgrade run failed: %v%s\n", ui.Rose, err, ui.Reset)
		return 1
	}

	return 0
}

func runHistory(args []string) int {
	fs := flag.NewFlagSet("history", flag.ExitOnError)
	limit := fs.Int("limit", 25, "Maximum number of history records to display")
	_ = fs.Parse(args)

	fmt.Print(ui.Banner(Version, runtime.GOOS+"/"+runtime.GOARCH))
	fmt.Printf("  %sAudit Upgrade History Log%s\n", ui.Bold+ui.White, ui.Reset)
	fmt.Println(ui.DarkZinc + strings.Repeat("─", 68) + ui.Reset)

	entries, err := history.ReadRecent(*limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  %s✗ Failed to read audit log: %v%s\n", ui.Rose, err, ui.Reset)
		return 1
	}

	if len(entries) == 0 {
		fmt.Printf("  No upgrade history recorded yet in %s\n\n", history.LogPath())
		return 0
	}

	for _, e := range entries {
		statusColor := ui.Emerald
		if e.Status != "ok" {
			statusColor = ui.Rose
		}

		fmt.Printf("  %s%s%s  %s  %s%-22s%s  %s%s → %s%s  %s[%s]%s\n",
			ui.DarkZinc, e.Timestamp.Format("2006-01-02 15:04:05"), ui.Reset,
			ui.BadgeManager(e.Manager),
			ui.Bold+ui.White, e.PackageID, ui.Reset,
			ui.Zinc, e.FromVersion,
			ui.White, e.ToVersion,
			statusColor, e.Status, ui.Reset,
		)
	}

	fmt.Printf("\n  %sShowing %d most recent records from %s%s\n\n", ui.DarkZinc, len(entries), history.LogPath(), ui.Reset)
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
	fmt.Print(ui.Banner(Version, runtime.GOOS+"/"+runtime.GOARCH))
	fmt.Printf("  %sDeep System Maintenance Sweep (Topgrade)%s\n", ui.Bold+ui.White, ui.Reset)
	fmt.Println(ui.DarkZinc + strings.Repeat("─", 68) + ui.Reset)
	fmt.Printf("  %s• Git repositories, submodules, and clean worktrees%s\n", ui.Zinc, ui.Reset)
	fmt.Printf("  %s• Toolchains & environments (Rust, Go, Python, Node, Ruby)%s\n", ui.Zinc, ui.Reset)
	fmt.Printf("  %s• Shell frameworks, editor extensions, containers%s\n", ui.Zinc, ui.Reset)
	fmt.Printf("  %s• macOS App Store & system package caches%s\n\n", ui.Zinc, ui.Reset)

	_ = auth.EnsureSudo()
	_ = config.EnsureMPMConfig()

	execEngine := executor.New()
	err := execEngine.ExecuteTopgrade(ctx, func(ev domain.UpgradeEvent) {
		switch ev.Type {
		case "start":
			fmt.Printf("  %s▶%s %s\n", ui.Cyan, ui.Reset, ev.Message)
		case "log":
			fmt.Printf("    %s%s%s\n", ui.Zinc, ev.Message, ui.Reset)
		case "package_done":
			if ev.Status == "ok" {
				fmt.Printf("  %s✓ %s%s\n", ui.Emerald, ev.Message, ui.Reset)
			} else {
				fmt.Printf("  %s✗ %s%s\n", ui.Rose, ev.Message, ui.Reset)
			}
		case "complete":
			fmt.Printf("\n  %s✓ %s%s\n\n", ui.Emerald+ui.Bold, ev.Message, ui.Reset)
		case "error":
			fmt.Printf("\n  %s✗ %s%s\n\n", ui.Rose+ui.Bold, ev.Message, ui.Reset)
		}
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "  %s✗ Sweep failed: %v%s\n\n", ui.Rose, err, ui.Reset)
		return 1
	}
	return 0
}
