# ---------- Stage 0: Build UI (Next.js export mode) ----------
FROM node:20-alpine3.22 AS ui
WORKDIR /ui/app
# Install deps using lockfile first for better caching
COPY app/package*.json ./
RUN npm ci
# Copy the rest of the UI source and build (Next.js with output:'export' writes to ./out)
COPY app/ .
RUN npm run build

# ---------- Stage 1: Build Go backend ----------
FROM golang:1.25 AS builder
# Work inside the Go module directory so go.mod is found
WORKDIR /src/go
COPY go/ .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /emby-analytics ./cmd/emby-analytics

# ---------- Stage 2: Final rootless distroless (UID:GID 1000:1000) ----------
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /emby-analytics /app/emby-analytics
# Copy the statically-exported UI to be served by the Go app
COPY --from=ui /ui/app/out /app/web
VOLUME ["/var/lib/emby-analytics"]
EXPOSE 8080
USER 1000:1000
ENTRYPOINT ["/app/emby-analytics"]
