// app/src/components/SWRProvider.tsx
'use client';

import { SWRConfig } from 'swr';
import { ReactNode } from 'react';

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
        
        // Global error handler
        onError: (error, key) => {
          console.error('SWR Error:', key, error);
        },
        
        // Global success handler for debugging
        onSuccess: (data, key) => {
          // Only log in development
          if (process.env.NODE_ENV === 'development') {
            console.log('SWR Success:', key, data);
          }
        },
      }}
    >
      {children}
    </SWRConfig>
  );
}