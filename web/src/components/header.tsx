import type { FC } from "react";
import { ShieldCheck, ShieldAlert, RefreshCw, Trash2, ArrowUpCircle, Flame, Zap, History } from "lucide-react";
import type { SystemStatus } from "../lib/types";

interface HeaderProps {
  status: SystemStatus | null;
  selectedCount: number;
  securityCount: number;
  isScanning: boolean;
  isUpgrading: boolean;
  onScan: () => void;
  onUpgrade: () => void;
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
  onClean,
  onTopgrade,
  onOpenHistory,
}) => {
  return (
    <header className="border-b border-zinc-800/80 bg-zinc-950/70 backdrop-blur px-6 py-4 sticky top-0 z-30">
      <div className="max-w-7xl mx-auto flex items-center justify-between gap-4">
        {/* Brand */}
        <div className="flex items-center gap-3">
          <div className="w-8 h-8 rounded bg-zinc-900 border border-zinc-700/60 flex items-center justify-center font-mono font-bold text-sm text-zinc-100 shadow-inner">
            ▲
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="text-base font-semibold tracking-tight text-zinc-100">VANGUARD</h1>
              <span className="text-[11px] font-mono uppercase tracking-wider text-zinc-400 bg-zinc-800/80 border border-zinc-700/50 px-1.5 py-0.5 rounded">
                v1.0
              </span>
              {securityCount > 0 && (
                <span className="flex items-center gap-1 text-[11px] font-mono font-semibold text-rose-300 bg-rose-950/90 border border-rose-800 px-2 py-0.5 rounded animate-pulse">
                  <Flame className="w-3 h-3 text-rose-400" />
                  {securityCount} CVE {securityCount === 1 ? "Fix" : "Fixes"} Urgent
                </span>
              )}
            </div>
            <p className="text-xs text-zinc-400">Vulnerability Shield & Package Controller</p>
          </div>
        </div>

        {/* Center: Elevation Status */}
        <div className="hidden md:flex items-center gap-2 text-xs font-mono bg-zinc-900/90 border border-zinc-800 px-3 py-1.5 rounded-full">
          {status?.sudo_active ? (
            <>
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
              <span className="text-zinc-300">Privilege Keepalive:</span>
              <span className="text-emerald-400 font-medium">Active</span>
            </>
          ) : (
            <>
              <ShieldAlert className="w-3.5 h-3.5 text-amber-400" />
              <span className="text-zinc-400">Privileges:</span>
              <span className="text-amber-400">Standard</span>
            </>
          )}
        </div>

        {/* Actions */}
        <div className="flex items-center gap-2">
          <button
            onClick={onOpenHistory}
            title="View upgrade audit history"
            className="flex items-center gap-1.5 text-xs text-zinc-400 hover:text-zinc-200 bg-zinc-900 hover:bg-zinc-800/80 border border-zinc-800 px-2.5 py-1.5 rounded-md transition-colors"
          >
            <History className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">History</span>
          </button>

          <button
            onClick={onTopgrade}
            disabled={isUpgrading}
            title="Run deep system-wide sweep via Topgrade"
            className="flex items-center gap-1.5 text-xs text-amber-300 hover:text-amber-100 bg-amber-950/40 hover:bg-amber-900/50 border border-amber-800/60 px-2.5 py-1.5 rounded-md transition-colors disabled:opacity-50"
          >
            <Zap className="w-3.5 h-3.5 text-amber-400" />
            <span className="hidden sm:inline">Sweep</span>
          </button>

          <button
            onClick={onClean}
            disabled={isUpgrading}
            title="Clean package caches, old versions, and logs"
            className="flex items-center gap-1.5 text-xs text-zinc-400 hover:text-zinc-200 bg-zinc-900 hover:bg-zinc-800/80 border border-zinc-800 px-2.5 py-1.5 rounded-md transition-colors disabled:opacity-50"
          >
            <Trash2 className="w-3.5 h-3.5" />
            <span className="hidden sm:inline">Clean</span>
          </button>

          <button
            onClick={onScan}
            disabled={isScanning || isUpgrading}
            className="flex items-center gap-1.5 text-xs text-zinc-300 hover:text-zinc-100 bg-zinc-900 hover:bg-zinc-800 border border-zinc-700/70 px-3 py-1.5 rounded-md transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${isScanning ? "animate-spin text-zinc-400" : ""}`} />
            <span>{isScanning ? "Scanning..." : "Refresh"}</span>
          </button>

          <button
            onClick={onUpgrade}
            disabled={selectedCount === 0 || isUpgrading}
            className={`flex items-center gap-2 text-xs font-medium px-4 py-1.5 rounded-md transition-all shadow-sm ${
              selectedCount > 0 && !isUpgrading
                ? "bg-zinc-100 hover:bg-white text-zinc-950 font-semibold cursor-pointer"
                : "bg-zinc-800 text-zinc-500 cursor-not-allowed border border-zinc-800"
            }`}
          >
            <ArrowUpCircle className="w-4 h-4" />
            <span>{isUpgrading ? "Upgrading..." : `Upgrade (${selectedCount})`}</span>
          </button>
        </div>
      </div>
    </header>
  );
};
