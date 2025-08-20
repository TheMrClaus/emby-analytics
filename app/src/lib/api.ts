// app/src/lib/api.ts
import {
  ActiveUserLifetime,
  CodecBuckets,
  NowEntry,
  OverviewData,
  QualityBuckets,
  RefreshState,
  TopItem,
  TopUser,
  UsageRow,
} from "../types";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";

// Generic JSON fetch helper
async function j<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`${res.status} ${res.statusText} ${text}`);
  }
  return res.json() as Promise<T>;
}

// GET helpers
export const fetchOverview = () => j<OverviewData>("/stats/overview");
export const fetchUsage = (days = 14) =>
  j<UsageRow[]>(`/stats/usage?days=${days}`);
export const fetchTopUsers = (days = 14, limit = 10) =>
  j<TopUser[]>(`/stats/top/users?days=${days}&limit=${limit}`);
export const fetchTopItems = (days = 14, limit = 10) =>
  j<TopItem[]>(`/stats/top/items?days=${days}&limit=${limit}`);
export const fetchQualities = () => j<QualityBuckets>("/stats/qualities");
export const fetchCodecs = () => j<CodecBuckets>("/stats/codecs");
export const fetchActiveUsersLifetime = (limit = 10) =>
  j<ActiveUserLifetime[]>(`/stats/active-users?limit=${limit}`);
export const fetchTotalUsers = () => j<number>("/stats/users/total");

// Admin refresh
export const startRefresh = () =>
  j<{ ok: boolean }>("/admin/refresh", { method: "POST" });

export const fetchRefreshStatus = () => j<RefreshState>("/admin/refresh/status");

// Now Playing snapshot (HTTP)
export const fetchNowSnapshot = () => j<NowEntry[]>("/now/snapshot");

// Image helpers
export const imgPrimary = (id: string) => `${API_BASE}/img/primary/${id}`;
export const imgBackdrop = (id: string) => `${API_BASE}/img/backdrop/${id}`;
