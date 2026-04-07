---
name: "Mini Program Page Refactor Blueprint"
description: "Use when drafting a Mini Program page or component refactor request that should be driven by backend truth, user-task boundaries, information architecture, and a step-by-step implementation path before coding. Trigger phrases: refactor Mini Program page, refactor component, redesign page, rebuild dashboard, clean up page IA, fix stats semantics, diagnose page before implementation."
routing-hints: "重构蓝图|refactor mini program page|setData 热点|weak-network UX blueprint|diagnose page before implementation|clean up page IA"
---
Generate an executable refactor brief for a specific Mini Program page or component. Do not start with visuals. Do not start with code. Do not assume API semantics. Verify facts first, define boundaries second, then produce the solution and implementation steps.

Scope:
- This is a prompt, not an agent mode
- It applies to any Mini Program page or component under `weapp/` that needs a systematic refactor
- This includes, but is not limited to: consumer pages, merchant pages, operator pages, platform pages, rider pages, stats pages, home pages, dashboards, entry-aggregation pages, management pages, form pages, detail pages, list pages, flow pages, and result pages
- If you only need a read-only diagnosis and not a refactor plan, use a read-only audit agent instead
- If the task is only a small copy tweak or a minor style fix, this blueprint is unnecessary

Core methodology:
- User task first, page structure second. Define what the user must accomplish on the page before deciding what the page should contain.
- The backend is the only source of truth. Capabilities, fields, states, permissions, pagination semantics, and metric definitions must come from the real backend contract, not from old frontend code, old copy, stale type definitions, or current page behavior.
- Backend truth first, frontend expression second. Every metric, state, entry, label, and sorting statement must be aligned with the real backend source.
- Information architecture first, component details second. Define first-screen hierarchy, sections, and primary tasks before selecting components.
- TDesign first, page styling minimal. Use TDesign MCP to find components by purpose and category first, then choose the best-fitting existing components and tokens. Do not default to only a few familiar components. Keep only the thinnest necessary page shell and local styles.
- Refactor the whole path together. Service calls, page state, event handlers, WXML, WXSS, loading states, error states, empty states, and refresh behavior must all be aligned together.
- Do a semantic sweep last. Titles, subtitles, buttons, notes, sorting labels, and percentage copy must be checked one by one against the real data source.

Prohibitions:
- Do not invent backend fields, states, capabilities, or metric semantics
- Do not use a counting endpoint as if it were a money endpoint
- Do not collapse `final_amount`, `platform_commission`, and `order_items.subtotal` into the same business concept
- Do not force future capabilities, cross-role capabilities, or unfinished capabilities into the current page
- Do not change only WXML or WXSS without updating service, state, handlers, and feedback behavior
- If backend sources are ambiguous, conflicting, missing, or insufficient to support the intended UX, do not guess on the frontend. Raise the issue explicitly and state whether backend changes are required.

Use the exact structure below:

## Background
- Target page or component path: <required>
- Target user or role: <consumer / merchant / operator / platform / rider / other>
- Primary page task: <required>
- Main current problems: <required>
- Related APIs / services / components: <optional>
- Success condition: <what should become faster, more accurate, or more reliable for the user>

## User Tasks And Page Boundaries
- State the 1 to 3 most important tasks the user must complete on this page
- State what must be visible on the first screen
- State which capabilities should remain on the current page and which should move into subcomponents, child pages, or separate flows
- State what this refactor should deliberately keep out of scope to avoid page overload, boundary drift, or tangled flows

## Fact Verification
- List the real backend sources involved in this page: routes, handlers, typed services, and when necessary SQL or aggregation sources
- For every core metric, state, entry, sorting rule, or explanatory note, provide:
  - Current page expression
  - Actual backend source
  - Real backend meaning
  - Whether drift or misleading semantics exist
- If the source itself is ambiguous, conflicting, or missing, or if the backend does not provide the capability required by the page, list it explicitly as a backend contract problem instead of forcing frontend-only consistency
- Explicitly check these high-risk areas:
  - Frontend type drift
  - References to backend fields that do not actually exist
  - Copy that silently changes business semantics
  - Stats cards that mix data from incompatible sources
  - Sorting labels, subtitles, and percentage statements that do not match the SQL or aggregation logic

