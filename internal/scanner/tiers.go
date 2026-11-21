package scanner

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/jankrloz/vanguard/internal/domain"
)

// topgradeExclusiveSteps maps Topgrade dry-run section titles to Topgrade --only step names
// for ecosystems MPM either declines (❌ 🛟 in unsupported.md) or lacks outdated detection for.
var topgradeExclusiveSteps = map[string]struct {
	step string
	desc string
}{
	"macOS system update":           {step: "system", desc: "macOS operating system and security updates (softwareupdate)"},
	"Sparkle":                       {step: "sparkle", desc: "macOS application Sparkle appcast updates"},
	"Tailscale":                     {step: "tailscale", desc: "Tailscale mesh VPN client update"},
	"Containers":                    {step: "containers", desc: "Container runtime images and volumes"},
	"Claude Code":                   {step: "claude_code", desc: "Claude Code CLI self-update"},
	"Claude Code Plugins":           {step: "claude_code_plugins", desc: "Claude Code marketplace plugins"},
	"Antigravity CLI":               {step: "antigravity_cli", desc: "Antigravity CLI (agy) self-update"},
	"Antigravity":                   {step: "antigravity", desc: "Antigravity IDE extensions"},
	"Antigravity IDE":               {step: "antigravity_ide", desc: "Antigravity IDE application"},
	"Cursor extensions":             {step: "cursor", desc: "Cursor editor extensions"},
	"Cursor Agent":                  {step: "cursor_agent", desc: "Cursor Agent CLI self-update"},
	"Codex":                         {step: "codex", desc: "OpenAI Codex CLI self-update"},
	"Hermes Agent":                  {step: "hermes_agent", desc: "Hermes Agent CLI self-update"},
	"OpenCode":                      {step: "opencode", desc: "OpenCode plugins and extensions"},
	"pi":                            {step: "pi", desc: "Pi coding agent CLI and extensions"},
	"Vite+":                         {step: "vite_plus", desc: "Vite+ unified web toolchain"},
	"GitHub CLI Extensions":         {step: "github_cli_extensions", desc: "GitHub CLI (gh) extensions"},
	"GitHub CLI Skills":             {step: "github_cli_skills", desc: "GitHub CLI (gh) agent skills"},
	"Visual Studio Code extensions": {step: "vscode", desc: "Visual Studio Code extensions"},
	"Windsurf extensions":           {step: "windsurf", desc: "Windsurf editor extensions"},
	"Zed":                           {step: "zed", desc: "Zed editor update"},
	"Cargo":                         {step: "cargo", desc: "Rust cargo-update binaries"},
	"Fisher":                        {step: "fisher", desc: "Fish shell Fisher plugins"},
	"Ollama":                        {step: "ollama", desc: "Ollama local LLM models"},
	"tldr":                          {step: "tldr", desc: "tldr man-page cache"},
}

func isManagerIgnored(mgr string, ignored []string) bool {
	for _, ign := range ignored {
		if strings.EqualFold(ign, mgr) {
			return true
		}
	}
	return false
}

// syncPackageManagers refreshes upstream package indexes (Tier 1 MPM sync) prior to scanning.
func syncPackageManagers(ctx context.Context, ignoredManagers []string) {
	if _, err := exec.LookPath("mpm"); err != nil {
		return
	}

	syncCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	args := []string{"--accessible", "--no-color"}
	hasCPAN := false
	hasGem := false
	hasUV := false
	for _, ign := range ignoredManagers {
		if !isSafeArg(ign) {
			continue
		}
		args = append(args, "--no-"+ign)
		if strings.EqualFold(ign, "cpan") {
			hasCPAN = true
		}
		if strings.EqualFold(ign, "gem") {
			hasGem = true
		}
		if strings.EqualFold(ign, "uv") {
			hasUV = true
		}
	}
	if !hasCPAN {
		args = append(args, "--no-cpan")
	}
	if !hasGem && isMacOSSystemGem() {
		args = append(args, "--no-gem")
	}
	if !hasUV && os.Getenv("VIRTUAL_ENV") == "" {
		args = append(args, "--no-uv")
	}
	args = append(args, "sync")

	_ = exec.CommandContext(syncCtx, "mpm", args...).Run()
}

