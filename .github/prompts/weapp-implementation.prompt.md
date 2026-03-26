# Mini Program Implementation Template

Use this template when asking for a concrete Mini Program change in `weapp/`.

## Mini Program Page Or Component Change

Request:

- Update <page or component>
- Follow `.github/standards/weapp/DESIGN_SYSTEM.md` and reuse existing TDesign-based patterns first
- Keep business-specific styles out of global styles unless they are truly shared
- Run the smallest relevant validation command and report what was executed

Required context:

- Target page or component path: <path>
- Desired behavior or UX change: <details>

Optional context:

- Existing reference page or component: <path>
- Related service or API change: <details>

Acceptance checklist:

- Page shell stays stable before data returns; no full-page white flash
- Loading, success, empty, and error states are all defined where relevant
- New fields or actions are wired through service calls, page state, handlers, and user-visible feedback
- Token-based spacing, radius, and color variables are used instead of hardcoded values
- Shared component boundaries remain clean and business styles do not leak globally