package scanner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/jankrloz/vanguard/internal/domain"
)

type BrewOutdatedResponse struct {
	Formulae []struct {
		Name              string   `json:"name"`
		InstalledVersions []string `json:"installed_versions"`
		CurrentVersion    string   `json:"current_version"`
		Pinned            bool     `json:"pinned"`
	} `json:"formulae"`
	Casks []struct {
		Name              string   `json:"name"`
		InstalledVersions []string `json:"installed_versions"`
		CurrentVersion    string   `json:"current_version"`
		Pinned            bool     `json:"pinned"`
	} `json:"casks"`
}

type Scanner struct{}

func New() *Scanner {
	return &Scanner{}
}

// Scan queries mpm and brew to find all outdated packages across all package managers.
func (s *Scanner) Scan(ctx context.Context) (*domain.ScanResult, error) {
	pkgMap := make(map[string]domain.Package)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Scan mpm
	wg.Add(1)
	go func() {
		defer wg.Done()
		mpmPkgs, err := s.scanMPM(ctx)
		if err == nil {
			mu.Lock()
			for _, p := range mpmPkgs {
				key := fmt.Sprintf("%s:%s", p.Manager, p.Name)
				pkgMap[key] = p
			}
			mu.Unlock()
		}
	}()

	// Scan brew
	wg.Add(1)
	go func() {
		defer wg.Done()
		brewPkgs, err := s.scanBrew(ctx)
		if err == nil {
			mu.Lock()
			for _, p := range brewPkgs {
				key := fmt.Sprintf("%s:%s", p.Manager, p.Name)
				// Prefer brew's pinned/version info if present
				pkgMap[key] = p
			}
			mu.Unlock()
		}
	}()

	wg.Wait()

	var packages []domain.Package
	safeCount := 0
	majorCount := 0
	pinnedCount := 0

	for _, p := range pkgMap {
		if p.Pinned {
			pinnedCount++
		}
		if p.Safe {
			safeCount++
		}
		if p.ChangeType == domain.ChangeMajor {
			majorCount++
		}
		packages = append(packages, p)
	}

	return &domain.ScanResult{
		Packages: packages,
		Summary: domain.ScanSummary{
			TotalPackages: len(packages),
			SafeCount:     safeCount,
			MajorCount:    majorCount,
			PinnedCount:   pinnedCount,
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

func (s *Scanner) scanBrew(ctx context.Context) ([]domain.Package, error) {
	scanCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(scanCtx, "brew", "outdated", "--json=v2")
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		return nil, fmt.Errorf("brew outdated failed: %w", err)
	}

	// Filter out non-JSON prelude lines (like ==> Downloading Homebrew API data)
	idx := bytes.Index(out, []byte("{"))
	if idx == -1 {
		return nil, fmt.Errorf("no json found in brew outdated output")
	}
	out = out[idx:]

	var resp BrewOutdatedResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal brew outdated json: %w", err)
	}

	var packages []domain.Package

	for _, f := range resp.Formulae {
		current := ""
		if len(f.InstalledVersions) > 0 {
			current = f.InstalledVersions[0]
		}
		changeType, safe := domain.ClassifyChange(current, f.CurrentVersion)
		packages = append(packages, domain.Package{
			ID:             fmt.Sprintf("brew:%s", f.Name),
			Name:           f.Name,
			Manager:        "brew",
			CurrentVersion: current,
			LatestVersion:  f.CurrentVersion,
			ChangeType:     changeType,
			Safe:           safe,
			Pinned:         f.Pinned,
			Selected:       safe && !f.Pinned,
		})
	}

	for _, c := range resp.Casks {
		current := ""
		if len(c.InstalledVersions) > 0 {
			current = c.InstalledVersions[0]
		}
		changeType, safe := domain.ClassifyChange(current, c.CurrentVersion)
		packages = append(packages, domain.Package{
			ID:             fmt.Sprintf("cask:%s", c.Name),
			Name:           c.Name,
			Manager:        "cask",
			CurrentVersion: current,
			LatestVersion:  c.CurrentVersion,
			ChangeType:     changeType,
			Safe:           safe,
			Pinned:         c.Pinned,
			Selected:       safe && !c.Pinned,
		})
	}

	return packages, nil
}
