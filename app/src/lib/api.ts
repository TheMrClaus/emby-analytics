// app/src/lib/api.ts
import {
  ActiveUserLifetime,
  CodecBuckets,
  ItemRow,
  MovieStats,
  SeriesStats,
  NowEntry,
  NowPlayingSummary,
  OverviewData,
  QualityBuckets,
  RefreshState,
  TopItem,
  TopUser,
  UsageRow,
  UserDetail,
  RuntimeOutlierResponse,
} from "../types";
import type { ServerAlias } from "../types/multi-server";

const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";

// --- Admin token handling ---
// Source order: localStorage (runtime) -> NEXT_PUBLIC_ADMIN_TOKEN (build time)
const ADMIN_TOKEN_STORAGE_KEY = "emby_admin_token";

function appendServerParam(path: string, server?: ServerAlias | string) {
  if (!server || server === "all") return path;
  const sep = path.includes("?") ? "&" : "?";
  return `${path}${sep}server=${encodeURIComponent(server)}`;
}

function readAdminToken(): string | null {
  try {
    if (typeof window !== "undefined") {
      const t = window.localStorage.getItem(ADMIN_TOKEN_STORAGE_KEY);
      if (t) return t;
    }
  } catch {
    /* ignore */
  }
  return process.env.NEXT_PUBLIC_ADMIN_TOKEN ?? null;
}

export function setAdminToken(token: string) {
  if (typeof window !== "undefined") {
    window.localStorage.setItem(ADMIN_TOKEN_STORAGE_KEY, token);
  }
}

export function clearAdminToken() {
  if (typeof window !== "undefined") {
    window.localStorage.removeItem(ADMIN_TOKEN_STORAGE_KEY);
  }
}

// Generic JSON fetch helper
async function j<T>(path: string, init?: RequestInit): Promise<T> {
  const isAdmin = path.startsWith("/admin");
  const maybeToken = isAdmin ? readAdminToken() : null;
  const authHeaders: Record<string, string> = {};
  if (maybeToken) {
    authHeaders["Authorization"] = `Bearer ${maybeToken}`;
  }

  const res = await fetch(`${API_BASE}${path}`, {
    credentials: isAdmin ? "include" : "same-origin",
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...authHeaders,
      ...(init?.headers ?? {}),
    },
  });
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new Error(`${res.status} ${res.statusText} ${text}`);
  }
  // Handle no-content responses (e.g., 204) and empty bodies
  if (res.status === 204) {
    return undefined as unknown as T;
  }
  const ct = res.headers.get("content-type") || "";
  if (!ct.toLowerCase().includes("application/json")) {
    // If not JSON or empty, try to read text and return undefined
    const text = await res.text().catch(() => "");
    if (!text) return undefined as unknown as T;
    try {
      return JSON.parse(text) as T;
    } catch {
      return undefined as unknown as T;
    }
  }
  return (await res.json()) as T;
}

// GET helpers
export const fetchOverview = () => j<OverviewData>("/stats/overview");
export const fetchUsage = (days = 14) => j<UsageRow[]>(`/stats/usage?days=${days}`);
export const fetchTopUsers = (days = 14, limit = 10, timeframe?: string) => {
  if (timeframe) {
    return j<TopUser[]>(`/stats/top/users?timeframe=${timeframe}&limit=${limit}`);
  }
  return j<TopUser[]>(`/stats/top/users?days=${days}&limit=${limit}`);
};
export const fetchTopItems = (
  days = 14,
  limit = 10,
  timeframe?: string,
  server?: ServerAlias | string
) => {
  let path: string;
  if (timeframe) {
    path = `/stats/top/items?timeframe=${timeframe}&limit=${limit}`;
  } else {
    path = `/stats/top/items?days=${days}&limit=${limit}`;
  }
  return j<TopItem[]>(appendServerParam(path, server));
};
export const fetchQualities = (server?: ServerAlias | string) =>
  j<QualityBuckets>(appendServerParam("/stats/qualities", server));
export const fetchCodecs = (server?: ServerAlias | string) =>
  j<CodecBuckets>(appendServerParam("/stats/codecs", server));
