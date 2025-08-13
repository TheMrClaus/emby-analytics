from fastapi import APIRouter
import httpx
from ..db import db
from ..config import EMBY_BASE_URL, EMBY_API_KEY

router = APIRouter(prefix="/items", tags=["items"])

@router.get("/by-ids")
async def items_by_ids(ids: str):
    id_list = [x for x in (ids or "").split(",") if x]
    if not id_list:
        return []

    # First: pull what we already have in SQLite
    placeholders = ",".join("?" for _ in id_list)
    conn = await db()
    try:
        cur = await conn.execute(
            f"select id, name, type from library_item where id in ({placeholders})",
            id_list,
        )
        rows = await cur.fetchall()
        base = {r["id"]: {"id": r["id"], "name": r["name"], "type": r["type"]} for r in rows}
    finally:
        await conn.close()

    # If we have Episodes, fetch full details from Emby to build a nice display name
    episode_ids = [iid for iid, rec in base.items() if (rec.get("type") == "Episode")]
    if episode_ids:
        async with httpx.AsyncClient(timeout=20) as s:
            r = await s.get(
                f"{EMBY_BASE_URL}/emby/Items",
                params={
                    "api_key": EMBY_API_KEY,
                    "Ids": ",".join(episode_ids),
                    "Fields": "SeriesName,ParentIndexNumber,IndexNumber,Name",
                },
            )
            r.raise_for_status()
            items = (r.json() or {}).get("Items") or []

        for it in items:
            iid = it.get("Id")
            series = it.get("SeriesName") or ""
            season = it.get("ParentIndexNumber")
            ep = it.get("IndexNumber")
            epname = it.get("Name") or (base.get(iid) or {}).get("name") or "—"

            # Sxx:Exx (zero-padded)
            if isinstance(season, int) and isinstance(ep, int):
                se = f"S{season:02d}:E{ep:02d}"
                display = f"{series} - {se} - {epname}" if series else f"{se} - {epname}"
            else:
                display = f"{series} - {epname}" if series else epname

            if iid in base:
                base[iid].update({
                    "display": display,
                    "series": series,
                    "season": season,
                    "episode": ep,
                })

    # Default display = name for non-episodes / anything missing
    out = []
    for rec in base.values():
        if "display" not in rec:
            rec["display"] = rec.get("name") or rec.get("type") or rec.get("id")
        out.append(rec)
    return out
