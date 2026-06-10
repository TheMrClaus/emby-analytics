import { ReactNode } from "react";

interface DataStateProps {
  isLoading?: boolean;
  error?: Error;
  data?: unknown;
  children?: ReactNode;
  fallback?: ReactNode;
  errorFallback?: ReactNode;
  loadingFallback?: ReactNode;
}

export function DataState({
  isLoading,
  error,
  data,
  children,
  fallback,
  errorFallback,
  loadingFallback,
}: DataStateProps) {
  // Show error state
  if (error) {
    if (errorFallback) {
      return <>{errorFallback}</>;
    }

    return (
      <div className="flex items-center justify-center p-6 bg-red-900/20 border border-red-500/20 rounded-2xl">
        <div className="text-center">
          <div className="text-red-400 mb-2">
            <svg className="w-8 h-8 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          </div>
          <p className="text-red-300 font-medium">Failed to load data</p>
          <p className="text-red-400 text-sm mt-1">{error.message}</p>
          <button
            className="mt-3 px-3 py-1 text-sm bg-red-900/40 border border-red-500/40 text-red-200 rounded hover:bg-red-900/60 transition-colors"
            onClick={() => window.location.reload()}
          >
            Try Again
          </button>
        </div>
      </div>
    );
  }

  // Show loading state
  if (isLoading) {
    if (loadingFallback) {
      return <>{loadingFallback}</>;
    }

    return (
      <div className="p-6 animate-pulse" aria-busy="true" aria-live="polite">
        <div className="h-4 bg-neutral-700 rounded w-32 mb-3"></div>
        <div className="h-3 bg-neutral-700/70 rounded w-full mb-2"></div>
        <div className="h-3 bg-neutral-700/70 rounded w-2/3"></div>
        <span className="sr-only">Loading…</span>
      </div>
    );
  }

  // Show empty state (when data exists but is empty)
  if (
    data !== undefined &&
    ((Array.isArray(data) && data.length === 0) ||
      (typeof data === "object" && data !== null && Object.keys(data).length === 0))
  ) {
    if (fallback) {
      return <>{fallback}</>;
    }

    return (
      <div className="flex items-center justify-center p-6 bg-neutral-800 border border-white/10 rounded-2xl">
        <div className="text-center">
          <div className="text-gray-500 mb-2">
            <svg className="w-8 h-8 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2M4 13h2m13-8v4h-2V5h2zM6 5v4h2V5H6z"
              />
            </svg>
          </div>
          <p className="text-gray-400">No data available</p>
        </div>
      </div>
    );
  }

  // Show children (successful state with data)
  return <>{children}</>;
}

// Hook for consistent error/loading patterns with SWR
export function useDataState<T>(swrResponse: { data?: T; error?: Error; isLoading?: boolean }) {
  const { data, error } = swrResponse;
  const val = data as unknown;

  return {
    ...swrResponse,
    isEmpty:
      val !== undefined &&
      ((Array.isArray(val) && val.length === 0) ||
        (typeof val === "object" && val !== null && Object.keys(val as object).length === 0)),
    hasData: data !== undefined && !error,
    isError: !!error,
  };
}
