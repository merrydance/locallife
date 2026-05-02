# LocalLife Prompt Routing Map

## Always-On Entry Points

- `AGENTS.md`: Codex root entrypoint for this repository.
- `.github/copilot-instructions.md`: workspace-wide rules and app summaries.
- `.github/README.md`: normalized index for instructions, prompts, standards, and routing rules.
- `locallife/AGENTS.md`: backend subtree rules.

## Instructions

- Backend: `backend-locallife`, `backend-api`, `backend-logic`, `backend-db-query`, `backend-db-sqlc`, `backend-worker`, `backend-scheduler`, `backend-cmd`, `backend-integration`, `backend-wechat`, `backend-media`, `backend-ocr`.
- Frontend: `web-ui`, `web-operator-ui`, `web-merchant-ui`, `web-shared-ui`, `weapp-mini-program`, `flutter-merchant-app`.
- Review: `review.instructions.md`.

## Prompts

- Backend: `backend-implementation`, `backend-bugfix`, `backend-payment-domain`, `backend-integration-test`, `backend-sql-review`, `backend-review-closure`, `backend-takeover`, `backend-task-card-implementation`, `backend-phase-batch-implementation`.
- Frontend: `web-implementation`, `web-review`, `weapp-implementation`, `weapp-review`, `flutter-implementation`, `flutter-bugfix`, `flutter-review`.
- General: `general-implementation`, `general-review`, `general-task-loop`, `general-incident-followup`, `business-flow-mermaid`.

## Standards Hot Paths

- Cross-cutting governance: `.github/standards/engineering/README.md`, `ENGINEERING_GOVERNANCE_BASELINE.md`, `VALIDATION_AND_RELEASE_MATRIX.md`, `AI_PROMPT_GOVERNANCE.md`, `INCIDENT_FEEDBACK_LOOP.md`.
- Backend: `.github/standards/backend/README.md`, `RUNTIME_ARCHITECTURE.md`, `WORKFLOW_AND_VALIDATION.md`, `API_CONTRACT_STANDARDS.md`, `ERROR_HANDLING.md`, `SQL_STANDARDS.md`, `BACKEND_CHANGE_SAFETY_CHECKLIST.md`, `BACKEND_REVIEW_CLOSEOUT_CHECKLIST.md`.
- WeChat payment: `.github/standards/domains/wechat-payment/README.md`.
- Web: `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`, `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`, `.github/standards/web/WEB_UI_STANDARDS.md`, `.github/standards/web/DESIGN_GUARDRAILS.md`, `.github/standards/web/design-system.md`.
- Mini Program: `.github/standards/weapp/README.md`, `PAGE_DELIVERY_BASELINE.md`, `DESIGN_SYSTEM.md`, `API_INTERACTION_CONTRACT.md`.
- Flutter: `.github/standards/flutter/README.md`, `FLUTTER_APP_ARCHITECTURE.md`, `PRODUCTION_ROBUSTNESS_BASELINE.md`, `PUSH_NOTIFICATION_STANDARDS.md`, `REVIEW_CHECKLIST.md`.
