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
    <div className="card p-4">
      <div className="h3 mb-2">Most Active (lifetime)</div>
      <table className="table-dark">
        <thead>
          <tr><th>User</th><th className="num">Watch Time</th></tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i}>
              <td>{r.user}</td>
              <td className="num">{fmt(r)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

