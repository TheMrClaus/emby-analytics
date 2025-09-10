## Convenience targets for local dev and CI

.PHONY: backend-run backend-build backend-test backend-fmt backend-vet lint-go frontend-dev frontend-build frontend-typecheck lint format

backend-run:
	cd go && go run ./cmd/emby-analytics

backend-build:
	# Derive build metadata; allow override via environment
	GIT_TAG=$$(git describe --tags --dirty --always 2>/dev/null || echo dev); \
	GIT_COMMIT=$$(git rev-parse --short HEAD 2>/dev/null || echo none); \
	BUILD_DATE=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	GIT_REPO=$${GIT_REPO:-$$(git config --get remote.origin.url | sed -E 's#(git@|https?://)([^/:]+)[:/](.+)\.git#\3#' | sed -E 's#^([^/]*)/(.*github.com/)?##' )}; \
	cd go && \
	GOCACHE=$(PWD)/go/.gocache CGO_ENABLED=0 go build \
	 -ldflags "-X emby-analytics/internal/version.Version=$$GIT_TAG -X emby-analytics/internal/version.Commit=$$GIT_COMMIT -X emby-analytics/internal/version.Date=$$BUILD_DATE -X emby-analytics/internal/version.Repo=$$GIT_REPO" \
	 -o emby-analytics ./cmd/emby-analytics

backend-test:
	cd go && go test ./...

backend-fmt:
	cd go && gofmt -s -w . && go fmt ./...

backend-vet:
	cd go && go vet ./...

lint-go: backend-fmt backend-vet

frontend-dev:
	cd app && npm run dev

frontend-build:
	cd app && npm run build

frontend-typecheck:
	cd app && npm run typecheck

lint: lint-go frontend-typecheck

format: backend-fmt
