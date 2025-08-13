import asyncio, httpx
from fastapi import APIRouter, status
from fastapi.responses import JSONResponse
from ..db import db
from ..config import EMBY_BASE, EMBY_KEY
from ..tasks import sync_users_from_emby

router = APIRouter(prefix="/admin", tags=["admin"])

# Library refresh worker (now exposes "total" for progress UI)
async def _refresh_worker(state: dict):
    state.update({"running": True, "page": 0, "imported": 0, "total": 0, "error": None})
    conn = await db()
    try:
        async with httpx.AsyncClient(timeout=30) as s:
            page, total_imported, per_page = 0, 0, 200
            while True:
                state["page"] = page
                r = await s.get(
                    f"{EMBY_BASE}/emby/Items",
                    params={
                        "api_key": EMBY_KEY,
                        "IncludeItemTypes": "Movie,Series,Episode",
                        "Recursive": "true",
                        "StartIndex": page * per_page,
                        "Limit": per_page,
                        "Fields": "DateCreated,MediaStreams,MediaSources,ProductionYear",
                    },
                )
                r.raise_for_status()
                j = r.json() or {}
                items = j.get("Items") or []

                # capture total once if Emby provides it
                if not state.get("total"):
                    try:
                        state["total"] = int(j.get("TotalRecordCount") or 0)
                    except Exception:
                        state["total"] = 0

                if not items:
                    break

                for i in items:
                    # consider video tracks on the default source AND all MediaSources
                    candidates = []
                    for st in i.get("MediaStreams") or []:
                        if (st.get("Type") or "").lower() == "video":
                            candidates.append({
                                "h": st.get("Height"),
                                "codec": (st.get("Codec") or "").lower() or None,
                            })
                    for src in i.get("MediaSources") or []:
                        for st in src.get("MediaStreams") or []:
                            if (st.get("Type") or "").lower() == "video":
                                candidates.append({
                                    "h": st.get("Height"),
                                    "codec": (st.get("Codec") or "").lower() or None,
                                })

                    # pick the highest resolution we found (best match to Emby's display)
                    best = max(
                        (c for c in candidates if isinstance(c.get("h"), int) and c["h"] > 0),
                        key=lambda c: c["h"],
                        default=None,
                    )
                    height = best["h"] if best else None
                    codec = best["codec"] if best else None
                    await conn.execute(
                        """
                        insert or replace into library_item
                          (id, type, name, added_at, video_codec, video_height)
                        values (?,?,?,?,?,?)
                        """,
                        (
                            i["Id"], i.get("Type"), i.get("Name"),
                            (i.get("DateCreated") or "")[:19], codec, height,
                        ),
                    )

                await conn.commit()
                total_imported += len(items)
                state["imported"] = total_imported
                page += 1

        state["running"] = False
    except Exception as e:
        state.update({"running": False, "error": str(e)})
    finally:
        await conn.close()

_refresh_state = {"running": False, "page": 0, "imported": 0, "total": 0, "error": None}

@router.post("/refresh")
async def admin_refresh():
    if _refresh_state["running"]:
        return {
            "started": False,
            "running": True,
            "imported": _refresh_state["imported"],
            "total": _refresh_state["total"],
            "page": _refresh_state["page"],
        }
    asyncio.create_task(_refresh_worker(_refresh_state))
    return {"started": True}

@router.get("/refresh/status")
async def admin_refresh_status():
    return _refresh_state

@router.post("/refresh/full")
async def admin_refresh_full():
    # wipe first, then start refresh
    conn = await db()
    try:
        await conn.execute("delete from library_item")
        await conn.commit()
    finally:
        await conn.close()
    asyncio.create_task(_refresh_worker(_refresh_state))
    return {"started": True, "full": True}

# Health endpoints
@router.get("/health")
async def health():
    from datetime import datetime
    return {"ok": True, "time": datetime.utcnow().isoformat()}

@router.get("/health/schema")
async def health_schema():
    from ..db import ensure_schema, db
    await ensure_schema()
    conn = await db()
    try:
        ver = await (await conn.execute("pragma user_version")).fetchone()
        objs = await (await conn.execute(
            "select name, type from sqlite_master where type in ('table','view') order by 2,1"
        )).fetchall()
        return {
            "ok": True,
            "user_version": int(ver[0] or 0),
            "objects": [{"name": r[0], "type": r[1]} for r in objs],
        }
    finally:
        await conn.close()


@router.get("/health/emby")
async def health_emby():
    try:
        async with httpx.AsyncClient(timeout=5) as s:
            r = await s.get(f"{EMBY_BASE}/emby/System/Info", params={"api_key": EMBY_KEY})
            r.raise_for_status()
            j = r.json()
            return {"ok": True, "server_name": j.get("ServerName"), "version": j.get("Version")}
    except Exception as e:
        return JSONResponse(status_code=502, content={"ok": False, "error": str(e)})

# Manual user sync trigger
@router.post("/users/sync", status_code=status.HTTP_202_ACCEPTED)
async def admin_users_sync():
    asyncio.create_task(sync_users_from_emby())
    return {"started": True}
