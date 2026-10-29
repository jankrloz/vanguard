package ui

import (
	"fmt"
	"image/color"
	"os"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/lipgloss/v2"
	"golang.org/x/term"
)

// Brand Palette Tokens (aligned 1:1 with Web & Static Site)
var (
	ColorCyan    = lipgloss.Color("#00F0FF") // Vanguard Electric Cyan (Core Identity)
	ColorCyanDim = lipgloss.Color("#0090A0")
	ColorIndigo  = lipgloss.Color("#6366F1") // Accent Indigo
	ColorRose    = lipgloss.Color("#FF2E93") // Threat Rose (Critical Zero-Days & Advisories)
	ColorOrange  = lipgloss.Color("#F97316") // High Severity
	ColorAmber   = lipgloss.Color("#FFB800") // Warning / Major Breaking Changes
	ColorEmerald = lipgloss.Color("#00FF9D") // Safe Updates (Verified SemVer)
	ColorZinc    = lipgloss.Color("#A1A1AA") // Subtitles / Descriptions
	ColorSubtle  = lipgloss.Color("#8E8E99") // Metadata / Dividers (WCAG AA compliant)
	ColorDark    = lipgloss.Color("#27272A") // Subtle Borders / Badge Chips
	ColorDeep    = lipgloss.Color("#09090B") // Canvas background
	ColorWhite   = lipgloss.Color("#FAFAFA")
)

// Lip Gloss v2 Base Styles
var (
	StyleBrandName = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite)

	StyleTagline = lipgloss.NewStyle().
			Foreground(ColorZinc)

	StyleMeta = lipgloss.NewStyle().
			Foreground(ColorSubtle)

	StyleSectionTitle = lipgloss.NewStyle().
				Bold(true).
				MarginTop(1)

	StyleManagerBadge = lipgloss.NewStyle().
				Foreground(ColorCyan).
				Background(ColorDark).
				Padding(0, 1).
				Bold(true)

	StyleSafeBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#09090B")).
			Background(ColorEmerald).
			Padding(0, 1).
			Bold(true)

	StyleMajorBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#09090B")).
			Background(ColorAmber).
			Padding(0, 1).
			Bold(true)

	StyleCriticalBadge = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#09090B")).
				Background(ColorRose).
				Padding(0, 1).
				Bold(true)

	StyleHighBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#09090B")).
			Background(ColorOrange).
			Padding(0, 1).
			Bold(true)

	StyleMediumBadge = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#09090B")).
				Background(ColorAmber).
				Padding(0, 1).
				Bold(true)

	StyleLowBadge = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Background(ColorDark).
			Padding(0, 1).
			Bold(true)

	StylePinnedBadge = lipgloss.NewStyle().
				Foreground(ColorAmber).
				Background(ColorDark).
				Padding(0, 1).
				Bold(true)

	StyleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorDark).
			Padding(0, 1).
			MarginTop(1)

	// Inline text styles (TrueColor via Lip Gloss v2)
	StyleWhiteBold   = lipgloss.NewStyle().Bold(true).Foreground(ColorWhite)
	StyleWhite       = lipgloss.NewStyle().Foreground(ColorWhite)
	StyleZinc        = lipgloss.NewStyle().Foreground(ColorZinc)
	StyleSubtle      = lipgloss.NewStyle().Foreground(ColorSubtle)
	StyleDarkDivider = lipgloss.NewStyle().Foreground(ColorDark)
	StyleCyanBold    = lipgloss.NewStyle().Bold(true).Foreground(ColorCyan)
	StyleEmeraldBold = lipgloss.NewStyle().Bold(true).Foreground(ColorEmerald)
	StyleRoseBold    = lipgloss.NewStyle().Bold(true).Foreground(ColorRose)
	StyleRoseText    = lipgloss.NewStyle().Foreground(ColorRose)
	StyleOrangeBold  = lipgloss.NewStyle().Bold(true).Foreground(ColorOrange)
	StyleAmberBold   = lipgloss.NewStyle().Bold(true).Foreground(ColorAmber)
)

// Legacy ANSI constants mapped to TrueColor RGB sequences so any direct string interpolation
// matches the Static Site & Web UI palette identically.
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Underline = "\033[4m"
	Cyan      = "\033[38;2;0;240;255m"
	CyanDim   = "\033[38;2;0;144;160m"
	Rose      = "\033[38;2;255;46;147m"
	RoseDim   = "\033[38;2;159;18;57m"
	Orange    = "\033[38;2;249;115;22m"
	Emerald   = "\033[38;2;0;255;157m"
	Amber     = "\033[38;2;255;184;0m"
	Zinc      = "\033[38;2;161;161;170m"
	DarkZinc  = "\033[38;2;142;142;153m"
	White     = "\033[1;38;2;250;250;250m"
)

// Banner renders the geometric stealth brand identity for Vanguard using Lip Gloss v2.
func Banner(version, platform string) string {
	cyan := lipgloss.NewStyle().Bold(true).Foreground(ColorCyan).Render
	indigo := lipgloss.NewStyle().Bold(true).Foreground(ColorIndigo).Render
	verBadge := lipgloss.NewStyle().
		Foreground(ColorCyan).
		Background(ColorDark).
		Padding(0, 1).
		Bold(true).
		Render("v" + version)

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString("  " + cyan("    ▲    ") + "\n")
	sb.WriteString("  " + cyan("   /█\\   ") + "  " + StyleBrandName.Render("V A N G U A R D") + "  " + verBadge + "\n")
	sb.WriteString("  " + cyan("  /███\\  ") + "  " + StyleTagline.Render("Vulnerability Shield & Package Orchestrator") + "\n")
	sb.WriteString("  " + indigo("  \\███/  ") + "  " + StyleMeta.Render(platform+" · OSV.dev · Trivy · Native Audits") + "\n")
	sb.WriteString("  " + indigo("   \\▼/   ") + "\n\n")
	return sb.String()
}

