import { useState, useMemo, useRef, useEffect, type FC, type MouseEvent } from "react";
import {
  Search,
  CheckSquare,
  Square,
  AlertTriangle,
  ShieldCheck,
  Pin,
  Flame,
  ChevronDown,
  ChevronRight,
  ExternalLink,
  X,
  MinusSquare,
  Loader2,
  CheckCircle2,
  ArrowUpCircle,
} from "lucide-react";
import type { Package, Vulnerability, FilterType } from "../lib/types";

interface PackageTableProps {
  packages: Package[];
  activeFilter: FilterType;
  isScanning: boolean;
  isUpgrading: boolean;
  activeManager?: string | null;
  completedManagers?: Record<string, "ok" | "failed">;
  onFilterChange: (filter: FilterType) => void;
  onTogglePackage: (id: string) => void;
  onTogglePin: (id: string) => void;
  onSelectSafe: () => void;
  onSelectSecurity: () => void;
  onSelectAll: () => void;
  onDeselectAll: () => void;
  onSelectOnlyFiltered: (filteredIds: string[]) => void;
  onDeselectFiltered: (filteredIds: string[]) => void;
  onUpgradeSubset: (ids: string[]) => void;
  onToggleManagerSelection: (manager: string) => void;
}

const ScanningSkeleton: FC = () => (
  <div className="space-y-3">
    <div className="bg-zinc-900/40 border border-zinc-800/80 rounded-lg p-8 text-center">
      <div className="inline-flex items-center justify-center w-10 h-10 rounded-full bg-zinc-800/80 mb-3 border border-zinc-700/60 shadow-inner">
        <Loader2 className="w-5 h-5 text-zinc-300 animate-spin" />
      </div>
      <h3 className="text-sm font-semibold text-zinc-200 font-mono">Scanning System Package Managers</h3>
      <p className="text-xs text-zinc-400 mt-1.5 max-w-md mx-auto">
        Querying Homebrew, Cargo, Pip, Gem, NPM, and 140+ package registries via MPM, computing SemVer diffs, and checking OSV.dev vulnerability intelligence...
      </p>
    </div>

    {/* Skeleton Rows */}
    <div className="bg-zinc-900/30 border border-zinc-800/60 rounded-lg overflow-hidden divide-y divide-zinc-800/40">
      <div className="bg-zinc-950/70 px-4 py-2 flex items-center justify-between">
        <div className="flex items-center gap-2">
          <div className="w-3.5 h-3.5 bg-zinc-800 rounded animate-pulse"></div>
          <div className="w-20 h-3 bg-zinc-800 rounded animate-pulse"></div>
        </div>
        <div className="w-16 h-2.5 bg-zinc-800 rounded animate-pulse"></div>
      </div>
      {[1, 2, 3, 4].map((i) => (
        <div key={i} className="px-4 py-2.5 flex items-center justify-between gap-4 animate-pulse">
          <div className="flex items-center gap-3">
            <div className="w-4 h-4 bg-zinc-800 rounded"></div>
            <div className="w-32 h-3 bg-zinc-800 rounded"></div>
          </div>
          <div className="flex items-center gap-3">
            <div className="w-24 h-3 bg-zinc-800 rounded"></div>
            <div className="w-16 h-4 bg-zinc-800 rounded"></div>
          </div>
        </div>
      ))}
    </div>
  </div>
);

