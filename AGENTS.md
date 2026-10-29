# AGENTS.md — Project Guide for Vanguard

> **Single source of truth** for AI coding agents working on the Vanguard codebase.

---

## 1. Project Purpose & Philosophy

**Vanguard** (`vg` / `up`) is a unified vulnerability shield and package update orchestrator for macOS and Linux.

### Incremental Philosophy (Layered Orchestration with Silent Native Fallback)
- **Layer 3 (Vanguard):** Multi-engine vulnerability intelligence (`OSV.dev`, `GHSA`, `osv-scanner`, `Trivy`, `npm audit`, `pip-audit`), SemVer breaking-change guards (`Safe Minor/Patch` vs. `Major Breaking`), interactive Web Dashboard, headless CLI, persistent JSONL audit trail, and privilege keepalive.
- **Layer 2 (MPM):** `meta-package-manager` provides primary multi-ecosystem discovery and upgrade orchestration across 35+ package managers.
- **Layer 1 (Native CLIs & Topgrade):** When `mpm` fails or lacks support for a manager/identifier (e.g., tap-qualified `brew` formulae, `gcloud` components, `uv`/`uvx`, `npm`, `pnpm`), Vanguard silently falls back to the native package manager CLI and isolates faults per package. System-wide maintenance sweeps delegate to `topgrade`.

---

## 2. Architecture & Modules

- **`cmd/vanguard/`**:
  - `main.go`: Entrypoint dispatcher (`check`, `apply`, `history`, `sweep`, `monitor`, `dashboard`, `version`).
  - `cli.go`: Headless CLI implementations:
    - `vg check [--security-only] [--manager <mgr>] [--limit <n>]`: Aligned severity & SemVer report; exits `1` when active vulnerabilities are present.
    - `vg apply [--all] [--security-only] [--major | --force] [--manager <mgr>] [--yes/-y]`: Terminal upgrade runner; `--all` (and default) applies safe minor/patch and security updates, while `--major` or `--force` includes major breaking upgrades.
    - `vg history [--limit <n>]`: Displays aligned upgrade audit records from `~/.local/state/vanguard/history.jsonl`.
    - `vg sweep`: Deep system maintenance sweep via Topgrade.
    - `vg monitor [--interval <duration>]`: Background vulnerability polling daemon with desktop notifications.
- **`internal/ui/brand.go`**: Terminal UI styling powered by `charm.land/lipgloss/v2` (`v2.0.6`) and `charm.land/bubbles/v2` (`v2.2.1` `spinner` and `progress` components) with automatic non-TTY headless fallback.
- **`internal/auth/sudo.go`**: Privilege pre-flight and background keepalive (`sudo -n true` every 40s), with strict user-namespace protection so user-scoped managers (`brew`, `npm`, `pnpm`, `cargo`, `pip`, `uv`, `uvx`, `gcloud`) never run under `sudo`.
- **`internal/config/config.go`**: Persistent configuration at `~/.config/vanguard/config.toml` (overridable via `VANGUARD_CONFIG_DIR`). Manages pinned packages and ignored managers.
- **`internal/history/history.go`**: Append-only JSONL audit log at `~/.local/state/vanguard/history.jsonl` (overridable via `VANGUARD_STATE_DIR`).
- **`internal/security/`**:
  - `advisor.go`: Queries `OSV.dev` batch API (`/v1/querybatch`) and hydrates full `GHSA`/`CVE` advisory details (`/v1/vulns/<id>`), computing CVSS v3.1 severity tiers (`CRITICAL`, `HIGH`, `MEDIUM`, `LOW`) and remediation versions.
  - `scanners.go`: Synthesizes CycloneDX 1.5 SBOM vulnerability scans via `osv-scanner` and `trivy` alongside native CLI security audits (`npm audit --global --json`, `pip-audit --format=json`) with canonical CVE/GHSA deduplication.
- **`internal/domain/`**:
  - `package.go`: Core domain entities (`Package`, `Vulnerability`, `ScanResult`, `ScanSummary`, `UpgradeEvent`).
  - `semver.go`: SemVer parser and classifier (`ClassifyChange` returning `patch`, `minor`, `major`).
- **`internal/scanner/scanner.go`**: Runs `mpm --accessible --no-color outdated`, filters out macOS system CPAN, SIP-protected system Ruby 2.6, unactivated `uv` base interpreters, and Git/path-sourced `uvx` receipts, and hydrates package descriptions (`brew info --json=v2 --installed`, PyPI versioned JSON API, npm registry, RubyGems, Crates.io, and `gcloud` component catalog).
- **`internal/executor/executor.go`**: Runs `mpm --<manager> upgrade` first (buffered), silently falls back to native package manager commands (`buildNativeFallbackCommands`) across 23 ecosystems when `mpm` fails, isolates faults per package on multi-package batch failures, and records outcomes to `history.jsonl`.
- **`internal/server/server.go`**: Loopback-hardened HTTP server (`127.0.0.1`) with DNS-rebinding Host validation, Origin CSRF guards, REST API endpoints (`/api/status`, `/api/outdated`, `/api/upgrade`, `/api/clean`, `/api/topgrade`, `/api/history`, `/api/config/pin`), and SSE live log streaming (`/api/events`).
- **`assets.go`**: Embeds compiled frontend (`//go:embed all:web/dist`) into the single Go binary.
- **`web/`**: Vite+ SPA (`React 19`, `Tailwind CSS v4`, `Lucide Icons`).
- **`docs/`**: Static landing page (`docs/index.html`) published via GitHub Pages.

---

## 3. Toolchain & Task Runner

All project operations are executed through **`go-task`** (`Taskfile.yml`):

| Command | Action |
|---|---|
| `task` / `task build` | Builds frontend assets (`web/dist`) and compiles Go binary (`bin/vanguard`) |
| `task build:web` | Compiles frontend assets in `web/dist` using `vp build` |
| `task build:go` | Compiles Go backend with embedded static assets |
| `task test` | Runs Go test suite (`go test -v ./...`) |
| `task install` | Installs `vanguard`, `vg`, and `up` to `~/.local/bin` and syncs Homebrew Cellar binary if linked |
| `task dev` | Runs backend in reverse-proxy mode (`--dev`) for live frontend HMR |

---

## 4. Engineering Standards

1. **Vulnerability Priority:** Packages with active CVEs or security advisories are ranked first (`CRITICAL` > `HIGH` > `MEDIUM` > `LOW`) and pre-selected for remediation.
2. **Strict SemVer Guard:** Routine `major` updates are unselected by default in the Web UI and held back in `vg apply` unless `--major` or `--force` is passed.
3. **MPM-First + Silent Native Fallback:** Always attempt `mpm` first in `ExecuteUpgrades` and fall back silently to native CLI commands on failure, isolating faults per package when a multi-package batch fails.
4. **Test Isolation:** Never mutate user config or state files in tests. Always set `t.Setenv("VANGUARD_CONFIG_DIR", t.TempDir())` and `t.Setenv("VANGUARD_STATE_DIR", t.TempDir())`.
