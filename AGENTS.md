# Repository Guidelines

## Project Structure & Module Organization
This repository contains a stock trading system with three main runtimes. `backend/` is a Go service: entry points live in `cmd/`, application code in `internal/`, reusable packages in `pkg/`, and SQL migrations in `internal/database/migrations/`. `frontend/` is a Svelte/Vite app with routes in `src/routes/`, shared UI in `src/components/`, API clients in `src/lib/api/`, stores in `src/lib/stores/`, and WebSocket code in `src/lib/ws/`. `python/` hosts backtesting, model, worker, and HTTP service code; modular backtest tests live under `python/backtest/modular/**/tests/`. Design and operations notes are in `docs/`.

## Build, Test, and Development Commands
Run commands from the repo root unless noted. Prefer repository scripts for validation; they encode the Docker, cache, resource-limit, and file-ownership rules documented in
[`docs/development-workflow.md`](docs/development-workflow.md).

- `cd backend && go run ./cmd/server`: start the Go API with local config and migrations.
- `cd frontend && npm run dev`: start the Vite development server.
- `backend/scripts/test.sh`: run Go `vet`, tests, and build for all packages.
- `backend/scripts/test.sh ./internal/market/...`: run targeted Go validation.
- `TEST_FLAGS="-count=1 -v" backend/scripts/test.sh ./internal/market/...`: pass extra Go test flags.
- `python/scripts/test.sh`: run Python pytest defaults through the project test image.
- `python/scripts/test.sh backtest/modular/sr_scoring/tests`: run targeted Python tests.
- `frontend/scripts/test.sh`: run the frontend production build check.
- `frontend/scripts/test.sh --install`: install frontend dependencies with `npm ci` before the build check.
- `scripts/smoke-dev.sh`: start the isolated Docker dev stack and wait for backend/python health checks.

For Docker-based validation, project/live separation, smoke tests, and reset commands, follow
[`docs/development-workflow.md`](docs/development-workflow.md).

## Coding Style & Naming Conventions
Format Go code with `gofmt`; keep packages lower-case and tests named `*_test.go`. Use idiomatic Svelte component names in PascalCase, such as `WatchlistTable.svelte`, and TypeScript modules in camelCase or domain names, such as `srZones.ts`. Python code should use 4-space indentation, `snake_case` modules and functions, and focused modules under `backtest/`, `models/`, or `utils/`.

## Testing Guidelines
Place Go unit tests beside the package they exercise and prefer table-driven tests for signal, store, and analysis logic. Python tests use pytest and follow `test_*.py` naming under each feature's `tests/` directory. Add or update tests when changing trading signals, persistence behavior, migrations, backtest calculations, or API contracts.

When validating development results, use the runtime scripts first: `backend/scripts/test.sh`, `python/scripts/test.sh`, `frontend/scripts/test.sh`, and `scripts/smoke-dev.sh`. Do not hand-write one-off `docker run` commands as the validation path. If a script lacks a needed capability, propose the script change and add it after confirmation.

When using Docker validation, use the isolated dev compose project documented in
[`docs/development-workflow.md`](docs/development-workflow.md). Do not use the live/deploy compose project for test data, migrations, or destructive reset commands.

## Agent-Specific Instructions
Use [`docs/development-workflow.md`](docs/development-workflow.md) as the shared workflow source for Docker validation and issue/todo/documentation handling.

For tests and smoke checks, prioritize the repository scripts from `docs/development-workflow.md`. Use direct Docker commands only for diagnostics such as inspecting status or logs, not as the primary validation command.

When receiving a request, first restate the understood requirements and wait for confirmation. Do not browse files, inspect docs, plan, edit, test, or run services before that confirmation. After the requirement is confirmed, inspect only the necessary context and propose a plan. Wait for plan confirmation before executing changes.

For large or high-impact changes, leave a written plan before implementation. This is mandatory for cross-module changes, changes touching more than one runtime, trading-signal or decision-logic changes, database/API/frontend contract changes, scheduler/data-sync changes, broad refactors, and validation workflow changes. The plan must be reviewable later, not only implied in conversation: record the goal, scope, affected files/modules, intended data or arbitration flow, risks, test strategy, and the documentation file where the rationale/current behavior will be archived after completion. Do not start implementation until the plan is confirmed. If implementation diverges from the confirmed plan, stop and report the delta before continuing. After implementation, keep the plan/todo/issue entry in place for review; remove it only after review confirms the implementation direction is correct.

When documenting findings, do not create a new standalone docs file by default. Route items by type: bugs, contradictions, misleading behavior, or known limitations go into `docs/issue.md`; future improvements, feature ideas, and refactors go into `docs/todo.md`; durable design or operation notes go into the existing topic document under `docs/`. Create a new docs file only when the user explicitly asks for a new standalone document or no existing topic file fits.

Close out completed items so `docs/issue.md` and `docs/todo.md` do not accumulate resolved work, but do not remove plan/todo/issue entries immediately after implementation. First update the entry status to implemented/review pending, keep the plan content for review, and remove it only after review confirms the implementation direction is correct. Before removing it, move any behavior, design, or limitation worth keeping into the relevant topic document under `docs/` (for example `stock-analysis.md` or `sr-zone-scoring.md`) as current-state documentation, and fix any cross-references that pointed at the removed entry so no dead links remain. Keep in these lists only work that is still open or awaiting review (`issue.md`: to-fix, in-progress, review pending, or a deliberately retained known limitation not yet documented elsewhere; `todo.md`: planned, in-progress, implemented/review pending). If an entry is filed in the wrong list, move it to the correct one instead of leaving it.

## Commit & Pull Request Guidelines
Recent commits use short, task-oriented subjects, often in Chinese, for example `更新缺漏部分` or `sr_zone強化處理完成`. Keep commit subjects concise and focused on one change. Pull requests should include a clear summary, test results, linked issue or task context when available, and screenshots or screen recordings for visible frontend changes.

## Security & Configuration Tips
Do not commit secrets, tokens, local databases, or generated virtual environments. Keep service settings in `backend/config.yaml` and `python/config.yaml`, and document any required environment variable changes in `docs/development-guide.md`.

## Token Budget Policy

- Do not repeatedly retry the same failed tool call.
- Stop after 2 identical tool failures.
- Do not scan the entire repository unless explicitly required.
- Prefer targeted file searches.
- Before large refactors, produce a plan and wait for approval.
- Run targeted tests before full test suites.
- Stop and report blockers instead of repeatedly debugging infrastructure failures.
