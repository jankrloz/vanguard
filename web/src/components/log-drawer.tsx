import { useEffect, useRef, useState, type FC } from "react";
import { Terminal, ChevronDown, CheckCircle2, XCircle, Loader2 } from "lucide-react";
import type { UpgradeEvent } from "../lib/types";

interface LogDrawerProps {
  isOpen: boolean;
  isUpgrading: boolean;
  events: UpgradeEvent[];
  onClose: () => void;
  onClear: () => void;
}

export const LogDrawer: FC<LogDrawerProps> = ({
  isOpen,
  isUpgrading,
  events,
  onClose,
  onClear,
}) => {
  const [autoScroll, setAutoScroll] = useState(true);
  const [copied, setCopied] = useState(false);
  const logEndRef = useRef<HTMLDivElement>(null);

  const handleCopy = () => {
    const text = events.map((e) => e.message).join("\n");
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  useEffect(() => {
    if (autoScroll && logEndRef.current) {
      logEndRef.current.scrollIntoView({ behavior: "smooth" });
    }
  }, [events, autoScroll]);

  if (!isOpen) return null;

  const lastEvent = events[events.length - 1];
  const isComplete = lastEvent?.type === "complete";
  const isError = lastEvent?.type === "error" || lastEvent?.status === "failed";

  return (
    <div className="fixed bottom-0 left-0 right-0 z-40 bg-zinc-950 border-t border-zinc-800 shadow-2xl flex flex-col max-h-[60vh] transition-all">
      {/* Drawer Header */}
      <div className="px-6 py-2.5 bg-zinc-900/90 border-b border-zinc-800/80 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Terminal className="w-4 h-4 text-zinc-400" />
          <span className="font-mono text-xs font-semibold text-zinc-200">
            Execution Log
          </span>

          {/* Status Indicator */}
          {isUpgrading && !isComplete ? (
            <span className="flex items-center gap-1.5 text-[11px] font-mono text-amber-400 bg-amber-950/60 border border-amber-800/60 px-2 py-0.5 rounded">
              <Loader2 className="w-3 h-3 animate-spin" /> Running
            </span>
          ) : isComplete ? (
            <span className="flex items-center gap-1.5 text-[11px] font-mono text-emerald-400 bg-emerald-950/60 border border-emerald-800/60 px-2 py-0.5 rounded">
              <CheckCircle2 className="w-3 h-3" /> Done
            </span>
          ) : isError ? (
            <span className="flex items-center gap-1.5 text-[11px] font-mono text-rose-400 bg-rose-950/60 border border-rose-800/60 px-2 py-0.5 rounded">
              <XCircle className="w-3 h-3" /> Error
            </span>
          ) : null}
        </div>

        <div className="flex items-center gap-2">
          <label className="flex items-center gap-1.5 text-[11px] font-mono text-zinc-400 cursor-pointer mr-2">
            <input
              type="checkbox"
              checked={autoScroll}
              onChange={(e) => setAutoScroll(e.target.checked)}
              className="rounded bg-zinc-900 border-zinc-700 text-zinc-300 focus:ring-0"
            />
            Auto-scroll
          </label>

          <button
            onClick={handleCopy}
            disabled={events.length === 0}
            className="text-[11px] font-mono text-zinc-400 hover:text-zinc-200 bg-zinc-800/80 hover:bg-zinc-800 px-2 py-1 rounded disabled:opacity-40"
          >
            {copied ? "Copied ✓" : "Copy"}
          </button>

          <button
            onClick={onClear}
            className="text-[11px] font-mono text-zinc-400 hover:text-zinc-200 bg-zinc-800/80 hover:bg-zinc-800 px-2 py-1 rounded"
          >
            Clear
          </button>

          <button
            onClick={onClose}
            className="text-zinc-400 hover:text-zinc-200 p-1 rounded"
          >
            <ChevronDown className="w-4 h-4" />
          </button>
        </div>
      </div>

      {/* Terminal Content */}
      <div className="p-4 font-mono text-xs overflow-y-auto space-y-1 bg-black/60 min-h-[180px] max-h-[400px]">
        {events.length === 0 ? (
          <div className="text-zinc-500 italic">No output yet. Trigger an action to stream execution logs.</div>
        ) : (
          events.map((ev, index) => {
            const isPkgStart = ev.type === "package_start";
            const isPkgDone = ev.type === "package_done";
            const isCompleteType = ev.type === "complete";
            const isErrorType = ev.type === "error" || ev.status === "failed";

            return (
              <div
                key={index}
                className={`leading-relaxed ${
                  isPkgStart
                    ? "text-blue-400 font-semibold mt-2"
                    : isPkgDone
                    ? ev.status === "failed"
                      ? "text-rose-400 font-semibold"
                      : "text-emerald-400 font-semibold"
                    : isCompleteType
                    ? "text-emerald-300 font-bold mt-2"
                    : isErrorType
                    ? "text-rose-400"
                    : "text-zinc-300"
                }`}
              >
                <span className="text-zinc-600 select-none mr-2">›</span>
                {ev.message}
              </div>
            );
          })
        )}
        <div ref={logEndRef} />
      </div>
    </div>
  );
};