export const fetchActiveUsersLifetime = (limit = 10) =>
  j<ActiveUserLifetime[]>(`/stats/active-users?limit=${limit}`);
export const fetchMovieStats = (server?: ServerAlias | string) =>
  j<MovieStats>(appendServerParam("/stats/movies", server));
export const fetchSeriesStats = (server?: ServerAlias | string) =>
  j<SeriesStats>(appendServerParam("/stats/series", server));
export const fetchTotalUsers = () => j<number>("/stats/users/total");

export interface MediaServerInfo {
  id: string;
  type: string;
  name: string;
  enabled: boolean;
  health?: {
    is_reachable?: boolean;
    error?: string;
  };
}

export const fetchServers = () => j<MediaServerInfo[]>("/api/servers");
export const fetchUserDetail = (userId: string, days = 30, limit = 10) =>
  j<UserDetail>(`/stats/users/${userId}?days=${days}&limit=${limit}`);
export const fetchItemsByIds = (ids: string[]) =>
  j<ItemRow[]>(`/items/by-ids?ids=${encodeURIComponent(ids.join(","))}`);
export const fetchRuntimeOutliers = (limit = 50) =>
  j<RuntimeOutlierResponse>(`/admin/library/runtime-outliers?limit=${limit}`);
type SessionDetail = {
  item_name: string;
  item_type: string;
  item_id: string;
  device_id: string;
  device_name: string;
  client_name: string;
  video_method: string;
  audio_method: string;
  subtitle_transcode: boolean;
  user_id: string;
  user_name: string;
  started_at: number;
  ended_at?: number | null;
  session_id: string;
  play_method: string;
};

export const fetchPlayMethods = (
  days = 30,
  options?: {
    limit?: number;
    offset?: number;
    show_all?: boolean;
    user_id?: string;
    media_type?: string;
  }
) => {
  const params = new URLSearchParams({
    days: days.toString(),
  });

  if (options?.limit) params.append("limit", options.limit.toString());
  if (options?.offset) params.append("offset", options.offset.toString());
  if (options?.show_all) params.append("show_all", options.show_all.toString());
  if (options?.user_id) params.append("user_id", options.user_id);
  if (options?.media_type) params.append("media_type", options.media_type);

  return j<{
    methods: Record<string, number>;
    detailed: Record<string, number>;
    transcodeDetails: Record<string, number>;
    sessionDetails: SessionDetail[];
    days: number;
    pagination: {
      limit: number;
      offset: number;
      count: number;
    };
  }>(`/stats/play-methods?${params}`);
};

// Admin refresh
export const startRefresh = () =>
  j<{ started: boolean }>("/admin/refresh/start", { method: "POST" });

export const syncAllServers = () => j<{ started: boolean }>("/admin/sync/all", { method: "POST" });

export const syncServer = (serverId: string) =>
  j<{ started: boolean }>(`/admin/sync/server/${encodeURIComponent(serverId)}`, {
    method: "POST",
  });

export const deleteServerMedia = (serverId: string) =>
  j<{ success: boolean }>(`/admin/server/${encodeURIComponent(serverId)}/media`, {
    method: "DELETE",
  });

export const fetchRefreshStatus = () => j<RefreshState>("/admin/refresh/status");

// Now Playing snapshot (HTTP)
export const fetchNowSnapshot = () => j<NowEntry[]>("/now/snapshot");

// Now Playing summary metrics (lightweight)
export const fetchNowPlayingSummary = () => j<NowPlayingSummary>("/api/now-playing/summary");

// Image helpers
export const imgPrimary = (id: string) => `${API_BASE}/img/primary/${id}`;
export const imgBackdrop = (id: string) => `${API_BASE}/img/backdrop/${id}`;

export interface LibraryItemResponse {
  id: string;
  name: string;
  media_type: string;
  height?: number;
  width?: number;
  codec: string;
}

export interface ItemsByCodecResponse {
  items: LibraryItemResponse[];
  total: number;
  codec: string;
  page: number;
  page_size: number;
}

