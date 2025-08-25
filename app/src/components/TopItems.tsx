// app/src/components/TopItems.tsx
import { useEffect, useState } from "react";
import { fetchTopItems, imgPrimary } from "../lib/api";
import type { TopItem, ItemRow } from "../types";
import { fmtTooltipTime } from "../lib/format";

export default function TopItems({ days = 14, limit = 10 }: { days?: number; limit?: number }) {
  const [rows, setRows] = useState<TopItem[]>([]);
  const [itemMapping, setItemMapping] = useState<Record<string, ItemRow>>({});

  useEffect(() => {
    fetchTopItems(days, limit).then(setRows).catch(() => {});
  }, [days, limit]);

  // Fetch enriched item data for display names (especially episodes)
  useEffect(() => {
    const itemIds = Array.from(new Set(rows.map(r => r.item_id).filter(Boolean)));
    if (!itemIds.length) {
      setItemMapping({});
      return;
    }
    
    fetch(`/items/by-ids?ids=${encodeURIComponent(itemIds.join(","))}`)
      .then(res => res.json())
      .then((items: ItemRow[]) => {
        const mapping: Record<string, ItemRow> = {};
        items.forEach(item => {
          mapping[item.id] = item;
        });
        setItemMapping(mapping);
      })
      .catch(() => {});
  }, [rows]);

  return (
    <div className="card p-4">
      <div className="h3 mb-2">Top Items (last {days} days)</div>
      <table className="table-dark">
        <thead>
          <tr><th>Item</th><th>Type</th><th className="num">Hours</th></tr>
        </thead>
        <tbody>
          {rows.map((r, i) => {
            const enriched = itemMapping[r.item_id];
            const displayName = enriched?.display || enriched?.name || r.name || "Unknown";
            const displayType = enriched?.type || r.type || "Unknown";
            
            return (
              <tr key={i}>
                <td className="flex items-center gap-3">
                  <img src={imgPrimary(r.item_id)} alt="" className="w-8 h-12 object-cover rounded" />
                  <span>{displayName}</span>
                </td>
                <td>{displayType}</td>
                <td className="num" title={fmtTooltipTime(r.hours)}>{r.hours.toFixed(2)}</td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}