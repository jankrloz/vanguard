import { useState, useMemo, type FC, type MouseEvent } from "react";
import { Search, CheckSquare, Square, AlertTriangle, ShieldCheck, Pin, Flame, ChevronDown, ChevronRight } from "lucide-react";
import type { Package, Vulnerability } from "../lib/types";

interface PackageTableProps {
  packages: Package[];
  onTogglePackage: (id: string) => void;
  onTogglePin: (id: string) => void;
  onSelectSafe: () => void;
  onSelectSecurity: () => void;
  onSelectAll: () => void;
  onDeselectAll: () => void;
}

export const PackageTable: FC<PackageTableProps> = ({
  packages,
  onTogglePackage,
  onTogglePin,
  onSelectSafe,
  onSelectSecurity,
  onSelectAll,
  onDeselectAll,
}) => {
  const [search, setSearch] = useState("");
  const [filterType, setFilterType] = useState<"all" | "security" | "safe" | "major">("all");
  const [selectedManager, setSelectedManager] = useState<string>("all");
  const [expandedVulns, setExpandedVulns] = useState<Record<string, boolean>>({});
  const [collapsedManagers, setCollapsedManagers] = useState<Record<string, boolean>>({});

  const managers = useMemo(() => {
    const set = new Set<string>();
    packages.forEach((p) => set.add(p.manager));
    return Array.from(set).sort();
  }, [packages]);

  const managerCounts = useMemo(() => {
    const map = new Map<string, number>();
    packages.forEach((p) => {
      map.set(p.manager, (map.get(p.manager) || 0) + 1);
    });
    return map;
  }, [packages]);

  const securityPackagesCount = useMemo(() => {
    return packages.filter((p) => p.has_vulnerability).length;
  }, [packages]);

  const filteredPackages = useMemo(() => {
    return packages.filter((p) => {
      const name = p.name;
      const mgr = p.manager;
      const changeType = p.change_type;
      const safe = p.safe;
      const hasVuln = p.has_vulnerability;

      if (search && !name.toLowerCase().includes(search.toLowerCase()) && !mgr.toLowerCase().includes(search.toLowerCase())) {
        return false;
      }

      if (filterType === "security" && !hasVuln) return false;
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
      const mgr = p.manager;
      const list = map.get(mgr) || [];
      list.push(p);
      map.set(mgr, list);
    });
    return Array.from(map.entries()).sort(([a], [b]) => a.localeCompare(b));
  }, [filteredPackages]);

  const toggleExpand = (id: string, e: MouseEvent) => {
    e.stopPropagation();
    setExpandedVulns((prev) => ({ ...prev, [id]: !prev[id] }));
  };

  return (
    <div className="space-y-4">
      {/* Controls Bar */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-3 bg-zinc-900/60 border border-zinc-800/80 p-3 rounded-lg">
        {/* Search */}
        <div className="relative flex-1 max-w-sm">
          <Search className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            type="text"
            placeholder="Search package, manager, or CVE..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-zinc-950/80 border border-zinc-800 rounded-md pl-9 pr-3 py-1.5 text-xs text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-zinc-700"
          />
        </div>

        {/* Filter Pills */}
        <div className="flex items-center gap-1.5 text-xs flex-wrap">
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

          {securityPackagesCount > 0 && (
            <button
              onClick={() => setFilterType("security")}
              className={`px-2.5 py-1 rounded transition-colors flex items-center gap-1 ${
                filterType === "security"
                  ? "bg-rose-950 text-rose-300 border border-rose-700 font-bold"
                  : "text-rose-400 hover:text-rose-300 hover:bg-rose-950/40"
              }`}
            >
              <Flame className="w-3 h-3 text-rose-400" />
              CVE Fixes ({securityPackagesCount})
            </button>
          )}

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
              className="bg-zinc-950 border border-zinc-800 text-xs text-zinc-300 rounded px-2 py-1 ml-1 focus:outline-none"
            >
              <option value="all">All Managers ({packages.length})</option>
              {managers.map((m) => (
                <option key={m} value={m}>
                  {m} ({managerCounts.get(m) || 0})
                </option>
              ))}
            </select>
          )}
        </div>

        {/* Bulk Selection Actions */}
        <div className="flex items-center gap-2 border-t md:border-t-0 pt-2 md:pt-0 border-zinc-800">
          {securityPackagesCount > 0 && (
            <button
              onClick={onSelectSecurity}
              className="text-[11px] font-mono text-rose-300 hover:text-rose-100 bg-rose-950/80 hover:bg-rose-900 border border-rose-700 px-2 py-1 rounded transition-colors font-semibold"
            >
              Select CVEs
            </button>
          )}
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
            All
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
          <h3 className="text-sm font-medium text-zinc-200">System Up to Date & Protected</h3>
          <p className="text-xs text-zinc-400 mt-1 max-w-sm mx-auto">
            No outdated packages or unpatched security advisories matching current criteria.
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {grouped.map(([manager, items]) => (
            <div
              key={manager}
              className="bg-zinc-900/60 border border-zinc-800/80 rounded-lg overflow-hidden"
            >
              {/* Group Header (Collapsible) */}
              <div
                onClick={() => setCollapsedManagers((prev) => ({ ...prev, [manager]: !prev[manager] }))}
                className="bg-zinc-950/70 border-b border-zinc-800/80 px-4 py-2.5 flex items-center justify-between cursor-pointer hover:bg-zinc-900/80 transition-colors select-none"
              >
                <div className="flex items-center gap-2">
                  {collapsedManagers[manager] ? (
                    <ChevronRight className="w-3.5 h-3.5 text-zinc-400" />
                  ) : (
                    <ChevronDown className="w-3.5 h-3.5 text-zinc-400" />
                  )}
                  <span className="font-mono text-xs uppercase font-semibold text-zinc-200">
                    {manager}
                  </span>
                  <span className="text-[11px] font-mono text-zinc-400 bg-zinc-800/80 px-1.5 py-0.2 rounded">
                    {items.length} {items.length === 1 ? "package" : "packages"}
                  </span>
                </div>
                <span className="text-[10px] font-mono text-zinc-500">
                  {collapsedManagers[manager] ? "Click to expand" : "Click to collapse"}
                </span>
              </div>

              {/* Table */}
              {!collapsedManagers[manager] && (
                <div className="divide-y divide-zinc-800/50">
                {items.map((pkg) => {
                  const id = pkg.id;
                  const name = pkg.name;
                  const cur = pkg.current_version;
                  const lat = pkg.latest_version;
                  const changeType = pkg.change_type;
                  const pinned = pkg.pinned;
                  const selected = pkg.selected;
                  const hasVuln = pkg.has_vulnerability;
                  const vulns = pkg.vulnerabilities || [];
                  const maxSev = pkg.max_severity || "HIGH";
                  const isExpanded = expandedVulns[id];

                  return (
                    <div key={id} className="flex flex-col">
                      <div
                        onClick={() => !pinned && onTogglePackage(id)}
                        className={`px-4 py-3 flex items-center justify-between gap-4 transition-colors ${
                          pinned
                            ? "opacity-60 bg-zinc-950/30 cursor-not-allowed"
                            : "hover:bg-zinc-800/40 cursor-pointer"
                        } ${selected ? "bg-zinc-800/20" : ""} ${
                          hasVuln ? "border-l-2 border-l-rose-500 bg-rose-950/10" : ""
                        }`}
                      >
                        {/* Left: Checkbox + Name + Security Tag */}
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

                              {hasVuln && (
                                <span
                                  onClick={(e) => toggleExpand(id, e)}
                                  className="flex items-center gap-1 text-[10px] font-mono font-bold bg-rose-950 text-rose-300 border border-rose-700 px-1.5 py-0.2 rounded hover:bg-rose-900 cursor-pointer"
                                >
                                  <Flame className="w-2.5 h-2.5 text-rose-400" />
                                  {maxSev} CVE ({vulns.length})
                                  {isExpanded ? (
                                    <ChevronDown className="w-2.5 h-2.5" />
                                  ) : (
                                    <ChevronRight className="w-2.5 h-2.5" />
                                  )}
                                </span>
                              )}

                              <button
                                type="button"
                                title={pinned ? "Unpin package" : "Pin package (keep current version)"}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  onTogglePin(id);
                                }}
                                className={`flex items-center gap-1 text-[10px] font-mono px-1.5 py-0.5 rounded transition-colors ${
                                  pinned
                                    ? "bg-amber-950/80 text-amber-300 border border-amber-800 hover:bg-amber-900"
                                    : "text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/80 border border-transparent"
                                }`}
                              >
                                <Pin className="w-2.5 h-2.5" />
                                <span>{pinned ? "Pinned" : "Pin"}</span>
                              </button>
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
                            {hasVuln ? (
                              <span className="inline-flex items-center gap-1 text-[11px] font-mono font-bold px-2 py-0.5 rounded bg-rose-950/90 text-rose-300 border border-rose-700 animate-pulse">
                                🚨 patch now
                              </span>
                            ) : changeType === "major" ? (
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

                      {/* Expandable CVE details */}
                      {hasVuln && isExpanded && (
                        <div className="bg-rose-950/20 border-t border-rose-900/40 px-8 py-2.5 text-xs font-mono space-y-2">
                          <div className="text-[11px] font-semibold text-rose-400 uppercase tracking-wider flex items-center justify-between">
                            <span>Advisory Intelligence (OSV.dev):</span>
                          </div>
                          {vulns.map((v: Vulnerability, i: number) => {
                            const fixedIn = v.fixed_in;
                            const isMitigated = v.is_mitigated;
                            return (
                              <div key={i} className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-zinc-300 bg-rose-950/30 p-2 rounded border border-rose-900/30">
                                <div className="flex items-baseline gap-2 min-w-0">
                                  <span className="text-rose-400 font-bold shrink-0">{v.id}</span>
                                  <span className="text-zinc-500">[{v.severity}]</span>
                                  <span className="text-zinc-400 truncate">{v.summary || "Security vulnerability fix available"}</span>
                                </div>

                                <div className="shrink-0 flex items-center gap-1.5 text-[11px]">
                                  {isMitigated === false ? (
                                    <span className="bg-rose-900/80 text-rose-200 border border-rose-700 px-1.5 py-0.5 rounded font-bold">
                                      Zero-Day (Unmitigated)
                                    </span>
                                  ) : fixedIn ? (
                                    <span className="bg-emerald-950 text-emerald-300 border border-emerald-800 px-1.5 py-0.5 rounded">
                                      Fixed in {fixedIn}
                                    </span>
                                  ) : null}
                                </div>
                              </div>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
