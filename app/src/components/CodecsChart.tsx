// app/src/components/CodecsChart.tsx
import { useMemo } from "react";
import { ResponsiveContainer, CartesianGrid, BarChart, Bar, XAxis, YAxis, Tooltip, Legend } from "recharts";
import { useCodecs } from "../hooks/useData";
import type { CodecBuckets } from "../types";
import { fmtInt } from "../lib/format";

import { colors } from '../theme/colors';

export default function CodecsChart() {
  const { data, error, isLoading } = useCodecs();

  const rows = useMemo(() => {
    if (!data) return [];
    return Object.entries(data.codecs).map(([codec, v]) => ({
      codec,
      Movie: v.Movie,
      Episode: v.Episode,
    }));
  }, [data]);

  if (error) {
    return (
      <div className="bg-neutral-800 rounded-2xl p-4 shadow">
        <div className="text-sm text-gray-400 mb-2">Codecs</div>
        <div className="text-red-400">Failed to load codecs data</div>
      </div>
    );
  }

  return (
    <div className="bg-neutral-800 rounded-2xl p-4 shadow">
      <div className="text-sm text-gray-400 mb-2">
        Codecs
        {isLoading && <span className="ml-2 text-xs opacity-60">Loading...</span>}
      </div>
      <div style={{ height: 280 }}>
        <ResponsiveContainer width="100%" height={300}>
          <BarChart data={rows} barCategoryGap={10} barGap={4} maxBarSize={38}>
            <defs>
              <linearGradient id="barGoldCodecs" x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={colors.gold600} />
                <stop offset="100%" stopColor={colors.gold400} />
              </linearGradient>
            </defs>

            <CartesianGrid vertical={false} stroke={colors.grid} />
            <XAxis 
              dataKey="codec" 
              tick={{ fill: colors.axis }} 
              axisLine={{ stroke: colors.grid }} 
              tickLine={{ stroke: colors.grid }} 
              interval={0} 
              angle={0} 
              height={48}
            />
            <YAxis 
              tick={{ fill: colors.axis }} 
              axisLine={{ stroke: colors.grid }} 
              tickLine={{ stroke: colors.grid }} 
            />
            <Tooltip
              wrapperStyle={{ borderRadius: 12, overflow: 'hidden' }}
              contentStyle={{ background: colors.tooltipBg, border: `1px solid ${colors.tooltipBorder}` }}
              labelStyle={{ color: colors.gold500 }}
              itemStyle={{ color: '#fff' }}
              formatter={(value: any) => [fmtInt(Number(value)), '']}
            />

            <Bar 
              dataKey="Movie"   
              fill="url(#barGoldCodecs)" 
              radius={[8,8,0,0]} 
              stroke="rgba(255,255,255,0.1)" 
              strokeWidth={0.5}
            />
            <Bar 
              dataKey="Episode" 
              fill={colors.ink}          
              radius={[8,8,0,0]} 
              stroke="rgba(255,255,255,0.08)"  
              strokeWidth={0.5}
            />

            <Legend />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}