from fastapi import APIRouter, Response
import httpx
from ..config import EMBY_BASE, EMBY_KEY

router = APIRouter(tags=["images"])

@router.get("/img/primary/{item_id}")
async def img_primary(item_id: str):
    url = f"{EMBY_BASE}/emby/Items/{item_id}/Images/Primary"
    async with httpx.AsyncClient(timeout=20) as s:
        r = await s.get(url, params={"api_key": EMBY_KEY, "quality": 90, "maxWidth": 300})
        r.raise_for_status()
        return Response(content=r.content, media_type=r.headers.get("Content-Type", "image/jpeg"))

@router.get("/img/backdrop/{item_id}")
async def img_backdrop(item_id: str):
    url = f"{EMBY_BASE}/emby/Items/{item_id}/Images/Backdrop"
    async with httpx.AsyncClient(timeout=20) as s:
        r = await s.get(url, params={"api_key": EMBY_KEY, "quality": 90, "maxWidth": 1280})
        r.raise_for_status()
        return Response(content=r.content, media_type=r.headers.get("Content-Type", "image/jpeg"))
