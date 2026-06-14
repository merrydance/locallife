# Merchant State Flows Codegraph

Status: created 2026-06-12
Source: merchant-side codegraph slices and code-review alignment
Scope: merchant Mini Program, App, and merchant-facing backend flows only

This directory keeps the merchant-side closure judgment in one place. The detailed evidence stays in the slice files; this README only centralizes the live backlog order so the docs stay readable and do not spread into extra index files.

Before creating or refreshing a merchant slice, use the workflow in
`artifacts/codegraph/README.md`: CodeGraph may be used for discovery and line
anchor drift checks, but the slice and edge artifacts are the durable
LocalLife-aware source of truth after review.

## Slice Map

- `merchant-app-bind-and-device.slice.md`: App bind, device register/heartbeat/unregister, push device truth.
- `merchant-application-onboarding.slice.md`: merchant application draft, OCR, review, approval, re-edit, recovery.
- `merchant-business-hours-auto-open.slice.md`: business-hours editor, auto-open scheduler, merchant open-state publish.
- `merchant-claim-recovery.slice.md`: claim recovery, dispute, payment, release visibility.
- `merchant-combo-and-catalog.slice.md`: combo and catalog state.
- `merchant-device-display-config.slice.md`: display config, printer config, auto-accept, local print events, reconciliation.
- `merchant-dish-status-and-inventory.slice.md`: dish availability and inventory.
- `merchant-finance-withdrawal.slice.md`: settlement-account onboarding, withdrawal, Baofu balance, callback/recovery.
- `merchant-manual-open-status.slice.md`: manual open/close versus automatic business-hours control.
- `merchant-marketing-rules.slice.md`: voucher, discount, delivery promotion, recharge rules.
- `merchant-member-balance-adjust.slice.md`: manual member balance adjustment and related ledger behavior.
- `merchant-membership-settings.slice.md`: balance-payment scene settings and checkout eligibility.
- `merchant-order-operations.slice.md`: order accept/reject/ready/complete, refund, print, and realtime refresh.
- `merchant-profile-update.slice.md`: profile, logo, tags, and shop-image truth.
- `merchant-reservation-and-table.slice.md`: reservations, tables, dining sessions, and related inventory/payment state.
- `merchant-review-reply.slice.md`: merchant reply to reviews.
- `merchant-staff-and-group.slice.md`: staff invite/bind/role/remove and group onboarding/join.

## Core Backlog

No remaining merchant-side code/test repair items are counted as live core backlog as of 2026-06-13.

The last code-aligned closure pass added or confirmed:

- `merchant-finance-withdrawal`: manager selected-merchant finance-read coverage across the merchant finance surface, settlement-timeline adjustment response proof, and existing Baofu fee v2 finance SQL proof.
- `merchant-application-onboarding`: GPS duplicate-location hard reject is narrowed to same-door/location collisions, nearby-but-distinct merchants are allowed, and merchant-facing correction copy is proof-covered.
- `merchant-order-operations`: kitchen realtime degradation coverage when open-status refresh fails.
- `merchant-device-display-config`: Flutter saved-printer reconnect, disconnected-device state clearing/copy, and stable scan/connect/print-failure copy.
- `merchant-business-hours-auto-open`: existing manual-vs-automatic override API/SQL proof is recognized as closure evidence.

Total core backlog: 0 items.

## External Evidence And Product Decisions

These are not counted as core code/test repair backlog because they require product policy, real hardware, provider/manual-operational evidence, or a separate design decision:

- `merchant-finance-withdrawal`: provider-timeout/manual-recovery remains an external operations drill. Once Baofu has accepted a withdrawal, callback or query should converge it to a terminal state; the remaining drill is proving ambiguous timeout/no-provider-evidence cases route to abnormal/failed/manual handling instead of being treated as success.
- `merchant-device-display-config`: stays open until real Android BLE hardware validation on target merchant devices.
- `merchant-business-hours-auto-open`: no current action; product decision 2026-06-13 keeps whole-day-close semantics for special-date close rows, and same-day partial close/open can wait for future merchant demand.

## Optional Contract Candidates

These are still worth keeping an eye on, but they are not counted in the core backlog because they are product-contract or cleanup choices rather than open closure gaps:

- `merchant-claim-recovery.slice.md`: whether `/v1/merchant/claims/:id/recovery` should exist for external clients.
- `merchant-app-bind-and-device.slice.md`: whether managers should be allowed to generate App bind codes, and whether unsupported native-push provider copy should be preflighted client-side before backend rejection.
- `merchant-dish-status-and-inventory.slice.md`: single-workflow transaction coverage if dish base fields, featured tags, and customizations are later moved behind one endpoint.
- `merchant-review-reply.slice.md`: whether reply history should be preserved or exposed as a separate clear/delete action.
- `merchant-member-balance-adjust.slice.md`: whether manual adjustment should return a durable `transaction_id`.
- `merchant-profile-update.slice.md`: whether GET should persist a default display-config row instead of returning a default response path.
- `merchant-order-operations.slice.md`: future state-changing retry/migration design for historical failed-refund rows, if product later promotes read-only reconciliation candidates into automatic mutation.
- `merchant-staff-and-group.slice.md`: group review alias drift and legacy pending-status reverse references.
- `merchant-business-hours-auto-open.slice.md`: same-day partial close/open special-date support if future merchant demand appears.

## How To Read This Backlog

- If a slice says "Missing high-value tests" but the item only appears under external evidence, product decisions, or optional contract candidates above, it is not an active code/test repair target.
- If a slice mentions "optional", "consider", "decide whether", or "contract-drift candidate", treat it as a separate product or cleanup choice unless the code changes again.
- Slice files keep the proof trail; this README keeps the ranking.
