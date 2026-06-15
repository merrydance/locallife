# Customer Takeout Provider E2E Evidence Template

Date: 2026-06-15
Risk theme: takeout checkout provider callback/recovery and visible status
Risk class: G3 - customer order payment callback, result re-entry, detail/list visibility
Status: template only; no provider run recorded in this file

## Purpose

Use this template for the actual Mini Program device or WeChat DevTools run that
proves a takeout checkout can leave or reload the payment result page while
Baofu callback/query evidence advances backend truth and order detail/list
remain visible.

Do not mark the takeout provider/E2E follow-up closed unless a filled evidence
file passes:

```bash
cd weapp
npm run check:customer-checkout-provider-e2e-evidence -- ../artifacts/production-risk-audit/flows/customer-takeout-provider-e2e-evidence-YYYY-MM-DD.md
```

## Target Flow

Flow type: takeout

## Device And Build

Device model: record the physical device model or WeChat DevTools profile.
WeChat version: record the WeChat version used for the run.
Mini Program build: record develop/trial/release build identifier.
Backend environment: record staging/release target and base URL.
Operator: record the person or release owner who ran the proof.

## Fixture Data

Order or reservation ID: record the takeout order id.
Payment Order ID: record the payment order id shown on the result route.
Provider fact ID: record the external payment fact id.
Provider application ID: record the external payment fact application id.

## Provider Evidence

Callback or query evidence: record the callback/query fact and terminal status.
Baofu evidence gate command: record the exact `scripts/baofu_provider_evidence_gate.sh`
command used with masked notes and the LocalLife callback/query endpoint.

## Recovery And Visibility

1. Client leaves or reloads payment result: record pass/fail and the observed route.
2. Backend payment truth reaches terminal state: record pass/fail and source of truth.
3. Detail page readback: record pass/fail and the order status shown.
4. List page readback: record pass/fail and the list status shown.
Screenshot or recording evidence: record the evidence path or artifact id.
Backend verification: record the backend query, API readback, or log/event id.

## Result

Verdict: fail

If the verdict is `pass`, the filled file must include concrete device/build,
fixture ids, provider fact/application ids, the Baofu evidence gate command,
detail/list readback, screenshot or recording evidence, and backend
verification. Keep this template as `fail`; create a separate filled evidence
file for each real provider/device run.
