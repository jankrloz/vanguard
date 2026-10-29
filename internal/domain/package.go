package domain

type ChangeType string

const (
	ChangePatch   ChangeType = "patch"
	ChangeMinor   ChangeType = "minor"
	ChangeMajor   ChangeType = "major"
	ChangeUnknown ChangeType = "unknown"
)

type Vulnerability struct {
	ID          string  `json:"id"`                 // CVE-202X-XXXX or GHSA-XXXX
	Summary     string  `json:"summary"`            // e.g. "Remote Code Execution"
	Severity    string  `json:"severity"`           // "CRITICAL", "HIGH", "MODERATE", "LOW"
	Score       float64 `json:"score,omitempty"`    // CVSS Base Score (e.g. 9.8)
	Source      string  `json:"source,omitempty"`   // NVD, GHSA, RustSec, PyPA, OSV
	FixedIn     string  `json:"fixed_in,omitempty"` // Version that fixes the CVE
	IsMitigated bool    `json:"is_mitigated"`       // True if candidate update fixes the CVE
}

type Package struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Manager          string          `json:"manager"`
	CurrentVersion   string          `json:"current_version"`
	LatestVersion    string          `json:"latest_version"`
	ChangeType       ChangeType      `json:"change_type"`
	Safe             bool            `json:"safe"`
	Pinned           bool            `json:"pinned"`
	Selected         bool            `json:"selected"`
	Description      string          `json:"description,omitempty"`
	SourceURL        string          `json:"source_url,omitempty"`
	Vulnerabilities  []Vulnerability `json:"vulnerabilities,omitempty"`
	HasVulnerability bool            `json:"has_vulnerability"`
	MaxSeverity      string          `json:"max_severity,omitempty"` // CRITICAL, HIGH, etc.
}

type ScanSummary struct {
	TotalPackages    int `json:"total_packages"`
	SafeCount        int `json:"safe_count"`
	MajorCount       int `json:"major_count"`
	PinnedCount      int `json:"pinned_count"`
	SecurityFixCount int `json:"security_fix_count"`
	CriticalCount    int `json:"critical_count"`
	HighCount        int `json:"high_count"`
	MediumCount      int `json:"medium_count"`
	LowCount         int `json:"low_count"`
}

type ScanResult struct {
	Packages []Package   `json:"packages"`
	Summary  ScanSummary `json:"summary"`
}

type UpgradeRequest struct {
	Packages []string `json:"packages"` // IDs or names to upgrade
}

type UpgradeEvent struct {
	Type    string `json:"type"` // "start", "log", "package_start", "package_done", "complete", "error"
	Package string `json:"package,omitempty"`
	Message string `json:"message"`
	Status  string `json:"status,omitempty"` // "ok", "failed", "running"
}