// hasPendingMacOSUpdates checks if Apple softwareupdate reports any pending OS updates.
func hasPendingMacOSUpdates(ctx context.Context) bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	if _, err := exec.LookPath("softwareupdate"); err != nil {
		return false
	}
	subCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(subCtx, "softwareupdate", "--list").CombinedOutput()
	if err != nil && len(out) == 0 {
		return false
	}
	text := string(out)
	return !strings.Contains(text, "No new software available") && strings.Contains(text, "* Label:")
}

// scanTopgrade probes Topgrade (Tier 2) for ecosystems MPM declines or lacks outdated detection for.
func scanTopgrade(ctx context.Context, ignoredManagers []string) []domain.Package {
	if isManagerIgnored("topgrade", ignoredManagers) {
		return nil
	}
	if _, err := exec.LookPath("topgrade"); err != nil {
		return nil
	}

	subCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	out, err := exec.CommandContext(subCtx, "topgrade", "--dry-run", "--no-retry").CombinedOutput()
	if err != nil && len(out) == 0 {
		return nil
	}

	pendingMacOS := false
	if !isManagerIgnored("macos", ignoredManagers) && !isManagerIgnored("system", ignoredManagers) {
		pendingMacOS = hasPendingMacOSUpdates(ctx)
	}

	return ParseTopgradeDryRun(bytes.NewReader(out), ignoredManagers, pendingMacOS)
}

var topgradeHeaderRe = regexp.MustCompile(`^――\s+\d{2}:\d{2}:\d{2}\s+-\s+(.+?)\s+――$`)

// ParseTopgradeDryRun parses `topgrade --dry-run` output into actionable Tier 2 packages,
// filtering out steps already covered by MPM or lacking actionable dry-run commands.
func ParseTopgradeDryRun(r io.Reader, ignoredManagers []string, hasMacOSUpdates bool) []domain.Package {
	type section struct {
		title       string
		hasDryRun   bool
		dryRunLines []string
	}

	var sections []section
	var current *section
	failedSteps := make(map[string]bool)
	inSummary := false

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if m := topgradeHeaderRe.FindStringSubmatch(line); len(m) == 2 {
			title := strings.TrimSpace(m[1])
			if title == "Summary" {
				inSummary = true
				current = nil
				continue
			}
			inSummary = false
			sections = append(sections, section{title: title})
			current = &sections[len(sections)-1]
			continue
		}
		if inSummary {
			if idx := strings.Index(line, ":"); idx != -1 {
				stepLabel := strings.TrimSpace(line[:idx])
				status := strings.TrimSpace(line[idx+1:])
				if strings.EqualFold(status, "FAILED") {
					failedSteps[stepLabel] = true
				}
			}
			continue
		}
		if current != nil && strings.HasPrefix(line, "Dry running:") {
			current.hasDryRun = true
			current.dryRunLines = append(current.dryRunLines, strings.TrimSpace(strings.TrimPrefix(line, "Dry running:")))
		}
	}

	var pkgs []domain.Package
	seen := make(map[string]bool)

	for _, sec := range sections {
		if !sec.hasDryRun || failedSteps[sec.title] {
			continue
		}

		// Special case: Topgrade's `uv` step performs `uv self update` (MPM only handles venv/uvx)
		if sec.title == "uv" {
			hasSelf := false
			for _, cmd := range sec.dryRunLines {
				if strings.Contains(cmd, "self update") {
					hasSelf = true
					break
				}
			}
			if !hasSelf || isManagerIgnored("uv", ignoredManagers) {
				continue
			}
			if !seen["uv"] {
				seen["uv"] = true
				pkgs = append(pkgs, domain.Package{
					ID:             "topgrade:uv",
					Name:           "uv",
					Manager:        "topgrade",
					CurrentVersion: probeToolVersion("uv", "--version"),
					LatestVersion:  "latest",
					ChangeType:     domain.ChangeMinor,
					Safe:           true,
					Selected:       true,
					Description:    "uv Python package manager self-update",
					SourceURL:      DeriveSourceURL("uv", "uv"),
				})
			}
			continue
		}

		meta, ok := topgradeExclusiveSteps[sec.title]
		if !ok {
			continue
		}
		if meta.step == "system" && !hasMacOSUpdates {
			continue
		}
		if isManagerIgnored(meta.step, ignoredManagers) || isManagerIgnored(sec.title, ignoredManagers) {
			continue
		}
		if seen[meta.step] {
			continue
		}
		seen[meta.step] = true

		curVer := detectLocalStepVersion(meta.step)
		pkgs = append(pkgs, domain.Package{
			ID:             "topgrade:" + meta.step,
			Name:           meta.step,
			Manager:        "topgrade",
			CurrentVersion: curVer,
			LatestVersion:  "latest",
			ChangeType:     domain.ChangeMinor,
			Safe:           true,
			Selected:       true,
			Description:    meta.desc,
			SourceURL:      DeriveSourceURL(meta.step, meta.step),
		})
	}

	return pkgs
}

