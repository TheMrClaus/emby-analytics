# ---------- Build stage ----------
FROM golang:1.22 AS builder
WORKDIR /src
COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 go build -o /emby-analytics ./cmd/emby-analytics

# Final stage
FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=builder /emby-analytics /app/emby-analytics
COPY go/web /app/web

VOLUME ["/var/lib/emby-analytics"]
EXPOSE 8080

# Run as media server UID:GID
USER 1000:1000

ENTRYPOINT ["/app/emby-analytics"]
