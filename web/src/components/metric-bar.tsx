import React from "react";
import { ShieldAlert, Flame } from "lucide-react";
import type { ScanSummary } from "../lib/types";

interface MetricBarProps {
  summary: ScanSummary;
  selectedCount: number;
}

export const MetricBar: React.FC<MetricBarProps> = ({ summary, selectedCount }) => {
  return (
    <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
      {/* Total */}
      <div className="bg-zinc-900/60 border border-zinc-800/80 rounded-lg p-3.5 flex flex-col justify-between">
        <span className="text-[11px] font-mono uppercase tracking-wider text-zinc-400">
          Pending Updates
        </span>
        <div className="flex items-baseline justify-between mt-2">
          <span className="text-2xl font-mono font-semibold text-zinc-100">
            {summary.total_packages}
          </span>
          <span className="text-xs font-mono text-zinc-400">
            {selectedCount} selected
          </span>
        </div>
      </div>

      {/* Security CVEs */}
      <div
        className={`rounded-lg p-3.5 flex flex-col justify-between border transition-all ${
          summary.security_fix_count > 0
            ? "bg-rose-950/40 border-rose-800/80 shadow-[0_0_15px_rgba(244,63,94,0.1)]"
            : "bg-zinc-900/60 border-zinc-800/80"
        }`}
      >
        <div className="flex items-center justify-between">
          <span
            className={`text-[11px] font-mono uppercase tracking-wider ${
              summary.security_fix_count > 0 ? "text-rose-400 font-bold flex items-center gap-1" : "text-zinc-400"
            }`}
          >
            {summary.security_fix_count > 0 && <Flame className="w-3 h-3 text-rose-400 animate-bounce" />}
            CVE / Zero-Day Fixes
          </span>
          <span
            className={`w-1.5 h-1.5 rounded-full ${
              summary.security_fix_count > 0 ? "bg-rose-500 animate-ping" : "bg-zinc-600"
            }`}
          ></span>
        </div>
        <div className="flex items-baseline justify-between mt-2">
          <span
            className={`text-2xl font-mono font-semibold ${
              summary.security_fix_count > 0 ? "text-rose-400 font-extrabold" : "text-zinc-400"
            }`}
          >
            {summary.security_fix_count}
          </span>
          <span className="text-[11px] text-zinc-400">
            {summary.security_fix_count > 0 ? "Top priority" : "Clean"}
          </span>
        </div>
      </div>

      {/* Safe */}
      <div className="bg-zinc-900/60 border border-zinc-800/80 rounded-lg p-3.5 flex flex-col justify-between">
        <div className="flex items-center justify-between">
          <span className="text-[11px] font-mono uppercase tracking-wider text-emerald-400/90">
            Safe (Minor & Patch)
          </span>
          <span className="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>
        </div>
        <div className="flex items-baseline justify-between mt-2">
          <span className="text-2xl font-mono font-semibold text-emerald-400">
            {summary.safe_count}
          </span>
          <span className="text-[11px] text-zinc-400">Auto-checked</span>
        </div>
      </div>

      {/* Breaking / Major */}
      <div className="bg-zinc-900/60 border border-zinc-800/80 rounded-lg p-3.5 flex flex-col justify-between">
        <div className="flex items-center justify-between">
          <span className="text-[11px] font-mono uppercase tracking-wider text-rose-400/90">
            Breaking (Major)
          </span>
          <span className="w-1.5 h-1.5 rounded-full bg-rose-500"></span>
        </div>
        <div className="flex items-baseline justify-between mt-2">
          <span className="text-2xl font-mono font-semibold text-rose-400">
            {summary.major_count}
          </span>
          <span className="text-[11px] text-zinc-400">Unchecked</span>
        </div>
      </div>

      {/* Pinned */}
      <div className="bg-zinc-900/60 border border-zinc-800/80 rounded-lg p-3.5 flex flex-col justify-between">
        <span className="text-[11px] font-mono uppercase tracking-wider text-zinc-400">
          Pinned / Held
        </span>
        <div className="flex items-baseline justify-between mt-2">
          <span className="text-2xl font-mono font-semibold text-zinc-300">
            {summary.pinned_count}
          </span>
          <span className="text-[11px] text-zinc-400">Locked</span>
        </div>
      </div>
    </div>
  );
};
