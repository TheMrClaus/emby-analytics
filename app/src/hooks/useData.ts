// app/src/hooks/useData.ts
import useSWR from "swr";
import {
  fetchOverview,
  fetchUsage,
  fetchTopUsers,
  fetchTopItems,
  fetchQualities,
  fetchCodecs,
  fetchActiveUsersLifetime,
  fetchMovieStats,
  fetchSeriesStats,
  fetchNowSnapshot,
  fetchRefreshStatus,
  fetchUserDetail,
  fetchVersion,
} from "../lib/api";
import type {
  OverviewData,
  UsageRow,
  TopUser,
  TopItem,
  QualityBuckets,
  CodecBuckets,
  ActiveUserLifetime,
  MovieStats,
  SeriesStats,
  NowEntry,
  RefreshState,
  UserDetail,
} from "../types";
import type { VersionInfo } from "../lib/api";
import type { ServerAlias } from "../types/multi-server";

// SWR configuration
const config = {
  revalidateOnFocus: false,
  revalidateOnReconnect: true,
  dedupingInterval: 2000, // Prevent duplicate requests within 2 seconds
};

// Overview data hook
export function useOverview() {
  return useSWR<OverviewData>("overview", () => fetchOverview(), config);
}

// Usage data hook with dynamic days parameter
export function useUsage(days = 14) {
  return useSWR<UsageRow[]>(["usage", days], () => fetchUsage(days), config);
}

// Top users hook with dynamic parameters + optional timeframe
export function useTopUsers(days = 14, limit = 10, timeframe?: string) {
  return useSWR<TopUser[]>(
    ["topUsers", days, limit, timeframe],
    () => fetchTopUsers(days, limit, timeframe),
    config
  );
}

// Top items hook with dynamic parameters + optional timeframe
export function useTopItems(
  days = 14,
  limit = 10,
  timeframe?: string,
  server?: ServerAlias | string
) {
  return useSWR<TopItem[]>(
    ["topItems", days, limit, timeframe, server ?? "all"],
    () => fetchTopItems(days, limit, timeframe, server),
    config
  );
}

// Qualities data hook
export function useQualities(server?: ServerAlias | string) {
  return useSWR<QualityBuckets>(
    ["qualities", server ?? "all"],
    () => fetchQualities(server),
    config
  );
}

// Codecs data hook
export function useCodecs(server?: ServerAlias | string) {
  return useSWR<CodecBuckets>(["codecs", server ?? "all"], () => fetchCodecs(server), config);
}

// Active users lifetime hook with dynamic limit
export function useActiveUsersLifetime(limit = 10) {
  return useSWR<ActiveUserLifetime[]>(
    ["activeUsersLifetime", limit],
    () => fetchActiveUsersLifetime(limit),
    config
  );
}

// Movie stats hook
export function useMovieStats(server?: ServerAlias | string) {
  return useSWR<MovieStats>(["movieStats", server ?? "all"], () => fetchMovieStats(server), config);
}

// Series stats hook
export function useSeriesStats(server?: ServerAlias | string) {
  return useSWR<SeriesStats>(
    ["seriesStats", server ?? "all"],
    () => fetchSeriesStats(server),
    config
  );
}

// User detail hook
export function useUserDetail(userId: string | null, days = 30, limit = 10) {
  return useSWR<UserDetail>(
    userId ? ["userDetail", userId, days, limit] : null,
    () => (userId ? fetchUserDetail(userId, days, limit) : null),
    config
  );
}

// Now playing snapshot with frequent refresh
export function useNowSnapshot() {
  return useSWR<NowEntry[]>("nowSnapshot", () => fetchNowSnapshot(), {
    ...config,
    refreshInterval: 5000, // Refresh every 5 seconds for real-time data
  });
}

// Refresh status with polling when refreshing
export function useRefreshStatus(enabled = true) {
  return useSWR<RefreshState>(enabled ? "refreshStatus" : null, () => fetchRefreshStatus(), {
    ...config,
    refreshInterval: 1000, // Poll every second when checking refresh status
  });
}

// App/version info
export function useVersion() {
  return useSWR<VersionInfo>("version", () => fetchVersion(), {
    ...config,
    revalidateOnFocus: true,
    dedupingInterval: 60000,
  });
}
