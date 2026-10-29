package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/jankrloz/vanguard/internal/domain"
)

type OSVQuery struct {
	Package OSVPackage `json:"package"`
	Version string     `json:"version,omitempty"`
}

type OSVPackage struct {
	Name      string `json:"name"`
	Ecosystem string `json:"ecosystem,omitempty"`
}

type OSVBatchRequest struct {
	Queries []OSVQuery `json:"queries"`
}

type OSVBatchResponse struct {
	Results []struct {
		Vulns []struct {
			ID string `json:"id"`
		} `json:"vulns"`
	} `json:"results"`
}

type OSVVulnDetail struct {
	ID       string   `json:"id"`
	Summary  string   `json:"summary"`
	Details  string   `json:"details"`
	Aliases  []string `json:"aliases"`
	Severity []struct {
		Type  string `json:"type"`
		Score string `json:"score"`
	} `json:"severity"`
	DatabaseSpecific struct {
		Severity string `json:"severity"`
	} `json:"database_specific"`
	Affected []struct {
		Ranges []struct {
			Events []struct {
				Introduced string `json:"introduced"`
				Fixed      string `json:"fixed"`
			} `json:"events"`
		} `json:"ranges"`
	} `json:"affected"`
}

type Advisor struct {
	client  *http.Client
	baseURL string
	cacheMu sync.RWMutex
	cache   map[string]*OSVVulnDetail
}

func NewAdvisor() *Advisor {
	return &Advisor{
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: "https://api.osv.dev/v1/querybatch",
		cache:   make(map[string]*OSVVulnDetail),
	}
}

// MapManagerToEcosystem maps local package manager to OSV ecosystem.
func MapManagerToEcosystem(mgr string) string {
	switch strings.ToLower(mgr) {
	case "npm", "pnpm", "bun", "yarn", "yarn-berry":
		return "npm"
	case "pip", "pipx", "uv", "uvx", "poetry", "pdm", "conda", "mamba", "pixi":
		return "PyPI"
	case "cargo":
		return "crates.io"
	case "composer":
		return "Packagist"
	case "gem", "bundler":
		return "RubyGems"
	case "go":
		return "Go"
	case "nuget", "dotnet":
		return "NuGet"
	case "pub", "flutter", "dart":
		return "Pub"
	case "hex", "mix":
		return "Hex"
	case "apt", "nala", "aptitude", "deb-get":
		return "Debian"
	case "apk":
		return "Alpine"
	default:
		return ""
	}
}

// FetchVulnDetail fetches full vulnerability advisory details by ID with caching.
func (a *Advisor) FetchVulnDetail(ctx context.Context, id string) (*OSVVulnDetail, error) {
	a.cacheMu.RLock()
	cached, ok := a.cache[id]
	a.cacheMu.RUnlock()
	if ok {
		return cached, nil
	}

	url := fmt.Sprintf("https://api.osv.dev/v1/vulns/%s", id)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("osv api status %d", resp.StatusCode)
	}

	var detail OSVVulnDetail
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		return nil, err
	}

	a.cacheMu.Lock()
	a.cache[id] = &detail
	a.cacheMu.Unlock()

	return &detail, nil
}

