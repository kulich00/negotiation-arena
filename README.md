# Negotiation Arena

Интерактивный тренажёр деловых переговоров. Пользователь проходит сценарий,
выбирает стратегию, вводит собственные формулировки и получает итоговый разбор.

## Стек

- Vue 3 Composition API + JavaScript + Vite
- Pinia + Vue Router
- Go 1.27 + `net/http`
- PostgreSQL + `pgx/v5`
- Docker Compose
- GitHub Actions

## Быстрый запуск

```bash
cp .env.example .env
docker compose up --build
```

После запуска:

- приложение: http://localhost:8080
- liveness: http://localhost:8080/health/live
- readiness: http://localhost:8080/health/ready

## Запуск без Docker

Frontend:

```bash
cd frontend
npm install
npm run dev
```

Backend:

```bash
cd backend
go mod download
go run ./cmd/server
```

Vite проксирует запросы `/api` и `/health` на `http://localhost:8080`.

## Что уже реализовано

- список сценариев;
- запуск переговорной сессии;
- текстовый диалог;
- локальная оценка аргументации, эмпатии и давления;
- ветвление ответов виртуального собеседника;
- завершение сессии и итоговый разбор;
- административный вход и создание сценария;
- резервный `MockProvider`, не требующий внешнего AI API;
- healthcheck;
- PostgreSQL-миграция;
- production Dockerfile;
- CI для frontend, backend и Docker.

## Переменные окружения

Все доступные переменные перечислены в `.env.example`. Для демонстрации можно
оставить `LLM_PROVIDER=mock`. Секреты и настоящий `.env` нельзя коммитить.

## Миграции

```bash
make migrate-up
```

Команда использует Goose и `DATABASE_URL` из окружения.

## Проверки

```bash
make test
make build
```

## Документация

- [Архитектура](docs/architecture.md)
- [Сценарий демонстрации](docs/demo.md)
- [API](docs/api.md)
