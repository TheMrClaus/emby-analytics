import React, { useCallback, useEffect, useMemo, useState } from "react";
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from "recharts";
import { colors } from "../theme/colors";
import { fetchPlayMethods, fetchConfig } from "../lib/api";
import { openInEmby } from "../lib/emby";

type SessionDetail = {
  item_name: string;
  item_type: string;
  item_id: string;
  device_id: string;
  device_name: string;
  client_name: string;
  video_method: string;
  audio_method: string;
  subtitle_transcode: boolean;
  user_id: string;
  user_name: string;
  started_at: number;
  ended_at?: number | null;
  session_id: string;
  play_method: string;
};

type PlayMethodResponse = {
  methods: Record<string, number>;
  detailed: Record<string, number>;
  transcodeDetails: Record<string, number>;
  sessionDetails: SessionDetail[];
  days: number;
  pagination: {
    limit: number;
    offset: number;
    count: number;
  };
};

const timeframeOptions = [
  { value: "all-time", label: "All Time" },
  { value: "90d", label: "90 Days" },
  { value: "30d", label: "30 Days" },
  { value: "14d", label: "14 Days" },
  { value: "7d", label: "7 Days" },
  { value: "3d", label: "3 Days" },
  { value: "1d", label: "1 Day" },
];

const SESSIONS_PER_PAGE = 25;

