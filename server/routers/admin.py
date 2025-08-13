import asyncio, httpx
from fastapi import APIRouter, status
from fastapi.responses import JSONResponse
from ..db import db
from ..config import EMBY_BASE, EMBY_KEY
from ..tasks import sync_users_from_emby

router = APIRouter(prefix="/admin", tags=["admin"])

# Library refresh worker (unchanged behavior)
async def _refresh_worker(state: dict):
    state.update({"running": True, "page": 0, "imported": 0, "error": None})
    conn = await db()
    try:
        async with httpx.AsyncClient(timeout=30) as s:
            page, total, per_page = 0, 0, 200
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
                        "Fields": "DateCreated,MediaStreams,ProductionYear",
                    },
                )
                r.raise_for_status()
                items = (r.json() or {}).get("Items") or []
                if not items:
                    break

                for i in items:
                    streams = i.get("MediaStreams") or []
                    v = next((st for st in streams if (st.get("Type") or "").lower() == "video"), {})
                    codec = (v.get("Codec") or "").lower() or None
                    height = v.get("Height") or None
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
                total += len(items)
                state["imported"] = total
                page += 1

        state["running"] = False
    except Exception as e:
        state.update({"running": False, "error": str(e)})
    finally:
        await conn.close()

_refresh_state = {"running": False, "page": 0, "imported": 0, "error": None}

@router.post("/refresh")
async def admin_refresh():
    if _refresh_state["running"]:
        return {"started": False, "running": True, "imported": _refresh_state["imported"], "page": _refresh_state["page"]}
    asyncio.create_task(_refresh_worker(_refresh_state))
    return {"started": True}

@router.get("/refresh/status")
async def admin_refresh_status():
    return _refresh_state

# Health endpoints
@router.get("/health")
async def health():
    from datetime import datetime
    return {"ok": True, "time": datetime.utcnow().isoformat()}

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

# Manual user sync trigger (optional but handy)
@router.post("/users/sync", status_code=status.HTTP_202_ACCEPTED)
async def admin_users_sync():
    asyncio.create_task(sync_users_from_emby())
    return {"started": True}
