# Эксплуатация и диагностика

## Локальный Docker Compose

Первый запуск:

```powershell
Copy-Item .env.example .env
docker compose up -d --build
```

Проверка состояния и журналов:

```powershell
docker compose ps
docker compose logs --tail 100 app
docker compose logs --tail 100 db
docker compose logs --tail 100 model
```

Пересборка после изменения кода:

```powershell
docker compose up -d --build app
```

После изменения ArenaLM, её зависимостей или артефакта модели:

```powershell
docker compose up -d --build model app
```

Остановка с сохранением PostgreSQL:

```powershell
docker compose down
```

Полный сброс локальной БД удаляет именованный volume и все данные:

```powershell
docker compose down -v
docker compose up -d --build
```

## Проверка работоспособности

`GET /health/live` проверяет, что HTTP-процесс отвечает:

```powershell
curl.exe -sS http://localhost:8080/health/live
```

Ожидаемый ответ:

```json
{"status":"ok"}
```

`GET /health/ready` дополнительно выполняет ping PostgreSQL, если сервер запущен
с `DATABASE_URL`:

```powershell
curl.exe -sS http://localhost:8080/health/ready
```

Ожидаемый ответ — `200 OK` и `{"status":"ready"}`. Ошибка соединения с БД
возвращает `503 Service Unavailable`.

Prometheus-метрики доступны без авторизации на локальном endpoint:

```powershell
curl.exe -sS http://localhost:8080/metrics
```

Экспортируются HTTP latency и статусы по шаблону маршрута, обращения к Gemini и
fallback, игровые намерения и техники, изменения показателей, запуски и исходы
сессий. ID игроков и сессий в labels не попадают.

Готовность собственной модели проверяется отдельно:

```powershell
curl.exe -sS http://localhost:8090/health
```

Если контейнер `model` станет недоступен после запуска, backend продолжит
работать через настроенный fallback и увеличит метрику
`arena_llm_operations_total{operation="arena_model_interpretation",result="fallback"}`.

Docker healthcheck использует liveness. Для проверки готовности приложения к
трафику следует использовать readiness.

## Журналы

Backend пишет JSON в stdout. Журнал каждого HTTP-запроса содержит:

- `requestId` — идентификатор из ответного `X-Request-ID`;
- `method` и `path`;
- `status`;
- `bytes`;
- `duration`.

Ключи Gemini, пароли и Bearer tokens не журналируются. При старте Gemini
выводятся модель и количество ключей.

Примеры диагностических сообщений:

- `Gemini returned HTTP 429` — текущая квота ключей исчерпана;
- `all configured Gemini API keys are temporarily unavailable` — весь пул на
  cooldown, используется локальный fallback;
- `configuration validation failed` — нарушены production-требования;
- `database connection failed` — PostgreSQL недоступен при запуске.

## Миграции

При непустом `DATABASE_URL` сервер применяет встроенные Goose-миграции до
запуска HTTP listener. Повторный запуск безопасен: уже применённые версии
пропускаются.

Ручной запуск из Unix shell:

```bash
export DATABASE_URL='postgres://arena:arena@localhost:5432/arena?sslmode=disable'
make migrate-up
```

Откат последней миграции:

```bash
make migrate-down
```

Перед откатом production-БД требуется резервная копия. Приложение обычно должно
быть остановлено, чтобы схема не менялась во время запросов.

## Проверки backend

```powershell
Set-Location backend
go test ./...
go vet ./...
go test -race ./...
go build ./cmd/server
```

### Проверка с PostgreSQL

Контейнер БД должен быть запущен и доступен через `localhost:5432`:

```powershell
$env:TEST_DATABASE_URL='postgres://arena:arena@localhost:5432/arena?sslmode=disable'
go test ./internal/repository -run TestPostgresPersistence -count=1 -v
```

Интеграционный тест применяет миграции и удаляет созданные им данные после
проверки.

Полная матрица проверок frontend, backend, ArenaLM, PostgreSQL и Docker
приведена в [отдельном руководстве](testing.md). Она соответствует шагам CI и
дополняет их локальной проверкой отказоустойчивости.

## Типовые проблемы

### Порт 8080 или 5432 уже занят

Проверьте занявший порт процесс либо измените публикацию порта в `compose.yaml`.
Изменение только `HTTP_ADDR` не меняет внешний Docker port mapping.

### Gemini всегда отвечает 429

Проверьте квоту проекта в Google AI Studio/Google Cloud. Несколько ключей одного
проекта могут иметь общий лимит. Приложение продолжит работу через локальный
fallback; после cooldown ключи автоматически вернутся в ротацию.

### Readiness возвращает 503

Проверьте `docker compose ps`, затем журналы `app` и `db`. Для запуска backend
на хосте в `DATABASE_URL` должен использоваться `localhost`, а внутри Compose —
имя сервиса `db`.

### API возвращает 429

Для публичных изменяющих запросов дождитесь числа секунд из `Retry-After`.
Для административного входа проверьте `ADMIN_LOGIN_MAX_ATTEMPTS` и
`ADMIN_LOGIN_WINDOW`.

## Render

`render.yaml` создаёт web service из Dockerfile и PostgreSQL. Перед запуском
нужно задать секретные `ADMIN_EMAIL` и `ADMIN_PASSWORD`. Для Gemini также
задаются `LLM_PROVIDER=gemini` и `LLM_API_KEYS` как секреты.

В Render `TRUST_PROXY_HEADERS=true`, поэтому rate limit использует адрес клиента
из заголовка proxy. Readiness endpoint `/health/ready` указан как health check.
