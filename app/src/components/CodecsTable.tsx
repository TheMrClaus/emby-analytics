// app/src/components/CodecsTable.tsx
import { useMemo } from "react";
import Link from "next/link";
import { useCodecs } from "../hooks/useData";
import { fmtInt } from "../lib/format";
import Card from "./ui/Card";

export default function CodecsTable() {
  const { data, isLoading } = useCodecs();

  const rows = useMemo(() => {
    if (!data?.codecs || isLoading) return [];
    return Object.keys(data.codecs)
      .map((codec) => ({
        codec,
        movies: data.codecs[codec]?.Movie || 0,
        episodes: data.codecs[codec]?.Episode || 0,
      }))
      .sort((a, b) => b.movies + b.episodes - (a.movies + a.episodes));
  }, [data, isLoading]);

  return (
    <Card
      title={
        <>Media Codecs {isLoading && <span className="ml-2 text-xs opacity-60">Loading...</span>}</>
      }
    >
      <table className="w-full text-sm text-left text-gray-300">
        <thead className="text-gray-400 border-b border-neutral-700">
          <tr>
            <th className="py-3">Codec</th>
            <th className="py-3 text-right">Movies</th>
            <th className="py-3 text-right">Episodes</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.codec} className="border-b border-neutral-800 last:border-0">
              <td className="py-3">
                <Link
                  href={`/codecs/${encodeURIComponent(r.codec)}`}
                  className="text-blue-400 hover:text-blue-300 hover:underline transition-colors"
                >
                  {r.codec}
                </Link>
              </td>
              <td className="py-3 text-right">
                {r.movies > 0 ? (
                  <Link
                    href={`/codecs/${encodeURIComponent(r.codec)}?media_type=Movie`}
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
                    href={`/codecs/${encodeURIComponent(r.codec)}?media_type=Episode`}
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
      {rows.length === 0 && !isLoading && (
        <div className="text-gray-500 text-center py-4">No codec data found</div>
      )}
    </Card>
  );
}
