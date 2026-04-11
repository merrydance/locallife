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

Area-specific instructions live in `.github/instructions/` and should be treated as stricter rules when the current file matches their `applyTo` patterns.

Reusable prompt templates live in `.github/prompts/`.

Use this file as a workspace routing summary. Detailed execution rules should live in `.github/instructions/`, and long-lived norms should live in `.github/standards/`.

## Prompt Artifact Hygiene

- Do not create a new prompt file for one-off analysis, planning notes, scratch implementation steps, or temporary task decomposition.
- Treat `.github/prompts/` as a library of reusable templates, not as a task-by-task archive.
- Default to not creating prompt files unless the user explicitly asks to save one or the prompt is expected to be reused across multiple future tasks.
- Prefer session memory, an existing design doc, or the active conversation for temporary working notes.
- If a reusable prompt already exists for the same workflow, update or replace it instead of creating a near-duplicate file.
- For a single topic, keep at most one `plan` prompt and one `implement` prompt unless the user explicitly requests a separate variant.
- After a task is completed, do not preserve temporary prompt drafts in the workspace unless the user asked to keep them or they clearly became a reusable team asset.

## Backend: `locallife/`

Read these first when changing backend behavior:

- `.github/standards/engineering/README.md`
- `.github/standards/backend/README.md`

Then open the smallest relevant backend deep docs for the current task instead of loading the full stack by default:

- `RUNTIME_ARCHITECTURE.md` for real entrypoints, async boundaries, or takeover context
- `WORKFLOW_AND_VALIDATION.md` for commands, regeneration, and validation depth
- `API_CONTRACT_STANDARDS.md` for route, status-code, and empty-state semantics
- `BACKEND_RISK_MAP.md` for funds, state machines, callback, worker, and recovery hot paths

Common commands:

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
- Use `db/sqlc/constants.go` as the single source of truth for business status constants. Do not add magic strings in handlers, logic, or tests.
- Use structured logging. Do not add `fmt.Println` or unstructured logging in request paths.
- Use the existing request error mapping patterns instead of inventing a new API error shape.

Backend generation and validation rules:

- If you change SQL in `locallife/db/query/` or schema assumptions, run `make sqlc`.
- If you change interfaces used by mocks, run `make mock` or `make sqlc` as appropriate.
- If you change Swagger annotations or routes, run `make swagger`.
- Keep non-test files in `locallife/api/`, `locallife/logic/`, and `locallife/worker/` within the existing file size guardrail enforced by `make lint-filesize`.
- Prefer `make test-unit` for focused validation. Run `make test-integration` only when the change touches integration flows or database-backed behavior.

## Web: `web/`

Read these first when changing the web app:

- `.github/standards/engineering/README.md`
- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
- `web/README.md`
- `.github/standards/web/WEB_UI_STANDARDS.md`
- `.github/standards/web/DESIGN_GUARDRAILS.md`
- `.github/standards/web/design-system.md`

Common commands:

- `npm run dev`
- `npm run build`
- `npm run lint`

Web conventions:

- Preserve the existing visual system and component patterns.
- Prefer existing components in `web/src/components/ui/` before creating new primitives.
- Do not hardcode one-off colors or typography tokens when a semantic utility already exists.
- Keep page-level data and API logic out of presentational components when the codebase already separates them.
- Check UI standards before changing operator or merchant pages.

## Flutter Merchant App: `merchant_app/`

Read these first when changing the Flutter merchant app:

- `.github/standards/engineering/README.md`
- `.github/standards/flutter/README.md`

Common commands:

- `flutter pub get`
- `flutter run`
- `flutter build apk --release`
- `flutter analyze`
- `flutter test`

Flutter conventions:

- Use Riverpod for state management.
- Feature-first directory structure: `lib/features/<feature>/`.
- Message deduplication is mandatory across all three delivery channels (WebSocket, push, polling).
- All user-facing strings must be in Chinese.
- Foreground Service must be active with a persistent notification.

## Mini Program: `weapp/`

Read these first when changing the Mini Program:

- `.github/standards/engineering/README.md`
- `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`
- `.github/standards/weapp/README.md`

Common commands:

- `npm run compile`
- `npm run lint`
- `npm run lint:fix`
- `npm run quality:check`

Mini Program conventions:

- Treat the backend contract as the sole source of truth for capabilities, fields, enums, and state semantics.
- Prefer TDesign Miniprogram before introducing local UI primitives or wrappers. Use the TDesign MCP component list and docs to inspect component groups by use, then choose the closest existing component.
- Keep page shell spacing consistent: the gap below the top navigation must follow one approved spacing pattern, horizontal page gutters must stay consistent across pages, and bottom content or actions must include safe-area handling.
- Treat user-facing copy as product copy, not developer terminology.

## Documentation Map

Use these docs as references instead of rewriting them:

- Media backend and migration docs: `.github/standards/domains/media/*`
- OCR rollout and refactor docs: `.github/standards/domains/ocr/*`
- WeChat payment plans and runbooks: `.github/standards/domains/wechat-payment/*`
- API contract rules: `.github/standards/backend/API_CONTRACT_STANDARDS.md`

## Practical Defaults For Agents

- For backend tasks, inspect existing files in the same domain package before adding new abstractions.
- For frontend tasks, inspect adjacent route segments or pages before creating new layout patterns.
- For generated-code workflows, update source files first, then regenerate, then run the smallest relevant validation command.
- Avoid broad refactors unless the task explicitly asks for them.