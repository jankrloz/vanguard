import { useState, useMemo, useEffect, type FC } from "react";
import {
  X,
  ShieldAlert,
  ShieldCheck,
  AlertTriangle,
  ArrowUpCircle,
  ExternalLink,
  CheckSquare,
  Flame,
  Loader2,
  Trash2,
  CheckCircle2,
} from "lucide-react";
import type { Package } from "../lib/types";

interface SelectedPanelProps {
  isOpen: boolean;
  packages: Package[];
  isUpgrading: boolean;
  onClose: () => void;
  onTogglePackage: (id: string) => void;
  onDeselectAll: () => void;
  onUpgrade: () => void;
}

export const SelectedPanel: FC<SelectedPanelProps> = ({
  isOpen,
  packages,
  isUpgrading,
  onClose,
  onTogglePackage,
  onDeselectAll,
  onUpgrade,
}) => {
  const [search, setSearch] = useState("");

  // Escape key closes modal
  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape" && !isUpgrading) {
        onClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, isUpgrading, onClose]);

  const selected = useMemo(() => {
    return packages.filter((p) => p.selected && !p.pinned);
  }, [packages]);

  const cves = useMemo(() => {
    return selected.filter((p) => p.has_vulnerability);
  }, [selected]);

  const majors = useMemo(() => {
    return selected.filter((p) => !p.has_vulnerability && p.change_type === "major");
  }, [selected]);

  const safes = useMemo(() => {
    return selected.filter((p) => !p.has_vulnerability && p.safe);
  }, [selected]);

  const others = useMemo(() => {
    return selected.filter(
      (p) => !p.has_vulnerability && !p.safe && p.change_type !== "major"
    );
  }, [selected]);

  const managers = useMemo(() => {
    return Array.from(new Set(selected.map((p) => p.manager))).sort();
  }, [selected]);

  if (!isOpen) return null;

  const matchesSearch = (p: Package) => {
    if (!search.trim()) return true;
    const q = search.toLowerCase();
    const matchesName = p.name.toLowerCase().includes(q);
    const matchesMgr = p.manager.toLowerCase().includes(q);
    const matchesVuln = (p.vulnerabilities || []).some(
      (v) => v.id.toLowerCase().includes(q) || v.summary.toLowerCase().includes(q)
    );
    return matchesName || matchesMgr || matchesVuln;
  };

  const filteredCves = cves.filter(matchesSearch);
  const filteredMajors = majors.filter(matchesSearch);
  const filteredSafes = safes.filter(matchesSearch);
  const filteredOthers = others.filter(matchesSearch);

  const handleStartUpgrade = () => {
    if (selected.length === 0 || isUpgrading) return;
    onUpgrade();
    onClose();
  };

  return (
    <div
      onClick={() => !isUpgrading && onClose()}
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150 cursor-pointer"
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="bg-zinc-950 border border-zinc-800 rounded-xl max-w-3xl w-full max-h-[85vh] flex flex-col shadow-2xl overflow-hidden cursor-default font-mono text-xs"
      >
        {/* Header */}
        <div className="px-5 py-3.5 border-b border-zinc-800 flex items-center justify-between bg-zinc-900/60">
          <div>
            <div className="flex items-center gap-2.5">
              <ArrowUpCircle className="w-4 h-4 text-zinc-200" />
              <h2 className="text-sm font-semibold text-zinc-100 tracking-tight">
                Queued Updates Review
              </h2>
              <span className="text-[10px] text-zinc-300 bg-zinc-800 border border-zinc-700/80 px-2 py-0.5 rounded font-semibold">
                {selected.length} items
              </span>
            </div>
            <p className="text-[11px] text-zinc-400 mt-0.5">
              Categorized by criticality and SemVer risk before dispatch
            </p>
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              disabled={isUpgrading}
              onClick={onClose}
              className="p-1.5 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 rounded-md transition-colors cursor-pointer disabled:opacity-40"
              title="Close (Esc)"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Sub-header pills */}
        <div className="px-5 py-2 border-b border-zinc-800/80 bg-zinc-900/40 flex flex-wrap items-center justify-between gap-2">
          <div className="flex flex-wrap items-center gap-1.5">
            {cves.length > 0 && (
              <span className="flex items-center gap-1 text-[10px] font-semibold text-rose-300 bg-rose-950/80 border border-rose-800/80 px-2 py-0.5 rounded">
                <Flame className="w-3 h-3 text-rose-400" />
                {cves.length} Critical CVEs
              </span>
            )}
            {majors.length > 0 && (
              <span className="flex items-center gap-1 text-[10px] font-semibold text-amber-300 bg-amber-950/80 border border-amber-800/80 px-2 py-0.5 rounded">
                <AlertTriangle className="w-3 h-3 text-amber-400" />
                {majors.length} Major
              </span>
            )}
            {safes.length > 0 && (
              <span className="flex items-center gap-1 text-[10px] font-semibold text-emerald-300 bg-emerald-950/80 border border-emerald-800/80 px-2 py-0.5 rounded">
                <ShieldCheck className="w-3 h-3 text-emerald-400" />
                {safes.length} Safe
              </span>
            )}
            {others.length > 0 && (
              <span className="text-[10px] font-semibold text-zinc-400 bg-zinc-800/80 border border-zinc-700/80 px-2 py-0.5 rounded">
                {others.length} Standard
              </span>
            )}
          </div>

          {selected.length > 6 && (
            <input
              type="text"
              placeholder="Search in queued items..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="bg-zinc-900 border border-zinc-800 rounded-md px-2.5 py-1 text-[11px] text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-zinc-700 w-48"
            />
          )}
        </div>

        {/* Content Body */}
        <div className="flex-1 overflow-y-auto p-4 space-y-4">
          {selected.length === 0 ? (
            <div className="py-16 text-center text-zinc-500">
              <CheckCircle2 className="w-8 h-8 text-zinc-600 mx-auto mb-2" />
              <p className="text-sm font-semibold text-zinc-300">Queue is empty</p>
              <p className="text-xs text-zinc-500 mt-1 max-w-xs mx-auto">
                No packages are currently selected. Mark items in the table or choose a quick filter preset.
              </p>
            </div>
          ) : (
            <>
              {/* 1. Critical & CVEs Section */}
              {filteredCves.length > 0 && (
                <div className="border border-rose-900/60 rounded-lg overflow-hidden bg-rose-950/10">
                  <div className="bg-rose-950/40 px-3.5 py-2 border-b border-rose-900/60 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <ShieldAlert className="w-3.5 h-3.5 text-rose-400" />
                      <span className="font-semibold text-rose-200">
                        Critical & Security Vulnerabilities ({filteredCves.length})
                      </span>
                    </div>
                    <span className="text-[10px] text-rose-400 bg-rose-900/40 px-1.5 py-0.2 rounded border border-rose-800/60">
                      Fix Urgent
                    </span>
                  </div>
                  <div className="divide-y divide-rose-950/40">
                    {filteredCves.map((p) => (
                      <div
                        key={p.id}
                        className="px-3.5 py-2 flex flex-col sm:flex-row sm:items-center justify-between gap-2 hover:bg-rose-950/20 transition-colors"
                      >
                        <div className="flex items-center gap-2.5 min-w-0">
                          <button
                            type="button"
                            disabled={isUpgrading}
                            onClick={() => onTogglePackage(p.id)}
                            className="text-zinc-200 hover:text-white transition-colors cursor-pointer shrink-0"
                            title="Unselect package"
                          >
                            <CheckSquare className="w-3.5 h-3.5 text-rose-400" />
                          </button>
                          <span className="text-[10px] uppercase font-bold text-zinc-400 bg-zinc-900 px-1.5 py-0.2 rounded border border-zinc-800 shrink-0">
                            {p.manager}
                          </span>
                          <span className="font-semibold text-zinc-200 truncate">
                            {p.name}
                          </span>
                          {p.source_url && (
                            <a
                              href={p.source_url}
                              target="_blank"
                              rel="noreferrer"
                              className="text-zinc-500 hover:text-zinc-300"
                              title="Upstream source"
                            >
                              <ExternalLink className="w-3 h-3" />
                            </a>
                          )}
                        </div>

                        <div className="flex items-center gap-3 shrink-0 self-end sm:self-center font-mono">
                          <span className="text-zinc-400">
                            {p.current_version} →{" "}
                            <span className="text-emerald-400 font-semibold">{p.latest_version}</span>
                          </span>
                          <div className="flex items-center gap-1">
                            {(p.vulnerabilities || []).slice(0, 2).map((v) => (
                              <span
                                key={v.id}
                                className="text-[9px] bg-rose-950 border border-rose-800 text-rose-300 px-1 py-0.2 rounded font-semibold"
                              >
                                {v.id}
                              </span>
                            ))}
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* 2. Major Breaking Section */}
              {filteredMajors.length > 0 && (
                <div className="border border-amber-900/50 rounded-lg overflow-hidden bg-amber-950/10">
                  <div className="bg-amber-950/30 px-3.5 py-2 border-b border-amber-900/50 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <AlertTriangle className="w-3.5 h-3.5 text-amber-400" />
                      <span className="font-semibold text-amber-200">
                        Major Breaking Updates ({filteredMajors.length})
                      </span>
                    </div>
                    <span className="text-[10px] text-amber-400 bg-amber-900/40 px-1.5 py-0.2 rounded border border-amber-800/60">
                      SemVer Major
                    </span>
                  </div>
                  <div className="divide-y divide-amber-950/30">
                    {filteredMajors.map((p) => (
                      <div
                        key={p.id}
                        className="px-3.5 py-2 flex flex-col sm:flex-row sm:items-center justify-between gap-2 hover:bg-amber-950/20 transition-colors"
                      >
                        <div className="flex items-center gap-2.5 min-w-0">
                          <button
                            type="button"
                            disabled={isUpgrading}
                            onClick={() => onTogglePackage(p.id)}
                            className="text-zinc-200 hover:text-white transition-colors cursor-pointer shrink-0"
                            title="Unselect package"
                          >
                            <CheckSquare className="w-3.5 h-3.5 text-amber-400" />
                          </button>
                          <span className="text-[10px] uppercase font-bold text-zinc-400 bg-zinc-900 px-1.5 py-0.2 rounded border border-zinc-800 shrink-0">
                            {p.manager}
                          </span>
                          <span className="font-semibold text-zinc-200 truncate">
                            {p.name}
                          </span>
                          {p.source_url && (
                            <a
                              href={p.source_url}
                              target="_blank"
                              rel="noreferrer"
                              className="text-zinc-500 hover:text-zinc-300"
                              title="Upstream source"
                            >
                              <ExternalLink className="w-3 h-3" />
                            </a>
                          )}
                        </div>

                        <div className="flex items-center gap-3 shrink-0 self-end sm:self-center font-mono">
                          <span className="text-zinc-400">
                            {p.current_version} →{" "}
                            <span className="text-amber-300 font-semibold">{p.latest_version}</span>
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* 3. Safe Updates Section */}
              {filteredSafes.length > 0 && (
                <div className="border border-emerald-900/40 rounded-lg overflow-hidden bg-emerald-950/10">
                  <div className="bg-emerald-950/30 px-3.5 py-2 border-b border-emerald-900/40 flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
                      <span className="font-semibold text-emerald-200">
                        Safe Minor & Patch Updates ({filteredSafes.length})
                      </span>
                    </div>
                    <span className="text-[10px] text-emerald-400 bg-emerald-900/40 px-1.5 py-0.2 rounded border border-emerald-800/60">
                      Compatible
                    </span>
                  </div>
                  <div className="divide-y divide-emerald-950/30">
                    {filteredSafes.map((p) => (
                      <div
                        key={p.id}
                        className="px-3.5 py-2 flex flex-col sm:flex-row sm:items-center justify-between gap-2 hover:bg-emerald-950/20 transition-colors"
                      >
                        <div className="flex items-center gap-2.5 min-w-0">
                          <button
                            type="button"
                            disabled={isUpgrading}
                            onClick={() => onTogglePackage(p.id)}
                            className="text-zinc-200 hover:text-white transition-colors cursor-pointer shrink-0"
                            title="Unselect package"
                          >
                            <CheckSquare className="w-3.5 h-3.5 text-emerald-400" />
                          </button>
                          <span className="text-[10px] uppercase font-bold text-zinc-400 bg-zinc-900 px-1.5 py-0.2 rounded border border-zinc-800 shrink-0">
                            {p.manager}
                          </span>
                          <span className="font-semibold text-zinc-200 truncate">
                            {p.name}
                          </span>
                          {p.source_url && (
                            <a
                              href={p.source_url}
                              target="_blank"
                              rel="noreferrer"
                              className="text-zinc-500 hover:text-zinc-300"
                              title="Upstream source"
                            >
                              <ExternalLink className="w-3 h-3" />
                            </a>
                          )}
                        </div>

                        <div className="flex items-center gap-3 shrink-0 self-end sm:self-center font-mono">
                          <span className="text-zinc-400">
                            {p.current_version} →{" "}
                            <span className="text-emerald-400 font-semibold">{p.latest_version}</span>
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* 4. Other Updates Section */}
              {filteredOthers.length > 0 && (
                <div className="border border-zinc-800 rounded-lg overflow-hidden bg-zinc-900/20">
                  <div className="bg-zinc-900/60 px-3.5 py-2 border-b border-zinc-800 flex items-center justify-between">
                    <span className="font-semibold text-zinc-300">
                      Other Updates ({filteredOthers.length})
                    </span>
                  </div>
                  <div className="divide-y divide-zinc-800/40">
                    {filteredOthers.map((p) => (
                      <div
                        key={p.id}
                        className="px-3.5 py-2 flex flex-col sm:flex-row sm:items-center justify-between gap-2 hover:bg-zinc-800/30 transition-colors"
                      >
                        <div className="flex items-center gap-2.5 min-w-0">
                          <button
                            type="button"
                            disabled={isUpgrading}
                            onClick={() => onTogglePackage(p.id)}
                            className="text-zinc-200 hover:text-white transition-colors cursor-pointer shrink-0"
                            title="Unselect package"
                          >
                            <CheckSquare className="w-3.5 h-3.5 text-zinc-400" />
                          </button>
                          <span className="text-[10px] uppercase font-bold text-zinc-400 bg-zinc-900 px-1.5 py-0.2 rounded border border-zinc-800 shrink-0">
                            {p.manager}
                          </span>
                          <span className="font-semibold text-zinc-200 truncate">
                            {p.name}
                          </span>
                        </div>

                        <div className="flex items-center gap-3 shrink-0 self-end sm:self-center font-mono">
                          <span className="text-zinc-400">
                            {p.current_version} →{" "}
                            <span className="text-zinc-200">{p.latest_version}</span>
                          </span>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </>
          )}
        </div>

        {/* Footer */}
        <div className="px-5 py-3 border-t border-zinc-800 bg-zinc-900/60 flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="text-[11px] text-zinc-400">
            {selected.length > 0 ? (
              <span>
                Total: <strong className="text-zinc-200">{selected.length}</strong> packages across{" "}
                <strong className="text-zinc-200">{managers.length}</strong> managers (
                {managers.join(", ")})
              </span>
            ) : (
              <span>No items selected</span>
            )}
          </div>

          <div className="flex items-center gap-2 self-end sm:self-center">
            {selected.length > 0 && (
              <button
                type="button"
                disabled={isUpgrading}
                onClick={onDeselectAll}
                className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-zinc-800 bg-zinc-950 hover:bg-zinc-800 text-zinc-400 hover:text-zinc-200 transition-colors cursor-pointer disabled:opacity-40"
              >
                <Trash2 className="w-3.5 h-3.5" />
                <span>Clear All</span>
              </button>
            )}

            <button
              type="button"
              disabled={isUpgrading}
              onClick={onClose}
              className="px-3.5 py-1.5 rounded-lg border border-zinc-800 bg-zinc-950 hover:bg-zinc-800 text-zinc-300 hover:text-zinc-100 transition-colors cursor-pointer disabled:opacity-40"
            >
              Cancel
            </button>

            <button
              type="button"
              disabled={selected.length === 0 || isUpgrading}
              onClick={handleStartUpgrade}
              className={`flex items-center gap-2 px-4 py-1.5 rounded-lg font-semibold transition-all shadow-sm ${
                selected.length > 0 && !isUpgrading
                  ? "bg-zinc-100 hover:bg-white text-zinc-950 cursor-pointer hover:shadow-zinc-500/10"
                  : "bg-zinc-800 text-zinc-500 cursor-not-allowed border border-zinc-700/50"
              }`}
            >
              {isUpgrading ? (
                <Loader2 className="w-4 h-4 animate-spin text-zinc-400" />
              ) : (
                <ArrowUpCircle className="w-4 h-4" />
              )}
              <span>Upgrade ({selected.length})</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
