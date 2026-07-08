# Repository Guidelines

## Project Structure & Module Organization
This repository contains a stock trading system with three main runtimes. `backend/` is a Go service: entry points live in `cmd/`, application code in `internal/`, reusable packages in `pkg/`, and SQL migrations in `migrations/` plus `internal/database/migrations/`. `frontend/` is a Svelte/Vite app with routes in `src/routes/`, shared UI in `src/components/`, API clients in `src/lib/api/`, stores in `src/lib/stores/`, and WebSocket code in `src/lib/ws/`. `python/` hosts backtesting, model, worker, and HTTP service code; modular backtest tests live under `python/backtest/modular/**/tests/`. Design and operations notes are in `docs/`.

## Build, Test, and Development Commands
Run commands from the relevant subdirectory unless noted.

- `cd backend && go run ./cmd/server`: start the Go API with local config and migrations.
- `cd backend && go test ./...`: run all Go tests.
- `cd frontend && npm install`: install frontend dependencies.
- `cd frontend && npm run dev`: start the Vite development server.
- `cd frontend && npm run build`: create the production frontend bundle.
- `cd python && .venv/Scripts/python.exe -m pytest backtest/ -v`: run Python backtest tests on Windows.
- `docker compose -f docker-compose.dev.yml up --build -d`: start the isolated Docker dev stack for validation.

For Docker-based validation, project/live separation, smoke tests, and reset commands, follow
[`docs/development-workflow.md`](docs/development-workflow.md).

## Coding Style & Naming Conventions
Format Go code with `gofmt`; keep packages lower-case and tests named `*_test.go`. Use idiomatic Svelte component names in PascalCase, such as `WatchlistTable.svelte`, and TypeScript modules in camelCase or domain names, such as `srZones.ts`. Python code should use 4-space indentation, `snake_case` modules and functions, and focused modules under `backtest/`, `models/`, or `utils/`.

## Testing Guidelines
Place Go unit tests beside the package they exercise and prefer table-driven tests for signal, store, and analysis logic. Python tests use pytest and follow `test_*.py` naming under each feature's `tests/` directory. Add or update tests when changing trading signals, persistence behavior, migrations, backtest calculations, or API contracts.

When validating development results with Docker, use the isolated dev compose project documented in
[`docs/development-workflow.md`](docs/development-workflow.md). Do not use the live/deploy compose project for test data, migrations, or destructive reset commands.

## Agent-Specific Instructions
Use [`docs/development-workflow.md`](docs/development-workflow.md) as the shared workflow source for Docker validation and issue/todo/documentation handling.

When receiving a request, first restate the understood requirements and wait for confirmation. Do not browse files, inspect docs, plan, edit, test, or run services before that confirmation. After the requirement is confirmed, inspect only the necessary context and propose a plan. Wait for plan confirmation before executing changes.

When documenting findings, do not create a new standalone docs file by default. Route items by type: bugs, contradictions, misleading behavior, or known limitations go into `docs/issue.md`; future improvements, feature ideas, and refactors go into `docs/todo.md`; durable design or operation notes go into the existing topic document under `docs/`. Create a new docs file only when the user explicitly asks for a new standalone document or no existing topic file fits.

Close out completed items so `docs/issue.md` and `docs/todo.md` do not accumulate resolved work. Once an `issue.md` item is fixed or a `todo.md` item is implemented, remove that entry entirely. Before removing it, move any behavior, design, or limitation worth keeping into the relevant topic document under `docs/` (for example `stock-analysis.md` or `sr-zone-scoring.md`) as current-state documentation, and fix any cross-references that pointed at the removed entry so no dead links remain. Keep in these lists only work that is still open (`issue.md`: to-fix or in-progress, or a deliberately retained known limitation not yet documented elsewhere; `todo.md`: planned or in-progress). If an entry is filed in the wrong list, move it to the correct one instead of leaving it.

## Commit & Pull Request Guidelines
Recent commits use short, task-oriented subjects, often in Chinese, for example `更新缺漏部分` or `sr_zone強化處理完成`. Keep commit subjects concise and focused on one change. Pull requests should include a clear summary, test results, linked issue or task context when available, and screenshots or screen recordings for visible frontend changes.

## Security & Configuration Tips
Do not commit secrets, tokens, local databases, or generated virtual environments. Keep service settings in `backend/config.yaml` and `python/config.yaml`, and document any required environment variable changes in `docs/development-guide.md`.
