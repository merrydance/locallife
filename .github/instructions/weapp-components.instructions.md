---
applyTo: "weapp/miniprogram/components/**"
---

# Mini Program Components Instructions

Apply these rules for files under `weapp/miniprogram/components/`.

## Role Of This Layer

- Keep this directory for reusable Mini Program components and presentation-level interaction building blocks.
- Do not move page-only business workflows or service orchestration into shared components.

## Core Rules

- Prefer TDesign Miniprogram for generic interaction building blocks. Check the TDesign MCP component groups and docs by use before creating a local shared abstraction.
- Keep component APIs stable, explicit, and reusable. Props and events should describe semantics, not a single page implementation.
- Shared components must not hide page-level API calls, route jumps, or role-specific state machines behind implicit side effects.
- Make state semantics explicit: loading, empty, disabled, error, selected, or placeholder states should be clear, and async action loading/disabled must be externally controllable.

## Validation Defaults

- Prefer `npm run quality:check` after changing shared Mini Program components because the blast radius can affect many pages.