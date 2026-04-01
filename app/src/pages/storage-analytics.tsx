import React, { useState, useMemo } from "react";
import Head from "next/head";
import Link from "next/link";
import Header from "../components/Header";
import { ArrowLeft, HardDrive, TrendingUp, Copy, Calendar } from "lucide-react";
import useSWR from "swr";
import { ResponsiveLine } from "@nivo/line";

interface StaleContentItem {
  id: string;
  title: string;
  item_type: string;
  server_id: string;
  size_gb: number;
  added_at?: string;
  file_path?: string;
}

interface ROIItem {
  id: string;
  title: string;
  item_type: string;
  server_id: string;
  play_count: number;
  watch_hours: number;
  size_gb: number;
  hours_per_gb: number;
}

interface DuplicateGroup {
  normalized_path: string;
  duplicate_count: number;
  total_size_gb: number;
  item_ids: string[];
  titles: string[];
}

interface SnapshotData {
  date: string;
  size_gb: number;
}

interface PredictedData {
  date: string;
  size_gb: number;
}

interface StoragePrediction {
  historical_data: SnapshotData[];
  predictions: PredictedData[];
  current_size_gb: number;
  projected_size_gb_6mo: number;
  growth_rate_gb_per_day: number;
  message?: string;
}

async function fetcher(url: string) {
  const res = await fetch(url);
  if (!res.ok) throw new Error("Failed to fetch");
  return res.json();
}

/** Format a value in GB to the most appropriate unit (GB, TB, PB, EB). */
function formatSizeGB(gb: number): string {
  const units = ["GB", "TB", "PB", "EB"];
  let value = gb;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex++;
  }
  return `${value.toFixed(2)} ${units[unitIndex]}`;
}

