# API

## Сценарии

- `GET /api/v1/scenarios` — список сценариев.
- `POST /api/v1/admin/scenarios` — создание сценария, нужен Bearer token.

## Сессии

- `POST /api/v1/sessions` — запустить сессию.
- `GET /api/v1/sessions/{id}` — получить состояние.
- `POST /api/v1/sessions/{id}/messages` — отправить реплику.
- `POST /api/v1/sessions/{id}/finish` — завершить переговоры.
- `GET /api/v1/sessions/{id}/result` — получить итог.

## Администратор

- `POST /api/v1/admin/login` — получить токен.

## Состояние

- `GET /health/live` — процесс работает.
- `GET /health/ready` — приложение готово и база доступна.

