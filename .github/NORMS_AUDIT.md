# Norms Audit

This file tracks whether high-value engineering and domain standards have a canonical location, an auto-matched instruction layer, and a reusable prompt layer.

## Score Goal

Target: `10/10`

Current assessed state: `10/10`

Definition of done:

1. Canonical standards are centralized under `.github/standards/`.
2. High-frequency implementation rules are mirrored in `.github/instructions/`.
3. High-frequency task entrypoints have normalized templates in `.github/prompts/`.
4. References point to canonical paths and no temporary compatibility layer is required.
5. Maintenance rules make future drift visible and correctable.

## Coverage Matrix

| Area | Canonical Standard | Instructions Coverage | Prompt Coverage | Status |
| --- | --- | --- | --- | --- |
| Backend core | `.github/standards/backend/AGENT.md` | `backend-locallife`, `backend-api`, `backend-logic`, `backend-db-*`, `backend-worker`, `backend-scheduler`, `backend-cmd`, `backend-integration` | `general-implementation`, `general-review`, `backend-implementation`, `backend-review-closure`, `backend-integration-test` | Covered |
| API contract | `.github/standards/backend/API_CONTRACT_STANDARDS.md` | `backend-api`, `review` | `general-review`, `backend-review-closure`, `general-implementation` | Covered |
| Web UI | `.github/standards/web/WEB_UI_STANDARDS.md`, `.github/standards/web/DESIGN_GUARDRAILS.md`, `.github/standards/web/design-system.md` | `web-ui`, `web-operator-ui`, `web-merchant-ui`, `web-shared-ui` | `general-implementation`, `general-review`, `web-implementation`, `web-review` | Covered |
| Mini Program UI | `.github/standards/weapp/DESIGN_SYSTEM.md`, `.github/standards/weapp/api/README.md` | `weapp-mini-program`, `weapp-pages`, `weapp-components` | `general-implementation`, `general-review`, `weapp-implementation`, `weapp-review` | Covered |
| Media domain | `.github/standards/domains/media/*` | `backend-media` | inherited through backend prompts | Covered |
| OCR domain | `.github/standards/domains/ocr/*` | `backend-ocr` | inherited through backend prompts | Covered |
| WeChat payment domain | `.github/standards/domains/wechat-payment/*` | `backend-wechat` | `backend-payment-runbook` | Covered |

## Ongoing Maintenance Focus

1. Add narrower prompts whenever a surface develops repeated task patterns that no longer fit `general-*` templates cleanly.
2. Re-audit coverage whenever a new high-risk domain gets its own standards or runbooks.
3. Keep canonical paths stable so new documents do not recreate scattered standards.