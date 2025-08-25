// app/src/components/TopUsers.tsx
import { useTopUsers } from "../hooks/useData";
import type { TopUser } from "../types";
import { fmtTooltipTime } from "../lib/format";

export default function TopUsers({ days = 14, limit = 10 }: { days?: number; limit?: number }) {
  const { data: rows = [], error, isLoading } = useTopUsers(days, limit);

  if (error) {
    return (
      <div className="bg-neutral-800 rounded-2xl p-4 shadow">
        <div className="text-sm text-gray-400 mb-2">Top Users (last {days} days)</div>
        <div className="text-red-400">Failed to load users data</div>
      </div>
    );
  }

  return (
    <div className="bg-neutral-800 rounded-2xl p-4 shadow">
      <div className="text-sm text-gray-400 mb-2">
        Top Users (last {days} days)
        {isLoading && <span className="ml-2 text-xs opacity-60">Loading...</span>}
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