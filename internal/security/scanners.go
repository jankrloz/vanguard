package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jankrloz/vanguard/internal/domain"
)

type cdxBOM struct {
	BOMFormat   string         `json:"bomFormat"`
	SpecVersion string         `json:"specVersion"`
	Version     int            `json:"version"`
	Metadata    cdxMetadata    `json:"metadata"`
	Components  []cdxComponent `json:"components"`
}

type cdxMetadata struct {
	Component cdxComponent `json:"component"`
}

type cdxComponent struct {
	Type    string `json:"type"`
	BOMRef  string `json:"bom-ref,omitempty"`
	Name    string `json:"name"`
	Version string `json:"version"`
	PURL    string `json:"purl,omitempty"`
}

// MapManagerToPURL converts a package manager and package name/version into a Package URL (PURL)
// compatible with OSV-Scanner and Trivy SBOM scanners.
func MapManagerToPURL(mgr, name, version string) string {
	if name == "" || version == "" {
		return ""
	}
	var purlType string
	switch strings.ToLower(mgr) {
	case "npm", "pnpm", "bun", "yarn", "yarn-berry":
		purlType = "npm"
	case "pip", "pipx", "uv", "uvx", "poetry", "pdm", "conda", "mamba", "pixi":
		purlType = "pypi"
	case "cargo", "rust":
		purlType = "cargo"
	case "gem", "bundler", "ruby":
		purlType = "gem"
	case "go":
		purlType = "golang"
	case "composer", "php":
		purlType = "composer"
	case "nuget", "dotnet":
		purlType = "nuget"
	case "pub", "flutter", "dart":
		purlType = "pub"
	case "hex", "mix":
		purlType = "hex"
	case "apk":
		purlType = "alpine"
	case "apt", "nala", "aptitude", "deb-get":
		purlType = "deb"
	default:
		return ""
	}
	// Handle scoped npm packages (@scope/pkg -> %40scope/pkg)
	escapedName := strings.ReplaceAll(name, "@", "%40")
	return fmt.Sprintf("pkg:%s/%s@%s", purlType, escapedName, version)
}

// RunExternalAndNativeScanners runs Tier 1 (osv-scanner, trivy sbom) and Tier 2 (native CLI audits:
// npm audit, pip-audit, cargo-audit) concurrently and merges discovered vulnerabilities into pkgs.
func (a *Advisor) RunExternalAndNativeScanners(ctx context.Context, pkgs []domain.Package) map[int][]domain.Vulnerability {
	results := make(map[int][]domain.Vulnerability)
	if len(pkgs) == 0 {
		return results
	}

	var mu sync.Mutex
	addFinding := func(pkgIdx int, v domain.Vulnerability) {
		mu.Lock()
		defer mu.Unlock()
		results[pkgIdx] = append(results[pkgIdx], v)
	}

	// Build temporary CycloneDX SBOM for Tier 1 scanners (osv-scanner & trivy)
	var components []cdxComponent
	purlToIdx := make(map[string]int)
	nameMgrToIdx := make(map[string]int)

	for i, p := range pkgs {
		nameMgrToIdx[strings.ToLower(p.Manager)+":"+strings.ToLower(p.Name)] = i
		nameMgrToIdx[strings.ToLower(p.Name)] = i
		purl := MapManagerToPURL(p.Manager, p.Name, p.CurrentVersion)
		if purl == "" {
			continue
		}
		purlToIdx[purl] = i
		components = append(components, cdxComponent{
			Type:    "library",
			BOMRef:  purl,
			Name:    p.Name,
			Version: p.CurrentVersion,
			PURL:    purl,
		})
	}

	var wg sync.WaitGroup

	if len(components) > 0 {
		bom := cdxBOM{
			BOMFormat:   "CycloneDX",
			SpecVersion: "1.5",
			Version:     1,
			Metadata: cdxMetadata{
				Component: cdxComponent{
					Type:    "application",
					Name:    "vanguard-fleet-scan",
					Version: "1.0.0",
				},
			},
			Components: components,
		}

		if bomBytes, err := json.Marshal(bom); err == nil {
			tmpDir, err := os.MkdirTemp("", "vanguard-sbom-*")
			if err == nil {
				defer os.RemoveAll(tmpDir)
				sbomPath := filepath.Join(tmpDir, "fleet.cdx.json")
				if err := os.WriteFile(sbomPath, bomBytes, 0600); err == nil {
					// Tier 1A: osv-scanner
					if osvPath, err := exec.LookPath("osv-scanner"); err == nil && osvPath != "" {
						wg.Add(1)
						go func() {
							defer wg.Done()
							runOSVScannerCLI(ctx, osvPath, sbomPath, pkgs, purlToIdx, nameMgrToIdx, addFinding)
						}()
					}

					// Tier 1B: trivy sbom
					if trivyPath, err := exec.LookPath("trivy"); err == nil && trivyPath != "" {
						wg.Add(1)
						go func() {
							defer wg.Done()
							runTrivyCLI(ctx, trivyPath, sbomPath, pkgs, purlToIdx, nameMgrToIdx, addFinding)
						}()
					}
				}
			}
		}
	}

	// Tier 2A: Native npm audit for npm/pnpm/yarn/bun packages
	wg.Add(1)
	go func() {
		defer wg.Done()
		runNativeNPMAudit(ctx, pkgs, nameMgrToIdx, addFinding)
	}()

	// Tier 2B: Native pip-audit for Python packages (pip, pipx, uv)
	wg.Add(1)
	go func() {
		defer wg.Done()
		runNativePipAudit(ctx, pkgs, nameMgrToIdx, addFinding)
	}()

	wg.Wait()
	return results
}

