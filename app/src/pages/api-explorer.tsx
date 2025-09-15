import React, { useMemo, useState } from "react";
import Link from "next/link";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";

type Param = {
  key: string;
  label?: string;
  kind: "path" | "query" | "body";
  required?: boolean;
  placeholder?: string;
};
type Endpoint = {
  id: string;
  category: string;
  method: "GET" | "POST" | "PUT" | "ALL";
  path: string; // may include :id path params
  description: string;
  usage: string;
  params?: Param[];
  dangerous?: boolean;
  binary?: boolean;
  note?: string;
};

// Read admin token similar to lib/api.ts
const ADMIN_TOKEN_STORAGE_KEY = "emby_admin_token";
function adminAuthHeaderFor(path: string): Record<string, string> {
  if (!path.startsWith("/admin") && !path.startsWith("/api/settings")) return {};
  try {
    if (typeof window !== "undefined") {
      const t = window.localStorage.getItem(ADMIN_TOKEN_STORAGE_KEY);
      if (t) return { Authorization: `Bearer ${t}` };
    }
  } catch {}
  if (process.env.NEXT_PUBLIC_ADMIN_TOKEN) {
    return { Authorization: `Bearer ${process.env.NEXT_PUBLIC_ADMIN_TOKEN}` };
  }
  return {};
}

