import React, { useMemo } from 'react';
import { PieChart, Pie, Cell, Tooltip, ResponsiveContainer, Legend } from 'recharts';
import { colors } from '@/src/theme/colors';
import type { PlayMethodCounts } from '@/src/types';

type Props = {
  data?: PlayMethodCounts | null;
  className?: string;
};

const SLICE_ORDER = ['DirectPlay', 'DirectStream', 'Transcode', 'Unknown'] as const;

export default function PlaybackMethodsCard({ data, className }: Props) {
  const chartData = useMemo(() => {
    const m = data?.methods || {};
    return SLICE_ORDER.map((k) => ({
      name: k,
      value: m[k] ?? 0,
    })).filter((d) => d.value > 0);
  }, [data]);

  // Black/Yellow theme friendly; warn on Transcode
  const palette: Record<string, string> = {
    DirectPlay: colors.gold600,
    DirectStream: colors.gold400,
    Transcode: '#f97316', // orange accent looks great on black/yellow
    Unknown: 'rgba(255,255,255,0.35)',
  };

  const total = chartData.reduce((a, b) => a + b.value, 0);

  return (
    <div className={`card-dark ${className ?? ''}`}>
      <div className="section-title">Playback Methods (last 30 days)</div>
      <div className="text-white/60 text-sm mb-2">
        Quick view of Direct Play vs Direct Stream vs Transcode
      </div>
      <div className="h-60">
        <ResponsiveContainer width="100%" height="100%">
          <PieChart>
            <Pie
              data={chartData}
              innerRadius="60%"
              outerRadius="90%"
              paddingAngle={2}
              dataKey="value"
              nameKey="name"
              isAnimationActive
            >
              {chartData.map((entry, idx) => (
                <Cell key={`slice-${idx}`} fill={palette[entry.name] ?? colors.gold500} />
              ))}
            </Pie>
            <Tooltip
              contentStyle={{
                background: colors.tooltipBg,
                border: `1px solid ${colors.tooltipBorder}`,
                borderRadius: 12,
              }}
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

