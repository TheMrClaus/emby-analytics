# Emby Analytics

Emby Analytics is a self-hosted dashboard and API service for monitoring and visualizing activity from your [Emby](https://emby.media/) media server. Built with **Go/Fiber v3** backend and **Next.js** frontend, it collects playback statistics, library information, and live "now playing" data, storing them in SQLite for historical analysis.

## Features

- **Real-time "Now Playing" dashboard** via Server-Sent Events (SSE)
- **Usage analytics** (hours watched per user/day)
- **Top users** and **top items** in custom time windows
- **Media quality breakdown** (4K / 1080p / 720p / SD / Unknown)
- **Codec distribution** analysis
- **Most active users** (over configurable time windows)
- **Library overview** by type (Movies, Series, Episodes)
- **Manual library refresh** and user sync from Emby
- **Admin controls** for data management and cleanup
- **Lightweight database** (SQLite) for persistence
- **Modern web UI** with Recharts visualizations

## Architecture

- **Backend**: Go with Fiber v3 framework
- **Frontend**: Next.js (static export served by Go backend)
- **Database**: SQLite
- **Real-time**: Server-Sent Events (SSE)
- **Images**: Proxied through backend from Emby

## Project Structure
```text
.
├── go/                          # Go backend
│   ├── cmd/emby-analytics/      # Main application
│   │   └── main.go              # Entry point & route setup
│   ├── internal/                # Internal packages
│   │   ├── config/              # Configuration management
│   │   ├── db/                  # Database schema & operations
│   │   ├── emby/                # Emby API client
│   │   ├── handlers/            # HTTP route handlers
│   │   │   ├── admin/           # Admin endpoints
│   │   │   ├── health/          # Health checks
│   │   │   ├── images/          # Image proxy
│   │   │   ├── items/           # Library items
│   │   │   ├── now/             # Now playing & SSE
│   │   │   └── stats/           # Statistics endpoints
│   │   └── tasks/               # Background sync tasks
│   ├── go.mod                   # Go module definition
│   └── go.sum                   # Go dependencies
├── app/                         # Next.js frontend
│   ├── src/pages/               # React pages
│   ├── package.json             # Frontend dependencies
│   └── next.config.js           # Next.js config (static export)
├── Dockerfile                   # Multi-stage Docker build
├── .env.example                 # Environment variables template
├── docker-compose-example.yml   # Docker Compose example configuration
└── README.md
```

## Requirements

- **Go 1.25+**
- **Node.js 18+** (for frontend development)
- **Emby Server** with API access
- **Docker** (optional, for containerized deployment)

## Development Setup

### Backend Setup
```bash
# Clone and navigate to project
git clone <your-repo>
cd emby-analytics

# Copy environment template
cp .env.example .env

# Edit .env with your Emby server details
# EMBY_BASE_URL=http://your-emby:8096
# EMBY_API_KEY=your_api_key_here

# Run backend in development
cd go
go mod tidy
go run ./cmd/emby-analytics
```

### Frontend Development
```bash
# Install and run frontend in dev mode
cd app
npm install
npm run dev
```

The frontend dev server will proxy API requests to the Go backend.

## Production Deployment

**Security Note:** Admin endpoints are protected with a token. If `ADMIN_TOKEN` is not set, the server will generate one automatically, persist it under the data directory, and (by default) set an HttpOnly cookie so the UI is authenticated without user action. For internet exposure, still place behind a reverse proxy.


### Option 1: Docker Compose (Recommended)
```bash
# Copy docker-compose-example.yml to docker-compose.yml
cp docker-compose-example.yml docker-compose.yml

# Edit docker-compose.yml with your Emby server details and data path
# EMBY_BASE_URL=http://your-emby:8096
# EMBY_API_KEY=your_api_key_here
# - /path/to/data:/var/lib/emby-analytics

# Build and run with Docker Compose
docker compose up -d --build
```

### Option 2: Manual Build
```bash
# Build frontend
cd app
npm run build

# Build Go binary
cd ../go
CGO_ENABLED=0 go build -o emby-analytics ./cmd/emby-analytics

# Copy built frontend to serve location
cp -r ../app/out ./web

# Run
./emby-analytics
```

## Configuration

Key environment variables (see `.env.example` for complete list):

- `EMBY_BASE_URL`: Your Emby server URL (e.g., `http://emby:8096`)
- `EMBY_API_KEY`: Emby API key (Settings → Advanced → API Keys)
- `SQLITE_PATH`: Database location (default: `/var/lib/emby-analytics/emby.db`)
- `WEB_PATH`: Static UI files path (default: `/app/web`)
- `REFRESH_INTERVAL`: Interval in seconds for background library refresh (default: `60`)
- `REFRESH_CHUNK_SIZE`: Number of items to process per refresh chunk (default: `100`)
- `HISTORY_DAYS`: Number of days of playback history to sync (default: `2`)
- `NOW_POLL_SEC`: Interval in seconds for polling "Now Playing" data (default: `5`)
- `LOG_LEVEL`: Logging level (e.g., `info`, `debug`, `warn`, `error`) (default: `info`)

### Admin Authentication

- Backend: set `ADMIN_TOKEN` to explicitly control the admin token, or omit it and let the server auto-generate and persist one.
- Frontend: to enable admin actions from the UI:
  - Set `NEXT_PUBLIC_ADMIN_TOKEN` during build, or
  - Use the UI prompt (Header → Admin Token) to store a token in the browser.
  - You can also set it manually in `localStorage` under key `emby_admin_token`.

All admin requests include `Authorization: Bearer <token>` automatically when configured.

#### Automatic UI authentication (no user step)

When `ADMIN_TOKEN` is not provided, the server will:

- Generate and save a token to `<data dir>/admin_token` (same directory as the SQLite DB).
- Enable `ADMIN_AUTO_COOKIE` automatically (unless explicitly set), setting an HttpOnly cookie (`admin_token`) so the UI can call admin routes without any manual step.

You can also explicitly set `ADMIN_AUTO_COOKIE=true` with your own `ADMIN_TOKEN` if desired. Only enable this in private/trusted deployments or behind an auth proxy.

## API Explorer (UI)

There is a built‑in API Explorer page that lists every backend endpoint with a description, suggested usage, parameter inputs, and a Run button that executes the call and shows the response.

- Path: `/api-explorer`
- Admin routes automatically include the token from localStorage or `NEXT_PUBLIC_ADMIN_TOKEN`.
- Binary endpoints (images) open in a new tab. The Now WebSocket route is documented but not runnable here.
- If the page returns Not Found after pulling changes, rebuild the UI (Next.js static export) or rebuild the Docker image so the new page is included under `/app/web`.

## API Endpoints

### Statistics
- `GET /stats/overview` - General library overview
- `GET /stats/usage` - Usage analytics by user/day
- `GET /stats/top/users` - Top users by watch time (also `/stats/top-users`)
- `GET /stats/top/items` - Most watched content (also `/stats/top-items`)
- `GET /stats/qualities` - Quality distribution
- `GET /stats/codecs` - Codec statistics
- `GET /stats/active-users` - Active users over lifetime
- `GET /stats/users/total` - Total user count
- `GET /stats/user/:id` - User detail statistics
- `GET /stats/play-methods` - Playback method distribution (also `/stats/playback-methods`)
- `GET /stats/items/by-codec/:codec` - Items by specific codec
- `GET /stats/items/by-quality/:quality` - Items by specific quality

### Now Playing
- `GET /now/snapshot` - Current playback snapshot
- `GET /now/ws` - WebSocket for live updates
- `POST /now/:id/pause` - Pause session
- `POST /now/:id/stop` - Stop session
- `POST /now/:id/message` - Send message to session

### Admin
- `POST /admin/refresh/start` - Start library refresh
- `GET /admin/refresh/status` - Refresh progress
- `POST /admin/reset-all` - Reset all data
- `POST /admin/reset-lifetime` - Reset lifetime watch data
- `POST /admin/users/force-sync` - Force user sync from Emby
- `ALL /admin/fix-pos-units` - Fix position units (internal)
- `GET /admin/debug/users` - Debug user data
- `POST /admin/recover-intervals` - Recover missing intervals (internal)

### Health
- `GET /health` - Database health
- `GET /health/emby` - Emby connection health

### Configuration
- `GET /config` - Get application configuration

Admin and debug endpoints (protected):

- `POST /admin/refresh/incremental` - Start incremental refresh
- `GET /admin/scheduler/stats` - Scheduler stats
- `POST /admin/cleanup/intervals/dedupe` and `GET /admin/cleanup/intervals/dedupe` - Interval dedupe
- `POST /admin/cleanup/backfill-playmethods` - Backfill per‑stream methods for historical sessions
- `GET /admin/webhook/stats` - Webhook endpoint info
- `GET /admin/debug/sessions` - Inspect recent `play_sessions` with filters
- `GET /admin/debug/emby-sessions` - Current sessions direct from Emby
- `POST /admin/debug/ingest-active` - Upsert rows for current active sessions

### Items & Images
- `GET /items/by-ids` - Get items by IDs
- `GET /img/primary/:id` - Get primary image
- `GET /img/backdrop/:id` - Get backdrop image

## Features in Detail

### Real-time Dashboard
- Live view of current playback sessions
- User avatars and session details
- Remote control capabilities (pause/stop/message)

### Analytics
- Historical usage trends per user
- Content popularity rankings
- Quality and codec breakdowns
- Activity heatmaps

### Data Management
- Automatic background syncing
- Manual refresh controls
- User data synchronization
- Data cleanup utilities

## Troubleshooting

### Occasional “database is locked” (SQLITE_BUSY)

SQLite allows multiple readers but only one writer at a time. Under bursts (e.g., session processor, minute‑ingest safeguard, and maintenance jobs writing together) a writer may find the DB busy for a moment.

This project mitigates contention by:
- Using Write‑Ahead Logging (WAL) to allow readers during writes.
- Setting `PRAGMA busy_timeout=5000` so writers retry briefly instead of failing immediately.
- Serializing DB connections with `SetMaxOpenConns(1)` to avoid multiple writer connections from this process.

These settings greatly reduce lock errors. If you still see sporadic logs, they’re usually transient and retried on the next tick. For heavier write loads, consider moving maintenance jobs off peak times, increasing `busy_timeout`, or placing the DB on fast local storage.

## Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
