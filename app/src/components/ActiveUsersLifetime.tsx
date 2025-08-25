// app/src/components/ActiveUsersLifetime.tsx
import { useEffect, useState } from "react";
import { fetchActiveUsersLifetime } from "../lib/api";
import type { ActiveUserLifetime } from "../types";

export default function ActiveUsersLifetime({ limit = 10 }: { limit?: number }) {
  const [rows, setRows] = useState<ActiveUserLifetime[]>([]);
  useEffect(() => {
    fetchActiveUsersLifetime(limit).then(setRows).catch(() => {});
  }, [limit]);

  const fmt = (r: ActiveUserLifetime) => {
    const parts = [];
    if (r.days) parts.push(`${r.days}d`);
    if (r.hours) parts.push(`${r.hours}h`);
    if (r.minutes) parts.push(`${r.minutes}m`);
    return parts.join(" ") || "0m";
  };

  return (
    <div className="bg-neutral-800 rounded-2xl p-4 shadow">
      <div className="text-sm text-gray-400 mb-2">Most Active (lifetime)</div>
      <table className="w-full text-sm text-left text-gray-300">
        <thead className="text-gray-400 border-b border-neutral-700">
          <tr>
            <th>User</th>
            <th className="text-right">Watch Time</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i} className="border-b border-neutral-700 last:border-0">
              <td>{r.user}</td>
              <td className="text-right tabular-nums">{fmt(r)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

