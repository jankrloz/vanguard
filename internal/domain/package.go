package domain

type ChangeType string

const (
	ChangePatch   ChangeType = "patch"
	ChangeMinor   ChangeType = "minor"
	ChangeMajor   ChangeType = "major"
	ChangeUnknown ChangeType = "unknown"
)

type Package struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Manager        string     `json:"manager"`
	CurrentVersion string     `json:"current_version"`
	LatestVersion  string     `json:"latest_version"`
	ChangeType     ChangeType `json:"change_type"`
	Safe           bool       `json:"safe"`
	Pinned         bool       `json:"pinned"`
	Selected       bool       `json:"selected"`
	Description    string     `json:"description,omitempty"`
}

type ScanSummary struct {
	TotalPackages int `json:"total_packages"`
	SafeCount     int `json:"safe_count"`
	MajorCount    int `json:"major_count"`
	PinnedCount   int `json:"pinned_count"`
}

type ScanResult struct {
	Packages []Package   `json:"packages"`
	Summary  ScanSummary `json:"summary"`
}

type UpgradeRequest struct {
	Packages []string `json:"packages"` // IDs or names to upgrade
}

type UpgradeEvent struct {
	Type    string `json:"type"`    // "start", "log", "package_start", "package_done", "complete", "error"
	Package string `json:"package,omitempty"`
	Message string `json:"message"`
	Status  string `json:"status,omitempty"` // "ok", "failed", "running"
}
