// app/src/components/UsageChart.tsx
import { useEffect, useMemo, useState } from "react";
import { ResponsiveContainer, BarChart, Bar, XAxis, YAxis, Tooltip, Legend } from "recharts";
import { fetchUsage } from "../lib/api";
import type { UsageRow } from "../types";
import { fmtAxisTime, fmtTooltipTime } from "../lib/format";

type ChartRow = { day: string; [user: string]: string | number };

export default function UsageChart({ days = 14 }: { days?: number }) {
  const [rows, setRows] = useState<UsageRow[]>([]);
  useEffect(() => {
    fetchUsage(days).then(setRows).catch(() => {});
  }, [days]);

  // pivot to stacked-per-day
  const data = useMemo<ChartRow[]>(() => {
    const byDay: Record<string, ChartRow> = {};
    const users = new Set<string>();
    for (const r of rows) {
      users.add(r.user);
      if (!byDay[r.day]) byDay[r.day] = { day: r.day };
      byDay[r.day][r.user] = (byDay[r.day][r.user] as number | undefined ?? 0) + r.hours;
    }
    // maintain sorted by day
    const arr = Object.values(byDay).sort((a, b) => (a.day as string).localeCompare(b.day as string));
    // ensure all user keys exist
    for (const row of arr) for (const u of users) row[u] = (row[u] as number | undefined) ?? 0;
    return arr;
  }, [rows]);

  const users = useMemo(() => {
    const s = new Set<string>();
    rows.forEach(r => s.add(r.user));
    return Array.from(s).sort();
  }, [rows]);

  return (
    <div className="card p-4">
      <div className="h3 mb-2">Usage (hours per day by user)</div>
      <div style={{ width: "100%", height: 300 }}>
        <ResponsiveContainer>
          <BarChart data={data}>
            <XAxis dataKey="day" />
            <YAxis tickFormatter={(v) => fmtAxisTime(Number(v))} />
            <Tooltip formatter={(v: any) => fmtTooltipTime(Number(v))} />
            <Legend />
            {users.map((u) => (
              <Bar key={u} stackId="h" dataKey={u} />
            ))}
          </BarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}

