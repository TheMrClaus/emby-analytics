# Repository Guidelines

## Project Structure & Modules
- `go/`: Fiber v3 backend (`cmd/emby-analytics`, `internal/{db,handlers,emby,tasks}`), SQLite migrations in `internal/db/migrations`.
- `app/`: Next.js 14 web UI (`src/{pages,components,hooks}`), Tailwind v4, static export via `next.config.js`.
- `data/`: Runtime SQLite and artifacts (gitignored).
- `Dockerfile`, `compose.yml`, `.env.example`: Deployment and config templates.

## Build, Test, and Development
- Backend (dev): `cd go && go run ./cmd/emby-analytics` — starts API on `:8080` (requires `.env`).
- Frontend (dev): `cd app && npm install && npm run dev` — runs Next.js on `:3000`.
- Frontend (build/export): `cd app && npm run build` — emits static site to `app/out/`.
- Backend (build): `cd go && CGO_ENABLED=0 go build -o emby-analytics ./cmd/emby-analytics`.
- Docker (local): `docker compose up --build -d` — uses `compose.yml` and mounts `./data`.

## Coding Style & Naming
- Go: run `go fmt ./...` and `go vet ./...`; package/file names lower_snake_case; exported identifiers UpperCamelCase; errors wrapped with context; avoid global state.
- TypeScript/React: 2‑space indent, single quotes, semicolons; components in `src/components` use PascalCase; hooks in `src/hooks` start with `use*`; prefer functional components, SWR for data.
- CSS: Tailwind utility classes; global tweaks in `src/styles/globals.css`.

## Testing Guidelines
- No formal test suite yet. For Go changes, add `_test.go` with table‑driven tests and run `go test ./...` in edited packages.
- For UI changes, validate by: `npm run build` (type‑checks) and manual smoke tests against `http://localhost:8080/health` and key routes (e.g., `/stats/overview`, `/now/ws`). Include screenshots for visual changes.

## Commits & Pull Requests
- Commits: imperative, concise subject; optionally prefix type (`fix:`, `feat:`, `docs:`). Examples from history: "Fix …", "Add …", "Update …".
- PRs: clear description, linked issue, endpoints touched, manual test steps, and screenshots for UI. Avoid bundling unrelated changes. Ensure `.env` and `data/` are never committed.

## Security & Configuration
- Configure via `.env` (copy from `.env.example`). Required: `EMBY_BASE_URL`, `EMBY_API_KEY`; optional: `SQLITE_PATH`, `WEB_PATH`, `SYNC_INTERVAL`.
- Persist data by mounting `./data:/var/lib/emby-analytics` (see compose). Never commit secrets or database files.