const endpoints: Endpoint[] = [
  // Health
  {
    id: "health",
    category: "Health",
    method: "GET",
    path: "/health",
    description: "Backend health and DB status.",
    usage: "Quick sanity check the app is running.",
  },
  {
    id: "health-emby",
    category: "Health",
    method: "GET",
    path: "/health/emby",
    description: "Connectivity to Emby and API key validity.",
    usage: "Verify Emby base URL/API key work.",
  },
  {
    id: "health-frontend",
    category: "Health",
    method: "GET",
    path: "/health/frontend",
    description: "Frontend data pipeline health check.",
    usage: "Test complete data flow from DB to UI.",
  },

  // Config
  {
    id: "config",
    category: "Config",
    method: "GET",
    path: "/config",
    description: "Returns UI config like Emby external URL and Server ID.",
    usage: "Used by UI deep-links to Emby.",
  },

  // Admin (Debug/Backfill)
  {
    id: "admin-debug-series-id",
    category: "Admin",
    method: "GET",
    path: "/admin/debug/series-id",
    description: "Resolve a Series ID by name via Emby search.",
    usage: "Check Emby SeriesId for linking.",
    params: [{ key: "name", kind: "query", required: true, placeholder: "Hostage (2025)" }],
  },
  {
    id: "admin-debug-series-from-episode",
    category: "Admin",
    method: "GET",
    path: "/admin/debug/series-from-episode",
    description: "Resolve a Series ID from an Episode ID via Emby.",
    usage: "Find parent series for an episode.",
    params: [{ key: "id", kind: "query", required: true, placeholder: "721139" }],
  },
  {
    id: "admin-backfill-series-dry",
    category: "Admin",
    method: "GET",
    path: "/admin/backfill/series",
    description: "Dry-run: count episodes missing series_id.",
    usage: "Preview backfill impact.",
  },
  {
    id: "admin-backfill-series-apply",
    category: "Admin",
    method: "POST",
    path: "/admin/backfill/series",
    description: "Apply: populate series_id/series_name for episodes.",
    usage: "Fix links for finished series.",
  },
  {
    id: "admin-cleanup-missing-dry",
    category: "Admin",
    method: "GET",
    path: "/admin/cleanup/missing-items",
    description: "Dry-run: scan library_item for IDs missing in Emby.",
    usage: "Identify stale items (deleted in Emby).",
    params: [{ key: "limit", kind: "query", placeholder: "1000" }],
  },
  {
    id: "admin-cleanup-missing-apply",
    category: "Admin",
    method: "POST",
    path: "/admin/cleanup/missing-items",
    description: "Apply: delete stale library items with no play_intervals.",
    usage: "Cleanup DB from stale Emby IDs.",
    params: [{ key: "limit", kind: "query", placeholder: "1000" }],
  },
  {
    id: "admin-backfill-started-at",
    category: "Admin",
    method: "POST",
    path: "/admin/cleanup/backfill-started-at",
    description: "Backfill started_at from earliest events or intervals (preserve if earlier).",
    usage: "Fix sessions showing incorrect start times due to reactivation overwrites.",
    dangerous: true,
  },
  {
    id: "admin-remap-item-dry",
    category: "Admin",
    method: "GET",
    path: "/admin/remap-item",
    description: "Dry-run: show references for remapping from one item_id to another.",
    usage: "Migrate history from old ID to new ID.",
    params: [
      { key: "from_id", kind: "query", required: true, placeholder: "old_id" },
      { key: "to_id", kind: "query", required: true, placeholder: "new_id" },
    ],
  },
  {
    id: "admin-remap-item-apply",
    category: "Admin",
    method: "POST",
    path: "/admin/remap-item",
    description:
      "Apply: remap play_intervals and play_sessions to new item_id; delete old library_item.",
    usage: "Fix broken links while preserving history.",
    params: [
      { key: "from_id", kind: "body", required: true, placeholder: "old_id" },
      { key: "to_id", kind: "body", required: true, placeholder: "new_id" },
    ],
  },
  {
    id: "admin-cleanup-jobs",
    category: "Admin",
    method: "GET",
    path: "/admin/cleanup/jobs",
    description: "List recent cleanup job audit logs.",
    usage: "View cleanup operation history.",
    params: [{ key: "limit", kind: "query", placeholder: "50" }],
  },
  {
    id: "admin-cleanup-job-details",
    category: "Admin",
    method: "GET",
    path: "/admin/cleanup/jobs/:jobId",
    description: "View detailed audit log for specific cleanup job.",
    usage: "Review what items were processed in a cleanup operation.",
    params: [{ key: "jobId", kind: "path", required: true, placeholder: "job-uuid" }],
  },
  {
    id: "admin-cleanup-scheduler-stats",
    category: "Admin",
    method: "GET",
    path: "/admin/cleanup/scheduler/stats",
    description: "View cleanup scheduler statistics and next run times.",
    usage: "Monitor automated weekly cleanup schedule.",
  },

  // Stats
  {
    id: "stats-overview",
    category: "Stats",
    method: "GET",
    path: "/stats/overview",
    description: "High-level library overview counters.",
    usage: "Populate Overview widgets.",
  },
  {
    id: "stats-usage",
    category: "Stats",
    method: "GET",
    path: "/stats/usage",
    description: "Watch time per day.",
    usage: "Usage trends over time.",
    params: [{ key: "days", kind: "query", placeholder: "14" }],
  },
  {
    id: "stats-top-users",
    category: "Stats",
    method: "GET",
    path: "/stats/top/users",
    description: "Top users by watch time.",
    usage: "Leaderboard of users.",
    params: [
      { key: "timeframe", kind: "query", placeholder: "1d|3d|7d|14d|30d" },
      { key: "limit", kind: "query", placeholder: "10" },
    ],
  },
  {
    id: "stats-top-items",
    category: "Stats",
    method: "GET",
    path: "/stats/top/items",
    description: "Top items by watch time (merges live intervals).",
    usage: "Find most watched items.",
    params: [
      { key: "timeframe", kind: "query", placeholder: "1d|3d|7d|14d|30d" },
      { key: "limit", kind: "query", placeholder: "10" },
    ],
  },
  {
    id: "stats-top-series",
    category: "Stats",
    method: "GET",
    path: "/stats/top/series",
    description: "Top series by watch time (across episodes).",
    usage: "Find most watched series.",
    params: [
      { key: "timeframe", kind: "query", placeholder: "1d|3d|7d|14d|30d" },
      { key: "limit", kind: "query", placeholder: "10" },
    ],
  },
  {
    id: "stats-qualities",
    category: "Stats",
    method: "GET",
    path: "/stats/qualities",
    description: "Distribution of media by resolution bucket.",
    usage: "Quality breakdown.",
  },
  {
    id: "stats-codecs",
    category: "Stats",
    method: "GET",
    path: "/stats/codecs",
    description: "Media distribution by codec.",
    usage: "Format/codecs breakdown.",
  },
  {
    id: "stats-active-users",
    category: "Stats",
    method: "GET",
    path: "/stats/active-users",
    description: "Most active users (lifetime).",
    usage: "All-time user totals.",
  },
  {
    id: "stats-users-total",
    category: "Stats",
    method: "GET",
    path: "/stats/users/total",
    description: "Total number of users known.",
    usage: "Count users in DB.",
  },
  {
    id: "stats-user-id",
    category: "Stats",
    method: "GET",
    path: "/stats/users/:id",
    description: "Details for one user.",
    usage: "Per-user drilldown.",
    params: [{ key: "id", kind: "path", required: true, placeholder: "emby-user-id" }],
  },
  {
    id: "stats-user-watch-time",
    category: "Stats",
    method: "GET",
    path: "/stats/users/:id/watch-time",
    description: "Watch time breakdown for specific user.",
    usage: "Per-user time analysis.",
    params: [{ key: "id", kind: "path", required: true, placeholder: "emby-user-id" }],
  },
  {
    id: "stats-users-watch-time",
    category: "Stats",
    method: "GET",
    path: "/stats/users/watch-time",
    description: "Watch time breakdown for all users.",
    usage: "All users time analysis.",
  },
  {
    id: "stats-play-methods",
    category: "Stats",
    method: "GET",
    path: "/stats/play-methods",
    description: "Playback methods summary and recent transcodes.",
    usage: "DirectPlay vs Transcode, with per-stream breakdown.",
    params: [{ key: "days", kind: "query", placeholder: "30" }],
  },
  {
    id: "stats-playback-methods",
    category: "Stats",
    method: "GET",
    path: "/stats/playback-methods",
    description: "Playback methods (backward compatibility alias).",
    usage: "Same as play-methods endpoint.",
    params: [{ key: "days", kind: "query", placeholder: "30" }],
  },
  {
    id: "stats-items-by-codec",
    category: "Stats",
    method: "GET",
    path: "/stats/items/by-codec/:codec",
    description: "List items by codec.",
    usage: "Inventory by codec.",
    params: [
      { key: "codec", kind: "path", required: true, placeholder: "H264" },
      { key: "page", kind: "query", placeholder: "1" },
      { key: "page_size", kind: "query", placeholder: "50" },
      { key: "media_type", kind: "query", placeholder: "Movie|Episode" },
    ],
  },
  {
    id: "stats-items-by-quality",
    category: "Stats",
    method: "GET",
    path: "/stats/items/by-quality/:quality",
    description: "List items by quality bucket.",
    usage: "Inventory by resolution.",
    params: [
      { key: "quality", kind: "path", required: true, placeholder: "4K|1080p|720p" },
      { key: "page", kind: "query", placeholder: "1" },
      { key: "page_size", kind: "query", placeholder: "50" },
      { key: "media_type", kind: "query", placeholder: "Movie|Episode" },
    ],
  },
  {
    id: "stats-movies",
    category: "Stats",
    method: "GET",
    path: "/stats/movies",
    description: "Movies library summary.",
    usage: "Totals and breakdowns for movies.",
  },
  {
    id: "stats-series",
    category: "Stats",
    method: "GET",
    path: "/stats/series",
    description: "Series library summary.",
    usage: "Totals and breakdowns for series.",
  },

  // Items & images
  {
    id: "items-by-ids",
    category: "Items",
    method: "GET",
    path: "/items/by-ids",
    description: "Batch item fetch by Emby IDs.",
    usage: "Resolve names and types for item IDs.",
    params: [{ key: "ids", kind: "query", required: true, placeholder: "id1,id2,id3" }],
  },
  {
    id: "img-primary",
    category: "Images",
    method: "GET",
    path: "/img/primary/:id",
    description: "Primary poster image.",
    usage: "Direct image link for posters.",
    params: [{ key: "id", kind: "path", required: true, placeholder: "item-id" }],
    binary: true,
  },
  {
    id: "img-backdrop",
    category: "Images",
    method: "GET",
    path: "/img/backdrop/:id",
    description: "Backdrop (fanart) image.",
    usage: "Direct image link for backdrops.",
    params: [{ key: "id", kind: "path", required: true, placeholder: "item-id" }],
    binary: true,
  },

  // Settings (API)
  {
    id: "api-settings",
    category: "Settings",
    method: "GET",
    path: "/api/settings",
    description: "Get application settings.",
    usage: "Retrieve current settings configuration.",
  },
  {
    id: "api-settings-update",
    category: "Settings",
    method: "PUT",
    path: "/api/settings/:key",
    description: "Update a setting value (admin).",
    usage: "Change feature flags or options. Protected.",
    params: [
      { key: "key", kind: "path", required: true, placeholder: "include_trakt_items" },
      { key: "value", kind: "body", required: true, placeholder: "true|false|string" },
    ],
  },

  // Now Playing
  {
    id: "now-snapshot-multi",
    category: "Now",
    method: "GET",
    path: "/api/now/snapshot",
    description: "Current active sessions across all configured servers.",
    usage: "Optionally filter by ?server=<server_id> (e.g., default-emby).",
    params: [
      { key: "server", kind: "query", required: false, placeholder: "emby|plex|jellyfin|all" },
    ],
  },
  {
    id: "now-snapshot",
    category: "Now",
    method: "GET",
    path: "/now/snapshot",
    description: "Current active sessions snapshot.",
    usage: "Populate Now Playing card.",
  },
  {
    id: "now-ws",
    category: "Now",
    method: "GET",
    path: "/now/ws",
    description: "WebSocket stream of active sessions.",
    usage: "Live updates every poll.",
    note: "Open in a WS client or via the UI card. Not runnable here.",
  },
  {
    id: "now-ws-multi",
    category: "Now",
    method: "GET",
    path: "/api/now/ws",
    description: "WebSocket stream of active sessions across servers.",
    usage: "Live updates with optional ?server=emby|plex|jellyfin|all.",
    note: "Open in a WS client or via updated UI. Not runnable here.",
  },
  {
    id: "now-pause",
    category: "Now",
    method: "POST",
    path: "/now/:id/pause",
    description: "Pause a session by SessionId.",
    usage: "Moderation or quick control.",
    params: [{ key: "id", kind: "path", required: true, placeholder: "session-id" }],
  },
  {
    id: "now-stop",
    category: "Now",
    method: "POST",
    path: "/now/:id/stop",
    description: "Stop a session by SessionId.",
    usage: "Moderation or quick control.",
    params: [{ key: "id", kind: "path", required: true, placeholder: "session-id" }],
  },
  // Body expects: { message: string }
  // Add 'message' as a body parameter so the explorer can send it
  {
    id: "now-message",
    category: "Now",
    method: "POST",
    path: "/now/:id/message",
    description: "Send on-screen message to session.",
    usage: "Inform users about maintenance, etc.",
    params: [
      { key: "id", kind: "path", required: true, placeholder: "session-id" },
      { key: "header", kind: "body", required: false, placeholder: "Emby Analytics" },
      { key: "text", kind: "body", required: true, placeholder: "Hello there 👋" },
      { key: "timeout_ms", kind: "body", required: false, placeholder: "5000" },
    ],
  },

  // Now Playing (multi-server controls)
  {
    id: "now-pause-server",
    category: "Now",
    method: "POST",
    path: "/api/now/sessions/:server/:id/pause",
    description: "Pause or resume a session on a specific server.",
    usage: "Multi-server aware moderation.",
    params: [
      { key: "server", kind: "path", required: true, placeholder: "emby|plex|jellyfin" },
      { key: "id", kind: "path", required: true, placeholder: "session-id" },
      { key: "paused", kind: "body", required: false, placeholder: "true|false" },
    ],
  },
  {
    id: "now-stop-server",
    category: "Now",
    method: "POST",
    path: "/api/now/sessions/:server/:id/stop",
    description: "Stop a session on a specific server.",
    usage: "Multi-server aware moderation.",
    params: [
      { key: "server", kind: "path", required: true, placeholder: "emby|plex|jellyfin" },
      { key: "id", kind: "path", required: true, placeholder: "session-id" },
    ],
  },
  {
    id: "now-message-server",
    category: "Now",
    method: "POST",
    path: "/api/now/sessions/:server/:id/message",
    description: "Send an on-screen message to a session on a specific server.",
    usage: "Inform users across servers.",
    params: [
      { key: "server", kind: "path", required: true, placeholder: "emby|plex|jellyfin" },
      { key: "id", kind: "path", required: true, placeholder: "session-id" },
      { key: "header", kind: "body", required: false, placeholder: "Emby Analytics" },
      { key: "text", kind: "body", required: true, placeholder: "Hello there 👋" },
      { key: "timeout_ms", kind: "body", required: false, placeholder: "5000" },
    ],
  },

  // Servers
  {
    id: "servers-list",
    category: "Servers",
    method: "GET",
    path: "/api/servers",
    description: "List configured media servers with health status.",
    usage: "Verify connectivity and IDs for server filtering.",
  },

  // Admin - Refresh & scheduler
  {
    id: "admin-refresh-start",
    category: "Admin",
    method: "POST",
    path: "/admin/refresh/start",
    description: "Full library refresh (rebuild index).",
    usage: "Initial import or resync. Protected.",
    dangerous: true,
  },
  {
    id: "admin-refresh-incremental",
    category: "Admin",
    method: "POST",
    path: "/admin/refresh/incremental",
    description: "Incremental refresh (new/changed).",
    usage: "Lightweight maintenance. Protected.",
  },
  {
    id: "admin-refresh-status",
    category: "Admin",
    method: "GET",
    path: "/admin/refresh/status",
    description: "Current refresh job status.",
    usage: "Track progress. Protected.",
  },
  {
    id: "admin-scheduler-stats",
    category: "Admin",
    method: "GET",
    path: "/admin/scheduler/stats",
    description: "Scheduler last/next runs stats.",
    usage: "Monitoring scheduled syncs. Protected.",
  },

  // Admin - Maintenance & cleanup
  {
    id: "admin-reset-all",
    category: "Admin",
    method: "POST",
    path: "/admin/reset-all",
    description: "Wipe analytics data (library + sessions).",
    usage: "Dangerous reset; requires reimport. Protected.",
    dangerous: true,
  },
  {
    id: "admin-reset-lifetime",
    category: "Admin",
    method: "POST",
    path: "/admin/reset-lifetime",
    description: "Reset lifetime watch counters only.",
    usage: "Recompute lifetime from intervals. Protected.",
    dangerous: true,
  },
  {
    id: "admin-fix-pos-units",
    category: "Admin",
    method: "ALL",
    path: "/admin/fix-pos-units",
    description: "Normalize position tick units.",
    usage: "Data hygiene if needed. Protected.",
  },
  {
    id: "admin-recover-intervals",
    category: "Admin",
    method: "POST",
    path: "/admin/recover-intervals",
    description: "Rebuild missing intervals from events.",
    usage: "Repair after crashes. Protected.",
  },
  {
    id: "admin-cleanup-intervals-dedupe",
    category: "Admin",
    method: "POST",
    path: "/admin/cleanup/intervals/dedupe",
    description: "Deduplicate overlapping intervals.",
    usage: "Cleanup duplicates. Protected.",
  },
  {
    id: "admin-cleanup-intervals-dedupe-get",
    category: "Admin",
    method: "GET",
    path: "/admin/cleanup/intervals/dedupe",
    description: "Dry-run dedupe info (if supported).",
    usage: "Inspect dedupe before applying. Protected.",
  },
  {
    id: "admin-cleanup-intervals-superset",
    category: "Admin",
    method: "POST",
    path: "/admin/cleanup/intervals/superset",
    description: "Remove session-spanning superset intervals.",
    usage: "Cleanup legacy fallback intervals. Protected.",
    dangerous: true,
  },
  {
    id: "admin-cleanup-intervals-superset-get",
    category: "Admin",
    method: "GET",
    path: "/admin/cleanup/intervals/superset",
    description: "Run cleanup via GET (alias).",
    usage: "Same as POST; convenience. Protected.",
    dangerous: true,
  },
  {
    id: "admin-fix-fallback-intervals",
    category: "Admin",
    method: "POST",
    path: "/admin/cleanup/intervals/fix-fallback",
    description:
      "Clamp legacy fallback intervals that over-count paused time using position ticks and item runtime.",
    usage:
      "Dry-run by default. To apply, set dry_run=false. Optionally tune slack_seconds (default 120).",
    params: [
      { key: "dry_run", kind: "query", placeholder: "true|false" },
      { key: "slack_seconds", kind: "query", placeholder: "120" },
    ],
    dangerous: true,
  },
  {
    id: "admin-backfill-playmethods",
    category: "Admin",
    method: "POST",
    path: "/admin/cleanup/backfill-playmethods",
    description: "Backfill per-stream methods for historical sessions.",
    usage: "Fix bubble accuracy on old rows. Protected.",
    params: [{ key: "days", kind: "query", placeholder: "90" }],
  },

  // Admin - Webhook & users
  {
    id: "admin-webhook-stats",
    category: "Admin",
    method: "GET",
    path: "/admin/webhook/stats",
    description: "Webhook endpoint info.",
    usage: "Configure Emby webhooks. Protected.",
  },
  {
    id: "admin-users-force-sync",
    category: "Admin",
    method: "POST",
    path: "/admin/users/force-sync",
    description: "Force a users sync from Emby.",
    usage: "Refresh user list. Protected.",
  },
  {
    id: "admin-debug-users",
    category: "Admin",
    method: "GET",
    path: "/admin/debug/users",
    description: "Debug listing of Emby users.",
    usage: "Inspect mapped users. Protected.",
  },

  // Admin - Debug (added)
  {
    id: "admin-debug-sessions",
    category: "Admin",
    method: "GET",
    path: "/admin/debug/sessions",
    description: "List recent play_sessions with flexible filters.",
    usage: "Troubleshoot session storage.",
    params: [
      { key: "q", kind: "query", placeholder: "title substring" },
      { key: "session_id", kind: "query", placeholder: "session id" },
      { key: "item_id", kind: "query", placeholder: "item id" },
      { key: "days", kind: "query", placeholder: "1" },
      { key: "activeOnly", kind: "query", placeholder: "true|false" },
      { key: "limit", kind: "query", placeholder: "100" },
    ],
  },
  {
    id: "admin-debug-emby-sessions",
    category: "Admin",
    method: "GET",
    path: "/admin/debug/emby-sessions",
    description: "Current sessions direct from Emby.",
    usage: "Compare Emby vs DB.",
  },
  {
    id: "admin-debug-ingest-active",
    category: "Admin",
    method: "POST",
    path: "/admin/debug/ingest-active",
    description: "Upsert rows for current active sessions.",
    usage: "Backfill when an item switch was missed.",
    dangerous: false,
  },
  {
    id: "admin-debug-item-intervals",
    category: "Admin",
    method: "GET",
    path: "/admin/debug/item-intervals/:id",
    description: "Inspect raw intervals for an item with per-session coalescing.",
    usage: "Troubleshoot inflated totals for a specific title.",
    params: [
      { key: "id", kind: "path", required: true, placeholder: "item-id" },
      { key: "days", kind: "query", placeholder: "14" },
    ],
  },

  // Admin - System monitoring (newly added)
  {
    id: "admin-metrics",
    category: "Admin",
    method: "GET",
    path: "/admin/metrics",
    description: "System performance metrics and database connection pool stats.",
    usage: "Monitor system health and performance. Protected.",
  },

  // Admin - Diagnostics (media metadata coverage)
  {
    id: "admin-diag-coverage",
    category: "Admin/Diagnostics",
    method: "GET",
    path: "/admin/diagnostics/media-field-coverage",
    description: "Counts of items with runtime, size, bitrate, width/height, codec.",
    usage: "Verify library metadata coverage. Protected.",
  },
  {
    id: "admin-diag-missing-runtime",
    category: "Admin/Diagnostics",
    method: "GET",
    path: "/admin/diagnostics/items/missing",
    description: "List items missing a field.",
    usage: "Audit missing metadata. Protected.",
    params: [
      {
        key: "field",
        kind: "query",
        required: true,
        placeholder: "run_time_ticks|file_size_bytes|bitrate_bps|width|height|video_codec",
      },
      { key: "media_type", kind: "query", placeholder: "Movie|Episode" },
      { key: "limit", kind: "query", placeholder: "50" },
    ],
  },
];

