// app/src/components/TopItems.tsx
import { useEffect, useState } from "react";
import { fetchTopItems, imgPrimary } from "../lib/api";
import type { TopItem } from "../types";
import { fmtTooltipTime } from "../lib/format";

export default function TopItems({ days = 14, limit = 10 }: { days?: number; limit?: number }) {
  const [rows, setRows] = useState<TopItem[]>([]);
  useEffect(() => {
    fetchTopItems(days, limit).then(setRows).catch(() => {});
  }, [days, limit]);

  return (
    <div className="card p-4">
      <div className="h3 mb-2">Top Items (last {days} days)</div>
      <table className="table-dark">
        <thead>
          <tr><th>Item</th><th>Type</th><th className="num">Hours</th></tr>
        </thead>
        <tbody>
          {rows.map((r, i) => (
            <tr key={i}>
              <td className="flex items-center gap-3">
                <img src={imgPrimary(r.item_id)} alt="" className="w-8 h-12 object-cover rounded" />
                <span>{r.name}</span>
              </td>
              <td>{r.type}</td>
              <td className="num" title={fmtTooltipTime(r.hours)}>{r.hours.toFixed(2)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

