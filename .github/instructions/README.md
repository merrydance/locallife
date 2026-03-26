# Instructions Index

This directory contains auto-matched AI instruction files.

## Naming Rule

- Use `<scope>-<area>.instructions.md`
- Prefer specific scopes over generic scopes
- Keep `applyTo` patterns as narrow as practical

## Priority Rule

When multiple instruction files match the same file, the more specific directory-focused file should be treated as stricter guidance than the broader workspace or backend file.

## Current Coverage

Backend base and layers:

- `backend-locallife.instructions.md`
- `backend-api.instructions.md`
- `backend-logic.instructions.md`
- `backend-db-query.instructions.md`
- `backend-db-sqlc.instructions.md`
- `backend-worker.instructions.md`
- `backend-scheduler.instructions.md`
- `backend-integration.instructions.md`
- `backend-cmd.instructions.md`
- `backend-media.instructions.md`
- `backend-ocr.instructions.md`
- `backend-wechat.instructions.md`

Frontend:

- `web-ui.instructions.md`
- `web-operator-ui.instructions.md`
- `web-merchant-ui.instructions.md`
- `web-shared-ui.instructions.md`
- `weapp-mini-program.instructions.md`
- `weapp-pages.instructions.md`
- `weapp-components.instructions.md`

Cross-cutting:

- `review.instructions.md`

## Maintenance Rule

If an original project standard is updated, mirror the most actionable operational changes here so the auto-matched instruction layer stays aligned.