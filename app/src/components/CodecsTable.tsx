// app/src/components/CodecsTable.tsx
import { useEffect, useMemo, useState } from "react";
import { fetchCodecs } from "../lib/api";
import type { CodecBuckets } from "../types";
import { fmtInt } from "../lib/format";
import Card from "./ui/Card";

export default function CodecsTable() {
  const [data, setData] = useState<CodecBuckets | null>(null);

  useEffect(() => {
    fetchCodecs().then(setData).catch(() => {});
  }, []);

  const rows = useMemo(() => {
    if (!data?.codecs) return [];
    return Object.keys(data.codecs)
      .map((codec) => ({
        codec,
        movies: data.codecs[codec]?.Movie || 0,
        episodes: data.codecs[codec]?.Episode || 0,
      }))
      .sort((a, b) => b.movies + b.episodes - (a.movies + a.episodes));
  }, [data]);

  return (
    <Card title="Media Codecs">
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
            <tr key={r.codec} className="border-b border-neutral-800 last:border-0">
              <td className="py-3">{r.codec}</td>
              <td className="py-3 text-right">{fmtInt(r.movies)}</td>
              <td className="py-3 text-right">{fmtInt(r.episodes)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </Card>
  );
}
