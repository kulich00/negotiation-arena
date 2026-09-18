.PHONY: dev test build frontend-install frontend-build backend-test migrate-up migrate-down

dev:
	docker compose up --build

frontend-install:
	cd frontend && npm ci

frontend-build:
	cd frontend && npm run build

backend-test:
	cd backend && go test ./...

test:
	cd frontend && npm test -- --run
	cd backend && go test ./...

build:
	cd frontend && npm run build
	cd backend && go build ./cmd/server

migrate-up:
	cd backend && go run github.com/pressly/goose/v3/cmd/goose@v3.24.3 -dir migrations postgres "$(DATABASE_URL)" up

migrate-down:
	cd backend && go run github.com/pressly/goose/v3/cmd/goose@v3.24.3 -dir migrations postgres "$(DATABASE_URL)" down

