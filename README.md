# AI Interview Coach

Production-like MVP ИИ-тренера для подготовки к техническим собеседованиям. Первый transport — Telegram bot, но бизнес-логика вынесена в service/interview слои и не зависит от Telegram.

## Что реализовано

- Telegram flow: `/start`, `/interview`, `/stop`, `/history`, `/help`.
- Выбор специализации, уровня и длительности интервью.
- MVP специализации:
  - Go Backend;
  - Java Backend;
  - JavaScript/TypeScript Frontend.
- Уровни: Junior, Middle, Senior.
- Режимы: Quick, Standard, Full.
- Adaptive Interview Engine:
  - не использует фиксированный список вопросов;
  - хранит покрытые темы, сильные/слабые темы, текущую сложность;
  - ограничивает follow-up по теме;
  - выбирает следующую тему с учетом interview plan;
  - валидирует structured LLM output.
- PostgreSQL persistence:
  - users;
  - specializations;
  - interview_sessions;
  - interview_answers.
- OpenAI-compatible LLM adapter через `/v1/chat/completions`.
- Backend grade calculation отдельно от LLM.
- Unit tests для engine, grade и service state flow.
- Docker Compose: `app` + `postgres`.

## Архитектура

```text
Telegram adapter
  -> InterviewService
    -> InterviewEngine
      -> LLMClient
    -> Repository
      -> PostgreSQL
```

Telegram слой отвечает только за команды, inline keyboards и отправку сообщений. В будущем можно добавить `cmd/api/` и REST API поверх тех же `InterviewService`, `InterviewEngine`, repositories и LLM client.

## Структура проекта

```text
cmd/bot/                 Telegram bot entrypoint
internal/config/         Environment configuration
internal/domain/         Domain models and repository interfaces
internal/interview/      Interview engine, catalog, grading, LLM interface
internal/llm/            OpenAI-compatible LLM implementation
internal/repository/     Persistence adapters
internal/service/        Application use cases
internal/telegram/       Telegram transport adapter
migrations/              PostgreSQL schema and seed data
```

## Prerequisites

- Go 1.25+
- Docker and Docker Compose
- Telegram Bot token from BotFather
- LLM API key for an OpenAI-compatible provider

## Environment variables

Copy `.env.example` to `.env` and fill secrets:

```env
TELEGRAM_BOT_TOKEN=
DATABASE_URL=postgres://postgres:postgres@postgres:5432/ai_interview?sslmode=disable
LLM_API_KEY=
LLM_BASE_URL=https://api.openai.com/v1
LLM_MODEL=gpt-4.1-mini
LLM_TIMEOUT=45s
LLM_MAX_RETRIES=2
MAX_ANSWER_LENGTH=6000
ANSWER_RATE_LIMIT_PER_MINUTE=6
START_RATE_LIMIT_PER_HOUR=10
LOG_LEVEL=info
```

Secrets must stay outside git.

## Run

```bash
cp .env.example .env
docker compose up --build
```

The app runs embedded idempotent migrations on startup. The root `migrations/` directory is kept for inspection and manual DB initialization.

For local development without Docker:

```bash
go mod tidy
go run ./cmd/bot
```

`DATABASE_URL` must point to a PostgreSQL database with `migrations/001_init.sql` applied.

## Tests and checks

```bash
go fmt ./...
go vet ./...
go test ./...
docker compose config
```

Tests use mock LLM implementations and do not call real LLM APIs.

## How to add a specialization

1. Add a row to `specializations` through a migration or admin seed.
2. Add a `SpecDefinition` in `internal/interview/catalog.go`.
3. Define plans per level through `[]domain.PlanArea`.
4. Add a Telegram callback button if it should be available in the bot UI.

The engine already passes specialization slug, plan and system prompt to the LLM.

## How to connect another LLM

Implement `interview.LLMClient`:

```go
type LLMClient interface {
    GenerateQuestion(...)
    EvaluateAnswer(...)
    GenerateFinalFeedback(...)
}
```

Wire the implementation in `cmd/bot/main.go`. Keep JSON validation in the engine and provider-specific retry/timeout behavior inside the adapter.

## MVP limitations

- Telegram wizard draft state is in memory. Active interview state is persisted in PostgreSQL.
- No web frontend and no REST API yet.
- Migrations are plain SQL. For production, add a migration runner for repeatable upgrades.
- No authentication beyond Telegram user identity.
- No payment/subscription model.
- No voice interviews or speech-to-text.

## Roadmap

1. Add `cmd/api/` REST API using the existing service layer.
2. Add migration runner and versioned deployment workflow.
3. Add progress analytics across multiple interviews.
4. Add vacancy-based interview generation.
5. Add learning plan generation and spaced repetition.
