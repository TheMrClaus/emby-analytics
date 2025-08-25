// app/src/components/NowPlaying.tsx
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

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

  // Crossfade + parallax state
  const [bgA, setBgA] = useState<string>("");
  const [bgB, setBgB] = useState<string>("");
  const [useA, setUseA] = useState<boolean>(true); // which layer is "on"
  const [parallaxY, setParallaxY] = useState<number>(0);

  // Compute next hero URL from the first session
  const nextHeroUrl = useMemo(() => {
    const first = sessions[0];
    if (!first?.item_id) return "";
    return `${apiBase}/img/backdrop/${encodeURIComponent(first.item_id)}`;
  }, [sessions]);

  // When the first session changes, crossfade layers
  useEffect(() => {
    if (!nextHeroUrl) return;
    if (useA) {
      setBgB(nextHeroUrl);
      requestAnimationFrame(() => setUseA(false));
    } else {
      setBgA(nextHeroUrl);
      requestAnimationFrame(() => setUseA(true));
    }
  }, [nextHeroUrl]); // eslint-disable-line react-hooks/exhaustive-deps

  // Parallax (respect reduced motion)
  useEffect(() => {
    const mql = window.matchMedia("(prefers-reduced-motion: reduce)");
    if (mql.matches) return; // no motion

    const onScroll = () => {
      const y = Math.min(60, window.scrollY * 0.12);
      setParallaxY(y);
    };
    onScroll(); // initialize
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
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
        } catch {
          /* ignore parse errors */
        }
      };
      ws.onerror = () => {
        if (!sessions.length) loadSnapshot();
      };
      ws.onclose = () => {
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
      try {
        wsRef.current?.close();
      } catch {}
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
    } catch {
      /* ignore */
    }
  };

  // ---------- UI helpers ----------
  const Chip = ({
    tone,
    label,
  }: {
    tone: "ok" | "warn";
    label: string;
  }) => (
    <span
      className={[
        "px-2 py-0.5 rounded-full text-xs font-medium border",
        tone === "ok"
          ? "bg-green-500/20 text-green-400 border-green-400/30"
          : "bg-orange-500/20 text-orange-400 border-orange-400/30",
      ].join(" ")}
    >
      {label}
    </span>
  );

  const pct = (n: number) =>
    Math.min(100, Math.max(0, Math.floor(Number.isFinite(n) ? n : 0)));

  return (
    <section className="hero p-6">
      {/* Crossfading, parallaxed backdrop (only if we have any session) */}
      {sessions.length > 0 && nextHeroUrl ? (
        <>
          <div
            className={`hero-layer ${useA ? "opacity-100" : "opacity-0"}`}
            style={{
              backgroundImage: bgA ? `url(${bgA})` : "none",
              transform: `translateY(${parallaxY}px) scale(1.05)`,
            }}
            aria-hidden
          />
          <div
            className={`hero-layer ${useA ? "opacity-0" : "opacity-100"}`}
            style={{
              backgroundImage: bgB ? `url(${bgB})` : "none",
              transform: `translateY(${parallaxY}px) scale(1.05)`,
            }}
            aria-hidden
          />
          <div className="hero-overlay" aria-hidden />
        </>
      ) : null}

      {/* Foreground content */}
      <div className="hero-foreground space-y-5">
        <h2 className="ty-title text-emerald-400">Now Playing</h2>

        {error && <div className="text-red-400 text-sm">{error}</div>}

        {sessions.length === 0 ? (
          <div className="text-gray-500 text-sm">Nobody is watching right now.</div>
        ) : (
          <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-7">
            {sessions.map((s) => {
              const isVideoTrans = (s.video_method || "Direct Play") === "Transcode";
              const isAudioTrans = (s.audio_method || "Direct Play") === "Transcode";
              const progress = pct(s.progress_pct);

              return (
                <article
                  key={s.session_id}
                  className="card overflow-hidden flex flex-col min-h-[380px] p-5"
                >
                  {/* Top row: poster + title/meta arranged symmetrically */}
                  <div className="flex gap-5">
                    {/* Poster column - fixed size to align all cards */}
                    <div className="shrink-0">
                      <img
                        src={
                          s.poster?.startsWith("/img/")
                            ? `${apiBase}${s.poster}`
                            : `${apiBase}/img/primary/${encodeURIComponent(s.item_id)}`
                        }
                        alt={s.title}
                        className="w-24 h-36 object-cover rounded-xl border border-white/10"
                      />
                    </div>

                    {/* Title + meta */}
                    <div className="min-w-0 flex-1 flex flex-col">
                      <div className="flex items-start justify-between gap-3">
                        <h3 className="text-base font-semibold text-white truncate">
                          {s.title}
                        </h3>
                        <span className="badge shrink-0">{s.user}</span>
                      </div>
                      <div className="text-xs text-gray-400 truncate">
                        {s.app} • {s.device}
                      </div>

                      {/* Stream + reasons */}
                      <div className="mt-2 text-sm text-gray-300 space-y-1">
                        <div className="flex items-center gap-2">
                          <span className="font-medium text-gray-200">Stream</span>
                          <span className="text-gray-300 truncate">
                            {s.container} ({(s.bitrate / 1_000_000).toFixed(1)} Mbps)
                          </span>
                        </div>
                        {s.trans_reason && (
                          <div className="text-amber-300/90 text-xs">
                            {s.trans_reason}
                          </div>
                        )}
                      </div>
                    </div>
                  </div>

                  {/* Divider */}
                  <div className="my-4 h-px bg-white/10" />

                  {/* Details grid keeps things aligned and tidy */}
                  <div className="grid grid-cols-2 gap-x-5 gap-y-3 text-sm">
                    {/* Video */}
                    <div className="space-y-1">
                      <div className="text-gray-400 text-xs font-medium tracking-wide">
                        VIDEO
                      </div>
                      <div className="text-gray-200">
                        {s.width}x{s.height} {s.video}
                      </div>
                      <div className="flex items-center gap-2">
                        <Chip
                          tone={isVideoTrans ? "warn" : "ok"}
                          label={s.video_method || "Direct Play"}
                        />
                        {s.dolby_vision && <span className="badge">Dolby Vision</span>}
                        {s.hdr10 && <span className="badge">HDR10</span>}
                      </div>
                    </div>

                    {/* Audio */}
                    <div className="space-y-1">
                      <div className="text-gray-400 text-xs font-medium tracking-wide">
                        AUDIO
                      </div>
                      <div className="text-gray-200">
                        {s.audio}
                        {s.audio_ch ? ` • ${s.audio_ch}.0` : ""}
                        {s.audio_lang ? ` • ${s.audio_lang.toUpperCase()}` : ""}
                      </div>
                      <div className="flex items-center gap-2">
                        <Chip
                          tone={isAudioTrans ? "warn" : "ok"}
                          label={s.audio_method || "Direct Play"}
                        />
                        {typeof s.trans_audio_bitrate === "number" && (
                          <span className="badge">
                            {Math.round(s.trans_audio_bitrate / 1000)} kbps
                          </span>
                        )}
                      </div>
                    </div>

                    {/* Subs */}
                    <div className="space-y-1">
                      <div className="text-gray-400 text-xs font-medium tracking-wide">
                        SUBS
                      </div>
                      <div className="text-gray-200">
                        {s.subs
                          ? `${s.subs}${s.sub_codec ? ` • ${s.sub_codec}` : ""}${
                              s.sub_lang ? ` • ${s.sub_lang.toUpperCase()}` : ""
                            }`
                          : "—"}
                      </div>
                    </div>

                    {/* Transcoding detail (if available) */}
                    <div className="space-y-1">
                      <div className="text-gray-400 text-xs font-medium tracking-wide">
                        TRANSCODE
                      </div>
                      <div className="text-gray-200">
                        {s.trans_video_from && s.trans_video_to
                          ? `${s.trans_video_from} → ${s.trans_video_to}`
                          : "—"}
                      </div>
                      {typeof s.trans_video_bitrate === "number" && (
                        <div className="text-xs text-gray-400">
                          ~{(s.trans_video_bitrate / 1_000_000).toFixed(1)} Mbps
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Progress */}
                  <div className="mt-5">
                    <div className="flex items-center justify-between text-xs text-gray-400 mb-1">
                      <span>Progress</span>
                      <span>{progress}%</span>
                    </div>
                    <div className="w-full h-2 bg-neutral-700/80 rounded-full overflow-hidden">
                      <div
                        className="h-2 rounded-full bg-gradient-to-r from-yellow-400 to-yellow-600"
                        style={{ width: `${progress}%` }}
                      />
                    </div>
                  </div>

                  {/* Push controls to the bottom and center them */}
                  <div className="mt-6 flex-1" />
                  <div className="pb-1 flex items-center justify-center gap-3">
                    <button className="badge px-3 py-1" onClick={() => send(s.session_id, "pause")}>
                      Pause
                    </button>
                    <button className="badge px-3 py-1" onClick={() => send(s.session_id, "unpause")}>
                      Resume
                    </button>
                    <button className="badge px-3 py-1" onClick={() => send(s.session_id, "stop")}>
                      Stop
                    </button>
                    <button
                      className="badge px-3 py-1"
                      onClick={() => {
                        const txt = prompt("Send a message:", "Hello!");
                        if (txt != null) send(s.session_id, "message", txt);
                      }}
                    >
                      Message
                    </button>
                  </div>
                </article>
              );
            })}
          </div>
        )}
      </div>
    </section>
  );
}