// EnrichPackages queries OSV.dev and annotates packages with CVEs and security advisories.
func (a *Advisor) EnrichPackages(ctx context.Context, pkgs []domain.Package) []domain.Package {
	if len(pkgs) == 0 {
		return pkgs
	}

	// 1. Identify packages belonging to supported OSV ecosystems
	type queryTarget struct {
		pkgIndex int
		query    OSVQuery
	}

	var targets []queryTarget
	for i, p := range pkgs {
		ecosystem := MapManagerToEcosystem(p.Manager)
		if ecosystem == "" {
			continue
		}
		targets = append(targets, queryTarget{
			pkgIndex: i,
			query: OSVQuery{
				Package: OSVPackage{
					Name:      p.Name,
					Ecosystem: ecosystem,
				},
				Version: p.CurrentVersion,
			},
		})
	}

	if len(targets) == 0 {
		return pkgs
	}

	endpoint := a.baseURL
	if endpoint == "" {
		endpoint = "https://api.osv.dev/v1/querybatch"
	}

	// 2. Query batch in chunks
	const chunkSize = 500
	var uniqueVulnIDs = make(map[string]bool)
	type pkgMatch struct {
		pkgIdx  int
		vulnIDs []string
	}
	var matches []pkgMatch

	for chunkStart := 0; chunkStart < len(targets); chunkStart += chunkSize {
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > len(targets) {
			chunkEnd = len(targets)
		}

		currentChunk := targets[chunkStart:chunkEnd]
		var queries []OSVQuery
		for _, t := range currentChunk {
			queries = append(queries, t.query)
		}

		reqBody, err := json.Marshal(OSVBatchRequest{Queries: queries})
		if err != nil {
			continue
		}

		req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(reqBody))
		if err != nil {
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := a.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			continue
		}

		var batchResp OSVBatchResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&batchResp)
		_ = resp.Body.Close()
		if decodeErr != nil {
			continue
		}

		for resIdx, res := range batchResp.Results {
			if resIdx >= len(currentChunk) || len(res.Vulns) == 0 {
				continue
			}

			pkgIdx := currentChunk[resIdx].pkgIndex
			var ids []string
			for _, v := range res.Vulns {
				ids = append(ids, v.ID)
				uniqueVulnIDs[v.ID] = true
			}
			matches = append(matches, pkgMatch{pkgIdx: pkgIdx, vulnIDs: ids})
		}
	}

	if len(uniqueVulnIDs) == 0 {
		return pkgs
	}

	// 3. Hydrate full details for each unique vulnerability ID concurrently
	detailsMap := make(map[string]*OSVVulnDetail)
	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // Concurrency limiter

	for id := range uniqueVulnIDs {
		wg.Add(1)
		go func(vulnID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			detailCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			detail, err := a.FetchVulnDetail(detailCtx, vulnID)
			if err == nil && detail != nil {
				mu.Lock()
				detailsMap[vulnID] = detail
				mu.Unlock()
			}
		}(id)
	}

	wg.Wait()

	// 4. Annotate matched packages
	for _, match := range matches {
		pkgIdx := match.pkgIdx
		var vulns []domain.Vulnerability
		maxSeverity := "LOW"

		for _, vid := range match.vulnIDs {
			detail := detailsMap[vid]
			id := vid
			summary := ""
			source := "OSV"
			sev := "HIGH"
			score := 0.0
			fixedIn := ""

			if detail != nil {
				// Prefer CVE alias if available
				cveAlias := ""
				for _, alias := range detail.Aliases {
					if strings.HasPrefix(alias, "CVE-") {
						cveAlias = alias
						break
					}
				}
				if cveAlias != "" && !strings.HasPrefix(id, "CVE-") {
					id = fmt.Sprintf("%s (%s)", cveAlias, vid)
				}

				summary = strings.TrimSpace(detail.Summary)
				if summary == "" && detail.Details != "" {
					lines := strings.Split(detail.Details, "\n")
					summary = strings.TrimSpace(lines[0])
					if len(summary) > 160 {
						summary = summary[:160] + "..."
					}
				}

				if strings.HasPrefix(vid, "GHSA-") {
					source = "GitHub Advisory (GHSA)"
				} else if strings.HasPrefix(vid, "CVE-") {
					source = "NVD / CVE"
				} else if strings.HasPrefix(vid, "RUSTSEC-") {
					source = "RustSec"
				} else if strings.HasPrefix(vid, "PYSEC-") {
					source = "PyPA"
				} else if strings.HasPrefix(vid, "GO-") {
					source = "Go Vuln DB"
				}

				sev = strings.ToUpper(detail.DatabaseSpecific.Severity)
				for _, s := range detail.Severity {
					if strings.Contains(s.Score, "AV:N") && strings.Contains(s.Score, "C:H") && strings.Contains(s.Score, "I:H") {
						if sev == "" {
							sev = "CRITICAL"
						}
						score = 9.8
					} else if strings.Contains(s.Score, "AV:N") {
						if sev == "" {
							sev = "HIGH"
						}
						score = 7.5
					} else if score == 0.0 {
						score = 5.0
					}
				}

				if sev == "" {
					sev = "HIGH"
				}
				if sev == "MODERATE" {
					sev = "MEDIUM"
				}

				// Find fixed version from ranges
				for _, aff := range detail.Affected {
					for _, rng := range aff.Ranges {
						for _, ev := range rng.Events {
							if ev.Fixed != "" {
								fixedIn = ev.Fixed
								break
							}
						}
						if fixedIn != "" {
							break
						}
					}
					if fixedIn != "" {
						break
					}
				}
			}

			if summary == "" {
				summary = "Security advisory fix available in upstream release"
			}

			if sev == "CRITICAL" {
				maxSeverity = "CRITICAL"
			} else if sev == "HIGH" && maxSeverity != "CRITICAL" {
				maxSeverity = "HIGH"
			} else if (sev == "MEDIUM" || sev == "MODERATE") && maxSeverity != "CRITICAL" && maxSeverity != "HIGH" {
				maxSeverity = "MEDIUM"
			}

			isMitigated := true
			if fixedIn != "" {
				tMaj, tMin, tPatch, tOk := domain.ParseVersion(pkgs[pkgIdx].LatestVersion)
				fMaj, fMin, fPatch, fOk := domain.ParseVersion(fixedIn)
				if tOk && fOk {
					if tMaj < fMaj || (tMaj == fMaj && tMin < fMin) || (tMaj == fMaj && tMin == fMin && tPatch < fPatch) {
						isMitigated = false
					}
				}
			} else {
				isMitigated = false
			}

			vulns = append(vulns, domain.Vulnerability{
				ID:          id,
				Summary:     summary,
				Severity:    sev,
				Score:       score,
				Source:      source,
				FixedIn:     fixedIn,
				IsMitigated: isMitigated,
			})
		}

		pkgs[pkgIdx].Vulnerabilities = vulns
		pkgs[pkgIdx].HasVulnerability = true
		pkgs[pkgIdx].MaxSeverity = maxSeverity
		pkgs[pkgIdx].Selected = true
	}

	return pkgs
}
