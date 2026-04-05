# Norms Audit

This file tracks whether high-value engineering and domain standards have a canonical location, an auto-matched instruction layer, and a reusable prompt layer.

## Audit Goal

Goal: keep the `.github/` customization surface maintainable by verifying three things for each high-value area:

1. Canonical standards live under `.github/standards/`.
2. High-frequency operational rules are mirrored in `.github/instructions/`.
3. High-frequency task entrypoints are represented in the active reusable prompt set under `.github/prompts/`.

Current assessed state: `covered, with manual-drift risk still present in audit documents and broad-scope entrypoints`

Definition of done:

1. Canonical standards are centralized under `.github/standards/`.
2. High-frequency implementation rules are mirrored in `.github/instructions/`.
3. High-frequency task entrypoints have normalized templates in `.github/prompts/`.
4. References point to canonical paths and no temporary compatibility layer is required.
5. Maintenance rules make future drift visible and correctable.

## Evidence Snapshot

Current evidence behind the assessed state:

- The active reusable prompt set is explicitly indexed in `.github/prompts/README.md` and currently matches the actual prompt files in `.github/prompts/`.
- One-off rider prompts that were not part of the indexed reusable set have been removed from `.github/prompts/` so the prompt directory is no longer mixing reusable routing assets with task-specific leftovers.
- Broad-scope entrypoints now prefer stable index docs such as `.github/standards/engineering/README.md` and `.github/standards/weapp/README.md` instead of repeating long nested `Read First` lists.
- Narrower instructions still exist for path-specific enforcement where the scope genuinely needs it.

## Coverage Matrix

| Area | Canonical Standard | Instructions Coverage | Prompt Coverage | Status |
| --- | --- | --- | --- | --- |
| Backend core | `.github/standards/backend/AGENT.md` | `backend-locallife`, `backend-api`, `backend-logic`, `backend-db-*`, `backend-worker`, `backend-scheduler`, `backend-cmd`, `backend-integration` | `general-implementation`, `general-review`, `backend-implementation`, `backend-review-closure`, `backend-integration-test` | Covered |
| API contract | `.github/standards/backend/API_CONTRACT_STANDARDS.md` | `backend-api`, `review` | `general-review`, `backend-review-closure`, `general-implementation` | Covered |
| Cross-cutting engineering governance | `.github/standards/engineering/ENGINEERING_GOVERNANCE_BASELINE.md`, `.github/standards/engineering/VALIDATION_AND_RELEASE_MATRIX.md`, `.github/standards/engineering/UNREACHABLE_DEPENDENCY_RISK_REGISTER.md`, `.github/standards/engineering/INCIDENT_FEEDBACK_LOOP.md`, `.github/standards/engineering/HIGH_RISK_CHANGE_CHECKLISTS.md` | `backend-locallife`, `web-ui`, `weapp-mini-program`, `review` | `general-implementation`, `general-review`, `general-task-loop`, `general-incident-followup` | Covered |
| Web UI | `.github/standards/web/WEB_UI_STANDARDS.md`, `.github/standards/web/DESIGN_GUARDRAILS.md`, `.github/standards/web/design-system.md` | `web-ui`, `web-operator-ui`, `web-merchant-ui`, `web-shared-ui` | `general-implementation`, `general-review`, `web-implementation`, `web-review` | Covered |
| Mini Program UI | `.github/standards/weapp/README.md`, `.github/standards/weapp/DESIGN_SYSTEM.md`, `.github/standards/weapp/INTERACTION_STANDARDS.md`, `.github/standards/weapp/API_INTERACTION_CONTRACT.md` | `weapp-mini-program`, `weapp-pages`, `weapp-components` | `general-implementation`, `general-review`, `weapp-implementation`, `weapp-review`, `weapp-page-refactor-blueprint`, `weapp-payment-flow` | Covered |
| Media domain | `.github/standards/domains/media/*` | `backend-media` | inherited through backend prompts | Covered |
| OCR domain | `.github/standards/domains/ocr/*` | `backend-ocr` | inherited through backend prompts | Covered |
| WeChat payment domain | `.github/standards/domains/wechat-payment/*` | `backend-wechat` | `backend-payment-runbook` | Covered |

## Residual Drift Risks

1. This audit file is still a manual artifact. If new prompts, instructions, or standards are added without updating this page, the coverage claim can drift from reality.
2. Broad-scope instruction files are thinner than before, but they still duplicate some risk and validation language by design. That duplication is lower-risk than prompt drift, but it remains a maintenance surface.
3. Domain coverage marked as `inherited through backend prompts` depends on prompt routing continuing to prefer the backend prompts plus the path-matched instructions; if a future domain introduces a distinct task pattern, this matrix should be revisited instead of assuming inheritance still fits.
4. Historical governance rollout artifacts must stay out of this coverage row unless they become active guidance again.

## Audit Checklist

Before marking a new area as covered, verify all of the following:

1. The canonical rule really lives under `.github/standards/` and not only in an instruction, prompt, or chat artifact.
2. The path-matched instruction layer points to the canonical rule instead of carrying a stale copy of the full standard.
3. The prompt layer uses only indexed reusable prompts; one-off prompts must not remain in `.github/prompts/` after the task ends.
4. Broad entrypoints prefer stable index docs over long nested reading lists.
5. Historical rollout material is not treated as default active guidance unless it is still the current operating baseline.

## Ongoing Maintenance Focus

1. Add narrower prompts whenever a surface develops repeated task patterns that no longer fit `general-*` templates cleanly.
2. Keep cross-cutting governance mirrors aligned whenever `standards/engineering/` changes.
3. Re-audit coverage whenever a new high-risk domain gets its own standards or runbooks.
4. Keep canonical paths stable so new documents do not recreate scattered standards.
5. Update this audit whenever a prompt is added, removed, or reclassified as non-reusable, because the prompt directory is a frequent drift surface.