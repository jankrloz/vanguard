# AGENTS.md — Project Guide for Vanguard

> **Single source of truth** for AI coding agents working on the Vanguard codebase.

---

## 1. Project Purpose & Philosophy

**Vanguard** is an interactive vulnerability shield and package update orchestrator for macOS and Linux.

### Incremental Philosophy (Never a Replacement)
Vanguard does **not** replace existing tools:
- **Layer 3 (Vanguard):** OSV.dev CVE intelligence, SemVer breaking-change triage, interactive ephemeral web dashboard, audit logging, and privilege keepalive.
- **Layer 2 (MPM):** `meta-package-manager` provides unified multi-manager inventory abstraction across 148 managers.
- **Layer 1 (Native Tools & Topgrade):** Native package managers (`brew`, `pip`, `npm`, `pnpm`, `cargo`, `gcloud`, etc.) and Topgrade fallback execute the actual binary and file operations.

Any feature or refactoring must maintain this incremental layering: never hardcode or rewrite what MPM or Topgrade already handle natively.

---

## 2. Architecture & Modules

The repository follows a clean, decoupled architecture:

- **`cmd/vanguard/`**:
  - `main.go`: Entrypoint dispatcher for subcommands (`check`, `apply`, `history`, `sweep`, `monitor`, `dashboard`, `version`).
  - `cli.go`: Headless CLI implementations:
    - `vg check [--security-only] [--manager <mgr>] [--limit <n>]`: Formatted terminal table; exits 1 on urgent CVEs.
    - `vg apply [--security-only] [--all] [--manager <mgr>] [--yes/-y]`: Headless terminal updates.
    - `vg history [--limit <n>]`: Displays persistent audit records.
    - `vg sweep`: Deep system sweep via Topgrade.
    - `vg monitor [--interval <duration>]`: Background polling loop with native macOS notifications.
- **`internal/auth/sudo.go`**: Privilege pre-flight (`sudo -v`) and background keepalive (`sudo -n true` every 40s) so root credentials remain valid throughout long upgrade runs.
- **`internal/config/config.go`**: Persistent user configuration at `~/.config/vanguard/config.toml` (overridable via `VANGUARD_CONFIG_DIR`). Manages pinned packages and manager ignore lists.
- **`internal/history/history.go`**: Append-only JSONL audit log at `~/.local/state/vanguard/history.jsonl` (overridable via `VANGUARD_STATE_DIR`).
- **`internal/security/advisor.go`**: Queries OSV.dev batch API for packages with supported ecosystems (`npm`, `PyPI`, `crates.io`, `Packagist`, `RubyGems`, `Go`, `NuGet`, `Pub`, `Hex`, `Debian`, `Alpine`). Queries in chunks of max 500, compares fix ranges against target versions to distinguish mitigated from unmitigated zero-days, and bubbles vulnerable packages to the top.
- **`internal/domain/`**:
  - `package.go`: Core data models (`Package`, `Vulnerability`, `ScanResult`, `ScanSummary`, `UpgradeEvent`).
  - `semver.go`: Version parser and classifier (`ClassifyChange` returning `patch`, `minor`, `major`).
  - `semver_test.go`: Unit tests for version transition classification.
- **`internal/scanner/scanner.go`**: Pure MPM scanner running `mpm --accessible --no-color outdated`, parsing columns and enriching packages with OSV advisor and persistent pins.
- **`internal/executor/executor.go`**: Pure MPM execution engine running `mpm --<manager> upgrade <packages...>`, `ExecuteTopgrade`, and `ExecuteClean`, streaming events via callbacks and recording results to audit history.
- **`internal/server/server.go`**: REST API endpoints (`/api/status`, `/api/outdated`, `/api/upgrade`, `/api/clean`, `/api/topgrade`, `/api/history`, `/api/config/pin`) and Server-Sent Events stream (`/api/events`). Synchronizes concurrent scans via `scanMu` and serves embedded static assets (`embed.FS`).
- **`assets.go`**: Roots `//go:embed all:web/dist` for zero-dependency standalone binary compilation.
- **`web/`**: Vite+ application (`React 19`, `Tailwind CSS v4`, `Lucide Icons`). Quiet chrome, high-contrast data density, collapsible manager groups, copyable execution logs, and audit history drawer.

---

## 3. Toolchain & Task Runner

All project operations are executed through **`go-task`** (`Taskfile.yml`):

| Command | Action |
|---|---|
| `task` / `task build` | Full build: builds Vite+ frontend and compiles Go binary into `bin/vanguard` |
| `task build:web` | Compiles frontend assets in `web/dist` using `vp build` |
| `task build:go` | Compiles Go backend with embedded static assets |
| `task test` | Runs Go test suite (`go test -v ./...`) |
| `task install` | Copies compiled binary to `~/.local/bin/vanguard` (and updates `vg`, `up` symlinks) |
| `task dev` | Runs backend in reverse-proxy mode (`--dev`) for live frontend HMR |

---

## 4. Engineering Standards

1. **Vulnerability Priority:** Any package with active CVEs or security advisories MUST be flagged, bubbled to the top, and recommended for immediate remediation.
2. **Strict SemVer Guard:** Never default a routine `major` release to `selected: true` without explicit user opt-in, unless it fixes an active critical security advisory.
3. **Pure MPM Layering:** Never re-introduce hardcoded manager switches (`brew`, `npm`, `pnpm`, etc.). Delegate discovery and upgrade execution directly to `mpm`.
4. **Test Isolation:** Never mutate human config or state files in tests. Always use `t.Setenv("VANGUARD_CONFIG_DIR", t.TempDir())` and `t.Setenv("VANGUARD_STATE_DIR", t.TempDir())`.
5. **No Fluff UI:** Follow anti-slop principles: flat 1px borders, collapsible sections, substance over decorative chrome.
