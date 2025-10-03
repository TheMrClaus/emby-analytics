// app/src/components/NowPlaying.tsx
import { useEffect, useMemo, useState } from "react";
import { useNowPlaying, type NowEntry } from "../contexts/NowPlayingContext";
import { fetchNowPlayingSummary } from "../lib/api";
import Image from "next/image";

const apiBase =
  (typeof window !== "undefined" &&
    (window as unknown as { NEXT_PUBLIC_API_BASE?: string }).NEXT_PUBLIC_API_BASE) ||
  process.env.NEXT_PUBLIC_API_BASE ||
  "";

export default function NowPlaying() {
  // Get sessions from context instead of managing WebSocket locally
  const { sessions, error } = useNowPlaying();
  const [summary, setSummary] = useState<{
    outbound_mbps: number;
    active_streams: number;
    active_transcodes: number;
  } | null>(null);
  const [msgOpen, setMsgOpen] = useState<Record<string, boolean>>({});
  const [msgText, setMsgText] = useState<Record<string, string>>({});
  const [overflowTitles, setOverflowTitles] = useState<Record<string, boolean>>({});
  const keyFor = (s: NowEntry) => `${(s.server_type || "emby").toLowerCase()}|${s.session_id}`;

  // Poll summary every 5s
  useEffect(() => {
    let stop = false;
    const load = async () => {
      try {
        const s = await fetchNowPlayingSummary();
        if (!stop) setSummary(s);
      } catch {
        // ignore
      }
    };
    // initial
    void load();
    const id = setInterval(load, 5000);
    return () => {
      stop = true;
      clearInterval(id);
    };
  }, []);

  // If any card is transcoding, we’ll render invisible placeholders
  // in non-transcoding cards to visually equalize heights subtly.
  const anyTranscoding = useMemo(
    () =>
      sessions.some(
        (s) =>
          (s.video_method || "Direct Play") === "Transcode" ||
          (s.audio_method || "Direct Play") === "Transcode"
      ),
    [sessions]
  );

  // Crossfade + parallax state
  const [bgA, setBgA] = useState<string>("");
  const [bgB, setBgB] = useState<string>("");
  const [useA, setUseA] = useState<boolean>(true); // which layer is "on"
  const [parallaxY, setParallaxY] = useState<number>(0);

  // Compute next hero URL from the first session
  const nextHeroUrl = useMemo(() => {
    const first = sessions[0];
    if (!first?.item_id) return "";
    const server = (first.server_type || "emby").toLowerCase();
    const path = `/img/backdrop/${server}/${encodeURIComponent(first.item_id)}`;
    if (!apiBase) return path;
    return `${apiBase}${path}`;
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

  // Local ticking state to update time display every second without burdening server
  const [_clockTick, setClockTick] = useState(0);
  useEffect(() => {
    const id = setInterval(() => setClockTick((t) => (t + 1) % 1_000_000), 1000);
    return () => clearInterval(id);
  }, []);

  const sendServer = async (
    entry: NowEntry,
    action: "pause" | "unpause" | "stop" | "message",
    messageText?: string
  ) => {
    const alias = (entry.server_type || "emby").toLowerCase();
    const sid = entry.session_id;
    try {
      if (action === "pause" || action === "unpause") {
        await fetch(`${apiBase}/api/now/sessions/${alias}/${sid}/pause`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ paused: action === "pause" }),
        });
      } else if (action === "stop") {
        await fetch(`${apiBase}/api/now/sessions/${alias}/${sid}/stop`, { method: "POST" });
      } else if (action === "message") {
        await fetch(`${apiBase}/api/now/sessions/${alias}/${sid}/message`, {
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

  const toggleMsg = (id: string) => {
    const s = sessions.find((x) => x.session_id === id);
    const k = s ? keyFor(s) : id;
    setMsgOpen((prev) => ({ ...prev, [k]: !prev[k] }));
  };
  const setMsg = (id: string, v: string) => {
    const s = sessions.find((x) => x.session_id === id);
    const k = s ? keyFor(s) : id;
    setMsgText((prev) => ({ ...prev, [k]: v }));
  };
  const sendMsg = async (id: string) => {
    const text = (msgText[id] ?? "").trim() || "Hello from Emby Analytics";
    const entry = sessions.find((x) => x.session_id === id);
    if (!entry) return;
    await sendServer(entry, "message", text);
    const k = keyFor(entry);
    setMsgOpen((prev) => ({ ...prev, [k]: false }));
  };

  // ---------- UI helpers ----------
  const theme = (serverType?: string) => {
    const t = (serverType || "emby").toLowerCase();
    switch (t) {
      case "plex":
        return { text: "text-[#e5a00d]", bar: "bg-[#e5a00d]", hex: "#e5a00d" };
      case "jellyfin":
        return { text: "text-[#aa5cc8]", bar: "bg-[#aa5cc8]", hex: "#aa5cc8" };
      case "emby":
      default:
        return { text: "text-[#52b54b]", bar: "bg-[#52b54b]", hex: "#52b54b" };
    }
  };

  const hexToRgb = (hex: string): { r: number; g: number; b: number } => {
    const m = /^#?([a-f\d]{2})([a-f\d]{2})([a-f\d]{2})$/i.exec(hex);
    if (!m) return { r: 82, g: 181, b: 75 };
    return { r: parseInt(m[1], 16), g: parseInt(m[2], 16), b: parseInt(m[3], 16) };
  };

  const getPosterUrl = (entry: NowEntry): string => {
    // Episodes use series poster for consistent aspect ratio
    if (entry.item_type === "Episode" && entry.series_id) {
      const server = (entry.server_type || "emby").toLowerCase();
      return `${apiBase}/img/primary/${server}/${entry.series_id}`;
    }

    // Use provided poster
    if (entry.poster) {
      return entry.poster.startsWith("/img/")
        ? `${apiBase}${entry.poster}`
        : entry.poster;
    }

    // Fallback to placeholder
    return "/placeholder-poster.jpg";
  };
  const Chip = ({ tone, label }: { tone: "ok" | "warn"; label: string }) => (
    <span
      className={[
        "px-1.5 py-0.5 rounded-full text-[10px] font-medium border whitespace-nowrap",
        tone === "ok"
          ? "bg-green-500/20 text-green-400 border-green-400/30"
          : "bg-orange-500/20 text-orange-400 border-orange-400/30",
      ].join(" ")}
    >
      {label}
    </span>
  );

  // Small inline icons for admin controls (no external deps)
  const Icon = ({
    name,
    className,
  }: {
    name: "pause" | "play" | "stop" | "message";
    className?: string;
  }) => {
    if (name === "pause") {
      return (
        <svg viewBox="0 0 24 24" fill="currentColor" className={className || "w-4 h-4"} aria-hidden>
          <path d="M6 5h4v14H6V5zm8 0h4v14h-4V5z" />
        </svg>
      );
    }
    if (name === "play") {
      return (
        <svg viewBox="0 0 24 24" fill="currentColor" className={className || "w-4 h-4"} aria-hidden>
          <path d="M8 5v14l11-7L8 5z" />
        </svg>
      );
    }
    if (name === "message") {
      return (
        <svg viewBox="0 0 24 24" fill="currentColor" className={className || "w-4 h-4"} aria-hidden>
          <path d="M4 4h16a2 2 0 0 1 2 2v9a2 2 0 0 1-2 2H9l-5 3v-3H4a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2z" />
        </svg>
      );
    }
    // stop
    return (
      <svg viewBox="0 0 24 24" fill="currentColor" className={className || "w-4 h-4"} aria-hidden>
        <path d="M6 6h12v12H6z" />
      </svg>
    );
  };

  const pct = (n: number) => Math.min(100, Math.max(0, Math.floor(Number.isFinite(n) ? n : 0)));

  const fmtHMS = (sec?: number) => {
    if (!Number.isFinite(sec as number) || (sec as number) <= 0) return "00:00";
    const s = Math.max(0, Math.floor(sec as number));
    const hh = Math.floor(s / 3600);
    const mm = Math.floor((s % 3600) / 60);
    const ss = s % 60;
    const pad = (n: number) => n.toString().padStart(2, "0");
    return hh > 0 ? `${pad(hh)}:${pad(mm)}:${pad(ss)}` : `${pad(mm)}:${pad(ss)}`;
  };

  const topBadge = (s: NowEntry): { label: string; tone: "ok" | "warn" } => {
    const isVideoTrans = (s.video_method || "Direct Play") === "Transcode";
    const isAudioTrans = (s.audio_method || "Direct Play") === "Transcode";
    if (isVideoTrans) return { label: "Video Transcode", tone: "warn" };
    if (isAudioTrans) return { label: "Audio Transcode", tone: "warn" };
    return { label: "Direct Play", tone: "ok" };
  };

  // Build labels for per-stream status
  const videoStatus = (s: NowEntry) => {
    const trans = (s.video_method || "Direct Play") === "Transcode";
    if (trans) {
      const to = s.trans_video_to?.toUpperCase();
      return { label: to ? `Transcode → ${to}` : "Transcoding", tone: "warn" as const };
    }
    return { label: "Direct Play", tone: "ok" as const };
  };

  const audioStatus = (s: NowEntry) => {
    const trans = (s.audio_method || "Direct Play") === "Transcode";
    if (trans) {
      const to = s.trans_audio_to?.toUpperCase();
      return { label: to ? `Transcode → ${to}` : "Transcoding", tone: "warn" as const };
    }
    return { label: "Direct Play", tone: "ok" as const };
  };

  // Heuristic: subtitles count as "burn‑in/transcoding" if video is transcoding
  // AND the reason mentions subs/burn, otherwise assume direct.
  const subsStatus = (s: NowEntry) => {
    const isVideoTrans = (s.video_method || "Direct Play") === "Transcode";
    const reason = (s.trans_reason || "").toLowerCase();
    const burnIn = isVideoTrans && /(sub|subtitle|burn)/.test(reason);
    if (burnIn) return { label: "Burn‑in", tone: "warn" as const };
    return { label: "Direct Play", tone: "ok" as const };
  };

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
        <div className="flex items-center gap-3 flex-wrap">
          {(() => {
            const first = sessions[0];
            const th = theme(first?.server_type);
            const rgb = hexToRgb(th.hex);
            return (
              <>
                <h2 className="ty-title" style={{ color: th.hex }}>
                  Now Playing
                </h2>
                <div className="flex items-center gap-2 text-xs">
                  <span
                    className="px-2 py-1 rounded-full border"
                    style={{
                      borderColor: `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, 0.3)`,
                      backgroundColor: `rgba(${rgb.r}, ${rgb.g}, ${rgb.b}, 0.1)`,
                      color: th.hex,
                    }}
                    title="5s rolling average of all active session bitrates."
                  >
                    Outbound: {summary ? (summary.outbound_mbps ?? 0).toFixed(1) : "0.0"} Mbps
                  </span>
                  <span className="px-2 py-1 rounded-full border border-gray-500/30 bg-gray-500/10 text-gray-200">
                    Streams: {summary ? summary.active_streams : 0}
                  </span>
                  <span className="px-2 py-1 rounded-full border border-orange-500/30 bg-orange-500/10 text-orange-300">
                    Transcodes: {summary ? summary.active_transcodes : 0}
                  </span>
                </div>
              </>
            );
          })()}
        </div>

        {error && <div className="text-red-400 text-sm">{error}</div>}

        {sessions.length === 0 ? (
          <div className="text-gray-500 text-sm">Nobody is watching right now.</div>
        ) : (
          <div className="flex flex-wrap gap-4 items-start">
            {sessions.map((s) => {
              const isVideoTrans = (s.video_method || "Direct Play") === "Transcode";
              const isAudioTrans = (s.audio_method || "Direct Play") === "Transcode";
              // Use server-reported progress by default; freeze when paused; live-tick when playing
              const progressServer = pct(s.progress_pct);
              const hasTime = (s.duration_sec ?? 0) > 0;
              // Derive client-side ticking position using server timestamp
              const deltaSec = Math.max(0, Math.floor((Date.now() - s.timestamp) / 1000));
              const livePos = hasTime
                ? Math.min(
                    s.duration_sec as number,
                    (s.position_sec || 0) + (s.is_paused ? 0 : deltaSec)
                  )
                : undefined;
              const progress = hasTime
                ? Math.max(
                    0,
                    Math.min(
                      100,
                      ((s.is_paused ? s.position_sec || 0 : livePos || 0) / (s.duration_sec || 1)) *
                        100
                    )
                  )
                : progressServer;
              const top = topBadge(s);
              const v = videoStatus(s);
              const a = audioStatus(s);
              const sub = subsStatus(s);

              const th = theme(s.server_type);
              return (
                <article
                  key={s.session_id}
                  className="card overflow-hidden flex flex-col p-3 flex-none w-[290px] rounded-lg"
                  style={{ borderColor: th.hex, borderWidth: 3, borderStyle: "solid" }}
                >
                  {/* Top row: poster + title/meta arranged symmetrically */}
                  <div className="flex gap-3">
                    {/* Poster column - fixed size to align all cards */}
                    <div className="shrink-0">
                      <Image
                        src={getPosterUrl(s)}
                        alt={s.title || "Unknown"}
                        width={64}
                        height={96}
                        className="object-cover rounded shadow-sm"
                        unoptimized
                        priority={false}
                      />
                    </div>

                    {/* Content column - variable width to balance card design */}
                    <div className="flex-1 min-w-0">
                      <div
                        className="ticker-wrapper mb-1.5 pl-2 h-[24px] overflow-hidden whitespace-nowrap"
                        ref={(wrapper) => {
                          if (wrapper) {
                            const key = keyFor(s);
                            const title = wrapper.querySelector("h3");
                            if (title) {
                              // Check on next frame to ensure proper measurement
                              requestAnimationFrame(() => {
                                const isOverflowing = title.scrollWidth > wrapper.clientWidth;
                                if (overflowTitles[key] !== isOverflowing) {
                                  setOverflowTitles((prev) => ({ ...prev, [key]: isOverflowing }));
                                }
                              });
                            }
                          }
                        }}
                      >
                        <h3
                          className={`font-semibold text-base text-white leading-snug inline-block ${
                            overflowTitles[keyFor(s)] ? "ticker-content" : ""
                          }`}
                        >
                          {overflowTitles[keyFor(s)]
                            ? `${s.title || "Unknown Title"} \u00A0\u00A0\u00A0 ${s.title || "Unknown Title"}`
                            : s.title || "Unknown Title"}
                        </h3>
                      </div>
                      <div className="text-xs text-gray-300 space-y-0.5 mb-2">
                        <div className="flex items-center justify-between gap-2">
                          <span className={`font-medium ${theme(s.server_type).text}`}>
                            {s.user}
                          </span>
                          <Chip tone={top.tone} label={top.label} />
                        </div>
                        <div className="flex items-center justify-between gap-2">
                          <div>{s.app || s.device || "Unknown Client"}</div>
                          <div className="flex gap-1">
                            {s.width && s.height && <Chip tone="ok" label={`${s.width}×${s.height}`} />}
                            {(() => {
                              const isVideoTrans = (s.video_method || "Direct Play") === "Transcode";
                              const isAudioTrans = (s.audio_method || "Direct Play") === "Transcode";
                              const remuxOnly =
                                (s.play_method || "") === "Transcode" && !isVideoTrans && !isAudioTrans;
                              if (!remuxOnly) return null;
                              const sp = (s.stream_path || "").toUpperCase();
                              const label = sp ? `Remux/${sp}` : "Remux";
                              return <Chip tone="ok" label={label} />;
                            })()}
                          </div>
                        </div>
                      </div>

                      {/* Progress moved below media rows to match their width */}
                    </div>
                  </div>

                  {/* Quality indicators */}
                  <div className="mt-3 flex-1 text-sm">
                    {/* Wrap details + progress to card width for proper mobile layout */}
                    <div className="grid w-full gap-1.5">
                      {/* Slim inline rows with no large spacing */}
                      <div className="text-gray-300">
                        <span className="text-gray-400">Video: </span>
                        <span className="text-white">{s.video || "Unknown"}</span>{" "}
                        <span
                          className={v.tone === "warn" ? "text-orange-400" : "text-emerald-400"}
                        >
                          {v.label}
                        </span>
                      </div>
                      <div className="text-gray-300">
                        <span className="text-gray-400">Audio: </span>
                        <span className="text-white">{s.audio || "Unknown"}</span>{" "}
                        <span
                          className={a.tone === "warn" ? "text-orange-400" : "text-emerald-400"}
                        >
                          {a.label}
                        </span>
                      </div>
                      <div className="text-gray-300">
                        <span className="text-gray-400">Subtitles: </span>
                        <span className="text-white">{s.subs || "None"}</span>{" "}
                        <span
                          className={sub.tone === "warn" ? "text-orange-400" : "text-emerald-400"}
                        >
                          {sub.label}
                        </span>
                      </div>
                      {s.bitrate > 0 && (
                        <div className="text-gray-300">
                          <span className="text-gray-400">Bitrate: </span>
                          <span className="text-white">
                            {(s.bitrate / 1_000_000).toFixed(1)} Mbps
                          </span>
                        </div>
                      )}
                      {/* Progress bound to the width of this block */}
                      <div>
                        <div className="flex items-center justify-between text-[11px] text-gray-400 mb-1">
                          <span>Progress</span>
                          <span>
                            {hasTime
                              ? `${fmtHMS(livePos)} / ${fmtHMS(s.duration_sec)}`
                              : `${progress}%`}
                          </span>
                        </div>
                        <div className="h-1.5 bg-neutral-700 rounded-full overflow-hidden w-full">
                          <div
                            className={`h-full ${theme(s.server_type).bar} transition-all duration-300`}
                            style={{ width: `${progress}%` }}
                          />
                        </div>
                      </div>
                      {/* If anything is transcoding, show the reason */}
                      {(isVideoTrans || isAudioTrans) && s.trans_reason && (
                        <div className="text-xs text-gray-400">
                          Reason: <span className="text-white">{s.trans_reason}</span>
                        </div>
                      )}
                      {/* Placeholder to balance height if any card is transcoding */}
                      {anyTranscoding && !(isVideoTrans || isAudioTrans) && (
                        <div className="text-xs text-transparent select-none" aria-hidden>
                          Reason: placeholder
                        </div>
                      )}
                      {/* Transcoding progress bar (only if transcoding) */}
                      {(isVideoTrans || isAudioTrans) && s.trans_pct !== undefined && (
                        <div>
                          <div className="flex items-center justify-between text-[11px] text-gray-400 mb-1">
                            <span>Transcoding</span>
                            <span>{pct(s.trans_pct)}%</span>
                          </div>
                          <div className="h-1.5 bg-neutral-700 rounded-full overflow-hidden w-full">
                            <div
                              className="h-full bg-orange-500 transition-all duration-300"
                              style={{ width: `${pct(s.trans_pct)}%` }}
                            />
                          </div>
                        </div>
                      )}
                      {/* Placeholder bar to balance height if any card is transcoding */}
                      {anyTranscoding && !(isVideoTrans || isAudioTrans) && (
                        <div aria-hidden>
                          <div className="flex items-center justify-between text-[11px] text-transparent mb-1">
                            <span>Transcoding</span>
                            <span>00%</span>
                          </div>
                          <div className="h-1.5 bg-transparent rounded-full overflow-hidden w-full">
                            <div className="h-full" />
                          </div>
                        </div>
                      )}
                    </div>
                    {/* Admin controls - icon-only, tight spacing; width bound to same block */}
                    <div className="pt-2 w-full">
                      <div className="flex items-center gap-2">
                        {(() => {
                          const isPlex = (s.server_type || "").toLowerCase() === "plex";
                          return (
                            <button
                              onClick={() => {
                                if (!isPlex) void sendServer(s, "pause");
                              }}
                              className={`p-2 rounded transition-colors ${isPlex ? "bg-neutral-800 text-neutral-500 cursor-not-allowed" : "bg-neutral-700 hover:bg-neutral-600"}`}
                              aria-label="Pause"
                              title={isPlex ? "Not supported" : "Pause"}
                              disabled={isPlex}
                            >
                              <Icon name="pause" />
                            </button>
                          );
                        })()}
                        {(() => {
                          const isPlex = (s.server_type || "").toLowerCase() === "plex";
                          return (
                            <button
                              onClick={() => {
                                if (!isPlex) void sendServer(s, "unpause");
                              }}
                              className={`p-2 rounded transition-colors ${isPlex ? "bg-neutral-800 text-neutral-500 cursor-not-allowed" : "bg-neutral-700 hover:bg-neutral-600"}`}
                              aria-label="Resume"
                              title={isPlex ? "Not supported" : "Resume"}
                              disabled={isPlex}
                            >
                              <Icon name="play" />
                            </button>
                          );
                        })()}
                        {(() => {
                          const isPlex = (s.server_type || "").toLowerCase() === "plex";
                          return (
                            <button
                              onClick={() => {
                                if (!isPlex) toggleMsg(s.session_id);
                              }}
                              className={`p-2 rounded transition-colors ${isPlex ? "bg-neutral-800 text-neutral-500 cursor-not-allowed" : "bg-neutral-700 hover:bg-neutral-600"}`}
                              aria-label="Message"
                              title={isPlex ? "Not supported" : "Message"}
                              disabled={isPlex}
                            >
                              <Icon name="message" />
                            </button>
                          );
                        })()}
                        <button
                          onClick={() => sendServer(s, "stop")}
                          className="p-2 bg-red-700 hover:bg-red-600 rounded transition-colors"
                          aria-label="Stop"
                          title="Stop"
                        >
                          <Icon name="stop" />
                        </button>
                      </div>
                      {msgOpen[keyFor(s)] && (
                        <div className="mt-2 flex items-center gap-2">
                          <input
                            autoFocus
                            type="text"
                            value={msgText[keyFor(s)] ?? ""}
                            onChange={(e) => setMsg(s.session_id, e.target.value)}
                            onKeyDown={(e) => {
                              if (e.key === "Enter") {
                                e.preventDefault();
                                void sendMsg(s.session_id);
                              } else if (e.key === "Escape") {
                                e.preventDefault();
                                toggleMsg(s.session_id);
                              }
                            }}
                            placeholder="Type a message…"
                            className="px-2 py-1 text-sm bg-neutral-800 border border-neutral-600 rounded text-white placeholder:text-neutral-400 min-w-[180px] z-10 relative"
                          />
                          <button
                            onClick={() => void sendServer(s, "message", msgText[keyFor(s)])}
                            className="px-2 py-1 text-xs rounded bg-emerald-600 hover:bg-emerald-500 text-white"
                          >
                            Send
                          </button>
                          <button
                            onClick={() => toggleMsg(s.session_id)}
                            className="px-2 py-1 text-xs rounded bg-neutral-700 hover:bg-neutral-600 text-white"
                          >
                            Cancel
                          </button>
                        </div>
                      )}
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
