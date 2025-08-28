// app/src/components/QualitiesTable.tsx
import { useEffect, useMemo, useState } from "react";
import { fetchQualities } from "../lib/api";
import type { QualityBuckets } from "../types";
import { fmtInt } from "../lib/format";
import Card from "./ui/Card";

export default function QualitiesTable() {
  const [data, setData] = useState<QualityBuckets | null>(null);

  useEffect(() => {
    fetchQualities().then(setData).catch(() => {});
  }, []);

  const order = ["4K", "1080p", "720p", "SD", "Unknown"];
  const rows = useMemo(() => {
    if (!data?.buckets) return [];
    return order.map((label) => ({
      label,
      movies: data.buckets[label]?.Movie || 0,
      episodes: data.buckets[label]?.Episode || 0,
    }));
  }, [data]);

  return (
    <Card title="Media Qualities">
      <table className="w-full text-sm text-left text-gray-300">
        <thead className="text-gray-400 border-b border-neutral-700">
          <tr>
            <th> </th>
            <th className="text-right">Movies</th>
            <th className="text-right">Episodes</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.label} className="border-b border-neutral-800 last:border-0">
              <td className="py-3">{r.label}</td>
              <td className="py-3 text-right">{fmtInt(r.movies)}</td>
              <td className="py-3 text-right">{fmtInt(r.episodes)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </Card>
  );
}
