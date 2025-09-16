import { useMultiServer } from "../contexts/MultiServerContext";
import ServerSelector from "./ServerSelector";

const colorByType: Record<string, { ring: string; badge: string; text: string }>= {
  plex: { ring: "ring-[#e5a00d]", badge: "bg-[#e5a00d]/20 text-[#e5a00d]", text: "text-[#e5a00d]" },
  emby: { ring: "ring-[#52b54b]", badge: "bg-[#52b54b]/20 text-[#52b54b]", text: "text-[#52b54b]" },
  jellyfin: { ring: "ring-[#aa5cc8]", badge: "bg-[#aa5cc8]/20 text-[#aa5cc8]", text: "text-[#aa5cc8]" },
};

export default function MultiServerNowPlaying() {
  const { sessions, error, server } = useMultiServer();

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="ty-title">Now Playing</h2>
        <ServerSelector />
      </div>
      {error && (
        <div className="text-sm text-amber-300">{error}</div>
      )}
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {sessions.map((s) => {
          const t = (s.server_type || "emby").toLowerCase();
          const col = colorByType[t] || colorByType.emby;
          const pct = Math.max(0, Math.min(100, s.progress_pct || 0));
          return (
            <div key={s.session_id} className={`relative rounded-2xl border border-neutral-800 bg-neutral-900/60 p-3 ring-2 ${col.ring}`}>
              <div className="absolute top-2 right-2 text-xs px-2 py-0.5 rounded-full border border-neutral-700 bg-neutral-800 text-gray-300">
                <span className={`mr-1 inline-block w-2 h-2 rounded-full ${col.text.replace("text-","bg-")}`}></span>
                {t.charAt(0).toUpperCase() + t.slice(1)}
              </div>
              <div className="flex gap-3">
                <div className="relative w-24 h-36 flex-shrink-0 rounded-lg overflow-hidden bg-neutral-800">
                  {s.poster ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={s.poster} alt="poster" className="object-cover w-full h-full" />
                  ) : (
                    <div className="w-full h-full" />
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="text-base font-semibold truncate">{s.title}</div>
                  <div className="text-xs text-gray-400 truncate">
                    {s.user} · {s.app} · {s.device}
                  </div>
                  <div className="mt-1 text-xs text-gray-300">
                    <span className="mr-2">{s.video}</span>
                    <span className="text-gray-500">•</span>
                    <span className="ml-2">{s.audio}</span>
                  </div>
                  <div className="mt-2 h-2 bg-neutral-800 rounded-full overflow-hidden">
                    <div className={`h-full ${col.text.replace("text-","bg-")}`} style={{ width: `${pct}%` }} />
                  </div>
                  <div className="mt-1 text-xs text-gray-400">
                    {s.position_sec ?? 0}s / {s.duration_sec ?? 0}s · {((s.bitrate || 0)/1_000_000).toFixed(1)} Mbps
                  </div>
                  {s.play_method?.toLowerCase() === "transcode" && (
                    <div className={`inline-block mt-2 text-[11px] px-2 py-0.5 rounded ${col.badge} border border-transparent`}>{s.stream_detail || s.stream_path || "Transcode"}</div>
                  )}
                </div>
              </div>
            </div>
          );
        })}
        {sessions.length === 0 && (
          <div className="text-sm text-gray-400">No active sessions {server !== "all" ? `for ${server}` : "now"}.</div>
        )}
      </div>
    </section>
  );
}
