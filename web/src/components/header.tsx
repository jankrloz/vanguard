import type { FC } from "react";
import { ShieldCheck, ShieldAlert, RefreshCw, Trash2, ArrowUpCircle, Zap, History, Loader2, ListFilter } from "lucide-react";
import type { SystemStatus } from "../lib/types";
import { BrandDelta } from "./brand-logo";

interface HeaderProps {
  status: SystemStatus | null;
  selectedCount: number;
  securityCount: number;
  isScanning: boolean;
  isUpgrading: boolean;
  onScan: () => void;
  onUpgrade: () => void;
  onOpenReview?: () => void;
  onClean: () => void;
  onTopgrade: () => void;
  onOpenHistory: () => void;
}

export const Header: FC<HeaderProps> = ({
  status,
  selectedCount,
  securityCount,
  isScanning,
  isUpgrading,
  onScan,
  onUpgrade,
  onOpenReview,
  onClean,
  onTopgrade,
  onOpenHistory,
}) => {
  return (
    <header className="border-b border-zinc-800/80 bg-zinc-950/80 backdrop-blur-md px-4 sm:px-6 py-3.5 sticky top-0 z-30">
      <div className="max-w-7xl mx-auto flex flex-wrap items-center justify-between gap-3">
        {/* Brand */}
        <div className="flex items-center gap-3 min-w-0">
          <BrandDelta size={34} className="w-8 h-8 shrink-0" />
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h1 className="text-sm font-semibold tracking-tight text-zinc-100 font-mono">VANGUARD</h1>
              <span className="text-[11px] font-mono uppercase tracking-wider text-zinc-300 bg-zinc-800/80 border border-zinc-700/50 px-1.5 py-0.5 rounded">
                {status?.version ? `v${status.version}` : "v0.2.0"}
              </span>
            </div>
            <p className="hidden sm:block text-xs text-zinc-400 truncate">Vulnerability Shield &amp; Package Controller</p>
          </div>
        </div>

        {/* Center: Elevation & Platform Status */}
        <div className="hidden lg:flex items-center gap-2.5 text-[11px] font-mono bg-zinc-900/80 border border-zinc-800/90 px-3 py-1 rounded-full">
          {status?.sudo_active ? (
            <>
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
              <span className="text-zinc-400">Elevation Keepalive:</span>
              <span className="text-emerald-400 font-medium">Active</span>
            </>
          ) : (
            <>
              <ShieldAlert className="w-3.5 h-3.5 text-amber-400" />
              <span className="text-zinc-400">Elevation:</span>
              <span className="text-amber-400">User mode</span>
            </>
          )}
          <span className="text-zinc-700">|</span>
          <span className="text-zinc-400">{status?.platform || "darwin"}</span>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-3">
          {/* Secondary Utilities */}
          <div className="flex items-center bg-zinc-900/80 border border-zinc-800 rounded-lg p-0.5">
            <button
              type="button"
              onClick={onOpenHistory}
              disabled={isUpgrading}
              title="Audit history of upgraded packages"
              className="flex items-center gap-1.5 text-xs text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/80 px-2.5 py-1.5 rounded-md transition-colors disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
            >
              <History className="w-3.5 h-3.5 text-zinc-400" />
              <span className="hidden sm:inline">History</span>
            </button>

            <button
              type="button"
              onClick={onTopgrade}
              disabled={isUpgrading || isScanning}
              title="Run deep system-wide maintenance sweep via Topgrade (Git repos, language toolchains, VS Code extensions, firmware, and macOS system caches)"
              className="flex items-center gap-1.5 text-xs text-zinc-400 hover:text-amber-300 hover:bg-zinc-800/80 px-2.5 py-1.5 rounded-md transition-colors disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
            >
              <Zap className="w-3.5 h-3.5 text-amber-400" />
              <span className="hidden sm:inline">Topgrade Sweep</span>
            </button>

            <button
              type="button"
              onClick={onClean}
              disabled={isUpgrading || isScanning}
              title="Clean package caches, old versions, and logs"
              className="flex items-center gap-1.5 text-xs text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/80 px-2.5 py-1.5 rounded-md transition-colors disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
            >
              <Trash2 className="w-3.5 h-3.5 text-zinc-400" />
              <span className="hidden sm:inline">Clean</span>
            </button>
          </div>

          {/* Primary Action Controls */}
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={onScan}
              disabled={isScanning || isUpgrading}
              title="Rescan system for outdated packages"
              className="flex items-center gap-1.5 text-xs text-zinc-300 hover:text-zinc-100 bg-zinc-900 hover:bg-zinc-800 border border-zinc-700/80 px-3 py-1.5 rounded-lg transition-colors disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${isScanning ? "animate-spin text-zinc-400" : ""}`} />
              <span>{isScanning ? "Scanning..." : "Rescan"}</span>
            </button>

            {selectedCount > 0 && onOpenReview && (
              <button
                type="button"
                onClick={onOpenReview}
                disabled={isUpgrading}
                title="Filter table to queued packages"
                className="flex items-center gap-1.5 text-xs font-mono text-zinc-300 hover:text-zinc-100 bg-zinc-900 hover:bg-zinc-800 border border-zinc-700/80 px-3 py-1.5 rounded-lg transition-colors disabled:opacity-40 disabled:cursor-not-allowed cursor-pointer"
              >
                <ListFilter className="w-3.5 h-3.5 text-cyan-400" />
                <span>Queue ({selectedCount})</span>
              </button>
            )}

            <button
              type="button"
              onClick={onUpgrade}
              disabled={selectedCount === 0 || isUpgrading || isScanning}
              title={selectedCount > 0 ? "Upgrade selected packages (⌘↵)" : "Select packages to upgrade"}
              className={`flex items-center gap-2 text-xs font-semibold px-4 py-1.5 rounded-lg transition-all shadow-sm ${
                selectedCount > 0 && !isUpgrading && !isScanning
                  ? "bg-zinc-100 hover:bg-white text-zinc-950 cursor-pointer hover:shadow-zinc-500/10"
                  : "bg-zinc-800/80 text-zinc-500 cursor-not-allowed border border-zinc-800"
              }`}
            >
              {isUpgrading ? (
                <Loader2 className="w-4 h-4 animate-spin text-zinc-400" />
              ) : (
                <ArrowUpCircle className="w-4 h-4" />
              )}
              <span>
                {isUpgrading
                  ? "Upgrading..."
                  : securityCount > 0 && selectedCount === securityCount
                  ? `Upgrade (${selectedCount} CVEs)`
                  : `Upgrade (${selectedCount})`}
              </span>
              {selectedCount > 0 && !isUpgrading && !isScanning && (
                <kbd className="hidden sm:inline text-[10px] font-mono bg-zinc-300 text-zinc-800 px-1 py-0.2 rounded font-normal">
                  ⌘↵
                </kbd>
              )}
            </button>
          </div>
        </div>
      </div>
    </header>
  );
};
