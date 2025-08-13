import asyncio
import json
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

# --- Sanitization helpers ---

def safe_value(val):
    """Convert nested objects to strings to prevent React rendering errors."""
    if isinstance(val, (dict, list)):
        return json.dumps(val)
    return val

def sanitize_sessions(data):
    """Recursively sanitize sessions data so it’s JSON serializable and frontend-safe."""
    if isinstance(data, list):
        return [sanitize_sessions(x) for x in data]
    elif isinstance(data, dict):
        return {k: sanitize_sessions(v) if isinstance(v, (dict, list)) else safe_value(v)
                for k, v in data.items()}
    else:
        return safe_value(data)

# Helper used by collector loop
async def publish_now(sessions):
    safe_sessions = sanitize_sessions(sessions)
    for q in list(_subscribers):
        if q.qsize() < 10:
            await q.put(safe_sessions)