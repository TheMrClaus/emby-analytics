# ---------- Stage 0: Build UI (old Next.js app) ----------
FROM node:20-alpine AS ui
WORKDIR /ui
COPY app/ ./app/
WORKDIR /ui/app
RUN npm ci
RUN npm run build && npm run export   # Next.js export mode creates ./out/

# ---------- Stage 1: Build Go backend ----------
FROM golang:1.22 AS builder
WORKDIR /src
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /emby-analytics ./go/cmd/emby-analytics

# ---------- Stage 2: Final rootless distroless (UID:GID 1000:1000) ----------
FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /emby-analytics /app/emby-analytics
COPY --from=ui /ui/app/out /app/web
VOLUME ["/var/lib/emby-analytics"]
EXPOSE 8080
USER 1000:1000
ENTRYPOINT ["/app/emby-analytics"]