import os, json, asyncio, re, httpx, aiosqlite
from datetime import datetime
from pathlib import Path
from fastapi import FastAPI, Response
from fastapi.responses import StreamingResponse, JSONResponse
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles

# --- Helpers -------------------------------------------------------------

def _format_title(item: dict) -> str:
    t = (item or {}).get("Type")
    if t == "Episode":
        series = item.get("SeriesName") or item.get("Name") or "—"
        season = item.get("ParentIndexNumber")
        ep = item.get("IndexNumber")
        epname = item.get("Name") or "—"
        if isinstance(season, int) and isinstance(ep, int):
            return f"{series} • S{season:02d}E{ep:02d} — {epname}"
        return f"{series} — {epname}"
    name = item.get("Name") or "—"
    year = item.get("ProductionYear")
    return f"{name} ({year})" if isinstance(year, int) else name

def _poster_url(item: dict) -> str | None:
    iid = (item or {}).get("Id")
    return f"/img/primary/{iid}" if iid else None

def _window_to_sql(window: str) -> str:
    m = re.fullmatch(r'(\d+)\s*([dhwm])', (window or '').lower().strip())
    if not m:
        return "-30 days"
    n, unit = m.groups()
    units = {'d': 'days', 'h': 'hours', 'w': 'weeks', 'm': 'months'}
    return f"-{n} {units[unit]}"

# --- Config --------------------------------------------------------------

EMBY_BASE = os.getenv("EMBY_BASE_URL", "http://emby:8096").rstrip("/")
EMBY_KEY  = os.getenv("EMBY_API_KEY", "")
DB_PATH   = os.getenv("SQLITE_PATH", "./emby.db")
KEEPALIVE_SEC = 15

app = FastAPI(title="Emby Analytics")
app.add_middleware(CORSMiddleware, allow_origins=["*"], allow_methods=["*"], allow_headers=["*"])

subscribers: set[asyncio.Queue] = set()
refresh_state = {"running": False, "page": 0, "imported": 0, "error": None}

SCHEMA = """
create table if not exists emby_user(id text primary key, name text);
create table if not exists library_item(id text primary key, type text, name text, added_at text);
create table if not exists play_event(
  id integer primary key autoincrement,
  emby_user_id text, item_id text, event_ts text, event_type text,
  position_ms integer, transcode integer
);

-- views (legacy; kept for simple queries)
create view if not exists daily_watch as
select substr(event_ts,1,10) as day, emby_user_id, sum(coalesce(position_ms,0))/3600000.0 as hours
from play_event group by 1,2;

-- indexes for speed
create index if not exists idx_play_event_ts     on play_event(event_ts);
create index if not exists idx_play_user_ts      on play_event(emby_user_id, event_ts);
create index if not exists idx_play_item_ts      on play_event(item_id, event_ts);
create index if not exists idx_library_type      on library_item(type);

-- add columns if not present (safe to run every start)
alter table library_item add column video_codec text;
alter table library_item add column video_height integer;

create index if not exists idx_library_codec   on library_item(video_codec);
create index if not exists idx_library_height  on library_item(video_height);
"""

# --- DB ------------------------------------------------------------------

async def db():
    conn = await aiosqlite.connect(DB_PATH)
    conn.row_factory = aiosqlite.Row
    return conn

# --- Startup -------------------------------------------------------------

@app.on_event("startup")
async def startup():
    conn = await db()
    await conn.executescript(SCHEMA)
    await conn.close()
    asyncio.create_task(collector_loop())

# --- Health --------------------------------------------------------------

@app.get("/health")
async def health(): return {"ok": True, "time": datetime.utcnow().isoformat()}

@app.get("/health/emby")
async def health_emby():
    try:
        async with httpx.AsyncClient(timeout=5) as s:
            r = await s.get(f"{EMBY_BASE}/emby/System/Info", params={"api_key": EMBY_KEY})
            r.raise_for_status()
            j = r.json()
            return {"ok": True, "server_name": j.get("ServerName"), "version": j.get("Version")}
    except Exception as e:
        return JSONResponse(status_code=502, content={"ok": False, "error": str(e)})

# --- Library refresh (background) ---------------------------------------

