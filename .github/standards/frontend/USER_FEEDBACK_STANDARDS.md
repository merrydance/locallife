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
- If a page already shows the result clearly after refresh, jump, return, tab switch, or state update, do not add a success Toast.
- Do not keep artificial delays only to let a success Toast stay visible before navigation.

### 2.2 Do Not Leak Raw Backend Errors

- Do not display raw backend `message`, English errors, SQL errors, provider diagnostics, gateway text, stack text, or internal field names in UI.
- Map failures into user-facing Chinese business copy first, then show them in the correct feedback channel.
- Technical detail belongs only in logs, monitoring, and debugging tools.

### 2.3 Feedback Channel Selection

- Toast: short, lightweight feedback for actions that stay on the current page and do not already have obvious structural feedback.
- Modal: confirmation, explanation, authorization guidance, destructive actions, or next-step instructions.
- Inline/page state: first-screen failure, refresh failure, weak-network retry, persistent action results, or any result that should remain visible after the action completes.
- Loading: processing only. Loading must not also carry final business meaning.

### 2.4 Do Not Stack Banner-Like Surfaces

- Top banners, inline banners, page notices, and similar strip-style prompt surfaces count as the same prompt family.
- Do not combine top banner, inline banner, Toast, and Modal for the same event.
- For Mini Program, do not use banner-style surfaces as the default primary feedback channel for transient action results.
- If a persistent result must remain visible, prefer a dedicated page state or result block over temporary banner stacking.

### 2.5 Persistent State Beats Ephemeral Toast

- If the page exposes the result through status text, amount changes, list mutations, steps, banners, badges, tabs, or result pages, prefer that structure over success Toast.
- If the action result should still be visible after several seconds, use inline state or a result page, not a disappearing Toast.
- If an operation is asynchronous and the final outcome will sync later, at most show one short acknowledgement such as “已提交，稍后同步”, then let page state take over.

## 3. Success Feedback Rules

### 3.1 Keep Success Toast Only When All Conditions Hold

- The user stays on the current page.
- The page does not already show a strong visual result.
- The action is lightweight or transient.
- The feedback fits within one short sentence.

Typical keep cases:

- add to cart while staying on the same page
- copy success
- send test command
- call waiter

### 3.2 Remove Success Toast When Any of These Happens

- the action immediately navigates, redirects, returns, or switches tabs
- the target page is itself a result surface
- the current page immediately reloads and the updated data already explains the result
- the current page already has an inline banner, result block, stepper, balance card, or status area that updates after success

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
- Is any success Toast redundant with navigation, refresh, result page, or visible state change?
- Can the user see any raw backend or English diagnostic text?
- Does first-screen failure have inline recovery instead of Toast-only handling?
- Are destructive or confirmatory flows using Modal rather than Toast?
- Are long-lived results kept visible through page structure rather than disappearing feedback?

## 6. Implementation Guidance By Surface

### 6.1 Mini Program

- Follow this standard together with the Mini Program design and prompt docs.
- Use shared utilities for error mapping and prompt dedup where available.
- Prefer dedicated page states or result blocks over banner-style prompts when the page already updates structurally.
- For transient action feedback in Mini Program, default to Toast or Modal instead of top-banner or inline-banner stacking.

### 6.2 Web

- Follow this standard together with Web UI standards and design guardrails.
- Use the existing alert, banner, dialog, and page-shell patterns instead of inventing ad-hoc feedback widgets.
- Keep backend enum values and errors mapped into business-readable copy before rendering.

## 7. Authority Relationship

- This file is the shared frontend authority for prompt and error feedback behavior.
- End-specific docs should add concrete examples, utilities, and exceptions for their runtime.
- If a Web or Mini Program rule conflicts with this file, align the end-specific rule instead of bypassing the system standard.