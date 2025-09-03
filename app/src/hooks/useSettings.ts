import useSWR from 'swr';
import { fetcher, apiPost } from '../lib/api';

export interface Setting {
  key: string;
  value: string;
  updated_at: string;
}

export function useSettings() {
  const { data, error, mutate } = useSWR<Setting[]>('/settings', fetcher);

  const updateSetting = async (key: string, value: string) => {
    // Optimistic update
    const currentData = data || [];
    const optimisticData = currentData.map(setting => 
      setting.key === key 
        ? { ...setting, value, updated_at: new Date().toISOString() }
        : setting
    );

    // If setting doesn't exist, add it
    if (!currentData.find(s => s.key === key)) {
      optimisticData.push({
        key,
        value,
        updated_at: new Date().toISOString()
      });
    }

    // Update SWR cache optimistically
    mutate(optimisticData, false);

    try {
      // Make API call
      const response = await fetch(`/settings/${key}`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ value }),
      });

      if (!response.ok) {
        throw new Error(`Failed to update setting: ${response.statusText}`);
      }

      // Revalidate to get the latest data from server
      await mutate();
    } catch (error) {
      // Revert optimistic update on error
      mutate();
      throw error;
    }
  };

  return {
    data,
    error,
    isLoading: !error && !data,
    updateSetting,
    mutate
  };
}