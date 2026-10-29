import React from "react";
import type { ScanSummary } from "../lib/types";

interface MetricBarProps {
  summary: ScanSummary;
  selectedCount: number;
}

export const MetricBar: React.FC<MetricBarProps> = ({ summary, selectedCount }) => {
  return (
    <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
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
          <span className="text-[11px] text-zinc-400">Unchecked by default</span>
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
