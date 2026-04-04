---
applyTo: "weapp/miniprogram/components/**"
---

# Mini Program Components Instructions

Apply these rules for files under `weapp/miniprogram/components/`.

## Role Of This Layer

- Keep this directory for reusable Mini Program components and presentation-level interaction building blocks.
- Do not move page-only business workflows or service orchestration into shared components.

## Component Rules

- Prefer TDesign Miniprogram primitives and existing design tokens first; only create a local component or wrapper when TDesign lacks the required capability.
- Do not override TDesign internal interaction, built-in affordances, or state behavior unless the change is required by a verified platform limitation or a missing component capability.
- Keep component APIs stable, role-agnostic where practical, and easy to reuse across pages.
- Shared components should expose predictable props and events rather than relying on hidden page assumptions.
- Loading, empty, placeholder, and disabled states should be explicit when the component semantics require them.

## Styling Rules

- Use token-based spacing, radius, and color values from the design system instead of hardcoded one-off values.
- Restrict TDesign styling changes to theme tokens, spacing, safe-area handling, and outer layout composition; do not restyle internal structure for local visual preference.
- Avoid leaking business-specific appearance into generic components unless the component is intentionally domain-scoped.

## Validation Defaults

- Prefer `npm run quality:check` after changing shared Mini Program components because the blast radius can affect many pages.