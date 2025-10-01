// app/src/components/QualitiesChart.tsx
import { useMemo } from "react";
import { ResponsiveBar } from "@nivo/bar";
import { useQualities } from "../hooks/useData";
import { fmtInt } from "../lib/format";

import { colors } from "../theme/colors";
import { useLibraryServer } from "../contexts/LibraryServerContext";

type QualityRow = { label: string; Movie: number; Episode: number };

export default function QualitiesChart() {
  const { server } = useLibraryServer();
  const { data, error, isLoading } = useQualities(server);
  const serverLabel = server === "all" ? "" : server.charAt(0).toUpperCase() + server.slice(1);

  const rows = useMemo<QualityRow[]>(() => {
    if (!data) return [];
    return Object.entries(data.buckets).map(([label, v]) => ({
      label,
      Movie: v.Movie,
      Episode: v.Episode,
    }));
  }, [data]);

  if (error) {
    return (
      <div className="bg-neutral-800 rounded-2xl p-4 shadow">
        <div className="text-sm text-gray-400 mb-2">Media Quality</div>
        <div className="text-red-400">Failed to load quality data</div>
      </div>
    );
  }

  return (
    <div className="bg-neutral-800 rounded-2xl p-4 shadow">
      <div className="text-sm text-gray-400 mb-2">
        Media Quality
        {serverLabel && (
          <span className="ml-2 text-xs text-gray-400 uppercase tracking-wide">{serverLabel}</span>
        )}
        {isLoading && <span className="ml-2 text-xs opacity-60">Loading...</span>}
      </div>
      <div style={{ height: 300 }}>
        <ResponsiveBar
          data={rows}
          keys={["Movie", "Episode"]}
          indexBy="label"
          margin={{ top: 50, right: 130, bottom: 50, left: 60 }}
          padding={0.3}
          innerPadding={4}
          groupMode="grouped"
          valueScale={{ type: "linear" }}
          indexScale={{ type: "band", round: true }}
          colors={({ id }) => {
            if (id === "Movie") {
              return colors.gold600;
            }
            return colors.ink;
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
            legend: "Quality",
            legendPosition: "middle",
            legendOffset: 32,
          }}
          axisLeft={{
            tickSize: 5,
            tickPadding: 5,
            tickRotation: 0,
            legend: "Count",
            legendPosition: "middle",
            legendOffset: -40,
            format: (v) => fmtInt(Number(v)),
          }}
          enableLabel={false}
          tooltip={({ id, value, color, indexValue }) => (
            <div
              style={{
                background: colors.tooltipBg,
                border: `1px solid ${colors.tooltipBorder}`,
                borderRadius: 12,
                padding: "8px 12px",
                color: "#fff",
              }}
            >
              <div style={{ color: colors.gold500 }}>{indexValue}</div>
              <div style={{ color }}>
                <strong>{id}</strong>: {fmtInt(Number(value))}
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
          theme={{
            axis: {
              ticks: {
                text: {
                  fill: colors.axis,
                },
              },
            },
            grid: {
              line: {
                stroke: colors.grid,
              },
            },
          }}
          role="application"
          ariaLabel="Media quality breakdown by Movie and Episode"
        />
      </div>
    </div>
  );
}
