'use client';
import { useEffect, useMemo, useState } from "react";
import {
  LineChart, Line, XAxis, YAxis, Tooltip, Legend, ResponsiveContainer,
  BarChart, Bar
} from "recharts";

// Use relative API when UI is served by FastAPI
const API = ""; // was process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080"

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

  // initial fetches
  useEffect(()=>{
    fetch(`${API}/stats/usage?days=14`).then(r=>r.json()).then(setUsage);
    fetch(`${API}/stats/overview`).then(r=>r.json()).then(setOverview);
    fetch(`${API}/stats/top/users?window=14d&limit=5`).then(r=>r.json()).then(setTopUsers);
    fetch(`${API}/stats/top/items?window=14d&limit=5`).then(r=>r.json()).then(setTopItems);
    fetch(`${API}/stats/qualities`).then(r=>r.json()).then(setQualities);
    fetch(`${API}/stats/codecs?limit=8`).then(r=>r.json()).then(setCodecs);
    fetch(`${API}/stats/active-users-lifetime?limit=1`).then(r=>r.json()).then(setActiveUsers);
    fetch(`${API}/stats/users/total`).then(r=>r.json()).then(d=>setTotalUsers(d.total_users||0));

    const es = new EventSource(`${API}/now/stream`);
    es.onmessage = (e)=> setNow(JSON.parse(e.data||"[]"));
    return ()=> es.close();
  },[]);

  // refresh status poll (continuous)
  useEffect(()=>{
    let cancelled = false;
    const poll = async () => {
      try {
        const s = await fetch(`${API}/admin/refresh/status`).then(r=>r.json());
        if (!cancelled && s) setRefresh(s);
      } catch (_) {}
    };
    const id = setInterval(poll, 1500);
    poll(); // immediate read
    return ()=> { cancelled = true; clearInterval(id); };
  },[]);

  // resolve Top Item IDs -> names
  useEffect(()=>{
    const ids = Array.from(new Set(topItems.map(t => t.item_id).filter(Boolean))) as string[];
    if (!ids.length) { setItemNameMap({}); return; }
    fetch(`${API}/items/by-ids?ids=${encodeURIComponent(ids.join(","))}`)
      .then(r=>r.json())
      .then((rows: ItemRow[])=>{
        const m: Record<string,string> = {};
        rows.forEach(r => { m[r.id] = r.display || r.name || r.type || r.id; });
        setItemNameMap(m);
      })
      .catch(()=>{ /* ignore */});
  }, [topItems]);

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
      const res = await fetch(`${API}/admin/refresh`, { method:"POST" }).then(r=>r.json());
      // optimistic: show running immediately if server accepted
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
        const d = await fetch(`${API}/stats/users/total`).then(r=>r.json());
        setTotalUsers(d.total_users||0);
      } catch (_){}
      await new Promise(r=>setTimeout(r, delayMs));
    }
  };

  const syncUsers = async () => {
    if (syncingUsers) return;
    setSyncingUsers(true);
    try {
      const res = await fetch(`${API}/admin/users/sync`, { method:"POST" }).then(r=>r.json()).catch(()=>null);
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

  return (
    <div style={{padding:20, fontFamily:"system-ui, sans-serif"}}>
      {/* Toast */}
      {toast && (
        <div style={{
          position:"fixed", top:14, right:14, background:"#333", color:"#fff",
          padding:"8px 12px", borderRadius:8, boxShadow:"0 2px 12px rgba(0,0,0,.15)", zIndex:9999
        }}>
          {toast}
        </div>
      )}

      <h1>Emby Analytics</h1>

      {/* Controls */}
      <div style={{display:"flex", gap:12, alignItems:"center", flexWrap:"wrap", marginTop:8}}>
        <button onClick={startRefresh} disabled={refresh.running}
          style={{padding:"6px 12px", border:"1px solid #ddd", borderRadius:8, cursor: refresh.running ? "not-allowed":"pointer"}}>
          {refresh.running ? "Importing..." : "Refresh Library"}
        </button>

        <button onClick={syncUsers} disabled={syncingUsers}
          style={{padding:"6px 12px", border:"1px solid #ddd", borderRadius:8, cursor: syncingUsers ? "not-allowed":"pointer"}}>
          {syncingUsers ? "Syncing…" : "Sync Users"}
        </button>

        {refresh.running && (
          <div style={{minWidth:260}}>
            <div style={{height:8, background:"#eee", borderRadius:6, overflow:"hidden"}}>
              <div
                style={{
                  height:8,
                  width: (() => {
                    const tot = refresh.total || 0;
                    if (!tot) return "100%"; // indeterminate width if total unknown
                    const w = (refresh.imported / Math.max(1, tot)) * 100;
                    return `${Math.max(1, Math.min(100, w))}%`;
                  })(),
                  background:"#666",
                  borderRadius:6,
                  transition:"width .3s"
                }}
              />
            </div>
            <div style={{fontSize:12, color:"#555", marginTop:4}}>
              Imported {refresh.imported}{refresh.total ? ` / ${refresh.total}` : ""} • page {refresh.page}
            </div>
          </div>
        )}
        {refresh.error && <span style={{color:"#c00"}}>Error: {refresh.error}</span>}
      </div>

      {/* Now Playing */}
      <h2 style={{marginTop:16}}>Now Playing</h2>
      <div style={{display:"grid", gap:12, gridTemplateColumns:"repeat(auto-fill, minmax(320px, 1fr))"}}>
        {now.length===0 && <div>Nothing playing.</div>}
        {now.map((s:any,i:number)=>(
          <div key={i} style={{border:"1px solid #ddd", borderRadius:12, padding:12, display:"flex", gap:12}}>
            {s.poster
              ? <img src={s.poster} alt="" width={90} height={135} style={{objectFit:"cover", borderRadius:8}}/>
              : <div style={{width:90,height:135,background:"#eee",borderRadius:8}}/>
            }
            <div style={{flex:1, minWidth:0}}>
              <div style={{fontWeight:700, overflow:"hidden", textOverflow:"ellipsis"}} title={s.title}>{s.title || "—"}</div>
              <div style={{color:"#555"}}>{s.user} • {s.app}{s.device ? ` • ${s.device}` : ""}</div>

              <div style={{marginTop:6, display:"flex", gap:6, flexWrap:"wrap", fontSize:12}}>
                <span style={{padding:"2px 6px", border:"1px solid #ddd", borderRadius:8}}>
                  {s.play_method || (s.video==="Transcode" || s.audio==="Transcode" ? "Transcode" : "Direct")}
                </span>
                <span style={{padding:"2px 6px", border:"1px solid #ddd", borderRadius:8}}>Video: {s.video}</span>
                <span style={{padding:"2px 6px", border:"1px solid #ddd", borderRadius:8}}>Audio: {s.audio}</span>
                <span style={{padding:"2px 6px", border:"1px solid #ddd", borderRadius:8}}>Subs: {s.subs}</span>
                {typeof s.bitrate === "number" && (
                  <span style={{padding:"2px 6px", border:"1px solid #ddd", borderRadius:8}}>
                    { (s.bitrate/1000000).toFixed(2) } Mbps
                  </span>
                )}
              </div>

              <div style={{marginTop:8, height:8, background:"#eee", borderRadius:6}}>
                <div style={{height:8, width:`${pct(s.progress_pct)}%`, background:"#666", borderRadius:6}}/>
              </div>
            </div>
          </div>
        ))}
      </div>

      {/* Usage line chart */}
      <h2 style={{marginTop:24}}>Usage (last 14 days)</h2>
      <div style={{width:"100%", height:320}}>
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

      <div style={{display:"grid", gap:16, gridTemplateColumns:"repeat(auto-fit,minmax(300px,1fr))", marginTop:24}}>
        {/* Media Qualities */}
        <div style={{background:"#3aaa35", color:"#fff", borderRadius:10, padding:12}}>
          <div style={{fontWeight:700, textAlign:"center"}}>Media Qualities</div>
          <table style={{width:"100%", marginTop:8}}>
            <thead><tr><th></th><th>Movies</th><th>Episodes</th></tr></thead>
            <tbody>
              {["4K","1080p","720p","SD","Unknown"].map(b=>(
                <tr key={b}>
                  <td>{b}</td>
                  <td style={{textAlign:"right"}}>{qualities.buckets?.[b]?.Movie || 0}</td>
                  <td style={{textAlign:"right"}}>{qualities.buckets?.[b]?.Episode || 0}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Media Codecs */}
        <div style={{background:"#3aaa35", color:"#fff", borderRadius:10, padding:12}}>
          <div style={{fontWeight:700, textAlign:"center"}}>Media Codecs</div>
          <table style={{width:"100%", marginTop:8}}>
            <thead><tr><th></th><th>Movies</th><th>Episodes</th></tr></thead>
            <tbody>
              {(codecs.codecs ? Object.keys(codecs.codecs) : []).map((c:string)=>(
                <tr key={c}>
                  <td>{c}</td>
                  <td style={{textAlign:"right"}}>{codecs.codecs[c]?.Movie || 0}</td>
                  <td style={{textAlign:"right"}}>{codecs.codecs[c]?.Episode || 0}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Most Active Users (single) */}
        <div style={{background:"#3aaa35", color:"#fff", borderRadius:10, padding:12}}>
          <div style={{fontWeight:700, textAlign:"center"}}>Most Active Users</div>
          {activeUsers.length === 0 ? <div>—</div> : (
            <div style={{display:"grid", gridTemplateColumns:"1fr auto auto auto", gap:8, marginTop:8, alignItems:"center"}}>
              <div>{activeUsers[0].user}</div>
              <div><b>Days</b><br/>{activeUsers[0].days}</div>
              <div><b>Hours</b><br/>{activeUsers[0].hours}</div>
              <div><b>Minutes</b><br/>{activeUsers[0].minutes}</div>
            </div>
          )}
        </div>

        {/* Total Users */}
        <div style={{background:"#3aaa35", color:"#fff", borderRadius:10, padding:12, display:"flex", flexDirection:"column", justifyContent:"center", alignItems:"center"}}>
          <div style={{fontWeight:700}}>Total Users</div>
          <div style={{fontSize:28, fontWeight:800}}>{totalUsers}</div>
        </div>
      </div>

      {/* Top users / items */}
      <div style={{display:"grid", gap:24, gridTemplateColumns:"repeat(auto-fit, minmax(320px, 1fr))", marginTop:24}}>
        <div>
          <h3>Top users (14d)</h3>
          <div style={{width:"100%", height:260}}>
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
        <div>
          <h3>Top items (14d)</h3>
          <div style={{width:"100%", height:260}}>
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
      <h2 style={{marginTop:24}}>Library overview</h2>
      <pre style={{whiteSpace:"pre-wrap", background:"#fafafa", border:"1px solid #eee", borderRadius:8, padding:12}}>
        {JSON.stringify(overview, null, 2)}
      </pre>
    </div>
  );
}
