# Standards Center

This directory is the canonical home for project-owned standards and normative documents.

## Layout

- `backend/`: backend engineering standards and API contract standards.
- `web/`: web UI standards and design guardrails.
- `weapp/`: Mini Program design standards and API-facing project conventions.
- `domains/media/`: media-domain standards, designs, and runbooks.
- `domains/ocr/`: OCR-domain standards, execution plans, checklists, and runbooks.
- `domains/wechat-payment/`: WeChat payment domain standards and runbooks.

## Canonical Rule

Files in this directory are the canonical project-owned standards.
References should point here directly. Do not recreate scattered copies or compatibility links unless a short migration window is explicitly required.

## Maintenance Rule

When a standard changes here, update the matching `.github/instructions/*.instructions.md` file if the change affects day-to-day implementation, review, regeneration, or validation behavior.