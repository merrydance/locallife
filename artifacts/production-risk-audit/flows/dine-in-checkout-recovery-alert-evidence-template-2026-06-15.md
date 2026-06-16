# Dine-In Checkout Recovery Alert Evidence Template

Date: 2026-06-15
Risk theme: release configuration / recovery observability
Risk class: G3 - paid dine-in session recovery failure alerting
Status: template only; no target alert run recorded in this file

## Purpose

Use this template for the actual target-environment alert proof that repeated
dine-in checkout recovery failures page or notify the responsible route.

Do not mark the target-environment alerting follow-up closed unless a filled
evidence file passes:

```bash
cd weapp
npm run check:dine-in-recovery-alert-evidence -- ../artifacts/production-risk-audit/flows/dine-in-checkout-recovery-alert-evidence-YYYY-MM-DD.md
```

## Alert Target

Alert rule owner: record the team or person accountable for the rule.
Target environment: record staging, release, or the exact monitored target.
Rule config link or evidence id: record the alert-rule path, dashboard/rule
link, screenshot id, or change request id for the target environment.

## Metric Coverage

Metric list_error: dine_in_checkout_recovery_scans_total{result="list_error"}
Metric close_failed: dine_in_checkout_recovery_sessions_total{result="close_failed"}

## Rule Definition

PromQL or equivalent expression: record the exact PromQL, managed-monitor rule,
or vendor-equivalent expression that watches both failure metrics.
Threshold and window: record the threshold, evaluation window, and duration
needed before the route is notified.

## Routing And Ownership

Receiver or on-call route: record the target owner, escalation route, or duty
rotation that receives the alert.
Notification channel: record the concrete paging, chat, incident, or ticket
channel used by the target environment.

## Verification Evidence

Firing or dry-run evidence: record the alert test, dry-run evaluation,
synthetic firing, or controlled evidence id that proves the rule evaluates and
routes.
Backend version or commit: record the backend version, release tag, or commit
that exposed the monitored metrics.

## Result

Verdict: fail

If the verdict is `pass`, the filled file must include concrete rule owner,
target environment, alert expression, receiver, notification route, target rule
config or evidence id, firing/dry-run proof, and backend version. Keep this
template as `fail`; create a separate filled evidence file for each real
target-environment alert run.
