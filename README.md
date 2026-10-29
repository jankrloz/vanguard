# Vanguard (`vg` / `up`)

<p align="center">
  <img src="assets/vanguard-social-preview.svg" alt="Vanguard — Vulnerability Shield & Package Orchestrator" width="100%" />
</p>

<p align="center">
  <a href="https://github.com/jankrloz/vanguard/releases"><img src="https://img.shields.io/github/v/release/jankrloz/vanguard?style=flat-square&color=00F0FF&label=Release" alt="Release" /></a>
  <a href="https://github.com/jankrloz/vanguard/blob/main/LICENSE"><img src="https://img.shields.io/badge/License-MIT-00FF9D?style=flat-square" alt="License" /></a>
  <img src="https://img.shields.io/badge/Architecture-Clean%20%2F%20Hexagonal-FFB800?style=flat-square" alt="Architecture" />
  <img src="https://img.shields.io/badge/Security-OSV.dev%20%26%20CVE-FF2E93?style=flat-square" alt="Security" />
  <img src="https://img.shields.io/badge/CLI-Lip%20Gloss%20%26%20Charm-00F0FF?style=flat-square" alt="CLI Stack" />
</p>

> **Vanguard is an interactive vulnerability shield and package upgrade orchestrator for macOS and Linux.**  
> Built with Go, Lip Gloss, React 19, and Tailwind CSS v4. Prioritizes urgent CVE/zero-day patches across 60+ package ecosystems, enforces strict SemVer breaking-change guards, protects Unix ownership boundaries, and streams real-time execution logs.

---

## ⚡ Key Capabilities

- **🚨 Fine-Grained CVE & Zero-Day Intelligence:** Direct batch querying against **OSV.dev** (Open Source Vulnerabilities) triaging NVD, GitHub Advisories (GHSA), RustSec, and PyPA databases with CVSS 3.1 score weighting.
- **🚦 SemVer Breaking-Change Guard:**
  - **Critical & CVE Fixes:** Pre-selected with pulsing threat badges and mitigation indicators.
  - **Safe Minor & Patch Updates:** Auto-checked for safe 1-click execution.
  - **Major Releases:** Safely unselected by default with breaking-change warnings.
- **🛡️ Strict Unix Ownership & Privilege Isolation:**
  - Prevents root pollution: User-level ecosystems (`brew`, `npm`, `cargo`, `pipx`, `mise`, `~/.rbenv`) are strictly blocked from running under `sudo`.
  - Non-interactive Touch ID & macOS Native Dialogs: Primes credentials via `/dev/tty` or macOS native `osascript` authorization (`"Vanguard is trying to execute privileged package updates."`) on demand only when system directories (like `/Library/Ruby/Gems`) require escalation.
- **🎨 Brandkit & CLI Charm Stack:** Powered by **Charm Lip Gloss** and **Bubbles** with dynamic progress spinners, rounded table cards, and high-visibility risk badges in both interactive TUI and scriptable headless modes (`-no-web`).
- **🗂️ In-Place Queue Filter:** Zero modals — reviewing your upgrade queue operates directly within the main table via the `Queued (N)` filter tab with contextual actions (`Clear Queue`, `Upgrade Queued`).
- **📦 Single Self-Contained Binary:** Go `embed.FS` packs all compiled HTML/JS/CSS assets inside an 11MB standalone binary with zero external runtime dependencies.

---

## 🎨 Brand Identity & Design Tokens

Vanguard adheres to a **stealth cybersecurity aesthetic** engineered for high signal-to-noise ratio in terminal and browser environments:

| Token | Hex | Role | Visual Usage |
|---|---|---|---|
| **Vanguard Cyan** | `#00F0FF` | Primary Identity | Prism shield emblem, active filters, CLI focus, spinners |
| **Threat Rose** | `#FF2E93` | Critical / Zero-Day | CVSS 9.0+ CVEs, unmitigated exploits, urgent action triggers |
| **High Orange** | `#FF8A00` | High Severity | CVSS 7.0–8.9 security advisories |
| **Warning Amber** | `#FFB800` | Major Breaking | SemVer major version jumps, breaking API changes |
| **Safe Emerald** | `#00FF9D` | Safe Updates | Verified minor & patch releases, clean security checks |
| **Obsidian Canvas** | `#09090B` | Deep Background | Quiet chrome UI, terminal cards, low visual fatigue |

---

## 🛡️ Vulnerability Severity Matrix

Not all CVEs carry the same risk. Vanguard surfaces upstream CVSS base scores and categorizes advisories into actionable risk tiers:

