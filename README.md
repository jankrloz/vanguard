# Vanguard (`vg` / `up`)

> **Interactive, biometric-secured vulnerability shield & package orchestrator for macOS & Linux.**  
> Detects and prioritizes urgent CVE/zero-day patches across 20+ package managers, enforces strict SemVer breaking-change guards, primes Touch ID once, and streams real-time execution logs.

---

## ⚡ The Mission: Vulnerability Shield First

Modern dev workstations run hundreds of dependencies across system package managers (`brew`, `cask`), language runtimes (`mise`, `uv`, `pip`), and package ecosystems (`npm`, `pnpm`, `gcloud`, `mas`).  
When critical vulnerabilities emerge (such as zero-days, RCEs, prototype pollution, or supply-chain poisoning):
1. **Security blindness:** Developers delay updates because terminal updaters either dump an avalanche of noisy logs or force risky major breaking changes.
2. **Delayed patching:** Routine updates hide critical security patches in a flat list.
3. **Password friction:** Repeated, blocking terminal sudo password prompts break focus and biometric Touch ID (`pam_tid.so`).

**Vanguard** transforms system updating into a proactive security habit:
- **🚨 Urgent CVE & Zero-Day Prioritization:** Live integration with **OSV.dev** (Open Source Vulnerabilities) automatically scans every outdated package against known CVEs and GHSAs, calculates CVSS severity, bubbles vulnerable packages to the top, and marks them for urgent 1-click remediation.
- **🛡️ Touch ID Biometric Priming & Keepalive:** Primes root permissions once at launch via macOS Touch ID (`pam_tid.so`) and maintains background credentials (`sudo -n true` every 40s) so terminal prompts never interrupt the workflow.
- **🚦 SemVer Breaking-Change Guard:**
  - **Urgent Security Fixes:** Highlighted with pulsing danger badges and pre-checked.
  - **Routine Safe Updates (Patch & Minor):** Auto-checked for safe 1-click execution.
  - **Major Releases:** Unselected by default with warning callouts to prevent breaking APIs or CLIs.
- **🌐 20+ Package Managers Unified:** Dispatches granular upgrade commands across `brew`, `cask`, `mise`, `uv`, `pnpm`, `npm`, `mas`, `gcloud`, etc.
- **⚡ Ephemeral Web Dashboard:** Built with **Vite+**, **React 19**, **Tailwind CSS v4**, and **Lucide Icons** following substance-first, quiet-chrome UI principles.
- **📡 Real-Time SSE Log Streaming:** Execution logs stream directly from child processes to a collapsible terminal drawer via Server-Sent Events.
- **📦 Single Self-Contained Binary:** Go `embed.FS` packs all compiled HTML/JS/CSS assets inside an 11MB standalone executable with zero runtime dependencies.
- **🧹 System Cleanup:** Integrated one-click disk and cache cleanup via `mo clean` (Mole) or `brew cleanup`.

---

## 🏗️ Architecture

```
┌────────────────────────────────────────────────────────┐
│             Vanguard Local Web Dashboard               │
│      React 19 • Tailwind CSS v4 • shadcn/ui • SSE      │
└───────────────────────────▲────────────────────────────┘
                            │ HTTP / SSE (/api/events)
┌───────────────────────────┴────────────────────────────┐
│                  Vanguard Go Engine                    │
│                                                        │
│  ┌──────────────────┐  ┌──────────────────┐  ┌───────┐  │
│  │ Touch ID Sudo    │  │ OSV.dev CVE      │  │ Server│  │
│  │ Priming & Keep   │  │ Advisor & SemVer │  │ Embed │  │
│  └────────▲─────────┘  └────────▲─────────┘  └───────┘  │
└───────────┼─────────────────────┼──────────────────────┘
            │                     │
┌───────────▼─────────────────────▼──────────────────────┐
│            Underlying System Package Managers          │
│   brew • cask • mise • uv • pnpm • npm • mas • gcloud  │
└────────────────────────────────────────────────────────┘
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

## 🛠️ CLI Options

| Flag | Default | Description |
|---|---|---|
| `--port <int>` | `9099` | Local HTTP server port (uses random free port if taken) |
| `--no-open` | `false` | Do not automatically open the dashboard in default browser |
| `--no-sudo` | `false` | Skip Touch ID / sudo pre-flight authentication |
| `--dev` | `false` | Reverse-proxy to Vite HMR server (`http://127.0.0.1:5173`) |

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
