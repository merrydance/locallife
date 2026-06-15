# Production Risk Audit Flow Inventory

Date: 2026-06-14
Source set: `artifacts/codegraph/**/*.slice.md` with matching `*.edges.json`
Total reusable flows: 40
Status: all reusable flows inventoried for six-phase documentation audit

## Method

This inventory is the durable coverage matrix for the long-running production
risk audit described in
`artifacts/production-risk-audit-and-refactor-charter-2026-06-14.md`.

The audit remains documentation-only:

- No production Go, SQL, frontend, Flutter, generated, migration, or config
  source is changed by this pass.
- Codegraph slices are used as reviewed discovery and evidence indexes.
- The current runtime source remains the stronger truth source before any
  future production-code change.
- Runtime tests are not implied by a ledger entry unless a command is listed in
  that ledger's validation notes.

## Phase Coverage Legend

- `source-audited`: the phase has a dedicated source-level flow audit document.
- `slice-ledgered`: the phase has been documented from the reviewed codegraph
  slice and its evidence anchors, without fresh package test execution.
- `read-only`: the flow has no production-code change from this audit pass.

## Flow Matrix

| # | Flow Key | Risk | Slice | Edges | State | Idempotency | Authorization | Transaction | External | Release |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | BAOFU-AGGREGATE-PAYMENT | G3 | `artifacts/codegraph/backend/baofu-payment/aggregate-payment.slice.md` | `artifacts/codegraph/backend/baofu-payment/aggregate-payment.edges.json` | source-audited | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 2 | BAOFU-REFUND | G3 | `artifacts/codegraph/backend/baofu-payment/refund.slice.md` | `artifacts/codegraph/backend/baofu-payment/refund.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 3 | CUSTOMER-DINE-IN-CHECKOUT | G3 | `artifacts/codegraph/customer-state-flows/customer-dine-in-session-menu-checkout.slice.md` | `artifacts/codegraph/customer-state-flows/customer-dine-in-session-menu-checkout.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 4 | CUSTOMER-DISCOVERY-BROWSE | G2 | `artifacts/codegraph/customer-state-flows/customer-discovery-and-merchant-browse.slice.md` | `artifacts/codegraph/customer-state-flows/customer-discovery-and-merchant-browse.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 5 | CUSTOMER-ORDER-AFTER-SALES | G3 | `artifacts/codegraph/customer-state-flows/customer-order-tracking-refund-after-sales.slice.md` | `artifacts/codegraph/customer-state-flows/customer-order-tracking-refund-after-sales.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 6 | CUSTOMER-PROFILE-WALLET | G2/G3 | `artifacts/codegraph/customer-state-flows/customer-profile-address-wallet-membership-reviews.slice.md` | `artifacts/codegraph/customer-state-flows/customer-profile-address-wallet-membership-reviews.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 7 | CUSTOMER-RESERVATION | G3 | `artifacts/codegraph/customer-state-flows/customer-reservation-lifecycle.slice.md` | `artifacts/codegraph/customer-state-flows/customer-reservation-lifecycle.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 8 | CUSTOMER-RUNTIME-AUTH | G3 | `artifacts/codegraph/customer-state-flows/customer-runtime-auth-session-support.slice.md` | `artifacts/codegraph/customer-state-flows/customer-runtime-auth-session-support.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 9 | CUSTOMER-TAKEOUT-CHECKOUT | G3 | `artifacts/codegraph/customer-state-flows/customer-takeout-cart-checkout-payment.slice.md` | `artifacts/codegraph/customer-state-flows/customer-takeout-cart-checkout-payment.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 10 | MERCHANT-APP-BIND | G3 | `artifacts/codegraph/merchant-state-flows/merchant-app-bind-and-device.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-app-bind-and-device.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 11 | MERCHANT-ONBOARDING | G3 | `artifacts/codegraph/merchant-state-flows/merchant-application-onboarding.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-application-onboarding.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 12 | MERCHANT-BIZ-HOURS-AUTO | G2 | `artifacts/codegraph/merchant-state-flows/merchant-business-hours-auto-open.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-business-hours-auto-open.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 13 | MERCHANT-CLAIM-RECOVERY | G3 | `artifacts/codegraph/merchant-state-flows/merchant-claim-recovery.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-claim-recovery.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 14 | MERCHANT-COMBO-CATALOG | G2 | `artifacts/codegraph/merchant-state-flows/merchant-combo-and-catalog.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-combo-and-catalog.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 15 | MERCHANT-DEVICE-DISPLAY | G2/G3 | `artifacts/codegraph/merchant-state-flows/merchant-device-display-config.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-device-display-config.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 16 | MERCHANT-DISH-INVENTORY | G2 | `artifacts/codegraph/merchant-state-flows/merchant-dish-status-and-inventory.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-dish-status-and-inventory.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 17 | MERCHANT-FINANCE-WITHDRAWAL | G3 | `artifacts/codegraph/merchant-state-flows/merchant-finance-withdrawal.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-finance-withdrawal.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 18 | MERCHANT-MANUAL-OPEN | G2 | `artifacts/codegraph/merchant-state-flows/merchant-manual-open-status.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-manual-open-status.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 19 | MERCHANT-MARKETING-RULES | G2 | `artifacts/codegraph/merchant-state-flows/merchant-marketing-rules.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-marketing-rules.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 20 | MERCHANT-MEMBER-BALANCE | G3 | `artifacts/codegraph/merchant-state-flows/merchant-member-balance-adjust.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-member-balance-adjust.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 21 | MERCHANT-MEMBERSHIP-SETTINGS | G2 | `artifacts/codegraph/merchant-state-flows/merchant-membership-settings.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-membership-settings.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 22 | MERCHANT-ORDER-OPS | G3/G2 | `artifacts/codegraph/merchant-state-flows/merchant-order-operations.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-order-operations.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 23 | MERCHANT-PROFILE-UPDATE | G2 | `artifacts/codegraph/merchant-state-flows/merchant-profile-update.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-profile-update.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 24 | MERCHANT-RESERVATION-TABLE | G3 | `artifacts/codegraph/merchant-state-flows/merchant-reservation-and-table.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-reservation-and-table.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 25 | MERCHANT-REVIEW-REPLY | G2 | `artifacts/codegraph/merchant-state-flows/merchant-review-reply.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-review-reply.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 26 | MERCHANT-STAFF-GROUP | G3 | `artifacts/codegraph/merchant-state-flows/merchant-staff-and-group.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-staff-and-group.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 27 | MERCHANT-STATS-ANALYTICS | G2 | `artifacts/codegraph/merchant-state-flows/merchant-stats-and-analytics.slice.md` | `artifacts/codegraph/merchant-state-flows/merchant-stats-and-analytics.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 28 | OPERATOR-DASHBOARD | G2 | `artifacts/codegraph/operator-state-flows/operator-dashboard-analytics-notifications.slice.md` | `artifacts/codegraph/operator-state-flows/operator-dashboard-analytics-notifications.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 29 | OPERATOR-DISPATCH-HALL | G2/G3 | `artifacts/codegraph/operator-state-flows/operator-dispatch-hall.slice.md` | `artifacts/codegraph/operator-state-flows/operator-dispatch-hall.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 30 | OPERATOR-FINANCE-WITHDRAWAL | G3 | `artifacts/codegraph/operator-state-flows/operator-finance-and-baofu-withdrawal.slice.md` | `artifacts/codegraph/operator-state-flows/operator-finance-and-baofu-withdrawal.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 31 | OPERATOR-MERCHANT-MGMT | G2 | `artifacts/codegraph/operator-state-flows/operator-merchant-management.slice.md` | `artifacts/codegraph/operator-state-flows/operator-merchant-management.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 32 | OPERATOR-REGION-RULES | G2/G3 | `artifacts/codegraph/operator-state-flows/operator-region-rules-and-expansion.slice.md` | `artifacts/codegraph/operator-state-flows/operator-region-rules-and-expansion.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 33 | OPERATOR-RIDER-MGMT | G2 | `artifacts/codegraph/operator-state-flows/operator-rider-management.slice.md` | `artifacts/codegraph/operator-state-flows/operator-rider-management.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 34 | OPERATOR-SAFETY-RECOVERY | G3 | `artifacts/codegraph/operator-state-flows/operator-safety-and-recovery.slice.md` | `artifacts/codegraph/operator-state-flows/operator-safety-and-recovery.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 35 | RIDER-ONBOARDING | G3 | `artifacts/codegraph/rider-state-flows/rider-application-onboarding.slice.md` | `artifacts/codegraph/rider-state-flows/rider-application-onboarding.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 36 | RIDER-CLAIMS-RECOVERY | G3 | `artifacts/codegraph/rider-state-flows/rider-claims-and-recovery.slice.md` | `artifacts/codegraph/rider-state-flows/rider-claims-and-recovery.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 37 | RIDER-DELIVERY-LIFECYCLE | G3 | `artifacts/codegraph/rider-state-flows/rider-delivery-lifecycle.slice.md` | `artifacts/codegraph/rider-state-flows/rider-delivery-lifecycle.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 38 | RIDER-DEPOSIT | G3 | `artifacts/codegraph/rider-state-flows/rider-deposit.slice.md` | `artifacts/codegraph/rider-state-flows/rider-deposit.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 39 | RIDER-INCOME-WITHDRAWAL | G3 | `artifacts/codegraph/rider-state-flows/rider-income-and-baofu-withdrawal.slice.md` | `artifacts/codegraph/rider-state-flows/rider-income-and-baofu-withdrawal.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |
| 40 | RIDER-WORKBENCH-LOCATION | G3 | `artifacts/codegraph/rider-state-flows/rider-workbench-status-location.slice.md` | `artifacts/codegraph/rider-state-flows/rider-workbench-status-location.edges.json` | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered | slice-ledgered |

