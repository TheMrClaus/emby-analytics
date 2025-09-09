// app/src/components/SWRProvider.tsx
"use client";

import { SWRConfig } from "swr";
import { ReactNode } from "react";

interface SWRProviderProps {
  children: ReactNode;
}

export default function SWRProvider({ children }: SWRProviderProps) {
  return (
    <SWRConfig
      value={{
        // Global configuration for all SWR hooks
        revalidateOnFocus: false,
        revalidateOnReconnect: true,
        refreshInterval: 0, // Disable by default, enable per-hook as needed
        dedupingInterval: 2000, // Prevent duplicate requests within 2 seconds
        errorRetryCount: 3,
        errorRetryInterval: 5000,

        // More aggressive error handling
        shouldRetryOnError: (error) => {
          // Don't retry on 4xx client errors, but retry on network/5xx errors
          if (error?.message?.includes("40")) return false;
          if (error?.message?.includes("401")) return false;
          if (error?.message?.includes("403")) return false;
          if (error?.message?.includes("404")) return false;
          return true;
        },

        // Global error handler with better reporting
        onError: (error, key) => {
          const timestamp = new Date().toISOString();
          const errorMsg = error?.message || "Unknown error";
          console.error(`[${timestamp}] SWR Error for "${key}":`, errorMsg);

          // Store error details for debugging
          if (typeof window !== "undefined") {
            const errors = JSON.parse(localStorage.getItem("swr_errors") || "[]");
            errors.push({
              timestamp,
              key,
              error: errorMsg,
            });
            // Keep only last 10 errors
            localStorage.setItem("swr_errors", JSON.stringify(errors.slice(-10)));
          }
        },

        // Enhanced success handler for monitoring
        onSuccess: (data, key) => {
          const timestamp = new Date().toISOString();

          // Always log successful data fetches in a more structured way
          if (process.env.NODE_ENV === "development") {
            console.log(
              `[${timestamp}] SWR Success for "${key}":`,
              Array.isArray(data) ? `${data.length} items` : typeof data
            );
          }

          // Clear any previous errors for this key
          if (typeof window !== "undefined") {
            const errors = JSON.parse(localStorage.getItem("swr_errors") || "[]");
            const filteredErrors = errors.filter((e: any) => e.key !== key);
            if (filteredErrors.length !== errors.length) {
              localStorage.setItem("swr_errors", JSON.stringify(filteredErrors));
            }
          }
        },
      }}
    >
      {children}
    </SWRConfig>
  );
}
