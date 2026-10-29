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
import type { Package, ScanSummary, SystemStatus, UpgradeEvent } from "./lib/types";

export function App() {
  const [status, setStatus] = useState<SystemStatus | null>(null);
  const [packages, setPackages] = useState<Package[]>([]);
  const [summary, setSummary] = useState<ScanSummary>({
    total_packages: 0,
    safe_count: 0,
    major_count: 0,
    pinned_count: 0,
    security_fix_count: 0,
  });

  const [isScanning, setIsScanning] = useState(false);
  const [isUpgrading, setIsUpgrading] = useState(false);
  const [events, setEvents] = useState<UpgradeEvent[]>([]);
  const [isLogOpen, setIsLogOpen] = useState(false);
  const [isHistoryOpen, setIsHistoryOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Load status and packages on mount
  const loadData = useCallback(async (refresh = false) => {
    setIsScanning(true);
    setError(null);
    try {
      const [sysStatus, scanResult] = await Promise.all([
        fetchStatus().catch(() => null),
        fetchOutdated(refresh),
      ]);

      if (sysStatus) setStatus(sysStatus);

      // Normalize package selection: preselect safe OR vulnerable packages
      const pkgs: Package[] = (scanResult.packages || []).map((p) => ({
        ...p,
        selected: Boolean((p.safe || p.has_vulnerability) && !p.pinned),
      }));

      setPackages(pkgs);
      setSummary(scanResult.summary || {
        total_packages: pkgs.length,
        safe_count: pkgs.filter((p) => p.safe).length,
        major_count: pkgs.filter((p) => p.change_type === "major").length,
        pinned_count: pkgs.filter((p) => p.pinned).length,
        security_fix_count: pkgs.filter((p) => p.has_vulnerability).length,
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
      } else if (event.type === "complete" || event.type === "error") {
        setIsUpgrading(false);
      }
    });

    return () => {
      unsubscribe();
    };
  }, [loadData]);

  // Toggle selection
  const handleTogglePackage = (id: string) => {
    setPackages((prev) =>
      prev.map((p) => {
        if (p.id === id) {
          return { ...p, selected: !p.selected };
        }
        return p;
      })
    );
  };

  const handleSelectSafe = () => {
    setPackages((prev) =>
      prev.map((p) => ({ ...p, selected: p.safe && !p.pinned }))
    );
  };

  const handleSelectSecurity = () => {
    setPackages((prev) =>
      prev.map((p) => ({ ...p, selected: Boolean(p.has_vulnerability && !p.pinned) }))
    );
  };

  const handleSelectAll = () => {
    setPackages((prev) =>
      prev.map((p) => ({ ...p, selected: !p.pinned }))
    );
  };

  const handleDeselectAll = () => {
    setPackages((prev) => prev.map((p) => ({ ...p, selected: false })));
  };

  const handleUpgrade = async () => {
    const selected = packages.filter((p) => p.selected);
    if (selected.length === 0) return;

    setIsUpgrading(true);
    setIsLogOpen(true);
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

  const selectedCount = packages.filter((p) => p.selected).length;

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
        onClean={handleClean}
        onTopgrade={handleTopgrade}
        onOpenHistory={() => setIsHistoryOpen(true)}
      />

      <main className="flex-1 max-w-7xl w-full mx-auto px-6 py-6 space-y-6">
        {/* Error notification */}
        {error && (
          <div className="bg-rose-950/70 border border-rose-800/80 rounded-lg p-3 text-xs text-rose-300 flex items-center justify-between">
            <span>{error}</span>
            <button
              onClick={() => setError(null)}
              className="text-rose-400 hover:text-rose-200 font-mono text-[11px]"
            >
              Dismiss
            </button>
          </div>
        )}

        {/* Metrics Strip */}
        <MetricBar summary={summary} selectedCount={selectedCount} />

        {/* Package Table & Controls */}
        <PackageTable
          packages={packages}
          onTogglePackage={handleTogglePackage}
          onTogglePin={handleTogglePin}
          onSelectSafe={handleSelectSafe}
          onSelectSecurity={handleSelectSecurity}
          onSelectAll={handleSelectAll}
          onDeselectAll={handleDeselectAll}
        />
      </main>

      {/* Floating Log Drawer Trigger when closed */}
      {!isLogOpen && events.length > 0 && (
        <button
          onClick={() => setIsLogOpen(true)}
          className="fixed bottom-4 right-6 z-30 font-mono text-xs bg-zinc-900 hover:bg-zinc-800 text-zinc-300 border border-zinc-700/80 px-3 py-1.5 rounded-full shadow-lg flex items-center gap-2 transition-all cursor-pointer"
        >
          <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
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
