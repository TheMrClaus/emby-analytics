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

### Option 1: Docker (Recommended)
```bash
# Build and run with Docker
docker build -t emby-analytics .
docker run -d \
  -p 8080:8080 \
  -v /path/to/data:/var/lib/emby-analytics \
  -e EMBY_BASE_URL=http://your-emby:8096 \
  -e EMBY_API_KEY=your_api_key \
  emby-analytics
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
- `SYNC_INTERVAL`: Background sync interval in seconds (default: 60)
- `HISTORY_DAYS`: Days of history to sync (default: 2)

## API Endpoints

### Statistics
- `GET /stats/overview` - General library overview
- `GET /stats/usage` - Usage analytics by user/day
- `GET /stats/top/users` - Top users by watch time
- `GET /stats/top/items` - Most watched content
- `GET /stats/qualities` - Quality distribution
- `GET /stats/codecs` - Codec statistics
- `GET /stats/activity` - Activity timeline

### Now Playing
- `GET /now` - Current playback snapshot
- `GET /now/stream` - SSE stream of live updates
- `POST /now/sessions/:id/pause` - Pause session
- `POST /now/sessions/:id/stop` - Stop session

### Admin
- `POST /admin/refresh` - Start library refresh
- `GET /admin/refresh/status` - Refresh progress
- `POST /admin/users/sync` - Sync users from Emby
- `POST /admin/reset-all-data` - Reset all data

### Health
- `GET /health` - Database health
- `GET /health/emby` - Emby connection health

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

## Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Commit changes (`git commit -m 'Add amazing feature'`)
4. Push to branch (`git push origin feature/amazing-feature`)
5. Open Pull Request

## License

[Add your license here]