type osvScannerCLIOutput struct {
	Results []struct {
		Packages []struct {
			Package struct {
				Name    string `json:"name"`
				Version string `json:"version"`
				PURL    string `json:"purl"`
			} `json:"package"`
			Groups []struct {
				IDs         []string `json:"ids"`
				Aliases     []string `json:"aliases"`
				MaxSeverity string   `json:"max_severity"`
			} `json:"groups"`
			Vulnerabilities []OSVVulnDetail `json:"vulnerabilities"`
		} `json:"packages"`
	} `json:"results"`
}

func runOSVScannerCLI(
	ctx context.Context,
	binPath, sbomPath string,
	pkgs []domain.Package,
	purlToIdx, nameMgrToIdx map[string]int,
	addFinding func(int, domain.Vulnerability),
) {
	subCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(subCtx, binPath, "scan", "source", "-L", sbomPath, "--format", "json")
	out, _ := cmd.Output()
	if len(out) == 0 {
		return
	}

	// osv-scanner may emit status lines before `{`
	idx := bytes.IndexByte(out, '{')
	if idx == -1 {
		return
	}

	var parsed osvScannerCLIOutput
	if err := json.Unmarshal(out[idx:], &parsed); err != nil {
		return
	}

	for _, res := range parsed.Results {
		for _, pkgEntry := range res.Packages {
			pkgIdx, ok := purlToIdx[pkgEntry.Package.PURL]
			if !ok {
				pkgIdx, ok = nameMgrToIdx[strings.ToLower(pkgEntry.Package.Name)]
				if !ok {
					continue
				}
			}

			groupScores := make(map[string]float64)
			for _, g := range pkgEntry.Groups {
				if sc, err := strconv.ParseFloat(g.MaxSeverity, 64); err == nil && sc > 0 {
					for _, id := range g.IDs {
						groupScores[id] = sc
					}
					for _, al := range g.Aliases {
						groupScores[al] = sc
					}
				}
			}

			for _, v := range pkgEntry.Vulnerabilities {
				score := groupScores[v.ID]
				sev := strings.ToUpper(v.DatabaseSpecific.Severity)
				if sev == "MODERATE" {
					sev = "MEDIUM"
				}
				if sev == "" {
					switch {
					case score >= 9.0:
						sev = "CRITICAL"
					case score >= 7.0:
						sev = "HIGH"
					case score >= 4.0:
						sev = "MEDIUM"
					default:
						sev = "HIGH"
					}
				}

				id := formatCanonicalVulnID(v.ID, append(v.Aliases, v.Upstream...))
				summary := strings.TrimSpace(v.Summary)
				if summary == "" && v.Details != "" {
					summary = firstLineSummary(v.Details)
				}
				if summary == "" {
					summary = "Security advisory detected by osv-scanner"
				}

				fixedIn := extractFixedVersion(&v)
				addFinding(pkgIdx, domain.Vulnerability{
					ID:          id,
					Summary:     summary,
					Severity:    sev,
					Score:       score,
					Source:      "osv-scanner",
					FixedIn:     fixedIn,
					IsMitigated: isVersionMitigated(pkgs[pkgIdx].LatestVersion, fixedIn),
				})
			}
		}
	}
}

