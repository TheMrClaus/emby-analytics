import { ReactNode } from "react";

interface DataStateProps {
  isLoading?: boolean;
  error?: Error;
  data?: any;
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
      <div className="flex items-center justify-center p-6 bg-red-50 border border-red-200 rounded-lg">
        <div className="text-center">
          <div className="text-red-600 mb-2">
            <svg className="w-8 h-8 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
          </div>
          <p className="text-red-800 font-medium">Failed to load data</p>
          <p className="text-red-600 text-sm mt-1">{error.message}</p>
          <button
            className="mt-3 px-3 py-1 text-sm bg-red-100 text-red-700 rounded hover:bg-red-200 transition-colors"
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
      <div className="flex items-center justify-center p-6">
        <div className="text-center">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600 mx-auto mb-2"></div>
          <p className="text-gray-600">Loading...</p>
        </div>
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
      <div className="flex items-center justify-center p-6 bg-gray-50 border border-gray-200 rounded-lg">
        <div className="text-center">
          <div className="text-gray-400 mb-2">
            <svg className="w-8 h-8 mx-auto" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2M4 13h2m13-8v4h-2V5h2zM6 5v4h2V5H6z"
              />
            </svg>
          </div>
          <p className="text-gray-600">No data available</p>
        </div>
      </div>
    );
  }

  // Show children (successful state with data)
  return <>{children}</>;
}

// Hook for consistent error/loading patterns with SWR
export function useDataState(swrResponse: { data?: any; error?: Error; isLoading?: boolean }) {
  const { data, error, isLoading } = swrResponse;

  return {
    ...swrResponse,
    isEmpty:
      data !== undefined &&
      ((Array.isArray(data) && data.length === 0) ||
        (typeof data === "object" && data !== null && Object.keys(data).length === 0)),
    hasData: data !== undefined && !error,
    isError: !!error,
  };
}
