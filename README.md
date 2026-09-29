# Negotiation Arena

[![CI](https://github.com/kulich00/negotiation-arena/actions/workflows/ci.yml/badge.svg)](https://github.com/kulich00/negotiation-arena/actions/workflows/ci.yml)

Интерактивный тренажёр деловых переговоров. Игрок проходит сценарии разной
сложности, формулирует ответы свободным текстом и получает объяснимый разбор
каждого хода и всей сессии.

Backend принимает окончательные решения детерминированно. Явные действия
распознаются локальными правилами, остальные реплики классифицирует ArenaLM на
PyTorch. Gemini при включении уточняет неоднозначные намерения и делает ответы
естественнее, но не управляет очками, исходом или прогрессом игрока. При
недоступности AI игровой цикл продолжает работать на локальном анализаторе.
Повторный ход получает подсказку о следующем полезном действии вместо
одинаковой реплики.

## Возможности

- сценарии `easy`, `medium` и `hard` со скрытыми условиями оппонента;
- Гарвардский метод, четыре этапа SPIN и подготовка BATNA;
- стандартный и сложный оппонент с настроением и сменой приоритетов;
- показатели доверия, аргументации и давления с разбором каждого изменения;
- контекстная оценка ходов без повторной награды за уже выполненное действие;
- десять стабильных классов ошибок, рекомендации и итоговый анализ переговоров;
- девять достижений, серии успешных прохождений и прогрессия сложности;
- открытие следующего уровня за успешные `100/100` или накопленные победы;
- контрольные точки и повтор с выбранного хода через новую ветку сессии;
- административное управление сценариями, история и статистика сессий;
- PostgreSQL, автоматические миграции и атомарное сохранение хода;
- Gemini с пулом из 1–8 ключей, cooldown и локальным fallback;
- продолжение диалога после рекомендуемого числа ходов и завершение по исходу;
- request ID, структурированные журналы, rate limit и health endpoints;
- Prometheus-метрики HTTP, Gemini/fallback, ходов, очков и исходов;
- анонимизированный JSONL/JSON экспорт корпуса переговоров для обучения и evals;
- собственная локальная ArenaLM для классификации свободных реплик;
- единый production-контейнер со встроенной Vue SPA.

## Стек

- Vue 3, Pinia, Vue Router, Vite;
- Go 1.27 и `net/http`;
- PostgreSQL 18, `pgx/v5`, Goose;
- Docker Compose;
- GitHub Actions.

## Быстрый запуск

Требуются Docker Desktop и Docker Compose.

PowerShell:

```powershell
Copy-Item .env.example .env
docker compose up -d --build
docker compose ps
```

Bash:

```bash
cp .env.example .env
docker compose up -d --build
docker compose ps
```

После запуска доступны:

- приложение: <http://localhost:8080>;
- liveness: <http://localhost:8080/health/live>;
- readiness: <http://localhost:8080/health/ready>;
- ArenaLM: <http://localhost:8090/health>;
- PostgreSQL: `localhost:5432`.

Быстрая проверка готовности в PowerShell:

```powershell
Invoke-RestMethod http://localhost:8080/health/ready
Invoke-RestMethod http://localhost:8090/health
```

Оба запроса должны вернуть `status: ready`. В ответе ArenaLM также будет версия
`arena-intents-v2-pytorch`.

`docker compose down` останавливает проект и сохраняет данные в volume
`postgres_data`. Подробные команды запуска, диагностики, миграций и очистки
данных приведены в [руководстве по эксплуатации](docs/operations.md).

## Режимы AI

По умолчанию в `.env.example` используется автономный режим:

```dotenv
LLM_PROVIDER=mock
```

Для Gemini:

```dotenv
LLM_PROVIDER=gemini
LLM_API_KEYS=ключ_1,ключ_2,ключ_3,ключ_4,ключ_5
LLM_MODEL=gemini-2.5-flash
LLM_TIMEOUT_SECONDS=15
```

Docker Compose автоматически подключает ArenaLM через
`ARENA_MODEL_URL=http://model:8090`. Компактная нейросеть на PyTorch
классифицирует намерения до обращения к внешней модели и использует
существующий fallback при низкой уверенности.
Обучение и устройство модели описаны в [ml/README.md](ml/README.md).

`LLM_API_KEYS` принимает от 1 до 8 уникальных ключей. При `401`, `403` или
`429` backend пробует следующий доступный ключ. Ключи с ошибками временно
исключаются из ротации. Если весь пул недоступен, ход обрабатывается локально и
не теряется.

Квота Gemini обычно назначается Google Cloud проекту. Несколько ключей одного
проекта могут использовать общую квоту. Сами ключи должны находиться только в
локальном `.env` или в секретах платформы; `.env` исключён из Git и Docker
build context.

Полное описание приоритетов переменных, cooldown и production-проверок есть в
[справочнике конфигурации](docs/configuration.md).

## Запуск для разработки

Frontend:

```bash
cd frontend
npm ci
npm run dev
```

Backend без PostgreSQL, с временным хранилищем в памяти:

```bash
cd backend
go mod download
go run ./cmd/server
```

Backend читает переменные из окружения процесса и самостоятельно не загружает
файл `.env`. Для постоянного хранения задайте `DATABASE_URL`; при прямом
запуске на хосте адрес PostgreSQL из Compose будет
`postgres://arena:arena@localhost:5432/arena?sslmode=disable`.

Vite проксирует `/api` и `/health` на `http://localhost:8080`.

## Проверки

Backend:

```bash
cd backend
go test ./...
go vet ./...
go test -race ./...
go build ./cmd/server
```

Frontend:

```bash
cd frontend
npm ci
npm run lint
npm test -- --run
npm run build
```

Полный набор также запускается в GitHub Actions. Интеграционный тест PostgreSQL
включается переменной `TEST_DATABASE_URL`; пример есть в
[руководстве по эксплуатации](docs/operations.md#проверка-с-postgresql).
Команды, ожидаемые результаты и ручная проверка fallback собраны в
[руководстве по тестированию](docs/testing.md).

## Структура

```text
backend/
  cmd/server/          запуск HTTP-сервера
  internal/httpapi/    маршруты и middleware
  internal/negotiation бизнес-логика тренажёра
  internal/repository/ PostgreSQL и in-memory репозитории
  internal/llm/        Gemini и локальный fallback
  migrations/          миграции Goose
frontend/              Vue SPA
ml/                    обучение, eval и HTTP-сервис ArenaLM
corpus/                схема и стартовый корпус диалогов
docs/                  проектная документация
```

## Документация

- [Конфигурация](docs/configuration.md)
- [Эксплуатация и диагностика](docs/operations.md)
- [API](docs/api.md)
- [Архитектура](docs/architecture.md)
- [Сценарий демонстрации](docs/demo.md)
- [Тестирование](docs/testing.md)
- [Состояние и дальнейшее развитие](docs/roadmap.md)
