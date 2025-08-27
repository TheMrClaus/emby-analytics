import { useState } from "react";
import { useTopUsers } from "../hooks/useData";
import type { TopUser } from "../types";
import { fmtTooltipTime } from "../lib/format";

const timeframeOptions = [
  { value: "all-time", label: "All Time" },
  { value: "30d", label: "30 Days" },
  { value: "14d", label: "14 Days" },
  { value: "7d", label: "7 Days" },
  { value: "3d", label: "3 Days" },
  { value: "1d", label: "1 Day" },
];

export default function TopUsers({ limit = 10 }: { limit?: number }) {
  const [timeframe, setTimeframe] = useState("14d");
  
  // Convert timeframe to days for the API (backwards compatibility)
  const days = timeframe === "all-time" ? 0 : 
    parseInt(timeframe.replace('d', '')) || 14;
  
  const { data: rows = [], error, isLoading } = useTopUsers(days, limit, timeframe);

  if (error) {
    return (
      <div className="bg-neutral-800 rounded-2xl p-4 shadow">
        <div className="flex justify-between items-center mb-2">
          <div className="text-sm text-gray-400">Top Users</div>
        </div>
        <div className="text-red-400">Failed to load users data</div>
      </div>
    );
  }

  const selectedOption = timeframeOptions.find(opt => opt.value === timeframe);

  return (
    <div className="bg-neutral-800 rounded-2xl p-4 shadow">
      <div className="flex justify-between items-center mb-2">
        <div className="text-sm text-gray-400">
          Top Users ({selectedOption?.label})
          {isLoading && <span className="ml-2 text-xs opacity-60">Loading...</span>}
        </div>
        <select 
          value={timeframe}
          onChange={(e) => setTimeframe(e.target.value)}
          className="bg-neutral-700 text-white text-xs px-2 py-1 rounded border border-neutral-600 focus:border-blue-500 focus:outline-none"
        >
          {timeframeOptions.map(option => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </div>
      
      <table className="w-full text-sm text-left text-gray-300">
        <thead className="text-gray-400 border-b border-neutral-700">
          <tr>
            <th className="py-1">User</th>
            <th className="py-1 text-right">Hours</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i} className="border-b border-neutral-700 last:border-0">
              <td className="py-1">{r.name}</td>
              <td
                className="py-1 text-right tabular-nums"
                title={fmtTooltipTime(r.hours)}
              >
                {r.hours.toFixed(2)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {rows.length === 0 && !isLoading && (
        <div className="text-gray-500 text-center py-4">No data available</div>
      )}
    </div>
  );
}