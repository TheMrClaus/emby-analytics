// app/src/components/TopUsers.tsx
import { useEffect, useState } from "react";
import { fetchTopUsers } from "../lib/api";
import type { TopUser } from "../types";
import { fmtTooltipTime } from "../lib/format";

export default function TopUsers({ days = 14, limit = 10 }: { days?: number; limit?: number }) {
  const [rows, setRows] = useState<TopUser[]>([]);
  useEffect(() => {
    fetchTopUsers(days, limit).then(setRows).catch(() => {});
  }, [days, limit]);

  return (
    <div className="card p-4">
      <div className="h3 mb-2">Top Users (last {days} days)</div>
      <table className="table-dark">
        <thead>
          <tr><th>User</th><th className="num">Hours</th></tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i}>
              <td>{r.name}</td>
              <td className="num" title={fmtTooltipTime(r.hours)}>{r.hours.toFixed(2)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