async def refresh_worker():
    refresh_state.update({"running": True, "page": 0, "imported": 0, "error": None})
    conn = await db()
    try:
        async with httpx.AsyncClient(timeout=30) as s:
            page, total, per_page = 0, 0, 200
            while True:
                refresh_state["page"] = page
                r = await s.get(
                    f"{EMBY_BASE}/emby/Items",
                    params={
                        "api_key": EMBY_KEY,
                        "IncludeItemTypes": "Movie,Series,Episode",
                        "Recursive": "true",
                        "StartIndex": page * per_page,
                        "Limit": per_page,
                        "Fields": "DateCreated,MediaStreams,ProductionYear",
                    },
                )
                r.raise_for_status()
                items = (r.json() or {}).get("Items") or []
                if not items:
                    break

                for i in items:
                    streams = i.get("MediaStreams") or []
                    v = next((s for s in streams if (s.get("Type") or "").lower() == "video"), {})
                    codec = (v.get("Codec") or "").lower() or None
                    height = v.get("Height") or None

                    await conn.execute(
                        """
                        insert or replace into library_item
                          (id, type, name, added_at, video_codec, video_height)
                        values (?,?,?,?,?,?)
                        """,
                        (
                            i["Id"],
                            i.get("Type"),
                            i.get("Name"),
                            (i.get("DateCreated") or "")[:19],
                            codec,
                            height,
                        ),
                    )

                await conn.commit()
                total += len(items)
                refresh_state["imported"] = total
                page += 1

        refresh_state["running"] = False
    except Exception as e:
        refresh_state.update({"running": False, "error": str(e)})
    finally:
        await conn.close()

@app.post("/admin/refresh")
async def admin_refresh():
    if refresh_state["running"]:
        return {"started": False, "running": True, "imported": refresh_state["imported"], "page": refresh_state["page"]}
    asyncio.create_task(refresh_worker())
    return {"started": True}

@app.get("/admin/refresh/status")
async def admin_refresh_status(): return refresh_state

# --- Stats ---------------------------------------------------------------

@app.get("/stats/overview")
async def stats_overview():
    conn = await db()
    try:
        cur = await conn.execute("select type, count(*) c from library_item group by 1")
        types = {r["type"]: r["c"] for r in await cur.fetchall()}
        return {"types": types}
    finally:
        await conn.close()

@app.get("/stats/usage")
async def stats_usage(days: int = 30):
    """
    Per-day watch hours by user using deltas between snapshots per (user,item),
    clamped by wall time and a per-tick cap.
    """
    cap_ms = 5000
    conn = await db()
    try:
        rows = await conn.execute("""
            WITH ordered AS (
              SELECT
                datetime(event_ts) AS ts,
                emby_user_id AS uid,
                item_id AS iid,
                position_ms AS pos,
                LAG(position_ms) OVER (
                  PARTITION BY emby_user_id, item_id
                  ORDER BY datetime(event_ts)
                ) AS prev_pos,
                LAG(datetime(event_ts)) OVER (
                  PARTITION BY emby_user_id, item_id
                  ORDER BY datetime(event_ts)
                ) AS prev_ts
              FROM play_event
              WHERE emby_user_id IS NOT NULL
                AND datetime(event_ts) >= datetime('now', ?)
            ),
            deltas AS (
              SELECT
                date(ts) AS day,
                uid,
                CAST(
                  MAX(0, MIN(
                    COALESCE(pos - prev_pos, 0),
                    ?,  -- cap per tick
                    CAST((julianday(ts) - julianday(prev_ts)) * 86400000 AS INTEGER) -- wall gap ms
                  )) AS INTEGER
                ) AS d_ms
              FROM ordered
              WHERE prev_pos IS NOT NULL AND prev_ts IS NOT NULL
              GROUP BY ts, uid, iid
            )
            SELECT d.day,
                   COALESCE(u.name, d.uid) AS user,
                   SUM(d.d_ms)/3600000.0 AS hours
            FROM deltas d
            LEFT JOIN emby_user u ON u.id = d.uid
            GROUP BY d.day, COALESCE(u.name, d.uid)
            ORDER BY d.day ASC;
        """, (f'-{days} day', cap_ms))
        data = await rows.fetchall()
        return [{"day": r["day"], "user": r["user"], "hours": float(r["hours"])} for r in data]
    finally:
        await conn.close()

