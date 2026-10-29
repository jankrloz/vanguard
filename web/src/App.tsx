import React, { useState, useEffect, useCallback } from "react";
import { Header } from "./components/header";
import { MetricBar } from "./components/metric-bar";
import { PackageTable } from "./components/package-table";
import { LogDrawer } from "./components/log-drawer";
import {
  fetchStatus,
  fetchOutdated,
  triggerUpgrade,
  triggerClean,
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
      const pkgs = (scanResult.packages || []).map((p) => {
        const safe = p.Safe !== undefined ? p.Safe : p.safe;
        const pinned = p.Pinned !== undefined ? p.Pinned : p.pinned;
        const hasVuln = p.HasVulnerability || p.has_vulnerability;
        return {
          ...p,
          selected: (safe || hasVuln) && !pinned,
        };
      });

      setPackages(pkgs);
      setSummary(scanResult.summary || {
        total_packages: pkgs.length,
        safe_count: pkgs.filter((p) => p.Safe || p.safe).length,
        major_count: pkgs.filter((p) => (p.ChangeType || p.change_type) === "major").length,
        pinned_count: pkgs.filter((p) => p.Pinned || p.pinned).length,
        security_fix_count: pkgs.filter((p) => p.HasVulnerability || p.has_vulnerability).length,
      });
    } catch (err: any) {
      setError(err?.message || "Failed to load package information");
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
        const pkgId = p.ID || p.id;
        if (pkgId === id) {
          return { ...p, selected: !p.selected };
        }
        return p;
      })
    );
  };

  const handleSelectSafe = () => {
    setPackages((prev) =>
      prev.map((p) => {
        const safe = p.Safe !== undefined ? p.Safe : p.safe;
        const pinned = p.Pinned !== undefined ? p.Pinned : p.pinned;
        return { ...p, selected: safe && !pinned };
      })
    );
  };

  const handleSelectSecurity = () => {
    setPackages((prev) =>
      prev.map((p) => {
        const hasVuln = p.HasVulnerability || p.has_vulnerability;
        const pinned = p.Pinned !== undefined ? p.Pinned : p.pinned;
        return { ...p, selected: Boolean(hasVuln && !pinned) };
      })
    );
  };

  const handleSelectAll = () => {
    setPackages((prev) =>
      prev.map((p) => {
        const pinned = p.Pinned !== undefined ? p.Pinned : p.pinned;
        return { ...p, selected: !pinned };
      })
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
      const ids = selected.map((p) => p.ID || p.id || p.Name || p.name);
      await triggerUpgrade(ids);
    } catch (err: any) {
      setError(err?.message || "Upgrade request failed");
      setIsUpgrading(false);
    }
  };

  const handleClean = async () => {
    setIsUpgrading(true);
    setIsLogOpen(true);
    setEvents([]);

    try {
      await triggerClean();
    } catch (err: any) {
      setError(err?.message || "Clean request failed");
      setIsUpgrading(false);
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
    </div>
  );
}
export default App;