// SectionHeader prints a styled CLI section title.
func SectionHeader(title string, count int, headerColor string) string {
	var c color.Color
	switch headerColor {
	case Rose, "#FF2E93":
		c = ColorRose
	case Emerald, "#00FF9D":
		c = ColorEmerald
	case Amber, "#FFB800":
		c = ColorAmber
	case Cyan, "#00F0FF":
		c = ColorCyan
	default:
		c = ColorZinc
	}

	titleText := lipgloss.NewStyle().Bold(true).Foreground(c).Render(fmt.Sprintf("%s (%d)", title, count))
	divider := lipgloss.NewStyle().Foreground(ColorSubtle).Render(strings.Repeat("─", 76))
	return fmt.Sprintf("\n  %s\n  %s", titleText, divider)
}

// BadgeManager formats a manager badge, e.g. [brew]
func BadgeManager(mgr string) string {
	return StyleManagerBadge.Render(mgr)
}

// BadgeManagerPadded formats a manager badge padded to a uniform column width
// so package names align vertically across rows.
func BadgeManagerPadded(mgr string, width int) string {
	if width < len(mgr) {
		width = len(mgr)
	}
	padded := fmt.Sprintf("%-*s", width, mgr)
	return StyleManagerBadge.Render(padded)
}

// BadgeSeverity renders an authoritative severity pill badge according to risk level.
func BadgeSeverity(severity string, score float64) string {
	sev := strings.ToUpper(severity)
	label := sev
	if score > 0 {
		label = fmt.Sprintf("%s %.1f", sev, score)
	}

	switch sev {
	case "CRITICAL":
		return StyleCriticalBadge.Render("🔥 " + label)
	case "HIGH":
		return StyleHighBadge.Render("⚠ " + label)
	case "MEDIUM", "MODERATE":
		return StyleMediumBadge.Render("● " + label)
	default:
		return StyleLowBadge.Render("○ " + label)
	}
}

// BadgeCVE formats a security CVE tag with severity rating.
func BadgeCVE(cve string, severity string) string {
	cvePill := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorRose).
		Render("[" + cve + "]")

	return fmt.Sprintf("%s %s", cvePill, BadgeSeverity(severity, 0))
}

// BadgeSafe formats a safe update tag.
func BadgeSafe() string {
	return StyleSafeBadge.Render("✓ SAFE")
}

// BadgeMajor formats a major breaking update tag.
func BadgeMajor() string {
	return StyleMajorBadge.Render("▲ MAJOR")
}

// BadgePinned formats a pinned tag.
func BadgePinned() string {
	return StylePinnedBadge.Render("PINNED")
}

// Box formats a multi-line message in a clean Lip Gloss v2 card.
func Box(title, content string, borderColor color.Color) string {
	style := StyleBox.BorderForeground(borderColor)
	titleText := lipgloss.NewStyle().Bold(true).Foreground(borderColor).Render(title)
	body := fmt.Sprintf("%s\n%s", titleText, content)
	return style.Render(body) + "\n"
}

// RenderProgress renders a Charm Bubbles v2 progress bar for upgrade progress reporting.
func RenderProgress(completed, total int) string {
	if total <= 0 {
		return ""
	}
	pct := float64(completed) / float64(total)
	if pct > 1.0 {
		pct = 1.0
	}
	prog := progress.New(
		progress.WithColors(ColorCyan, ColorEmerald),
		progress.WithWidth(28),
	)
	return fmt.Sprintf("%s %s", prog.ViewAs(pct), StyleSubtle.Render(fmt.Sprintf("(%d/%d)", completed, total)))
}

// StartSpinner starts a Charm Bubbles v2 spinner during CLI scans.
// Returns a stop function. If output is not a TTY (headless/piped), prints a single static status.
func StartSpinner(msg string) func(finalMsg string) {
	isTTY := term.IsTerminal(int(os.Stdout.Fd()))
	if !isTTY {
		fmt.Printf("  • %s...\n", msg)
		return func(finalMsg string) {
			if finalMsg != "" {
				fmt.Printf("  ✓ %s\n", finalMsg)
			}
		}
	}

	sp := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(StyleCyanBold),
	)
	fps := sp.Spinner.FPS
	if fps <= 0 {
		fps = 80 * time.Millisecond
	}

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		ticker := time.NewTicker(fps)
		defer ticker.Stop()
		fmt.Printf("\r  %s %s", sp.View(), StyleZinc.Render(msg))
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				sp, _ = sp.Update(sp.Tick())
				fmt.Printf("\r  %s %s", sp.View(), StyleZinc.Render(msg))
			}
		}
	}()

	return func(finalMsg string) {
		close(done)
		wg.Wait()
		fmt.Printf("\r\033[K")
		if finalMsg != "" {
			fmt.Printf("  %s %s\n", StyleEmeraldBold.Render("✓"), StyleWhite.Render(finalMsg))
		}
	}
}