## Durable Theme Ledgers

- State sequencing: `artifacts/production-risk-audit/state-sequencing-ledger-2026-06.md`
- Idempotency and retry: `artifacts/production-risk-audit/idempotency-retry-ledger-2026-06.md`
- Authorization boundaries: `artifacts/production-risk-audit/authorization-boundary-ledger-2026-06.md`
- Transaction consistency: `artifacts/production-risk-audit/transaction-consistency-ledger-2026-06.md`
- External dependencies: `artifacts/production-risk-audit/external-dependency-ledger-2026-06.md`
- Release configuration: `artifacts/production-risk-audit/release-configuration-ledger-2026-06.md`

## Detailed Flow Materials

- `artifacts/production-risk-audit/flows/state-sequencing-baofu-order-payment-success-2026-06-14.md`
- `artifacts/production-risk-audit/flows/external-dependency-baofu-provider-evidence-gate-2026-06-15.md`
- `artifacts/production-risk-audit/flows/state-sequencing-customer-dine-in-checkout-convergence-2026-06-15.md`
- `artifacts/production-risk-audit/flows/state-sequencing-customer-takeout-checkout-rehydration-payment-2026-06-15.md`
- `artifacts/production-risk-audit/flows/idempotency-merchant-order-actions-concurrent-validation-2026-06-15.md`
- `artifacts/production-risk-audit/flows/release-scheduler-worker-readiness-gate-2026-06-15.md`

These additional 2026-06-15 materials are execution cards/gates for the five
remaining real follow-up areas from the seven-issue sync. They do not change
production code and do not by themselves promote the full flow matrix cells from
`slice-ledgered` to source-audited; use them as the next source-audit targets
before production-code changes.
