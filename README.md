# Vanguard (`vg` / `up`)

> **Interactive, biometric-secured package update orchestrator & dashboard for macOS & Linux.**  
> Keep your system and developer toolchains perpetually up-to-date with zero noise, Touch ID biometric priming, and strict SemVer breaking-change guards.

---

## ⚡ The Problem

Modern dev machines accumulate dozens of package managers (`brew`, `cask`, `npm`, `pnpm`, `mise`, `uv`, `pip`, `gcloud`, `mas`).  
Existing solutions fall short in three major ways:
1. **Log noise:** Terminal scripts produce hundreds of lines of intermediate garbage, cleanup logs, and transient warnings.
2. **Blind mass-upgrades:** Tools like `topgrade` or `brew upgrade` apply major breaking updates automatically without interactive triage.
3. **Password friction:** Repeated, blocking terminal sudo password prompts that break biometric Touch ID integrations (`pam_tid.so`).

**Vanguard** solves this by serving an ephemeral, high-contrast local web dashboard backed by a native Go engine that inspects pending updates across 20+ package managers, categorizes them by SemVer impact, primes Touch ID once, and streams upgrade logs in real time.

---

## ✨ Features

- **🛡️ Touch ID Biometric Priming & Keepalive:** Primes root permissions once at launch via macOS Touch ID (`pam_tid.so`) and maintains background credentials (`sudo -n true` every 40s) so terminal prompts never interrupt the workflow.
- **🚦 SemVer Breaking-Change Guard:**
  - **Safe Updates (Patch & Minor):** Auto-checked for 1-click execution.
  - **Major Releases:** Highlighted with warning indicators and unselected by default to prevent unexpected API/CLI breakages.
- **🌐 20+ Package Managers Unified:** Powered by `meta-package-manager` (`mpm`) and native Homebrew (`brew`) JSON inspection. Dispatches targeted commands per package manager.
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
│  │ Touch ID Sudo    │  │ SemVer Classifier│  │ Server│  │
│  │ Priming & Keep   │  │ Patch/Minor/Major│  │ Embed │  │
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

### Prerequisites
- Go 1.24+
- Node.js & pnpm (or Vite+ `vp`)
- Homebrew & `mpm` (`brew install meta-package-manager`)

### Installation via Taskfile
```bash
# Clone the repository
git clone https://github.com/jankrloz/vanguard.git ~/dev/personal/vanguard
cd ~/dev/personal/vanguard

# Build frontend and compile standalone binary
task build

# Install to ~/.local/bin/up (or ~/.local/bin/vg)
task install
```

### Running Vanguard
```bash
# Launch interactive dashboard (prompts Touch ID, launches browser)
vg

# Or with custom port / options
vg --port 9099 --no-open
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
