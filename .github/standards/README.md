# Standards Center

This directory is the canonical home for project-owned standards and normative documents.

## Layout

- `engineering/`: cross-cutting engineering governance for security, consistency, resilience, validation, release readiness, and incident feedback.
- `frontend/`: cross-frontend architecture and feedback standards shared by Web, Mini Program, and Flutter merchant app.
- `backend/`: backend engineering standards and API contract standards.
- `web/`: web UI standards and design guardrails.
- `weapp/`: Mini Program design standards and API-facing project conventions.
- `domains/media/`: media-domain standards, designs, runbooks, and an index that separates active guidance from historical rollout material.
- `domains/ocr/`: OCR-domain standards, runbooks, and an index that separates active guidance from historical rollout material.
- `domains/wechat-payment/`: WeChat payment standards, runbooks, and an index that separates active guidance from historical rollout material.
- `domains/baofu-payment/`: Baofoo/Baofu BaoCaiTong payment contract standards, field matrices, raw-source ledger, and evidence ledger.

External API and provider contract rules live under `backend/EXTERNAL_API_CONTRACT_STANDARDS.md`; provider-specific source matrices and evidence ledgers should live under the matching `domains/*` directory when one exists.

## Canonical Rule

Files in this directory are the canonical project-owned standards.
References should point here directly. Do not recreate scattered copies or compatibility links unless a short migration window is explicitly required.

## Maintenance Rule

When a standard changes here, update the matching `.github/instructions/*.instructions.md` file if the change affects day-to-day implementation, review, regeneration, or validation behavior.

For changes under `engineering/`, also evaluate whether the affected rule should be mirrored into `.github/prompts/`, `.github/workflows/`, or `.github/NORMS_AUDIT.md`, because cross-cutting governance is expected to propagate beyond standards-only guidance.

When a rollout plan, cutover checklist, migration playbook, or release-only document completes its purpose, keep it discoverable as historical material but avoid leaving it in the default hot path for routine code changes.

## Historical Placement Convention

- Use the domain root for active guidance that should remain in the default reading path.
- Use the matching domain `historical/` directory for completed rollout plans, execution trackers, cutover checklists, one-time release runbooks, rollback plans tied to a finished rollout, and migration diaries.
- If a historical document becomes active again because a rollout is reopened, move it back only if it is once again the current operating baseline.
