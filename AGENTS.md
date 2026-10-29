# AGENTS.md — Project Guide for Vanguard

> **Single source of truth** for AI coding agents working on the Vanguard codebase.

---

## 1. Project Purpose & Philosophy

**Vanguard** is an interactive, biometric-secured vulnerability shield and package update orchestrator for macOS and Linux.

### Incremental Philosophy (Never a Replacement)
Vanguard does **not** replace existing tools:
- **Layer 3:** Vanguard provides Touch ID keepalive, OSV.dev CVE intelligence, SemVer triage, and an ephemeral web dashboard.
- **Layer 2:** MPM (`meta-package-manager`) provides granular multi-manager inventory abstraction.
- **Layer 1:** Topgrade and native managers (`brew`, `mise`, `uv`, `pnpm`, `npm`) execute the real work.

Any feature or refactoring must maintain this incremental layering: never rewrite what MPM, Topgrade, or Homebrew already do natively.

---

## 2. Architecture & Modules

The repository follows a clean, decoupled architecture:

- **`cmd/vanguard/main.go`**: CLI entrypoint. Handles flags (`--port`, `--no-open`, `--no-sudo`, `--dev`), primes Touch ID via `auth.EnsureSudo()`, starts keepalive goroutine, serves static embedded assets, and opens default browser.
- **`internal/auth/sudo.go`**: Biometric Touch ID pre-flight (`sudo -v` via `/dev/tty` / terminal) and background keepalive (`sudo -n true` every 40s) so root credentials remain valid throughout long upgrade runs.
- **`internal/security/advisor.go`**: Queries OSV.dev batch API to match outdated package versions against known vulnerabilities (CVEs, GHSAs, CVSS severity). Flags packages with `HasVulnerability = true`, sets `MaxSeverity`, and bubbles them to the top of the update queue.
- **`internal/domain/`**:
  - `package.go`: Core data models (`Package`, `Vulnerability`, `ScanResult`, `ScanSummary`, `UpgradeEvent`).
  - `semver.go`: Version parser and classifier (`ClassifyChange` returning `patch`, `minor`, `major`).
  - `semver_test.go`: Unit tests for version transition classification.
- **`internal/scanner/scanner.go`**: High-performance scanner running `mpm` (excluding noisy system cpan/gem) and `brew outdated --json=v2` in parallel, then enriched with `security.Advisor`.
- **`internal/executor/executor.go`**: Dispatches targeted upgrades grouped by package manager (`brew`, `cask`, `pnpm`, `npm`, `uv`, `mise`, `mpm`), streaming stdout/stderr line-by-line via events.
- **`internal/server/server.go`**: REST API endpoints (`/api/status`, `/api/outdated`, `/api/upgrade`, `/api/clean`) and Server-Sent Events stream (`/api/events`). Serves embedded static assets (`embed.FS`).
- **`assets.go`**: Roots `//go:embed all:web/dist` for zero-dependency standalone binary compilation.
- **`web/`**: Vite+ application (`React 19`, `Tailwind CSS v4`, `Lucide Icons`). Follows the *Operate* mode from `references/app-screens.md`: quiet chrome, flat surfaces, color reserved for data (crimson for CVE fixes, emerald for routine safe updates, amber for minor, rose for breaking major).

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
3. **Surgical Diffs:** Edit only code directly related to the user request. Do not introduce speculative abstractions or third-party libraries when standard library or existing tools suffice.
4. **No Fluff / Anti-Slop UI:** Adhere to `/design-taste-frontend` (`references/app-screens.md`): no gratuitous gradients, no icon badges in colored circles, flat 1px borders, high contrast data tables.
5. **Verification:** Always verify both `task test` and `task build` before declaring a turn complete.