export default function PlaybackMethodsCard() {
  const [data, setData] = useState<PlayMethodResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [showDetailed, setShowDetailed] = useState(false);
  const [timeframe, setTimeframe] = useState("30d");
  const [embyExternalUrl, setEmbyExternalUrl] = useState<string>("");
  const [embyServerId, setEmbyServerId] = useState<string>("");

  // Enhanced state for detailed view
  const [currentPage, setCurrentPage] = useState(1);
  const [searchTerm, setSearchTerm] = useState("");
  const [userFilter, setUserFilter] = useState("");
  const [showTranscodeOnly, setShowTranscodeOnly] = useState(false);
  const [allSessions, setAllSessions] = useState<SessionDetail[]>([]);
  const [totalSessions, setTotalSessions] = useState(0);
  const [allUniqueUsers, setAllUniqueUsers] = useState<Array<{ id: string; name: string }>>([]);
  const [activeFilters, setActiveFilters] = useState<Set<string>>(new Set());

  const loadData = useCallback(
    async (resetPagination = false) => {
      setLoading(true);
      setError(null);

      try {
        const days = timeframe === "all-time" ? 0 : parseInt(timeframe.replace("d", "")) || 30;
        const page = resetPagination ? 1 : currentPage;
        const offset = (page - 1) * SESSIONS_PER_PAGE;

        const result = await fetchPlayMethods(days, {
          limit: SESSIONS_PER_PAGE,
          offset: offset,
          show_all: !showTranscodeOnly,
          user_id: userFilter || undefined,
        });

        setData(result);

        // Defensive check for sessionDetails
        const sessionDetails = Array.isArray(result.sessionDetails) ? result.sessionDetails : [];

        if (resetPagination) {
          setCurrentPage(1);
          setAllSessions(sessionDetails);
        } else {
          // For pagination, we need to accumulate sessions
          if (page === 1) {
            setAllSessions(sessionDetails);
          } else {
            setAllSessions((prev) => [...prev, ...sessionDetails]);
          }
        }

        // Track unique users from all sessions ever fetched - with null check
        if (sessionDetails.length > 0) {
          const newUsers = sessionDetails.reduce(
            (acc, session) => {
              if (session.user_name && !acc.find((u) => u.id === session.user_id)) {
                acc.push({ id: session.user_id, name: session.user_name });
              }
              return acc;
            },
            [] as Array<{ id: string; name: string }>
          );

          setAllUniqueUsers((prev) => {
            const combined = [...prev];
            newUsers.forEach((newUser) => {
              if (!combined.find((u) => u.id === newUser.id)) {
                combined.push(newUser);
              }
            });
            return combined.sort((a, b) => a.name.localeCompare(b.name));
          });
        }

        // Estimate total sessions (this is a rough estimate)
        setTotalSessions(sessionDetails.length + offset);
      } catch (e: unknown) {
        setError((e as Error)?.message || "Failed to load playback methods");
      } finally {
        setLoading(false);
      }
    },
    [timeframe, currentPage, userFilter, showTranscodeOnly]
  );

  useEffect(() => {
    loadData(true);
  }, [timeframe, userFilter, showTranscodeOnly]);

  useEffect(() => {
    if (showDetailed && currentPage > 1) {
      loadData(false);
    }
  }, [currentPage]);

  // Fetch config once on component mount to get Emby external URL
  useEffect(() => {
    fetchConfig()
      .then((config) => {
        setEmbyExternalUrl(config.emby_external_url);
        setEmbyServerId(config.emby_server_id);
      })
      .catch((e) => console.error("Failed to fetch config:", e));
  }, []);

  // Summary chart data - only DirectPlay vs Transcode
  const summaryChartData = useMemo(() => {
    const methods = data?.methods || {};
    return [
      { name: "Direct", value: methods.DirectPlay || 0, color: "#22c55e" }, // green-500 to match Now Playing
      { name: "Transcode", value: methods.Transcode || 0, color: "#f97316" }, // orange-500 to match Now Playing
    ].filter((d) => d.value > 0);
  }, [data]);

  // Detailed transcode breakdown
  const transcodeBreakdown = useMemo(() => {
    if (!data?.transcodeDetails) return [];
    const details = data.transcodeDetails;
    return [
      { name: "Direct", value: details.Direct || 0 },
      { name: "Video Transcode", value: details.TranscodeVideo || 0 },
      { name: "Audio Transcode", value: details.TranscodeAudio || 0 },
      { name: "Subtitle Transcode", value: details.TranscodeSubtitle || 0 },
    ].filter((d) => d.value > 0);
  }, [data]);

  // Filtered sessions based on search term and active rectangle filters
  const filteredSessions = useMemo(() => {
    let sessions = allSessions;

    // Apply search filter
    if (searchTerm) {
      const term = searchTerm.toLowerCase();
      sessions = sessions.filter(
        (session) =>
          session.item_name?.toLowerCase().includes(term) ||
          session.user_name?.toLowerCase().includes(term) ||
          session.device_name?.toLowerCase().includes(term) ||
          session.client_name?.toLowerCase().includes(term)
      );
    }

    // Apply rectangle filters
    if (activeFilters.size > 0) {
      sessions = sessions.filter((session) => {
        if (
          activeFilters.has("Direct") &&
          session.video_method !== "Transcode" &&
          session.audio_method !== "Transcode" &&
          !session.subtitle_transcode
        ) {
          return true;
        }
        if (activeFilters.has("TranscodeVideo") && session.video_method === "Transcode") {
          return true;
        }
        if (activeFilters.has("TranscodeAudio") && session.audio_method === "Transcode") {
          return true;
        }
        if (activeFilters.has("TranscodeSubtitle") && session.subtitle_transcode) {
          return true;
        }
        return false;
      });
    }

    return sessions;
  }, [allSessions, searchTerm, activeFilters]);

  // Format date/time
  const formatDateTime = (timestamp: number) => {
    return new Date(timestamp * 1000).toLocaleString(undefined, {
      month: "short",
      day: "numeric",
      hour: "2-digit",
      minute: "2-digit",
    });
  };

  // Get unique users for filter dropdown (removed - using allUniqueUsers state instead)

  const selectedOption = timeframeOptions.find((opt) => opt.value === timeframe);
  const total = summaryChartData.reduce((a, b) => a + b.value, 0);

  const handleChartClick = () => {
    setShowDetailed(true);
    if (allSessions.length === 0) {
      loadData(true);
    }
  };

  const handleBackClick = () => {
    setShowDetailed(false);
    setSearchTerm("");
    setUserFilter("");
    setCurrentPage(1);
    setActiveFilters(new Set());
  };

  const handleRectangleClick = (filterType: string) => {
    setActiveFilters((prev) => {
      const newFilters = new Set(prev);

      // Handle Direct filter while transcode-only is checked
      if (filterType === "Direct" && showTranscodeOnly) {
        setShowTranscodeOnly(false); // Silently uncheck
        return new Set(["Direct"]);
      }

      // Toggle filter
      if (newFilters.has(filterType)) {
        newFilters.delete(filterType);
      } else {
        newFilters.add(filterType);
      }

      return newFilters;
    });
  };

  const clearAllFilters = () => {
    setActiveFilters(new Set());
  };

  const getRectangleFilterKey = (name: string) => {
    switch (name) {
      case "Direct":
        return "Direct";
      case "Video Transcode":
        return "TranscodeVideo";
      case "Audio Transcode":
        return "TranscodeAudio";
      case "Subtitle Transcode":
        return "TranscodeSubtitle";
      default:
        return name;
    }
  };

  const loadMoreSessions = () => {
    if (!loading) {
      setCurrentPage((prev) => prev + 1);
    }
  };

  const getTranscodeBubbles = (session: SessionDetail) => {
    const bubbles = [];

    if (session.video_method === "Transcode") {
      bubbles.push(
        <span
          key="video"
          className="px-2 py-1 bg-orange-500/20 text-orange-400 border border-orange-400/30 rounded text-xs"
        >
          Video
        </span>
      );
    }

    if (session.audio_method === "Transcode") {
      bubbles.push(
        <span
          key="audio"
          className="px-2 py-1 bg-orange-500/20 text-orange-400 border border-orange-400/30 rounded text-xs"
        >
          Audio
        </span>
      );
    }

    // Add subtitle bubble if this session has subtitle transcoding
    if (session.subtitle_transcode) {
      bubbles.push(
        <span
          key="subtitle"
          className="px-2 py-1 bg-orange-500/20 text-orange-400 border border-orange-400/30 rounded text-xs"
        >
          Subtitle
        </span>
      );
    }

    // Add green Direct bubble if no transcoding at all
    if (
      session.video_method !== "Transcode" &&
      session.audio_method !== "Transcode" &&
      !session.subtitle_transcode
    ) {
      bubbles.push(
        <span
          key="direct"
          className="px-2 py-1 bg-green-500/20 text-green-400 border border-green-400/30 rounded text-xs"
        >
          Direct
        </span>
      );
    }

    return bubbles;
  };

  if (error) {
    return (
      <div className="bg-neutral-800 rounded-2xl p-4 shadow inline-block w-full align-top break-inside-avoid mb-4">
        <div className="text-sm text-gray-400 mb-2">Playback Methods</div>
        <div className="text-red-400">{error}</div>
      </div>
    );
  }

  return (
    <div className="bg-neutral-800 rounded-2xl p-4 shadow inline-block w-full align-top break-inside-avoid mb-4">
      <div className="flex items-center justify-between mb-3">
        <div className="text-sm text-gray-400">Playback Methods ({selectedOption?.label})</div>
        {showDetailed ? (
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
        )}
      </div>

      {!showDetailed ? (
        <>
          <div
            className="h-64 cursor-pointer"
            onClick={handleChartClick}
            title="Click to view detailed breakdown"
          >
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={summaryChartData} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
                <XAxis dataKey="name" tick={{ fontSize: 12 }} />
                <YAxis tick={{ fontSize: 12 }} />
                <Tooltip
                  contentStyle={{
                    background: colors.tooltipBg,
                    border: `1px solid ${colors.tooltipBorder}`,
                    borderRadius: 12,
                  }}
                  formatter={(val: number | string) => [`${val} sessions`, ""]}
                />
                <Bar dataKey="value" name="Sessions">
                  {summaryChartData.map((entry, idx) => (
                    <Cell key={`bar-${idx}`} fill={entry.color} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
          <div className="mt-3 text-white/70 text-sm text-center">
            Total sessions: <span className="text-white">{total}</span>
            <br />
            <span className="text-xs text-gray-400">💡 Click chart to view detailed breakdown</span>
          </div>
        </>
      ) : (
        <div className="space-y-4">
          {/* Enhanced Controls */}
          <div className="flex flex-col sm:flex-row gap-3 mb-4">
            <div className="flex-1">
              <input
                type="text"
                placeholder="Search sessions..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
                className="w-full bg-neutral-700 text-white text-xs px-3 py-2 rounded border border-neutral-600 focus:border-blue-500 focus:outline-none"
              />
            </div>
            <select
              value={userFilter}
              onChange={(e) => setUserFilter(e.target.value)}
              className="bg-neutral-700 text-white text-xs px-3 py-2 rounded border border-neutral-600 focus:border-blue-500 focus:outline-none"
            >
              <option value="">All Users</option>
              {allUniqueUsers.map((user) => (
                <option key={user.id} value={user.id}>
                  {user.name}
                </option>
              ))}
            </select>
            <label className="flex items-center gap-2 text-xs text-gray-300">
              <input
                type="checkbox"
                checked={showTranscodeOnly}
                onChange={(e) => setShowTranscodeOnly(e.target.checked)}
                className="rounded"
              />
              Show Transcode Only
            </label>
          </div>

          {/* Summary Stats */}
          <div className="grid grid-cols-4 gap-4 text-center text-sm">
            {transcodeBreakdown.map((item, idx) => {
              const filterKey = getRectangleFilterKey(item.name);
              const isActive = activeFilters.has(filterKey);
              return (
                <div
                  key={idx}
                  onClick={() => handleRectangleClick(filterKey)}
                  className={`bg-neutral-700/50 rounded p-3 cursor-pointer transition-all hover:bg-neutral-600/50 ${
                    isActive ? "ring-2 ring-blue-500 bg-blue-500/20" : ""
                  }`}
                >
                  <div className="text-white font-bold text-lg">{item.value}</div>
                  <div className="text-gray-400">{item.name}</div>
                </div>
              );
            })}
          </div>

          {/* Clear Filters Button */}
          {activeFilters.size > 0 && (
            <div className="flex justify-center">
              <button
                onClick={clearAllFilters}
                className="text-xs px-3 py-1 rounded bg-neutral-600 text-gray-300 hover:bg-neutral-500 transition-colors"
              >
                Clear Filters ({activeFilters.size})
              </button>
            </div>
          )}

          {/* Session Details */}
          <div>
            <div className="flex items-center justify-between mb-3">
              <div className="text-sm text-gray-300">
                {showTranscodeOnly ? "Transcode Sessions" : "All Sessions"} (
                {filteredSessions.length})
              </div>
              {loading && <div className="text-xs text-gray-400">Loading...</div>}
            </div>

            <div className="space-y-2 max-h-96 overflow-y-auto">
              {filteredSessions.map((session, idx) => (
                <div
                  key={`${session.session_id}-${idx}`}
                  className="py-3 px-4 bg-neutral-700/30 rounded hover:bg-neutral-700/50 transition-colors"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex-1 min-w-0">
                      <div
                        className="font-medium text-white truncate cursor-pointer hover:text-blue-400 transition-colors mb-1"
                        onClick={() => openInEmby(session.item_id, embyExternalUrl, embyServerId)}
                        title="Click to open in Emby"
                      >
                        {session.item_name || "Unknown Media"}
                      </div>

                      <div className="text-xs text-gray-400 space-y-1">
                        <div className="flex flex-wrap gap-2">
                          <span>{session.item_type}</span>
                          <span>•</span>
                          <span>{session.user_name || session.user_id}</span>
                          <span>•</span>
                          <span>
                            {session.client_name || session.device_name || session.device_id}
                          </span>
                        </div>

                        <div className="flex flex-wrap gap-2">
                          <span>Started: {formatDateTime(session.started_at)}</span>
                          {session.ended_at && (
                            <>
                              <span>•</span>
                              <span>Ended: {formatDateTime(session.ended_at)}</span>
                            </>
                          )}
                        </div>
                      </div>
                    </div>

                    <div className="flex gap-2 shrink-0 flex-wrap">
                      {getTranscodeBubbles(session)}
                    </div>
                  </div>
                </div>
              ))}

              {filteredSessions.length === 0 && !loading && (
                <div className="text-gray-500 text-center py-6">
                  {searchTerm || userFilter
                    ? "No matching sessions found"
                    : showTranscodeOnly
                      ? "No transcode sessions found"
                      : "No sessions found"}
                </div>
              )}
            </div>

            {/* Load More Button */}
            {!searchTerm &&
              !userFilter &&
              allSessions.length >= SESSIONS_PER_PAGE &&
              data?.sessionDetails &&
              Array.isArray(data.sessionDetails) &&
              data.sessionDetails.length >= SESSIONS_PER_PAGE && (
                <div className="mt-4 text-center">
                  <button
                    onClick={loadMoreSessions}
                    disabled={loading}
                    className="px-4 py-2 bg-blue-600 text-white rounded hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed text-sm"
                  >
                    {loading ? "Loading..." : "Load More"}
                  </button>
                </div>
              )}
          </div>

          <div className="text-white/70 text-sm border-t border-neutral-700 pt-2 flex justify-between items-center">
            <span>
              Total sessions: <span className="text-white">{total}</span>
            </span>
            {filteredSessions.length !== allSessions.length && (
              <span className="text-xs">
                Showing {filteredSessions.length} of {allSessions.length}
              </span>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
