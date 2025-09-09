// app/src/components/Header.tsx
import { useRef } from "react";
import { useUsage, useNowSnapshot, useRefreshStatus } from "../hooks/useData";
import { startRefresh, setAdminToken, clearAdminToken } from "../lib/api";
import { fmtHours } from "../lib/format";

type SnapshotEntry = {
  play_method?: string;
};

export default function Header() {
  // SWR-powered data
  const { data: weeklyUsage = [], error: usageError } = useUsage(7);
  const { data: nowPlaying = [], error: snapshotError } = useNowSnapshot();
  const { data: refreshStatus } = useRefreshStatus(true); // poll regularly

  // Derived UI counters
  const weeklyHours = weeklyUsage.reduce((acc, r) => acc + (r.hours || 0), 0);
  const streamsTotal = nowPlaying.length;
  const directPlay = nowPlaying.filter(
    (s: SnapshotEntry) => s.play_method === "DirectPlay" || s.play_method === "DirectStream"
  ).length;
  const transcoding = streamsTotal - directPlay;

  // Progress %
  const imported = Number(refreshStatus?.imported ?? 0);
  const total = Number(refreshStatus?.total ?? 0);
  const progress = total > 0 ? Math.max(0, Math.min(100, (imported / total) * 100)) : 0;

  // The running state is now driven directly by the SWR hook
  const isRunning = Boolean(refreshStatus?.running);

  // ---- Double-click / spam click guard ----
  const clickLockRef = useRef(false);

  const handleRefresh = async () => {
    // Block if lock engaged, UI already refreshing, or backend says it's running.
    if (clickLockRef.current || isRunning) return;

    // Engage a very short lock so rapid multiple clicks can't queue multiple jobs.
    clickLockRef.current = true;
    setTimeout(() => {
      clickLockRef.current = false;
    }, 1200);

    try {
      await startRefresh(); // Fiber v3: kicks off the job; progress read via useRefreshStatus
    } catch (err: any) {
      const msg = String(err?.message || err || "");
      // If unauthorized, prompt for admin token and retry once
      if (typeof window !== "undefined" && msg.startsWith("401")) {
        const t = window.prompt("Enter admin token to use for admin actions:");
        if (t && t.trim()) {
          setAdminToken(t.trim());
          try {
            await startRefresh();
            return;
          } catch (e) {
            console.error("Failed to start refresh after setting token:", e);
          }
        }
      }
      console.error("Failed to start refresh:", err);
    }
  };

  return (
    <header className="bg-neutral-900 border-b border-neutral-700 px-6 py-4">
      <div className="flex items-center justify-between">
        {/* Title + Clock */}
        <div className="flex items-center gap-8">
          <a
            href="/"
            className="text-2xl font-bold text-white hover:text-amber-300 transition-colors cursor-pointer"
          >
            Emby Analytics
          </a>
        </div>

        {/* Stats + Refresh */}
        <div className="flex items-center gap-6">
          {/* Weekly Hours */}
          <div className="text-center">
            <div className="text-sm text-gray-400">Weekly Hours</div>
            <div className="text-xl font-bold text-white">
              {usageError ? (
                <span className="text-red-400 text-sm">Error</span>
              ) : (
                fmtHours(weeklyHours)
              )}
            </div>
          </div>

          {/* Current Streams */}
          <div className="text-center">
            <div className="text-sm text-gray-400">Streams</div>
            <div className="text-xl font-bold text-white">
              {snapshotError ? (
                <span className="text-red-400 text-sm">Error</span>
              ) : (
                <>
                  {streamsTotal}
                  {streamsTotal > 0 && (
                    <span className="text-sm text-gray-400 ml-1">
                      ({directPlay}D/{transcoding}T)
                    </span>
                  )}
                </>
              )}
            </div>
          </div>

          {/* Refresh Control (always yellow) */}
          <div className="relative">
            <button
              onClick={handleRefresh}
              disabled={isRunning}
              className={[
                "relative rounded-lg px-4 py-2 font-semibold text-black",
                "bg-amber-600 hover:bg-amber-500 active:translate-y-[1px]",
                "shadow-md transition-colors",
                "h-10",
                isRunning ? "opacity-90 cursor-not-allowed" : "",
              ].join(" ")}
              style={{ minWidth: 220 }}
            >
              <span className="relative z-10">
                {!isRunning && "Refresh Library Index"}
                {isRunning && (
                  <>
                    {"Refreshing… "}
                    {Math.round(progress)}%
                    {total > 0 && (
                      <span className="text-xs ml-1 opacity-90">
                        ({imported}/{total})
                      </span>
                    )}
                  </>
                )}
              </span>

              {/* Inline progress bar, only while refreshing */}
              {isRunning && (
                <span
                  className="absolute left-1 right-1 bottom-1 h-1 rounded-sm bg-amber-900/40"
                  aria-hidden="true"
                >
                  <span
                    className="absolute left-0 top-0 h-full rounded-sm bg-amber-300 transition-all duration-300"
                    style={{ width: `${Math.max(2, Math.min(100, progress))}%` }}
                  />
                </span>
              )}
            </button>
          </div>

          {/* Admin token quick actions + API Explorer */}
          <div className="flex items-center gap-3 text-sm">
            <a
              href="/settings"
              className="text-blue-300 hover:text-white underline decoration-dotted"
            >
              Settings
            </a>
            <span className="text-gray-500">|</span>
            <a
              href="/api-explorer"
              className="text-blue-300 hover:text-white underline decoration-dotted"
            >
              API Explorer
            </a>
            <span className="text-gray-500">|</span>
            <button
              className="text-gray-300 hover:text-white underline decoration-dotted"
              onClick={() => {
                if (typeof window === "undefined") return;
                const t = window.prompt("Set admin token");
                if (t && t.trim()) setAdminToken(t.trim());
              }}
            >
              Admin Token
            </button>
            <span className="text-gray-500">|</span>
            <button
              className="text-gray-400 hover:text-white underline decoration-dotted"
              onClick={() => clearAdminToken()}
            >
              Clear
            </button>
          </div>
        </div>
      </div>
    </header>
  );
}
