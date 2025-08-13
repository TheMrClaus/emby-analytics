# server/tasks.py
import asyncio
import httpx
from datetime import datetime
from server.config import EMBY_BASE_URL, EMBY_API_KEY, KEEPALIVE_SEC
from server.db import db
import logging

log = logging.getLogger(__name__)

# -----------------------------------------------------------------------------
# Sync users from Emby API
# -----------------------------------------------------------------------------
async def sync_users_from_emby():
    """
    Pull the list of users from Emby and store/update them in the emby_user table.
    """
    url = f"{EMBY_BASE_URL}/Users?api_key={EMBY_API_KEY}"
    async with httpx.AsyncClient(timeout=30) as client:
        resp = await client.get(url)
        resp.raise_for_status()
        users = resp.json()

    conn = await db()
    try:
        for user in users:
            await conn.execute(
                "INSERT OR REPLACE INTO emby_user (id, name) VALUES (?, ?)",
                (user["Id"], user["Name"]),
            )
        await conn.commit()
    finally:
        await conn.close()

# -----------------------------------------------------------------------------
# Backfill lifetime watch time
# -----------------------------------------------------------------------------
async def backfill_lifetime_watch():
    """
    Pull *all-time* watch totals per user directly from Emby (Movies+Episodes with IsPlayed=true)
    and store them in lifetime_watch. This makes the app correct on first run.
    """
    log.info("Backfilling lifetime totals from Emby history...")
    async with httpx.AsyncClient(timeout=60.0) as client:
        # users
        ur = await client.get(f"{EMBY_BASE_URL}/Users", params={"api_key": EMBY_API_KEY})
        ur.raise_for_status()
        users = ur.json() or []

        conn = await db()
        try:
            # clear + refill to keep it simple and consistent
            await conn.execute("DELETE FROM lifetime_watch")

            for u in users:
                uid = u.get("Id")
                if not uid:
                    continue

                total_ms = 0
                start = 0
                page = 200

                while True:
                    r = await client.get(
                        f"{EMBY_BASE_URL}/Users/{uid}/Items",
                        params={
                            "api_key": EMBY_API_KEY,
                            "IncludeItemTypes": "Movie,Episode",
                            "Recursive": "true",
                            "IsPlayed": "true",
                            "StartIndex": start,
                            "Limit": page,
                            "Fields": "RunTimeTicks,UserData"
                        },
                    )
                    r.raise_for_status()
                    j = r.json() or {}
                    items = j.get("Items") or []
                    if not items:
                        break

                    for it in items:
                        ticks = int(it.get("RunTimeTicks") or 0)
                        plays = int(((it.get("UserData") or {}).get("PlayCount")) or 1)
                        if ticks > 0 and plays > 0:
                            total_ms += (ticks // 10_000) * plays

                    start += len(items)

                await conn.execute(
                    "INSERT OR REPLACE INTO lifetime_watch (user_id, total_ms, updated_at) VALUES (?,?,datetime('now'))",
                    (uid, int(total_ms)),
                )

            await conn.commit()
            log.info("Lifetime totals backfill complete.")
        finally:
            await conn.close()

# -----------------------------------------------------------------------------
# Periodic background tasks
# -----------------------------------------------------------------------------
async def users_sync_loop(hours: int = 24):
    """
    Loop to refresh user list from Emby every X hours.
    """
    while True:
        try:
            await sync_users_from_emby()
            log.info("User sync completed.")
        except Exception as e:
            log.exception(f"User sync failed: {e}")
        await asyncio.sleep(hours * 3600)

def _normalize_sessions(sessions: list[dict]) -> list[dict]:
    out = []
    for s in sessions or []:
        item = s.get("NowPlayingItem") or {}
        ps   = s.get("PlayState") or {}
        tc   = s.get("TranscodingInfo") or {}
        streams = (item.get("MediaStreams") or [])
        video_streams = [st for st in streams if st.get("Type") == "Video"]
        audio_streams = [st for st in streams if st.get("Type") == "Audio"]
        sub_streams   = [st for st in streams if st.get("Type") == "Subtitle"]

        vid = video_streams[0] if video_streams else {}
        aud = audio_streams[0] if audio_streams else {}

        # Build a poster URL if possible
        poster = None
        if item.get("Id"):
            tag = (item.get("ImageTags") or {}).get("Primary", "")
            poster = (
                f"{EMBY_BASE_URL}/Items/{item['Id']}/Images/Primary"
                f"?fillWidth=240&quality=80"
                f"{'&tag='+tag if tag else ''}"
                f"&api_key={EMBY_API_KEY}"
            )

        # Build a human-ish title
        title = item.get("Name") or ""
        if item.get("SeriesName"):
            # e.g. "Series • S02E05 Name"
            ep = item.get("IndexNumber")
            season = item.get("ParentIndexNumber")
            epbits = []
            if season is not None: epbits.append(f"S{int(season):02d}")
            if ep     is not None: epbits.append(f"E{int(ep):02d}")
            epcode = "".join(epbits) or ""
            if title and epcode:
                title = f"{item['SeriesName']} • {epcode} {title}"
            else:
                title = item["SeriesName"]

        out.append({
            "user": s.get("UserName") or s.get("UserId") or "",
            "title": title,
            "poster": poster,
            "method": "Transcode" if tc else "Direct",
            "video": {
                "codec": tc.get("VideoCodec") or vid.get("Codec"),
                "height": vid.get("Height"),
                "bitrate": (tc.get("Bitrate") or vid.get("BitRate")),
            },
            "audio": {
                "codec": tc.get("AudioCodec") or aud.get("Codec"),
                "channels": aud.get("Channels"),
            },
            "subs": {
                "present": bool(sub_streams)
            },
            "positionMs": int((ps.get("PositionTicks") or 0) / 10000),
            "runTimeMs":  int((item.get("RunTimeTicks") or 0) / 10000),
            "itemId": item.get("Id"),
        })
    return out

async def collector_loop(publish_now):
    from server.db import db
    last_positions = {}

    async with httpx.AsyncClient(timeout=10) as client:
        while True:
            try:
                resp = await client.get(f"{EMBY_BASE_URL}/Sessions?api_key={EMBY_API_KEY}")
                resp.raise_for_status()
                raw_sessions = resp.json()

                # 1) push normalized sessions to the UI
                await publish_now(_normalize_sessions(raw_sessions))

                # 2) log play events (unchanged)
                for sess in raw_sessions:
                    user_id = sess.get("UserId")
                    item = (sess.get("NowPlayingItem") or {})
                    item_id = item.get("Id")
                    pos_ms = sess.get("PlayState", {}).get("PositionTicks")
                    pos_ms = int(pos_ms / 10000) if pos_ms else None
                    if user_id and item_id and pos_ms is not None:
                        last_key = (user_id, item_id)
                        if last_positions.get(last_key) != pos_ms:
                            last_positions[last_key] = pos_ms
                            conn = await db()
                            await conn.execute("""
                                INSERT INTO play_event (
                                    emby_user_id, item_id, event_ts, event_type, position_ms, transcode
                                )
                                VALUES (?, ?, datetime('now'), 'playing', ?, 0)
                            """, (user_id, item_id, pos_ms))
                            await conn.commit()
                            await conn.close()
            except Exception as e:
                log.exception(f"Collector loop error: {e}")

            await asyncio.sleep(KEEPALIVE_SEC)
