# Merchant Auto-Accept Print Plan - 2026-05-28

## Goal

Allow merchants with cloud printers to opt into automatic order acceptance after payment succeeds, so the existing `accepted` print trigger can print paid orders without requiring manual merchant action.

## Current Findings

- Merchant 11 has an active Feieyun printer and default display config.
- Order 14 reached `paid` / `pending_kitchen`, but no `print_logs` were created.
- Current backend only schedules order printing when `AcceptMerchantOrder` or `MarkMerchantOrderReady` runs.
- Default print trigger is `accepted`; payment success currently publishes a merchant notification but does not accept the order.
- There is no backend auto-accept setting today.

## Design

Add `auto_accept_paid_orders` to `order_display_configs`.

- Default: `false`, preserving existing behavior for merchants without printers or merchants that want manual review.
- API: include the field in `GET/PUT /v1/merchant/display-config`.
- Payment side effect: after the payment-success transaction commits and the `order_payment_succeeded` outbox is dispatched, if the merchant config enables `auto_accept_paid_orders`, call the same acceptance transition used by manual merchant acceptance.
- Printing: dispatch the existing `accepted` print task after auto-accept succeeds, using the same stable task key shape as the manual accept path.
- Idempotency: if the order is already accepted/preparing on outbox retry, re-dispatch the stable `accepted` print task after rechecking config and printer eligibility. Print-log task-key uniqueness keeps repeated delivery from printing duplicates, while still recovering the partial-success case where the order transition committed but the print task enqueue failed.

## Implementation Tasks

1. Add DB schema support.
   - Create a migration adding `order_display_configs.auto_accept_paid_orders BOOLEAN NOT NULL DEFAULT false`.
   - Update `locallife/db/query/order_display_config.sql` selects/inserts/updates/upserts.
   - Run `make sqlc`.

2. Expose the setting in display-config API.
   - Add `auto_accept_paid_orders` to response and update request DTOs.
   - Defaults stay `false` when no config exists.
   - Update API tests for default, create, and update behavior.

3. Add auto-accept decision logic.
   - Add a focused logic helper that loads display config and returns whether a paid order should be auto-accepted.
   - Require `auto_accept_paid_orders=true`, print enabled for the order type, `accepted` print trigger enabled, and at least one active Feieyun printer that supports the order type.
   - This keeps the setting useful for printer-backed merchants and harmless for merchants without printers.

4. Wire payment-success outbox dispatch.
   - In `dispatchOrderPaymentSucceededOutbox`, after loading the order and before/around notification dispatch, call the auto-accept helper.
   - If auto-accept succeeds, schedule the stable `accepted` print task.
   - If the order is already accepted/preparing, re-check eligibility and re-schedule the same stable print task so outbox retries can recover enqueue failures.
   - If the order is no longer eligible for acceptance or accepted-print recovery, continue without changing it.
   - Unexpected DB or transition failures should return an error so the outbox retries.

5. Validate.
   - Targeted tests for logic and worker outbox behavior.
   - `make sqlc` after SQL changes.
   - Run focused packages: `go test ./logic ./worker ./api`.
   - Because this touches order/payment post-commit behavior, run `make test-safety` if local dependencies are available.

## Risk

Risk class: G2.

This changes an async post-payment state transition and can alter merchant order flow. It does not change funds movement, but it is adjacent to the payment-success outbox and must remain idempotent and retry-safe.