@app.get("/stats/top/users")
async def top_users(window: str = "30d", limit: int = 10):
    cap_ms = 5000
    since = _window_to_sql(window)
    conn = await db()
    try:
        rows = await conn.execute("""
            WITH o AS (
              SELECT datetime(event_ts) ts, emby_user_id uid, item_id iid, position_ms pos,
                     LAG(position_ms) OVER (PARTITION BY emby_user_id,item_id ORDER BY datetime(event_ts)) ppos,
                     LAG(datetime(event_ts)) OVER (PARTITION BY emby_user_id,item_id ORDER BY datetime(event_ts)) pts
              FROM play_event
              WHERE emby_user_id IS NOT NULL
                AND datetime(event_ts) >= datetime('now', ?)
            ), d AS (
              SELECT uid,
                     CAST(MAX(0, MIN(COALESCE(pos-ppos,0), ?, CAST((julianday(ts)-julianday(pts))*86400000 AS INTEGER))) AS INTEGER) d_ms
              FROM o
              WHERE ppos IS NOT NULL AND pts IS NOT NULL
              GROUP BY ts, uid, iid
            )
            SELECT COALESCE(u.name,d.uid) user, SUM(d.d_ms)/3600000.0 hours
            FROM d LEFT JOIN emby_user u ON u.id=d.uid
            GROUP BY COALESCE(u.name,d.uid)
            ORDER BY hours DESC
            LIMIT ?
        """, (since, cap_ms, limit))
        data = await rows.fetchall()
        return [{"user": r["user"], "hours": float(r["hours"])} for r in data]
    finally:
        await conn.close()

@app.get("/stats/top/items")
async def top_items(window: str = "30d", limit: int = 10):
    cap_ms = 5000
    since = _window_to_sql(window)
    conn = await db()
    try:
        rows = await conn.execute("""
            WITH o AS (
              SELECT datetime(event_ts) ts, item_id iid, position_ms pos,
                     LAG(position_ms) OVER (PARTITION BY item_id ORDER BY datetime(event_ts)) ppos,
                     LAG(datetime(event_ts)) OVER (PARTITION BY item_id ORDER BY datetime(event_ts)) pts
              FROM play_event
              WHERE datetime(event_ts) >= datetime('now', ?)
            ), d AS (
              SELECT iid,
                     CAST(MAX(0, MIN(COALESCE(pos-ppos,0), ?, CAST((julianday(ts)-julianday(pts))*86400000 AS INTEGER))) AS INTEGER) d_ms
              FROM o
              WHERE ppos IS NOT NULL AND pts IS NOT NULL
              GROUP BY ts, iid
            )
            SELECT iid AS item_id, SUM(d_ms)/3600000.0 hours
            FROM d
            GROUP BY iid
            ORDER BY hours DESC
            LIMIT ?
        """, (since, cap_ms, limit))
        data = await rows.fetchall()
        return [{"item_id": r["item_id"], "hours": float(r["hours"])} for r in data]
    finally:
        await conn.close()

@app.get("/stats/qualities")
async def stats_qualities():
    """
    Buckets by video_height: 4K(>=2160), 1080p(1080-2159), 720p(720-1079), SD(1-719), Unknown(NULL/0).
    Returns {buckets: {bucket: {Movie: n, Episode: n}}}
    """
    conn = await db()
    try:
        cur = await conn.execute("""
            with buckets as (
              select
                case
                  when video_height is null or video_height = 0 then 'Unknown'
                  when video_height >= 2160 then '4K'
                  when video_height >= 1080 then '1080p'
                  when video_height >= 720  then '720p'
                  else 'SD'
                end as bucket,
                coalesce(type,'Unknown') as t,
                count(*) as c
              from library_item
              group by 1,2
            )
            select bucket, t as type, c from buckets;
        """)
        rows = await cur.fetchall()
        out = {}
        for r in rows:
            out.setdefault(r["bucket"], {}).setdefault(r["type"], 0)
            out[r["bucket"]][r["type"]] += r["c"]
        return {"buckets": out}
    finally:
        await conn.close()