export async function fetchItemsByCodec(
  codec: string,
  page: number = 1,
  pageSize: number = 50,
  mediaType?: string
): Promise<ItemsByCodecResponse> {
  const params = new URLSearchParams({
    page: page.toString(),
    page_size: pageSize.toString(),
  });

  if (mediaType) {
    params.append("media_type", mediaType);
  }

  const response = await fetch(`/stats/items/by-codec/${encodeURIComponent(codec)}?${params}`);
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  return response.json();
}

export interface GenreListItem extends LibraryItemResponse {
  genres?: string[];
}

export interface ItemsByGenreResponse {
  items: GenreListItem[];
  total: number;
  genre: string;
  page: number;
  page_size: number;
}

export async function fetchItemsByGenre(
  genre: string,
  page: number = 1,
  pageSize: number = 50,
  mediaType?: string
): Promise<ItemsByGenreResponse> {
  const params = new URLSearchParams({
    page: page.toString(),
    page_size: pageSize.toString(),
  });

  if (mediaType) {
    params.append("media_type", mediaType);
  }

  const response = await fetch(`/stats/items/by-genre/${encodeURIComponent(genre)}?${params}`);
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  return response.json();
}

export interface SeriesRow {
  id: string;
  name: string;
  genres?: string[];
  height?: number;
  width?: number;
  codec?: string;
}

export interface SeriesByGenreResponse {
  items: SeriesRow[];
  total: number;
  genre: string;
  page: number;
  page_size: number;
}

export async function fetchSeriesByGenre(
  genre: string,
  page: number = 1,
  pageSize: number = 50
): Promise<SeriesByGenreResponse> {
  const params = new URLSearchParams({
    page: page.toString(),
    page_size: pageSize.toString(),
  });
  const res = await fetch(`/stats/series/by-genre/${encodeURIComponent(genre)}?${params}`);
  if (!res.ok) throw new Error(`HTTP error! status: ${res.status}`);
  return res.json();
}

export interface ItemsByQualityResponse {
  items: LibraryItemResponse[];
  total: number;
  quality: string;
  height_range: string;
  page: number;
  page_size: number;
}

export async function fetchItemsByQuality(
  quality: string,
  page: number = 1,
  pageSize: number = 50,
  mediaType?: string
): Promise<ItemsByQualityResponse> {
  const params = new URLSearchParams({
    page: page.toString(),
    page_size: pageSize.toString(),
  });

  if (mediaType) {
    params.append("media_type", mediaType);
  }

  const response = await fetch(`/stats/items/by-quality/${encodeURIComponent(quality)}?${params}`);
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  return response.json();
}

export interface ConfigResponse {
  emby_external_url: string;
  emby_server_id: string;
  plex_external_url?: string;
  jellyfin_external_url?: string;
  plex_server_id?: string;
}

export const fetchConfig = async (): Promise<ConfigResponse> => {
  const response = await fetch(`${API_BASE}/config`);
  if (!response.ok) {
    throw new Error("Failed to fetch config");
  }
  return response.json();
};

// Version info
export interface VersionInfo {
  version: string;
  commit: string;
  date: string;
  repo?: string;
  url?: string;
  latest_tag?: string;
  latest_url?: string;
  update_available?: boolean;
}

export const fetchVersion = () => j<VersionInfo>("/version");

// ---- App Users (Admin) ----
export type AppUser = {
  id: number;
  username: string;
  role: "admin" | "user" | string;
  created_at?: string;
};

export const fetchAppUsers = () => j<AppUser[]>("/admin/app-users");

export const createAppUser = (username: string, password: string, role: "admin" | "user") =>
  j<AppUser>("/admin/app-users", {
    method: "POST",
    body: JSON.stringify({ username, password, role }),
  });

export const updateAppUser = (
  id: number,
  patch: Partial<{ username: string; password: string; role: "admin" | "user" }>
) =>
  j<AppUser>(`/admin/app-users/${id}`, {
    method: "PUT",
    body: JSON.stringify(patch),
  });

export const deleteAppUser = (id: number) =>
  j<void>(`/admin/app-users/${id}`, { method: "DELETE" });
