package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

// Brand Palette Tokens
var (
	ColorCyan    = lipgloss.Color("#00F0FF") // Vanguard Electric Cyan (Core Identity)
	ColorCyanDim = lipgloss.Color("#0090A0")
	ColorRose    = lipgloss.Color("#FF2E93") // Threat Rose (Critical Zero-Days & Advisories)
	ColorOrange  = lipgloss.Color("#FF8A00") // High Severity
	ColorAmber   = lipgloss.Color("#FFB800") // Warning / Major Breaking Changes
	ColorEmerald = lipgloss.Color("#00FF9D") // Safe Updates (Verified SemVer)
	ColorZinc    = lipgloss.Color("#A1A1AA") // Subtitles / Metadata
	ColorDark    = lipgloss.Color("#27272A") // Subtle Borders / Chips
	ColorDeep    = lipgloss.Color("#09090B") // Canvas background
	ColorWhite   = lipgloss.Color("#F4F4F5")
)

// Lip Gloss Base Styles
var (
	StyleBrandName = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorWhite)

	StyleTagline = lipgloss.NewStyle().
			Foreground(ColorZinc)

	StyleMeta = lipgloss.NewStyle().
			Foreground(ColorDark)

	StyleSectionTitle = lipgloss.NewStyle().
				Bold(true).
				MarginTop(1)

	StyleManagerBadge = lipgloss.NewStyle().
				Foreground(ColorCyan).
				Background(ColorDark).
				Padding(0, 1).
				Bold(true)

	StyleSafeBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(ColorEmerald).
			Padding(0, 1).
			Bold(true)

	StyleMajorBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(ColorAmber).
			Padding(0, 1).
			Bold(true)

	StyleCriticalBadge = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(ColorRose).
				Padding(0, 1).
				Bold(true)

	StyleHighBadge = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(ColorOrange).
			Padding(0, 1).
			Bold(true)

	StyleMediumBadge = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000000")).
				Background(ColorAmber).
				Padding(0, 1).
				Bold(true)

	StyleLowBadge = lipgloss.NewStyle().
			Foreground(ColorWhite).
			Background(ColorDark).
			Padding(0, 1)

	StyleBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorDark).
			Padding(0, 1).
			MarginTop(1)
)

// Legacy ANSI for backward compatibility
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Underline = "\033[4m"
	Cyan      = "\033[38;5;39m"
	CyanDim   = "\033[38;5;31m"
	Rose      = "\033[38;5;197m"
	RoseDim   = "\033[38;5;88m"
	Emerald   = "\033[38;5;78m"
	Amber     = "\033[38;5;214m"
	Zinc      = "\033[38;5;244m"
	DarkZinc  = "\033[38;5;238m"
	White     = "\033[1;37m"
)

// Banner renders the geometric stealth brand identity for Vanguard using Lip Gloss.
func Banner(version, platform string) string {
	delta := lipgloss.NewStyle().Foreground(ColorCyan).Render

	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(delta("      ▲\n"))
	sb.WriteString(delta("     /█\\     ") + StyleBrandName.Render("V A N G U A R D") + "\n")
	sb.WriteString(delta("    /███\\    ") + StyleTagline.Render("Vulnerability Shield & Package Orchestrator") + "\n")
	sb.WriteString(delta("    \\███/    ") + StyleMeta.Render("v"+version+" · "+platform) + "\n")
	sb.WriteString(delta("     \\█/\n"))
	return sb.String()
}

// SectionHeader prints a styled CLI section title.
func SectionHeader(title string, count int, headerColor string) string {
	var c lipgloss.Color
	switch headerColor {
	case Rose, "#FF2E93":
		c = ColorRose
	case Emerald, "#00FF9D":
		c = ColorEmerald
	case Amber, "#FFB800":
		c = ColorAmber
	default:
		c = ColorZinc
	}

	titleText := lipgloss.NewStyle().Bold(true).Foreground(c).Render(fmt.Sprintf("%s (%d)", title, count))
	divider := lipgloss.NewStyle().Foreground(ColorDark).Render(strings.Repeat("─", 68))
	return fmt.Sprintf("\n  %s\n  %s", titleText, divider)
}

// BadgeManager formats a manager badge, e.g. [brew]
func BadgeManager(mgr string) string {
	return StyleManagerBadge.Render(mgr)
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

// Box formats a multi-line message in a clean Lip Gloss card.
func Box(title, content string, borderColor lipgloss.Color) string {
	style := StyleBox.Copy().BorderForeground(borderColor)
	titleText := lipgloss.NewStyle().Bold(true).Foreground(borderColor).Render(title)
	body := fmt.Sprintf("%s\n%s", titleText, content)
	return style.Render(body) + "\n"
}

// StartSpinner starts an animated Electric Cyan terminal spinner during long CLI tasks.
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

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	cyan := lipgloss.NewStyle().Foreground(ColorCyan).Bold(true).Render
	zinc := lipgloss.NewStyle().Foreground(ColorZinc).Render

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-done:
				return
			default:
				frame := frames[i%len(frames)]
				fmt.Printf("\r  %s %s", cyan(frame), zinc(msg))
				time.Sleep(75 * time.Millisecond)
				i++
			}
		}
	}()

	return func(finalMsg string) {
		close(done)
		wg.Wait()
		// Clear line and print final status
		fmt.Printf("\r\033[K")
		if finalMsg != "" {
			emerald := lipgloss.NewStyle().Foreground(ColorEmerald).Bold(true).Render
			fmt.Printf("  %s %s\n", emerald("✓"), finalMsg)
		}
	}
}
