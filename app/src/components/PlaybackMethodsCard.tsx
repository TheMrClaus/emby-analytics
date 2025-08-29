import React, { useEffect, useMemo, useState } from 'react';
import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Legend, Cell } from 'recharts';
import { colors } from '../theme/colors';
import { fetchPlayMethods } from '../lib/api';

type PlayMethodResponse = {
  methods: Record<string, number>;
  detailed: Record<string, number>;
  days: number;
};

const METHOD_ORDER = ['DirectPlay', 'AudioOnly', 'VideoOnly', 'BothTranscode', 'Unknown'] as const;

export default function PlaybackMethodsCard() {
  const [data, setData] = useState<PlayMethodResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [showDetailed, setShowDetailed] = useState(false);

  useEffect(() => {
    fetchPlayMethods().then(setData).catch((e) => setError(e?.message || 'Failed to load playback methods'));
  }, []);

  const chartData = useMemo(() => {
    const methods = data?.methods || {};
    return METHOD_ORDER.map((method) => ({
      name: method === 'BothTranscode' ? 'Video + Audio' : 
            method === 'VideoOnly' ? 'Video Transcode' :
            method === 'AudioOnly' ? 'Audio Transcode' : method,
      value: methods[method] ?? 0,
      originalKey: method
    })).filter((d) => d.value > 0);
  }, [data]);

  const detailedData = useMemo(() => {
    if (!data?.detailed) return [];
    return Object.entries(data.detailed)
      .map(([key, value]) => {
        const [video, audio] = key.split('|');
        return {
          combination: `${video === 'DirectPlay' ? 'Direct' : 'Transcode'} Video / ${audio === 'DirectPlay' ? 'Direct' : 'Transcode'} Audio`,
          sessions: value,
          video,
          audio
        };
      })
      .filter(d => d.sessions > 0)
      .sort((a, b) => b.sessions - a.sessions);
  }, [data]);

  const palette: Record<string, string> = {
    DirectPlay: colors.gold600,
    AudioOnly: colors.gold400, 
    VideoOnly: '#f97316',
    BothTranscode: '#ef4444',
    Unknown: 'rgba(255,255,255,0.35)',
  };

  const total = chartData.reduce((a, b) => a + b.value, 0);

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
          Playback Methods (last {data?.days || 30} days)
        </div>
        <button
          onClick={() => setShowDetailed(!showDetailed)}
          className="text-xs px-2 py-1 rounded bg-neutral-700 text-gray-300 hover:bg-neutral-600 transition-colors"
        >
          {showDetailed ? 'Summary' : 'Detailed'}
        </button>
      </div>

      {!showDetailed ? (
        <>
          <div className="h-64">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={chartData} margin={{ top: 5, right: 30, left: 20, bottom: 5 }}>
                <XAxis 
                  dataKey="name" 
                  tick={{ fontSize: 12 }}
                  angle={-45}
                  textAnchor="end"
                  height={60}
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
                  {chartData.map((entry, idx) => (
                    <Cell key={`bar-${idx}`} fill={palette[entry.originalKey] ?? colors.gold500} />
                  ))}
                </Bar>
              </BarChart>
            </ResponsiveContainer>
          </div>
          <div className="mt-3 text-white/70 text-sm">
            Total sessions: <span className="text-white">{total}</span>
          </div>
        </>
      ) : (
        <div className="space-y-3">
          <div className="text-sm text-gray-300 mb-3">Detailed Breakdown:</div>
          <div className="space-y-2 max-h-64 overflow-y-auto">
            {detailedData.map((item, idx) => (
              <div key={idx} className="flex items-center justify-between py-2 px-3 bg-neutral-700/50 rounded">
                <div className="text-sm text-gray-300">{item.combination}</div>
                <div className="text-white font-medium">{item.sessions}</div>
              </div>
            ))}
          </div>
          <div className="mt-3 text-white/70 text-sm border-t border-neutral-700 pt-2">
            Total sessions: <span className="text-white">{total}</span>
          </div>
        </div>
      )}
    </div>
  );
}