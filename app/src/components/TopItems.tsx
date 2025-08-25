// app/src/components/TopItems.tsx
import { useEffect, useState } from "react";
import { fetchTopItems, imgPrimary } from "../lib/api";
import type { TopItem, ItemRow } from "../types";
import { fmtTooltipTime } from "../lib/format";

// Helper function to fetch enriched item data
async function fetchItemsByIds(ids: string[]): Promise<ItemRow[]> {
  const API_BASE = process.env.NEXT_PUBLIC_API_BASE ?? "";
  const res = await fetch(`${API_BASE}/items/by-ids?ids=${encodeURIComponent(ids.join(","))}`, {
    headers: { "Content-Type": "application/json" }
  });
  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText}`);
  }
  return res.json();
}

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
    
    fetchItemsByIds(itemIds)
      .then((items: ItemRow[]) => {
        const mapping: Record<string, ItemRow> = {};
        items.forEach(item => {
          mapping[item.id] = item;
        });
        setItemMapping(mapping);
      })
      .catch(err => {
        console.error("Failed to fetch enriched item data:", err);
        // Fallback: create basic mapping from original data
        const fallbackMapping: Record<string, ItemRow> = {};
        rows.forEach(r => {
          if (r.item_id && r.name) {
            fallbackMapping[r.item_id] = {
              id: r.item_id,
              name: r.name,
              type: r.type,
              display: r.name
            };
          }
        });
        setItemMapping(fallbackMapping);
      });
  }, [rows]);

  return (
    <div className="bg-neutral-800 rounded-2xl p-4 shadow">
      <div className="text-sm text-gray-400 mb-2">Top Items (last {days} days)</div>
      <table className="w-full text-sm text-left text-gray-300">
        <thead className="text-gray-400 border-b border-neutral-700">
          <tr>
            <th>Item</th>
            <th>Type</th>
            <th className="text-right">Hours</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r, i) => {
            const enriched = itemMapping[r.item_id];

            let displayName = "Unknown Item";
            if (enriched?.display && enriched.display !== "") {
              displayName = enriched.display;
            } else if (enriched?.name && enriched.name !== "") {
              displayName = enriched.name;
            } else if (r.name && r.name !== "") {
              displayName = r.name;
            }

            let displayType = "Unknown";
            if (enriched?.type && enriched.type !== "") {
              displayType = enriched.type;
            } else if (r.type && r.type !== "") {
              displayType = r.type;
            }

            return (
              <tr key={i}>
                <td className="flex items-center gap-3">
                  <img src={imgPrimary(r.item_id)} alt="" className="w-8 h-12 object-cover rounded" />
                  <span>{displayName}</span>
                </td>
                <td>{displayType}</td>
                <td className="text-right" title={fmtTooltipTime(r.hours)}>
                  {r.hours.toFixed(2)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}