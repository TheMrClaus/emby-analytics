// app/src/components/TopItems.tsx
import { useTopItems } from "../hooks/useData";
import { imgPrimary } from "../lib/api";
import type { TopItem } from "../types";
import { fmtTooltipTime } from "../lib/format";

export default function TopItems({ days = 14, limit = 10 }: { days?: number; limit?: number }) {
  const { data: rows = [], error, isLoading } = useTopItems(days, limit);

  if (error) {
    return (
      <div className="bg-neutral-800 rounded-2xl p-4 shadow">
        <div className="text-sm text-gray-400 mb-2">Top Items (last {days} days)</div>
        <div className="text-red-400">Failed to load items data</div>
      </div>
    );
  }

  return (
    <div className="bg-neutral-800 rounded-2xl p-4 shadow">
      <div className="text-sm text-gray-400 mb-2">
        Top Items (last {days} days)
        {isLoading && <span className="ml-2 text-xs opacity-60">Loading...</span>}
      </div>
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
            // Use the enriched display name from backend
            const displayName = r.display || r.name || "Unknown Item";
            const displayType = r.type || "Unknown";

            return (
              <tr key={i}>
                <td className="flex items-center gap-3">
                  <img 
                    src={imgPrimary(r.item_id)} 
                    alt="" 
                    className="w-8 h-12 object-cover rounded"
                    onError={(e) => {
                      // Fallback for broken images
                      (e.target as HTMLImageElement).style.display = 'none';
                    }}
                  />
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
      {rows.length === 0 && !isLoading && (
        <div className="text-gray-500 text-center py-4">No items found</div>
      )}
    </div>
  );
}