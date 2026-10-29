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
			Upstream []string `json:"upstream"`
			Severity []struct {
				Type  string `json:"type"`
				Score string `json:"score"`
			} `json:"severity"`
			DatabaseSpecific struct {
				Severity string `json:"severity"`
			} `json:"database_specific"`
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
	case "npm", "pnpm", "bun":
		return "npm"
	case "pip", "uv", "uvx":
		return "PyPI"
	case "cargo":
		return "crates.io"
	case "composer":
		return "Packagist"
	case "gem":
		return "RubyGems"
	case "go":
		return "Go"
	default:
		return ""
	}
}

// EnrichPackages queries OSV.dev and annotates packages with CVEs and security advisories.
func (a *Advisor) EnrichPackages(ctx context.Context, pkgs []domain.Package) []domain.Package {
	if len(pkgs) == 0 {
		return pkgs
	}

	var queries []OSVQuery
	for _, p := range pkgs {
		ecosystem := MapManagerToEcosystem(p.Manager)
		q := OSVQuery{
			Package: OSVPackage{
				Name:      p.Name,
				Ecosystem: ecosystem,
			},
			Version: p.CurrentVersion,
		}
		queries = append(queries, q)
	}

	reqBody, err := json.Marshal(OSVBatchRequest{Queries: queries})
	if err != nil {
		return pkgs
	}

	endpoint := a.baseURL
	if endpoint == "" {
		endpoint = "https://api.osv.dev/v1/querybatch"
	}

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return pkgs
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return pkgs
	}
	defer resp.Body.Close()

	var batchResp OSVBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return pkgs
	}

	for i, res := range batchResp.Results {
		if i >= len(pkgs) || len(res.Vulns) == 0 {
			continue
		}

		var vulns []domain.Vulnerability
		maxSeverity := "LOW"

		for _, v := range res.Vulns {
			id := v.ID
			if len(v.Upstream) > 0 {
				id = v.Upstream[0] + " (" + v.ID + ")"
			}

			sev := strings.ToUpper(v.DatabaseSpecific.Severity)
			if sev == "" {
				sev = "HIGH"
			}

			if sev == "CRITICAL" {
				maxSeverity = "CRITICAL"
			} else if sev == "HIGH" && maxSeverity != "CRITICAL" {
				maxSeverity = "HIGH"
			} else if sev == "MODERATE" && maxSeverity != "CRITICAL" && maxSeverity != "HIGH" {
				maxSeverity = "MODERATE"
			}

			vulns = append(vulns, domain.Vulnerability{
				ID:       id,
				Summary:  v.Summary,
				Severity: sev,
			})
		}

		pkgs[i].Vulnerabilities = vulns
		pkgs[i].HasVulnerability = true
		pkgs[i].MaxSeverity = maxSeverity
		// Security fixes should be urgently prioritized and pre-checked
		pkgs[i].Selected = true
	}

	return pkgs
}
