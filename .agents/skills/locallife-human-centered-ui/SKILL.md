---
name: locallife-human-centered-ui
description: Use when designing, implementing, or reviewing LocalLife frontend UI for web, weapp, or merchant_app where user habits, task fit, information architecture, interaction flow, ViewState, API flattening, or "does this feel like it understands the user" matters.
---

# LocalLife Human-Centered UI

## Purpose

Use this skill to translate user research signals, role habits, and task context into frontend design decisions before selecting components or styling. It complements `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`; it does not replace Web, Mini Program, or Flutter standards.

## Required Routing

1. Follow the repository routing first: `AGENTS.md`, `.github/copilot-instructions.md`, `.github/README.md`, and the matching `.github/instructions/*.instructions.md`.
2. Load this skill when the task touches a user-facing flow, management page, page redesign, UI review, API-flattening risk, or interaction complaint.
3. Then load the smallest reference below that matches the surface:
   - Web operator/platform admin: `references/operator-admin.md`
   - Web or app merchant operations: `references/merchant-admin.md`
   - Mini Program/mobile touch flows: `references/weapp-mobile.md`
   - Turning research, feedback, or business notes into UI decisions: `references/research-to-ui-checklist.md`
   - Findings-first UI review: `references/review-rubric.md`

## Core Workflow

Before changing UI structure, answer these in the working notes or hand-off:

1. **Role and moment**: Who is using this, and what is happening around them right now?
2. **Primary task**: What are they trying to finish, decide, recover, or monitor?
3. **Habit path**: What should be remembered, defaulted, grouped, prefilled, or one-click because users repeat it?
4. **First-screen judgment**: What must be visible immediately for a correct decision?
5. **Risk and recovery**: What can go wrong, how costly is it, and how does the user recover without guessing?
6. **ViewState**: Model loading, empty, error, stale data, disabled, submitting, success, unknown result, retry, and re-entry.
7. **Implementation fit**: Only after the above, choose page boundaries, component boundaries, controls, copy, and visual treatment under the matching `.github` standards.

## Output Expectations

For implementation, include a compact **Human-Centered UI Check**:

- Role and primary task
- High-frequency path and default assumptions
- First-screen information priority
- State to preserve on refresh, tab switch, or return
- Failure/recovery paths that must be visible
- Non-goals or low-frequency actions that should not crowd the main flow

For review, report user-task and habit-fit defects before cosmetic issues when they affect page structure, action priority, recovery, or API flattening.

## Guardrails

- Do not invent backend truth. Backend contracts remain the source for fields, status, permissions, money, and final result semantics.
- Do not add explanatory cards to compensate for weak information architecture. Fix structure, labels, state, and action hierarchy first.
- Do not let low-frequency capabilities crowd the high-frequency task path.
- Do not treat UI/UX style libraries as page-architecture authority. They may inform visual polish after LocalLife task modeling is complete.