```
┌──────────┬───────────┬────────────────────────────────────────────────────────┐
│ Severity │ CVSS Base │ Action & Triage Policy                                 │
├──────────┼───────────┼────────────────────────────────────────────────────────┤
│ CRITICAL │ 9.0–10.0  │ Remotely exploitable without authentication (RCE).     │
│          │           │ Auto-selected, urgent priority #1.                     │
├──────────┼───────────┼────────────────────────────────────────────────────────┤
│ HIGH     │ 7.0–8.9   │ Privilege escalation, DoS, auth bypass.               │
│          │           │ Auto-selected with high-visibility warning badge.       │
├──────────┼───────────┼────────────────────────────────────────────────────────┤
│ MEDIUM   │ 4.0–6.9   │ Requires specific conditions, local access, or CSRF.   │
│          │           │ Tagged with advisory detail link for developer review. │
├──────────┼───────────┼────────────────────────────────────────────────────────┤
│ LOW      │ 0.1–3.9   │ Minimal impact or theoretical edge cases.              │
│          │           │ Logged in security intelligence drawer.                │
└──────────┴───────────┴────────────────────────────────────────────────────────┘
```

---

## 💡 What is the "CPAN" Package Manager?

When scanning macOS systems, developers often see **1,600+ or 2,100+ available updates** attributed to `cpan`:

- **What is CPAN?** CPAN is the *Comprehensive Perl Archive Network* — Perl's official package repository (analogous to npm for Node or PyPI for Python).
- **Why are thousands of updates listed on macOS?**  
  macOS ships with a legacy vendor Perl installation (`/usr/bin/perl`, Perl 5.34). This system installation bundles hundreds of "dual-lived" core Perl modules in `/System/Library/Perl`. When package scanners query CPAN, CPAN reports every newer standalone release published upstream since Apple froze the macOS Perl vendor bundle.
- **Why upgrading them is dangerous:**  
  Updating system Perl modules requires overriding macOS System Integrity Protection (SIP) or `/Library/Perl`, which risks breaking internal macOS operating system scripts.
- **Vanguard's Solution:**  
  Vanguard **excludes `cpan` by default** (`--no-cpan` and `ignored_managers = ["cpan"]`), eliminating 2,100+ irrelevant items and focusing your scan on real developer toolchains (`brew`, `npm`, `pip`, `cargo`, `gem`, `pnpm`, `uv`).

---

## 🏗️ Architecture

```
┌───────────────────────────────────────────────────────────────────────────┐
│                    Layer 3: Vanguard (Shield & UX)                        │
│   OSV.dev CVE Triage • SemVer Safety Guard • Privilege Keepalive (TID)    │
│    Lip Gloss / Charm CLI • Ephemeral Web Dashboard • SSE Real-Time Logs   │
└─────────────────────────────────────▲─────────────────────────────────────┘
                                      │
┌─────────────────────────────────────┴─────────────────────────────────────┐
│                 Layer 2: MPM (Meta Package Manager)                       │
│    Granular Multi-Ecosystem Abstraction & Inspection (60+ Managers)       │
└─────────────────────────────────────▲─────────────────────────────────────┘
                                      │ (delegation & sinkhole fallback)
┌─────────────────────────────────────┴─────────────────────────────────────┐
│               Layer 1: Topgrade & Native Package Managers                 │
│      Topgrade (System sweep, Rustup, Vim plugins, OS firmware)            │
│      Homebrew (Formulae & Casks) • mise • uv • pnpm • npm • mas • gcloud   │
└───────────────────────────────────────────────────────────────────────────┘
```

---

## 🚀 Installation & Usage

### Build and Install via Taskfile
```bash
git clone https://github.com/jankrloz/vanguard.git ~/dev/personal/vanguard
cd ~/dev/personal/vanguard

task build
task install
```

### CLI Commands (`vg` / `up`)

```bash
# Launch interactive web dashboard (default)
vg
vg dashboard

# Headless check with Lip Gloss styled output (exits 1 if critical CVEs found)
vg check
vg check --security-only
vg check --manager cask
vg check --limit 20

# Non-interactive upgrade execution directly in terminal
vg apply --yes
vg apply --security-only -y
vg apply --manager npm -y
vg apply --all -y

# View persistent upgrade audit history
vg history --limit 30

# System-wide maintenance sweep via Topgrade (Git repos, toolchains, caches)
vg sweep   # or: vg topgrade
```

### Dashboard Flags

| Flag | Default | Description |
|---|---|---|
| `-port <int>` | `9099` | Local HTTP server port (uses random free port if taken) |
| `-no-web` | `false` | Run headless daemon without opening browser (`-no-open` supported as alias) |
| `-no-sudo` | `false` | Skip privilege pre-flight priming |
| `-dev` | `false` | Reverse-proxy to Vite HMR server (`http://127.0.0.1:5173`) |

---

## ⚙️ Configuration & Audit Trail

- **Configuration:** `~/.config/vanguard/config.toml` (manages pinned packages and ignored managers like `cpan`).
- **Audit History:** `~/.local/state/vanguard/history.jsonl` (persists timestamp, package ID, manager, version diff, and execution outcome).

---

## 📜 License

MIT © Juan Carlos Navarrete Gordillo
