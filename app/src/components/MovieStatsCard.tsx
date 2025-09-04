import { useMovieStats } from "../hooks/useData";
import { DataState, useDataState } from "./DataState";
import { fmtInt, fmtHours } from "../lib/format";
import Card from "./ui/Card";

export default function MovieStatsCard() {
  const swrResponse = useMovieStats();
  const { data, error, isLoading, hasData } = useDataState(swrResponse);

  return (
    <DataState
      isLoading={isLoading}
      error={error}
      data={data}
      errorFallback={
        <Card title="Movie Statistics">
          <div className="text-red-300 text-sm">Unable to load movie stats</div>
          <div className="text-xs text-red-400 mt-1">Check server connection</div>
        </Card>
      }
      loadingFallback={
        <Card title="Movie Statistics">
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
        <Card title="Movie Statistics">
          <div className="grid grid-cols-2 lg:grid-cols-3 gap-4">
            {/* Total Movies */}
            <StatItem
              label="Total Movies"
              value={fmtInt(data.total_movies)}
              isStale={data.total_movies === 0}
            />

            {/* Movies Added This Month */}
            <StatItem
              label="Added This Month"
              value={fmtInt(data.movies_added_this_month)}
            />

            {/* Total Runtime */}
            <StatItem
              label="Total Runtime"
              value={fmtHours(data.total_runtime_hours)}
            />

            {/* Largest Movie */}
            <StatItem
              label="Largest Movie"
              value={`${data.largest_movie_gb.toFixed(1)} GB`}
              subtitle={data.largest_movie_name}
            />

            {/* Longest Movie */}
            <StatItem
              label="Longest Movie"
              value={`${Math.floor(data.longest_runtime_minutes / 60)}h ${data.longest_runtime_minutes % 60}m`}
              subtitle={data.longest_movie_name}
            />

            {/* Shortest Movie */}
            <StatItem
              label="Shortest Movie"
              value={`${Math.floor(data.shortest_runtime_minutes / 60)}h ${data.shortest_runtime_minutes % 60}m`}
              subtitle={data.shortest_movie_name}
            />

            {/* Newest Movie */}
            <StatItem
              label="Newest Added"
              value={new Date(data.newest_movie.date).toLocaleDateString()}
              subtitle={data.newest_movie.name}
            />

            {/* Most Watched */}
            <StatItem
              label="Most Watched"
              value={fmtHours(data.most_watched_movie.hours)}
              subtitle={data.most_watched_movie.name}
            />
          </div>

          {/* Popular Genres */}
          {data.popular_genres && data.popular_genres.length > 0 && (
            <div className="mt-6">
              <h3 className="text-sm font-medium text-gray-400 mb-3">Popular Genres</h3>
              <div className="flex flex-wrap gap-2">
                {data.popular_genres.map((genre) => (
                  <div 
                    key={genre.genre}
                    className="bg-blue-900/30 border border-blue-500/30 rounded-lg px-3 py-1 text-sm"
                  >
                    <span className="text-blue-200">{genre.genre}</span>
                    <span className="text-blue-300 ml-1">({fmtInt(genre.count)})</span>
                  </div>
                ))}
              </div>
            </div>
          )}
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
