# Mini Program Review Template

Use this template when asking for a Mini Program review in `weapp/`.

## Mini Program Review

Request:

- Review this change with findings first, ordered by severity
- Check it against `.github/standards/weapp/DESIGN_SYSTEM.md`
- Prioritize broken service-to-state-to-view wiring, missing page states, token violations, and debug leftovers
- Flag business styles leaking into shared global styles or shared components
- If there are no findings, say so explicitly and mention residual risks

Required context:

- Changed page or component paths: <paths>

Optional context:

- Expected behavior: <details>
- Reference page or component: <path>
- Validation evidence already run: <commands or none>

Review checklist:

- New fields and actions propagate through service layer, state, handlers, and visible UI
- App Shell structure remains stable during loading and error states
- TDesign or existing shared components were used where appropriate
- User-facing copy and affordances are clear in weak-network and empty-data scenarios