# Vanguard (`vg` / `up`)

> **Interactive vulnerability shield & package update orchestrator for macOS & Linux.**  
> Detects and prioritizes urgent CVE/zero-day patches across 20+ package managers, enforces strict SemVer breaking-change guards, and streams real-time execution logs.

---

## ⚡ The Mission: Vulnerability Shield First

Modern dev workstations run hundreds of dependencies across system package managers (`brew`, `cask`), language runtimes (`mise`, `uv`, `pip`), and package ecosystems (`npm`, `pnpm`, `gcloud`, `mas`).  
When critical vulnerabilities emerge (such as zero-days, RCEs, prototype pollution, or supply-chain poisoning):
1. **Security blindness:** Developers delay updates because terminal updaters either dump an avalanche of noisy logs or force risky major breaking changes.
2. **Delayed patching:** Routine updates hide critical security patches in a flat list.
3. **Privilege friction:** Repeated sudo prompts interrupt long upgrade sessions.

**Vanguard** transforms system updating into a proactive security habit:
- **🚨 Urgent CVE & Zero-Day Prioritization:** Live integration with **OSV.dev** (Open Source Vulnerabilities) automatically scans every outdated package against known CVEs and GHSAs, calculates CVSS severity, bubbles vulnerable packages to the top, and marks them for urgent 1-click remediation.
- **🚦 SemVer Breaking-Change Guard:**
  - **Urgent Security Fixes:** Highlighted with pulsing danger badges and pre-checked.
  - **Routine Safe Updates (Patch & Minor):** Auto-checked for safe 1-click execution.
  - **Major Releases:** Unselected by default with warning callouts to prevent breaking APIs or CLIs.
- **🌐 20+ Package Managers Unified:** Dispatches granular upgrade commands across `brew`, `cask`, `mise`, `uv`, `pnpm`, `npm`, `mas`, `gcloud`, etc., leveraging MPM and falling back to Topgrade.
- **⚡ Ephemeral Web Dashboard:** Built with **Vite+**, **React 19**, **Tailwind CSS v4**, and **Lucide Icons** following substance-first, quiet-chrome UI principles.
- **📡 Real-Time SSE Log Streaming:** Execution logs stream directly from child processes to a collapsible terminal drawer via Server-Sent Events.
- **🛡️ Frictionless Privilege Keepalive:** Primes root permissions once at launch and maintains background credentials (`sudo -n true` every 40s) so terminal prompts never interrupt the workflow (supporting macOS Touch ID when enabled).
- **📦 Single Self-Contained Binary:** Go `embed.FS` packs all compiled HTML/JS/CSS assets inside an 11MB standalone executable with zero runtime dependencies.
- **🧹 System Cleanup:** Integrated one-click disk and cache cleanup via `mo clean` (Mole) or `brew cleanup`.

---

## 🏗️ Architecture: Incremental by Design

> **Vanguard is an additive, incremental layer — not a replacement.**  
> It does not reinvent package management or discard battle-tested tools. Vanguard wraps `mpm`; `mpm` wraps `brew`, native language managers, and falls back to `topgrade` as its deliberate sinkhole. You keep 100% of your existing configurations, lockfiles, and tools while gaining proactive zero-day detection, SemVer safety, and interactive visual triage.

```
┌───────────────────────────────────────────────────────────────────────────┐
│                    Layer 3: Vanguard (Shield & UX)                        │
│   OSV.dev CVE Intelligence • SemVer Breaking Guard • Privilege Keepalive  │
│        Interactive Ephemeral Dashboard • SSE Real-Time Terminal Log       │
└─────────────────────────────────────▲─────────────────────────────────────┘
                                      │
┌─────────────────────────────────────┴─────────────────────────────────────┐
│                 Layer 2: MPM (Meta Package Manager)                       │
│    Granular Multi-Ecosystem Abstraction & Inspection (20+ Managers)       │
└─────────────────────────────────────▲─────────────────────────────────────┘
                                      │ (delegation & sinkhole fallback)
┌─────────────────────────────────────┴─────────────────────────────────────┐
│               Layer 1: Topgrade & Native Package Managers                 │
│      Topgrade (System, Rustup, Vim, OS firmware)                          │
│      Homebrew (Formulae & Casks) • mise • uv • pnpm • npm • mas • gcloud   │
└───────────────────────────────────────────────────────────────────────────┘
```

---

## 🚀 Quickstart

### Option 1: Install via Homebrew (Recommended)
```bash
brew tap jankrloz/tap
brew install vanguard

# Or in a single command:
brew install jankrloz/tap/vanguard
```

### Option 2: Build from Source via Taskfile
```bash
git clone https://github.com/jankrloz/vanguard.git ~/dev/personal/vanguard
cd ~/dev/personal/vanguard

task build
task install
```

### Running Vanguard
```fish
# In any terminal (launches interactive dashboard with Touch ID)
vg

# Or using the classic alias
up
```

---

## 🛠️ CLI Modes & Commands

```bash
# Launch interactive web dashboard (default)
vg
vg dashboard

# Headless security & outdated scan (exits 1 if critical CVEs found)
vg check
vg check --security-only
vg check --manager cask
vg check --limit 20

# Headless non-interactive package upgrades directly in terminal
vg apply --yes
vg apply --security-only -y
vg apply --manager npm -y
vg apply --all -y

# View persistent upgrade audit history
vg history --limit 30

# Run deep system-wide sweep via Topgrade
vg sweep

# Run background security monitor with macOS notifications
vg monitor --interval 1h
```

### Dashboard Flags

| Flag | Default | Description |
|---|---|---|
| `--port <int>` | `9099` | Local HTTP server port (uses random free port if taken) |
| `--no-open` | `false` | Do not automatically open the dashboard in default browser |
| `--no-sudo` | `false` | Skip privilege pre-flight priming |
| `--dev` | `false` | Reverse-proxy to Vite HMR server (`http://127.0.0.1:5173`) |

---

## ⚙️ Configuration & Audit Trail

- **Configuration & Pins:** Stored in `~/.config/vanguard/config.toml` (persists pinned packages that should never be upgraded, and ignored managers).
- **Audit History:** Stored in `~/.local/state/vanguard/history.jsonl` (JSONL log recording timestamp, package ID, manager, version transitions, and execution outcome).

---

## 🧑‍💻 Development

Use [`go-task`](https://taskfile.dev):

```bash
# Start frontend and backend in dev mode
task dev

# Run unit test suite
task test

# Rebuild all artifacts
task build
```

---

## 📜 License

MIT © Juan Carlos Navarrete Gordillo
