// app/src/components/QualitiesChart.tsx
import { useEffect, useMemo, useState } from "react";
import { ResponsiveContainer, BarChart, Bar, XAxis, YAxis, Tooltip, Legend } from "recharts";
import { fetchQualities } from "../lib/api";
import type { QualityBuckets } from "../types";
import { fmtInt } from "../lib/format";

export default function QualitiesChart() {
  const [data, setData] = useState<QualityBuckets | null>(null);
  useEffect(() => {
    fetchQualities().then(setData).catch(() => {});
  }, []);

  const rows = useMemo(() => {
    if (!data) return [];
    return Object.entries(data.buckets).map(([label, v]) => ({
      label,
      Movie: v.Movie,
      Episode: v.Episode,
    }));
  }, [data]);

  return (
    <div className="card p-4">
      <div className="h3 mb-2">Media Quality</div>
      <div style={{ height: 280 }}>
        <ResponsiveContainer>
          <BarChart data={rows}>
            <XAxis dataKey="label" />
            <YAxis tickFormatter={fmtInt} />
            <Tooltip />
            <Legend />
            <Bar dataKey="Movie" fill="#FFD700" />
            <Bar dataKey="Episode" fill="#000000" />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

