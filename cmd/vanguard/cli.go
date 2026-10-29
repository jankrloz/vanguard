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

func computeColumnWidths(pkgs []domain.Package) (mgrWidth, nameWidth, verWidth int) {
	mgrWidth = 4
	nameWidth = 20
	verWidth = 15
	for _, p := range pkgs {
		if len(p.Manager) > mgrWidth {
			mgrWidth = len(p.Manager)
		}
		if len(p.Name) > nameWidth {
			nameWidth = len(p.Name)
		}
		vLen := len(p.CurrentVersion) + 3 + len(p.LatestVersion)
		if vLen > verWidth {
			verWidth = vLen
		}
	}
	if mgrWidth > 10 {
		mgrWidth = 10
	}
	if nameWidth > 28 {
		nameWidth = 28
	}
	if verWidth > 26 {
		verWidth = 26
	}
	return mgrWidth, nameWidth, verWidth
}

func formatPackageRow(p domain.Package, mgrWidth, nameWidth, verWidth int) string {
	var sb strings.Builder

	badge := ui.BadgeSafe()
	if p.HasVulnerability && len(p.Vulnerabilities) > 0 {
		badge = ui.BadgeCVE(p.Vulnerabilities[0].ID, p.MaxSeverity)
	} else if p.ChangeType == domain.ChangeMajor {
		badge = ui.BadgeMajor()
	}

	pinBadge := ""
	if p.Pinned {
		pinBadge = " " + ui.BadgePinned()
	}

	displayName := p.Name
	if len(displayName) > nameWidth {
		displayName = displayName[:nameWidth-1] + "…"
	}
	namePad := strings.Repeat(" ", max(0, nameWidth-len(displayName)))

	rawVer := p.CurrentVersion + " → " + p.LatestVersion
	verRuneLen := len(p.CurrentVersion) + 3 + len(p.LatestVersion)
	verPad := strings.Repeat(" ", max(0, verWidth-verRuneLen))
	styledVer := ui.StyleSubtle.Render(p.CurrentVersion+" →") + " " + ui.StyleWhiteBold.Render(p.LatestVersion) + verPad
	_ = rawVer

	sb.WriteString(fmt.Sprintf("  %s  %s%s  %s  %s%s\n",
		ui.BadgeManagerPadded(p.Manager, mgrWidth),
		ui.StyleWhiteBold.Render(displayName),
		namePad,
		styledVer,
		badge,
		pinBadge,
	))

	subIndent := strings.Repeat(" ", 4)
	if p.HasVulnerability && len(p.Vulnerabilities) > 0 {
		for _, v := range p.Vulnerabilities {
			summary := v.Summary
			if summary == "" {
				summary = "Security advisory fix available in upstream release"
			}
			srcTag := ""
			if v.Source != "" {
				srcTag = " " + ui.StyleSubtle.Render("["+v.Source+"]")
			}
			sb.WriteString(fmt.Sprintf("%s%s%s: %s\n",
				subIndent,
				ui.StyleRoseBold.Render("🚨 "+v.ID),
				srcTag,
				ui.StyleRoseText.Render(summary),
			))
		}
	}

	if p.Description != "" && (len(p.Vulnerabilities) == 0 || p.Description != p.Vulnerabilities[0].Summary) {
		sb.WriteString(fmt.Sprintf("%s%s %s\n",
			subIndent,
			ui.StyleSubtle.Render("└─"),
			ui.StyleZinc.Render(p.Description),
		))
	}

	return sb.String()
}

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

	mgrWidth, nameWidth, verWidth := computeColumnWidths(candidates)

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
				fmt.Printf("    %s\n", ui.StyleSubtle.Render(fmt.Sprintf("... and %d more items", len(pkgs)-*limit)))
				break
			}
			fmt.Print(formatPackageRow(p, mgrWidth, nameWidth, verWidth))
		}
	}

	printGroup("CRITICAL & SECURITY ADVISORIES", cves, ui.Rose)
	printGroup("SAFE MINOR & PATCH UPDATES", safes, ui.Emerald)
	printGroup("MAJOR BREAKING UPDATES", majors, ui.Amber)
	printGroup("OTHER PACKAGES", others, ui.Zinc)

	sep := ui.StyleSubtle.Render(" │ ")
	summaryText := strings.Join([]string{
		"Total: " + ui.StyleWhiteBold.Render(fmt.Sprintf("%d", res.Summary.TotalPackages)),
		"Critical: " + ui.StyleRoseBold.Render(fmt.Sprintf("%d", res.Summary.CriticalCount)),
		"High: " + ui.StyleOrangeBold.Render(fmt.Sprintf("%d", res.Summary.HighCount)),
		"Med: " + ui.StyleAmberBold.Render(fmt.Sprintf("%d", res.Summary.MediumCount)),
		"Safe: " + ui.StyleEmeraldBold.Render(fmt.Sprintf("%d", res.Summary.SafeCount)),
		"Major: " + ui.StyleAmberBold.Render(fmt.Sprintf("%d", res.Summary.MajorCount)),
		"Pinned: " + ui.StyleZinc.Render(fmt.Sprintf("%d", res.Summary.PinnedCount)),
	}, sep)

	divider := ui.StyleSubtle.Render(strings.Repeat("─", 76))
	fmt.Printf("\n  %s\n  %s\n\n", divider, summaryText)

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

	fmt.Print(ui.Banner(Version, runtime.GOOS+"/"+runtime.GOARCH))

	_ = auth.EnsureSudo()
	_ = config.EnsureMPMConfig()

	stopSpinner := ui.StartSpinner("Scanning packages across registries for update candidates")
	s := scanner.New()
	res, err := s.Scan(ctx)
	if err != nil {
		stopSpinner("")
		fmt.Fprintf(os.Stderr, "  %s✗ Scan failed: %v%s\n", ui.Rose, err, ui.Reset)
		return 1
	}
	stopSpinner(fmt.Sprintf("Discovered %d outdated packages (%d safe/security, %d major breaking)",
		len(res.Packages),
		res.Summary.SafeCount+res.Summary.SecurityFixCount,
		res.Summary.MajorCount,
	))

	var candidates []domain.Package
	var skippedMajors []domain.Package
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
		} else if p.ChangeType == domain.ChangeMajor {
			skippedMajors = append(skippedMajors, p)
		}
	}

	if len(candidates) == 0 {
		if len(skippedMajors) > 0 {
			fmt.Printf("  %s %s\n",
				ui.StyleEmeraldBold.Render("✓"),
				ui.StyleWhite.Render("All safe minor/patch and security updates are up to date."),
			)
			mgrW, nameW, verW := computeColumnWidths(skippedMajors)
			fmt.Println(ui.SectionHeader("HELD BACK — MAJOR BREAKING UPDATES", len(skippedMajors), ui.Amber))
			for _, p := range skippedMajors {
				fmt.Print(formatPackageRow(p, mgrW, nameW, verW))
			}
			fmt.Printf("\n  %s %s\n\n",
				ui.StyleAmberBold.Render("ℹ"),
				ui.StyleZinc.Render("Run 'vg apply --all' to upgrade major breaking releases."),
			)
			return 0
		}
		fmt.Printf("  %s✓ All packages are up to date and protected.%s\n\n", ui.Emerald, ui.Reset)
		return 0
	}

	mgrWidth, nameWidth, verWidth := computeColumnWidths(candidates)

	fmt.Println(ui.SectionHeader("QUEUED FOR UPGRADE", len(candidates), "#00F0FF"))
	for _, p := range candidates {
		fmt.Print(formatPackageRow(p, mgrWidth, nameWidth, verWidth))
	}
	if len(skippedMajors) > 0 {
		fmt.Printf("\n  %s %s\n",
			ui.StyleAmberBold.Render("ℹ"),
			ui.StyleZinc.Render(fmt.Sprintf("%d major breaking update(s) held back for safety (pass --all to include).", len(skippedMajors))),
		)
	}

	if !yes {
		fmt.Printf("\n  %s ", ui.StyleCyanBold.Render("Proceed with upgrade? [y/N]:"))
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			fmt.Printf("  %s\n\n", ui.StyleZinc.Render("Aborted by user."))
			return 0
		}
	}

	fmt.Printf("\n  %s\n", ui.StyleCyanBold.Render("Executing upgrades..."))
	execEngine := executor.New()
	totalCandidates := len(candidates)
	completedCount := 0
	pkgCountByMgr := make(map[string]int)
	for _, p := range candidates {
		pkgCountByMgr[p.Manager]++
	}

	err = execEngine.ExecuteUpgrades(ctx, candidates, func(ev domain.UpgradeEvent) {
		switch ev.Type {
		case "start":
			fmt.Printf("  %s %s\n", ui.StyleCyanBold.Render("▶"), ui.StyleZinc.Render(ev.Message))
		case "package_start":
			fmt.Printf("\n  %s %s  %s\n",
				ui.StyleCyanBold.Render("==>"),
				ui.BadgeManager(ev.Package),
				ui.StyleWhiteBold.Render(strings.TrimPrefix(ev.Message, fmt.Sprintf("Upgrading [%s] packages: ", ev.Package))),
			)
		case "log":
			fmt.Printf("      %s %s\n", ui.StyleSubtle.Render("│"), ui.StyleZinc.Render(ev.Message))
		case "package_done":
			batch := pkgCountByMgr[ev.Package]
			if batch <= 0 {
				batch = 1
			}
			completedCount += batch
			if ev.Status == "ok" {
				fmt.Printf("  %s  %s\n", ui.StyleEmeraldBold.Render("✓ "+ev.Message), ui.RenderProgress(completedCount, totalCandidates))
			} else {
				fmt.Printf("  %s  %s\n", ui.StyleRoseBold.Render("✗ "+ev.Message), ui.RenderProgress(completedCount, totalCandidates))
			}
		case "complete":
			fmt.Printf("\n  %s\n\n", ui.StyleEmeraldBold.Render("✓ "+ev.Message))
		case "error":
			fmt.Printf("\n  %s\n\n", ui.StyleRoseBold.Render("✗ "+ev.Message))
		}
	})

	if err != nil {
		return 1
	}

	return 0
}

