import { useSeriesStats } from "../hooks/useData";
import { DataState, useDataState } from "./DataState";
import { fmtInt, fmtHours } from "../lib/format";
import Card from "./ui/Card";

export default function SeriesStatsCard() {
  const swrResponse = useSeriesStats();
  const { data, error, isLoading, hasData } = useDataState(swrResponse);

  return (
    <DataState
      isLoading={isLoading}
      error={error}
      data={data}
      errorFallback={
        <Card title="Series Statistics">
          <div className="text-red-300 text-sm">Unable to load series stats</div>
          <div className="text-xs text-red-400 mt-1">Check server connection</div>
        </Card>
      }
      loadingFallback={
        <Card title="Series Statistics">
          <div className="animate-pulse">
            <div className="grid grid-cols-2 gap-4">
              {[1, 2, 3, 4, 5, 6].map(i => (
                <div key={i}>
                  <div className="h-3 bg-neutral-700 rounded w-20 mb-1"></div>
                  <div className="h-5 bg-neutral-700 rounded w-16"></div>
                </div>
              ))}
            </div>
          </div>
        </Card>
      }
    >
      {hasData && (
        <Card title="Series Statistics">
          <div className="grid grid-cols-2 lg:grid-cols-3 gap-4">
            <StatItem label="Total Series" value={fmtInt(data.total_series)} />
            <StatItem label="Total Episodes" value={fmtInt(data.total_episodes)} />

            <StatItem
              label="Largest Series (Total)"
              value={`${data.largest_series_total_gb.toFixed(1)} GB`}
              subtitle={data.largest_series_name}
            />

            <StatItem
              label="Largest Episode"
              value={`${data.largest_episode_gb.toFixed(1)} GB`}
              subtitle={data.largest_episode_name}
            />

            <StatItem
              label="Longest Series"
              value={`${Math.floor(data.longest_series_runtime_minutes / 60)}h ${data.longest_series_runtime_minutes % 60}m`}
              subtitle={data.longest_series_name}
            />

            <StatItem
              label="Most Watched Series"
              value={fmtHours(data.most_watched_series.hours)}
              subtitle={data.most_watched_series.name}
            />

            <StatItem
              label="Time to Watch All TV"
              value={fmtHours(data.total_episode_runtime_hours)}
            />

            <StatItem
              label="Newest Added Series"
              value={new Date(data.newest_series.date).toLocaleDateString()}
              subtitle={data.newest_series.name}
            />

            <StatItem
              label="Episodes Added This Month"
              value={fmtInt(data.episodes_added_this_month)}
            />
          </div>
        </Card>
      )}
    </DataState>
  );
}

function StatItem({ 
  label, 
  value, 
  subtitle, 
  isStale 
}: { 
  label: string; 
  value: string; 
  subtitle?: string; 
  isStale?: boolean; 
}) {
  return (
    <div className={`${isStale ? 'text-yellow-300' : 'text-white'}`}>
      <div className={`text-xs ${isStale ? 'text-yellow-400' : 'text-gray-400'} mb-1`}>
        {label}
        {isStale && <span className="ml-1">⚠️</span>}
      </div>
      <div className={`font-semibold ${isStale ? 'text-yellow-300' : 'text-white'}`}>
        {value}
      </div>
      {subtitle && (
        <div className={`text-xs mt-1 ${isStale ? 'text-yellow-400' : 'text-gray-500'} truncate`} 
             title={subtitle}>
          {subtitle}
        </div>
      )}
      {isStale && (
        <div className="text-xs text-yellow-400 mt-1">No data available</div>
      )}
    </div>
  );
}

