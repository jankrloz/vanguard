package scanner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/jankrloz/vanguard/internal/config"
	"github.com/jankrloz/vanguard/internal/domain"
	"github.com/jankrloz/vanguard/internal/security"
)

func packageRank(p domain.Package) int {
	if p.HasVulnerability {
		switch strings.ToUpper(p.MaxSeverity) {
		case "CRITICAL":
			return 0
		case "HIGH":
			return 1
		case "MODERATE", "MEDIUM":
			return 2
		default:
			return 3
		}
	}
	if p.Safe {
		return 10
	}
	if p.ChangeType == domain.ChangeMajor {
		return 20
	}
	return 30
}

// DeriveSourceURL maps package manager and package identifier to authoritative registry or repo URL.
func DeriveSourceURL(manager, name string) string {
	m := strings.ToLower(manager)
	switch m {
	case "brew", "homebrew":
		return "https://formulae.brew.sh/formula/" + name
	case "cask":
		return "https://formulae.brew.sh/cask/" + name
	case "npm", "pnpm", "yarn", "bun":
		return "https://www.npmjs.com/package/" + name
	case "pip", "pipx", "pypi", "uv", "poetry", "pdm":
		return "https://pypi.org/project/" + name
	case "cargo", "rust":
		return "https://crates.io/crates/" + name
	case "gem", "bundler", "ruby":
		return "https://rubygems.org/gems/" + name
	case "go":
		return "https://pkg.go.dev/" + name
	case "cpan":
		return "https://metacpan.org/pod/" + name
	case "composer", "php":
		return "https://packagist.org/packages/" + name
	case "nuget", "dotnet":
		return "https://www.nuget.org/packages/" + name
	case "pub", "flutter", "dart":
		return "https://pub.dev/packages/" + name
	case "hex", "mix":
		return "https://hex.pm/packages/" + name
	default:
		return ""
	}
}

type Scanner struct{}

func New() *Scanner {
	return &Scanner{}
}

// Scan queries mpm to find all outdated packages across all package managers,
// enforces configuration pins/ignores, and enriches results with OSV.dev CVE intelligence.
func (s *Scanner) Scan(ctx context.Context) (*domain.ScanResult, error) {
	cfg, _ := config.Load()

	// 1. Discover outdated packages purely via mpm
	packages, err := s.scanMPM(ctx, cfg.Packages.IgnoredManagers)
	if err != nil {
		return nil, fmt.Errorf("mpm scan failed: %w", err)
	}

	// 2. Apply persistent config pins
	for i := range packages {
		if cfg.IsPinned(packages[i].ID) || cfg.IsPinned(packages[i].Name) {
			packages[i].Pinned = true
			packages[i].Selected = false
		}
	}

	// 3. Enrich with OSV.dev vulnerability intelligence
	advisor := security.NewAdvisor()
	packages = advisor.EnrichPackages(ctx, packages)

	// 4. Enrich package descriptions
	enrichDescriptions(ctx, packages)

	safeCount := 0
	majorCount := 0
	pinnedCount := 0
	securityFixCount := 0
	criticalCount := 0
	highCount := 0
	mediumCount := 0
	lowCount := 0

	// 4. Calculate summaries
	for _, p := range packages {
		if p.Pinned {
			pinnedCount++
		}
		if p.Safe {
			safeCount++
		}
		if p.ChangeType == domain.ChangeMajor {
			majorCount++
		}
		if p.HasVulnerability {
			securityFixCount++
			switch strings.ToUpper(p.MaxSeverity) {
			case "CRITICAL":
				criticalCount++
			case "HIGH":
				highCount++
			case "MEDIUM", "MODERATE":
				mediumCount++
			default:
				lowCount++
			}
		}
	}

	// 5. Sort: Critical/CVEs first, then Safe, then Major
	sort.SliceStable(packages, func(i, j int) bool {
		rI, rJ := packageRank(packages[i]), packageRank(packages[j])
		if rI != rJ {
			return rI < rJ
		}
		return packages[i].Name < packages[j].Name
	})

	return &domain.ScanResult{
		Packages: packages,
		Summary: domain.ScanSummary{
			TotalPackages:    len(packages),
			SafeCount:        safeCount,
			MajorCount:       majorCount,
			PinnedCount:      pinnedCount,
			SecurityFixCount: securityFixCount,
			CriticalCount:    criticalCount,
			HighCount:        highCount,
			MediumCount:      mediumCount,
			LowCount:         lowCount,
		},
	}, nil
}

