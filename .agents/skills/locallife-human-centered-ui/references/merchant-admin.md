# Merchant Admin

Use for merchant dashboards, order, dish, table, reservation, finance, marketing, catalog, and settings workflows across Web or merchant app surfaces.

## User Model

Merchants and store staff are trying to keep daily operations moving. They prefer direct task completion, clear business wording, safe editing, and defaults that match store routines.

## Design Heuristics

- Start from the shop task: fulfill, edit, publish, reconcile, configure, check, or recover.
- Surface urgent operational state first: new orders, unavailable items, reservation conflicts, payout/reconciliation issues, stock or table changes.
- Use business labels, not platform language. Translate status and errors into what the merchant can do next.
- Default and remember repeated choices: current store, current date, active tab, common category, last filter, draft form, selected time period.
- Keep editing flows safe: show what changes, validate near the field, disable invalid submit, preserve draft on recoverable failure.
- Separate high-frequency operations from setup or edge-case settings.
- Favor one-step or inline fixes for common recoverable problems.

## Anti-Patterns

- Showing every backend field with equal weight.
- Making setup/admin controls compete with daily operations.
- Resetting filters, forms, or selected context after a failed submit.
- Using success feedback that disappears while the changed business state is not visible.
- Hiding validation until final submit when the correction is obvious earlier.

## Implementation Check

- What task is the merchant trying to finish in less than a minute?
- Which choices can be safely defaulted or remembered?
- What data must be visible before a save, publish, cancel, refund-like, or disable action?
- Does failure preserve the user's work and explain the next safe step?
