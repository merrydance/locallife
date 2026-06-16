# Dine-In Checkout Device E2E Evidence Template

Date: 2026-06-15
Risk theme: Mini Program device re-entry / payment result recovery
Risk class: G3 - paid dine-in session close after WeChat payment result re-entry
Status: template only; no device run recorded in this file

## Purpose

Use this template for the actual Mini Program device or WeChat DevTools
end-to-end run that proves pending dine-in checkout context survives payment
result reload and paid-status polling.

Do not mark the device/E2E follow-up closed unless a filled evidence file
passes:

```bash
cd weapp
npm run check:dine-in-device-e2e-evidence -- ../artifacts/production-risk-audit/flows/dine-in-checkout-device-e2e-evidence-YYYY-MM-DD.md
```

## Device And Build

Device model: record the physical device model or WeChat DevTools profile.
WeChat version: record the WeChat version used for the run.
Mini Program build: record develop/trial/release build identifier.
Backend environment: record local/staging/release target and base URL.
Operator: record the person or release owner who ran the proof.

## Fixture Data

Session ID: record the dining session id created by QR scan/open session.
Order ID: record the dine-in order id created by checkout.
Payment Order ID: record the payment order id shown on the result route.

## Execution Evidence

1. QR scan/open session: record pass or fail and the observed page/session state.
2. Dine-in checkout saves pending context: record pass or fail and how the
   pending checkout context was observed.
3. Payment result reload while pending_confirmation: record pass or fail and
   confirm polling resumes after reload or foreground re-entry.
4. Backend payment reaches paid: record pass or fail and the backend/source of
   payment truth.
5. Paid polling triggers session checkout: record pass or fail and the UI state
   after the paid result is observed.
6. Backend session readback is closed/non-actionable: record pass or fail and
   the backend readback used to prove session convergence.
Screenshot or recording evidence: record the evidence path or artifact id.
Backend verification: record the backend query, API readback, or log/event id
that proves the dining session is closed or no longer actionable.

## Result

Verdict: fail

If the verdict is `pass`, the filled file must include concrete device/build,
fixture ids, screenshot or recording evidence, and backend verification. Keep
this template as `fail`; create a separate filled evidence file for each real
device run.
