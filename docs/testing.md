# Тестирование

Документ описывает проверки, которые подтверждают работу интерфейса, backend,
PostgreSQL и локальной модели. Команды запускаются из корня репозитория, если в
разделе не указано другое.

## Требования

- Go 1.27;
- Node.js 22 и npm;
- Python 3.13 с зависимостями из `ml/requirements.txt`;
- Docker Desktop с Docker Compose.

Для Python рекомендуется использовать локальное окружение `.venv`. Установка
PyTorch и остальных зависимостей описана в [руководстве ArenaLM](../ml/README.md).

## Backend

```powershell
Set-Location backend
go test ./...
go vet ./...
go test -race ./...
go build ./cmd/server
Set-Location ..
```

Проверки охватывают расчёт доверия, аргументации и давления, SPIN/BATNA,
предложения, исходы, прогрессию сложности, достижения, ветвление сессий,
административный API, middleware и AI fallback.

## PostgreSQL

Запустите БД и передайте тесту отдельный DSN:

```powershell
docker compose up -d db
Set-Location backend
$env:TEST_DATABASE_URL='postgres://arena:arena@localhost:5432/arena?sslmode=disable'
go test ./internal/repository -run TestPostgresPersistence -count=1 -v
Remove-Item Env:TEST_DATABASE_URL
Set-Location ..
```

Тест применяет миграции, проверяет постоянное хранение и удаляет созданные им
данные.

## Frontend

```powershell
Set-Location frontend
npm ci
npm run lint
npm test -- --run
npm run build
Set-Location ..
```

`npm run build` также проверяет, что production SPA может быть встроена в
Go-приложение.

## ArenaLM

```powershell
python -m unittest scripts.test_prepare_corpus ml.tests.test_model -v
python -m ml.evaluate --minimum-accuracy 0.90
```

Eval использует отдельный фиксированный набор из 33 реплик. Для текущего
артефакта ожидаются accuracy 0,9394 и macro F1 0,9409. Порог CI равен 0,90.

Переобучение не требуется для обычного запуска. После изменения корпуса или
модели артефакт создаётся командой:

```powershell
python -m ml.train `
  --corpus corpus/seed-v1.jsonl `
  --output ml/model/arena-intents-v2.pt `
  --version arena-intents-v2-pytorch
```

После переобучения обязательно повторите model tests и eval.

## Docker и health endpoints

```powershell
docker compose up -d --build
docker compose ps
Invoke-RestMethod http://localhost:8080/health/live
Invoke-RestMethod http://localhost:8080/health/ready
Invoke-RestMethod http://localhost:8090/health
```

Все три контейнера должны иметь статус `healthy`. Ожидаемые ответы:

```json
{"status":"ok"}
{"status":"ready"}
{"status":"ready","modelVersion":"arena-intents-v2-pytorch"}
```

Интерфейс после проверки доступен по адресу <http://localhost:8080>.

## Проверка AI fallback

Проверка подтверждает, что отказ ArenaLM не останавливает игровой ход:

1. Создайте игрока и активную сессию через интерфейс.
2. Выполните хотя бы один обычный ход и убедитесь, что он сохранён.
3. Остановите только модель: `docker compose stop model`.
4. Отправьте ещё одну реплику. Backend должен вернуть успешный ответ с локальной
   интерпретацией и сохранить ход.
5. Запустите модель: `docker compose start model`.
6. Проверьте `http://localhost:8090/health`.

При `LLM_PROVIDER=gemini` ответы `401`, `403`, `429`, сетевые ошибки и
исчерпание всего пула ключей также не отменяют ход. Backend перебирает доступные
ключи, а затем использует локальную интерпретацию и детерминированную реплику.

## Проверка перед коммитом

```powershell
git diff --check
git status --short
```

CI повторяет lint, unit tests, race detector, сборку frontend/backend, model
tests, eval и сборку двух Docker-образов. PostgreSQL integration test и ручная
проверка fallback выполняются локально.
