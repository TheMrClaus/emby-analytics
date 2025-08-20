// app/src/components/NowPlaying.tsx
import { useNowStream } from "../hooks/useNowStream";
import { imgPrimary } from "../lib/api";

export default function NowPlaying() {
  const { items, error } = useNowStream();

  return (
    <div className="card p-4">
      <div className="h3 mb-2">Now Playing</div>
      {error ? <div className="ty-muted mb-2">{error}</div> : null}
      {items.length === 0 ? (
        <div className="ty-muted">No active sessions</div>
      ) : (
        <ul className="grid sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {items.map((s) => (
            <li key={s.session_id} className="flex gap-3">
              <img src={imgPrimary(s.item_id)} alt="" className="w-12 h-18 object-cover rounded" />
              <div>
                <div className="font-medium">{s.title}</div>
                <div className="ty-muted text-xs">
                  {s.user} • {s.app} on {s.device}
                </div>
                <div className="ty-caption mt-1">
                  {s.video}{s.audio ? ` • ${s.audio}` : ""}{s.subs ? ` • Subs: ${s.subs}` : ""}
                </div>
                <div className="ty-caption">Progress: {Math.round(s.progress_pct)}%</div>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

