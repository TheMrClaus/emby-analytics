import React, { useEffect, useMemo, useState } from 'react';
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer, Legend } from 'recharts';
import { colors } from '../theme/colors';
import { fetchPlayMethods } from '../lib/api';

type PlayMethodCounts = { methods: Record<string, number> };

const SLICE_ORDER = ['DirectPlay', 'DirectStream', 'Transcode', 'Unknown'] as const;

export default function PlaybackMethodsCard() {
  const [data, setData] = useState<PlayMethodCounts | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchPlayMethods().then(setData).catch((e) => setError(e?.message || 'Failed to load playback methods'));
  }, []);

  const chartData = useMemo(() => {
    const m = data?.methods || {};
    return SLICE_ORDER.map((k) => ({ name: k, value: m[k] ?? 0 })).filter((d) => d.value > 0);
  }, [data]);

  const palette: Record<string, string> = {
    DirectPlay: colors.gold600,
    DirectStream: colors.gold400,
    Transcode: '#f97316',
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
      <div className="text-sm text-gray-400 mb-2">Playback Methods (last 30 days)</div>
      <div className="h-60">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie data={chartData} innerRadius="60%" outerRadius="90%" paddingAngle={2} dataKey="value" nameKey="name" isAnimationActive>
              {chartData.map((entry, idx) => (
                <Cell key={`slice-${idx}`} fill={palette[entry.name] ?? colors.gold500} />
              ))}
            </Pie>
            <Tooltip
              contentStyle={{ background: colors.tooltipBg, border: `1px solid ${colors.tooltipBorder}`, borderRadius: 12 }}
              formatter={(val: any, name: any) => [`${val}`, name]}
            />
            <Legend verticalAlign="bottom" height={24} />
          </PieChart>
        </ResponsiveContainer>
      </div>
      <div className="mt-3 text-white/70 text-sm">
        Total sessions: <span className="text-white">{total}</span>
      </div>
    </div>
  );
}