export const PackageTable: FC<PackageTableProps> = ({
  packages,
  activeFilter,
  isScanning,
  isUpgrading,
  activeManager,
  completedManagers = {},
  onFilterChange,
  onTogglePackage,
  onTogglePin,
  onSelectSafe,
  onSelectSecurity,
  onSelectAll,
  onDeselectAll,
  onSelectOnlyFiltered,
  onDeselectFiltered,
  onUpgradeSubset,
  onToggleManagerSelection,
}) => {
  const [search, setSearch] = useState("");
  const [selectedManager, setSelectedManager] = useState<string>("all");
  const [expandedVulns, setExpandedVulns] = useState<Record<string, boolean>>({});
  const [collapsedManagers, setCollapsedManagers] = useState<Record<string, boolean>>({});
  const searchInputRef = useRef<HTMLInputElement>(null);

  // Global key listener for '/' search focus
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "/" && document.activeElement !== searchInputRef.current) {
        e.preventDefault();
        searchInputRef.current?.focus();
      } else if (e.key === "Escape" && document.activeElement === searchInputRef.current) {
        setSearch("");
        searchInputRef.current?.blur();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);

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

  const safePackagesCount = useMemo(() => {
    return packages.filter((p) => p.safe).length;
  }, [packages]);

  const majorPackagesCount = useMemo(() => {
    return packages.filter((p) => p.change_type === "major").length;
  }, [packages]);

  const pinnedPackagesCount = useMemo(() => {
    return packages.filter((p) => p.pinned).length;
  }, [packages]);

  const filteredPackages = useMemo(() => {
    return packages.filter((p) => {
      const name = p.name;
      const mgr = p.manager;
      const changeType = p.change_type;
      const safe = p.safe;
      const hasVuln = p.has_vulnerability;
      const pinned = p.pinned;

      if (search) {
        const q = search.toLowerCase();
        const matchesName = name.toLowerCase().includes(q);
        const matchesMgr = mgr.toLowerCase().includes(q);
        const matchesVuln = (p.vulnerabilities || []).some(
          (v) => v.id.toLowerCase().includes(q) || v.summary.toLowerCase().includes(q)
        );
        if (!matchesName && !matchesMgr && !matchesVuln) {
          return false;
        }
      }

      if (activeFilter === "security" && !hasVuln) return false;
      if (activeFilter === "safe" && !safe) return false;
      if (activeFilter === "major" && changeType !== "major") return false;
      if (activeFilter === "pinned" && !pinned) return false;
      if (selectedManager !== "all" && mgr !== selectedManager) return false;

      return true;
    });
  }, [packages, search, activeFilter, selectedManager]);

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

  const isFilterActive =
    activeFilter !== "all" || search.trim() !== "" || selectedManager !== "all";

  const filteredSelectable = useMemo(() => {
    return filteredPackages.filter((p) => !p.pinned);
  }, [filteredPackages]);

  const filteredSelected = useMemo(() => {
    return filteredPackages.filter((p) => p.selected && !p.pinned);
  }, [filteredPackages]);

  const filterLabel = useMemo(() => {
    if (activeFilter === "security") return "CVE Vulnerabilities";
    if (activeFilter === "safe") return "Safe Updates";
    if (activeFilter === "major") return "Major Breaking Updates";
    if (activeFilter === "pinned") return "Pinned Packages";
    if (selectedManager !== "all") return `Manager: ${selectedManager}`;
    if (search.trim() !== "") return `Search: "${search}"`;
    return "";
  }, [activeFilter, selectedManager, search]);

  // If initial scanning and no packages loaded yet, render skeleton
  if (isScanning && packages.length === 0) {
    return <ScanningSkeleton />;
  }

  return (
    <div className="space-y-3.5">
      {/* Live Upgrade Progress Notification */}
      {isUpgrading && (
        <div className="bg-blue-950/70 border border-blue-800/90 rounded-lg p-3 flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 text-xs font-mono shadow-lg animate-in fade-in">
          <div className="flex items-center gap-2.5 text-blue-200">
            <Loader2 className="w-4 h-4 animate-spin text-blue-400 shrink-0" />
            <span>
              {activeManager
                ? `Executing upgrades for manager: [${activeManager}]...`
                : "Upgrade queue dispatched, executing commands..."}
            </span>
          </div>
          <span className="text-[11px] text-zinc-400 bg-zinc-900/80 px-2 py-0.5 rounded border border-zinc-800 shrink-0">
            Controls locked during execution
          </span>
        </div>
      )}

      {/* Subtle indicator for background rescan when data already exists */}
      {isScanning && packages.length > 0 && (
        <div className="flex items-center gap-2 text-xs font-mono text-zinc-400 bg-zinc-900/60 border border-zinc-800 px-3 py-1.5 rounded-lg animate-pulse">
          <Loader2 className="w-3.5 h-3.5 animate-spin text-zinc-400" />
          <span>Rescanning package registries for latest version diffs...</span>
        </div>
      )}

      {/* Controls Bar */}
      <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-3 bg-zinc-900/60 border border-zinc-800/80 p-2.5 rounded-lg">
        {/* Search */}
        <div className="relative flex-1 max-w-sm">
          <Search className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            ref={searchInputRef}
            type="text"
            disabled={isUpgrading}
            placeholder="Search package, manager, or CVE... (/)"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-zinc-950/80 border border-zinc-800/90 rounded-md pl-9 pr-8 py-1.5 text-xs font-mono text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-zinc-600 focus:ring-1 focus:ring-zinc-600/30 disabled:opacity-50"
          />
          {search && (
            <button
              type="button"
              disabled={isUpgrading}
              onClick={() => setSearch("")}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-500 hover:text-zinc-300 p-0.5"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>

        {/* Filter Pills - Order: Critical/CVEs -> Safe -> Major -> All -> Pinned */}
        <div className="flex items-center gap-1.5 text-xs flex-wrap">
          {securityPackagesCount > 0 && (
            <button
              type="button"
              disabled={isUpgrading}
              onClick={() => onFilterChange("security")}
              className={`px-2.5 py-1 rounded-md transition-colors flex items-center gap-1 text-xs font-mono disabled:opacity-50 ${
                activeFilter === "security"
                  ? "bg-rose-950 text-rose-300 border border-rose-600 font-semibold"
                  : "text-rose-400 hover:text-rose-300 hover:bg-rose-950/40 border border-rose-900/40 cursor-pointer"
              }`}
            >
              <Flame className="w-3 h-3 text-rose-400" />
              Critical / CVEs ({securityPackagesCount})
            </button>
          )}

          <button
            type="button"
            disabled={isUpgrading}
            onClick={() => onFilterChange("safe")}
            className={`px-2.5 py-1 rounded-md transition-colors text-xs font-mono disabled:opacity-50 ${
              activeFilter === "safe"
                ? "bg-emerald-950 text-emerald-300 border border-emerald-600 font-semibold"
                : "text-zinc-400 hover:text-emerald-400 hover:bg-zinc-800/50 border border-transparent cursor-pointer"
            }`}
          >
            Safe ({safePackagesCount})
          </button>

          <button
            type="button"
            disabled={isUpgrading}
            onClick={() => onFilterChange("major")}
            className={`px-2.5 py-1 rounded-md transition-colors text-xs font-mono disabled:opacity-50 ${
              activeFilter === "major"
                ? "bg-amber-950 text-amber-300 border border-amber-600 font-semibold"
                : "text-zinc-400 hover:text-amber-400 hover:bg-zinc-800/50 border border-transparent cursor-pointer"
            }`}
          >
            Major ({majorPackagesCount})
          </button>

          <button
            type="button"
            disabled={isUpgrading}
            onClick={() => onFilterChange("all")}
            className={`px-2.5 py-1 rounded-md transition-colors text-xs font-mono disabled:opacity-50 ${
              activeFilter === "all"
                ? "bg-zinc-800 text-zinc-100 font-semibold border border-zinc-700"
                : "text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/50 border border-transparent cursor-pointer"
            }`}
          >
            All ({packages.length})
          </button>

          {pinnedPackagesCount > 0 && (
            <button
              type="button"
              disabled={isUpgrading}
              onClick={() => onFilterChange("pinned")}
              className={`px-2.5 py-1 rounded-md transition-colors text-xs font-mono disabled:opacity-50 ${
                activeFilter === "pinned"
                  ? "bg-zinc-800 text-zinc-100 border border-zinc-600 font-semibold"
                  : "text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/50 border border-transparent cursor-pointer"
              }`}
            >
              Pinned ({pinnedPackagesCount})
            </button>
          )}

          {/* Manager filter select */}
          {managers.length > 1 && (
            <select
              disabled={isUpgrading}
              value={selectedManager}
              onChange={(e) => setSelectedManager(e.target.value)}
              className="bg-zinc-950 border border-zinc-800 text-xs font-mono text-zinc-300 rounded-md px-2 py-1 ml-1 focus:outline-none focus:border-zinc-700 cursor-pointer disabled:opacity-50"
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
        <div className="flex items-center gap-1.5 border-t lg:border-t-0 pt-2 lg:pt-0 border-zinc-800/80">
          {securityPackagesCount > 0 && (
            <button
              type="button"
              disabled={isUpgrading}
              onClick={onSelectSecurity}
              className="text-[11px] font-mono text-rose-300 hover:text-rose-100 bg-rose-950/80 hover:bg-rose-900 border border-rose-800 px-2.5 py-1 rounded-md transition-colors font-medium cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            >
              Select CVEs
            </button>
          )}
          <button
            type="button"
            disabled={isUpgrading}
            onClick={onSelectSafe}
            className="text-[11px] font-mono text-emerald-400 hover:text-emerald-200 bg-emerald-950/40 hover:bg-emerald-950/70 border border-emerald-900/60 px-2.5 py-1 rounded-md transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          >
            Select Safe
          </button>
          <button
            type="button"
            disabled={isUpgrading}
            onClick={onSelectAll}
            className="text-[11px] font-mono text-zinc-300 hover:text-zinc-100 bg-zinc-950 hover:bg-zinc-800 border border-zinc-800 px-2.5 py-1 rounded-md transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          >
            All
          </button>
          <button
            type="button"
            disabled={isUpgrading}
            onClick={onDeselectAll}
            className="text-[11px] font-mono text-zinc-400 hover:text-zinc-200 bg-zinc-950 hover:bg-zinc-800 border border-zinc-800 px-2.5 py-1 rounded-md transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          >
            Clear
          </button>
        </div>
      </div>

      {/* Contextual Filter Scope & Action Bar */}
      {isFilterActive && filteredPackages.length > 0 && (
        <div className="bg-zinc-900/90 border border-zinc-800 rounded-lg p-2.5 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs font-mono shadow-sm">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="text-zinc-500">Filtered:</span>
            <span className="text-zinc-200 font-semibold bg-zinc-800/80 px-2 py-0.5 rounded border border-zinc-700/60">
              {filterLabel}
            </span>
            <span className="text-zinc-400">
              ({filteredSelected.length} of {filteredPackages.length} selected)
            </span>
          </div>

          <div className="flex items-center gap-2 flex-wrap">
            <button
              type="button"
              disabled={isUpgrading}
              onClick={() => onSelectOnlyFiltered(filteredPackages.map((p) => p.id))}
              className="text-[11px] px-2.5 py-1 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-200 transition-colors cursor-pointer font-medium disabled:opacity-40"
              title="Deselect all background packages and select only these visible items"
            >
              Select Only Visible ({filteredSelectable.length})
            </button>

            <button
              type="button"
              disabled={isUpgrading || filteredSelected.length === 0}
              onClick={() => onDeselectFiltered(filteredPackages.map((p) => p.id))}
              className="text-[11px] px-2.5 py-1 rounded bg-zinc-950 hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 border border-zinc-800 transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
            >
              Clear Visible
            </button>

            {filteredSelected.length > 0 && (
              <button
                type="button"
                disabled={isUpgrading}
                onClick={() => onUpgradeSubset(filteredSelected.map((p) => p.id))}
                className="text-[11px] px-3 py-1 rounded bg-rose-600 hover:bg-rose-500 text-white font-semibold transition-colors flex items-center gap-1.5 cursor-pointer shadow-sm disabled:opacity-50"
              >
                <ArrowUpCircle className="w-3.5 h-3.5" />
                <span>Upgrade Filtered ({filteredSelected.length})</span>
              </button>
            )}
          </div>
        </div>
      )}

      {/* Package Groups or Empty State */}
      {grouped.length === 0 ? (
        <div className="bg-zinc-900/40 border border-zinc-800/80 rounded-lg p-12 text-center">
          <ShieldCheck className="w-8 h-8 text-emerald-500/80 mx-auto mb-2" />
          <h3 className="text-sm font-semibold text-zinc-200">System Protected & Up to Date</h3>
          <p className="text-xs text-zinc-400 mt-1 max-w-sm mx-auto">
            {search || activeFilter !== "all" || selectedManager !== "all"
              ? "No packages match the active search or filter criteria."
              : "No outdated packages or unpatched security advisories found across managers."}
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {grouped.map(([manager, items]) => {
            const isManagerActive = isUpgrading && activeManager === manager;
            const managerOutcome = completedManagers[manager];
            const selectableItems = items.filter((p) => !p.pinned);
            const selectedItems = items.filter((p) => p.selected && !p.pinned);
            const isAllSelected = selectableItems.length > 0 && selectedItems.length === selectableItems.length;
            const isPartiallySelected = selectedItems.length > 0 && !isAllSelected;

            return (
              <div
                key={manager}
                className={`bg-zinc-900/50 border rounded-lg overflow-hidden transition-all ${
                  isManagerActive
                    ? "border-blue-500 ring-1 ring-blue-500/30"
                    : managerOutcome === "ok"
                    ? "border-emerald-700/80"
                    : managerOutcome === "failed"
                    ? "border-rose-700/80"
                    : "border-zinc-800/80"
                }`}
              >
                {/* Group Header */}
                <div className="bg-zinc-950/80 border-b border-zinc-800/80 px-3.5 py-2 flex items-center justify-between select-none">
                  <div className="flex items-center gap-2.5">
                    {/* Collapsible toggle */}
                    <button
                      type="button"
                      disabled={isUpgrading}
                      onClick={() => setCollapsedManagers((prev) => ({ ...prev, [manager]: !prev[manager] }))}
                      className="text-zinc-400 hover:text-zinc-200 p-0.5 rounded cursor-pointer disabled:opacity-40"
                      title={collapsedManagers[manager] ? "Expand manager" : "Collapse manager"}
                    >
                      {collapsedManagers[manager] ? (
                        <ChevronRight className="w-4 h-4" />
                      ) : (
                        <ChevronDown className="w-4 h-4" />
                      )}
                    </button>

                    {/* Manager selection checkbox */}
                    <button
                      type="button"
                      disabled={selectableItems.length === 0 || isUpgrading}
                      onClick={() => onToggleManagerSelection(manager)}
                      className="text-zinc-400 hover:text-zinc-200 transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                      title={
                        isAllSelected
                          ? `Deselect all ${manager} packages`
                          : `Select all ${manager} packages`
                      }
                    >
                      {isAllSelected ? (
                        <CheckSquare className="w-4 h-4 text-zinc-100" />
                      ) : isPartiallySelected ? (
                        <MinusSquare className="w-4 h-4 text-zinc-300" />
                      ) : (
                        <Square className="w-4 h-4 text-zinc-600" />
                      )}
                    </button>

                    <div className="flex items-center gap-2">
                      <span className="font-mono text-xs uppercase font-semibold text-zinc-200">
                        {manager}
                      </span>
                      <span className="text-[10px] font-mono text-zinc-400 bg-zinc-800/80 border border-zinc-700/50 px-1.5 py-0.2 rounded">
                        {selectedItems.length}/{items.length} selected
                      </span>

                      {/* Live manager status badge */}
                      {isManagerActive && (
                        <span className="flex items-center gap-1 text-[10px] font-mono text-blue-300 bg-blue-950 border border-blue-700 px-1.5 py-0.2 rounded">
                          <Loader2 className="w-3 h-3 animate-spin text-blue-400" />
                          Upgrading...
                        </span>
                      )}
                      {managerOutcome === "ok" && (
                        <span className="flex items-center gap-1 text-[10px] font-mono text-emerald-300 bg-emerald-950 border border-emerald-700 px-1.5 py-0.2 rounded font-semibold">
                          <CheckCircle2 className="w-3 h-3 text-emerald-400" />
                          Done
                        </span>
                      )}
                      {managerOutcome === "failed" && (
                        <span className="flex items-center gap-1 text-[10px] font-mono text-rose-300 bg-rose-950 border border-rose-700 px-1.5 py-0.2 rounded font-semibold">
                          Failed
                        </span>
                      )}
                    </div>
                  </div>

                  <span className="text-[11px] font-mono text-zinc-500">
                    {collapsedManagers[manager] ? "Collapsed" : `${items.length} items`}
                  </span>
                </div>

                {/* Table rows */}
                {!collapsedManagers[manager] && (
                  <div className="divide-y divide-zinc-800/40">
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

                      const isPkgActive = isManagerActive && selected;
                      const isPkgDone = managerOutcome === "ok" && selected;
                      const isPkgFailed = managerOutcome === "failed" && selected;

                      return (
                        <div key={id} className="flex flex-col">
                          <div
                            onClick={() => !pinned && !isUpgrading && onTogglePackage(id)}
                            className={`px-3.5 py-2 flex items-center justify-between gap-4 transition-colors ${
                              isUpgrading
                                ? "cursor-wait"
                                : pinned
                                ? "opacity-60 bg-zinc-950/40 cursor-not-allowed"
                                : "hover:bg-zinc-800/30 cursor-pointer"
                            } ${selected ? "bg-zinc-800/20" : ""} ${
                              hasVuln ? "border-l-2 border-l-rose-500 bg-rose-950/15" : ""
                            } ${isPkgActive ? "bg-blue-950/30 border-l-2 border-l-blue-400" : ""}`}
                          >
                            {/* Left: Checkbox + Name + Badges */}
                            <div className="flex items-center gap-2.5 min-w-0">
                              <button
                                type="button"
                                disabled={pinned || isUpgrading}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  if (!pinned && !isUpgrading) onTogglePackage(id);
                                }}
                                className="text-zinc-400 hover:text-zinc-200 transition-colors focus:outline-none shrink-0 disabled:cursor-not-allowed"
                              >
                                {isPkgActive ? (
                                  <Loader2 className="w-4 h-4 animate-spin text-blue-400" />
                                ) : isPkgDone ? (
                                  <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                                ) : selected ? (
                                  <CheckSquare className="w-4 h-4 text-zinc-100" />
                                ) : (
                                  <Square className="w-4 h-4 text-zinc-600" />
                                )}
                              </button>

                              <div className="min-w-0 flex items-center gap-2">
                                <span className="font-mono text-xs font-semibold text-zinc-200 truncate">
                                  {name}
                                </span>

                                {pkg.source_url && (
                                  <a
                                    href={pkg.source_url}
                                    target="_blank"
                                    rel="noopener noreferrer"
                                    onClick={(e) => e.stopPropagation()}
                                    title={`Open upstream repository/registry (${pkg.source_url})`}
                                    className="text-zinc-500 hover:text-zinc-300 transition-colors p-0.5 inline-flex items-center shrink-0 cursor-pointer"
                                  >
                                    <ExternalLink className="w-3 h-3" />
                                  </a>
                                )}

                                {pkg.description && (
                                  <span className="text-[11px] text-zinc-500 truncate max-w-xs hidden xl:inline" title={pkg.description}>
                                    {pkg.description}
                                  </span>
                                )}

                                {hasVuln && (
                                  <button
                                    type="button"
                                    onClick={(e) => toggleExpand(id, e)}
                                    className="flex items-center gap-1 text-[10px] font-mono font-bold bg-rose-950 text-rose-300 border border-rose-700/80 px-1.5 py-0.2 rounded hover:bg-rose-900 cursor-pointer shrink-0"
                                    title="Click to view OSV security advisory details"
                                  >
                                    <Flame className="w-2.5 h-2.5 text-rose-400" />
                                    <span>{maxSev} CVE ({vulns.length})</span>
                                    {isExpanded ? (
                                      <ChevronDown className="w-2.5 h-2.5" />
                                    ) : (
                                      <ChevronRight className="w-2.5 h-2.5" />
                                    )}
                                  </button>
                                )}

                                <button
                                  type="button"
                                  disabled={isUpgrading}
                                  title={pinned ? "Unpin package" : "Pin package (lock current version)"}
                                  onClick={(e) => {
                                    e.stopPropagation();
                                    if (!isUpgrading) onTogglePin(id);
                                  }}
                                  className={`flex items-center gap-1 text-[10px] font-mono px-1.5 py-0.5 rounded transition-colors shrink-0 disabled:opacity-40 disabled:cursor-not-allowed ${
                                    pinned
                                      ? "bg-amber-950/90 text-amber-300 border border-amber-700 hover:bg-amber-900"
                                      : "text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/80 border border-transparent"
                                  }`}
                                >
                                  <Pin className="w-2.5 h-2.5" />
                                  <span>{pinned ? "Pinned" : "Pin"}</span>
                                </button>
                              </div>
                            </div>

                            {/* Right: Versions + Change Type Badge */}
                            <div className="flex items-center gap-3.5 shrink-0">
                              {/* Version flow */}
                              <div className="flex items-center gap-1.5 font-mono text-xs tabular-nums">
                                <span className="text-zinc-400">{cur}</span>
                                <span className="text-zinc-600 select-none">→</span>
                                <span className="text-zinc-100 font-semibold">{lat}</span>
                              </div>

                              {/* Semver / Status Badge */}
                              <div className="w-24 flex justify-end">
                                {isPkgActive ? (
                                  <span className="inline-flex items-center gap-1 text-[10px] font-mono font-bold px-1.5 py-0.5 rounded bg-blue-950 text-blue-300 border border-blue-700 uppercase tracking-wider">
                                    <Loader2 className="w-2.5 h-2.5 animate-spin" /> RUNNING
                                  </span>
                                ) : isPkgDone ? (
                                  <span className="inline-flex items-center text-[10px] font-mono font-bold px-1.5 py-0.5 rounded bg-emerald-950 text-emerald-300 border border-emerald-700 uppercase tracking-wider">
                                    UPGRADED ✓
                                  </span>
                                ) : isPkgFailed ? (
                                  <span className="inline-flex items-center text-[10px] font-mono font-bold px-1.5 py-0.5 rounded bg-rose-950 text-rose-300 border border-rose-700 uppercase tracking-wider">
                                    FAILED ✗
                                  </span>
                                ) : hasVuln ? (
                                  <span className="inline-flex items-center text-[10px] font-mono font-bold px-1.5 py-0.5 rounded bg-rose-950 text-rose-300 border border-rose-700 uppercase tracking-wider">
                                    CVE FIX
                                  </span>
                                ) : changeType === "major" ? (
                                  <span className="inline-flex items-center gap-1 text-[10px] font-mono font-semibold px-1.5 py-0.5 rounded bg-amber-950/60 text-amber-300 border border-amber-800/60 uppercase tracking-wider">
                                    <AlertTriangle className="w-2.5 h-2.5 text-amber-400" />
                                    MAJOR
                                  </span>
                                ) : changeType === "minor" ? (
                                  <span className="inline-flex items-center text-[10px] font-mono font-medium px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-300 border border-zinc-700 uppercase tracking-wider">
                                    MINOR
                                  </span>
                                ) : changeType === "patch" ? (
                                  <span className="inline-flex items-center text-[10px] font-mono font-medium px-1.5 py-0.5 rounded bg-emerald-950/60 text-emerald-400 border border-emerald-800/60 uppercase tracking-wider">
                                    PATCH
                                  </span>
                                ) : (
                                  <span className="inline-flex items-center text-[10px] font-mono px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-400 border border-zinc-700 uppercase tracking-wider">
                                    UPDATE
                                  </span>
                                )}
                              </div>
                            </div>
                          </div>

                          {/* Expandable CVE details with OSV.dev deep links */}
                          {hasVuln && isExpanded && (
                            <div className="bg-rose-950/20 border-t border-rose-900/40 px-6 py-2.5 text-xs font-mono space-y-2">
                              <div className="text-[10px] font-semibold text-rose-400 uppercase tracking-wider flex items-center justify-between">
                                <span>Security Advisory Intelligence (OSV.dev):</span>
                                <span className="text-zinc-500 font-normal">Click CVE ID to open upstream advisory</span>
                              </div>
                              {vulns.map((v: Vulnerability, i: number) => {
                                const fixedIn = v.fixed_in;
                                const isMitigated = v.is_mitigated;
                                const osvUrl = `https://osv.dev/vulnerability/${encodeURIComponent(v.id)}`;

                                return (
                                  <div
                                    key={i}
                                    className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 text-zinc-300 bg-rose-950/30 p-2 rounded-md border border-rose-900/40"
                                  >
                                    <div className="flex items-center gap-2 min-w-0 flex-wrap sm:flex-nowrap">
                                      <a
                                        href={osvUrl}
                                        target="_blank"
                                        rel="noopener noreferrer"
                                        onClick={(e) => e.stopPropagation()}
                                        className="text-rose-400 hover:text-rose-200 font-bold shrink-0 inline-flex items-center gap-1 underline underline-offset-2 decoration-rose-500/50"
                                        title={`View ${v.id} details on OSV.dev`}
                                      >
                                        <span>{v.id}</span>
                                        <ExternalLink className="w-3 h-3" />
                                      </a>
                                      {v.source && (
                                        <span className="text-[10px] px-1.5 py-0.2 rounded bg-zinc-900 border border-zinc-700/80 text-zinc-300 shrink-0">
                                          {v.source}
                                        </span>
                                      )}
                                      <span
                                        className={`text-[10px] px-1.5 py-0.2 rounded shrink-0 font-bold uppercase ${
                                          v.severity === "CRITICAL"
                                            ? "bg-rose-950 text-rose-200 border border-rose-600"
                                            : v.severity === "HIGH"
                                            ? "bg-rose-900/60 text-rose-300 border border-rose-700"
                                            : "bg-amber-950 text-amber-300 border border-amber-700"
                                        }`}
                                      >
                                        {v.score ? `${v.severity} (${v.score})` : v.severity || "HIGH"}
                                      </span>
                                      <span className="text-zinc-300 truncate text-[11px]" title={v.summary}>
                                        {v.summary || "Security vulnerability fix available in upstream release"}
                                      </span>
                                    </div>

                                    <div className="shrink-0 flex items-center gap-1.5 text-[10px]">
                                      {isMitigated === false ? (
                                        <span className="bg-rose-900/90 text-rose-100 border border-rose-600 px-1.5 py-0.5 rounded font-bold">
                                          Zero-Day (Unmitigated)
                                        </span>
                                      ) : fixedIn ? (
                                        <span className="bg-emerald-950 text-emerald-300 border border-emerald-700 px-1.5 py-0.5 rounded font-mono">
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
            );
          })}
        </div>
      )}
    </div>
  );
};
