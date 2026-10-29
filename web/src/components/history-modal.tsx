import { useState, useEffect, type FC } from "react";
import { X, History, RefreshCw, CheckCircle2, XCircle } from "lucide-react";
import { fetchHistory } from "../lib/api";
import type { HistoryEntry } from "../lib/types";

interface HistoryModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const HistoryModal: FC<HistoryModalProps> = ({ isOpen, onClose }) => {
  const [entries, setEntries] = useState<HistoryEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [search, setSearch] = useState("");

  const load = async () => {
    setLoading(true);
    try {
      const data = await fetchHistory(100);
      setEntries(data || []);
    } catch {
      setEntries([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (isOpen) {
      load();
    }
  }, [isOpen]);

  // Escape key closes modal
  useEffect(() => {
    if (!isOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        onClose();
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const filtered = entries.filter((e) => {
    if (!search) return true;
    const q = search.toLowerCase();
    return (
      e.package_id.toLowerCase().includes(q) ||
      e.manager.toLowerCase().includes(q) ||
      e.status.toLowerCase().includes(q)
    );
  });

  return (
    <div
      onClick={onClose}
      className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm animate-in fade-in duration-150 cursor-pointer"
    >
      <div
        onClick={(e) => e.stopPropagation()}
        className="bg-zinc-950 border border-zinc-800 rounded-xl max-w-3xl w-full max-h-[85vh] flex flex-col shadow-2xl overflow-hidden cursor-default"
      >
        {/* Header */}
        <div className="px-5 py-3.5 border-b border-zinc-800 flex items-center justify-between bg-zinc-900/60">
          <div className="flex items-center gap-2.5">
            <History className="w-4 h-4 text-zinc-300" />
            <h2 className="text-sm font-semibold text-zinc-100 font-mono tracking-tight">
              Audit Upgrade History
            </h2>
            <span className="text-[10px] font-mono text-zinc-400 bg-zinc-800/80 border border-zinc-700/50 px-2 py-0.5 rounded">
              {entries.length} records
            </span>
          </div>

          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={load}
              disabled={loading}
              title="Refresh history"
              className="p-1.5 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 rounded-md transition-colors cursor-pointer"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? "animate-spin text-zinc-200" : ""}`} />
            </button>
            <button
              type="button"
              onClick={onClose}
              className="p-1.5 text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800 rounded-md transition-colors cursor-pointer"
              title="Close (Esc)"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* Filter bar */}
        <div className="p-3 border-b border-zinc-800/80 bg-zinc-900/30">
          <input
            type="text"
            placeholder="Search package, manager, or status..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-zinc-900 border border-zinc-800 rounded-md px-3 py-1.5 text-xs text-zinc-200 placeholder-zinc-500 focus:outline-none focus:border-zinc-700 font-mono"
          />
        </div>

        {/* Body table */}
        <div className="flex-1 overflow-y-auto divide-y divide-zinc-800/50 font-mono text-xs">
          {loading && entries.length === 0 ? (
            <div className="p-12 text-center text-zinc-500 text-xs">Loading audit records...</div>
          ) : filtered.length === 0 ? (
            <div className="p-12 text-center text-zinc-500 text-xs">
              No upgrade history records found in ~/.local/state/vanguard/history.jsonl
            </div>
          ) : (
            filtered.map((item, idx) => {
              const isOk = item.status === "ok";
              const timeStr = new Date(item.timestamp).toLocaleString();
              return (
                <div
                  key={idx}
                  className="px-5 py-2.5 flex items-center justify-between gap-4 hover:bg-zinc-900/40 transition-colors"
                >
                  <div className="flex items-center gap-3 min-w-0">
                    {isOk ? (
                      <CheckCircle2 className="w-4 h-4 text-emerald-400 shrink-0" />
                    ) : (
                      <XCircle className="w-4 h-4 text-rose-400 shrink-0" />
                    )}

                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="font-semibold text-zinc-200 truncate">
                          {item.package_id}
                        </span>
                        <span className="text-[10px] text-zinc-400 bg-zinc-800 px-1.5 py-0.2 rounded uppercase">
                          {item.manager}
                        </span>
                      </div>
                      <div className="text-[10px] text-zinc-500">{timeStr}</div>
                    </div>
                  </div>

                  <div className="flex items-center gap-3.5 shrink-0">
                    <div className="text-right text-[11px] tabular-nums">
                      <span className="text-zinc-400">{item.from_version}</span>
                      <span className="text-zinc-600 mx-1.5 select-none">→</span>
                      <span className="text-zinc-200 font-medium">{item.to_version}</span>
                    </div>

                    <span
                      className={`text-[10px] font-bold px-1.5 py-0.5 rounded uppercase ${
                        isOk
                          ? "bg-emerald-950/80 text-emerald-300 border border-emerald-800"
                          : "bg-rose-950/80 text-rose-300 border border-rose-800"
                      }`}
                    >
                      {item.status}
                    </span>
                  </div>
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
};
