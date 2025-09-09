import useSWR from "swr";

// Admin token handling (same pattern as api.ts)
const ADMIN_TOKEN_STORAGE_KEY = "emby_admin_token";

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

export interface Setting {
  key: string;
  value: string;
  updated_at: string;
}

// Fetch function for settings
const fetchSettings = async (): Promise<Setting[]> => {
  const response = await fetch("/api/settings");
  if (!response.ok) {
    throw new Error("Failed to fetch settings");
  }
  return response.json();
};

export function useSettings() {
  const { data, error, mutate } = useSWR<Setting[]>("/api/settings", fetchSettings);

  const updateSetting = async (key: string, value: string) => {
    // Optimistic update
    const currentData = data || [];
    const optimisticData = currentData.map((setting) =>
      setting.key === key ? { ...setting, value, updated_at: new Date().toISOString() } : setting
    );

    // If setting doesn't exist, add it
    if (!currentData.find((s) => s.key === key)) {
      optimisticData.push({
        key,
        value,
        updated_at: new Date().toISOString(),
      });
    }

    // Update SWR cache optimistically
    mutate(optimisticData, false);

    try {
      // Make API call with admin authentication
      const maybeToken = readAdminToken();
      const authHeaders: Record<string, string> = {};
      if (maybeToken) {
        authHeaders["Authorization"] = `Bearer ${maybeToken}`;
      }

      const response = await fetch(`/api/settings/${key}`, {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          ...authHeaders,
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
    mutate,
  };
}
