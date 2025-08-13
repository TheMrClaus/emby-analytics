# server/db.py
import aiosqlite
from .config import DB_PATH

async def db():
    conn = await aiosqlite.connect(DB_PATH)
    conn.row_factory = aiosqlite.Row
    return conn

async def ensure_schema():
    conn = await db()
    try:
        # Base tables / views / indexes (safe to run repeatedly)
        await conn.executescript("""
create table if not exists emby_user(
  id   text primary key,
  name text
);

create table if not exists library_item(
  id        text primary key,
  type      text,
  name      text,
  added_at  text
);

create table if not exists play_event(
  id           integer primary key autoincrement,
  emby_user_id text,
  item_id      text,
  event_ts     text,
  event_type   text,
  position_ms  integer,
  transcode    integer
);

create table if not exists lifetime_watch(
  user_id    text primary key,
  total_ms   integer not null default 0,
  computed_at text,
  updated_at  text
);

create view if not exists daily_watch as
select substr(event_ts,1,10) as day,
       emby_user_id,
       sum(coalesce(position_ms,0))/3600000.0 as hours
from play_event
group by 1,2;

create index if not exists idx_play_event_ts on play_event(event_ts);
create index if not exists idx_play_user_ts  on play_event(emby_user_id, event_ts);
create index if not exists idx_play_item_ts  on play_event(item_id, event_ts);
create index if not exists idx_library_type  on library_item(type);
""")

        # Add new columns to library_item if missing
        cur = await conn.execute("PRAGMA table_info(library_item)")
        cols = {row["name"] for row in await cur.fetchall()}
        if "video_codec" not in cols:
            await conn.execute("alter table library_item add column video_codec text")
        if "video_height" not in cols:
            await conn.execute("alter table library_item add column video_height integer")

        await conn.executescript("""
create index if not exists idx_library_codec   on library_item(video_codec);
create index if not exists idx_library_height  on library_item(video_height);
""")

        # Add new columns to lifetime_watch if missing
        cur = await conn.execute("PRAGMA table_info(lifetime_watch)")
        cols = {row["name"] for row in await cur.fetchall()}
        if "updated_at" not in cols:
            await conn.execute("alter table lifetime_watch add column updated_at text")

        await conn.commit()

    finally:
        await conn.close()
