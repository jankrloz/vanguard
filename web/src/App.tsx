import { useState, useEffect, useCallback } from "react";
import { Header } from "./components/header";
import { MetricBar } from "./components/metric-bar";
import { PackageTable } from "./components/package-table";
import { LogDrawer } from "./components/log-drawer";
import { HistoryModal } from "./components/history-modal";
import {
  fetchStatus,
  fetchOutdated,
  triggerUpgrade,
  triggerClean,
  triggerTopgrade,
  togglePin,
  subscribeToEvents,
} from "./lib/api";
import type { Package, ScanSummary, SystemStatus, UpgradeEvent, FilterType } from "./lib/types";

export function App() {
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [packages, setPackages] = useState<Package[]>([]);
  const [activeFilter, setActiveFilter] = useState<FilterType>("all");
  const [summary, setSummary] = useState<ScanSummary>({
    total_packages: 0,
    safe_count: 0,
    major_count: 0,
    pinned_count: 0,
    security_fix_count: 0,
  });

  const [isScanning, setIsScanning] = useState(true);
  const [isUpgrading, setIsUpgrading] = useState(false);
  const [activeManager, setActiveManager] = useState<string | null>(null);
  const [completedManagers, setCompletedManagers] = useState<Record<string, "ok" | "failed">>({});
  const [events, setEvents] = useState<UpgradeEvent[]>([]);
  const [isLogOpen, setIsLogOpen] = useState(false);
  const [isHistoryOpen, setIsHistoryOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load status and packages
  const loadData = useCallback(async (refresh = false) => {
    setIsScanning(true);
    setError(null);
    try {
      const [sysStatus, scanResult] = await Promise.all([
        fetchStatus().catch(() => null),
        fetchOutdated(refresh),
      ]);

      if (sysStatus) setStatus(sysStatus);

      // Smart selection: prioritize security fixes if present.
      // Never blindly select 1,600+ packages on initial scan.
      const hasSecurity = (scanResult.packages || []).some((p) => p.has_vulnerability);
      const pkgs: Package[] = (scanResult.packages || []).map((p) => ({
        ...p,
        selected: hasSecurity
          ? Boolean(p.has_vulnerability && !p.pinned)
          : Boolean(p.safe && !p.pinned),
      }));

      setPackages(pkgs);
      const calculatedSummary = scanResult.summary || {
        total_packages: pkgs.length,
        safe_count: pkgs.filter((p) => p.safe).length,
        major_count: pkgs.filter((p) => p.change_type === "major").length,
        pinned_count: pkgs.filter((p) => p.pinned).length,
        security_fix_count: pkgs.filter((p) => p.has_vulnerability).length,
      };
      setSummary(calculatedSummary);

      // Default filter priority: Critical/CVEs first, then Safe, else All
      setActiveFilter((current) => {
        if (current === "all" || current === "security" || current === "safe") {
          return hasSecurity ? "security" : "safe";
        }
        return current;
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Failed to load package information";
      setError(msg);
    } finally {
      setIsScanning(false);
    }
  }, []);

  useEffect(() => {
    loadData(false);

    // Subscribe to SSE log stream
    const unsubscribe = subscribeToEvents((event) => {
      setEvents((prev) => [...prev, event]);
      setIsLogOpen(true);

      if (event.type === "start") {
        setIsUpgrading(true);
        setActiveManager(null);
        setCompletedManagers({});
      } else if (event.type === "package_start") {
        if (event.package) setActiveManager(event.package);
      } else if (event.type === "package_done") {
        if (event.package) {
          setCompletedManagers((prev) => ({
            ...prev,
            [event.package!]: event.status === "failed" ? "failed" : "ok",
          }));
        }
      } else if (event.type === "complete" || event.type === "error") {
        setIsUpgrading(false);
        setActiveManager(null);
        if (event.type === "complete") {
          // Seamlessly auto-refresh packages to remove upgraded entries from list
          setTimeout(() => {
            loadData(true);
          }, 1200);
        }
      }
    });

    return () => {
      unsubscribe();
    };
  }, [loadData]);

  // Toggle selection
  const handleTogglePackage = (id: string) => {
    if (isUpgrading) return;
    setPackages((prev) =>
      prev.map((p) => {
        if (p.id === id) {
          return { ...p, selected: !p.selected };
        }
        return p;
      })
    );
  };

  const handleToggleManagerSelection = (manager: string) => {
    if (isUpgrading) return;
    setPackages((prev) => {
      const managerPkgs = prev.filter((p) => p.manager === manager && !p.pinned);
      const allSelected = managerPkgs.length > 0 && managerPkgs.every((p) => p.selected);
      return prev.map((p) => {
        if (p.manager === manager && !p.pinned) {
          return { ...p, selected: !allSelected };
        }
        return p;
      });
    });
  };

  const handleSelectSafe = () => {
    if (isUpgrading) return;
    setPackages((prev) =>
      prev.map((p) => ({ ...p, selected: p.safe && !p.pinned }))
    );
  };

  const handleSelectSecurity = () => {
    if (isUpgrading) return;
    setPackages((prev) =>
      prev.map((p) => ({ ...p, selected: Boolean(p.has_vulnerability && !p.pinned) }))
    );
  };

  const handleSelectAll = () => {
    if (isUpgrading) return;
    setPackages((prev) =>
      prev.map((p) => ({ ...p, selected: !p.pinned }))
    );
  };

  const handleDeselectAll = () => {
    if (isUpgrading) return;
    setPackages((prev) => prev.map((p) => ({ ...p, selected: false })));
  };

  const handleSelectOnlyFiltered = (filteredIds: string[]) => {
    if (isUpgrading) return;
    const targetSet = new Set(filteredIds);
    setPackages((prev) =>
      prev.map((p) => ({
        ...p,
        selected: targetSet.has(p.id) && !p.pinned,
      }))
    );
  };

  const handleDeselectFiltered = (filteredIds: string[]) => {
    if (isUpgrading) return;
    const targetSet = new Set(filteredIds);
    setPackages((prev) =>
      prev.map((p) => (targetSet.has(p.id) ? { ...p, selected: false } : p))
    );
  };

  const handleUpgrade = async () => {
    const selected = packages.filter((p) => p.selected && !p.pinned);
    if (selected.length === 0 || isUpgrading || isScanning) return;

    setIsUpgrading(true);
    setIsLogOpen(true);
    setActiveManager(null);
    setCompletedManagers({});
    setEvents([]);

    try {
      const ids = selected.map((p) => p.id);
      await triggerUpgrade(ids);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Upgrade request failed";
      setError(msg);
      setIsUpgrading(false);
    }
  };

  const handleClean = async () => {
    if (isUpgrading || isScanning) return;
    setIsUpgrading(true);
    setIsLogOpen(true);
    setEvents([]);

    try {
      await triggerClean();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Clean request failed";
      setError(msg);
      setIsUpgrading(false);
    }
  };

  const handleTopgrade = async () => {
    if (isUpgrading || isScanning) return;
    setIsUpgrading(true);
    setIsLogOpen(true);
    setEvents([]);

    try {
      await triggerTopgrade();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Topgrade sweep request failed";
      setError(msg);
      setIsUpgrading(false);
    }
  };

  const handleTogglePin = async (id: string) => {
    if (isUpgrading) return;
    try {
      const res = await togglePin(id);
      setPackages((prev) =>
        prev.map((p) => {
          if (p.id === id) {
            return {
              ...p,
              pinned: res.pinned,
              selected: res.pinned ? false : p.selected,
            };
          }
          return p;
        })
      );
      setSummary((prev) => ({
        ...prev,
        pinned_count: res.pinned ? prev.pinned_count + 1 : Math.max(0, prev.pinned_count - 1),
      }));
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Failed to toggle package pin";
      setError(msg);
    }
  };

  const selectedCount = packages.filter((p) => p.selected && !p.pinned).length;

  // Keyboard shortcut: Cmd/Ctrl + Enter triggers upgrade
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
        if (selectedCount > 0 && !isUpgrading && !isScanning) {
          e.preventDefault();
          handleUpgrade();
        }
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [selectedCount, isUpgrading, isScanning, packages]);

  return (
    <div className="min-h-screen bg-[#0c0d0e] text-zinc-100 flex flex-col selection:bg-zinc-800">
      <Header
        status={status}
        selectedCount={selectedCount}
        securityCount={summary.security_fix_count}
        isScanning={isScanning}
        isUpgrading={isUpgrading}
        onScan={() => loadData(true)}
        onUpgrade={handleUpgrade}
        onOpenReview={() => setActiveFilter("queued")}
        onClean={handleClean}
        onTopgrade={handleTopgrade}
        onOpenHistory={() => setIsHistoryOpen(true)}
      />

      <main className="flex-1 max-w-7xl w-full mx-auto px-6 py-5 space-y-4">
        {/* Error notification */}
        {error && (
          <div className="bg-rose-950/70 border border-rose-800/80 rounded-lg p-3 text-xs text-rose-300 flex items-center justify-between">
            <span>{error}</span>
            <button
              type="button"
              onClick={() => setError(null)}
              className="text-rose-400 hover:text-rose-200 font-mono text-[11px] cursor-pointer"
            >
              Dismiss
            </button>
          </div>
        )}

        {/* Metrics Strip with interactive filter triggers */}
        <MetricBar
          summary={summary}
          selectedCount={selectedCount}
          activeFilter={activeFilter}
          isScanning={isScanning}
          isUpgrading={isUpgrading}
          onFilterChange={setActiveFilter}
        />

        {/* Package Table & Controls */}
        <PackageTable
          packages={packages}
          activeFilter={activeFilter}
          isScanning={isScanning}
          isUpgrading={isUpgrading}
          activeManager={activeManager}
          completedManagers={completedManagers}
          onFilterChange={setActiveFilter}
          onTogglePackage={handleTogglePackage}
          onTogglePin={handleTogglePin}
          onSelectSafe={handleSelectSafe}
          onSelectSecurity={handleSelectSecurity}
          onSelectAll={handleSelectAll}
          onDeselectAll={handleDeselectAll}
          onSelectOnlyFiltered={handleSelectOnlyFiltered}
          onDeselectFiltered={handleDeselectFiltered}
          onToggleManagerSelection={handleToggleManagerSelection}
          onUpgrade={handleUpgrade}
        />
      </main>

      {/* Persistent Floating Queue Review Dock when items are selected */}
      {selectedCount > 0 && !isUpgrading && activeFilter !== "queued" && (
        <div className="fixed bottom-4 left-1/2 -translate-x-1/2 z-30 font-mono text-xs bg-zinc-950/90 border border-zinc-700/80 shadow-2xl rounded-full px-4 py-2 flex items-center gap-3 backdrop-blur-md animate-in slide-in-from-bottom-3 duration-150">
          <div className="flex items-center gap-2">
            <span className="w-2 h-2 rounded-full bg-cyan-400 animate-pulse"></span>
            <span className="text-zinc-200 font-semibold">
              {selectedCount} queued
            </span>
            <span className="text-zinc-600">|</span>
            <div className="flex items-center gap-1.5 text-[11px]">
              {packages.filter((p) => p.selected && p.has_vulnerability).length > 0 && (
                <span className="text-rose-400 font-medium">
                  {packages.filter((p) => p.selected && p.has_vulnerability).length} CVEs
                </span>
              )}
              <span className="text-zinc-400">
                {packages.filter((p) => p.selected && !p.has_vulnerability && p.safe).length} Safe
              </span>
            </div>
          </div>

          <div className="flex items-center gap-2 pl-2 border-l border-zinc-800">
            <button
              type="button"
              onClick={() => setActiveFilter("queued")}
              className="px-2.5 py-1 rounded-full bg-zinc-900 hover:bg-zinc-800 text-zinc-300 hover:text-white border border-zinc-700/80 text-[11px] font-medium transition-colors cursor-pointer"
            >
              Review Queue
            </button>
            <button
              type="button"
              onClick={handleUpgrade}
              className="px-3 py-1 rounded-full bg-zinc-100 hover:bg-white text-zinc-950 text-[11px] font-semibold transition-colors shadow-sm cursor-pointer"
            >
              Upgrade Now
            </button>
          </div>
        </div>
      )}

      {/* Floating Log Drawer Trigger when closed */}
      {!isLogOpen && events.length > 0 && (
        <button
          type="button"
          onClick={() => setIsLogOpen(true)}
          className="fixed bottom-4 right-6 z-30 font-mono text-xs bg-zinc-900 hover:bg-zinc-800 text-zinc-300 border border-zinc-700/80 px-3 py-1.5 rounded-full shadow-lg flex items-center gap-2 transition-all cursor-pointer"
        >
          <span className="w-2 h-2 rounded-full bg-emerald-400"></span>
          <span>View Execution Logs ({events.length})</span>
        </button>
      )}

      {/* Slide-over Log Drawer */}
      <LogDrawer
        isOpen={isLogOpen}
        isUpgrading={isUpgrading}
        events={events}
        onClose={() => setIsLogOpen(false)}
        onClear={() => setEvents([])}
      />

      {/* Audit History Modal */}
      <HistoryModal
        isOpen={isHistoryOpen}
        onClose={() => setIsHistoryOpen(false)}
      />
    </div>
  );
}

export default App;
