// app/src/components/NowPlaying.tsx
import { useCallback, useEffect, useMemo, useState } from "react";
import { useNowPlaying, type NowEntry } from "../contexts/NowPlayingContext";

const apiBase =
  (typeof window !== "undefined" && (window as any).NEXT_PUBLIC_API_BASE) ||
  process.env.NEXT_PUBLIC_API_BASE ||
  "";

export default function NowPlaying() {
  // Get sessions from context instead of managing WebSocket locally
  const { sessions, error } = useNowPlaying();

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
                            : s.poster || "/placeholder-poster.jpg"
                        }
                        alt={s.title || "Unknown"}
                        className="w-20 h-28 object-cover rounded shadow-sm"
                      />
                    </div>

                    {/* Content column - variable width to balance card design */}
                    <div className="flex-1 min-w-0">
                      <h3 className="font-bold text-lg text-white leading-tight mb-2 line-clamp-2">
                        {s.title || "Unknown Title"}
                      </h3>
                      <div className="text-sm text-gray-300 space-y-1 mb-3">
                        <div>
                          <span className="font-medium text-emerald-400">{s.user}</span>
                        </div>
                        <div>{s.app || s.device || "Unknown Client"}</div>
                      </div>

                      {/* Playback progress */}
                      <div className="mt-auto">
                        <div className="flex items-center justify-between text-xs text-gray-400 mb-1">
                          <span>Progress</span>
                          <span>{progress}%</span>
                        </div>
                        <div className="h-2 bg-neutral-700 rounded-full overflow-hidden">
                          <div
                            className="h-full bg-emerald-500 transition-all duration-300"
                            style={{ width: `${progress}%` }}
                          />
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* Quality indicators */}
                  <div className="mt-4 space-y-3 flex-1">
                    <div className="flex flex-wrap gap-2">
                      <Chip tone="ok" label={s.play_method || "Unknown"} />
                      {s.container && <Chip tone="ok" label={s.container.toUpperCase()} />}
                      {s.width && s.height && (
                        <Chip tone="ok" label={`${s.width}×${s.height}`} />
                      )}
                    </div>

                    {/* Video & Audio streams */}
                    <div className="space-y-2 text-sm">
                      <div className="flex items-center justify-between">
                        <span className="text-gray-400">Video:</span>
                        <div className="flex items-center gap-2">
                          <span className="text-white">{s.video || "Unknown"}</span>
                          {isVideoTrans && <Chip tone="warn" label="Transcoding" />}
                        </div>
                      </div>
                      <div className="flex items-center justify-between">
                        <span className="text-gray-400">Audio:</span>
                        <div className="flex items-center gap-2">
                          <span className="text-white">{s.audio || "Unknown"}</span>
                          {isAudioTrans && <Chip tone="warn" label="Transcoding" />}
                        </div>
                      </div>
                      <div className="flex items-center justify-between">
                        <span className="text-gray-400">Subtitles:</span>
                        <span className="text-white">{s.subs || "None"}</span>
                      </div>
                      {s.bitrate > 0 && (
                        <div className="flex items-center justify-between">
                          <span className="text-gray-400">Bitrate:</span>
                          <span className="text-white">
                            {(s.bitrate / 1_000_000).toFixed(1)} Mbps
                          </span>
                        </div>
                      )}
                    </div>

                    {/* Transcoding progress bar (only if transcoding) */}
                    {(isVideoTrans || isAudioTrans) && s.trans_pct !== undefined && (
                      <div>
                        <div className="flex items-center justify-between text-xs text-gray-400 mb-1">
                          <span>Transcoding</span>
                          <span>{pct(s.trans_pct)}%</span>
                        </div>
                        <div className="h-1.5 bg-neutral-700 rounded-full overflow-hidden">
                          <div
                            className="h-full bg-orange-500 transition-all duration-300"
                            style={{ width: `${pct(s.trans_pct)}%` }}
                          />
                        </div>
                      </div>
                    )}

                    {/* Admin controls */}
                    <div className="flex gap-2 pt-2 border-t border-neutral-700">
                      <button
                        onClick={() => send(s.session_id, "pause")}
                        className="flex-1 px-3 py-1.5 bg-neutral-700 hover:bg-neutral-600 rounded text-xs font-medium transition-colors"
                      >
                        Pause
                      </button>
                      <button
                        onClick={() => send(s.session_id, "unpause")}
                        className="flex-1 px-3 py-1.5 bg-neutral-700 hover:bg-neutral-600 rounded text-xs font-medium transition-colors"
                      >
                        Resume
                      </button>
                      <button
                        onClick={() => send(s.session_id, "stop")}
                        className="flex-1 px-3 py-1.5 bg-red-700 hover:bg-red-600 rounded text-xs font-medium transition-colors"
                      >
                        Stop
                      </button>
                    </div>
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