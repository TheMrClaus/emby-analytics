'use client';
import { useEffect, useMemo, useState } from "react";
import {
  LineChart, Line, XAxis, YAxis, Tooltip, Legend, ResponsiveContainer,
  BarChart, Bar
} from "recharts";

/**
 * API base:
 * - When UI is served by the Go server or same origin behind a reverse proxy (Traefik, etc.),
 *   leave NEXT_PUBLIC_API_BASE unset -> this becomes "" and all calls are relative.
 * - When developing locally, set NEXT_PUBLIC_API_BASE (e.g., http://localhost:8080).
 */
const apiBase = (process.env.NEXT_PUBLIC_API_BASE || "").replace(/\/+$/, "");

type UsageRow = { day: string; user: string; hours: number };
type TopUser = { user: string; hours: number };
type TopItem = { item_id: string | null; hours: number };
type ItemRow = { id: string; name?: string; type?: string; display?: string };
type RefreshState = { running: boolean; imported: number; total?: number; page: number; error: string | null };

export default function Home(){
  const [now, setNow] = useState<any[]>([]);
  const [usage, setUsage] = useState<UsageRow[]>([]);
  const [overview, setOverview] = useState<any>({});
  const [topUsers, setTopUsers] = useState<TopUser[]>([]);
  const [topItems, setTopItems] = useState<TopItem[]>([]);
  const [refresh, setRefresh] = useState<RefreshState>({running:false, imported:0, total:0, page:0, error:null});
  const [qualities, setQualities] = useState<any>({});
  const [codecs, setCodecs] = useState<any>({});
  const [activeUsers, setActiveUsers] = useState<any[]>([]);
  const [totalUsers, setTotalUsers] = useState<number>(0);

  // niceties
  const [syncingUsers, setSyncingUsers] = useState(false);
  const [toast, setToast] = useState<string | null>(null);
  const [itemNameMap, setItemNameMap] = useState<Record<string, string>>({});

  // Pretty time: keep data as hours, render h/m/s nicely
  const fmtAxisTime = (h: number) => {
    if (!isFinite(h) || h <= 0) return "0m";
    if (h < 1/60) return `${Math.round(h * 3600)}s`;      // < 1 min
    if (h < 1)     return `${Math.round(h * 60)}m`;       // < 1 hour
    if (h < 10)    return `${h.toFixed(1)}h`;             // 1–10h
    return `${Math.round(h)}h`;                           // 10h+
  };

  const fmtTooltipTime = (h: number) => {
    if (!isFinite(h) || h <= 0) return "0m";
    const totalMin = Math.round(h * 60);
    if (totalMin < 1) return `${Math.round(h * 3600)}s`;
    if (totalMin < 60) return `${totalMin}m`;
    const hr = Math.floor(totalMin / 60);
    const min = totalMin % 60;
    return min ? `${hr}h ${min}m` : `${hr}h`;
  };

  const fmtInt = (n: number) => {
    if (!Number.isFinite(n)) return "0";
    return Math.trunc(n).toLocaleString();
  };

  // initial fetches + now-playing snapshot + SSE with polling fallback
  useEffect(()=>{
    // stats & overview
    fetch(`${apiBase}/stats/usage?days=14`).then(r=>r.json()).then(setUsage).catch(()=>{});
    fetch(`${apiBase}/stats/overview`).then(r=>r.json()).then(setOverview).catch(()=>{});
    fetch(`${apiBase}/stats/top/users?window=14d&limit=5`).then(r=>r.json()).then(setTopUsers).catch(()=>{});
    fetch(`${apiBase}/stats/top/items?window=14d&limit=5`).then(r=>r.json()).then(setTopItems).catch(()=>{});
    fetch(`${apiBase}/stats/qualities`).then(r=>r.json()).then(setQualities).catch(()=>{});
    fetch(`${apiBase}/stats/codecs?limit=8`).then(r=>r.json()).then(setCodecs).catch(()=>{});
    fetch(`${apiBase}/stats/active-users-lifetime?limit=1`).then(r=>r.json()).then(setActiveUsers).catch(()=>{});
    fetch(`${apiBase}/stats/users/total`).then(r=>r.json()).then(d=>setTotalUsers(d.total_users||0)).catch(()=>{});

    // one-time snapshot so Now Playing renders fast
    fetch(`${apiBase}/now`).then(r=>r.json()).then(rows=>{ if (Array.isArray(rows)) setNow(rows); }).catch(()=>{});

    let pollId: any = null;
    let sseErrors = 0;

    const startPolling = () => {
      if (pollId) return;
      pollId = setInterval(async () => {
        try {
          const rows = await fetch(`${apiBase}/now`, { cache: "no-store" }).then(r=>r.json());
          if (Array.isArray(rows)) setNow(rows);
        } catch {}
      }, 5000);
    };

    const es = new EventSource(`${apiBase}/now/stream`, { withCredentials: true });
    es.onmessage = (e) => {
      try {
        const rows = JSON.parse(e.data || "[]");
        if (Array.isArray(rows)) setNow(rows);
        sseErrors = 0;
      } catch {}
    };
    es.onerror = () => {
      // if SSE fails twice in a row (proxy blocking, etc.), fall back to polling
      sseErrors++;
      if (sseErrors >= 2) startPolling();
    };

    return ()=> {
      es.close();
      if (pollId) clearInterval(pollId);
    };
  }, [apiBase]);

  // refresh status poll (continuous)
  useEffect(()=>{
    let cancelled = false;
    const poll = async () => {
      try {
        const s = await fetch(`${apiBase}/admin/refresh/status`).then(r=>r.json());
        if (!cancelled && s) setRefresh(s);
      } catch (_) {}
    };
    const id = setInterval(poll, 1500);
    poll(); // immediate read
    return ()=> { cancelled = true; clearInterval(id); };
  }, [apiBase]);

  // resolve Top Item IDs -> names
  useEffect(()=>{
    const ids = Array.from(new Set(topItems.map(t => t.item_id).filter(Boolean))) as string[];
    if (!ids.length) { setItemNameMap({}); return; }
    fetch(`${apiBase}/items/by-ids?ids=${encodeURIComponent(ids.join(","))}`)
      .then(r=>r.json())
      .then((rows: ItemRow[])=>{
        const m: Record<string,string> = {};
        rows.forEach(r => { m[r.id] = r.display || r.name || r.type || r.id; });
        setItemNameMap(m);
      })
      .catch(()=>{ /* ignore */});
  }, [topItems, apiBase]);

  // reshape usage -> one line per user
  const days = useMemo(()=>Array.from(new Set(usage.map(u=>u.day))).sort(), [usage]);
  const users = useMemo(()=>Array.from(new Set(usage.map(u=>u.user))), [usage]);
  const series = useMemo(()=>days.map(d=>{
    const row:any = { day: d };
    users.forEach(u=>{
      row[u] = usage.filter(x=>x.day===d && x.user===u).reduce((a,b)=>a+b.hours,0);
    });
    return row;
  }), [days, users, usage]);

  // Top items with resolved names
  const topItemsDisplay = useMemo(
    () => topItems.map(x => ({ item: itemNameMap[x.item_id || ""] || x.item_id || "Unknown", hours: x.hours })),
    [topItems, itemNameMap]
  );

  const startRefresh = async () => {
    try {
      const res = await fetch(`${apiBase}/admin/refresh`, { method:"POST" }).then(r=>r.json());
      if (res?.started || res?.running) setRefresh(prev => ({...prev, running: true}));
      setToast(res?.started ? "Library refresh started" : "Library refresh already running");
    } catch {
      setToast("Failed to start library refresh");
    } finally {
      setTimeout(()=>setToast(null), 2000);
    }
  };

  // re-fetch total users a few times after sync
  const refetchTotalUsers = async (tries=6, delayMs=1000) => {
    for (let i=0; i<tries; i++) {
      try {
        const d = await fetch(`${apiBase}/stats/users/total`).then(r=>r.json());
        setTotalUsers(d.total_users||0);
      } catch (_){}
      await new Promise(r=>setTimeout(r, delayMs));
    }
  };

  const syncUsers = async () => {
    if (syncingUsers) return;
    setSyncingUsers(true);
    try {
      const res = await fetch(`${apiBase}/admin/users/sync`, { method:"POST" }).then(r=>r.json()).catch(()=>null);
      setToast(res?.started ? "User sync started" : "User sync already running");
      refetchTotalUsers();
    } catch {
      setToast("Failed to start user sync");
    } finally {
      setTimeout(()=>setToast(null), 2500);
      setSyncingUsers(false);
    }
  };

  const pct = (n:number)=> Math.max(0, Math.min(100, n||0));

  const control = async (sessionId: string, action: "pause"|"unpause"|"stop"|"message", messageText?: string) => {
    try {
      if (action === "pause" || action === "unpause") {
        await fetch(`${apiBase}/now/sessions/${sessionId}/pause`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ paused: action === "pause" })
        });
      } else if (action === "stop") {
        await fetch(`${apiBase}/now/sessions/${sessionId}/stop`, { method: "POST" });
      } else if (action === "message") {
        await fetch(`${apiBase}/now/sessions/${sessionId}/message`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ text: messageText || "" })
        });
      }
    } catch (_) {
      // ignore in UI for now
    }
  };

  return (
    <div className="min-h-dvh">
      {/* Top nav */}
      <header className="sticky top-0 z-40 backdrop-blur border-b border-white/10 bg-black/20">
        <div className="mx-auto max-w-7xl px-4 py-3 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="size-7 rounded-lg bg-green-500/20 ring-1 ring-green-500/30" />
            <span className="font-semibold tracking-tight">Emby Analytics</span>
          </div>
          <div className="ty-muted">live telemetry & insights</div>
        </div>
      </header>

      {/* Page content */}
      <main className="mx-auto max-w-7xl px-4 py-6 space-y-6">

        {/* Toast */}
        {toast && (
          <div className="fixed top-3 right-3 z-[9999] rounded-lg bg-black/80 text-white px-3 py-2 shadow-lg">
            {toast}
          </div>
        )}

        <h1 className="sr-only">Emby Analytics</h1>

        {/* Controls */}
        <div className="flex flex-wrap items-center gap-3 mt-2">
          <button
            onClick={startRefresh}
            disabled={refresh.running}
            className="px-3 py-1.5 rounded-xl border border-white/10 bg-white/5 hover:bg-white/10 disabled:opacity-50"
          >
            {refresh.running ? "Importing..." : "Refresh Library"}
          </button>

          <button
            onClick={syncUsers}
            disabled={syncingUsers}
            className="px-3 py-1.5 rounded-xl border border-white/10 bg-transparent hover:bg-white/5 disabled:opacity-50"
          >
            {syncingUsers ? "Syncing…" : "Sync Users"}
          </button>

          {refresh.running && (
            <div className="min-w-[260px]">
              <div className="h-2 bg-white/10 rounded-full overflow-hidden">
                <div
                  className="h-full bg-white/60 transition-[width] duration-300"
                  style={{
                    width: (() => {
                      const tot = refresh.total || 0;
                      if (!tot) return "100%"; // indeterminate if total unknown
                      const w = (refresh.imported / Math.max(1, tot)) * 100;
                      return `${Math.max(1, Math.min(100, w))}%`;
                    })(),
                  }}
                />
              </div>
              <div className="text-xs text-white/70 mt-1">
                Imported {refresh.imported}{refresh.total ? ` / ${refresh.total}` : ""} • page {refresh.page}
              </div>
            </div>
          )}
          {refresh.error && <span className="text-red-400">Error: {refresh.error}</span>}
        </div>

        {/* Now Playing */}
        <h2 className="ty-title mt-4">Now Playing</h2>
        <div className="grid gap-3 grid-cols-1 md:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
          {now.length===0 && <div className="text-white/70">Nothing playing.</div>}
          {now.map((s:any,i:number)=>(
            <div key={i} className="card p-3 flex gap-3">
              {s.poster
                ? <img src={s.poster} alt="" width={90} height={135} className="w-[90px] h-[135px] object-cover rounded-xl ring-1 ring-white/10"/>
                : <div className="w-[90px] h-[135px] rounded-xl bg-white/5 ring-1 ring-white/10"/>
              }
              <div className="flex-1 min-w-0">
                <div className="font-semibold truncate" title={s.title}>{s.title || "—"}</div>
                <div className="text-xs text-white/60">{s.user} • {s.app}{s.device ? ` • ${s.device}` : ""}</div>

                <div className="mt-2 space-y-1 text-sm leading-5">
                  <div>
                    <span className="font-medium">Stream:</span>{" "}
                    {s.container ? s.container : "—"}
                    {typeof s.bitrate === "number" && s.bitrate > 0 ? ` (${(s.bitrate/1000000).toFixed(1)} Mbps)` : ""}
                    {" "}→ {s.play_method === "Direct" ? "Direct Play" : "Transcode"}
                    {s.play_method !== "Direct" && (
                      <>
                        <div className="mt-1 text-xs opacity-80">
                          {"\u2192"} {s.stream_detail || `${s.stream_path || "Transcode"} (${(s.bitrate/1000000).toFixed(1)} Mbps)`}
                        </div>
                        {s.trans_reason && (
                          <div className="text-xs opacity-80">
                            {s.trans_reason}
                          </div>
                        )}
                      </>
                    )}
                  </div>

                  <div>
                    <span className="font-medium">Video:</span>{" "}
                    {s.video_detail || s.video || "—"}{" "}
                    → {s.video_method === "Transcode"
                        ? `Transcode (${s.trans_video_to || "—"})`
                        : "Direct Play"}
                  </div>

                  <div>
                    <span className="font-medium">Audio:</span>{" "}
                    {s.audio_detail || s.audio || "—"}{" "}
                    → {s.audio_method === "Transcode"
                        ? `Transcode (${s.trans_audio_to || "—"}${s.trans_audio_bitrate ? ` ${(s.trans_audio_bitrate/1000).toFixed(0)} kbps` : ""})`
                        : "Direct Play"}
                  </div>

                  <div>
                    <span className="font-medium">Sub:</span>{" "}
                    {s.sub_lang || s.sub_codec
                      ? `${s.sub_lang || "Unknown"} - ${s.sub_codec || "Unknown"}`
                      : (s.subs || "None")}
                    {" "}→ Direct
                  </div>
                </div>

                <div className="mt-3 flex flex-wrap gap-2">
                  <button
                    onClick={() => control(s.session_id, "pause")}
                    className="px-2 py-1 rounded-lg border border-white/10 bg-white/5 hover:bg-white/10 text-xs shrink-0"
                  >
                    Pause
                  </button>
                  <button
                    onClick={() => control(s.session_id, "unpause")}
                    className="px-2 py-1 rounded-lg border border-white/10 bg-white/5 hover:bg-white/10 text-xs shrink-0"
                  >
                    Unpause
                  </button>
                  <button
                    onClick={() => control(s.session_id, "stop")}
                    className="px-2 py-1 rounded-lg border border-white/10 bg-white/5 hover:bg-white/10 text-xs shrink-0"
                  >
                    Stop
                  </button>
                  <button
                    onClick={() => {
                      const text = prompt("Message to client:");
                      if (text) control(s.session_id, "message", text);
                    }}
                    className="px-2 py-1 rounded-lg border border-white/10 bg-white/5 hover:bg-white/10 text-xs shrink-0"
                  >
                    Message
                  </button>
                </div>

                <div className="mt-2 h-2 bg-white/10 rounded-full">
                  <div className="h-full bg-white/60 rounded-full" style={{ width:`${pct(s.progress_pct)}%` }}/>
                    {/* transcode progress (red) */}
                    {s.play_method !== "Direct" && (
                      <div
                        className="absolute left-0 top-0 h-2 rounded"
                        style={{ background: "#ef4444", width: `${Math.min(100, Math.max(0, s.trans_pct || 0))}%`, opacity: 0.8 }}
                        title="Transcode progress"
                      />
                    )}
                </div>
              </div>
            </div>
          ))}
        </div>

        {/* Usage line chart */}
        <h2 className="ty-title mt-6">Usage (last 14 days)</h2>
        <div className="card p-4 h-80">
          <ResponsiveContainer>
            <LineChart data={series}>
              <XAxis dataKey="day" />
              <YAxis tickFormatter={fmtAxisTime} />
              <Tooltip formatter={(v: any, name: string) => [fmtTooltipTime(Number(v)), name]} />
              <Legend />
              {users.map(u => <Line key={u} type="monotone" dataKey={u} dot={false} />)}
            </LineChart>
          </ResponsiveContainer>
        </div>

        {/* Quick stats grid */}
        <div className="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-4">
          {/* Media Qualities */}
          <div className="card p-4">
            <div className="ty-h3 text-center">Media Qualities</div>
            <table className="table-dark mt-2">
              <thead>
                <tr>
                  <th> </th>
                  <th className="num">Movies</th>
                  <th className="num">Episodes</th>
                </tr>
              </thead>
              <tbody>
                {["4K","1080p","720p","SD","Unknown"].map(b=>(
                  <tr key={b}>
                    <td>{b}</td>
                    <td className="num">{fmtInt(qualities.buckets?.[b]?.Movie || 0)}</td>
                    <td className="num">{fmtInt(qualities.buckets?.[b]?.Episode || 0)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Media Codecs */}
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
                {(codecs.codecs ? Object.keys(codecs.codecs) : []).map((c:string)=>(
                  <tr key={c}>
                    <td>{c}</td>
                    <td className="num">{fmtInt(codecs.codecs[c]?.Movie || 0)}</td>
                    <td className="num">{fmtInt(codecs.codecs[c]?.Episode || 0)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Most Active Users (single) */}
          <div className="card p-4">
            <div className="ty-h3 text-center">Most Active Users</div>
            {activeUsers.length === 0 ? <div className="text-center text-white/60 mt-2">—</div> : (
              <div className="grid grid-cols-[1fr_auto_auto_auto] gap-2 mt-3 items-center text-sm">
                <div>{activeUsers[0].user}</div>
                <div><b>Days</b><br/>{activeUsers[0].days}</div>
                <div><b>Hours</b><br/>{activeUsers[0].hours}</div>
                <div><b>Minutes</b><br/>{activeUsers[0].minutes}</div>
              </div>
            )}
          </div>

          {/* Total Users */}
          <div className="card p-4 flex flex-col items-center justify-center">
            <div className="font-semibold">Total Users</div>
            <div className="text-3xl font-extrabold mt-1">{fmtInt(totalUsers)}</div>
          </div>
        </div>

        {/* Top users / items */}
        <div className="grid gap-6 grid-cols-1 md:grid-cols-2">
          <div className="card p-4">
            <div className="ty-h3 mb-2">Top users (14d)</div>
            <div className="h-64">
              <ResponsiveContainer>
                <BarChart data={topUsers.map(x=>({ user: x.user, hours: x.hours }))}>
                  <XAxis dataKey="user" />
                  <YAxis tickFormatter={fmtAxisTime} />
                  <Tooltip formatter={(v)=>[fmtTooltipTime(v as number), "time"]} />
                  <Bar dataKey="hours" />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </div>
          <div className="card p-4">
            <div className="ty-h3 mb-2">Top items (14d)</div>
            <div className="h-64">
              <ResponsiveContainer>
                <BarChart data={topItemsDisplay}>
                  <XAxis dataKey="item" />
                  <YAxis tickFormatter={fmtAxisTime} />
                  <Tooltip formatter={(v)=>[fmtTooltipTime(v as number), "time"]} />
                  <Bar dataKey="hours" />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </div>
        </div>

        {/* Library overview */}
        <h2 className="ty-title mt-6">Library overview</h2>
        <pre className="card p-4 text-sm whitespace-pre-wrap overflow-auto">
          {JSON.stringify(overview, null, 2)}
        </pre>
      </main>
    </div>
  );
}
