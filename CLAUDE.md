# LocalLife Workspace Instructions

This workspace contains multiple applications. Choose the target project first, then follow that project's commands and conventions.

See `.github/README.md` for the normalized index of standards, instructions, prompt templates, naming conventions, and authoritative source documents.

## Workspace Layout

- `locallife/`: Go backend API, workers, scheduler, SQL migrations, generated sqlc code, and integration tests.
- `merchant_app/`: Flutter Android merchant app for order notifications, voice alerts, and receipt printing.
- `web/`: Next.js web console.
- `weapp/`: WeChat Mini Program.
- `legal_exports/`: exported agreement HTML files and helper scripts. Treat as reference material unless the task is specifically about legal content export.

## How To Work In This Workspace

1. Identify the target area before editing.
2. Run commands from the correct project directory instead of the workspace root.
3. Prefer updating existing patterns over introducing a new architecture.
4. Link to authoritative docs in this repo instead of duplicating them in code comments or new docs.
5. Prefer area index docs and broad instructions first; only drill into deeper standards when the active path actually needs them.

## Backend: `locallife/`

Read these first:

- `.github/standards/engineering/README.md`
- `.github/standards/backend/README.md`

Common commands (run from `locallife/`):

- `make test-unit`
- `make test-integration`
- `make test`
- `make server`
- `make sqlc`
- `make mock`
- `make swagger`
- `make migrateup`
- `make new_migration name=<name>`

Backend conventions:

- Keep the HTTP three-layer split: `api/` handles transport, `logic/` holds business logic, `db/sqlc/` owns persistence.
- Do not put business logic in handlers.
- Inject dependencies explicitly through constructors or service structs. Do not introduce package-level runtime globals.
- Core functions should accept `context.Context` as the first argument.
- Use `db/sqlc/constants.go` as the single source of truth for business status constants.
- Use structured logging via zerolog. Do not add `fmt.Println` or unstructured logging in request paths.
- Use existing request error mapping patterns instead of inventing a new API error shape.

Backend generation:

- If you change SQL in `locallife/db/query/` or schema assumptions, run `make sqlc`.
- If you change interfaces used by mocks, run `make mock` or `make sqlc` as appropriate.
- If you change Swagger annotations or routes, run `make swagger`.

## Flutter Merchant App: `merchant_app/`

Read these first:

- `.github/standards/flutter/README.md`
- `.github/standards/flutter/FLUTTER_APP_ARCHITECTURE.md`
- `.github/standards/engineering/README.md`

Common commands (run from `merchant_app/`):

- `flutter pub get`
- `flutter run`
- `flutter build apk --release`
- `flutter analyze`
- `flutter test`

Flutter conventions:

- Use Riverpod for state management. Prefer `StreamProvider` for WebSocket and push message streams.
- Follow feature-first directory structure: `lib/features/<feature>/`.
- Keep UI, business logic, and data layers separated.
- All user-facing strings must be in Chinese.
- Message deduplication is mandatory: all three delivery channels (WebSocket, push, polling) may deliver the same order notification.
- Do not ignore Android lifecycle. Foreground Service must stay active with a persistent notification.
- When touching push notification or keep-alive logic, read `.github/standards/flutter/PUSH_NOTIFICATION_STANDARDS.md`.

## Web: `web/`

Read these first:

- `.github/standards/engineering/README.md`
- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
- `web/README.md`

Common commands: `npm run dev`, `npm run build`, `npm run lint`

## Mini Program: `weapp/`

Read these first:

- `.github/standards/engineering/README.md`
- `.github/standards/weapp/README.md`

Common commands: `npm run compile`, `npm run lint`, `npm run lint:fix`

## Practical Defaults

- For backend tasks, inspect existing files in the same domain package before adding new abstractions.
- For Flutter tasks, inspect existing providers and services before creating new patterns.
- For frontend tasks, inspect adjacent route segments or pages before creating new layout patterns.
- For generated-code workflows, update source files first, then regenerate, then run the smallest relevant validation command.
- Avoid broad refactors unless the task explicitly asks for them.