type trivyCLIOutput struct {
	Results []struct {
		Vulnerabilities []struct {
			VulnerabilityID string   `json:"VulnerabilityID"`
			VendorIDs       []string `json:"VendorIDs"`
			PkgName         string   `json:"PkgName"`
			PkgIdentifier   struct {
				PURL string `json:"PURL"`
			} `json:"PkgIdentifier"`
			InstalledVersion string `json:"InstalledVersion"`
			FixedVersion     string `json:"FixedVersion"`
			Severity         string `json:"Severity"`
			Title            string `json:"Title"`
			Description      string `json:"Description"`
			CVSS             map[string]struct {
				V3Score float64 `json:"V3Score"`
			} `json:"CVSS"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
}

func runTrivyCLI(
	ctx context.Context,
	binPath, sbomPath string,
	pkgs []domain.Package,
	purlToIdx, nameMgrToIdx map[string]int,
	addFinding func(int, domain.Vulnerability),
) {
	subCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	cmd := exec.CommandContext(subCtx, binPath, "sbom", "--format", "json", "--quiet", sbomPath)
	out, _ := cmd.Output()
	if len(out) == 0 {
		return
	}

	idx := bytes.IndexByte(out, '{')
	if idx == -1 {
		return
	}

	var parsed trivyCLIOutput
	if err := json.Unmarshal(out[idx:], &parsed); err != nil {
		return
	}

	for _, res := range parsed.Results {
		for _, tv := range res.Vulnerabilities {
			pkgIdx, ok := purlToIdx[tv.PkgIdentifier.PURL]
			if !ok {
				pkgIdx, ok = nameMgrToIdx[strings.ToLower(tv.PkgName)]
				if !ok {
					continue
				}
			}

			var maxScore float64
			for _, c := range tv.CVSS {
				if c.V3Score > maxScore {
					maxScore = c.V3Score
				}
			}

			sev := strings.ToUpper(tv.Severity)
			if sev == "" || sev == "UNKNOWN" {
				switch {
				case maxScore >= 9.0:
					sev = "CRITICAL"
				case maxScore >= 7.0:
					sev = "HIGH"
				case maxScore >= 4.0:
					sev = "MEDIUM"
				default:
					sev = "LOW"
				}
			}

			summary := strings.TrimSpace(tv.Title)
			if summary == "" && tv.Description != "" {
				summary = firstLineSummary(tv.Description)
			}
			if summary == "" {
				summary = "Vulnerability detected by Trivy"
			}

			// First fixed version if comma-separated
			fixedIn := strings.TrimSpace(strings.Split(tv.FixedVersion, ",")[0])
			id := formatCanonicalVulnID(tv.VulnerabilityID, tv.VendorIDs)

			addFinding(pkgIdx, domain.Vulnerability{
				ID:          id,
				Summary:     summary,
				Severity:    sev,
				Score:       maxScore,
				Source:      "Trivy",
				FixedIn:     fixedIn,
				IsMitigated: isVersionMitigated(pkgs[pkgIdx].LatestVersion, fixedIn),
			})
		}
	}
}

func runNativeNPMAudit(
	ctx context.Context,
	pkgs []domain.Package,
	nameMgrToIdx map[string]int,
	addFinding func(int, domain.Vulnerability),
) {
	npmPath, err := exec.LookPath("npm")
	if err != nil || npmPath == "" {
		return
	}

	deps := make(map[string]string)
	for _, p := range pkgs {
		switch strings.ToLower(p.Manager) {
		case "npm", "pnpm", "yarn", "bun":
			deps[p.Name] = p.CurrentVersion
		}
	}
	if len(deps) == 0 {
		return
	}

	tmpDir, err := os.MkdirTemp("", "vanguard-npm-audit-*")
	if err != nil {
		return
	}
	defer os.RemoveAll(tmpDir)

	pkgJSON := map[string]interface{}{
		"name":         "vanguard-npm-audit",
		"version":      "1.0.0",
		"private":      true,
		"dependencies": deps,
	}
	b, _ := json.Marshal(pkgJSON)
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), b, 0600); err != nil {
		return
	}

	subCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	// Generate lockfile and run npm audit --json
	lockCmd := exec.CommandContext(subCtx, npmPath, "install", "--package-lock-only", "--ignore-scripts", "--no-audit")
	lockCmd.Dir = tmpDir
	if err := lockCmd.Run(); err != nil {
		return
	}

	auditCmd := exec.CommandContext(subCtx, npmPath, "audit", "--json")
	auditCmd.Dir = tmpDir
	out, _ := auditCmd.Output()
	if len(out) == 0 {
		return
	}

	var auditResp struct {
		Vulnerabilities map[string]struct {
			Name     string          `json:"name"`
			Severity string          `json:"severity"`
			Via      json.RawMessage `json:"via"`
		} `json:"vulnerabilities"`
	}
	if err := json.Unmarshal(out, &auditResp); err != nil {
		return
	}

	for pkgName, vuln := range auditResp.Vulnerabilities {
		pkgIdx, ok := nameMgrToIdx["npm:"+strings.ToLower(pkgName)]
		if !ok {
			pkgIdx, ok = nameMgrToIdx[strings.ToLower(pkgName)]
			if !ok {
				continue
			}
		}

		var viaObjects []struct {
			Title    string  `json:"title"`
			URL      string  `json:"url"`
			Severity string  `json:"severity"`
			CVSS     struct {
				Score float64 `json:"score"`
			} `json:"cvss"`
		}
		_ = json.Unmarshal(vuln.Via, &viaObjects)
		for _, vo := range viaObjects {
			if vo.Title == "" {
				continue
			}
			id := "NPM-ADVISORY"
			if idx := strings.LastIndex(vo.URL, "/"); idx != -1 {
				id = vo.URL[idx+1:]
			}
			sev := strings.ToUpper(vo.Severity)
			if sev == "MODERATE" {
				sev = "MEDIUM"
			}
			if sev == "" {
				sev = strings.ToUpper(vuln.Severity)
			}
			addFinding(pkgIdx, domain.Vulnerability{
				ID:          id,
				Summary:     vo.Title,
				Severity:    sev,
				Score:       vo.CVSS.Score,
				Source:      "npm audit",
				IsMitigated: true,
			})
		}
	}
}

func runNativePipAudit(
	ctx context.Context,
	pkgs []domain.Package,
	nameMgrToIdx map[string]int,
	addFinding func(int, domain.Vulnerability),
) {
	pipAuditPath, err := exec.LookPath("pip-audit")
	if err != nil || pipAuditPath == "" {
		return
	}

	var reqLines []string
	for _, p := range pkgs {
		switch strings.ToLower(p.Manager) {
		case "pip", "pipx", "uv", "poetry", "pdm":
			reqLines = append(reqLines, fmt.Sprintf("%s==%s", p.Name, p.CurrentVersion))
		}
	}
	if len(reqLines) == 0 {
		return
	}

	tmpDir, err := os.MkdirTemp("", "vanguard-pip-audit-*")
	if err != nil {
		return
	}
	defer os.RemoveAll(tmpDir)

	reqFile := filepath.Join(tmpDir, "requirements.txt")
	if err := os.WriteFile(reqFile, []byte(strings.Join(reqLines, "\n")+"\n"), 0600); err != nil {
		return
	}

	subCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(subCtx, pipAuditPath, "-r", reqFile, "--format", "json", "--progress-spinner", "off", "--no-deps")
	out, _ := cmd.Output()
	if len(out) == 0 {
		return
	}

	var parsed struct {
		Dependencies []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
			Vulns   []struct {
				ID          string   `json:"id"`
				Aliases     []string `json:"aliases"`
				Description string   `json:"description"`
				FixVersions []string `json:"fix_versions"`
			} `json:"vulns"`
		} `json:"dependencies"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return
	}

	for _, dep := range parsed.Dependencies {
		pkgIdx, ok := nameMgrToIdx[strings.ToLower(dep.Name)]
		if !ok {
			continue
		}
		for _, v := range dep.Vulns {
			fixedIn := ""
			if len(v.FixVersions) > 0 {
				fixedIn = v.FixVersions[0]
			}
			id := formatCanonicalVulnID(v.ID, v.Aliases)
			summary := firstLineSummary(v.Description)
			if summary == "" {
				summary = "Python security advisory detected by pip-audit"
			}
			addFinding(pkgIdx, domain.Vulnerability{
				ID:          id,
				Summary:     summary,
				Severity:    "HIGH",
				Source:      "pip-audit",
				FixedIn:     fixedIn,
				IsMitigated: isVersionMitigated(pkgs[pkgIdx].LatestVersion, fixedIn),
			})
		}
	}
}

func formatCanonicalVulnID(primary string, aliases []string) string {
	cveAlias := ""
	ghsaAlias := ""
	for _, a := range aliases {
		if strings.HasPrefix(a, "CVE-") && cveAlias == "" {
			cveAlias = a
		}
		if strings.HasPrefix(a, "GHSA-") && ghsaAlias == "" {
			ghsaAlias = a
		}
	}
	if strings.HasPrefix(primary, "CVE-") {
		if ghsaAlias != "" {
			return fmt.Sprintf("%s (%s)", primary, ghsaAlias)
		}
		return primary
	}
	if cveAlias != "" {
		return fmt.Sprintf("%s (%s)", cveAlias, primary)
	}
	return primary
}

func firstLineSummary(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	s := strings.TrimSpace(lines[0])
	if len(s) > 180 {
		s = s[:180] + "..."
	}
	return s
}

func extractFixedVersion(detail *OSVVulnDetail) string {
	if detail == nil {
		return ""
	}
	for _, aff := range detail.Affected {
		for _, rng := range aff.Ranges {
			for _, ev := range rng.Events {
				if ev.Fixed != "" {
					return ev.Fixed
				}
			}
		}
	}
	return ""
}

func isVersionMitigated(latestVersion, fixedIn string) bool {
	if fixedIn == "" {
		return false
	}
	tMaj, tMin, tPatch, tOk := domain.ParseVersion(latestVersion)
	fMaj, fMin, fPatch, fOk := domain.ParseVersion(fixedIn)
	if tOk && fOk {
		if tMaj < fMaj || (tMaj == fMaj && tMin < fMin) || (tMaj == fMaj && tMin == fMin && tPatch < fPatch) {
			return false
		}
	}
	return true
}

// canonicalVulnKey extracts the primary CVE or GHSA key for deduplicating findings across scanners.
func canonicalVulnKey(id string) string {
	fields := strings.Fields(id)
	if len(fields) > 0 {
		return strings.Trim(fields[0], "()")
	}
	return id
}

// mergePackageVulnerabilities deduplicates and merges vulnerability findings from OSV API,
// osv-scanner, Trivy, and native CLI audits.
func mergePackageVulnerabilities(existing []domain.Vulnerability, extra []domain.Vulnerability) ([]domain.Vulnerability, string) {
	byKey := make(map[string]int)
	var merged []domain.Vulnerability

	addOrMerge := func(v domain.Vulnerability) {
		key := canonicalVulnKey(v.ID)
		if idx, exists := byKey[key]; exists {
			cur := &merged[idx]
			if len(v.ID) > len(cur.ID) {
				cur.ID = v.ID
			}
			if (cur.Summary == "" || strings.HasPrefix(cur.Summary, "Security advisory")) && v.Summary != "" {
				cur.Summary = v.Summary
			}
			if v.Score > cur.Score {
				cur.Score = v.Score
			}
			if severityRank(v.Severity) < severityRank(cur.Severity) {
				cur.Severity = v.Severity
			}
			if cur.FixedIn == "" && v.FixedIn != "" {
				cur.FixedIn = v.FixedIn
				cur.IsMitigated = v.IsMitigated
			}
			if v.Source != "" && !strings.Contains(cur.Source, v.Source) {
				cur.Source = cur.Source + " · " + v.Source
			}
			return
		}
		byKey[key] = len(merged)
		merged = append(merged, v)
	}

	for _, v := range existing {
		addOrMerge(v)
	}
	for _, v := range extra {
		addOrMerge(v)
	}

	maxSeverity := "LOW"
	for _, v := range merged {
		if severityRank(v.Severity) < severityRank(maxSeverity) {
			maxSeverity = v.Severity
		}
	}

	return merged, maxSeverity
}

func severityRank(sev string) int {
	switch strings.ToUpper(sev) {
	case "CRITICAL":
		return 0
	case "HIGH":
		return 1
	case "MEDIUM", "MODERATE":
		return 2
	default:
		return 3
	}
}
