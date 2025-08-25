// app/src/components/QualitiesChart.tsx
import { useEffect, useMemo, useState } from "react";
import { ResponsiveContainer, CartesianGrid, BarChart, Bar, XAxis, YAxis, Tooltip, Legend } from "recharts";
import { fetchQualities } from "../lib/api";
import type { QualityBuckets } from "../types";
import { fmtInt } from "../lib/format";
import ChartLegend from "./charts/Legend";
import { colors } from '../theme/colors';

type QualityRow = { label: string; Movie: number; Episode: number };

export default function QualitiesChart() {
  const [data, setData] = useState<QualityBuckets | null>(null);
  useEffect(() => {
    fetchQualities().then(setData).catch(() => {});
  }, []);

const rows = useMemo<QualityRow[]>(() => {
  if (!data) return [];
  return Object.entries(data.buckets).map(([label, v]) => ({
    label,
    Movie: v.Movie,
    Episode: v.Episode,
  }));
}, [data]);

  return (
    <div className="bg-neutral-800 rounded-2xl p-4 shadow">
      <div className="text-sm text-gray-400 mb-2">Media Quality</div>
      <div style={{ height: 280 }}>
        <ResponsiveContainer width="100%" height={300}>
          <BarChart data={rows} barCategoryGap={12} barGap={4} maxBarSize={44}>
            {/* Premium defs */}
            <defs>
              {/* rich gold gradient for Movie */}
              <linearGradient id="barGold" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={colors.gold600} />
                <stop offset="100%" stopColor={colors.gold400} />
              </linearGradient>
            </defs>

            <CartesianGrid vertical={false} stroke={colors.grid} />
            <XAxis dataKey="quality" tick={{ fill: colors.axis }} axisLine={{ stroke: colors.grid }} tickLine={{ stroke: colors.grid }} />
            <YAxis tick={{ fill: colors.axis }} axisLine={{ stroke: colors.grid }} tickLine={{ stroke: colors.grid }} />
            <Tooltip
              wrapperStyle={{ borderRadius: 12, overflow: 'hidden' }}
              contentStyle={{ background: colors.tooltipBg, border: `1px solid ${colors.tooltipBorder}` }}
              labelStyle={{ color: colors.gold500 }}
              itemStyle={{ color: '#fff' }}
            />

            {/* Bars: Movie = gold gradient; Episode = black with subtle stroke */}
            <Bar
              dataKey="Movie"
              fill="url(#barGold)"
              radius={[8, 8, 0, 0]}
              stroke="rgba(0,0,0,0.1)"
              strokeWidth={0.5}
            />
            <Bar
              dataKey="Episode"
              fill={colors.ink}
              radius={[8, 8, 0, 0]}
              stroke="var(--ink-raise)"
              strokeWidth={1}
            />

            <Legend
              verticalAlign="bottom"
              align="center"
              content={
                <ChartLegend
                  items={[
                    { label: 'Movie', color: colors.gold500, gradientId: 'barGold' },
                    { label: 'Episode', color: colors.ink },
                  ]}
                />
              }
            />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

