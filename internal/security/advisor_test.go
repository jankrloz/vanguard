package security

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jankrloz/vanguard/internal/domain"
)

func TestMapManagerToEcosystem(t *testing.T) {
	cases := []struct {
		manager   string
		ecosystem string
	}{
		{"npm", "npm"},
		{"pnpm", "npm"},
		{"bun", "npm"},
		{"pip", "PyPI"},
		{"uv", "PyPI"},
		{"uvx", "PyPI"},
		{"cargo", "crates.io"},
		{"composer", "Packagist"},
		{"gem", "RubyGems"},
		{"go", "Go"},
		{"unknown", ""},
	}

	for _, c := range cases {
		got := MapManagerToEcosystem(c.manager)
		if got != c.ecosystem {
			t.Errorf("MapManagerToEcosystem(%q) = %q, want %q", c.manager, got, c.ecosystem)
		}
	}
}

func TestEnrichPackages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := OSVBatchResponse{
			Results: []struct {
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
			}{
				{
					Vulns: []struct {
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
					}{
						{
							ID:       "GHSA-1234",
							Summary:  "Remote Code Execution",
							Upstream: []string{"CVE-2024-9999"},
							DatabaseSpecific: struct {
								Severity string `json:"severity"`
							}{
								Severity: "CRITICAL",
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	advisor := &Advisor{
		client:  server.Client(),
		baseURL: server.URL,
	}

	pkgs := []domain.Package{
		{
			ID:             "npm:lodash",
			Name:           "lodash",
			Manager:        "npm",
			CurrentVersion: "4.17.15",
			LatestVersion:  "4.17.21",
			ChangeType:     domain.ChangePatch,
			Safe:           true,
			Selected:       false,
		},
	}

	enriched := advisor.EnrichPackages(context.Background(), pkgs)

	if len(enriched) != 1 {
		t.Fatalf("expected 1 package, got %d", len(enriched))
	}

	p := enriched[0]
	if !p.HasVulnerability {
		t.Errorf("expected package to have vulnerability")
	}
	if p.MaxSeverity != "CRITICAL" {
		t.Errorf("expected max severity CRITICAL, got %s", p.MaxSeverity)
	}
	if !p.Selected {
		t.Errorf("expected vulnerable package to be auto-selected")
	}
	if len(p.Vulnerabilities) != 1 || p.Vulnerabilities[0].ID != "CVE-2024-9999 (GHSA-1234)" {
		t.Errorf("unexpected vulnerability ID: %+v", p.Vulnerabilities)
	}
}
