.PHONY: dev test build frontend-install frontend-build backend-test model-train model-eval model-test migrate-up migrate-down

dev:
	docker compose up --build

frontend-install:
	cd frontend && npm ci

frontend-build:
	cd frontend && npm run build

backend-test:
	cd backend && go test ./...

model-train:
	python -m ml.train --corpus corpus/seed-v1.jsonl --output ml/model/arena-intents-v1.json --version arena-intents-v1

model-eval:
	python -m ml.evaluate --minimum-accuracy 0.85

model-test:
	python -m unittest ml.tests.test_model -v

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

