// app/src/hooks/useData.ts
import useSWR from 'swr';
import {
  fetchOverview,
  fetchUsage,
  fetchTopUsers,
  fetchTopItems,
  fetchQualities,
  fetchCodecs,
  fetchActiveUsersLifetime,
  fetchNowSnapshot,
  fetchRefreshStatus,
} from '../lib/api';
import type {
  OverviewData,
  UsageRow,
  TopUser,
  TopItem,
  QualityBuckets,
  CodecBuckets,
  ActiveUserLifetime,
  NowEntry,
  RefreshState,
} from '../types';

// SWR configuration
const config = {
  revalidateOnFocus: false,
  revalidateOnReconnect: true,
  dedupingInterval: 2000, // Prevent duplicate requests within 2 seconds
};

// Generic fetcher that matches SWR's expected signature
const fetcher = (fn: () => Promise<any>) => fn();

// Overview data hook
export function useOverview() {
  return useSWR<OverviewData>('overview', () => fetchOverview(), config);
}

// Usage data hook with dynamic days parameter
export function useUsage(days = 14) {
  return useSWR<UsageRow[]>(
    ['usage', days],
    () => fetchUsage(days),
    config
  );
}

// Top users hook with dynamic parameters + optional timeframe
export function useTopUsers(days = 14, limit = 10, timeframe?: string) {
  return useSWR<TopUser[]>(
    ['topUsers', days, limit, timeframe],
    () => fetchTopUsers(days, limit, timeframe),
    config
  );
}

// Top items hook with dynamic parameters + optional timeframe  
export function useTopItems(days = 14, limit = 10, timeframe?: string) {
  return useSWR<TopItem[]>(
    ['topItems', days, limit, timeframe],
    () => fetchTopItems(days, limit, timeframe),
    config
  );
}

// Qualities data hook
export function useQualities() {
  return useSWR<QualityBuckets>('qualities', () => fetchQualities(), config);
}

// Codecs data hook
export function useCodecs() {
  return useSWR<CodecBuckets>('codecs', () => fetchCodecs(), config);
}

// Active users lifetime hook with dynamic limit
export function useActiveUsersLifetime(limit = 10) {
  return useSWR<ActiveUserLifetime[]>(
    ['activeUsersLifetime', limit],
    () => fetchActiveUsersLifetime(limit),
    config
  );
}

// Now playing snapshot with frequent refresh
export function useNowSnapshot() {
  return useSWR<NowEntry[]>(
    'nowSnapshot',
    () => fetchNowSnapshot(),
    {
      ...config,
      refreshInterval: 5000, // Refresh every 5 seconds for real-time data
    }
  );
}

// Refresh status with polling when refreshing
export function useRefreshStatus(enabled = true) {
  return useSWR<RefreshState>(
    enabled ? 'refreshStatus' : null,
    () => fetchRefreshStatus(),
    {
      ...config,
      refreshInterval: 1000, // Poll every second when checking refresh status
    }
  );
}