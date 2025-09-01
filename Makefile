## Convenience targets for local dev and CI

.PHONY: backend-run backend-build backend-test backend-fmt backend-vet lint-go frontend-dev frontend-build frontend-typecheck lint format

backend-run:
	cd go && go run ./cmd/emby-analytics

backend-build:
	cd go && CGO_ENABLED=0 go build -o emby-analytics ./cmd/emby-analytics

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