func runHistory(args []string) int {
	fs := flag.NewFlagSet("history", flag.ExitOnError)
	limit := fs.Int("limit", 25, "Maximum number of history records to display")
	_ = fs.Parse(args)

	fmt.Print(ui.Banner(Version, runtime.GOOS+"/"+runtime.GOARCH))
	fmt.Println(ui.SectionHeader("AUDIT UPGRADE HISTORY LOG", *limit, "#00F0FF"))

	entries, err := history.ReadRecent(*limit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  %s\n", ui.StyleRoseBold.Render(fmt.Sprintf("✗ Failed to read audit log: %v", err)))
		return 1
	}

	if len(entries) == 0 {
		fmt.Printf("  %s\n\n", ui.StyleZinc.Render("No upgrade history recorded yet in "+history.LogPath()))
		return 0
	}

	mgrWidth := 4
	nameWidth := 18
	verWidth := 15
	for _, e := range entries {
		if len(e.Manager) > mgrWidth {
			mgrWidth = len(e.Manager)
		}
		cleanPkg := strings.TrimPrefix(e.PackageID, e.Manager+":")
		if len(cleanPkg) > nameWidth {
			nameWidth = len(cleanPkg)
		}
		vLen := len(e.FromVersion) + 3 + len(e.ToVersion)
		if vLen > verWidth {
			verWidth = vLen
		}
	}
	if nameWidth > 28 {
		nameWidth = 28
	}
	if verWidth > 26 {
		verWidth = 26
	}

	for _, e := range entries {
		statusBadge := ui.StyleSafeBadge.Render("✓ OK")
		if e.Status != "ok" {
			statusBadge = ui.StyleCriticalBadge.Render("✗ FAILED")
		}
		cleanPkg := strings.TrimPrefix(e.PackageID, e.Manager+":")
		if len(cleanPkg) > nameWidth {
			cleanPkg = cleanPkg[:nameWidth-1] + "…"
		}
		namePad := strings.Repeat(" ", max(0, nameWidth-len(cleanPkg)))

		verRuneLen := len(e.FromVersion) + 3 + len(e.ToVersion)
		verPad := strings.Repeat(" ", max(0, verWidth-verRuneLen))
		styledVer := ui.StyleSubtle.Render(e.FromVersion+" →") + " " + ui.StyleWhiteBold.Render(e.ToVersion) + verPad

		fmt.Printf("  %s  %s  %s%s  %s  %s\n",
			ui.StyleSubtle.Render(e.Timestamp.Format("2006-01-02 15:04:05")),
			ui.BadgeManagerPadded(e.Manager, mgrWidth),
			ui.StyleWhiteBold.Render(cleanPkg),
			namePad,
			styledVer,
			statusBadge,
		)
	}

	fmt.Printf("\n  %s\n\n", ui.StyleSubtle.Render(fmt.Sprintf("Showing %d most recent records from %s", len(entries), history.LogPath())))
	return 0
}

