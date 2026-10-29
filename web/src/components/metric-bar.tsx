import type { FC } from "react";
import { ShieldAlert, ShieldCheck, AlertTriangle, Pin, Package as PackageIcon } from "lucide-react";
import type { ScanSummary, FilterType } from "../lib/types";

interface MetricBarProps {
  summary: ScanSummary;
  selectedCount: number;
  activeFilter: FilterType;
  isScanning: boolean;
  onFilterChange: (filter: FilterType) => void;
}

export const MetricBar: FC<MetricBarProps> = ({
  summary,
  selectedCount,
  activeFilter,
  isScanning,
  onFilterChange,
}) => {
  const handleCardClick = (target: FilterType) => {
    if (activeFilter === target && target !== "all") {
      onFilterChange("all");
    } else {
      onFilterChange(target);
    }
  };

  const isInitialScanning = isScanning && summary.total_packages === 0;

  return (
    <div className="grid grid-cols-2 lg:grid-cols-5 gap-2.5">
      {/* 1. Critical & CVEs */}
      <button
        type="button"
        onClick={() => handleCardClick("security")}
        className={`text-left rounded-lg p-3 flex flex-col justify-between border transition-all cursor-pointer ${
          activeFilter === "security"
            ? "bg-rose-950/50 border-rose-500"
            : summary.security_fix_count > 0
            ? "bg-rose-950/20 border-rose-900/60 hover:border-rose-700 hover:bg-rose-950/30"
            : "bg-zinc-900/50 border-zinc-800/80 hover:border-zinc-700 hover:bg-zinc-900/80"
        }`}
      >
        <div className="flex items-center justify-between w-full">
          <span
            className={`text-[11px] font-mono uppercase tracking-wider flex items-center gap-1.5 ${
              summary.security_fix_count > 0 ? "text-rose-400 font-semibold" : "text-zinc-400"
            }`}
          >
            <ShieldAlert className={`w-3.5 h-3.5 ${summary.security_fix_count > 0 ? "text-rose-400" : "text-zinc-500"}`} />
            Critical & CVEs
          </span>
          {activeFilter === "security" && (
            <span className="text-[10px] font-mono font-medium text-rose-400">ACTIVE</span>
          )}
        </div>
        <div className="flex items-baseline justify-between mt-2.5">
          {isInitialScanning ? (
            <span className="text-2xl font-mono font-semibold text-zinc-600 animate-pulse">--</span>
          ) : (
            <span
              className={`text-2xl font-mono font-semibold tabular-nums ${
                summary.security_fix_count > 0 ? "text-rose-300 font-bold" : "text-zinc-400"
              }`}
            >
              {summary.security_fix_count}
            </span>
          )}
          <span className="text-[11px] font-mono text-zinc-400">
            {isInitialScanning ? "Scanning..." : summary.security_fix_count > 0 ? "Urgent fix" : "Clean"}
          </span>
        </div>
      </button>

      {/* 2. Safe Updates */}
      <button
        type="button"
        onClick={() => handleCardClick("safe")}
        className={`text-left rounded-lg p-3 flex flex-col justify-between border transition-all cursor-pointer ${
          activeFilter === "safe"
            ? "bg-emerald-950/40 border-emerald-500"
            : "bg-zinc-900/50 border-zinc-800/80 hover:border-zinc-700 hover:bg-zinc-900/80"
        }`}
      >
        <div className="flex items-center justify-between w-full">
          <span className="text-[11px] font-mono uppercase tracking-wider text-emerald-400/90 flex items-center gap-1.5">
            <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
            Safe Updates
          </span>
          {activeFilter === "safe" && (
            <span className="text-[10px] font-mono font-medium text-emerald-400">ACTIVE</span>
          )}
        </div>
        <div className="flex items-baseline justify-between mt-2.5">
          {isInitialScanning ? (
            <span className="text-2xl font-mono font-semibold text-zinc-600 animate-pulse">--</span>
          ) : (
            <span className="text-2xl font-mono font-semibold tabular-nums text-emerald-400">
              {summary.safe_count}
            </span>
          )}
          <span className="text-[11px] font-mono text-zinc-400">Minor & Patch</span>
        </div>
      </button>

      {/* 3. Major Breaking */}
      <button
        type="button"
        onClick={() => handleCardClick("major")}
        className={`text-left rounded-lg p-3 flex flex-col justify-between border transition-all cursor-pointer ${
          activeFilter === "major"
            ? "bg-amber-950/40 border-amber-500"
            : "bg-zinc-900/50 border-zinc-800/80 hover:border-zinc-700 hover:bg-zinc-900/80"
        }`}
      >
        <div className="flex items-center justify-between w-full">
          <span className="text-[11px] font-mono uppercase tracking-wider text-amber-400/90 flex items-center gap-1.5">
            <AlertTriangle className="w-3.5 h-3.5 text-amber-400" />
            Major Breaking
          </span>
          {activeFilter === "major" && (
            <span className="text-[10px] font-mono font-medium text-amber-400">ACTIVE</span>
          )}
        </div>
        <div className="flex items-baseline justify-between mt-2.5">
          {isInitialScanning ? (
            <span className="text-2xl font-mono font-semibold text-zinc-600 animate-pulse">--</span>
          ) : (
            <span className="text-2xl font-mono font-semibold tabular-nums text-amber-300">
              {summary.major_count}
            </span>
          )}
          <span className="text-[11px] font-mono text-zinc-400">Review diff</span>
        </div>
      </button>

      {/* 4. All Updates */}
      <button
        type="button"
        onClick={() => handleCardClick("all")}
        className={`text-left rounded-lg p-3 flex flex-col justify-between border transition-all cursor-pointer ${
          activeFilter === "all"
            ? "bg-zinc-900 border-zinc-500"
            : "bg-zinc-900/50 border-zinc-800/80 hover:border-zinc-700 hover:bg-zinc-900/80"
        }`}
      >
        <div className="flex items-center justify-between w-full">
          <span className="text-[11px] font-mono uppercase tracking-wider text-zinc-400 flex items-center gap-1.5">
            <PackageIcon className="w-3.5 h-3.5 text-zinc-400" />
            All Updates
          </span>
          {activeFilter === "all" && (
            <span className="text-[10px] font-mono font-medium text-zinc-300">ACTIVE</span>
          )}
        </div>
        <div className="flex items-baseline justify-between mt-2.5">
          {isInitialScanning ? (
            <span className="text-2xl font-mono font-semibold text-zinc-600 animate-pulse">--</span>
          ) : (
            <span className="text-2xl font-mono font-semibold tabular-nums text-zinc-100">
              {summary.total_packages}
            </span>
          )}
          <span className="text-[11px] font-mono text-zinc-400">
            {isInitialScanning ? "Scanning..." : `${selectedCount} selected`}
          </span>
        </div>
      </button>

      {/* 5. Pinned / Held */}
      <button
        type="button"
        onClick={() => handleCardClick("pinned")}
        className={`text-left rounded-lg p-3 flex flex-col justify-between border transition-all cursor-pointer ${
          activeFilter === "pinned"
            ? "bg-zinc-900 border-zinc-500"
            : "bg-zinc-900/50 border-zinc-800/80 hover:border-zinc-700 hover:bg-zinc-900/80"
        }`}
      >
        <div className="flex items-center justify-between w-full">
          <span className="text-[11px] font-mono uppercase tracking-wider text-zinc-400 flex items-center gap-1.5">
            <Pin className="w-3.5 h-3.5 text-zinc-400" />
            Pinned / Held
          </span>
          {activeFilter === "pinned" && (
            <span className="text-[10px] font-mono font-medium text-zinc-300">ACTIVE</span>
          )}
        </div>
        <div className="flex items-baseline justify-between mt-2.5">
          {isInitialScanning ? (
            <span className="text-2xl font-mono font-semibold text-zinc-600 animate-pulse">--</span>
          ) : (
            <span className="text-2xl font-mono font-semibold tabular-nums text-zinc-300">
              {summary.pinned_count}
            </span>
          )}
          <span className="text-[11px] font-mono text-zinc-400">Excluded</span>
        </div>
      </button>
    </div>
  );
};
