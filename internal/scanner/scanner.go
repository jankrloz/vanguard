package scanner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jankrloz/vanguard/internal/domain"
	"github.com/jankrloz/vanguard/internal/security"
)

type Scanner struct{}

func New() *Scanner {
	return &Scanner{}
}

// Scan queries mpm to find all outdated packages across all package managers,
// checks pinned states, and enriches results with OSV.dev CVE intelligence.
func (s *Scanner) Scan(ctx context.Context) (*domain.ScanResult, error) {
	// 1. Discover outdated packages via mpm
	packages, err := s.scanMPM(ctx)
	if err != nil {
		return nil, fmt.Errorf("mpm scan failed: %w", err)
	}

	// 2. Query pinned packages from brew (fast 10ms lookup)
	pinnedSet := s.getPinnedBrewPackages(ctx)

	safeCount := 0
	majorCount := 0
	pinnedCount := 0
	securityFixCount := 0

	for i := range packages {
		if pinnedSet[packages[i].Name] {
			packages[i].Pinned = true
			packages[i].Selected = false
		}
	}

	// 3. Enrich with OSV.dev vulnerability intelligence
	advisor := security.NewAdvisor()
	packages = advisor.EnrichPackages(ctx, packages)

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

func (s *Scanner) scanMPM(ctx context.Context) ([]domain.Package, error) {
	scanCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(scanCtx, "mpm",
		"--no-cpan", "--no-gem",
		"--accessible", "--no-color",
		"outdated",
		"--columns", "manager_id,package_id,installed_version,latest_version",
	)

	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("mpm outdated failed: %w", err)
	}

	var packages []domain.Package
	scanner := bufio.NewScanner(bytes.NewReader(out))

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

	return packages, nil
}

func (s *Scanner) getPinnedBrewPackages(ctx context.Context) map[string]bool {
	pinned := make(map[string]bool)
	cmd := exec.CommandContext(ctx, "brew", "list", "--pinned")
	out, err := cmd.Output()
	if err != nil {
		return pinned
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		pkg := strings.TrimSpace(scanner.Text())
		if pkg != "" {
			pinned[pkg] = true
		}
	}
	return pinned
}
