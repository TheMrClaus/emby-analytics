# --- Build the Next.js static UI ---
FROM node:20-bullseye AS frontend-builder
WORKDIR /app
COPY app/package*.json ./
RUN npm ci
COPY app/ ./
RUN npm run build  # will output ./out because of output:'export'

# --- API runtime (serves API + static UI) ---
FROM python:3.12-slim AS api
WORKDIR /app
# System deps (optional but handy)
RUN apt-get update && apt-get install -y --no-install-recommends curl && rm -rf /var/lib/apt/lists/*

# Copy server
COPY server/ ./server/
# Copy exported UI to the path FastAPI expects: repo_root/app/out
RUN mkdir -p /app/app/out
COPY --from=frontend-builder /app/out/ /app/app/out/

# Python deps
RUN pip install --no-cache-dir -r server/requirements.txt

# Env & data
ENV EMBY_BASE_URL=http://emby:8096 \
    SQLITE_PATH=/data/emby.db
EXPOSE 8080

# Create data dir for SQLite
RUN mkdir -p /data
VOLUME ["/data"]

# Run FastAPI (serves UI + API on :8080)
CMD ["uvicorn", "server.main:app", "--host", "0.0.0.0", "--port", "8080"]

