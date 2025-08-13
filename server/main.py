from pathlib import Path
import asyncio
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles

from .config import KEEPALIVE_SEC
from .db import ensure_schema
from .tasks import (
    collector_loop,
    users_sync_loop,
    sync_users_from_emby,
    backfill_lifetime_watch
)
from .routers import stats, admin, images, now, items

app = FastAPI(title="Emby Analytics")

# Allow all origins for the API
app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_methods=["*"],
    allow_headers=["*"]
)

# Register API routers
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
    # 1. Ensure DB schema & indexes exist
    await ensure_schema()

    # 2. Populate the users table from Emby
    await sync_users_from_emby()

    # 3. Populate all-time totals from Emby history (lifetime_watch)
    await backfill_lifetime_watch()

    # 4. Start periodic background tasks
    asyncio.create_task(users_sync_loop(24))  # refresh users every 24h

    # 5. Start live play-event collector
    from .routers.now import publish_now
    asyncio.create_task(collector_loop(publish_now))