export default function StorageAnalyticsPage() {
  const [staleSort, setStaleSort] = useState<"size" | "title">("size");
  const [roiSort, setRoiSort] = useState<"worst" | "best">("worst");

  const { data: staleData } = useSWR<{ stale_items: StaleContentItem[]; total_count: number }>(
    "/stats/storage/stale-content?limit=100",
    fetcher,
    { refreshInterval: 60000 }
  );

  const { data: roiData } = useSWR<{ roi_items: ROIItem[]; total_count: number; sort: string }>(
    `/stats/storage/roi?limit=100&sort=${roiSort}`,
    fetcher,
    { refreshInterval: 60000 }
  );

  const { data: duplicatesData } = useSWR<{
    duplicate_groups: DuplicateGroup[];
    total_groups: number;
  }>("/stats/storage/duplicates?limit=50", fetcher, { refreshInterval: 60000 });

  const { data: predictionsData } = useSWR<StoragePrediction>(
    "/stats/storage/predictions",
    fetcher,
    { refreshInterval: 300000 }
  );

  const staleItems = useMemo(() => {
    if (!staleData?.stale_items) return [];
    const items = [...staleData.stale_items];
    if (staleSort === "title") {
      items.sort((a, b) => a.title.localeCompare(b.title));
    }
    return items;
  }, [staleData, staleSort]);

  const totalStaleSize = useMemo(() => {
    return staleItems.reduce((acc, item) => acc + item.size_gb, 0);
  }, [staleItems]);

  const chartData = useMemo(() => {
    if (!predictionsData) return [];
    
    const historical = predictionsData.historical_data.map((d) => ({
      x: d.date,
      y: d.size_gb,
    }));

    const predictions = predictionsData.predictions.map((d) => ({
      x: d.date,
      y: d.size_gb,
    }));

    return [
      {
        id: "Historical",
        data: historical,
      },
      {
        id: "Predicted",
        data: predictions,
      },
    ];
  }, [predictionsData]);

  return (
    <>
      <Head>
        <title>Storage Analytics - Emby Analytics</title>
        <meta name="viewport" content="initial-scale=1, width=device-width" />
      </Head>
      <div className="min-h-screen bg-neutral-900 text-white">
        <Header />
        <main className="p-4 md:p-6 space-y-6 border-t border-neutral-800">
          {/* Breadcrumb */}
          <div className="flex items-center gap-2 text-sm">
            <Link
              href="/"
              className="text-blue-300 hover:text-white flex items-center gap-1 underline decoration-dotted"
            >
              <ArrowLeft className="w-4 h-4" />
              Dashboard
            </Link>
            <span className="text-gray-500">/</span>
            <span className="text-gray-300">Storage Analytics</span>
          </div>

          {/* Page Title */}
          <div className="flex items-center gap-3">
            <HardDrive className="w-8 h-8 text-amber-400" />
            <h1 className="text-3xl font-bold text-white">Storage Analytics</h1>
          </div>

          {/* Summary Cards */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div className="bg-neutral-800 border border-neutral-700 rounded-lg p-4">
              <div className="text-sm text-gray-400 mb-1">Current Storage</div>
              <div className="text-2xl font-bold text-white">
                {predictionsData && predictionsData.current_size_gb > 0
                  ? formatSizeGB(predictionsData.current_size_gb)
                  : "—"}
              </div>
            </div>
            <div className="bg-neutral-800 border border-neutral-700 rounded-lg p-4">
              <div className="text-sm text-gray-400 mb-1">Stale Content Size</div>
              <div className="text-2xl font-bold text-amber-400">
                {totalStaleSize > 0 ? formatSizeGB(totalStaleSize) : "—"}
              </div>
              <div className="text-xs text-gray-500 mt-1">
                {staleItems.length} items never played
              </div>
            </div>
            <div className="bg-neutral-800 border border-neutral-700 rounded-lg p-4">
              <div className="text-sm text-gray-400 mb-1">Projected Size (6mo)</div>
              <div className="text-2xl font-bold text-blue-400">
                {predictionsData && predictionsData.projected_size_gb_6mo > 0
                  ? formatSizeGB(predictionsData.projected_size_gb_6mo)
                  : "—"}
              </div>
              <div className="text-xs text-gray-500 mt-1">
                {predictionsData && predictionsData.growth_rate_gb_per_day > 0
                  ? `+${formatSizeGB(predictionsData.growth_rate_gb_per_day)}/day`
                  : ""}
              </div>
            </div>
          </div>

          {/* Accuracy Warning */}
          {predictionsData?.message && (
            <div className="bg-amber-900/20 border border-amber-700/50 rounded-lg px-4 py-3 flex items-start gap-3">
              <TrendingUp className="w-5 h-5 text-amber-400 mt-0.5 shrink-0" />
              <p className="text-sm text-amber-200">{predictionsData.message}</p>
            </div>
          )}

          {/* Storage Predictions */}
          <div className="bg-neutral-800 border border-neutral-700 rounded-lg p-6">
            <div className="flex items-center gap-2 mb-4">
              <TrendingUp className="w-5 h-5 text-blue-400" />
              <h2 className="text-xl font-semibold text-white">Storage Growth Forecast</h2>
            </div>
            {chartData.length > 0 ? (
              <div style={{ height: 300 }}>
                <ResponsiveLine
                  data={chartData}
                  margin={{ top: 20, right: 20, bottom: 50, left: 60 }}
                  xScale={{ type: "point" }}
                  yScale={{ type: "linear", min: "auto", max: "auto" }}
                  axisBottom={{
                    tickSize: 5,
                    tickPadding: 5,
                    tickRotation: -45,
                    legend: "Date",
                    legendOffset: 45,
                    legendPosition: "middle",
                  }}
                  axisLeft={{
                    tickSize: 5,
                    tickPadding: 5,
                    tickRotation: 0,
                    legend: "Size (GB)",
                    legendOffset: -50,
                    legendPosition: "middle",
                  }}
                  colors={["#60a5fa", "#fbbf24"]}
                  pointSize={6}
                  pointBorderWidth={2}
                  pointBorderColor={{ from: "serieColor" }}
                  enableArea={false}
                  useMesh={true}
                  legends={[
                    {
                      anchor: "top-right",
                      direction: "column",
                      translateX: 0,
                      translateY: 0,
                      itemWidth: 100,
                      itemHeight: 20,
                      itemTextColor: "#999",
                      symbolSize: 12,
                      symbolShape: "circle",
                    },
                  ]}
                  theme={{
                    axis: {
                      ticks: { text: { fill: "#9ca3af" } },
                      legend: { text: { fill: "#d1d5db" } },
                    },
                    grid: { line: { stroke: "#374151", strokeWidth: 1 } },
                    tooltip: {
                      container: {
                        background: "#1f2937",
                        color: "#f3f4f6",
                        fontSize: 12,
                        borderRadius: "4px",
                        padding: "8px 12px",
                      },
                    },
                  }}
                />
              </div>
            ) : (
              <div className="text-gray-400 text-sm">Loading forecast data...</div>
            )}
          </div>

          {/* Stale Content */}
          <div className="bg-neutral-800 border border-neutral-700 rounded-lg p-6">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <Calendar className="w-5 h-5 text-amber-400" />
                <h2 className="text-xl font-semibold text-white">Stale Content (Never Played)</h2>
              </div>
              <select
                value={staleSort}
                onChange={(e) => setStaleSort(e.target.value as "size" | "title")}
                className="px-3 py-1 bg-neutral-700 border border-neutral-600 rounded text-sm text-white"
              >
                <option value="size">Sort by Size</option>
                <option value="title">Sort by Title</option>
              </select>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="bg-neutral-900 border-b border-neutral-700">
                  <tr>
                    <th className="text-left p-3 font-semibold text-gray-300">Title</th>
                    <th className="text-left p-3 font-semibold text-gray-300">Type</th>
                    <th className="text-left p-3 font-semibold text-gray-300">Server</th>
                    <th className="text-right p-3 font-semibold text-gray-300">Size</th>
                  </tr>
                </thead>
                <tbody>
                  {staleItems.length === 0 && (
                    <tr>
                      <td colSpan={4} className="text-center p-6 text-gray-400">
                        No stale content found
                      </td>
                    </tr>
                  )}
                  {staleItems.map((item) => (
                    <tr key={item.id} className="border-b border-neutral-700 hover:bg-neutral-750">
                      <td className="p-3 text-white">{item.title}</td>
                      <td className="p-3 text-gray-400">{item.item_type}</td>
                      <td className="p-3 text-gray-400">{item.server_id}</td>
                      <td className="p-3 text-right text-amber-400 font-mono">
                        {formatSizeGB(item.size_gb)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* ROI Analysis */}
          <div className="bg-neutral-800 border border-neutral-700 rounded-lg p-6">
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <TrendingUp className="w-5 h-5 text-blue-400" />
                <h2 className="text-xl font-semibold text-white">ROI Analysis (Hours/GB)</h2>
              </div>
              <select
                value={roiSort}
                onChange={(e) => setRoiSort(e.target.value as "worst" | "best")}
                className="px-3 py-1 bg-neutral-700 border border-neutral-600 rounded text-sm text-white"
              >
                <option value="worst">Worst ROI (Deletion Candidates)</option>
                <option value="best">Best ROI</option>
              </select>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead className="bg-neutral-900 border-b border-neutral-700">
                  <tr>
                    <th className="text-left p-3 font-semibold text-gray-300">Title</th>
                    <th className="text-left p-3 font-semibold text-gray-300">Type</th>
                    <th className="text-right p-3 font-semibold text-gray-300">Plays</th>
                    <th className="text-right p-3 font-semibold text-gray-300">Watch Hours</th>
                    <th className="text-right p-3 font-semibold text-gray-300">Size</th>
                    <th className="text-right p-3 font-semibold text-gray-300">Hours/GB</th>
                  </tr>
                </thead>
                <tbody>
                  {!roiData?.roi_items && (
                    <tr>
                      <td colSpan={6} className="text-center p-6 text-gray-400">
                        Loading...
                      </td>
                    </tr>
                  )}
                  {roiData?.roi_items.length === 0 && (
                    <tr>
                      <td colSpan={6} className="text-center p-6 text-gray-400">
                        No ROI data available
                      </td>
                    </tr>
                  )}
                  {roiData?.roi_items.map((item) => {
                    const roiColor =
                      item.hours_per_gb >= 1
                        ? "text-green-400"
                        : item.hours_per_gb >= 0.5
                        ? "text-yellow-400"
                        : "text-red-400";
                    return (
                      <tr key={item.id} className="border-b border-neutral-700 hover:bg-neutral-750">
                        <td className="p-3 text-white">{item.title}</td>
                        <td className="p-3 text-gray-400">{item.item_type}</td>
                        <td className="p-3 text-right text-gray-300">{item.play_count}</td>
                        <td className="p-3 text-right text-gray-300">
                          {item.watch_hours.toFixed(1)}
                        </td>
                        <td className="p-3 text-right text-gray-400 font-mono">
                          {formatSizeGB(item.size_gb)}
                        </td>
                        <td className={`p-3 text-right font-mono font-bold ${roiColor}`}>
                          {item.hours_per_gb.toFixed(2)}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>

          {/* Duplicates */}
          <div className="bg-neutral-800 border border-neutral-700 rounded-lg p-6">
            <div className="flex items-center gap-2 mb-4">
              <Copy className="w-5 h-5 text-purple-400" />
              <h2 className="text-xl font-semibold text-white">Duplicate Files</h2>
            </div>
            <div className="space-y-4">
              {!duplicatesData?.duplicate_groups && (
                <div className="text-gray-400 text-sm">Loading...</div>
              )}
              {duplicatesData?.duplicate_groups.length === 0 && (
                <div className="text-gray-400 text-sm">No duplicates found</div>
              )}
              {duplicatesData?.duplicate_groups.map((group, idx) => (
                <div key={idx} className="bg-neutral-900 border border-neutral-700 rounded p-4">
                  <div className="flex items-center justify-between mb-2">
                    <div className="text-sm text-gray-400 font-mono truncate flex-1 mr-4">
                      {group.normalized_path}
                    </div>
                    <div className="text-purple-400 font-bold">
                      {formatSizeGB(group.total_size_gb)}
                    </div>
                  </div>
                  <div className="text-xs text-gray-500">
                    {group.duplicate_count} copies found
                  </div>
                  <div className="mt-2 space-y-1">
                    {group.titles.map((title, i) => (
                      <div key={i} className="text-sm text-gray-300">
                        • {title}
                      </div>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </main>
      </div>
    </>
  );
}