function substitutePath(path: string, values: Record<string, string>) {
  let out = path;
  Object.entries(values).forEach(([k, v]) => {
    out = out.replace(new RegExp(`:${k}\\b`, "g"), encodeURIComponent(v));
  });
  return out;
}

export default function APIExplorerPage() {
  const [filter, setFilter] = useState("");
  type EndpointState = {
    inputs?: Record<string, string>;
    request?: { method: string; url: string; body?: Record<string, string> };
    status?: string;
    ms?: number;
    output?: unknown;
  };
  const [state, setState] = useState<Record<string, EndpointState>>({});
  const [busy, setBusy] = useState<string | null>(null);

  const filtered = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return endpoints;
    return endpoints.filter((e) =>
      [e.id, e.category, e.path, e.description, e.usage].some((t) => t.toLowerCase().includes(q))
    );
  }, [filter]);

  async function run(ep: Endpoint) {
    // Build path with path params
    const inputs = (state[ep.id]?.inputs ?? {}) as Record<string, string>;
    const path = substitutePath(ep.path, inputs);
    // Build query string
    const qs = new URLSearchParams();
    ep.params?.forEach((p) => {
      if (p.kind === "query") {
        const v = inputs[p.key]?.trim();
        if (v) qs.set(p.key, v);
      }
    });
    const url = `${API_BASE}${path}${qs.toString() ? `?${qs.toString()}` : ""}`;

    // Build JSON body if present
    const bodyParams = Object.fromEntries(
      (ep.params || [])
        .filter((p) => p.kind === "body")
        .map((p) => [p.key, inputs[p.key]])
        .filter(([_, v]) => v !== undefined && v !== "") as [string, string][]
    );
    const hasBody =
      Object.keys(bodyParams).length > 0 && (ep.method === "POST" || ep.method === "PUT");

    // Don’t try to fetch binary here
    if (ep.binary) {
      window.open(url, "_blank");
      return;
    }
    if (ep.note && ep.note.includes("Not runnable")) return;

    if (ep.dangerous) {
      // Stronger confirmation for full reset
      if (ep.id === "admin-reset-all") {
        const txt = window.prompt(
          "This will WIPE analytics data (library + sessions). Type RESET to confirm."
        );
        if (!txt || txt.trim().toUpperCase() !== "RESET") return;
      } else {
        const ok = window.confirm(
          `This action may modify data. Proceed with ${ep.method} ${ep.path}?`
        );
        if (!ok) return;
      }
    }

    setBusy(ep.id);
    const t0 = performance.now();
    try {
      const res = await fetch(url, {
        method: ep.method === "ALL" ? "GET" : ep.method,
        headers: {
          "Content-Type": "application/json",
          ...adminAuthHeaderFor(ep.path),
        },
        body: hasBody ? JSON.stringify(bodyParams) : undefined,
      });
      const dt = Math.round(performance.now() - t0);
      const text = await res.text();
      let body: unknown = null;
      try {
        body = JSON.parse(text);
      } catch {
        body = text;
      }
      setState((s) => ({
        ...s,
        [ep.id]: {
          ...(s[ep.id] || {}),
          request: {
            method: ep.method === "ALL" ? "GET" : ep.method,
            url,
            body: hasBody ? bodyParams : undefined,
          },
          status: `${res.status} ${res.statusText}`,
          ms: dt,
          output: body,
        },
      }));
    } catch (e: unknown) {
      setState((s) => ({
        ...s,
        [ep.id]: {
          ...(s[ep.id] || {}),
          status: "error",
          ms: 0,
          output: String((e as Error)?.message || e),
        },
      }));
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="p-6 space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Link
            href="/"
            className="text-xs px-2 py-1 rounded bg-neutral-700 text-gray-200 hover:bg-neutral-600"
          >
            ← Back
          </Link>
          <h2 className="text-xl font-semibold text-white">API Explorer</h2>
        </div>
        <input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Search endpoints…"
          className="bg-neutral-800 border border-neutral-700 rounded px-3 py-2 text-white text-sm w-80"
        />
      </div>

      <div className="grid md:grid-cols-2 gap-4">
        {filtered.map((ep) => {
          const st = state[ep.id] || {};
          return (
            <div key={ep.id} className="bg-neutral-800 rounded-2xl p-4 border border-neutral-700">
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <span
                    className={`text-xs font-semibold px-2 py-1 rounded ${ep.method === "GET" ? "bg-green-600/20 text-green-400 border border-green-600/40" : ep.method === "POST" || ep.method === "PUT" ? "bg-blue-600/20 text-blue-400 border border-blue-600/40" : "bg-gray-600/20 text-gray-300 border border-gray-600/40"}`}
                  >
                    {ep.method}
                  </span>
                  <code className="text-sm text-white">{ep.path}</code>
                </div>
                <button
                  className={`text-xs px-3 py-1 rounded ${ep.binary || (ep.note && ep.note.includes("Not runnable")) ? "bg-neutral-700 text-gray-400 cursor-not-allowed" : "bg-neutral-700 hover:bg-neutral-600 text-white"} ${ep.dangerous ? "border border-red-500/40 text-red-300" : "border border-neutral-600"}`}
                  onClick={() => run(ep)}
                  disabled={!!busy || ep.binary || (ep.note && ep.note.includes("Not runnable"))}
                >
                  {busy === ep.id ? "Running…" : ep.binary ? "Open" : "Run"}
                </button>
              </div>
              <div className="text-sm text-gray-300">{ep.description}</div>
              <div className="text-xs text-gray-400 mb-2">Use: {ep.usage}</div>
              {ep.note && <div className="text-xs text-amber-300 mb-2">Note: {ep.note}</div>}

              {/* Params */}
              {(ep.params?.length ?? 0) > 0 && (
                <div className="mt-2 grid grid-cols-2 gap-2">
                  {ep.params!.map((p) => (
                    <div key={p.key} className="flex flex-col">
                      <label className="text-xs text-gray-400 mb-1">
                        {p.label || p.key} {p.required && <span className="text-red-400">*</span>}
                      </label>
                      <input
                        onChange={(e) =>
                          setState((s) => ({
                            ...s,
                            [ep.id]: {
                              ...(s[ep.id] || {}),
                              inputs: { ...(s[ep.id]?.inputs || {}), [p.key]: e.target.value },
                            },
                          }))
                        }
                        placeholder={p.placeholder}
                        className="bg-neutral-900 border border-neutral-700 rounded px-2 py-1 text-white text-xs"
                      />
                    </div>
                  ))}
                </div>
              )}

              {/* Output */}
              {st.status && (
                <div className="mt-3 text-xs">
                  {st.request && (
                    <div className="text-gray-400 mb-1">
                      Request: <span className="text-white">{st.request.method}</span>{" "}
                      <code className="text-white break-all">{st.request.url}</code>
                      {st.request.body && (
                        <>
                          <br />
                          Body:{" "}
                          <code className="text-white">{JSON.stringify(st.request.body)}</code>
                        </>
                      )}
                    </div>
                  )}
                  <div className="text-gray-400 mb-1">
                    Response: <span className="text-white">{st.status}</span>
                    {typeof st.ms === "number" && (
                      <span className="text-gray-500"> · {st.ms} ms</span>
                    )}
                  </div>
                  <pre className="bg-neutral-900 border border-neutral-700 rounded p-2 overflow-x-auto whitespace-pre-wrap text-gray-200 max-h-64">
                    {typeof st.output === "string" ? st.output : JSON.stringify(st.output, null, 2)}
                  </pre>
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
