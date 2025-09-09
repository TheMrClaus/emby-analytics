// app/src/components/QualitiesTable.tsx
import { useMemo } from "react";
import Link from "next/link";
import { useQualities } from "../hooks/useData";
import { fmtInt } from "../lib/format";
import Card from "./ui/Card";

export default function QualitiesTable() {
  const { data, isLoading } = useQualities();

  const order = ["8K", "4K", "1080p", "720p", "SD", "Resolution Not Available"];
  const rows = useMemo(() => {
    if (!data?.buckets || isLoading) return [];
    return order.map((label) => ({
      label,
      movies: data.buckets[label]?.Movie || 0,
      episodes: data.buckets[label]?.Episode || 0,
    }));
  }, [data, isLoading]);

  return (
    <Card
      title={
        <>
          Media Qualities {isLoading && <span className="ml-2 text-xs opacity-60">Loading...</span>}
        </>
      }
    >
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
      {rows.length === 0 && !isLoading && (
        <div className="text-gray-500 text-center py-4">No quality data found</div>
      )}
    </Card>
  );
}
