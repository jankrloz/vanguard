package security

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
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
			ID       string   `json:"id"`
			Summary  string   `json:"summary"`
			Details  string   `json:"details"`
			Upstream []string `json:"upstream"`
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
		} `json:"vulns"`
	} `json:"results"`
}

type Advisor struct {
	client  *http.Client
	baseURL string
}

func NewAdvisor() *Advisor {
	return &Advisor{
		client: &http.Client{
			Timeout: 8 * time.Second,
		},
		baseURL: "https://api.osv.dev/v1/querybatch",
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

// EnrichPackages queries OSV.dev and annotates packages with CVEs and security advisories.
func (a *Advisor) EnrichPackages(ctx context.Context, pkgs []domain.Package) []domain.Package {
	if len(pkgs) == 0 {
		return pkgs
	}

	// 1. Identify packages belonging to supported OSV ecosystems (prevents cross-ecosystem false matches)
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

	// 2. Query in chunks of max 500 (OSV API maximum is 1000)
	const chunkSize = 500
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
			var vulns []domain.Vulnerability
			maxSeverity := "LOW"

			for _, v := range res.Vulns {
				id := v.ID
				if len(v.Upstream) > 0 {
					id = v.Upstream[0] + " (" + v.ID + ")"
				}

				summary := v.Summary
				if summary == "" && v.Details != "" {
					summary = v.Details
					if len(summary) > 80 {
						summary = summary[:80] + "..."
					}
				}

				source := "OSV"
				if strings.HasPrefix(v.ID, "GHSA-") {
					source = "GitHub Advisory (GHSA)"
				} else if strings.HasPrefix(v.ID, "CVE-") {
					source = "NVD / CVE"
				} else if strings.HasPrefix(v.ID, "RUSTSEC-") {
					source = "RustSec (Cargo Audit)"
				} else if strings.HasPrefix(v.ID, "PYSEC-") {
					source = "PyPA (Pip Audit)"
				} else if strings.HasPrefix(v.ID, "GO-") {
					source = "Go Vulnerability Database"
				}

				score := 0.0
				sev := strings.ToUpper(v.DatabaseSpecific.Severity)
				for _, s := range v.Severity {
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
					}
				}

				if sev == "" {
					sev = "HIGH"
				}
				if sev == "MODERATE" {
					sev = "MEDIUM"
				}

				if sev == "CRITICAL" {
					maxSeverity = "CRITICAL"
				} else if sev == "HIGH" && maxSeverity != "CRITICAL" {
					maxSeverity = "HIGH"
				} else if (sev == "MEDIUM" || sev == "MODERATE") && maxSeverity != "CRITICAL" && maxSeverity != "HIGH" {
					maxSeverity = "MEDIUM"
				}

				// Find fixed version from ranges
				fixedIn := ""
				for _, aff := range v.Affected {
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
	}

	return pkgs
}
