export type ChangeType = "patch" | "minor" | "major" | "unknown";

export interface Vulnerability {
  id: string;
  summary: string;
  severity: string;
}

export interface Package {
  id: string;
  name: string;
  manager: string;
  current_version: string;
  latest_version: string;
  change_type: ChangeType;
  safe: boolean;
  pinned: boolean;
  selected: boolean;
  description?: string;
  vulnerabilities?: Vulnerability[];
  has_vulnerability?: boolean;
  max_severity?: string;
}

export interface ScanSummary {
  total_packages: number;
  safe_count: number;
  major_count: number;
  pinned_count: number;
  security_fix_count: number;
}

export interface ScanResult {
  packages: Package[];
  summary: ScanSummary;
}

export interface UpgradeEvent {
  type: "start" | "log" | "package_start" | "package_done" | "complete" | "error";
  package?: string;
  message: string;
  status?: "ok" | "failed" | "running";
}

export interface SystemStatus {
  status: string;
  sudo_active: boolean;
  platform: string;
  timestamp: string;
}
