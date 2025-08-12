# Emby Analytics

Emby Analytics is a self-hosted dashboard and API service for monitoring and visualizing activity from your [Emby](https://emby.media/) media server.  
It collects playback statistics, library information, and live "now playing" data, storing them in SQLite for historical analysis.  
A web interface built with Next.js provides rich charts and tables powered by [Recharts](https://recharts.org/).


## Features

- **Real-time "Now Playing" dashboard** via Server-Sent Events (SSE)
- **Usage analytics** (hours watched per user/day)
- **Top users** and **top items** in custom time windows
- **Media quality breakdown** (4K / 1080p / 720p / SD / Unknown)
- **Codec distribution**
- **Most active users** (over a configurable time window)
- **Library overview** by type (Movies, Series, Episodes)
- **Manual library refresh** from Emby
- **Lightweight database** (SQLite) for persistence
- **Single-page React UI** served via FastAPI


## Project Structure
```text
.
├── app/                     # Next.js frontend
│   ├── src/pages/            # React pages (main dashboard in index.tsx)
│   ├── package.json          # Frontend dependencies
│   └── next.config.js        # Next.js config (static export)
├── server/                   # FastAPI backend
│   ├── main.py                # API, SSE, data collector
│   └── requirements.txt       # Python dependencies
├── .gitignore
└── README.md
```

## Requirements
Python 3.10+
Node.js 18+
An accessible Emby Server with API key
pip and npm / yarn

## Backend Setup
1. Clone repo & install dependencies
```bash
git clone https://github.com/yourusername/emby-analytics.git
cd emby-analytics/server
python -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

2. Configure environment variables
```bash
export EMBY_BASE_URL=http://emby:8096
export EMBY_API_KEY=your_api_key_here
export SQLITE_PATH=./emby.db
```

3. Run the server
```bash
uvicorn main:app --reload --host 0.0.0.0 --port 8080
```

## Frontend Setup
1. Install dependencies
```bash
cd app
npm install
```

2. Run in dev mode
```bash
npm run dev
```
By default, it uses the backend at the same origin.
You can set NEXT_PUBLIC_API_BASE to override the API endpoint.

3. Build for production:
```bash
npm run build
npm run start
```

4. Static export for embedding in FastAPI
```bash
npm run build
npx next export
```
This creates an out/ directory served by FastAPI automatically if present.

## API Endpoints
The backend provides several REST endpoints:
| Endpoint                | Description                     |
| ----------------------- | ------------------------------- |
| `/health`               | Backend health check            |
| `/health/emby`          | Checks Emby connectivity        |
| `/admin/refresh`        | Starts library refresh          |
| `/admin/refresh/status` | Gets refresh status             |
| `/stats/overview`       | Library type counts             |
| `/stats/usage`          | Usage stats by user/day         |
| `/stats/top/users`      | Top users                       |
| `/stats/top/items`      | Top items                       |
| `/stats/qualities`      | Media quality distribution      |
| `/stats/codecs`         | Codec usage distribution        |
| `/stats/active-users`   | Most active users               |
| `/stats/users/total`    | Total registered users          |
| `/now/stream`           | Real-time SSE for "Now Playing" |
| `/img/primary/{id}`     | Proxied poster image            |
| `/img/backdrop/{id}`    | Proxied backdrop image          |

## Deployment
You can run backend and frontend separately, or use FastAPI to serve the static export of the frontend.
For production, use gunicorn or uvicorn workers for the backend, and reverse proxy with Nginx or Traefik.

Example (backend only):
```bash
uvicorn main:app --host 0.0.0.0 --port 8080 --workers 4
```

## License
MIT License — free to use, modify, and distribute.
