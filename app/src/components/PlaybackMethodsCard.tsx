import React, { useEffect, useMemo, useState } from 'react';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from 'recharts';
import { colors } from '../theme/colors';
import { fetchPlayMethods, fetchConfig } from '../lib/api';
import { openInEmby } from '../lib/emby';

type SessionDetail = {
  item_name: string;
  item_type: string;
  item_id: string;
  device_id: string;
  client_name: string;
  video_method: string;
  audio_method: string;
};

type PlayMethodResponse = {
  methods: Record<string, number>;
  detailed: Record<string, number>;
  transcodeDetails: Record<string, number>;
  sessionDetails: SessionDetail[];
  days: number;
};

const timeframeOptions = [
  { value: "all-time", label: "All Time" },
  { value: "30d", label: "30 Days" },
  { value: "14d", label: "14 Days" },
  { value: "7d", label: "7 Days" },
  { value: "3d", label: "3 Days" },
  { value: "1d", label: "1 Day" },
];

export default function PlaybackMethodsCard() {
  const [data, setData] = useState<PlayMethodResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showDetailed, setShowDetailed] = useState(false);
  const [timeframe, setTimeframe] = useState("30d");
  const [embyExternalUrl, setEmbyExternalUrl] = useState<string>('');
  const [embyServerId, setEmbyServerId] = useState<string>('');

  useEffect(() => {
    const days = timeframe === "all-time" ? 0 : parseInt(timeframe.replace("d", "")) || 30;
    fetchPlayMethods(days).then(setData).catch((e) => setError(e?.message || 'Failed to load playback methods'));
  }, [timeframe]);

  // Fetch config once on component mount to get Emby external URL
  useEffect(() => {
    fetchConfig()
      .then((config) => {
        setEmbyExternalUrl(config.emby_external_url);
        setEmbyServerId(config.emby_server_id);
      })
      .catch((e) => console.error('Failed to fetch config:', e));
  }, []);

  // Summary chart data - only DirectPlay vs Transcode
  const summaryChartData = useMemo(() => {
    const methods = data?.methods || {};
    return [
      { name: 'DirectPlay', value: methods.DirectPlay || 0, color: '#22c55e' }, // green-500 to match Now Playing
      { name: 'Transcode', value: methods.Transcode || 0, color: '#f97316' }      // orange-500 to match Now Playing
    ].filter(d => d.value > 0);
  }, [data]);

  // Detailed transcode breakdown
  const transcodeBreakdown = useMemo(() => {
    if (!data?.transcodeDetails) return [];
    const details = data.transcodeDetails;
    return [
      { name: 'Video Transcode', value: details.TranscodeVideo || 0 },
      { name: 'Audio Transcode', value: details.TranscodeAudio || 0 },
      { name: 'Subtitle Transcode', value: details.TranscodeSubtitle || 0 }
    ].filter(d => d.value > 0);
  }, [data]);

  const selectedOption = timeframeOptions.find((opt) => opt.value === timeframe);
  const total = summaryChartData.reduce((a, b) => a + b.value, 0);

  const handleChartClick = () => {
    setShowDetailed(true);
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
        <div className="text-sm text-gray-400">
          Playback Methods ({selectedOption?.label})
        </div>
        {showDetailed ? (
          <button
            onClick={() => setShowDetailed(false)}
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
          <div className="h-64 cursor-pointer" onClick={handleChartClick} title="Click to view detailed breakdown">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={summaryChartData} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
                <XAxis 
                  dataKey="name" 
                  tick={{ fontSize: 12 }}
                />
                <YAxis tick={{ fontSize: 12 }} />
                <Tooltip
                  contentStyle={{ 
                    background: colors.tooltipBg, 
                    border: `1px solid ${colors.tooltipBorder}`, 
                    borderRadius: 12 
                  }}
                  formatter={(val: any) => [`${val} sessions`, '']}
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
          {/* Summary Stats */}
          <div className="grid grid-cols-3 gap-4 text-center text-sm">
            {transcodeBreakdown.map((item, idx) => (
              <div key={idx} className="bg-neutral-700/50 rounded p-3">
                <div className="text-white font-bold text-lg">{item.value}</div>
                <div className="text-gray-400">{item.name}</div>
              </div>
            ))}
          </div>

          {/* Session Details */}
          <div>
            <div className="text-sm text-gray-300 mb-3">Recent Transcode Sessions:</div>
            <div className="space-y-2 max-h-64 overflow-y-auto">
              {(data?.sessionDetails || []).map((session, idx) => (
                <div key={idx} className="flex items-center justify-between py-3 px-4 bg-neutral-700/30 rounded hover:bg-neutral-700/50 transition-colors">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-3">
                      <div className="flex-1 min-w-0">
                        <div 
                          className="font-medium text-white truncate cursor-pointer hover:text-blue-400 transition-colors" 
                          onClick={() => openInEmby(session.item_id, embyExternalUrl, embyServerId)}
                          title="Click to open in Emby"
                        >
                          {session.item_name || 'Unknown Media'}
                        </div>
                        <div className="text-xs text-gray-400 mt-1">
                          {session.item_type} • {session.client_name || session.device_id}
                        </div>
                      </div>
                      <div className="flex gap-2 shrink-0">
                        {session.video_method === 'Transcode' && (
                          <span className="px-2 py-1 bg-orange-500/20 text-orange-400 border border-orange-400/30 rounded text-xs">
                            Video
                          </span>
                        )}
                        {session.audio_method === 'Transcode' && (
                          <span className="px-2 py-1 bg-orange-500/20 text-orange-400 border border-orange-400/30 rounded text-xs">
                            Audio
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                </div>
              ))}
              {(!data?.sessionDetails || data.sessionDetails.length === 0) && (
                <div className="text-gray-500 text-center py-6">No recent transcode sessions found</div>
              )}
            </div>
          </div>
          
          <div className="text-white/70 text-sm border-t border-neutral-700 pt-2">
            Total sessions: <span className="text-white">{total}</span>
          </div>
        </div>
      )}
    </div>
  );
}