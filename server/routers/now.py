import asyncio, json
from fastapi import APIRouter
from fastapi.responses import StreamingResponse
from ..config import KEEPALIVE_SEC

router = APIRouter(prefix="/now", tags=["now"])

_subscribers: set[asyncio.Queue] = set()

@router.get("/stream")
async def now_stream():
    async def gen():
        q: asyncio.Queue = asyncio.Queue()
        _subscribers.add(q)
        try:
            while True:
                try:
                    data = await asyncio.wait_for(q.get(), timeout=KEEPALIVE_SEC)
                    yield f"data: {json.dumps(data)}\n\n"
                except asyncio.TimeoutError:
                    yield ": keep-alive\n\n"
        finally:
            _subscribers.discard(q)
    return StreamingResponse(gen(), media_type="text/event-stream")

# Helper used by collector loop
async def publish_now(sessions):
    for q in list(_subscribers):
        if q.qsize() < 10:
            await q.put(sessions)
