# Frontend User Feedback Standards

> Scope: all user-facing frontend surfaces in this workspace, including Web and Mini Program.
>
> This document defines system-level rules for prompts, errors, loading feedback, and success feedback.
> End-specific documents may specialize these rules, but must not weaken them.

## 1. Goal

Frontend feedback must satisfy two constraints at the same time:

1. One user action should produce one primary prompt.
2. Backend, gateway, database, provider, or English diagnostic errors must never be shown directly to end users.

This is a product and engineering constraint, not a page-level preference.

## 2. System Rules

### 2.1 One Event, One Primary Prompt

- A single user action may only have one primary feedback channel.
- Do not stack Toast, Modal, inline banner, result page, and refreshed page state to describe the same outcome twice.
- Choose the clearest single channel for the current context.
- “Page result” means the user can directly perceive the outcome from visible page structure or data changes, such as status text, amount updates, list mutations, badges, result blocks, steps, or the target page state after jump, return, tab switch, or refresh.
- If structural feedback is weak, delayed, or easy to miss, one success Toast may be kept as the only primary prompt.
- Success Toast duration may be moderately extended when needed, but do not block a natural transition only to force the Toast to linger.

### 2.2 Do Not Leak Raw Backend Errors

- Do not display raw backend `message`, English errors, SQL errors, provider diagnostics, gateway text, stack text, or internal field names in UI.
- Map failures into user-facing Chinese business copy first, then show them in the correct feedback channel.
- Technical detail belongs only in logs, monitoring, and debugging tools.

### 2.3 Feedback Channel Selection

- Toast: short or moderately extended feedback when it is the clearest single confirmation, especially for lightweight actions or subtle state changes that users could otherwise miss.
- Modal: confirmation, explanation, authorization guidance, destructive actions, or next-step instructions.
- Inline/page state: first-screen failure, refresh failure, weak-network retry, persistent action results, or any result that should remain visible and revisitable after the action completes.
- Loading: processing only. Loading must not also carry final business meaning.

### 2.4 Do Not Stack Banner-Like Surfaces

- Top banners, inline banners, page notices, and similar strip-style prompt surfaces count as the same prompt family.
- Do not combine top banner, inline banner, Toast, and Modal for the same event.
- For Mini Program, do not use banner-style surfaces as the default primary feedback channel for transient action results.
- If a persistent result must remain visible, prefer a dedicated page state or result block over temporary banner stacking.

### 2.5 Lasting Visibility Needs Lasting State

- If the result should still be visible after several seconds or be re-checked later, use inline state or a result page instead of relying only on Toast.
- If an operation is asynchronous and the final outcome will sync later, at most show one short acknowledgement such as “已提交，稍后同步”, then let page state take over.

## 3. Success Feedback Rules

### 3.1 Use Success Toast When It Is The Clearest Single Prompt

- Only one primary prompt is shown.
- The Toast is the clearest way to confirm the result in the current context, or structural feedback would be too subtle or easy to miss.
- The feedback fits within one short sentence.
- The duration is long enough to be noticed, but not so long that it turns into flow-blocking noise.

Typical keep cases:

- add to cart while staying on the same page
- copy success
- send test command
- call waiter
- save or apply a subtle state change on the current page when the updated structure is not obvious enough on its own

### 3.2 Replace Success Toast With Another Single Prompt When Any of These Happens

- a Modal, result page, or inline state already fully explains the outcome
- the result must remain visible for several seconds or be revisited later
- immediate navigation would make the Toast effectively invisible or misleading
- multiple prompt surfaces would otherwise describe the same outcome

Typical remove cases:

- pay success then jump to payment success page
- save success then navigate back to a list that already shows new state
- cancel success then refresh the detail page into cancelled state
- submit success then keep a page-level action notice or status card

## 4. Error Feedback Rules

### 4.1 First-Screen And Core Data Failures

- First-screen failure must use a visible page-level error state with retry.
- List and detail refresh failures should preserve the last good data when possible and surface a non-blocking inline error instead of replacing the page with a Toast-only failure.
- Weak-network and retryable failures should give the user a clear retry surface.

### 4.2 Action Failures

- Use a short business-readable message.
- Do not show both inline error state and Toast for the same failure unless they describe different scopes.
- If a Modal is already explaining the failure and next step, do not also fire a Toast with the same message.

## 5. Review Checklist

Use this checklist in frontend PR review:

- Does one action produce only one primary prompt?
- Is any success Toast duplicated by another equally clear prompt surface?
- Can the user see any raw backend or English diagnostic text?
- Does first-screen failure have inline recovery instead of Toast-only handling?
- Are destructive or confirmatory flows using Modal rather than Toast?
- Are long-lived results kept visible through page structure rather than disappearing feedback?

## 6. Implementation Guidance By Surface

### 6.1 Mini Program

- Follow this standard together with the Mini Program interaction, performance, and API contract standards. When runtime prompt integration details matter, check `weapp/miniprogram/utils/user-facing.ts` and `weapp/miniprogram/utils/prompt-feedback.ts`.
- Use shared utilities for error mapping and prompt dedup where available.
- Prefer whichever single prompt channel users can perceive most clearly in context; for long-lived outcomes, use dedicated page states or result blocks.
- For transient action feedback in Mini Program, default to Toast or Modal instead of top-banner or inline-banner stacking.

### 6.2 Web

- Follow this standard together with Web UI standards and design guardrails.
- Use the existing alert, banner, dialog, and page-shell patterns instead of inventing ad-hoc feedback widgets.
- Keep backend enum values and errors mapped into business-readable copy before rendering.

## 7. Authority Relationship

- This file is the shared frontend authority for prompt and error feedback behavior.
- End-specific docs should add concrete examples, utilities, and exceptions for their runtime.
- If a Web or Mini Program rule conflicts with this file, align the end-specific rule instead of bypassing the system standard.