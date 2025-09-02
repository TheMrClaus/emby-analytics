# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Emby Analytics is a self-hosted dashboard for monitoring Emby media server activity. It uses a Go backend with Fiber v3 and a Next.js frontend, storing data in SQLite. The application provides real-time "now playing" monitoring via WebSocket/SSE and historical analytics.

## Architecture

**Backend (Go)**:
- `go/cmd/emby-analytics/main.go` - Main application entry point with Fiber setup
- `go/internal/` - Internal packages organized by domain:
  - `config/` - Environment-based configuration management
  - `db/` - SQLite database operations and migrations  
  - `emby/` - Emby API client and WebSocket handling
  - `handlers/` - HTTP route handlers organized by feature (admin, health, images, items, now, stats)
  - `tasks/` - Background sync tasks and the Intervalizer for real-time analytics
  - `queries/` - Database query logic

**Frontend (Next.js)**:
- `app/src/pages/` - React pages
- `app/src/components/` - Reusable UI components including charts
- `app/src/contexts/` - React contexts for state management
- `app/src/hooks/` - Custom React hooks
- Uses Tailwind CSS for styling, Recharts for visualizations, SWR for data fetching

**Key Data Flow**:
1. Emby WebSocket connection feeds real-time events to Intervalizer
2. Intervalizer processes events and updates SQLite database
3. Frontend polls API endpoints for statistics and real-time data
4. SSE broadcasts live "now playing" updates to connected clients

## Common Development Commands

### Backend Development
```bash
# Run Go backend in development (from project root)
cd go
go mod tidy
go run ./cmd/emby-analytics

# Run tests (if any exist)
cd go
go test ./...

# Build binary
cd go
CGO_ENABLED=0 go build -o emby-analytics ./cmd/emby-analytics
```

### Frontend Development
```bash
# Run Next.js in development mode (from project root)
cd app
npm install
npm run dev

# Build for production (static export)
cd app
npm run build

# Preview production build locally
cd app
npm run preview
```

### Full Production Build
```bash
# Build frontend static files
cd app && npm run build

# Build Go binary with embedded frontend
cd go
cp -r ../app/out ./web
CGO_ENABLED=0 go build -o emby-analytics ./cmd/emby-analytics
```

### Docker
```bash
# Build container
docker build -t emby-analytics .

# Run with environment variables
docker run -d \
  -p 8080:8080 \
  -v /path/to/data:/var/lib/emby-analytics \
  -e EMBY_BASE_URL=http://your-emby:8096 \
  -e EMBY_API_KEY=your_api_key \
  emby-analytics
```

## Configuration

Environment variables are loaded from `.env` file or system environment. Key settings:

- `EMBY_BASE_URL` - Emby server URL (default: http://emby:8096)
- `EMBY_API_KEY` - Emby API key (required)  
- `SQLITE_PATH` - Database location (default: /var/lib/emby-analytics/emby.db)
- `WEB_PATH` - Static frontend files path (default: /app/web)
- `SYNC_INTERVAL_SEC` - Background sync interval (default: 300)
- `HISTORY_DAYS` - Days of history to sync (default: 2)

See `.env.example` for complete configuration options.

## Key Components

**Real-time Analytics**:
- `go/internal/tasks/intervalizer.go` - Core real-time analytics engine that processes Emby events
- `go/internal/emby/ws.go` - WebSocket client for Emby server events
- `go/internal/handlers/now/broadcaster.go` - SSE broadcaster for "now playing" updates

**Database Layer**:
- Uses embedded SQLite migrations in `go/internal/db/migrations/`
- Database operations centralized in `go/internal/db/` and `go/internal/queries/`
- Schema supports users, items, sessions, and various analytics tables

**API Endpoints**:
- `/stats/*` - Historical analytics and statistics
- `/now/*` - Real-time session data and controls  
- `/admin/*` - Administrative functions (refresh, sync, reset)
- `/health/*` - Health checks for database and Emby connectivity
- `/img/*` - Proxied images from Emby with caching

**Frontend Architecture**:
- Next.js with static export for production deployment
- SWR for efficient data fetching and caching
- Recharts for data visualization components
- Responsive design with Tailwind CSS

## Database Schema

The SQLite database includes tables for:
- `users` - Emby user information
- `library_items` - Movies, shows, episodes with metadata
- `play_sessions` - Individual playback sessions with detailed analytics
- `session_intervals` - Time-based analytics for accurate watch time calculation
- Various lookup tables for qualities, codecs, play methods

## Development Tips

- The Intervalizer is the core component - it converts Emby's real-time events into structured analytics data
- WebSocket connection to Emby is maintained automatically with reconnection logic
- Database migrations are embedded in the Go binary and run automatically on startup
- Frontend development server proxies API calls to the Go backend on port 8080
- Use the `/health` endpoints to verify Emby connectivity during development
- Admin endpoints provide manual control over data sync and refresh operations