// app/src/components/NowPlaying.tsx
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNowStream } from "../hooks/useNowStream";


type NowEntry = {
  timestamp: number;
  title: string;
  user: string;
  app: string;
  device: string;
  play_method: string;
  video: string;
  audio: string;
  subs: string;
  bitrate: number;
  progress_pct: number;
  poster: string;
  session_id: string;
  item_id: string;
  item_type?: string;
  container?: string;
  width?: number;
  height?: number;
  dolby_vision?: boolean;
  hdr10?: boolean;
  audio_lang?: string;
  audio_ch?: number;
  sub_lang?: string;
  sub_codec?: string;
  trans_video_from?: string;
  trans_video_to?: string;
  trans_audio_from?: string;
  trans_audio_to?: string;
  video_method?: string;
  audio_method?: string;
  stream_path?: string;
  stream_detail?: string;
  trans_reason?: string;
  trans_pct?: number;
  trans_audio_bitrate?: number;
  trans_video_bitrate?: number;
};

const apiBase =
  (typeof window !== "undefined" && (window as any).NEXT_PUBLIC_API_BASE) ||
  process.env.NEXT_PUBLIC_API_BASE ||
  "";

export default function NowPlaying() {
  const [sessions, setSessions] = useState<NowEntry[]>([]);
  const [error, setError] = useState<string | null>(null);
  const wsRef = useRef<WebSocket | null>(null);

  const wsURL = useMemo(() => {
    if (typeof window === "undefined") return "";
    const proto = window.location.protocol === "https:" ? "wss" : "ws";
    return `${proto}://${window.location.host}/now/ws`;
  }, []);

  const loadSnapshot = useCallback(async () => {
    try {
      const res = await fetch(`${apiBase}/now/snapshot`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: NowEntry[] = await res.json();
      setSessions(data || []);
      setError(null);
    } catch (e: any) {
      setError(`Failed to load now playing: ${e.message ?? e}`);
    }
  }, []);

  const connectWS = useCallback(() => {
    if (!wsURL) return;
    try {
      const ws = new WebSocket(wsURL);
      wsRef.current = ws;

      ws.onmessage = (ev) => {
        try {
          const data: NowEntry[] = JSON.parse(ev.data);
          setSessions(Array.isArray(data) ? data : []);
        } catch {/* ignore parse errors */}
      };
      ws.onerror = () => {
        // Leave a snapshot fallback so the section still shows something.
        if (!sessions.length) loadSnapshot();
      };
      ws.onclose = () => {
        // simple reconnect with backoff
        setTimeout(connectWS, 2000);
      };
    } catch {
      /* noop */
    }
  }, [wsURL, loadSnapshot, sessions.length]);

  useEffect(() => {
    loadSnapshot();
    connectWS();
    return () => {
      try { wsRef.current?.close(); } catch {}
    };
  }, [loadSnapshot, connectWS]);

  const send = async (
    sessionId: string,
    action: "pause" | "unpause" | "stop" | "message",
    messageText?: string
  ) => {
    try {
      if (action === "pause" || action === "unpause") {
        await fetch(`${apiBase}/now/${sessionId}/pause`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ paused: action === "pause" }),
        });
      } else if (action === "stop") {
        await fetch(`${apiBase}/now/${sessionId}/stop`, { method: "POST" });
      } else if (action === "message") {
        await fetch(`${apiBase}/now/${sessionId}/message`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            header: "Admin message",
            text: messageText ?? "Hello from Emby Analytics",
            timeout_ms: 4000,
          }),
        });
      }
    } catch {/* ignore */}
  };

// app/src/components/NowPlaying.tsx

  return (
    <section className="relative">
      {/* Backdrop (first active session only) */}
      {sessions.length > 0 && sessions[0]?.item_id ? (
        <>
          <div
            className="hero-bg"
            style={{
              backgroundImage: `url(${apiBase}/img/backdrop/${encodeURIComponent(
                sessions[0].item_id
              )})`,
            }}
          />
          <div className="hero-overlay" />
        </>
      ) : null}

      {/* Foreground content */}
      <div className="relative z-10 space-y-4">
        <h2 className="ty-title">Now Playing</h2>

        {error && <div className="text-red-400 text-sm">{error}</div>}

        {sessions.length === 0 ? (
          <div className="ty-muted text-sm">Nobody is watching right now.</div>
        ) : (
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
            {sessions.map((s) => (
              <article key={s.session_id} className="card p-4 flex gap-4">
                {/* poster */}
                <img
                  src={
                    s.poster?.startsWith("/img/")
                      ? `${apiBase}${s.poster}`
                      : `${apiBase}/img/primary/${encodeURIComponent(s.item_id)}`
                  }
                  alt={s.title}
                  className="w-20 h-28 object-cover rounded-lg border border-white/10"
                />

                {/* details */}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <div className="h3 truncate">{s.title}</div>
                    <span className="badge">{s.user}</span>
                  </div>
                  <div className="ty-muted truncate">
                    {s.app} • {s.device}
                  </div>

                  <div className="mt-2 text-sm space-y-1">
                    <div className="font-medium">Stream</div>
                    <div>
                      {s.container} ({(s.bitrate / 1_000_000).toFixed(1)} Mbps)
                    </div>
                    {s.trans_reason && <div>{s.trans_reason}</div>}

                    <div className="font-medium mt-1">Video</div>
                    <div>
                      {s.width}x{s.height} {s.video} • {s.video_method || "Direct Play"}
                    </div>

                    <div className="font-medium mt-1">Audio</div>
                    <div>
                      {s.audio} • {s.audio_method || "Direct Play"}
                    </div>

                    {s.subs && (
                      <>
                        <div className="font-medium mt-1">Subs</div>
                        <div>
                          {s.subs} • {s.sub_codec || "—"}
                        </div>
                      </>
                    )}
                  </div>

                  <div className="mt-3">
                    <div className="ty-caption mb-1">
                      Progress {Math.floor(s.progress_pct)}%
                    </div>
                    <div className="w-full h-2 bg-white/10 rounded-full overflow-hidden">
                      <div
                        className="h-2 bg-white/60"
                        style={{
                          width: `${Math.min(100, Math.max(0, s.progress_pct))}%`,
                        }}
                      />
                    </div>
                  </div>

                  {/* controls */}
                  <div className="mt-3 flex gap-2">
                    <button className="badge" onClick={() => send(s.session_id, "pause")}>
                      Pause
                    </button>
                    <button className="badge" onClick={() => send(s.session_id, "unpause")}>
                      Resume
                    </button>
                    <button className="badge" onClick={() => send(s.session_id, "stop")}>
                      Stop
                    </button>
                    <button
                      className="badge"
                      onClick={() => {
                        const txt = prompt("Send a message:", "Hello!");
                        if (txt != null) send(s.session_id, "message", txt);
                      }}
                    >
                      Message
                    </button>
                  </div>
                </div>
              </article>
            ))}
          </div>
        )}
      </div>
    </section>
  );
}