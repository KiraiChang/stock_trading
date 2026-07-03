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
- `docker-compose up --build`: start the composed stack, including supporting services.

## Coding Style & Naming Conventions
Format Go code with `gofmt`; keep packages lower-case and tests named `*_test.go`. Use idiomatic Svelte component names in PascalCase, such as `WatchlistTable.svelte`, and TypeScript modules in camelCase or domain names, such as `srZones.ts`. Python code should use 4-space indentation, `snake_case` modules and functions, and focused modules under `backtest/`, `models/`, or `utils/`.

## Testing Guidelines
Place Go unit tests beside the package they exercise and prefer table-driven tests for signal, store, and analysis logic. Python tests use pytest and follow `test_*.py` naming under each feature's `tests/` directory. Add or update tests when changing trading signals, persistence behavior, migrations, backtest calculations, or API contracts.

## Agent-Specific Instructions
When receiving a request, first restate the understood requirements and wait for confirmation. Do not browse files, inspect docs, plan, edit, test, or run services before that confirmation. After the requirement is confirmed, inspect only the necessary context and propose a plan. Wait for plan confirmation before executing changes.

## Commit & Pull Request Guidelines
Recent commits use short, task-oriented subjects, often in Chinese, for example `更新缺漏部分` or `sr_zone強化處理完成`. Keep commit subjects concise and focused on one change. Pull requests should include a clear summary, test results, linked issue or task context when available, and screenshots or screen recordings for visible frontend changes.

## Security & Configuration Tips
Do not commit secrets, tokens, local databases, or generated virtual environments. Keep service settings in `backend/config.yaml` and `python/config.yaml`, and document any required environment variable changes in `docs/development-guide.md`.