## Diagnosis
- List the main problems
- For each problem, include:
  - Problem description
  - Risk level
  - User-facing impact
  - Maintenance impact
- If the root cause is backend contract ambiguity, missing capability, misleading field naming, or a backend semantics gap, label it explicitly as requiring backend decision or backend changes rather than disguising it as a frontend-only issue
- The diagnosis must cover:
  - User or role boundaries
  - Information architecture
  - Backend contract alignment
  - State design
  - Interaction efficiency
  - Failure and recovery behavior
  - Component boundaries
  - Styling maintenance cost

## High-Quality Solution
- Explain the refactor direction by module or page area
- At minimum cover:
  - What the core regions of the page should contain
  - How first-screen key information, metrics, or actions should be selected, named, and explained
  - How content areas, action areas, entry areas, list areas, form areas, or result areas should be organized
  - How low-frequency capabilities should be demoted, split out, or moved off the page
  - How loading, success, empty, error, and refresh states should be structured
  - How weak-network behavior, retry, foreground re-entry, and duplicate-tap protection should work
  - Which parts should be matched to TDesign components through TDesign MCP, which parts should rely on default TDesign behavior, and which parts should keep only the thinnest local styling

## Data And Copy Alignment Rules
- For every key metric, specify:
  - Display name
  - Backend source
  - Real business meaning
  - Allowed wording
  - Forbidden misleading wording
- For every list or stats module, specify:
  - Sorting basis
  - Money-field source
  - Percentage basis
  - Notes that must be explicitly shown on the page

## Implementation Steps
- Step 1: User tasks and information architecture convergence
  - Define the page's primary task, current-page boundary, first-screen essentials, and what should move to child pages or child flows
- Step 2: Backend contract and metric verification
  - Verify routes, DTOs, services, and when needed SQL, then remove frontend type drift and API misuse
- Step 3: Page skeleton and section design
  - Define the page shell, top regions, card hierarchy, and section structure before selecting components and styles
- Step 4: TDesign-first implementation
  - Use TDesign MCP to find the most suitable components by purpose and category first, then map the solution to concrete components. Reuse existing components, icons, and tokens. Avoid both over-localized styling and over-reliance on only a few familiar components.
- Step 5: End-to-end propagation
  - Update services, state, handlers, WXML, WXSS, feedback behavior, and recovery paths together
- Step 6: Semantic sweep
  - Recheck titles, subtitles, buttons, notes, sorting labels, money names, and percentage copy against the real source of truth
- Step 7: Gate validation
  - Run the smallest relevant validation commands and confirm that page shell, request boundary, role contract, business status boundary, and other relevant gates still pass

## Non-Goals
- State what this refactor does not change
- State which identified opportunities are intentionally deferred
- If progress is blocked by missing backend truth, explicitly state which parts the frontend should not fake or force through in this round

## Implementation Requirements
- The final output must be production-ready code, not a concept sketch
- Loading, success, empty, error, and refresh states must be covered where relevant
- First-screen request scope and unnecessary concurrency must be called out explicitly
- setData granularity, duplicate-tap protection, foreground re-entry refresh, and retry paths must be checked
- The delivery notes must explicitly state which backend endpoint or field each key metric comes from
- If ambiguity or missing backend sources are found, the delivery notes must explicitly state the gap, why the frontend cannot safely guess, and whether backend changes are recommended
- Run the smallest relevant validation commands and report the results

Additional requirements:
- If the target is a home page, dashboard, or management surface, explicitly check whether entry density is too high
- If the target is a stats page, explicitly check money semantics, sorting semantics, percentage copy, and explanatory notes
- If the target is a form page, submission flow page, or result page, explicitly check draft preservation, in-flight state, duplicate submission protection, and failure recovery
- If the target is a list page or detail page, explicitly check first-screen request scope, pagination truth, back-navigation continuity, and empty-state semantics
- If the target shares patterns across roles, explicitly check whether role boundaries have been broken
- Even if no major problem is found, still report residual risks and improvement opportunities