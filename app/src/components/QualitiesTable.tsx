// app/src/components/QualitiesTable.tsx
import { useEffect, useMemo, useState } from "react";
import { fetchQualities } from "../lib/api";
import type { QualityBuckets } from "../types";
import { fmtInt } from "../lib/format";

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
    <div className="card-dark">
      <div className="section-title text-center">Media Qualities</div>
      <table className="table-dark mt-2 w-full">
        <thead>
          <tr>
            <th> </th>
            <th className="num">Movies</th>
            <th className="num">Episodes</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.label}>
              <td>{r.label}</td>
              <td className="num">{fmtInt(r.movies)}</td>
              <td className="num">{fmtInt(r.episodes)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
