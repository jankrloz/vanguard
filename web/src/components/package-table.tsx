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
  ListFilter,
  ArrowUpCircle,
} from "lucide-react";
import type { Package, Vulnerability, FilterType } from "../lib/types";
import type { OperationType } from "../App";

interface PackageTableProps {
  packages: Package[];
  activeFilter: FilterType;
  isScanning: boolean;
  isUpgrading: boolean;
  activeOperation?: OperationType;
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
  onToggleManagerSelection: (manager: string) => void;
  onUpgrade?: () => void;
}

const ScanningSkeleton: FC = () => (
  <div className="space-y-3">
    <div className="bg-zinc-900/40 border border-zinc-800/80 rounded-lg p-8 text-center">
      <div className="inline-flex items-center justify-center w-10 h-10 rounded-full bg-zinc-800/80 mb-3 border border-zinc-700/60 shadow-inner">
        <Loader2 className="w-5 h-5 text-zinc-300 animate-spin" />
      </div>
      <h3 className="text-sm font-semibold text-zinc-200 font-mono">Scanning System Package Managers</h3>
      <p className="text-xs text-zinc-400 mt-1.5 max-w-md mx-auto">
        Querying registries via MPM, computing SemVer diffs, and checking OSV.dev, Trivy &amp; native advisory intelligence...
      </p>
    </div>

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
  activeOperation = "upgrade",
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
  onToggleManagerSelection,
  onUpgrade,
}) => {
  const [search, setSearch] = useState("");
  const [selectedManagers, setSelectedManagers] = useState<string[]>([]);
  const [isManagerDropdownOpen, setIsManagerDropdownOpen] = useState(false);
  const managerDropdownRef = useRef<HTMLDivElement>(null);
  const [expandedVulns, setExpandedVulns] = useState<Record<string, boolean>>({});
  const [collapsedManagers, setCollapsedManagers] = useState<Record<string, boolean>>({});
  const searchInputRef = useRef<HTMLInputElement>(null);

  // Close manager dropdown on click outside or Escape
  useEffect(() => {
    if (!isManagerDropdownOpen) return;
    const handleClickOutside = (e: globalThis.MouseEvent) => {
      if (managerDropdownRef.current && !managerDropdownRef.current.contains(e.target as Node)) {
        setIsManagerDropdownOpen(false);
      }
    };
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setIsManagerDropdownOpen(false);
      }
    };
    document.addEventListener("mousedown", handleClickOutside);
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [isManagerDropdownOpen]);

  // Global key listener for '/' search focus
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const targetTag = (e.target as HTMLElement)?.tagName;
      const isEditing =
        targetTag === "INPUT" ||
        targetTag === "TEXTAREA" ||
        (e.target as HTMLElement)?.isContentEditable;
      if (e.key === "/" && !isEditing && document.activeElement !== searchInputRef.current) {
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

  const queuedPackagesCount = useMemo(() => {
    return packages.filter((p) => p.selected && !p.pinned).length;
  }, [packages]);

  const isManagerFiltered = useMemo(() => {
    return selectedManagers.length > 0 && selectedManagers.length < managers.length;
  }, [selectedManagers, managers]);

  const activeManagerSet = useMemo(() => {
    return new Set(isManagerFiltered ? selectedManagers : managers);
  }, [isManagerFiltered, selectedManagers, managers]);

  const toggleManager = (m: string) => {
    if (!isManagerFiltered) {
      setSelectedManagers(managers.filter((x) => x !== m));
    } else if (selectedManagers.includes(m)) {
      const next = selectedManagers.filter((x) => x !== m);
      setSelectedManagers(next.length === 0 ? [] : next);
    } else {
      const next = [...selectedManagers, m];
      if (next.length >= managers.length) {
        setSelectedManagers([]);
      } else {
        setSelectedManagers(next);
      }
    }
  };

  const filteredPackages = useMemo(() => {
    return packages.filter((p) => {
      const name = p.name;
      const mgr = p.manager;
      const changeType = p.change_type;
      const safe = p.safe;
      const hasVuln = p.has_vulnerability;
      const pinned = p.pinned;
      const selected = p.selected;

      if (search) {
        const q = search.toLowerCase();
        const matchesName = name.toLowerCase().includes(q);
        const matchesMgr = mgr.toLowerCase().includes(q);
        const matchesDesc = (p.description || "").toLowerCase().includes(q);
        const matchesVuln = (p.vulnerabilities || []).some(
          (v) => v.id.toLowerCase().includes(q) || v.summary.toLowerCase().includes(q)
        );
        if (!matchesName && !matchesMgr && !matchesDesc && !matchesVuln) {
          return false;
        }
      }

      if (activeFilter === "security" && !hasVuln) return false;
      if (activeFilter === "safe" && !safe) return false;
      if (activeFilter === "major" && changeType !== "major") return false;
      if (activeFilter === "pinned" && !pinned) return false;
      if (activeFilter === "queued" && (!selected || pinned)) return false;
      if (isManagerFiltered && !activeManagerSet.has(mgr)) return false;

      return true;
    });
  }, [packages, search, activeFilter, isManagerFiltered, activeManagerSet]);

  const isFilterActive =
    activeFilter !== "all" || search.trim() !== "" || isManagerFiltered;

  const filteredSelectable = useMemo(() => {
    return filteredPackages.filter((p) => !p.pinned);
  }, [filteredPackages]);

  const filteredSelected = useMemo(() => {
    return filteredPackages.filter((p) => p.selected && !p.pinned);
  }, [filteredPackages]);

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

  const operationTitle =
    activeOperation === "clean"
      ? "Cleaning Package Caches & Old Versions"
      : activeOperation === "topgrade"
      ? "Running Deep System Maintenance Sweep (Topgrade)"
      : "Executing Package Upgrades";

  const operationSubtitle =
    activeOperation === "clean"
      ? "Purging orphaned downloads and cache directories across package managers..."
      : activeOperation === "topgrade"
      ? "Updating Git repositories, toolchains, shell frameworks, and system caches..."
      : activeManager
      ? `Active manager: [${activeManager}] — logs streaming in drawer`
      : "Dispatched upgrade queue to system package managers...";

  if (isScanning && packages.length === 0) {
    return <ScanningSkeleton />;
  }

  return (
    <div className="space-y-3">
      {/* Live Execution Progress Notification */}
      {isUpgrading && (
        <div className="bg-blue-950/70 border border-blue-800/90 rounded-lg p-3 flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 text-xs font-mono shadow-lg animate-in fade-in">
          <div className="flex items-center gap-2.5 text-blue-200">
            <Loader2 className="w-4 h-4 animate-spin text-blue-400 shrink-0" />
            <span>{operationSubtitle}</span>
          </div>
          <span className="text-[11px] text-zinc-300 bg-zinc-900/80 px-2 py-0.5 rounded border border-zinc-800 shrink-0">
            Controls locked during execution
          </span>
        </div>
      )}

      {/* Background rescan subtle banner */}
      {isScanning && packages.length > 0 && (
        <div className="flex items-center gap-2 text-xs font-mono text-zinc-300 bg-zinc-900/60 border border-zinc-800 px-3 py-1.5 rounded-lg">
          <Loader2 className="w-3.5 h-3.5 animate-spin text-zinc-400" />
          <span>Rescanning package registries for latest version diffs...</span>
        </div>
      )}

      {/* Unified Controls & Scope Bar (Search + Managers + Queue Toggle + Bulk Selection) */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5 bg-zinc-900/60 border border-zinc-800/80 p-2.5 rounded-lg">
        <div className="flex flex-wrap items-center gap-2 flex-1 min-w-0">
          {/* Search */}
          <div className="relative flex-1 min-w-[200px] sm:max-w-xs">
            <Search className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-zinc-400 pointer-events-none" />
            <input
              ref={searchInputRef}
              id="package-search"
              name="package-search"
              aria-label="Search packages, descriptions, or CVEs"
              type="text"
              disabled={isUpgrading}
              placeholder="Search package or CVE... (/)"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="w-full bg-zinc-950/90 border border-zinc-800/90 rounded-md pl-9 pr-8 py-1.5 text-xs font-mono text-zinc-100 placeholder-zinc-400 focus:outline-none focus:border-zinc-600 focus:ring-1 focus:ring-zinc-600/30 disabled:opacity-50"
            />
            {search && (
              <button
                type="button"
                aria-label="Clear search query"
                disabled={isUpgrading}
                onClick={() => setSearch("")}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-400 hover:text-zinc-200 p-0.5 cursor-pointer"
              >
                <X className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          {/* Custom Multi-select Manager Filter Dropdown */}
          {managers.length > 1 && (
            <div className="relative" ref={managerDropdownRef}>
              <button
                type="button"
                disabled={isUpgrading}
                aria-expanded={isManagerDropdownOpen}
                onClick={() => setIsManagerDropdownOpen((prev) => !prev)}
                className={`px-2.5 py-1.5 rounded-md text-xs font-mono transition-colors flex items-center gap-1.5 cursor-pointer disabled:opacity-50 ${
                  isManagerFiltered
                    ? "bg-zinc-800 text-zinc-100 font-semibold border border-zinc-600 shadow-sm"
                    : "text-zinc-300 hover:text-zinc-100 hover:bg-zinc-800/60 border border-zinc-800/90 bg-zinc-950/80"
                }`}
                title="Filter by package managers"
              >
                <span>
                  {isManagerFiltered
                    ? `Managers (${selectedManagers.length}/${managers.length})`
                    : `Managers: All (${managers.length})`}
                </span>
                <ChevronDown
                  className={`w-3.5 h-3.5 text-zinc-400 transition-transform duration-150 ${
                    isManagerDropdownOpen ? "rotate-180 text-zinc-200" : ""
                  }`}
                />
              </button>

              {isManagerDropdownOpen && (
                <div className="absolute top-full left-0 mt-1.5 w-64 bg-zinc-950 border border-zinc-800/90 rounded-lg shadow-2xl p-2 z-30 font-mono text-xs backdrop-blur-md animate-in fade-in zoom-in-95 duration-100">
                  <div className="flex items-center justify-between px-2 py-1 border-b border-zinc-800/80 mb-1.5">
                    <span className="text-[11px] font-semibold text-zinc-300 uppercase tracking-wider">
                      Package Managers
                    </span>
                    {isManagerFiltered && (
                      <button
                        type="button"
                        onClick={() => setSelectedManagers([])}
                        className="text-[11px] text-cyan-400 hover:text-cyan-300 px-1.5 py-0.5 rounded hover:bg-zinc-800 transition-colors cursor-pointer"
                        title="Show all package managers"
                      >
                        Reset
                      </button>
                    )}
                  </div>

                  <div className="max-h-56 overflow-y-auto space-y-0.5 pr-0.5">
                    {managers.map((m) => {
                      const isChecked = !isManagerFiltered || selectedManagers.includes(m);
                      const count = managerCounts.get(m) || 0;
                      return (
                        <div
                          key={m}
                          className="group flex items-center justify-between px-2 py-1.5 rounded-md hover:bg-zinc-900 transition-colors"
                        >
                          <button
                            type="button"
                            onClick={() => toggleManager(m)}
                            className="flex items-center gap-2 min-w-0 flex-1 text-left cursor-pointer"
                          >
                            {isChecked ? (
                              <CheckSquare className="w-3.5 h-3.5 text-zinc-200 shrink-0" />
                            ) : (
                              <Square className="w-3.5 h-3.5 text-zinc-500 shrink-0" />
                            )}
                            <span
                              className={`text-xs truncate ${
                                isChecked ? "text-zinc-100 font-medium" : "text-zinc-400"
                              }`}
                            >
                              {m}
                            </span>
                            <span className="text-[11px] text-zinc-400 font-mono">
                              ({count})
                            </span>
                          </button>

                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              setSelectedManagers([m]);
                            }}
                            className="text-[11px] text-zinc-400 hover:text-cyan-400 px-1.5 py-0.5 rounded hover:bg-zinc-800 transition-colors cursor-pointer"
                            title={`Filter only ${m}`}
                          >
                            only
                          </button>
                        </div>
                      );
                    })}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Queued Filter Toggle Pill */}
          {(queuedPackagesCount > 0 || activeFilter === "queued") && (
            <button
              type="button"
              disabled={isUpgrading}
              onClick={() => onFilterChange(activeFilter === "queued" ? "all" : "queued")}
              className={`px-2.5 py-1.5 rounded-md transition-colors flex items-center gap-1.5 text-xs font-mono disabled:opacity-50 cursor-pointer ${
                activeFilter === "queued"
                  ? "bg-cyan-950 text-cyan-200 border border-cyan-500 font-semibold"
                  : "text-cyan-300 hover:text-cyan-200 bg-cyan-950/30 hover:bg-cyan-950/60 border border-cyan-900/60"
              }`}
            >
              <ListFilter className="w-3.5 h-3.5 text-cyan-400" />
              <span>Queued ({queuedPackagesCount})</span>
            </button>
          )}
        </div>

        {/* Adaptive Contextual Selection Actions */}
        <div className="flex flex-wrap items-center gap-2 border-t sm:border-t-0 pt-2 sm:pt-0 border-zinc-800/80">
          {activeFilter === "queued" ? (
            <>
              <span className="text-[11px] font-mono text-zinc-300">
                {filteredSelected.length} queued
              </span>
              <button
                type="button"
                disabled={isUpgrading || filteredSelected.length === 0}
                onClick={onDeselectAll}
                className="text-[11px] font-mono text-zinc-300 hover:text-zinc-100 bg-zinc-950 hover:bg-zinc-800 border border-zinc-800 px-2.5 py-1 rounded-md transition-colors cursor-pointer disabled:opacity-40"
              >
                Clear Queue
              </button>
              {onUpgrade && (
                <button
                  type="button"
                  disabled={isUpgrading || filteredSelected.length === 0}
                  onClick={onUpgrade}
                  className="flex items-center gap-1.5 px-3 py-1 rounded-md bg-zinc-100 hover:bg-white text-zinc-950 text-[11px] font-semibold transition-colors shadow-sm cursor-pointer disabled:opacity-40"
                >
                  <ArrowUpCircle className="w-3.5 h-3.5" />
                  <span>Upgrade ({filteredSelected.length})</span>
                </button>
              )}
            </>
          ) : isFilterActive ? (
            <>
              <span className="text-[11px] font-mono text-zinc-300">
                {filteredSelected.length}/{filteredPackages.length} selected
              </span>
              <button
                type="button"
                disabled={isUpgrading}
                onClick={() => onSelectOnlyFiltered(filteredPackages.map((p) => p.id))}
                className="text-[11px] font-mono text-zinc-100 hover:text-white bg-zinc-800 hover:bg-zinc-700 border border-zinc-700 px-2.5 py-1 rounded-md transition-colors cursor-pointer font-medium disabled:opacity-40"
                title="Select only the visible filtered packages and clear others"
              >
                Select Visible ({filteredSelectable.length})
              </button>
              <button
                type="button"
                disabled={isUpgrading || filteredSelected.length === 0}
                onClick={() => onDeselectFiltered(filteredPackages.map((p) => p.id))}
                className="text-[11px] font-mono text-zinc-300 hover:text-zinc-100 bg-zinc-950 hover:bg-zinc-800 border border-zinc-800 px-2.5 py-1 rounded-md transition-colors cursor-pointer disabled:opacity-40"
              >
                Clear
              </button>
            </>
          ) : (
            <>
              {securityPackagesCount > 0 && (
                <button
                  type="button"
                  disabled={isUpgrading}
                  onClick={onSelectSecurity}
                  className="text-[11px] font-mono text-rose-300 hover:text-rose-100 bg-rose-950/40 hover:bg-rose-950/70 border border-rose-900/60 px-2.5 py-1 rounded-md transition-colors cursor-pointer disabled:opacity-40"
                >
                  Select CVEs ({securityPackagesCount})
                </button>
              )}
              <button
                type="button"
                disabled={isUpgrading}
                onClick={onSelectSafe}
                className="text-[11px] font-mono text-emerald-300 hover:text-emerald-100 bg-emerald-950/40 hover:bg-emerald-950/70 border border-emerald-900/60 px-2.5 py-1 rounded-md transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
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
                className="text-[11px] font-mono text-zinc-300 hover:text-zinc-100 bg-zinc-950 hover:bg-zinc-800 border border-zinc-800 px-2.5 py-1 rounded-md transition-colors cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
              >
                Clear
              </button>
            </>
          )}
        </div>
      </div>

      {/* Queued Review Banner */}
      {activeFilter === "queued" && filteredPackages.length > 0 && (
        <div className="bg-cyan-950/30 border border-cyan-800/60 rounded-lg p-3 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs font-mono animate-in fade-in">
          <div className="flex items-center gap-2.5 text-cyan-200">
            <ListFilter className="w-4 h-4 text-cyan-400 shrink-0" />
            <span>
              Queue Review: <strong>{filteredPackages.length}</strong>{" "}
              {filteredPackages.length === 1 ? "package" : "packages"} selected for upgrade across{" "}
              <strong>{grouped.length}</strong> {grouped.length === 1 ? "manager" : "managers"}.
            </span>
          </div>
          <div className="flex items-center gap-2 self-end sm:self-center">
            <button
              type="button"
              disabled={isUpgrading}
              onClick={() => onFilterChange("all")}
              className="px-2.5 py-1 rounded bg-zinc-900 hover:bg-zinc-800 text-zinc-300 hover:text-white border border-zinc-700/80 text-[11px] transition-colors cursor-pointer disabled:opacity-40"
            >
              Back to All
            </button>
          </div>
        </div>
      )}

      {/* Package Groups or Empty State */}
      {grouped.length === 0 ? (
        <div className="bg-zinc-900/40 border border-zinc-800/80 rounded-lg p-12 text-center">
          <ShieldCheck className="w-8 h-8 text-emerald-500/80 mx-auto mb-2" />
          <h3 className="text-sm font-semibold text-zinc-200">
            {activeFilter === "queued" ? "No Packages in Queue" : "System Protected & Up to Date"}
          </h3>
          <p className="text-xs text-zinc-400 mt-1 max-w-sm mx-auto">
            {activeFilter === "queued"
              ? "Your upgrade queue is empty. Choose packages from Safe, Vulnerabilities, or All to queue them."
              : search || activeFilter !== "all" || isManagerFiltered
              ? "No packages match the active search or filter criteria."
              : "No outdated packages or unpatched security advisories found across managers."}
          </p>
          {activeFilter === "queued" && (
            <button
              type="button"
              onClick={() => onFilterChange(securityPackagesCount > 0 ? "security" : "safe")}
              className="mt-3 px-3 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-zinc-200 text-xs font-mono transition-colors cursor-pointer"
            >
              Select Packages to Upgrade
            </button>
          )}
        </div>
      ) : (
        <div className="space-y-3 relative">
          {/* Active Execution Lockdown Overlay */}
          {isUpgrading && (
            <div className="absolute inset-0 bg-zinc-950/70 backdrop-blur-[1.5px] z-30 rounded-lg flex flex-col items-center justify-start pt-16 pointer-events-auto">
              <div className="bg-zinc-900 border border-zinc-700/80 rounded-xl p-5 shadow-2xl flex flex-col items-center gap-3 max-w-md text-center animate-in fade-in zoom-in-95 mx-4">
                <div className="p-3 bg-blue-950/80 border border-blue-800/80 rounded-full">
                  <Loader2 className="w-6 h-6 animate-spin text-blue-400" />
                </div>
                <div>
                  <h4 className="text-sm font-semibold font-mono text-zinc-100">
                    {operationTitle}
                  </h4>
                  <p className="text-xs text-zinc-300 mt-1 font-mono">
                    {operationSubtitle}
                  </p>
                </div>
                <span className="text-[11px] font-mono text-zinc-300 bg-zinc-950 px-3 py-1 rounded-md border border-zinc-800/80">
                  Interface locked — package selection disabled during execution
                </span>
              </div>
            </div>
          )}

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
                <div className="bg-zinc-950/80 border-b border-zinc-800/80 px-3.5 py-2 flex flex-wrap items-center justify-between gap-2 select-none">
                  <div className="flex items-center gap-2.5 min-w-0">
                    {/* Collapsible toggle */}
                    <button
                      type="button"
                      disabled={isUpgrading}
                      aria-label={collapsedManagers[manager] ? `Expand ${manager}` : `Collapse ${manager}`}
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
                      role="checkbox"
                      aria-checked={isAllSelected}
                      aria-label={isAllSelected ? `Deselect all ${manager} packages` : `Select all ${manager} packages`}
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
                        <Square className="w-4 h-4 text-zinc-500" />
                      )}
                    </button>

                    <div className="flex items-center gap-2 flex-wrap">
                      <span className="font-mono text-xs uppercase font-semibold text-zinc-100">
                        {manager}
                      </span>
                      <span className="text-[11px] font-mono text-zinc-300 bg-zinc-800/80 border border-zinc-700/50 px-1.5 py-0.5 rounded">
                        {selectedItems.length}/{items.length} selected
                      </span>

                      {/* Live manager status badge */}
                      {isManagerActive && (
                        <span className="flex items-center gap-1 text-[11px] font-mono text-blue-300 bg-blue-950 border border-blue-700 px-1.5 py-0.5 rounded">
                          <Loader2 className="w-3 h-3 animate-spin text-blue-400" />
                          Upgrading...
                        </span>
                      )}
                      {managerOutcome === "ok" && (
                        <span className="flex items-center gap-1 text-[11px] font-mono text-emerald-300 bg-emerald-950 border border-emerald-700 px-1.5 py-0.5 rounded font-semibold">
                          <CheckCircle2 className="w-3 h-3 text-emerald-400" />
                          Done
                        </span>
                      )}
                      {managerOutcome === "failed" && (
                        <span className="flex items-center gap-1 text-[11px] font-mono text-rose-300 bg-rose-950 border border-rose-700 px-1.5 py-0.5 rounded font-semibold">
                          Failed
                        </span>
                      )}
                    </div>
                  </div>

                  <span className="text-[11px] font-mono text-zinc-400">
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
                            className={`px-3.5 py-2.5 flex flex-col sm:flex-row sm:items-center justify-between gap-2 sm:gap-4 transition-colors ${
                              isUpgrading
                                ? "cursor-wait"
                                : pinned
                                ? "opacity-60 bg-zinc-950/40 cursor-not-allowed"
                                : "hover:bg-zinc-800/30 cursor-pointer"
                            } ${selected ? "bg-zinc-800/20" : ""} ${
                              hasVuln ? "border-l-2 border-l-rose-500 bg-rose-950/15" : ""
                            } ${isPkgActive ? "bg-blue-950/30 border-l-2 border-l-blue-400" : ""}`}
                          >
                            {/* Left: Checkbox + Name + Source Link + Pin + Description + CVE Summary */}
                            <div className="flex-1 flex items-start sm:items-center gap-2.5 min-w-0">
                              <button
                                type="button"
                                role="checkbox"
                                aria-checked={selected}
                                aria-label={`Select ${name}`}
                                disabled={pinned || isUpgrading}
                                onClick={(e) => {
                                  e.stopPropagation();
                                  if (!pinned && !isUpgrading) onTogglePackage(id);
                                }}
                                className="mt-0.5 sm:mt-0 text-zinc-400 hover:text-zinc-200 transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-cyan-400 rounded shrink-0 disabled:cursor-not-allowed"
                              >
                                {isPkgActive ? (
                                  <Loader2 className="w-4 h-4 animate-spin text-blue-400" />
                                ) : isPkgDone ? (
                                  <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                                ) : selected ? (
                                  <CheckSquare className="w-4 h-4 text-zinc-100" />
                                ) : (
                                  <Square className="w-4 h-4 text-zinc-500" />
                                )}
                              </button>

                              <div className="min-w-0 flex-1 flex flex-col justify-center">
                                <div className="flex items-center gap-2 min-w-0">
                                  <span className="font-mono text-xs font-semibold text-zinc-100 truncate" title={name}>
                                    {name}
                                  </span>

                                  {pkg.source_url && (
                                    <a
                                      href={pkg.source_url}
                                      target="_blank"
                                      rel="noopener noreferrer"
                                      aria-label={`Open ${name} upstream source`}
                                      onClick={(e) => e.stopPropagation()}
                                      title={`Open upstream repository/registry (${pkg.source_url})`}
                                      className="text-zinc-400 hover:text-zinc-200 transition-colors p-0.5 inline-flex items-center shrink-0 cursor-pointer"
                                    >
                                      <ExternalLink className="w-3 h-3" />
                                    </a>
                                  )}

                                  <button
                                    type="button"
                                    disabled={isUpgrading}
                                    title={pinned ? "Unpin package" : "Pin package (lock current version)"}
                                    onClick={(e) => {
                                      e.stopPropagation();
                                      if (!isUpgrading) onTogglePin(id);
                                    }}
                                    className={`flex items-center gap-1 text-[11px] font-mono px-1.5 py-0.5 rounded transition-colors shrink-0 disabled:opacity-40 disabled:cursor-not-allowed ${
                                      pinned
                                        ? "bg-amber-950/90 text-amber-300 border border-amber-700 hover:bg-amber-900"
                                        : "text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/80 border border-transparent"
                                    }`}
                                  >
                                    <Pin className="w-2.5 h-2.5" />
                                    <span>{pinned ? "Pinned" : "Pin"}</span>
                                  </button>
                                </div>

                                {pkg.description && pkg.description !== vulns[0]?.summary && (
                                  <p
                                    className="text-xs font-sans text-zinc-400 truncate mt-0.5 leading-snug"
                                    title={pkg.description}
                                  >
                                    {pkg.description}
                                  </p>
                                )}

                                {hasVuln && vulns.length > 0 && vulns[0].summary && (
                                  <p
                                    className="text-xs font-mono text-rose-300 font-medium mt-0.5 leading-snug flex items-center gap-1.5 min-w-0"
                                    title={`${vulns[0].id}: ${vulns[0].summary}`}
                                  >
                                    <span className="text-rose-400 font-bold shrink-0">
                                      {vulns[0].id.split(" ")[0]}:
                                    </span>
                                    <span className="truncate min-w-0 flex-1 font-sans">{vulns[0].summary}</span>
                                    {vulns.length > 1 && (
                                      <span className="text-rose-400/90 shrink-0 text-[11px] font-mono">
                                        (+{vulns.length - 1} more)
                                      </span>
                                    )}
                                  </p>
                                )}
                              </div>
                            </div>

                            {/* Right: Versions + Single Authority Badge */}
                            <div className="pl-6 sm:pl-0 flex items-center justify-between sm:justify-end gap-3 shrink-0">
                              {/* Version flow */}
                              <div className="flex items-center gap-1.5 font-mono text-xs tabular-nums">
                                <span className="text-zinc-400 max-w-[14ch] sm:max-w-[20ch] truncate" title={cur}>
                                  {cur}
                                </span>
                                <span className="text-zinc-400 select-none">→</span>
                                <span className="text-zinc-100 font-semibold max-w-[14ch] sm:max-w-[20ch] truncate" title={lat}>
                                  {lat}
                                </span>
                              </div>

                              {/* Single Badge Authority */}
                              <div className="min-w-[7.5rem] sm:min-w-[8.5rem] shrink-0 flex justify-end">
                                {isPkgActive ? (
                                  <span className="inline-flex items-center gap-1 text-[11px] font-mono font-bold px-2 py-0.5 rounded bg-blue-950 text-blue-300 border border-blue-700 uppercase tracking-wider whitespace-nowrap">
                                    <Loader2 className="w-2.5 h-2.5 animate-spin" /> RUNNING
                                  </span>
                                ) : isPkgDone ? (
                                  <span className="inline-flex items-center text-[11px] font-mono font-bold px-2 py-0.5 rounded bg-emerald-950 text-emerald-300 border border-emerald-700 uppercase tracking-wider whitespace-nowrap">
                                    UPGRADED ✓
                                  </span>
                                ) : isPkgFailed ? (
                                  <span className="inline-flex items-center text-[11px] font-mono font-bold px-2 py-0.5 rounded bg-rose-950 text-rose-300 border border-rose-700 uppercase tracking-wider whitespace-nowrap">
                                    FAILED ✗
                                  </span>
                                ) : hasVuln ? (
                                  <button
                                    type="button"
                                    aria-expanded={Boolean(isExpanded)}
                                    onClick={(e) => toggleExpand(id, e)}
                                    className={`inline-flex items-center gap-1 text-[11px] font-mono font-bold px-2 py-0.5 rounded border cursor-pointer uppercase tracking-wider whitespace-nowrap transition-colors ${
                                      maxSev === "CRITICAL"
                                        ? "bg-rose-950 text-rose-300 border-rose-600 hover:bg-rose-900"
                                        : maxSev === "HIGH"
                                        ? "bg-orange-950 text-orange-300 border-orange-700 hover:bg-orange-900"
                                        : maxSev === "MEDIUM" || maxSev === "MODERATE"
                                        ? "bg-amber-950/70 text-amber-300 border-amber-700/80 hover:bg-amber-900"
                                        : "bg-zinc-800 text-zinc-300 border-zinc-700 hover:bg-zinc-700"
                                    }`}
                                    title="Click to expand security advisory details"
                                  >
                                    {maxSev === "CRITICAL" ? (
                                      <Flame className="w-3 h-3 text-rose-400 shrink-0" />
                                    ) : (
                                      <AlertTriangle className="w-3 h-3 text-orange-400 shrink-0" />
                                    )}
                                    <span>{maxSev} ({vulns.length})</span>
                                    {isExpanded ? (
                                      <ChevronDown className="w-3 h-3 shrink-0" />
                                    ) : (
                                      <ChevronRight className="w-3 h-3 shrink-0" />
                                    )}
                                  </button>
                                ) : changeType === "major" ? (
                                  <span className="inline-flex items-center gap-1 text-[11px] font-mono font-semibold px-2 py-0.5 rounded bg-amber-950/60 text-amber-300 border border-amber-800/60 uppercase tracking-wider whitespace-nowrap">
                                    <AlertTriangle className="w-3 h-3 text-amber-400 shrink-0" />
                                    MAJOR
                                  </span>
                                ) : changeType === "minor" ? (
                                  <span className="inline-flex items-center text-[11px] font-mono font-medium px-2 py-0.5 rounded bg-zinc-800 text-zinc-300 border border-zinc-700 uppercase tracking-wider whitespace-nowrap">
                                    MINOR
                                  </span>
                                ) : changeType === "patch" ? (
                                  <span className="inline-flex items-center text-[11px] font-mono font-medium px-2 py-0.5 rounded bg-emerald-950/60 text-emerald-400 border border-emerald-800/60 uppercase tracking-wider whitespace-nowrap">
                                    PATCH
                                  </span>
                                ) : (
                                  <span className="inline-flex items-center text-[11px] font-mono px-2 py-0.5 rounded bg-zinc-800 text-zinc-300 border border-zinc-700 uppercase tracking-wider whitespace-nowrap">
                                    UPDATE
                                  </span>
                                )}
                              </div>
                            </div>
                          </div>

                          {/* Expandable CVE details */}
                          {hasVuln && isExpanded && (
                            <div className="bg-rose-950/20 border-t border-rose-900/40 px-4 sm:px-6 py-3 text-xs font-mono space-y-2">
                              <div className="text-[11px] font-semibold text-rose-400 uppercase tracking-wider flex items-center justify-between flex-wrap gap-2">
                                <span>Security Advisory Intelligence ({vulns.length}):</span>
                                <span className="text-zinc-400 font-normal">Click CVE/GHSA ID to inspect upstream advisory</span>
                              </div>
                              {vulns.map((v: Vulnerability, i: number) => {
                                const fixedIn = v.fixed_in;
                                const isMitigated = v.is_mitigated;
                                const canonicalId = v.id.split(" ")[0];
                                const osvUrl = `https://osv.dev/vulnerability/${encodeURIComponent(canonicalId)}`;

                                return (
                                  <div
                                    key={i}
                                    className="flex flex-col gap-1.5 text-zinc-300 bg-rose-950/30 p-2.5 rounded-md border border-rose-900/40"
                                  >
                                    <div className="flex items-center justify-between gap-2 flex-wrap">
                                      <div className="flex items-center gap-2 flex-wrap">
                                        <a
                                          href={osvUrl}
                                          target="_blank"
                                          rel="noopener noreferrer"
                                          onClick={(e) => e.stopPropagation()}
                                          className="text-rose-300 hover:text-rose-100 font-bold inline-flex items-center gap-1 underline underline-offset-2 decoration-rose-500/50"
                                          title={`View ${canonicalId} details on OSV.dev`}
                                        >
                                          <span>{v.id}</span>
                                          <ExternalLink className="w-3 h-3 shrink-0" />
                                        </a>
                                        {v.source && (
                                          <span className="text-[11px] px-1.5 py-0.5 rounded bg-zinc-900 border border-zinc-700/80 text-zinc-300">
                                            {v.source}
                                          </span>
                                        )}
                                        <span
                                          className={`text-[11px] px-1.5 py-0.5 rounded font-bold uppercase border ${
                                            v.severity === "CRITICAL"
                                              ? "bg-rose-950 text-rose-200 border-rose-600"
                                              : v.severity === "HIGH"
                                              ? "bg-orange-950 text-orange-200 border-orange-700"
                                              : v.severity === "MEDIUM" || v.severity === "MODERATE"
                                              ? "bg-amber-950 text-amber-200 border-amber-700"
                                              : "bg-zinc-800 text-zinc-300 border-zinc-700"
                                          }`}
                                        >
                                          {v.score ? `${v.severity} (${v.score})` : v.severity || "HIGH"}
                                        </span>
                                      </div>

                                      <div className="shrink-0 flex items-center gap-1.5 text-[11px]">
                                        {isMitigated === false ? (
                                          <span className="bg-rose-900/90 text-rose-100 border border-rose-600 px-2 py-0.5 rounded font-bold">
                                            Zero-Day (Unmitigated)
                                          </span>
                                        ) : fixedIn ? (
                                          <span className="bg-emerald-950 text-emerald-300 border border-emerald-700 px-2 py-0.5 rounded font-mono">
                                            Fixed in {fixedIn}
                                          </span>
                                        ) : null}
                                      </div>
                                    </div>

                                    <p className="text-zinc-200 text-xs font-sans leading-relaxed break-words">
                                      {v.summary || "Security vulnerability fix available in upstream release"}
                                    </p>
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
