# GEMINI.md

## Project Overview

This project, Emby Analytics, is a self-hosted dashboard and API service for monitoring and visualizing activity from an Emby media server.

The backend is written in Go using the Fiber v3 framework, and the frontend is a Next.js application. The backend serves the frontend's static export. Data is stored in a SQLite database. Real-time updates on the "Now Playing" dashboard are handled via Server-Sent Events (SSE).

The project is structured as a monorepo with the Go backend in the `go/` directory and the Next.js frontend in the `app/` directory.

## Building and Running

### Development

To run the backend and frontend in a development environment:

1.  **Backend (Go):**
    *   **Navigate to the Go directory:**
        ```bash
        cd go
        ```
    *   **Install dependencies:**
        ```bash
        go mod tidy
        ```
    *   **Run the application:**
        ```bash
        go run ./cmd/emby-analytics
        ```

2.  **Frontend (Next.js):**
    *   **Navigate to the app directory:**
        ```bash
        cd app
        ```
    *   **Install dependencies:**
        ```bash
        npm install
        ```
    *   **Run the development server:**
        ```bash
        npm run dev
        ```
    The frontend development server will proxy API requests to the Go backend.

### Production (Docker Compose Recommended)

The recommended way to run this project in production is using Docker Compose.

1.  **Copy Docker Compose example:**
    ```bash
    cp docker-compose-example.yml docker-compose.yml
    ```
2.  **Edit `docker-compose.yml`:** Update environment variables (`EMBY_BASE_URL`, `EMBY_API_KEY`) and the data volume path (`/path/to/data`).
3.  **Build and run with Docker Compose:**
    ```bash
    docker compose up -d --build
    ```

## Development Conventions

*   The backend and frontend are developed in their respective `go/` and `app/` directories.
*   The backend is responsible for all data fetching and storage.
*   The frontend is a static site that consumes the backend's API.
*   Configuration is managed through environment variables. A `.env.example` file is provided as a template.
*   Database migrations are located in `go/internal/db/migrations` and are managed with `golang-migrate`.