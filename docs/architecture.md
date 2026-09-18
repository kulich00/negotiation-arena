# Архитектура

Проект реализован как модульный монолит.

```text
Пользователь / Администратор
            ↓
        Vue 3 SPA
            ↓
       Go HTTP API
       ↙         ↘
PostgreSQL    Negotiation Engine
                    ↓
             AI Provider / Mock
```

## Frontend

Vue отвечает за интерфейс игрока и администратора. Состояние переговорной
сессии хранится в Pinia. Повторяемая логика выносится в composables, HTTP-вызовы
изолированы в `src/api`.

## Backend

- `httpapi` — HTTP-маршруты и middleware;
- `negotiation` — бизнес-логика и оценка реплик;
- `repository` — хранение сценариев и сессий;
- `llm` — абстракция AI-провайдера;
- `database` — подключение к PostgreSQL;
- `webapp` — встроенная production-сборка Vue.

## Надёжность AI

Сценарный движок работает без внешней модели. AI улучшает анализ текста и
формулировки ответов, но при ошибке используется `MockProvider`.

## Production

Vue собирается на первом этапе Dockerfile. Результат встраивается в Go-бинарник.
В production запускается один контейнер и управляемая PostgreSQL.

