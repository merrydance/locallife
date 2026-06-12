# Operator Admin

Use for operator, platform, audit, rule, review, exception, finance-adjacent, and internal management pages.

## User Model

Operators usually need to scan, compare, decide, route, approve, reject, or investigate. They value density, predictable filters, trustworthy status, and fast recovery more than decorative UI.

## Design Heuristics

- Put the decision object first: status, risk, owner, time, amount, exception reason, next action.
- Preserve repeated work context: filters, selected tab, sort, page, search term, and last selected row when returning.
- Prefer tables for comparable lists and master-detail for inspect-and-act work.
- Make missing records explainable through filters, permissions, empty states, or stale-data state.
- Keep batch actions close to selected rows and disabled until selection and permission are valid.
- Make destructive or irreversible actions explicit with confirmation, reason capture, disabled/in-flight state, and visible post-action state.
- Keep audit/history secondary but discoverable; it should support confidence without blocking the main decision.

## Anti-Patterns

- API field panels with no primary decision or next action.
- Multiple cards that split one continuous review task into unrelated surfaces.
- Filters that reset after every action or return.
- Status badges that expose raw enum names or hide unknown/inconsistent states.
- Toast-only handling for first-screen failure, review failure, or high-impact action failure.

## Implementation Check

- What is the operator's first correct decision on this screen?
- Which filters or table columns match daily triage habits?
- What must stay visible after approve/reject/disable/refresh?
- Can the operator recover from timeout, partial failure, or stale data without leaving the task?
