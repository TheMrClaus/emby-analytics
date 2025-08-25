// app/src/components/CodecsChart.tsx
import { useEffect, useMemo, useState } from "react";
import { ResponsiveContainer, CartesianGrid, BarChart, Bar, XAxis, YAxis, Tooltip, Legend } from "recharts";
import { fetchCodecs } from "../lib/api";
import type { CodecBuckets } from "../types";
import { fmtInt } from "../lib/format";
import ChartLegend from './charts/Legend';
import { colors } from '../theme/colors';

export default function CodecsChart() {
  const [data, setData] = useState<CodecBuckets | null>(null);
  useEffect(() => {
    fetchCodecs().then(setData).catch(() => {});
  }, []);

  const rows = useMemo(() => {
    if (!data) return [];
    return Object.entries(data.codecs).map(([codec, v]) => ({
      codec,
      Movie: v.Movie,
      Episode: v.Episode,
    }));
  }, [data]);

  return (
    <div className="card p-4">
      <div className="h3 mb-2">Codecs</div>
      <div style={{ height: 280 }}>
        <ResponsiveContainer width="100%" height={300}>
          <BarChart data={data} barCategoryGap={10} barGap={4} maxBarSize={38}>
            <defs>
              <linearGradient id="barGoldCodecs" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={colors.gold600} />
                <stop offset="100%" stopColor={colors.gold400} />
              </linearGradient>
            </defs>

            <CartesianGrid vertical={false} stroke={colors.grid} />
            <XAxis dataKey="codec" tick={{ fill: colors.axis }} axisLine={{ stroke: colors.grid }} tickLine={{ stroke: colors.grid }} interval={0} angle={0} height={48}/>
            <YAxis tick={{ fill: colors.axis }} axisLine={{ stroke: colors.grid }} tickLine={{ stroke: colors.grid }} />
            <Tooltip
              wrapperStyle={{ borderRadius: 12, overflow: 'hidden' }}
              contentStyle={{ background: colors.tooltipBg, border: `1px solid ${colors.tooltipBorder}` }}
              labelStyle={{ color: colors.gold500 }}
              itemStyle={{ color: '#fff' }}
            />

            <Bar dataKey="Movie"   fill="url(#barGoldCodecs)" radius={[8,8,0,0]} stroke="rgba(0,0,0,0.1)" strokeWidth={0.5}/>
            <Bar dataKey="Episode" fill={colors.ink}          radius={[8,8,0,0]} stroke="var(--ink-raise)"  strokeWidth={1}/>

            <Legend
              verticalAlign="bottom"
              align="center"
              content={
                <ChartLegend
                  items={[
                    { label: 'Movie', color: colors.gold500, gradientId: 'barGoldCodecs' },
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

