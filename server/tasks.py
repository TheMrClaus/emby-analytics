import asyncio, httpx
from datetime import datetime
from .config import EMBY_BASE, EMBY_KEY
from .db import db

# --- Helpers (kept private to the collector) ----------------------------

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

# --- Daily user sync from Emby ------------------------------------------

async def sync_users_from_emby():
    async with httpx.AsyncClient(timeout=15) as s:
        r = await s.get(f"{EMBY_BASE}/emby/Users", params={"api_key": EMBY_KEY})
        r.raise_for_status()
        users = r.json() or []

    conn = await db()
    try:
        # upsert current users
        rows = [(u.get("Id"), u.get("Name")) for u in users if u.get("Id")]
        if rows:
            await conn.executemany(
                "insert or replace into emby_user (id, name) values (?, ?)", rows
            )
        # prune users that no longer exist in Emby
        ids = [r[0] for r in rows]
        if ids:
            placeholders = ",".join("?" * len(ids))
            await conn.execute(
                f"delete from emby_user where id not in ({placeholders})", ids
            )
        else:
            # Emby returned no users; treat as authoritative
            await conn.execute("delete from emby_user")
        await conn.commit()
    finally:
        await conn.close()

async def users_sync_loop(interval_hours: int = 24):
    while True:
        try:
            await sync_users_from_emby()
        except Exception as e:
            print("users sync error:", e)
        await asyncio.sleep(interval_hours * 3600)

# --- Live collector loop (polls Emby /Sessions) -------------------------

async def collector_loop(publish_now):
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
                            continue

                        play_state = ses.get("PlayState") or {}
                        now_item = ses.get("NowPlayingItem") or {}
                        item_id = now_item.get("Id")
                        if play_state.get("IsPaused") or not item_id:
                            continue

                        # upsert user
                        try:
                            await conn.execute(
                                "insert or replace into emby_user (id, name) values (?, ?)",
                                (uid, ses.get("UserName")),
                            )
                        except Exception as e:
                            print("user upsert error:", e)

                        # persist play tick
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
                                    (play_state.get("PositionTicks") or 0) // 10_000,
                                    1 if (ses.get("TranscodingInfo") is not None) else 0,
                                ),
                            )
                        except Exception as e:
                            print("event save error:", e)

                        # transcode/direct flags + progress
                        ti = ses.get("TranscodingInfo") or {}
                        reasons = ti.get("TranscodeReasons") or []
                        video_direct = ti.get("IsVideoDirect", True)
                        audio_direct = ti.get("IsAudioDirect", True)
                        subs_transcoding = bool(
                            ti.get("SubtitleDeliveryUrl") or
                            any("Subtitle" in str(r) for r in reasons)
                        )
                        npsi = ses.get("NowPlayingStreamInfo") or {}
                        bitrate_bps = npsi.get("BitRate")
                        rt_ticks = (now_item.get("RunTimeTicks") or 0)
                        pos_ticks = (play_state.get("PositionTicks") or 0)
                        progress_pct = (float(pos_ticks) / rt_ticks * 100) if rt_ticks else 0.0

                        now_list.append({
                            "item_id": item_id,
                            "title": _format_title(now_item),
                            "user": ses.get("UserName"),
                            "device": ses.get("DeviceName"),
                            "app": ses.get("Client"),
                            "play_method": play_state.get("PlayMethod"),
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
