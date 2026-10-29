package ui

import (
	"fmt"
	"strings"
)

// ANSI Color & Styling Codes
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Underline = "\033[4m"

	// Palette
	Cyan     = "\033[38;5;39m"  // Brand Primary
	CyanDim  = "\033[38;5;31m"
	Rose     = "\033[38;5;197m" // Security Critical
	RoseDim  = "\033[38;5;88m"
	Emerald  = "\033[38;5;78m"  // Safe Upgrades
	Amber    = "\033[38;5;214m" // Major Breaking
	Zinc     = "\033[38;5;244m" // Metadata / Subtitles
	DarkZinc = "\033[38;5;238m" // Borders / Dividers
	White    = "\033[1;37m"
)

// Banner renders the geometric stealth brand identity for Vanguard.
func Banner(version, platform string) string {
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(Cyan + "      ▲\n" + Reset)
	sb.WriteString(Cyan + "     /█\\     " + Bold + White + "V A N G U A R D" + Reset + "\n")
	sb.WriteString(Cyan + "    /███\\    " + Zinc + "Vulnerability Shield & Package Orchestrator" + Reset + "\n")
	sb.WriteString(Cyan + "    \\███/    " + DarkZinc + "v" + version + " · " + platform + Reset + "\n")
	sb.WriteString(Cyan + "     \\█/\n" + Reset)
	return sb.String()
}

// SectionHeader prints a styled CLI section title.
func SectionHeader(title string, count int, color string) string {
	header := fmt.Sprintf("%s%s %s (%d)%s", Bold, color, title, count, Reset)
	line := DarkZinc + strings.Repeat("─", 68) + Reset
	return fmt.Sprintf("\n  %s\n  %s", header, line)
}

// BadgeManager formats a manager badge, e.g. [brew]
func BadgeManager(mgr string) string {
	return fmt.Sprintf("%s[%s%s%s]%s", DarkZinc, Cyan, mgr, DarkZinc, Reset)
}

// BadgeCVE formats a security CVE tag
func BadgeCVE(cve string, severity string) string {
	return fmt.Sprintf("%s%s[%s: %s]%s", Bold, Rose, cve, severity, Reset)
}

// BadgeSafe formats a safe tag
func BadgeSafe() string {
	return fmt.Sprintf("%s[✓ SAFE]%s", Emerald, Reset)
}

// BadgeMajor formats a major breaking tag
func BadgeMajor() string {
	return fmt.Sprintf("%s%s[▲ MAJOR]%s", Bold, Amber, Reset)
}

// Box formats a multi-line message in a clean terminal card.
func Box(title, content string, borderColor string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  %s┌─ %s%s %s%s\n", borderColor, Bold, title, borderColor, strings.Repeat("─", 50)))
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		sb.WriteString(fmt.Sprintf("  %s│%s  %s\n", borderColor, Reset, l))
	}
	sb.WriteString(fmt.Sprintf("  %s└%s\n", borderColor, strings.Repeat("─", 54)+Reset))
	return sb.String()
}
