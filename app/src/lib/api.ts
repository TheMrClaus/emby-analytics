// app/src/lib/api.ts
import {
  ActiveUserLifetime,
  CodecBuckets,
  ItemRow,
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
export const fetchTopUsers = (days = 14, limit = 10, timeframe?: string) => {
  if (timeframe) {
    return j<TopUser[]>(`/stats/top/users?timeframe=${timeframe}&limit=${limit}`);
  }
  return j<TopUser[]>(`/stats/top/users?days=${days}&limit=${limit}`);
};
export const fetchTopItems = (days = 14, limit = 10, timeframe?: string) => {
  if (timeframe) {
    return j<TopItem[]>(`/stats/top/items?timeframe=${timeframe}&limit=${limit}`);
  }
  return j<TopItem[]>(`/stats/top/items?days=${days}&limit=${limit}`);
};
export const fetchQualities = () => j<QualityBuckets>("/stats/qualities");
export const fetchCodecs = () => j<CodecBuckets>("/stats/codecs");
export const fetchActiveUsersLifetime = (limit = 10) =>
  j<ActiveUserLifetime[]>(`/stats/active-users?limit=${limit}`);
export const fetchTotalUsers = () => j<number>("/stats/users/total");
export const fetchItemsByIds = (ids: string[]) =>
  j<ItemRow[]>(`/items/by-ids?ids=${encodeURIComponent(ids.join(","))}`);
type SessionDetail = {
  item_name: string;
  item_type: string;
  item_id: string;
  device_id: string;
  client_name: string;
  video_method: string;
  audio_method: string;
};

export const fetchPlayMethods = (days = 30) =>
  j<{ 
    methods: Record<string, number>;
    detailed: Record<string, number>;
    transcodeDetails: Record<string, number>;
    sessionDetails: SessionDetail[];
    days: number;
  }>(`/stats/play-methods?days=${days}`);

// Admin refresh
export const startRefresh = () =>
  j<{ started: boolean }>("/admin/refresh/start", { method: "POST" });

export const fetchRefreshStatus = () => j<RefreshState>("/admin/refresh/status");

// Now Playing snapshot (HTTP)
export const fetchNowSnapshot = () => j<NowEntry[]>("/now/snapshot");

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
    params.append('media_type', mediaType);
  }
  
  const response = await fetch(`/stats/items/by-codec/${encodeURIComponent(codec)}?${params}`);
  if (!response.ok) {
    throw new Error(`HTTP error! status: ${response.status}`);
  }
  return response.json();
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
    params.append('media_type', mediaType);
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
}

export const fetchConfig = async (): Promise<ConfigResponse> => {
  const response = await fetch(`${API_BASE}/config`);
  if (!response.ok) {
    throw new Error('Failed to fetch config');
  }
  return response.json();
};