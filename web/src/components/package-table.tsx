import React, { useState, useMemo } from "react";
import { Search, CheckSquare, Square, AlertTriangle, ShieldCheck, Pin } from "lucide-react";
import type { Package } from "../lib/types";

interface PackageTableProps {
  packages: Package[];
  onTogglePackage: (id: string) => void;
  onSelectSafe: () => void;
  onSelectAll: () => void;
  onDeselectAll: () => void;
}

export const PackageTable: React.FC<PackageTableProps> = ({
  packages,
  onTogglePackage,
  onSelectSafe,
  onSelectAll,
  onDeselectAll,
}) => {
  const [search, setSearch] = useState("");
  const [filterType, setFilterType] = useState<"all" | "safe" | "major">("all");
  const [selectedManager, setSelectedManager] = useState<string>("all");

  const managers = useMemo(() => {
    const set = new Set<string>();
    packages.forEach((p) => set.add(p.Manager || p.manager));
    return Array.from(set).sort();
  }, [packages]);

  const filteredPackages = useMemo(() => {
    return packages.filter((p) => {
      const name = p.Name || p.name;
      const mgr = p.Manager || p.manager;
      const changeType = p.ChangeType || p.change_type;
      const safe = p.Safe !== undefined ? p.Safe : p.safe;

      if (search && !name.toLowerCase().includes(search.toLowerCase()) && !mgr.toLowerCase().includes(search.toLowerCase())) {
        return false;
      }

      if (filterType === "safe" && !safe) return false;
      if (filterType === "major" && changeType !== "major") return false;
      if (selectedManager !== "all" && mgr !== selectedManager) return false;

      return true;
    });
  }, [packages, search, filterType, selectedManager]);

  // Group by manager
  const grouped = useMemo(() => {
    const map = new Map<string, Package[]>();
    filteredPackages.forEach((p) => {
      const mgr = p.Manager || p.manager;
      const list = map.get(mgr) || [];
      list.push(p);
      map.set(mgr, list);
    });
    return Array.from(map.entries()).sort(([a], [b]) => a.localeCompare(b));
  }, [filteredPackages]);

  return (
    <div className="space-y-4">
      {/* Controls Bar */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-3 bg-zinc-900/60 border border-zinc-800/80 p-3 rounded-lg">
        {/* Search */}
        <div className="relative flex-1 max-w-sm">
          <Search className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            type="text"
            placeholder="Search package or manager..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-zinc-950/80 border border-zinc-800 rounded-md pl-9 pr-3 py-1.5 text-xs text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-zinc-700"
          />
        </div>

        {/* Filter Pills */}
        <div className="flex items-center gap-1.5 text-xs">
          <button
            onClick={() => setFilterType("all")}
            className={`px-2.5 py-1 rounded transition-colors ${
              filterType === "all"
                ? "bg-zinc-800 text-zinc-100 font-medium"
                : "text-zinc-400 hover:text-zinc-200 hover:bg-zinc-900"
            }`}
          >
            All ({packages.length})
          </button>
          <button
            onClick={() => setFilterType("safe")}
            className={`px-2.5 py-1 rounded transition-colors ${
              filterType === "safe"
                ? "bg-emerald-950/80 text-emerald-400 border border-emerald-800/60 font-medium"
                : "text-zinc-400 hover:text-emerald-400 hover:bg-zinc-900"
            }`}
          >
            Safe Only
          </button>
          <button
            onClick={() => setFilterType("major")}
            className={`px-2.5 py-1 rounded transition-colors ${
              filterType === "major"
                ? "bg-rose-950/80 text-rose-400 border border-rose-800/60 font-medium"
                : "text-zinc-400 hover:text-rose-400 hover:bg-zinc-900"
            }`}
          >
            Major Breaking
          </button>

          {/* Manager filter select */}
          {managers.length > 1 && (
            <select
              value={selectedManager}
              onChange={(e) => setSelectedManager(e.target.value)}
              className="bg-zinc-950 border border-zinc-800 text-xs text-zinc-300 rounded px-2 py-1 ml-2 focus:outline-none"
            >
              <option value="all">All Managers</option>
              {managers.map((m) => (
                <option key={m} value={m}>
                  {m}
                </option>
              ))}
            </select>
          )}
        </div>

        {/* Bulk Selection Actions */}
        <div className="flex items-center gap-2 border-t md:border-t-0 pt-2 md:pt-0 border-zinc-800">
          <button
            onClick={onSelectSafe}
            className="text-[11px] font-mono text-emerald-400/90 hover:text-emerald-300 bg-emerald-950/40 hover:bg-emerald-950/70 border border-emerald-900/60 px-2 py-1 rounded transition-colors"
          >
            Select Safe
          </button>
          <button
            onClick={onSelectAll}
            className="text-[11px] font-mono text-zinc-400 hover:text-zinc-200 bg-zinc-950 border border-zinc-800 px-2 py-1 rounded transition-colors"
          >
            Select All
          </button>
          <button
            onClick={onDeselectAll}
            className="text-[11px] font-mono text-zinc-400 hover:text-zinc-200 bg-zinc-950 border border-zinc-800 px-2 py-1 rounded transition-colors"
          >
            Clear
          </button>
        </div>
      </div>

      {/* Package Groups */}
      {grouped.length === 0 ? (
        <div className="bg-zinc-900/40 border border-zinc-800/80 rounded-lg p-10 text-center">
          <ShieldCheck className="w-8 h-8 text-emerald-500/80 mx-auto mb-2" />
          <h3 className="text-sm font-medium text-zinc-200">System Up to Date</h3>
          <p className="text-xs text-zinc-400 mt-1 max-w-sm mx-auto">
            No outdated packages matching current criteria. Click "Refresh" to trigger a fresh scan.
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {grouped.map(([manager, items]) => (
            <div
              key={manager}
              className="bg-zinc-900/60 border border-zinc-800/80 rounded-lg overflow-hidden"
            >
              {/* Group Header */}
              <div className="bg-zinc-950/70 border-b border-zinc-800/80 px-4 py-2.5 flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="font-mono text-xs uppercase font-semibold text-zinc-200">
                    {manager}
                  </span>
                  <span className="text-[11px] font-mono text-zinc-400 bg-zinc-800/80 px-1.5 py-0.2 rounded">
                    {items.length} {items.length === 1 ? "package" : "packages"}
                  </span>
                </div>
              </div>

              {/* Table */}
              <div className="divide-y divide-zinc-800/50">
                {items.map((pkg) => {
                  const id = pkg.ID || pkg.id;
                  const name = pkg.Name || pkg.name;
                  const cur = pkg.CurrentVersion || pkg.current_version;
                  const lat = pkg.LatestVersion || pkg.latest_version;
                  const changeType = pkg.ChangeType || pkg.change_type;
                  const pinned = pkg.Pinned !== undefined ? pkg.Pinned : pkg.pinned;
                  const selected = pkg.Selected !== undefined ? pkg.Selected : pkg.selected;

                  return (
                    <div
                      key={id}
                      onClick={() => !pinned && onTogglePackage(id)}
                      className={`px-4 py-3 flex items-center justify-between gap-4 transition-colors ${
                        pinned
                          ? "opacity-60 bg-zinc-950/30 cursor-not-allowed"
                          : "hover:bg-zinc-800/40 cursor-pointer"
                      } ${selected ? "bg-zinc-800/20" : ""}`}
                    >
                      {/* Left: Checkbox + Name */}
                      <div className="flex items-center gap-3 min-w-0">
                        <button
                          type="button"
                          disabled={pinned}
                          onClick={(e) => {
                            e.stopPropagation();
                            if (!pinned) onTogglePackage(id);
                          }}
                          className="text-zinc-400 hover:text-zinc-200 transition-colors focus:outline-none"
                        >
                          {selected ? (
                            <CheckSquare className="w-4 h-4 text-zinc-100" />
                          ) : (
                            <Square className="w-4 h-4 text-zinc-600" />
                          )}
                        </button>

                        <div className="min-w-0">
                          <div className="flex items-center gap-2">
                            <span className="font-mono text-xs font-medium text-zinc-200 truncate">
                              {name}
                            </span>
                            {pinned && (
                              <span className="flex items-center gap-1 text-[10px] font-mono bg-zinc-800 text-zinc-400 border border-zinc-700/60 px-1.5 py-0.2 rounded">
                                <Pin className="w-2.5 h-2.5" /> Pinned
                              </span>
                            )}
                          </div>
                        </div>
                      </div>

                      {/* Right: Versions + Badge */}
                      <div className="flex items-center gap-4 shrink-0">
                        {/* Version flow */}
                        <div className="flex items-center gap-1.5 font-mono text-xs">
                          <span className="text-zinc-400">{cur}</span>
                          <span className="text-zinc-600">→</span>
                          <span className="text-zinc-100 font-medium">{lat}</span>
                        </div>

                        {/* Semver Badge */}
                        <div className="w-24 flex justify-end">
                          {changeType === "major" ? (
                            <span className="inline-flex items-center gap-1 text-[11px] font-mono font-medium px-2 py-0.5 rounded bg-rose-950/60 text-rose-400 border border-rose-800/60">
                              <AlertTriangle className="w-3 h-3 text-rose-400" />
                              major
                            </span>
                          ) : changeType === "minor" ? (
                            <span className="inline-flex items-center text-[11px] font-mono font-medium px-2 py-0.5 rounded bg-amber-950/60 text-amber-400 border border-amber-800/60">
                              minor
                            </span>
                          ) : changeType === "patch" ? (
                            <span className="inline-flex items-center text-[11px] font-mono font-medium px-2 py-0.5 rounded bg-emerald-950/60 text-emerald-400 border border-emerald-800/60">
                              patch
                            </span>
                          ) : (
                            <span className="inline-flex items-center text-[11px] font-mono px-2 py-0.5 rounded bg-zinc-800 text-zinc-400 border border-zinc-700">
                              update
                            </span>
                          )}
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
