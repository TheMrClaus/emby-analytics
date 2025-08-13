from fastapi import APIRouter
from ..db import db

router = APIRouter(prefix="/items", tags=["items"])

@router.get("/by-ids")
async def items_by_ids(ids: str):
    id_list = [x for x in (ids or "").split(",") if x]
    if not id_list:
        return []
    placeholders = ",".join("?" for _ in id_list)
    conn = await db()
    try:
        cur = await conn.execute(
            f"select id, name, type from library_item where id in ({placeholders})",
            id_list,
        )
        rows = await cur.fetchall()
        return [{"id": r["id"], "name": r["name"], "type": r["type"]} for r in rows]
    finally:
        await conn.close()
