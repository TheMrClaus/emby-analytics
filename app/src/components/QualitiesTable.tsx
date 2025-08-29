// app/src/components/QualitiesTable.tsx
import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
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
            <th className="py-3">Quality</th>
            <th className="py-3 text-right">Movies</th>
            <th className="py-3 text-right">Episodes</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.label} className="border-b border-neutral-800 last:border-0">
              <td className="py-3">
                <Link 
                  href={`/qualities/${encodeURIComponent(r.label)}`}
                  className="text-blue-400 hover:text-blue-300 hover:underline transition-colors"
                >
                  {r.label}
                </Link>
              </td>
              <td className="py-3 text-right">
                {r.movies > 0 ? (
                  <Link 
                    href={`/qualities/${encodeURIComponent(r.label)}?media_type=Movie`}
                    className="text-blue-400 hover:text-blue-300 hover:underline transition-colors"
                  >
                    {fmtInt(r.movies)}
                  </Link>
                ) : (
                  <span className="text-gray-500">0</span>
                )}
              </td>
              <td className="py-3 text-right">
                {r.episodes > 0 ? (
                  <Link 
                    href={`/qualities/${encodeURIComponent(r.label)}?media_type=Episode`}
                    className="text-blue-400 hover:text-blue-300 hover:underline transition-colors"
                  >
                    {fmtInt(r.episodes)}
                  </Link>
                ) : (
                  <span className="text-gray-500">0</span>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </Card>
  );
}