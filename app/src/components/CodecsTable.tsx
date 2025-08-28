// app/src/components/CodecsTable.tsx
import { useEffect, useMemo, useState } from "react";
import { fetchCodecs } from "../lib/api";
import type { CodecBuckets } from "../types";
import { fmtInt } from "../lib/format";

export default function CodecsTable() {
  const [data, setData] = useState<CodecBuckets | null>(null);

  useEffect(() => {
    fetchCodecs().then(setData).catch(() => {});
  }, []);

  const rows = useMemo(() => {
    if (!data?.codecs) return [];
    // sort by total (movies+episodes) desc for a nicer table
    return Object.keys(data.codecs)
      .map((codec) => ({
        codec,
        movies: data.codecs[codec]?.Movie || 0,
        episodes: data.codecs[codec]?.Episode || 0,
      }))
      .sort((a, b) => b.movies + b.episodes - (a.movies + a.episodes));
  }, [data]);

  return (
    <div className="card p-4">
      <div className="ty-h3 text-center">Media Codecs</div>
      <table className="table-dark mt-2">
        <thead>
          <tr>
            <th> </th>
            <th className="num">Movies</th>
            <th className="num">Episodes</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.codec}>
              <td>{r.codec}</td>
              <td className="num">{fmtInt(r.movies)}</td>
              <td className="num">{fmtInt(r.episodes)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