func detectLocalStepVersion(step string) string {
	switch step {
	case "claude_code":
		return probeToolVersion("claude", "--version")
	case "antigravity_cli":
		return probeToolVersion("agy", "--version")
	case "hermes_agent":
		return probeToolVersion("hermes", "--version")
	case "cursor_agent":
		return probeToolVersion("cursor-agent", "--version")
	case "codex":
		return probeToolVersion("codex", "--version")
	case "opencode":
		return probeToolVersion("opencode", "--version")
	case "pi":
		return probeToolVersion("pi", "--version")
	case "vite_plus":
		return probeToolVersion("vp", "--version")
	default:
		return "installed"
	}
}

var semverExtractRe = regexp.MustCompile(`v?(\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?)`)

func probeToolVersion(bin string, args ...string) string {
	if _, err := exec.LookPath(bin); err != nil {
		return "installed"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	if err != nil && len(out) == 0 {
		return "installed"
	}
	if m := semverExtractRe.FindStringSubmatch(string(out)); len(m) == 2 {
		return m[1]
	}
	return "installed"
}

// scanMole checks Mole (Tier 3) when installed standalone (not already reported by Homebrew).
func scanMole(ctx context.Context, existing []domain.Package, ignoredManagers []string) []domain.Package {
	if isManagerIgnored("mole", ignoredManagers) {
		return nil
	}
	if _, err := exec.LookPath("mo"); err != nil {
		return nil
	}
	for _, p := range existing {
		if strings.EqualFold(p.Name, "mole") && (p.Manager == "brew" || p.Manager == "cask") {
			return nil
		}
	}
	// If installed via Homebrew and brew didn't report it outdated, Mole is already up to date.
	if _, err := exec.LookPath("brew"); err == nil {
		subCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		err := exec.CommandContext(subCtx, "brew", "list", "--formula", "mole").Run()
		cancel()
		if err == nil {
			return nil
		}
	}

	ver := probeToolVersion("mo", "--version")
	return []domain.Package{
		{
			ID:             "mole:mole",
			Name:           "mole",
			Manager:        "mole",
			CurrentVersion: ver,
			LatestVersion:  "latest",
			ChangeType:     domain.ChangeMinor,
			Safe:           true,
			Selected:       true,
			Description:    "Mole macOS cleaner and optimizer self-update",
			SourceURL:      "https://mole.fit",
		},
	}
}

// scanCustom probes Tier 4 custom ecosystems not covered by MPM, Topgrade, or Mole:
//   - `skills check` (npx skills global skill updates)
//   - `apm outdated` (Agent Package Manager, distinct from Atom apm)
func scanCustom(ctx context.Context, ignoredManagers []string) []domain.Package {
	var pkgs []domain.Package

	if !isManagerIgnored("skills", ignoredManagers) {
		if _, err := exec.LookPath("skills"); err == nil {
			subCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
			out, _ := exec.CommandContext(subCtx, "skills", "check").CombinedOutput()
			cancel()
			pkgs = append(pkgs, ParseSkillsCheckOutput(bytes.NewReader(out))...)
		}
	}

	if !isManagerIgnored("apm", ignoredManagers) {
		if _, err := exec.LookPath("apm"); err == nil {
			verCtx, verCancel := context.WithTimeout(ctx, 2*time.Second)
			verOut, verErr := exec.CommandContext(verCtx, "apm", "--version").CombinedOutput()
			verCancel()
			if verErr == nil && strings.Contains(string(verOut), "Agent Package Manager") {
				subCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				out, _ := exec.CommandContext(subCtx, "apm", "outdated").CombinedOutput()
				cancel()
				pkgs = append(pkgs, ParseAPMOutdatedOutput(bytes.NewReader(out))...)
			}
		}
	}

	return pkgs
}

var skillsUpdateLineRe = regexp.MustCompile(`^[•\-\*]\s+([A-Za-z0-9_.-]+(?:/[A-Za-z0-9_.-]+)*)`)

// ParseSkillsCheckOutput parses `skills check` output for skills with available updates.
func ParseSkillsCheckOutput(r io.Reader) []domain.Package {
	var pkgs []domain.Package
	seen := make(map[string]bool)
	inUpdates := false

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "available update") || strings.Contains(lower, "updates available") {
			inUpdates = true
			continue
		}
		if strings.HasPrefix(lower, "warning:") || strings.Contains(lower, "deleted upstream") {
			inUpdates = false
			continue
		}
		if !inUpdates {
			continue
		}
		if m := skillsUpdateLineRe.FindStringSubmatch(line); len(m) == 2 {
			name := m[1]
			if seen[name] {
				continue
			}
			seen[name] = true
			pkgs = append(pkgs, domain.Package{
				ID:             "skills:" + name,
				Name:           name,
				Manager:        "skills",
				CurrentVersion: "installed",
				LatestVersion:  "latest",
				ChangeType:     domain.ChangeMinor,
				Safe:           true,
				Selected:       true,
				Description:    fetchSkillDescription(name),
				SourceURL:      DeriveSourceURL("skills", name),
			})
		}
	}
	return pkgs
}