@app.get("/stats/codecs")
async def stats_codecs(limit: int = 10):
    """
    Top codecs by count, split by type. Unknown/empty grouped as 'unknown'.
    """
    conn = await db()
    try:
        cur = await conn.execute("""
            with norm as (
              select lower(coalesce(nullif(video_codec,''),'unknown')) as codec,
                     coalesce(type,'Unknown') as t
              from library_item
            ),
            agg as (
              select codec, t as type, count(*) c
              from norm
              group by 1,2
            )
            select codec, type, c
            from agg
            order by (select sum(c) from agg a2 where a2.codec=agg.codec) desc, codec asc, type asc
            limit ?;
        """, (limit * 2,))
        rows = await cur.fetchall()
        out = {}
        for r in rows:
            out.setdefault(r["codec"], {}).setdefault(r["type"], 0)
            out[r["codec"]][r["type"]] += r["c"]
        return {"codecs": out}
    finally:
        await conn.close()

@app.get("/stats/active-users")
async def stats_active_users(window: str = "30d", limit: int = 5):
    """
    Returns top users by total watch time in the window.
    """
    cap_ms = 5000
    since = _window_to_sql(window)
    conn = await db()
    try:
        cur = await conn.execute("""
            with o as (
              select datetime(event_ts) ts, emby_user_id uid, item_id iid, position_ms pos,
                     lag(position_ms) over (partition by emby_user_id,item_id order by datetime(event_ts)) ppos,
                     lag(datetime(event_ts)) over (partition by emby_user_id,item_id order by datetime(event_ts)) pts
              from play_event
              where emby_user_id is not null
                and datetime(event_ts) >= datetime('now', ?)
            ),
            d as (
              select uid,
                     cast(max(0, min(coalesce(pos-ppos,0), ?, cast((julianday(ts)-julianday(pts))*86400000 as integer))) as integer) d_ms
              from o
              where ppos is not null and pts is not null
              group by ts, uid, iid
            )
            select coalesce(u.name, d.uid) as user,
                   sum(d_ms) / 60000.0 as minutes
            from d left join emby_user u on u.id=d.uid
            group by coalesce(u.name, d.uid)
            order by minutes desc
            limit ?;
        """, (since, cap_ms, limit))
        rows = await cur.fetchall()
        # Return breakdown too (days/hours/min for convenience)
        out = []
        for r in rows:
            mins = float(r["minutes"])
            days = int(mins // (60*24)); mins -= days * 60*24
            hours = int(mins // 60);     mins -= hours * 60
            out.append({"user": r["user"], "days": days, "hours": hours, "minutes": int(mins), "total_minutes": int(float(r["minutes"]))})
        return out
    finally:
        await conn.close()

@app.get("/stats/users/total")
async def stats_total_users():
    conn = await db()
    try:
        cur = await conn.execute("select count(*) as c from emby_user")
        r = await cur.fetchone()
        return {"total_users": r["c"]}
    finally:
        await conn.close()

@app.get("/items/by-ids")
async def items_by_ids(ids: str):
    id_list = [x for x in (ids or "").split(",") if x]
    if not id_list: return []
    placeholders = ",".join("?" for _ in id_list)
    conn = await db()
    try:
        cur = await conn.execute(
            f"select id, name, type from library_item where id in ({placeholders})", id_list
        )
        rows = await cur.fetchall()
        return [{"id": r["id"], "name": r["name"], "type": r["type"]} for r in rows]
    finally:
        await conn.close()

# --- Images (proxy Emby so API key stays server-side) --------------------

@app.get("/img/primary/{item_id}")
async def img_primary(item_id: str):
    url = f"{EMBY_BASE}/emby/Items/{item_id}/Images/Primary"
    async with httpx.AsyncClient(timeout=20) as s:
        r = await s.get(url, params={"api_key": EMBY_KEY, "quality": 90, "maxWidth": 300})
        r.raise_for_status()
        return Response(content=r.content, media_type=r.headers.get("Content-Type", "image/jpeg"))

@app.get("/img/backdrop/{item_id}")
async def img_backdrop(item_id: str):
    url = f"{EMBY_BASE}/emby/Items/{item_id}/Images/Backdrop"
    async with httpx.AsyncClient(timeout=20) as s:
        r = await s.get(url, params={"api_key": EMBY_KEY, "quality": 90, "maxWidth": 1280})
        r.raise_for_status()
        return Response(content=r.content, media_type=r.headers.get("Content-Type", "image/jpeg"))

# --- SSE -----------------------------------------------------------------

@app.get("/now/stream")
async def now_stream():
    async def gen():
        q: asyncio.Queue = asyncio.Queue()
        subscribers.add(q)
        try:
            while True:
                try:
                    data = await asyncio.wait_for(q.get(), timeout=KEEPALIVE_SEC)
                    yield f"data: {json.dumps(data)}\n\n"
                except asyncio.TimeoutError:
                    yield ": keep-alive\n\n"  # comment line keeps connection alive
        finally:
            subscribers.discard(q)
    return StreamingResponse(gen(), media_type="text/event-stream")

async def publish_now(sessions):
    for q in list(subscribers):
        if q.qsize() < 10:
            await q.put(sessions)

# --- Collector -----------------------------------------------------------

async def collector_loop():
    async with httpx.AsyncClient(timeout=10) as s:
        while True:
            try:
                resp = await s.get(f"{EMBY_BASE}/emby/Sessions", params={"api_key": EMBY_KEY})
                resp.raise_for_status()
                payload = resp.json() or []

                conn = await db()
                try:
                    now_list = []

                    for ses in payload:
                        uid = ses.get("UserId")
                        if not uid:
                            continue  # skip anonymous

                        play_state = ses.get("PlayState") or {}
                        now_item = ses.get("NowPlayingItem") or {}
                        item_id = now_item.get("Id")

                        # actively playing, must have an item
                        if play_state.get("IsPaused") or not item_id:
                            continue

                        # upsert user -> name
                        try:
                            await conn.execute(
                                "insert or replace into emby_user (id, name) values (?, ?)",
                                (uid, ses.get("UserName")),
                            )
                        except Exception as e:
                            print("user upsert error:", e)

                        # persist lightweight play update
                        try:
                            await conn.execute(
                                """
                                insert into play_event
                                  (emby_user_id, item_id, event_ts, event_type, position_ms, transcode)
                                values (?,?,?,?,?,?)
                                """,
                                (
                                    uid,
                                    item_id,
                                    datetime.utcnow().isoformat(timespec="seconds"),
                                    "update",
                                    (play_state.get("PositionTicks") or 0) // 10_000,  # ticks → ms
                                    1 if (ses.get("TranscodingInfo") is not None) else 0,
                                ),
                            )
                        except Exception as e:
                            print("event save error:", e)

                        # transcode/direct details
                        ti = ses.get("TranscodingInfo") or {}
                        reasons = ti.get("TranscodeReasons") or []
                        video_direct = ti.get("IsVideoDirect", True)
                        audio_direct = ti.get("IsAudioDirect", True)
                        subs_transcoding = bool(
                            ti.get("SubtitleDeliveryUrl") or
                            any("Subtitle" in str(r) for r in reasons)
                        )

                        # bitrate + progress
                        npsi = ses.get("NowPlayingStreamInfo") or {}
                        bitrate_bps = npsi.get("BitRate")
                        rt_ticks = (now_item.get("RunTimeTicks") or 0)
                        pos_ticks = (play_state.get("PositionTicks") or 0)
                        progress_pct = (float(pos_ticks) / rt_ticks * 100) if rt_ticks else 0.0

                        # build card
                        title = _format_title(now_item)
                        now_list.append({
                            "item_id": item_id,
                            "title": title,
                            "user": ses.get("UserName"),
                            "device": ses.get("DeviceName"),
                            "app": ses.get("Client"),
                            "play_method": play_state.get("PlayMethod"),  # DirectPlay | DirectStream | Transcode
                            "video": "Direct" if video_direct else "Transcode",
                            "audio": "Direct" if audio_direct else "Transcode",
                            "subs": ("Transcode" if subs_transcoding
                                     else ("None" if play_state.get("SubtitleStreamIndex") in (None, -1) else "Direct")),
                            "bitrate": bitrate_bps,
                            "progress_pct": round(max(0.0, min(100.0, progress_pct)), 1),
                            "poster": _poster_url(now_item),
                        })

                    await conn.commit()
                    await publish_now(now_list)

                finally:
                    await conn.close()

            except Exception as e:
                print("collector error:", e)

            await asyncio.sleep(2)

# --- Static UI (serve Next.js export if present) -------------------------

BASE_DIR = Path(__file__).resolve().parents[1]
WEB_DIR = BASE_DIR / "app" / "out"
if WEB_DIR.is_dir():
    app.mount("/", StaticFiles(directory=str(WEB_DIR), html=True), name="web")
