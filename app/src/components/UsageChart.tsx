// app/src/components/UsageChart.tsx
import { useMemo } from "react";
import { ResponsiveBar } from "@nivo/bar";
import { useUsage } from "../hooks/useData";
import { fmtAxisTime, fmtTooltipTime } from "../lib/format";
import { colors } from "../theme/colors";

type ChartRow = { day: string; [user: string]: string | number };

export default function UsageChart({ days = 14 }: { days?: number }) {
  const { data: rows = [], error, isLoading } = useUsage(days);

  // pivot to stacked-per-day
  const data = useMemo<ChartRow[]>(() => {
    const byDay: Record<string, ChartRow> = {};
    const users = new Set<string>();

    for (const r of rows) {
      users.add(r.user);
      if (!byDay[r.day]) byDay[r.day] = { day: r.day };
      byDay[r.day][r.user] = ((byDay[r.day][r.user] as number | undefined) ?? 0) + r.hours;
    }

    // maintain sorted by day
    const arr = Object.values(byDay).sort((a, b) =>
      (a.day as string).localeCompare(b.day as string)
    );

    // ensure all user keys exist (convert Set -> array to avoid downlevel iteration issue)
    const userArr = Array.from(users);
    for (const row of arr) {
      for (const u of userArr) {
        row[u] = (row[u] as number | undefined) ?? 0;
      }
    }

    return arr;
  }, [rows]);

  const users = useMemo(() => {
    const s = new Set<string>();
    rows.forEach((r) => s.add(r.user));
    return Array.from(s).sort();
  }, [rows]);

  const themed = [colors.gold600, "#7a7a7a", "#4d4d4d", "#b99d3a"]; // gold + charcoals

  if (error) {
    return (
      <div className="bg-neutral-800 rounded-2xl p-4 shadow">
        <div className="text-sm text-gray-400 mb-2">Usage (hours per day by user)</div>
        <div className="text-red-400">Failed to load usage data</div>
      </div>
    );
  }

  return (
    <div className="bg-neutral-800 rounded-2xl p-4 shadow">
      <div className="text-sm text-gray-400 mb-2">
        Usage (hours per day by user)
        {isLoading && <span className="ml-2 text-xs opacity-60">Loading...</span>}
      </div>
      <div style={{ width: "100%", height: 300 }}>
        <ResponsiveBar
          data={data}
          keys={users}
          indexBy="day"
          margin={{ top: 50, right: 130, bottom: 50, left: 60 }}
          padding={0.3}
          valueScale={{ type: "linear" }}
          indexScale={{ type: "band", round: true }}
          colors={({ id }) => {
            const userIndex = users.indexOf(id as string);
            return themed[userIndex % themed.length];
          }}
          borderColor={{ from: "color", modifiers: [["darker", 0.1]] }}
          borderWidth={0.5}
          borderRadius={6}
          axisTop={null}
          axisRight={null}
          axisBottom={{
            tickSize: 5,
            tickPadding: 5,
            tickRotation: 0,
            legend: "Day",
            legendPosition: "middle",
            legendOffset: 32,
          }}
          axisLeft={{
            tickSize: 5,
            tickPadding: 5,
            tickRotation: 0,
            legend: "Hours",
            legendPosition: "middle",
            legendOffset: -40,
            format: (v) => fmtAxisTime(Number(v)),
          }}
          enableLabel={false}
          tooltip={({ id, value, color }) => (
            <div
              style={{
                background: "#1f2937",
                border: "1px solid #374151",
                borderRadius: "8px",
                padding: "8px 12px",
                color: "#fff",
              }}
            >
              <div style={{ color }}>
                <strong>{id}</strong>: {fmtTooltipTime(Number(value))}
              </div>
            </div>
          )}
          legends={[
            {
              dataFrom: "keys",
              anchor: "bottom-right",
              direction: "column",
              justify: false,
              translateX: 120,
              translateY: 0,
              itemsSpacing: 2,
              itemWidth: 100,
              itemHeight: 20,
              itemDirection: "left-to-right",
              itemOpacity: 0.85,
              symbolSize: 20,
              effects: [
                {
                  on: "hover",
                  style: {
                    itemOpacity: 1,
                  },
                },
              ],
            },
          ]}
          role="application"
          ariaLabel="Usage hours per day by user"
        />
      </div>
    </div>
  );
}
