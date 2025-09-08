import { useState, useMemo } from "react";
import { useTopUsers, useUserDetail } from "../hooks/useData";
import { fmtTooltipTime, fmtSpanDHMW } from "../lib/format";
import Card from "./ui/Card";
import type { TopUser } from "../types";

const timeframeOptions = [
  { value: "all-time", label: "All Time" },
  { value: "30d", label: "30 Days" },
  { value: "14d", label: "14 Days" },
  { value: "7d", label: "7 Days" },
  { value: "3d", label: "3 Days" },
  { value: "1d", label: "1 Day" },
];

export default function TopUsers({ limit = 10 }: { limit?: number }) {
  const [timeframe, setTimeframe] = useState("14d");
  const [showDetailed, setShowDetailed] = useState(false);
  const [selectedUser, setSelectedUser] = useState<TopUser | null>(null);
  const [userFilter, setUserFilter] = useState<string>("");

  // Convert timeframe to days for the API (backwards compatibility)
  const days = timeframe === "all-time" ? 0 : parseInt(timeframe.replace("d", "")) || 14;

  const { data: rows = [], error, isLoading } = useTopUsers(days, limit, timeframe);
  const { data: userDetail, error: userDetailError, isLoading: userDetailLoading } = useUserDetail(
    userFilter || selectedUser?.user_id || null,
    days
  );

  if (error) {
    return (
      <Card title="Top Users">
        <div className="text-red-400">Failed to load users data</div>
      </Card>
    );
  }

  const selectedOption = timeframeOptions.find((opt) => opt.value === timeframe);

  const handleUserClick = (user: TopUser) => {
    if (user.user_id) {
      setSelectedUser(user);
      setUserFilter(user.user_id);
      setShowDetailed(true);
    }
  };

  const handleBackClick = () => {
    setShowDetailed(false);
    setSelectedUser(null);
    setUserFilter("");
  };

  // Get all users for filter dropdown
  const allUsers = useMemo(() => {
    return rows.filter(user => user.user_id).map(user => ({
      id: user.user_id!,
      name: user.name
    }));
  }, [rows]);

  // Current user for display
  const currentUser = useMemo(() => {
    const userId = userFilter || selectedUser?.user_id;
    return allUsers.find(user => user.id === userId) || selectedUser;
  }, [allUsers, userFilter, selectedUser]);

  return (
    <Card
      title={
        <>
          {showDetailed ? (
            <>
              {currentUser?.name || 'User Details'} ({selectedOption?.label})
              {userDetailLoading && <span className="ml-2 text-xs opacity-60">Loading...</span>}
            </>
          ) : (
            <>
              Top Users ({selectedOption?.label})
              {isLoading && <span className="ml-2 text-xs opacity-60">Loading...</span>}
            </>
          )}
        </>
      }
      right={
        showDetailed ? (
          <button
            onClick={handleBackClick}
            className="text-xs px-2 py-1 rounded bg-neutral-700 text-gray-300 hover:bg-neutral-600 transition-colors flex items-center gap-1"
          >
            ← Back
          </button>
        ) : (
          <select
            value={timeframe}
            onChange={(e) => setTimeframe(e.target.value)}
            className="bg-neutral-700 text-white text-xs px-2 py-1 rounded border border-neutral-600 focus:border-blue-500 focus:outline-none"
          >
            {timeframeOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        )
      }
    >
      {!showDetailed ? (
        <>
          <table className="w-full text-sm text-left text-gray-300">
            <thead className="text-gray-400 border-b border-neutral-700">
              <tr>
                <th className="py-1">User</th>
                <th className="py-1 text-right">Hours</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r, i) => (
                <tr key={i} className="border-b border-neutral-800 last:border-0">
                  <td className="py-1">
                    <span 
                      className="cursor-pointer hover:text-blue-400 transition-colors"
                      onClick={() => handleUserClick(r)}
                    >
                      {r.name}
                    </span>
                  </td>
                  <td className="py-1 text-right tabular-nums" title={fmtTooltipTime(r.hours)}>
                    {fmtSpanDHMW(r.hours)}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {rows.length === 0 && !isLoading && (
            <div className="text-gray-500 text-center py-4">No data available</div>
          )}
        </>
      ) : (
        <div className="space-y-4">
          {/* Enhanced Controls */}
          <div className="flex flex-col sm:flex-row gap-3 mb-4">
            <select
              value={timeframe}
              onChange={(e) => setTimeframe(e.target.value)}
              className="bg-neutral-700 text-white text-xs px-3 py-2 rounded border border-neutral-600 focus:border-blue-500 focus:outline-none"
            >
              {timeframeOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <select
              value={userFilter || selectedUser?.user_id || ''}
              onChange={(e) => setUserFilter(e.target.value)}
              className="bg-neutral-700 text-white text-xs px-3 py-2 rounded border border-neutral-600 focus:border-blue-500 focus:outline-none"
            >
              {allUsers.map(user => (
                <option key={user.id} value={user.id}>{user.name}</option>
              ))}
            </select>
          </div>

          {userDetailError && (
            <div className="text-red-400 text-sm">Failed to load user details</div>
          )}

          {userDetailLoading && (
            <div className="text-gray-400 text-sm">Loading user details...</div>
          )}

          {userDetail && (
            <>
              {/* Summary Stats */}
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 text-center text-sm">
                <div className="bg-neutral-700/50 rounded p-3">
                  <div className="text-white font-bold text-lg">{userDetail.total_movies}</div>
                  <div className="text-gray-400">Movies Watched</div>
                </div>
                <div className="bg-neutral-700/50 rounded p-3">
                  <div className="text-white font-bold text-lg">{userDetail.total_series_finished}</div>
                  <div className="text-gray-400">Series Finished</div>
                </div>
                <div className="bg-neutral-700/50 rounded p-3">
                  <div className="text-white font-bold text-lg">{userDetail.total_episodes}</div>
                  <div className="text-gray-400">Episodes Watched</div>
                </div>
                <div className="bg-neutral-700/50 rounded p-3">
                  <div className="text-white font-bold text-lg">{fmtSpanDHMW(userDetail.total_hours)}</div>
                  <div className="text-gray-400">Total Time Watched</div>
                </div>
              </div>

              {/* Last Seen Movies */}
              {userDetail.last_seen_movies.length > 0 && (
                <div>
                  <div className="text-sm text-gray-300 mb-3">Last Seen Movies</div>
                  <div className="space-y-2 max-h-48 overflow-y-auto">
                    {userDetail.last_seen_movies.map((movie, idx) => (
                      <div key={`${movie.item_id}-${idx}`} className="py-2 px-3 bg-neutral-700/30 rounded">
                        <div className="font-medium text-white text-sm">{movie.name}</div>
                        <div className="text-xs text-gray-400">
                          {new Date(movie.hours * 1000).toLocaleString()}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Last Seen Episodes */}
              {userDetail.last_seen_episodes && userDetail.last_seen_episodes.length > 0 && (
                <div>
                  <div className="text-sm text-gray-300 mb-3">Last Seen Episodes</div>
                  <div className="space-y-2 max-h-48 overflow-y-auto">
                    {userDetail.last_seen_episodes.map((episode, idx) => (
                      <div key={`${episode.item_id}-${idx}`} className="py-2 px-3 bg-neutral-700/30 rounded">
                        <div className="font-medium text-white text-sm">{episode.name}</div>
                        <div className="text-xs text-gray-400">
                          {new Date(episode.hours * 1000).toLocaleString()}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {/* Finished Series */}
              {userDetail.finished_series && userDetail.finished_series.length > 0 && (
                <div>
                  <div className="text-sm text-gray-300 mb-3">Finished Series ({userDetail.finished_series.length})</div>
                  <div className="space-y-2 max-h-48 overflow-y-auto">
                    {userDetail.finished_series.map((series, idx) => (
                      <div key={`${series.item_id}-${idx}`} className="py-2 px-3 bg-neutral-700/30 rounded">
                        <div className="font-medium text-white text-sm">{series.name}</div>
                        <div className="text-xs text-gray-400">
                          {series.type} • {Math.round(series.hours)} episodes watched
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      )}
    </Card>
  );
}