func runMonitor(ctx context.Context, args []string) int {
	fs := flag.NewFlagSet("monitor", flag.ExitOnError)
	interval := fs.Duration("interval", 1*time.Hour, "Check frequency interval (e.g. 30m, 1h, 4h)")
	_ = fs.Parse(args)

	fmt.Print(ui.Banner(Version, runtime.GOOS+"/"+runtime.GOARCH))
	fmt.Printf("  %s %v\n", ui.StyleZinc.Render("Active polling interval:"), *interval)
	fmt.Printf("  %s\n\n", ui.StyleSubtle.Render("Press Ctrl+C to terminate."))

	s := scanner.New()

	runCheckOnce := func() {
		fmt.Printf("  [%s] Scanning for vulnerabilities...\n", time.Now().Format("15:04:05"))
		res, err := s.Scan(ctx)
		if err != nil {
			fmt.Printf("  %s\n", ui.StyleAmberBold.Render(fmt.Sprintf("⚠ Scan failed: %v", err)))
			return
		}

		if res.Summary.SecurityFixCount > 0 {
			msg := fmt.Sprintf("%d urgent security CVE fixes available across your packages.", res.Summary.SecurityFixCount)
			fmt.Printf("  %s\n", ui.StyleRoseBold.Render("🚨 ALERT: "+msg))

			if runtime.GOOS == "darwin" {
				script := fmt.Sprintf(`display notification %q with title "Vanguard Security Shield" sound name "Ping"`, msg)
				_ = exec.Command("osascript", "-e", script).Run()
			}
		} else {
			fmt.Printf("  %s\n", ui.StyleEmeraldBold.Render(fmt.Sprintf("✓ System clean (%d outdated packages, 0 CVEs)", res.Summary.TotalPackages)))
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
	fmt.Printf("  %s\n", ui.StyleWhiteBold.Render("Deep System Maintenance Sweep (Topgrade)"))
	fmt.Printf("  %s\n", ui.StyleSubtle.Render(strings.Repeat("─", 76)))
	fmt.Printf("  %s\n", ui.StyleZinc.Render("• Git repositories, submodules, and clean worktrees"))
	fmt.Printf("  %s\n", ui.StyleZinc.Render("• Toolchains & environments (Rust, Go, Python, Node, Ruby)"))
	fmt.Printf("  %s\n", ui.StyleZinc.Render("• Shell frameworks, editor extensions, containers"))
	fmt.Printf("  %s\n\n", ui.StyleZinc.Render("• macOS App Store & system package caches"))

	_ = auth.EnsureSudo()
	_ = config.EnsureMPMConfig()

	execEngine := executor.New()
	err := execEngine.ExecuteTopgrade(ctx, func(ev domain.UpgradeEvent) {
		switch ev.Type {
		case "start":
			fmt.Printf("  %s %s\n", ui.StyleCyanBold.Render("▶"), ev.Message)
		case "log":
			fmt.Printf("    %s\n", ui.StyleZinc.Render(ev.Message))
		case "package_done":
			if ev.Status == "ok" {
				fmt.Printf("  %s\n", ui.StyleEmeraldBold.Render("✓ "+ev.Message))
			} else {
				fmt.Printf("  %s\n", ui.StyleRoseBold.Render("✗ "+ev.Message))
			}
		case "complete":
			fmt.Printf("\n  %s\n\n", ui.StyleEmeraldBold.Render("✓ "+ev.Message))
		case "error":
			fmt.Printf("\n  %s\n\n", ui.StyleRoseBold.Render("✗ "+ev.Message))
		}
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "  %s\n\n", ui.StyleRoseBold.Render(fmt.Sprintf("✗ Sweep failed: %v", err)))
		return 1
	}
	return 0
}
