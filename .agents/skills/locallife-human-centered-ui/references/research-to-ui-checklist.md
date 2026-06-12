# Research To UI Checklist

Use when starting from interviews, complaints, product notes, incident learnings, business requests, or vague "make this easier" UI goals.

## Translate Signals

Capture only what affects design:

- Role: who experiences the problem.
- Moment: when it happens in the user's day or workflow.
- Frequency: first-time, occasional, daily, repeated per order/item/customer.
- Trigger: what brings the user to this page.
- Desired outcome: what "done" means to them.
- Current workaround: what they do when the product fails them.
- Failure cost: delay, money, incorrect status, customer complaint, operational risk, or lost confidence.
- Device/context: desktop scanning, mobile touch, noisy store, weak network, multitasking.

## Turn Into UI Decisions

- Page boundary: one task page, page group, master-detail, wizard, modal, or inline edit.
- Information priority: first-screen must-know, secondary support, hidden/advanced.
- Action priority: primary, secondary, batch, destructive, low-frequency.
- Defaults and memory: filters, tabs, store, date range, draft, selected item, sort.
- ViewState: loading, empty, error, stale, disabled, submitting, success, unknown, retry, re-entry.
- Feedback channel: page state, inline error, dialog, toast, result page. One event gets one primary prompt.
- Copy: business language that tells the user what happened and what to do next.

## Red Flags

- The solution starts from endpoint count, DTO groups, or existing DOM/WXML/Widget tree.
- The design needs a top explanatory card to make page boundaries understandable.
- High-frequency tasks require repeated manual filtering or retyping.
- Failure loses user input or context.
- The user must infer whether a result is final, pending, failed, or unknown.
