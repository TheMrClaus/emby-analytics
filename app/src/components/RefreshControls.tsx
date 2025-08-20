// app/src/components/RefreshControls.tsx
import { useMemo } from "react";
import { useRefresh } from "../hooks/useRefresh";

export default function RefreshControls() {
  const { state, start } = useRefresh();
  const progress = useMemo(() => {
    if (!state.total || state.total <= 0) return 0;
    return Math.min(100, Math.round((state.imported / state.total) * 100));
  }, [state.imported, state.total]);

  return (
    <div className="card p-4 flex items-center gap-3">
      <button
        className="px-3 py-1 rounded-2xl border border-white/10 hover:bg-white/10"
        onClick={start}
        disabled={state.running}
      >
        {state.running ? "Refreshing…" : "Start Refresh"}
      </button>
      <div className="ty-muted">
        {state.running ? `Page ${state.page} • ${state.imported}/${state.total ?? "?"}` : "Idle"}
      </div>
      <div className="flex-1" />
      <div className="w-48 h-2 bg-white/10 rounded-full overflow-hidden">
        <div
          className="h-full bg-white/70"
          style={{ width: `${progress}%` }}
        />
      </div>
      {state.error ? <div className="text-red-400 text-sm ml-2">{state.error}</div> : null}
    </div>
  );
}

