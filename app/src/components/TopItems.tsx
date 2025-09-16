import { useEffect, useState } from "react";
import Image from "next/image";
import { useTopItems } from "../hooks/useData";
import { imgPrimary, fetchConfig } from "../lib/api";
import { fmtTooltipTime, fmtHours } from "../lib/format";
import Card from "./ui/Card";
import { openInEmby } from "../lib/emby";

const timeframeOptions = [
  { value: "all-time", label: "All Time" },
  { value: "30d", label: "30 Days" },
  { value: "14d", label: "14 Days" },
  { value: "7d", label: "7 Days" },
  { value: "3d", label: "3 Days" },
  { value: "1d", label: "1 Day" },
];

export default function TopItems({ limit = 10 }: { limit?: number }) {
  const [timeframe, setTimeframe] = useState("14d");
  const [embyExternalUrl, setEmbyExternalUrl] = useState<string>("");
  const [embyServerId, setEmbyServerId] = useState<string>("");
  const [plexExternalUrl, setPlexExternalUrl] = useState<string>("");
  const [jfExternalUrl,   setJfExternalUrl] = useState<string>("");
  const [plexServerId, setPlexServerId] = useState<string>("");

  // Convert timeframe to days for the API (backwards compatibility)
  const days = timeframe === "all-time" ? 0 : parseInt(timeframe.replace("d", "")) || 14;

  const { data: rows = [], error, isLoading } = useTopItems(days, limit, timeframe);

  // Fetch Emby config once for deep-linking to items
  useEffect(() => {
    fetchConfig()
      .then((cfg) => {
        setEmbyExternalUrl(cfg.emby_external_url);
        setEmbyServerId(cfg.emby_server_id);
        setPlexExternalUrl(cfg.plex_external_url || "");
        setJfExternalUrl(cfg.jellyfin_external_url || "");
        setPlexServerId(cfg.plex_server_id || "");
      })
      .catch(() => {
        /* best-effort; keep links disabled if it fails */
      });
  }, []);

  if (error) {
    return (
      <Card title="Top Items">
        <div className="text-red-400">Failed to load items data</div>
      </Card>
    );
  }

  const selectedOption = timeframeOptions.find((opt) => opt.value === timeframe);

  return (
    <Card
      title={
        <>
          Top Items ({selectedOption?.label})
          {isLoading && <span className="ml-2 text-xs opacity-60">Loading...</span>}
        </>
      }
      right={
        <select
          value={timeframe}
          onChange={(e) => setTimeframe(e.target.value)}
          className="bg-neutral-700 text-white text-xs px-2 py-1 rounded border border-neutral-600 focus:border-blue-500 focus:outline-none"
        >
          {timeframeOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      }
    >
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
              <tr key={i} className="border-b border-neutral-800 last:border-0">
                <td className="py-3">
                  <div className="flex items-center gap-3">
                    <Image
                      src={r.server_type ? `${process.env.NEXT_PUBLIC_API_BASE ?? ""}/img/primary/${r.server_type}/${r.item_id}` : imgPrimary(r.item_id)}
                      alt={displayName}
                      width={32}
                      height={48}
                      className="object-cover rounded"
                    />
                    <span
                      className="cursor-pointer hover:text-blue-400 transition-colors"
                      title={embyExternalUrl ? "Click to open in Emby" : undefined}
                      onClick={() => {
                        const st = (r.server_type || "").toLowerCase();
                        if (st === "emby") {
                          if (!embyExternalUrl) return;
                          openInEmby(r.item_id, embyExternalUrl, embyServerId);
                          return;
                        }
                        if (st === "plex") {
                          if (!plexExternalUrl) return;
                          const base = plexExternalUrl.replace(/\/$/, "");
                          const sid = plexServerId || "";
                          const url = sid
                            ? `${base}/web/index.html#!/server/${encodeURIComponent(sid)}/details?key=${encodeURIComponent("/library/metadata/" + r.item_id)}`
                            : `${base}/web/index.html#!/details?key=${encodeURIComponent("/library/metadata/" + r.item_id)}`;
                          window.open(url, "_blank", "noopener,noreferrer");
                          return;
                        }
                        if (st === "jellyfin") {
                          if (!jfExternalUrl) return;
                          const url = `${jfExternalUrl.replace(/\/$/, "")}/web/#/details?id=${encodeURIComponent(r.item_id)}`;
                          window.open(url, "_blank", "noopener,noreferrer");
                          return;
                        }
                      }}
                    >
                      {displayName}
                    </span>
                  </div>
                </td>
                <td className="py-3">{displayType}</td>
                <td className="py-3 text-right" title={fmtTooltipTime(r.hours)}>
                  {fmtHours(r.hours)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
      {rows.length === 0 && !isLoading && (
        <div className="text-gray-500 text-center py-4">No items found</div>
      )}
    </Card>
  );
}
