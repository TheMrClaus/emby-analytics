# ---------- Stage 0: Build UI (Next.js export mode) ----------
FROM node:20-alpine3.22@sha256:c233e52f53d1e1f7d5b74158416c1761665e7188b394b2976c6b223d5118d39c AS ui
WORKDIR /ui/app
# Install deps using lockfile first for better caching
COPY app/package*.json ./
RUN npm ci
COPY app/ .
RUN npm run build

# ---------- Stage 1: Build Go backend ----------
FROM golang:1.25@sha256:d894b901a117b375b42d729c782786a454d436e1c9e42e47265a7f920f2694b8 AS builder
# Work inside the Go module directory so go.mod is found
WORKDIR /src/go
COPY go/ .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /emby-analytics ./cmd/emby-analytics

# ---------- Stage 2: Final rootless distroless (UID:GID 1000:1000) ----------
FROM gcr.io/distroless/static-debian12@sha256:695a7090b8f36c56b7c5ec4d1b64e03f56e0d9b43e74c833d791244e8379df4b
WORKDIR /app
COPY --from=builder /emby-analytics /app/emby-analytics
# Copy the statically-exported UI to be served by the Go app
COPY --from=ui /ui/app/out /app/web

VOLUME ["/var/lib/emby-analytics"]
EXPOSE 8080