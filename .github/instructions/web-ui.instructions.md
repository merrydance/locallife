---
applyTo: "web/**"
---

# Web Instructions

Apply these rules for files under `web/`.

More specific web instruction files under `.github/instructions/` take precedence when their `applyTo` pattern matches, especially for `web/src/app/operator/`, `web/src/app/merchant/`, and `web/src/components/ui/`.

## Read First

- `.github/standards/engineering/README.md`
- `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`
- `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
- `web/README.md`
- `.github/standards/web/WEB_UI_STANDARDS.md`
- `.github/standards/web/DESIGN_GUARDRAILS.md`
- `.github/standards/web/design-system.md`

Use `.github/standards/engineering/README.md` as the stable governance index, then open the baseline or validation matrix when the active change needs deeper risk or release-readiness guidance.
Use `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md` before designing non-trivial Web pages, multi-state flows, operator/merchant workflows, or any UI that aggregates multiple backend capabilities.

## Risk Classification

- Treat style-only or copy-only work as `G0` only when no data contract, page state, dangerous action, permission display, or user flow behavior changes.
- Treat ordinary page or component work as `G1` when it does not alter status semantics, sensitive fields, dangerous operations, or cross-surface behavior.
- Escalate to `G2` when the change affects multi-state task flows, approval/rejection, disabling/enabling, payout-like status handling, cross-page field propagation, or failure recovery behavior.
- Escalate to `G3` when the UI exposes or controls authz-sensitive actions, sensitive/private data, money-related operations, moderation-sensitive material, or any flow where an incorrect state or confirmation pattern could cause a high-impact incident.

## Working Style

- Preserve the existing visual system and component patterns.
- Start from the user's task and the required ViewState before deciding page sections or component boundaries; do not mirror API shape into the route by default.
- Prefer existing components in `web/src/components/ui/` before creating new primitives.
- Do not hardcode one-off colors or typography tokens when a semantic utility already exists.
- Check the existing route segment and nearby pages before introducing a new layout pattern.
- Keep page-level data fetching and API logic out of presentational components when the codebase already separates them.
- Preserve established operator and merchant page patterns unless the task explicitly changes the design system.

## UI Rules To Apply Directly

- Use the established `PageHeader + PageContent` page skeleton.
- Keep a single business flow within at most two card layers unless the task explicitly changes the visual system.
- Prefer the existing component white list and semantics from `.github/standards/web/WEB_UI_STANDARDS.md`: status filters should use `Select`, category switching should use `Tabs`, and list data should use the existing table patterns.
- Keep user-facing copy business-readable. Map backend enum values to readable labels instead of exposing raw enum strings.
- Treat feedback behavior as a system rule: no raw backend errors in UI, no redundant success prompt after navigation or structural page update, and no Toast-only handling for first-screen failures.
- Do not use developer-facing phrasing such as `debug`, `fallback`, `proxy`, or “与小程序一致” in operator-facing or merchant-facing UI copy.
- Any user-visible status or field added in a high-risk flow must be threaded through types, API calls, page state, loading/empty/error branches, dangerous-action confirmation, and disabled/in-flight states as applicable.
- API responses must be adapted into business-readable view models or domain objects before presentational components render them; raw DTO field groups should not become the visual architecture.
- For `G2` and `G3` changes, explicitly check what the user sees during submit, timeout, retry, partial failure, and refresh-after-action scenarios.

## Validation Defaults

- Run commands from `web/`.
- Common commands: `npm run dev`, `npm run build`, `npm run lint`.
- Prefer the smallest relevant validation command for the area you changed.
- In hand-off, state the risk class and any residual UX or contract risk that was not verified, using concrete paths instead of generic caveats.
