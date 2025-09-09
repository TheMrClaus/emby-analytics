// app/src/components/OverviewCards.tsx
import { useOverview } from "../hooks/useData";
import { DataState, useDataState } from "./DataState";
import { fmtInt } from "../lib/format";

export default function OverviewCards() {
  const swrResponse = useOverview();
  const { data, error, isLoading, hasData } = useDataState(swrResponse);

  return (
    <DataState
      isLoading={isLoading}
      error={error}
      data={data}
      errorFallback={
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="bg-red-900/20 border border-red-500/20 rounded-2xl p-4 shadow">
              <div className="text-sm text-red-400">Connection Error</div>
              <div className="text-red-300 text-sm">Unable to load stats</div>
              <div className="text-xs text-red-400 mt-1">Check server connection</div>
            </div>
          ))}
        </div>
      }
      loadingFallback={
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {[1, 2, 3, 4].map((i) => (
            <div key={i} className="bg-neutral-800 rounded-2xl p-4 shadow animate-pulse">
              <div className="h-4 bg-neutral-700 rounded w-16 mb-2"></div>
              <div className="h-8 bg-neutral-700 rounded w-20"></div>
            </div>
          ))}
        </div>
      }
    >
      {hasData && (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          <Card
            label="Users"
            value={fmtInt(data.total_users)}
            isStale={!data || data.total_users === 0}
          />
          <Card
            label="Library Items"
            value={fmtInt(data.total_items)}
            isStale={!data || data.total_items === 0}
          />
          <Card
            label="Total Plays"
            value={fmtInt(data.total_plays)}
            isStale={!data || data.total_plays === 0}
          />
          <Card
            label="Unique Plays"
            value={fmtInt(data.unique_plays)}
            isStale={!data || data.unique_plays === 0}
          />
        </div>
      )}
    </DataState>
  );
}

function Card({ label, value, isStale }: { label: string; value: string; isStale?: boolean }) {
  return (
    <div
      className={`rounded-2xl p-4 shadow ${isStale ? "bg-yellow-900/20 border border-yellow-500/20" : "bg-neutral-800"}`}
    >
      <div className={`text-sm ${isStale ? "text-yellow-400" : "text-gray-400"}`}>
        {label}
        {isStale && <span className="ml-1 text-xs">⚠️</span>}
      </div>
      <div className={`text-2xl font-bold ${isStale ? "text-yellow-300" : "text-white"}`}>
        {value}
      </div>
      {isStale && <div className="text-xs text-yellow-400 mt-1">No data available</div>}
    </div>
  );
}