// ParseAPMOutdatedOutput parses `apm outdated` tabular/bullet output for Agent Package Manager.
func ParseAPMOutdatedOutput(r io.Reader) []domain.Package {
	var pkgs []domain.Package
	seen := make(map[string]bool)

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "[") || strings.HasPrefix(line, "─") || strings.HasPrefix(line, "Package") {
			continue
		}
		// Match lines like: "pkg-name  1.0.0 -> 1.1.0" or "pkg-name  1.0.0  1.1.0"
		cleaned := strings.ReplaceAll(line, "->", " ")
		cleaned = strings.ReplaceAll(cleaned, "→", " ")
		fields := strings.Fields(cleaned)
		if len(fields) >= 3 && isSafeArg(fields[0]) {
			name, cur, lat := fields[0], fields[1], fields[2]
			if cur == lat || seen[name] {
				continue
			}
			seen[name] = true
			changeType, safe := domain.ClassifyChange(cur, lat)
			pkgs = append(pkgs, domain.Package{
				ID:             "apm:" + name,
				Name:           name,
				Manager:        "apm",
				CurrentVersion: cur,
				LatestVersion:  lat,
				ChangeType:     changeType,
				Safe:           safe,
				Selected:       safe,
				Description:    "Agent Package Manager (APM) dependency",
				SourceURL:      DeriveSourceURL("apm", name),
			})
		}
	}
	return pkgs
}
