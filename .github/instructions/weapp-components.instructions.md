---
applyTo: "weapp/miniprogram/components/**"
---

# Mini Program Components Instructions

Apply these rules for files under `weapp/miniprogram/components/`.

## Role Of This Layer

- Keep this directory for reusable Mini Program components and presentation-level interaction building blocks.
- Do not move page-only business workflows or service orchestration into shared components.

## Component Contract Rules

- Prefer TDesign Miniprogram primitives and existing design tokens first; only create a local component or wrapper when TDesign lacks the required capability.
- Do not override TDesign internal interaction, built-in affordances, or state behavior unless the change is required by a verified platform limitation or a missing component capability.
- Keep component APIs stable, role-agnostic where practical, and easy to reuse across pages.
- Shared components should expose predictable props and events rather than relying on hidden page assumptions.
- Loading, empty, placeholder, and disabled states should be explicit when the component semantics require them.
- Shared components should not hide page-level API calls, route jumps, or role-specific state machines behind implicit side effects.
- Prefer explicit prop names and event names that describe semantics, not page-specific implementation details.
- If a component supports async action affordances, its loading and disabled behavior must be externally controllable.

## State Semantics

- A reusable component should make it obvious which state it is rendering: default, loading, empty, disabled, error, or selected.
- Do not overload one boolean flag to represent unrelated states such as “loading or empty” or “disabled or submitting”.
- If a component renders fallback copy or empty affordances, keep them generic enough to remain reusable.

## Styling Rules

- Use token-based spacing, radius, and color values from the design system instead of hardcoded one-off values.
- Restrict TDesign styling changes to theme tokens, spacing, safe-area handling, and outer layout composition; do not restyle internal structure for local visual preference.
- Avoid leaking business-specific appearance into generic components unless the component is intentionally domain-scoped.
- Main tap targets, icon buttons, and action rows must preserve usable touch area; avoid tiny click regions for important actions.
- Long labels and mixed icon-text layouts must degrade gracefully without breaking alignment or clipping interactive content.

## Validation Defaults

- Prefer `npm run quality:check` after changing shared Mini Program components because the blast radius can affect many pages.