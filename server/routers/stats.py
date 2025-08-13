from fastapi import APIRouter
import re
from ..db import db

router = APIRouter(prefix="/stats", tags=["stats"])

def _window_to_sql(window: str) -> str:
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
        cur = await conn.execute("select type, count(*) c from library_item group by 1")
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

@router.get("/codecs")
async def stats_codecs(limit: int = 10):
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

@router.get("/active-users")
async def stats_active_users(window: str = "30d", limit: int = 5):
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
        out = []
        for r in rows:
            mins = float(r["minutes"])
            days = int(mins // (60*24)); mins -= days * 60*24
            hours = int(mins // 60);     mins -= hours * 60
            out.append({"user": r["user"], "days": days, "hours": hours, "minutes": int(mins), "total_minutes": int(float(r["minutes"]))})
        return out
    finally:
        await conn.close()

@router.get("/users/total")
async def stats_total_users():
    conn = await db()
    try:
        cur = await conn.execute("select count(*) as c from emby_user")
        r = await cur.fetchone()
        return {"total_users": r["c"]}
    finally:
        await conn.close()
