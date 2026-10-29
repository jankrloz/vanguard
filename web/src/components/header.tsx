import React from "react";
import { ShieldCheck, ShieldAlert, RefreshCw, Trash2, ArrowUpCircle } from "lucide-react";
import type { SystemStatus } from "../lib/types";

interface HeaderProps {
  status: SystemStatus | null;
  selectedCount: number;
  isScanning: boolean;
  isUpgrading: boolean;
  onScan: () => void;
  onUpgrade: () => void;
  onClean: () => void;
}

export const Header: React.FC<HeaderProps> = ({
  status,
  selectedCount,
  isScanning,
  isUpgrading,
  onScan,
  onUpgrade,
  onClean,
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
            </div>
            <p className="text-xs text-zinc-400">System Package & Dependency Controller</p>
          </div>
        </div>

        {/* Center: Sudo Status */}
        <div className="hidden md:flex items-center gap-2 text-xs font-mono bg-zinc-900/90 border border-zinc-800 px-3 py-1.5 rounded-full">
          {status?.sudo_active ? (
            <>
              <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
              <span className="text-zinc-300">Touch ID Sudo:</span>
              <span className="text-emerald-400 font-medium">Primed & Active</span>
            </>
          ) : (
            <>
              <ShieldAlert className="w-3.5 h-3.5 text-amber-400" />
              <span className="text-zinc-400">Sudo:</span>
              <span className="text-amber-400">Standard</span>
            </>
          )}
        </div>

        {/* Actions */}
        <div className="flex items-center gap-2.5">
          <button
            onClick={onClean}
            disabled={isUpgrading}
            title="Clean package caches, old versions, and logs"
            className="flex items-center gap-1.5 text-xs text-zinc-400 hover:text-zinc-200 bg-zinc-900 hover:bg-zinc-800/80 border border-zinc-800 px-3 py-1.5 rounded-md transition-colors disabled:opacity-50"
          >
            <Trash2 className="w-3.5 h-3.5" />
            <span>Clean</span>
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
