// app/src/components/CodecsChart.tsx
import { useEffect, useMemo, useState } from "react";
import { ResponsiveContainer, BarChart, Bar, XAxis, YAxis, Tooltip, Legend } from "recharts";
import { fetchCodecs } from "../lib/api";
import type { CodecBuckets } from "../types";
import { fmtInt } from "../lib/format";

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
        <ResponsiveContainer>
          <BarChart data={rows}>
            <XAxis dataKey="codec" />
            <YAxis tickFormatter={fmtInt} />
            <Tooltip />
            <Legend />
            <Bar dataKey="Movie" />
            <Bar dataKey="Episode" />
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

