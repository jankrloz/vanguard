import { useState, useEffect, useCallback, useMemo } from "react";
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

export type OperationType = "upgrade" | "clean" | "topgrade" | null;

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
  const [activeOperation, setActiveOperation] = useState<OperationType>(null);
  const [confirmAction, setConfirmAction] = useState<"clean" | "topgrade" | null>(null);
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
        setActiveOperation(null);
        setActiveManager(null);
        if (event.type === "complete") {
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
    setActiveOperation("upgrade");
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
      setActiveOperation(null);
    }
  };

  const executeClean = async () => {
    setConfirmAction(null);
    if (isUpgrading || isScanning) return;
    setIsUpgrading(true);
    setActiveOperation("clean");
    setIsLogOpen(true);
    setEvents([]);

    try {
      await triggerClean();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Clean request failed";
      setError(msg);
      setIsUpgrading(false);
      setActiveOperation(null);
    }
  };

  const executeTopgrade = async () => {
    setConfirmAction(null);
    if (isUpgrading || isScanning) return;
    setIsUpgrading(true);
    setActiveOperation("topgrade");
    setIsLogOpen(true);
    setEvents([]);

    try {
      await triggerTopgrade();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : "Topgrade sweep request failed";
      setError(msg);
      setIsUpgrading(false);
      setActiveOperation(null);
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

  const { selectedCount, queuedCveCount, queuedSafeCount } = useMemo(() => {
    let total = 0;
    let cves = 0;
    let safe = 0;
    for (const p of packages) {
      if (p.selected && !p.pinned) {
        total++;
        if (p.has_vulnerability) {
          cves++;
        } else if (p.safe) {
          safe++;
        }
      }
    }
    return { selectedCount: total, queuedCveCount: cves, queuedSafeCount: safe };
  }, [packages]);

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

  const showQueueDock = selectedCount > 0 && !isUpgrading && activeFilter !== "queued";
  const showLogTrigger = !isLogOpen && events.length > 0;

  return (
    <div className="min-h-screen bg-[#09090b] text-zinc-100 flex flex-col selection:bg-zinc-800">
      <Header
        status={status}
        selectedCount={selectedCount}
        securityCount={summary.security_fix_count}
        isScanning={isScanning}
        isUpgrading={isUpgrading}
        onScan={() => loadData(true)}
        onUpgrade={handleUpgrade}
        onOpenReview={() => setActiveFilter("queued")}
        onClean={() => setConfirmAction("clean")}
        onTopgrade={() => setConfirmAction("topgrade")}
        onOpenHistory={() => setIsHistoryOpen(true)}
      />

      <main className={`flex-1 max-w-7xl w-full mx-auto px-4 sm:px-6 py-5 space-y-4 ${showQueueDock || showLogTrigger ? "pb-24" : ""}`}>
        {/* Confirmation Banner for System-Wide Actions */}
        {confirmAction && !isUpgrading && (
          <div
            role="alertdialog"
            aria-labelledby="confirm-action-title"
            className="bg-amber-950/60 border border-amber-700/80 rounded-lg p-3.5 text-xs flex flex-col sm:flex-row sm:items-center justify-between gap-3 animate-in fade-in"
          >
            <div className="space-y-0.5">
              <div id="confirm-action-title" className="font-mono font-semibold text-amber-200">
                {confirmAction === "topgrade"
                  ? "Confirm Deep System Maintenance Sweep (Topgrade)?"
                  : "Confirm Package Cache Cleanup?"}
              </div>
              <p className="text-zinc-300">
                {confirmAction === "topgrade"
                  ? "Topgrade will update Git repositories, language toolchains, shell frameworks, and system caches."
                  : "This will purge downloaded package manager caches, orphaned archives, and old build artifacts."}
              </p>
            </div>
            <div className="flex items-center gap-2 shrink-0 self-end sm:self-center font-mono">
              <button
                type="button"
                onClick={() => setConfirmAction(null)}
                className="px-3 py-1.5 rounded-md bg-zinc-900 hover:bg-zinc-800 text-zinc-300 border border-zinc-700 text-xs cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={confirmAction === "topgrade" ? executeTopgrade : executeClean}
                className="px-3 py-1.5 rounded-md bg-amber-400 hover:bg-amber-300 text-zinc-950 font-semibold text-xs cursor-pointer"
              >
                {confirmAction === "topgrade" ? "Run Topgrade Sweep" : "Clean Caches"}
              </button>
            </div>
          </div>
        )}

        {/* Error notification */}
        {error && (
          <div
            role="alert"
            className="bg-rose-950/70 border border-rose-800/80 rounded-lg p-3 text-xs text-rose-200 flex flex-wrap items-center justify-between gap-2"
          >
            <span className="break-words">{error}</span>
            <div className="flex items-center gap-2 shrink-0 font-mono">
              <button
                type="button"
                onClick={() => loadData(true)}
                className="px-2.5 py-1 rounded bg-rose-900/80 hover:bg-rose-800 text-rose-100 border border-rose-700 text-[11px] cursor-pointer"
              >
                Retry Scan
              </button>
              <button
                type="button"
                onClick={() => setError(null)}
                className="text-rose-300 hover:text-rose-100 text-[11px] px-2 py-1 cursor-pointer"
              >
                Dismiss
              </button>
            </div>
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
          activeOperation={activeOperation}
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

      {/* Unified Responsive Floating Bottom Bar (prevents overlap between Queue Dock and Log Trigger) */}
      {(showQueueDock || showLogTrigger) && (
        <div className="fixed bottom-4 inset-x-0 z-30 px-3 flex flex-wrap items-center justify-center gap-2.5 pointer-events-none">
          {showQueueDock && (
            <div className="pointer-events-auto font-mono text-xs bg-zinc-950/95 border border-zinc-700/80 shadow-2xl rounded-full px-3.5 py-2 flex flex-wrap items-center justify-center gap-2.5 max-w-full backdrop-blur-md">
              <div className="flex items-center gap-2">
                <span className="w-2 h-2 rounded-full bg-cyan-400 shrink-0"></span>
                <span className="text-zinc-100 font-semibold whitespace-nowrap">
                  {selectedCount} queued
                </span>
                <span className="text-zinc-600 select-none">|</span>
                <div className="flex items-center gap-1.5 text-[11px] whitespace-nowrap">
                  {queuedCveCount > 0 && (
                    <span className="text-rose-400 font-medium">
                      {queuedCveCount} CVEs
                    </span>
                  )}
                  <span className="text-zinc-300">
                    {queuedSafeCount} Safe
                  </span>
                </div>
              </div>

              <div className="flex items-center gap-2 pl-2 border-l border-zinc-800">
                <button
                  type="button"
                  onClick={() => setActiveFilter("queued")}
                  className="px-2.5 py-1 rounded-full bg-zinc-900 hover:bg-zinc-800 text-zinc-200 hover:text-white border border-zinc-700/80 text-[11px] font-medium transition-colors cursor-pointer whitespace-nowrap"
                >
                  Review Queue
                </button>
                <button
                  type="button"
                  onClick={handleUpgrade}
                  className="px-3 py-1 rounded-full bg-zinc-100 hover:bg-white text-zinc-950 text-[11px] font-semibold transition-colors shadow-sm cursor-pointer whitespace-nowrap"
                >
                  Upgrade Now
                </button>
              </div>
            </div>
          )}

          {showLogTrigger && (
            <button
              type="button"
              onClick={() => setIsLogOpen(true)}
              className="pointer-events-auto font-mono text-xs bg-zinc-900/95 hover:bg-zinc-800 text-zinc-200 border border-zinc-700/80 px-3.5 py-2 rounded-full shadow-lg flex items-center gap-2 transition-all cursor-pointer whitespace-nowrap backdrop-blur-md"
            >
              <span className="w-2 h-2 rounded-full bg-emerald-400 shrink-0"></span>
              <span>View Execution Logs ({events.length})</span>
            </button>
          )}
        </div>
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
