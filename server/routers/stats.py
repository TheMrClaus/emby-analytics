from fastapi import APIRouter
import re
from ..db import db

router = APIRouter(prefix="/stats", tags=["stats"])

def _window_to_sql(window: str) -> str | None:
    if (window or "").lower().strip() in ("all", "lifetime", "ever"):
        return None
    m = re.fullmatch(r'(\d+)\s*([dhwm])', (window or '').lower().strip())
    if not m:
        return "-30 days"
    n, unit = m.groups()
    units = {'d': 'days', 'h': 'hours', 'w': 'weeks', 'm': 'months'}
    return f"-{n} {units[unit]}"

@router.get("/overview")
async def stats_overview():
    conn = await db()
    try:
        cur = await conn.execute("SELECT type, count(*) c FROM library_item GROUP BY 1")
        types = {r["type"]: r["c"] for r in await cur.fetchall()}
        return {"types": types}
    finally:
        await conn.close()

@router.get("/usage")
async def stats_usage(days: int = 30):
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
                    CAST((julianday(ts) - julianday(prev_ts)) * 86400000 AS INTEGER)
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

@router.get("/top/users")
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

@router.get("/top/items")
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

@router.get("/qualities")
async def stats_qualities():
    conn = await db()
    try:
        cur = await conn.execute("""
            WITH buckets AS (
              SELECT
                CASE
                  WHEN video_height IS NULL OR video_height = 0 THEN 'Unknown'
                  WHEN video_height >= 2160 THEN '4K'
                  WHEN video_height >= 1080 THEN '1080p'
                  WHEN video_height >= 720  THEN '720p'
                  ELSE 'SD'
                END AS bucket,
                COALESCE(type,'Unknown') AS t,
                COUNT(*) AS c
              FROM library_item
              GROUP BY 1,2
            )
            SELECT bucket, t AS type, c FROM buckets;
        """)
        rows = await cur.fetchall()
        out = {}
        for r in rows:
            out.setdefault(r["bucket"], {}).setdefault(r["type"], 0)
            out[r["bucket"]][r["type"]] += r["c"]
        return {"buckets": out}
    finally:
        await conn.close()

@router.get("/codecs")
async def stats_codecs(limit: int = 10):
    conn = await db()
    try:
        cur = await conn.execute("""
            WITH norm AS (
              SELECT LOWER(COALESCE(NULLIF(video_codec,''),'unknown')) AS codec,
                     COALESCE(type,'Unknown') AS t
              FROM library_item
            ),
            agg AS (
              SELECT codec, t AS type, COUNT(*) c
              FROM norm
              GROUP BY 1,2
            )
            SELECT codec, type, c
            FROM agg
            ORDER BY (SELECT SUM(c) FROM agg a2 WHERE a2.codec=agg.codec) DESC, codec ASC, type ASC
            LIMIT ?;
        """, (limit * 2,))
        rows = await cur.fetchall()
        out = {}
        for r in rows:
            out.setdefault(r["codec"], {}).setdefault(r["type"], 0)
            out[r["codec"]][r["type"]] += r["c"]
        return {"codecs": out}
    finally:
        await conn.close()

def _format_active_users(rows, limit):
    # Ensure at least `limit` entries so frontend won't crash
    out = []
    for r in rows:
        ms = int(r["total_ms"] or 0)
        mins = ms // 60000
        days = mins // (60 * 24)
        mins -= days * 60 * 24
        hours = mins // 60
        mins -= hours * 60
        out.append({
            "user": r["user"] or "Unknown",
            "days": days,
            "hours": hours,
            "minutes": int(mins)
        })
    # Pad if fewer than limit
    while len(out) < limit:
        out.append({"user": "Unknown", "days": 0, "hours": 0, "minutes": 0})
    return out

async def _get_active_users(limit: int):
    conn = await db()
    try:
        cur = await conn.execute("""
            SELECT COALESCE(u.name, lw.user_id) AS user,
                   lw.total_ms
            FROM lifetime_watch lw
            LEFT JOIN emby_user u ON u.id = lw.user_id
            ORDER BY lw.total_ms DESC
            LIMIT ?
        """, (limit,))
        rows = await cur.fetchall()
        return _format_active_users(rows, limit)
    finally:
        await conn.close()

@router.get("/active-users")
async def stats_active_users(limit: int = 5):
    return await _get_active_users(limit)

@router.get("/active-users-lifetime")
async def stats_active_users_lifetime(limit: int = 5):
    return await _get_active_users(limit)


        
@router.get("/active-users-lifetime")
async def stats_active_users_lifetime(limit: int = 5):
    """
    Return most active users of all time using lifetime_watch table.
    This matches the frontend's expected structure.
    """
    conn = await db()
    try:
        cur = await conn.execute("""
            SELECT COALESCE(u.name, lw.user_id) AS user,
                   lw.total_ms
            FROM lifetime_watch lw
            LEFT JOIN emby_user u ON u.id = lw.user_id
            ORDER BY lw.total_ms DESC
            LIMIT ?
        """, (limit,))
        rows = await cur.fetchall()

        out = []
        for r in rows:
            ms = int(r["total_ms"] or 0)
            mins = ms // 60000
            days = mins // (60 * 24)
            mins -= days * 60 * 24
            hours = mins // 60
            mins -= hours * 60
            out.append({
                "user": r["user"],
                "days": days,
                "hours": hours,
                "minutes": int(mins)
            })
        return out
    finally:
        await conn.close()

@router.get("/users/total")
async def stats_total_users():
    conn = await db()
    try:
        cur = await conn.execute("SELECT count(*) AS c FROM emby_user")
        r = await cur.fetchone()
        return {"total_users": r["c"]}
    finally:
        await conn.close()
