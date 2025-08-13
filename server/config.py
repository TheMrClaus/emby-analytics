import os

EMBY_BASE_URL = os.getenv("EMBY_BASE_URL", "http://emby:8096").rstrip("/")
EMBY_API_KEY  = os.getenv("EMBY_API_KEY", "")
DB_PATH   = os.getenv("SQLITE_PATH", "./emby.db")
KEEPALIVE_SEC = int(os.getenv("KEEPALIVE_SEC", "15"))