func (s *Scanner) scanMPM(ctx context.Context, ignoredManagers []string) ([]domain.Package, error) {
	scanCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	args := []string{
		"--accessible", "--no-color",
	}

	// Add ignore flags
	hasCPAN := false
	for _, ign := range ignoredManagers {
		args = append(args, "--no-"+ign)
		if strings.EqualFold(ign, "cpan") {
			hasCPAN = true
		}
	}

	// Exclude macOS system Perl CPAN dual-lived core packages (2,140 modules) by default
	if !hasCPAN {
		args = append(args, "--no-cpan")
	}

	args = append(args, "outdated", "--columns", "manager_id,package_id,installed_version,latest_version")

	cmd := exec.CommandContext(scanCtx, "mpm", args...)

	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("mpm outdated failed: %w", err)
	}

	return ParseMPMOutput(bytes.NewReader(out)), nil
}

// ParseMPMOutput parses tabular output from `mpm --accessible --no-color outdated`.
func ParseMPMOutput(r io.Reader) []domain.Package {
	var packages []domain.Package
	scanner := bufio.NewScanner(r)

	inTable := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "warning:") || strings.HasPrefix(line, "error:") {
			continue
		}

		if strings.HasPrefix(line, "Manager") && strings.Contains(line, "Package ID") {
			inTable = true
			continue
		}

		if strings.Contains(line, "packages total") {
			break
		}

		if !inTable {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) >= 4 {
			mgr := fields[0]
			pkgID := fields[1]
			installed := fields[2]
			latest := fields[3]

			changeType, safe := domain.ClassifyChange(installed, latest)
			pkg := domain.Package{
				ID:             fmt.Sprintf("%s:%s", mgr, pkgID),
				Name:           pkgID,
				Manager:        mgr,
				CurrentVersion: installed,
				LatestVersion:  latest,
				ChangeType:     changeType,
				Safe:           safe,
				Pinned:         false,
				Selected:       safe,
				SourceURL:      DeriveSourceURL(mgr, pkgID),
			}
			packages = append(packages, pkg)
		}
	}

	return packages
}

func enrichDescriptions(ctx context.Context, packages []domain.Package) {
	var brewNames []string
	for _, p := range packages {
		if (p.Manager == "brew" || p.Manager == "cask") && p.Description == "" {
			brewNames = append(brewNames, p.Name)
		}
	}

	if len(brewNames) > 0 {
		descMap := fetchBrewDescriptions(ctx, brewNames)
		for i := range packages {
			if desc, ok := descMap[packages[i].Name]; ok && desc != "" {
				packages[i].Description = desc
			}
		}
	}

	// For gems: if description is empty, try quick gem specification
	for i := range packages {
		if packages[i].Manager == "gem" && packages[i].Description == "" {
			subCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
			out, err := exec.CommandContext(subCtx, "gem", "specification", packages[i].Name, "description").Output()
			cancel()
			if err == nil {
				d := strings.TrimSpace(string(out))
				d = strings.TrimPrefix(d, "--- ")
				d = strings.TrimSpace(d)
				if d != "" && !strings.Contains(d, "ERROR") {
					packages[i].Description = d
				}
			}
		}
	}

	// Fallback to vulnerability summary if description is still empty
	for i := range packages {
		if packages[i].Description == "" && len(packages[i].Vulnerabilities) > 0 {
			packages[i].Description = packages[i].Vulnerabilities[0].Summary
		}
	}
}

func fetchBrewDescriptions(ctx context.Context, names []string) map[string]string {
	descMap := make(map[string]string)
	if len(names) == 0 {
		return descMap
	}

	for i := 0; i < len(names); i += 50 {
		end := i + 50
		if end > len(names) {
			end = len(names)
		}
		batch := names[i:end]
		args := append([]string{"desc"}, batch...)

		subCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
		cmd := exec.CommandContext(subCtx, "brew", args...)
		out, err := cmd.Output()
		cancel()

		if err != nil && len(out) == 0 {
			continue
		}

		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if idx := strings.Index(line, ":"); idx != -1 {
				pkgPart := strings.TrimSpace(line[:idx])
				pkgPart = strings.TrimSuffix(pkgPart, " ✔")
				pkgPart = strings.TrimSpace(pkgPart)
				descPart := strings.TrimSpace(line[idx+1:])
				if pkgPart != "" && descPart != "" {
					descMap[pkgPart] = descPart
				}
			}
		}
	}
	return descMap
}
