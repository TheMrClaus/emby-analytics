import { useSeriesStats } from "../hooks/useData";
import { DataState, useDataState } from "./DataState";
import { fmtInt, fmtLongSpanFromMinutes, fmtLongSpanFromHours } from "../lib/format";
import Card from "./ui/Card";
import Link from "next/link";
import { useLibraryServer } from "../contexts/LibraryServerContext";

export default function SeriesStatsCard() {
  const { server } = useLibraryServer();
  const swrResponse = useSeriesStats(server);
  const { data, error, isLoading, hasData } = useDataState(swrResponse);

  const serverLabel = server === "all" ? "" : server.charAt(0).toUpperCase() + server.slice(1);

  const title = (
    <div className="flex items-center gap-2">
      <span>Series Statistics</span>
      {serverLabel && (
        <span className="text-xs text-gray-400 uppercase tracking-wide">{serverLabel}</span>
      )}
    </div>
  );

  return (
    <DataState
      isLoading={isLoading}
      error={error}
      data={data}
      errorFallback={
        <Card title={title}>
          <div className="text-red-300 text-sm">Unable to load series stats</div>
          <div className="text-xs text-red-400 mt-1">Check server connection</div>
        </Card>
      }
      loadingFallback={
        <Card title={title}>
          <div className="animate-pulse">
            <div className="grid grid-cols-2 gap-4">
              {[1, 2, 3, 4, 5, 6].map((i) => (
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
        <Card title={title}>
          <div className="grid grid-cols-2 lg:grid-cols-3 gap-4">
            {/* Consistent ordering with Movie card */}
            <StatItem label="Total Series" value={fmtInt(data.total_series)} />
            <StatItem label="Total Episodes" value={fmtInt(data.total_episodes)} />

            {/* Most Watched Series: Name - Hh Mm */}
            <StatItem
              label="Most Watched Series"
              value={`${data.most_watched_series.name} - ${fmtHoursHM(data.most_watched_series.hours)}`}
            />

            {/* Largest totals */}
            <StatItem
              label="Largest Series (Total Size)"
              value={`${data.largest_series_total_gb.toFixed(1)} GB`}
              subtitle={data.largest_series_name}
            />
            <StatItem
              label="Largest Episode"
              value={`${data.largest_episode_gb.toFixed(1)} GB`}
              subtitle={data.largest_episode_name}
            />

            {/* Runtimes */}
            <StatItem
              label="Longest Series"
              value={fmtLongSpanFromMinutes(data.longest_series_runtime_minutes)}
              subtitle={data.longest_series_name}
            />
            <StatItem
              label="Time to Watch All TV"
              value={fmtLongSpanFromHours(data.total_episode_runtime_hours)}
            />

            {/* Recency */}
            <StatItem
              label="Newest Added Episode"
              value={`${data.newest_series.name} ${new Date(data.newest_series.date).toLocaleDateString("en-US", { month: "numeric", day: "numeric", year: "numeric" })}`}
            />
            <StatItem
              label="Episodes Added This Month"
              value={fmtInt(data.episodes_added_this_month)}
            />
          </div>

          {/* Popular Genres */}
          {data.popular_genres && data.popular_genres.length > 0 && (
            <div className="mt-6">
              <h3 className="text-sm font-medium text-gray-400 mb-3">Popular Genres</h3>
              <div className="flex flex-wrap gap-2">
                {data.popular_genres.map((genre: { genre: string; count: number }) => (
                  <Link
                    key={genre.genre}
                    href={`/genres?genre=${encodeURIComponent(genre.genre)}&media_type=Series`}
                    className="bg-purple-900/30 border border-purple-500/30 rounded-lg px-3 py-1 text-sm hover:bg-purple-900/50"
                    title={`View episodes in ${genre.genre}`}
                  >
                    <span className="text-purple-200">{genre.genre}</span>
                    <span className="text-purple-300 ml-1">({fmtInt(genre.count)})</span>
                  </Link>
                ))}
              </div>
            </div>
          )}
        </Card>
      )}
    </DataState>
  );
}

function fmtHoursHM(hours: number) {
  const totalMinutes = Math.round(hours * 60);
  const h = Math.floor(totalMinutes / 60);
  const m = totalMinutes % 60;
  if (h <= 0) return `${m}m`;
  if (m <= 0) return `${h}h`;
  return `${h}h ${m}m`;
}

function StatItem({
  label,
  value,
  subtitle,
  isStale,
}: {
  label: string;
  value: string;
  subtitle?: string;
  isStale?: boolean;
}) {
  return (
    <div className={`${isStale ? "text-yellow-300" : "text-white"}`}>
      <div className={`text-xs ${isStale ? "text-yellow-400" : "text-gray-400"} mb-1`}>
        {label}
        {isStale && <span className="ml-1">⚠️</span>}
      </div>
      <div className={`font-semibold ${isStale ? "text-yellow-300" : "text-white"}`}>{value}</div>
      {subtitle && (
        <div
          className={`text-xs mt-1 ${isStale ? "text-yellow-400" : "text-gray-500"} truncate`}
          title={subtitle}
        >
          {subtitle}
        </div>
      )}
      {isStale && <div className="text-xs text-yellow-400 mt-1">No data available</div>}
    </div>
  );
}
