# GEMINI.md

## Project Overview

This project, Emby Analytics, is a self-hosted dashboard and API service for monitoring and visualizing activity from an Emby media server.

The backend is written in Go using the Fiber v3 framework, and the frontend is a Next.js application. The backend serves the frontend's static export. Data is stored in a SQLite database. Real-time updates on the "Now Playing" dashboard are handled via Server-Sent Events (SSE).

The project is structured as a monorepo with the Go backend in the `go/` directory and the Next.js frontend in the `app/` directory.

## Building and Running

### Backend (Go)

To run the backend in a development environment:

1.  **Navigate to the Go directory:**
    ```bash
    cd go
    ```
2.  **Install dependencies:**
    ```bash
    go mod tidy
    ```
3.  **Run the application:**
    ```bash
    go run ./cmd/emby-analytics
    ```

### Frontend (Next.js)

To run the frontend in a development environment:

1.  **Navigate to the app directory:**
    ```bash
    cd app
    ```
2.  **Install dependencies:**
    ```bash
    npm install
    ```
3.  **Run the development server:**
    ```bash
    npm run dev
    ```

The frontend development server will proxy API requests to the Go backend.

### Production

The recommended way to run this project in production is by using the provided `Dockerfile`.

1.  **Build the Docker image:**
    ```bash
    docker build -t emby-analytics .
    ```
2.  **Run the Docker container:**
    ```bash
    docker run -d \
      -p 8080:8080 \
      -v /path/to/data:/var/lib/emby-analytics \
      -e EMBY_BASE_URL=http://your-emby:8096 \
      -e EMBY_API_KEY=your_api_key \
      emby-analytics
    ```

## Development Conventions

*   The backend and frontend are developed in their respective `go/` and `app/` directories.
*   The backend is responsible for all data fetching and storage.
*   The frontend is a static site that consumes the backend's API.
*   Configuration is managed through environment variables. A `.env.example` file is provided as a template.
*   Database migrations are located in `go/internal/db/migrations` and are managed with `golang-migrate`.
