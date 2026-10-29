package scanner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/jankrloz/vanguard/internal/config"
	"github.com/jankrloz/vanguard/internal/domain"
	"github.com/jankrloz/vanguard/internal/security"
)

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

	safeCount := 0
	majorCount := 0
	pinnedCount := 0
	securityFixCount := 0

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
		}
	}

	// 5. Sort: vulnerable packages float to the top
	for i := 0; i < len(packages); i++ {
		for j := i + 1; j < len(packages); j++ {
			if !packages[i].HasVulnerability && packages[j].HasVulnerability {
				packages[i], packages[j] = packages[j], packages[i]
			}
		}
	}

	return &domain.ScanResult{
		Packages: packages,
		Summary: domain.ScanSummary{
			TotalPackages:    len(packages),
			SafeCount:        safeCount,
			MajorCount:       majorCount,
			PinnedCount:      pinnedCount,
			SecurityFixCount: securityFixCount,
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
	for _, ign := range ignoredManagers {
		args = append(args, "--no-"+ign)
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
			}
			packages = append(packages, pkg)
		}
	}

	return packages
}
