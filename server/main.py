from pathlib import Path
import asyncio
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles

from .config import KEEPALIVE_SEC
from .db import ensure_schema
from .tasks import collector_loop, users_sync_loop, sync_users_from_emby
from .routers import stats, admin, images, now, items

app = FastAPI(title="Emby Analytics")
app.add_middleware(CORSMiddleware, allow_origins=["*"], allow_methods=["*"], allow_headers=["*"])

# Routers
app.include_router(stats.router)
app.include_router(admin.router)
app.include_router(images.router)
app.include_router(now.router)
app.include_router(items.router)

# Serve the exported Next.js UI (if present)
BASE_DIR = Path(__file__).resolve().parents[1]
WEB_DIR = BASE_DIR / "app" / "out"
if WEB_DIR.is_dir():
    app.mount("/", StaticFiles(directory=str(WEB_DIR), html=True), name="web")

@app.on_event("startup")
async def startup():
    # DB schema / indexes
    await ensure_schema()
    # Seed users immediately and refresh daily
    asyncio.create_task(sync_users_from_emby())
    asyncio.create_task(users_sync_loop(24))
    # Start live collector; inject publisher from the now router
    from .routers.now import publish_now
    asyncio.create_task(collector_loop(publish_now))
