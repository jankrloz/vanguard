# AGENTS.md — Project Guide for Vanguard

> **Single source of truth** for AI coding agents working on the Vanguard codebase.

---

## 1. Project Purpose

**Vanguard** is an interactive, biometric-secured package update orchestrator and ephemeral web dashboard for macOS and Linux. It inspects outdated packages across 20+ package managers, enforces a SemVer breaking-change guard, primes Touch ID once, and streams real-time execution logs.

---

## 2. Architecture & Modules

The repository follows a clean, decoupled architecture:

- **`cmd/up/main.go`**: CLI entrypoint. Handles flags (`--port`, `--no-open`, `--no-sudo`, `--dev`), primes Touch ID via `auth.EnsureSudo()`, starts keepalive goroutine, serves static embedded assets, and opens default browser.
- **`internal/auth/sudo.go`**: Biometric Touch ID pre-flight (`sudo -v` via `/dev/tty` / terminal) and background keepalive (`sudo -n true` every 40s) so root credentials remain valid throughout long upgrade runs.
- **`internal/domain/`**:
  - `package.go`: Core data models (`Package`, `ScanResult`, `ScanSummary`, `UpgradeEvent`).
  - `semver.go`: Version parser and classifier (`ClassifyChange` returning `patch`, `minor`, `major`).
  - `semver_test.go`: Unit tests for version transition classification.
- **`internal/scanner/scanner.go`**: High-performance scanner running `mpm` (excluding noisy system cpan/gem) and `brew outdated --json=v2` in parallel.
- **`internal/executor/executor.go`**: Dispatches targeted upgrades grouped by package manager (`brew`, `cask`, `pnpm`, `npm`, `uv`, `mise`, `mpm`), streaming stdout/stderr line-by-line via events.
- **`internal/server/server.go`**: REST API endpoints (`/api/status`, `/api/outdated`, `/api/upgrade`, `/api/clean`) and Server-Sent Events stream (`/api/events`). Serves embedded static assets (`embed.FS`).
- **`assets.go`**: Roots `//go:embed all:web/dist` for zero-dependency standalone binary compilation.
- **`web/`**: Vite+ application (`React 19`, `Tailwind CSS v4`, `Lucide Icons`). Follows the *Operate* mode from `references/app-screens.md`: quiet chrome, flat surfaces, color reserved for data (emerald for patch, amber for minor, rose for breaking major).

---

## 3. Toolchain & Task Runner

All project operations are executed through **`go-task`** (`Taskfile.yml`):

| Command | Action |
|---|---|
| `task` / `task build` | Full build: builds Vite+ frontend and compiles Go binary into `bin/up` |
| `task build:web` | Compiles frontend assets in `web/dist` using `vp build` |
| `task build:go` | Compiles Go backend with embedded static assets |
| `task test` | Runs Go test suite (`go test -v ./...`) |
| `task install` | Copies compiled binary to `~/.local/bin/up` |
| `task dev` | Runs backend in reverse-proxy mode (`--dev`) for live frontend HMR |

---

## 4. Engineering Standards

1. **Strict SemVer Guard:** Never default a `major` release to `selected: true`. Major releases are breaking changes by definition and require explicit user opt-in.
2. **Surgical Diffs:** Edit only code directly related to the user request. Do not introduce speculative abstractions or third-party libraries when standard library or existing tools suffice.
3. **No Fluff / Anti-Slop UI:** Adhere to `/design-taste-frontend` (`references/app-screens.md`): no gratuitous gradients, no icon badges in colored circles, flat 1px borders, high contrast data tables.
4. **Verification:** Always verify both `task test` and `task build` before declaring a turn complete.
