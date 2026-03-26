---
applyTo: "weapp/miniprogram/components/**"
---

# Mini Program Components Instructions

Apply these rules for files under `weapp/miniprogram/components/`.

## Role Of This Layer

- Keep this directory for reusable Mini Program components and presentation-level interaction building blocks.
- Do not move page-only business workflows or service orchestration into shared components.

## Component Rules

- Prefer TDesign Miniprogram primitives and the existing design tokens before adding custom structural styles.
- Keep component APIs stable, role-agnostic where practical, and easy to reuse across pages.
- Shared components should expose predictable props and events rather than relying on hidden page assumptions.
- Loading, empty, placeholder, and disabled states should be explicit when the component semantics require them.

## Styling Rules

- Use token-based spacing, radius, and color values from the design system instead of hardcoded one-off values.
- Avoid leaking business-specific appearance into generic components unless the component is intentionally domain-scoped.

## Validation Defaults

- Prefer `npm run quality:check` after changing shared Mini Program components because the blast radius can affect many pages.