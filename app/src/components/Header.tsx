// app/src/components/Header.tsx
import { useRef } from "react";
import Link from "next/link";
import { useUsage, useRefreshStatus, useVersion } from "../hooks/useData";
import { startRefresh, setAdminToken, syncAllServers } from "../lib/api";
import { useRouter } from "next/router";
import { fmtHours } from "../lib/format";

export default function Header() {
  // SWR-powered data
  const { data: weeklyUsage = [], error: usageError } = useUsage(7);
  const { data: refreshStatus } = useRefreshStatus(true); // poll regularly
  const { data: versionInfo } = useVersion();

  // Derived UI counters
  const weeklyHours = weeklyUsage.reduce((acc, r) => acc + (r.hours || 0), 0);

  // Progress %
  const aggregateProcessed = Number(
    refreshStatus?.aggregate_processed ?? refreshStatus?.imported ?? 0
  );
  const aggregateTotal = Number(refreshStatus?.aggregate_total ?? refreshStatus?.total ?? 0);
  const refreshOnly = refreshStatus?.refresh_only;

  let displayTotal = aggregateTotal;
  let displayProcessed = aggregateProcessed;
  if (!displayTotal && refreshOnly?.total) {
    displayTotal = Number(refreshOnly.total ?? 0);
    displayProcessed = Number(refreshOnly.imported ?? 0);
  }

  const progress =
    displayTotal > 0 ? Math.max(0, Math.min(100, (displayProcessed / displayTotal) * 100)) : 0;

  // The running state is now driven directly by the aggregated status
  const isRunning = Boolean(refreshStatus?.running);

  // ---- Double-click / spam click guard ----
  const clickLockRef = useRef(false);

  const router = useRouter();

  const performSyncs = async () => {
    let primaryError: unknown = null;
    try {
      await startRefresh();
    } catch (err) {
      primaryError = err;
    }
    try {
      await syncAllServers();
    } catch (err) {
      if (primaryError == null) {
        primaryError = err;
      }
    }
    if (primaryError) {
      throw primaryError;
    }
  };

  const handleRefresh = async () => {
    // Block if lock engaged, UI already refreshing, or backend says it's running.
    if (clickLockRef.current || isRunning) return;

    // Engage a very short lock so rapid multiple clicks can't queue multiple jobs.
    clickLockRef.current = true;
    setTimeout(() => {
      clickLockRef.current = false;
    }, 1200);

    try {
      await performSyncs();
    } catch (err: unknown) {
      const msg = String((err as Error)?.message || err || "");
      // If unauthorized, prompt for admin token and retry once
      if (typeof window !== "undefined" && msg.startsWith("401")) {
        const t = window.prompt("Enter admin token to use for admin actions:");
        if (t && t.trim()) {
          setAdminToken(t.trim());
          try {
            await performSyncs();
            return;
          } catch (e) {
            console.error("Failed to start refresh after setting token:", e);
          }
        }
      }
      console.error("Failed to start refresh:", err);
    }
  };

  const handleLogout = async () => {
    try {
      await fetch("/auth/logout", { method: "POST", credentials: "include" });
    } catch {}
    try {
      if (typeof window !== "undefined") {
        window.localStorage.removeItem("emby_admin_token");
      }
    } catch {}
    router.replace("/login");
  };

  return (
    <header className="bg-neutral-900 border-b border-neutral-700 px-6 py-4">
      <div className="flex items-center justify-between">
        {/* Title + Clock */}
        <div className="flex items-center gap-8">
          <Link
            href="/"
            className="text-2xl font-bold text-white hover:text-amber-300 transition-colors cursor-pointer"
          >
            Emby Analytics
          </Link>
        </div>

        {/* Stats + Refresh */}
        <div className="flex items-center gap-6">
          {/* Version badge */}
          <div className="text-xs text-gray-300">
            {versionInfo && (
              <a
                href={versionInfo.url || "#"}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-2 px-2 py-1 rounded-md bg-neutral-800 hover:bg-neutral-700 border border-neutral-700"
                title={
                  versionInfo.update_available
                    ? `Update available: ${versionInfo.latest_tag}`
                    : `Version ${versionInfo.version}`
                }
              >
                <span className="font-mono">
                  {versionInfo.version}
                  {versionInfo.commit && versionInfo.version === "dev" && (
                    <span className="opacity-70">@{versionInfo.commit}</span>
                  )}
                </span>
                {versionInfo.update_available && (
                  <span
                    className="inline-block w-2 h-2 rounded-full bg-red-500"
                    title={`New: ${versionInfo.latest_tag}`}
                  />
                )}
              </a>
            )}
          </div>
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
                    {displayTotal > 0 && (
                      <span className="text-xs ml-1 opacity-90">
                        ({displayProcessed}/{displayTotal})
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

          {/* Quick nav links */}
          <div className="flex items-center gap-3 text-sm">
            <Link
              href="/settings"
              className="text-blue-300 hover:text-white underline decoration-dotted"
            >
              Settings
            </Link>
            <span className="text-gray-500">|</span>
            <Link
              href="/api-explorer"
              className="text-blue-300 hover:text-white underline decoration-dotted"
            >
              API Explorer
            </Link>
            <span className="text-gray-500">|</span>
            <button
              onClick={handleLogout}
              className="text-red-300 hover:text-white underline decoration-dotted"
              title="Logout"
            >
              Logout
            </button>
          </div>
        </div>
      </div>
    </header>
  );
}
