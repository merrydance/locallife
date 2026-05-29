# Baofu Reservation Dish Adjustment Refactor Design

Risk class: G3 - Baofu aggregate payment, pre-share refund, reservation state, inventory, callback/query fact application, idempotency, and profit-sharing timing.

Status: implemented in the active working tree; final validation and commit/push are in progress.

Related design: `artifacts/baofu-reservation-completed-share-and-refund-transaction-design-2026-05-29.md`

## Executive Decision

Full-payment reservation dish changes must take effect only after the required money boundary is satisfied.

For a net positive dish adjustment, the money boundary is Baofu `reservation_addon` payment success from a terminal callback or query fact. Baofu unified-order synchronous success is not enough, because the project Baofu contract matrix treats `returnCode=SUCCESS` as communication success only. The final payment state comes from callback/query facts.

For a net negative dish adjustment, the money boundary is the local guarded refund-order reservation. The effective dish change and all required local `refund_orders` must commit atomically. Provider refund submission stays post-commit and recoverable.

For a zero-delta dish adjustment, no money boundary is needed, so the effective items can change immediately inside the same guarded reservation transaction.

This makes the product model simple:

- "退菜" means backend computes a negative net delta, applies the target snapshot, and creates refund order(s).
- "加菜" means backend computes a positive net delta, creates a pending adjustment and a Baofu add-on payment, and applies the target snapshot only after payment success.
- "换菜" is not a special money flow. It is the same target-snapshot adjustment. The backend decides whether the final result is zero, refund, or add-on payment.

## Business Design Or Technical Vulnerability

The current positive-delta full-payment path is a pure technical vulnerability against the confirmed business rule, not merely an optional business-design choice.

The confirmed business rule is: add-money reservation changes are effective only after payment success.

Current code violates that rule:

- `locallife/logic/reservation_dishes.go:41-103` appends add-dish items before creating the `reservation_addon` payment.
- `locallife/logic/reservation_dishes.go:132-300` replaces reservation items at `ReplaceReservationItemsTx` or `ReplaceReservationItemsWithRefundOrdersTx` before creating the positive-delta payment at `:266-273`.
- `locallife/api/table_reservation.go:1532-1538` and `:1626-1633` call the logic without passing `PaymentFacade`, so a full-payment positive adjustment can mutate items and then fail with "baofu payment facade not configured".
- `weapp/miniprogram/pages/reservation/modify/index.ts:315-318` awaits `modifyDishes` and immediately navigates back. It does not inspect the returned payment response or call the existing `completePaymentWorkflow`.

The earlier completed-at profit-sharing timing was a business-rule correction with money-path consequences. This new positive-adjustment issue is narrower: once the business rule is "paid success before effect", the current mutation-before-success implementation is technically wrong and must be fixed.

## Current Production Path Evidence

### Baofu Payment Boundary

Baofu aggregate WeChat JSAPI payment is current production scope in `.github/standards/domains/baofu-payment/CAPABILITY_GROUP_INDEX.md`. The important contract constraints are:

- Unified order is only payment creation. It is not terminal payment success.
- Payment terminal state is produced by callback or order query facts.
- Provider calls must remain outside database transactions.
- Callback/query facts must be persisted and applied idempotently.

Local code follows this pattern:

- `locallife/logic/baofu_payment_order_route.go:76-129` creates a local payment order, then calls Baofu `CreateWechatJSAPIOrder`, then returns WeChat pay params.
- `locallife/logic/baofu_payment_service.go:103-173` records a create-payment command and calls Baofu unified order.
- `locallife/logic/baofu_payment_service.go:175-284` records terminal payment facts and creates a fact application when terminal.
- `locallife/logic/payment_fact_application_service.go:396-428` applies reservation payment facts.
- `locallife/db/sqlc/tx_payment_success.go:172-200` currently handles `reservation_addon` success by creating a reservation payment, adding prepaid amount, and syncing inventory. It does not apply a pending dish-change snapshot because no such durable object exists yet.

### Baofu Refund And Share Boundary

Existing refund/share guardrails should be kept:

- `locallife/db/sqlc/tx_refund.go:52-147` locks the payment order, checks Baofu profit-sharing guard, counts occupied refund amount, and creates the local refund order.
- `locallife/db/sqlc/tx_baofu_profit_sharing.go:45-231` blocks profit sharing when active refunds exist and allows successful-refund net sharing only for `reservation` and `reservation_addon`.
- The completed-share design already moved reservation profit sharing to `completed_at` and scoped partial-refund net sharing to reservation payment orders.

The new adjustment design must build on that:

- Negative adjustment uses the guarded refund helper in the same DB transaction as the item snapshot change.
- Positive add-on payment remains an independent `reservation_addon` payment order and later an independent Baofu share bill.
- Partial refunds on reservation and add-on payment orders continue to reduce later net share. They must not permanently block sharing if there is no active refund.

## Target Invariants

After implementation:

- Effective reservation dishes are only rows in `reservation_items`.
- Pending positive adjustments are not written to `reservation_items`.
- Pending positive adjustments are not visible to kitchen, start-cooking, completion, reservation inventory, or profit-sharing calculations as effective dishes.
- `table_reservations.prepaid_amount` increases for add-on payments only inside the payment-success fact application.
- `reservation_payments(type='addon')` is created only inside the payment-success fact application.
- The positive adjustment target snapshot is applied exactly once, guarded by the linked `payment_order_id` and adjustment status.
- Duplicate Baofu callbacks or query facts do not duplicate items, prepaid amount, reservation payment records, inventory, or applied status.
- Payment failure, close, or timeout closes/expires the pending adjustment without changing effective dishes.
- A pending positive adjustment blocks start-cooking and completion. The merchant should not start preparing a target that has not been paid.
- Cancellation while a positive adjustment payment is pending should fail with a clear conflict until the pending payment is closed or expires. This avoids local cancellation racing a remotely successful payment.
- Negative adjustments commit effective items and local refund orders atomically. Provider refund submission and recovery remain post-commit.
- Active pending/processing refund orders continue to block Baofu profit sharing.
- No Baofu HTTP call runs inside a DB transaction.
- No historical data remediation is required for this rollout because the issue was found before affected historical records exist.

## Data Model

Add a durable adjustment model. This is the audit object for add, remove, and replace operations.

### `reservation_adjustments`

Recommended fields:

- `id BIGSERIAL PRIMARY KEY`
- `reservation_id BIGINT NOT NULL REFERENCES table_reservations(id)`
- `user_id BIGINT NOT NULL`
- `merchant_id BIGINT NOT NULL`
- `direction TEXT NOT NULL CHECK (direction IN ('positive','negative','zero'))`
- `status TEXT NOT NULL`
- `current_total BIGINT NOT NULL CHECK (current_total >= 0)`
- `target_total BIGINT NOT NULL CHECK (target_total >= 0)`
- `delta_amount BIGINT NOT NULL`
- `payment_order_id BIGINT UNIQUE NULL REFERENCES payment_orders(id)`
- `failure_reason TEXT NULL`
- `close_reason TEXT NULL`
- `applied_at TIMESTAMPTZ NULL`
- `closed_at TIMESTAMPTZ NULL`
- `created_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`

Suggested statuses:

- `creating_payment`: positive adjustment row exists while local payment creation/external unified-order setup is in progress.
- `pending_payment`: Baofu add-on payment is pending and target snapshot is not effective.
- `applying`: payment success fact owns the application lock.
- `applied`: target snapshot is effective.
- `closed`: user/provider closed before payment success.
- `failed`: provider terminal failure or unrecoverable local setup failure.
- `expired`: local timeout closed the payment before success.

Negative and zero adjustments can be recorded as `applied` in the same transaction that changes effective items. If negative refund provider submission later fails, the refund order state records the provider result; the adjustment remains the business snapshot that was applied.

Add a partial unique index so only one positive unresolved adjustment can exist per reservation:

```sql
CREATE UNIQUE INDEX reservation_adjustments_one_active_positive_uidx
ON reservation_adjustments(reservation_id)
WHERE status IN ('creating_payment', 'pending_payment', 'applying');
```

### `reservation_adjustment_items`

Use normalized target snapshot rows, not only a JSON blob. This lets SQL transactions validate and apply the snapshot without ad hoc parsing.

Recommended fields:

- `id BIGSERIAL PRIMARY KEY`
- `adjustment_id BIGINT NOT NULL REFERENCES reservation_adjustments(id) ON DELETE CASCADE`
- `dish_id BIGINT NULL`
- `combo_id BIGINT NULL`
- `quantity SMALLINT NOT NULL CHECK (quantity > 0)`
- `unit_price BIGINT NOT NULL CHECK (unit_price >= 0)`
- `total_price BIGINT NOT NULL CHECK (total_price >= 0)`
- `position INT NOT NULL DEFAULT 0`
- check exactly one of `dish_id` or `combo_id` is present

The rows are the intended target snapshot, not a delta list. Applying an adjustment deletes current `reservation_items` and inserts these rows.

### Pending Inventory Holds

For production robustness, positive adjustments should also hold incremental dish inventory while payment is pending. Without this, a user can pay successfully and then the success transaction can fail because stock disappeared between payment creation and callback.

Recommended table:

- `reservation_adjustment_inventory_holds`
- `adjustment_id`
- `merchant_id`
- `dish_id`
- `reservation_date`
- `quantity`
- `status IN ('held','converted','released')`
- `expires_at`
- timestamps

The hold increments `daily_inventory.reserved_quantity` but does not write `reservation_inventory` or `reservation_items`. On payment success, the hold is converted into the effective reservation inventory while the target snapshot is applied. On payment close/fail/timeout, the hold is released.

This is still consistent with "payment success before effect": the customer-facing/kitchen-facing reservation dishes do not change before payment. The inventory hold is a short-lived capacity reservation tied to the pending payment, similar to a payment timeout hold.

If the team chooses not to implement pending inventory holds in the first commit, the payment-success application must have an explicit abnormal path for insufficient inventory after payment success, including operator alerting and a refund-support path. The recommended plan below includes holds because it is the safer production design.

## State Flows

### Zero Delta

1. Lock reservation.
2. Validate owner, status, no cooking started, and no active positive adjustment.
3. Validate target items and compute `delta = 0`.
4. Replace `reservation_items` and sync effective `reservation_inventory` in one DB transaction.
5. Record `reservation_adjustments(status='applied', direction='zero')`.
6. Return `outcome='applied'`.

### Positive Delta

1. Lock reservation.
2. Validate owner, status, no cooking started, and no active positive adjustment.
3. Validate target items and compute `delta > 0`.
4. In a DB transaction, create `reservation_adjustments(status='creating_payment')`, create normalized adjustment items, create local `payment_orders(business_type='reservation_addon')`, link `payment_order_id`, and hold incremental dish inventory.
5. Commit the DB transaction.
6. Call Baofu unified order outside the DB transaction.
7. If Baofu setup succeeds, mark adjustment `pending_payment`, return pay params, and schedule payment timeout.
8. If Baofu setup fails, close the local payment, release inventory holds, mark adjustment `failed` or `closed`, and return a safe payment-setup error. Effective items remain unchanged.

The add-on payment is independent from the original reservation payment. Later profit sharing also remains independent per payment order.

### Positive Payment Success

Triggered only by terminal Baofu payment callback/query fact application:

1. Mark the linked Baofu payment order paid, using existing idempotent payment fact behavior.
2. Lock payment order and linked `reservation_adjustments` by `payment_order_id`.
3. If payment is already processed or adjustment is already `applied`, return idempotent success.
4. Require adjustment status `pending_payment` or `applying`.
5. Lock reservation and re-check status allows modification and no cooking started. Normal flows satisfy this because start-cooking/completion/cancel are blocked while pending.
6. Replace `reservation_items` from `reservation_adjustment_items`.
7. Convert inventory holds into effective `reservation_inventory` without double-counting daily reserved inventory.
8. Create `reservation_payments(type='addon')`.
9. Add `table_reservations.prepaid_amount += payment_order.amount`.
10. Mark adjustment `applied`, set `applied_at`, and mark payment processed.

If an impossible race leaves the reservation in a non-applicable state after the user has paid, the system must fail visibly rather than silently losing money. The preferred fail-safe is to leave the fact application failed/retryable with a structured critical alert and an operator runbook. A later extension can automate a compensating add-on refund, but it should not silently apply an invalid dish snapshot.

### Positive Payment Close, Failure, Or Timeout

Triggered by Baofu terminal failure/close facts, user close, or local timeout close:

1. Lock linked adjustment.
2. If already `applied`, do not close it. A late close/failure fact after success is a terminal conflict and must be logged.
3. If status is `creating_payment` or `pending_payment`, release inventory holds.
4. Mark adjustment `closed`, `failed`, or `expired`.
5. Effective `reservation_items`, `reservation_inventory`, `reservation_payments`, and `prepaid_amount` remain unchanged.

`locallife/worker/task_payment_timeout.go:86-100` currently only updates the payment order to closed. The new flow must close or expire the linked adjustment as part of the local timeout branch.

### Negative Delta

1. Lock reservation.
2. Validate owner, status, no cooking started, and no active positive adjustment.
3. Validate target items and compute `delta < 0`.
4. Allocate refund amount across paid reservation and paid `reservation_addon` payment orders, subtracting pending/processing/success refund amounts.
5. In one DB transaction, record adjustment, replace `reservation_items`, sync effective reservation inventory, and create all guarded refund orders.
6. Commit.
7. Schedule provider refund processing tasks after commit. If scheduling fails, log and rely on recovery.
8. Return `outcome='refund_initiated'` with refund order summary.

The refund order allocation may span the original `reservation` payment and one or more `reservation_addon` payments. That is expected and is separate from profit sharing. Later sharing uses each paid payment order's net amount.

### Add-Dishes Endpoint

The existing `add-dishes` route should become an adapter over the same target-snapshot adjustment service:

1. Load current effective `reservation_items`.
2. Merge requested added items into a target snapshot.
3. Call the same adjustment flow as `modify-dishes`.

This avoids a second write path where add-dishes can append effective items before payment.

## Cross-Business Impact

### Reservation Detail, List, And UI

Reservation detail and list should continue showing effective items from `reservation_items`. Add a pending-adjustment summary only when it helps the user resume or close payment:

- `pending_adjustment_id`
- `pending_payment_order_id`
- `pending_delta_amount`
- `pending_status`

Do not merge pending target items into ordinary effective item lists unless the UI labels them as pending. Kitchen and merchant operational screens must use effective items only.

### Kitchen, Start Cooking, And Completion

Start-cooking and completion must check for active positive adjustments and reject with conflict if one exists.

This is necessary because a pending positive adjustment represents a target the user has not paid for. If the merchant starts cooking or completes while the payment is pending, the payment-success fact may later try to mutate a closed operational workflow.

Current entrypoints to guard:

- `locallife/logic/reservation.go:421-462` completion.
- `locallife/logic/reservation.go` start-cooking path around `UpdateReservationCookingStarted`.
- API action-state builders so merchant UI can disable the action.

### Cancellation

Cancellation while a positive adjustment payment is pending should reject with conflict and return the pending payment reference. The user or UI can close the pending add-on payment first, or the timeout worker can expire it. This avoids a race where local cancellation succeeds but Baofu later reports the add-on payment successful.

Negative or zero already-applied adjustments do not block cancellation. Existing cancellation refund policy remains a separate business flow.

### Profit Sharing

Profit sharing remains anchored to merchant `completed_at`, per the completed-share design.

Positive add-on payments are independent `reservation_addon` payment orders. After completion, each paid add-on payment gets its own net Baofu share bill. Partial refunds on reservation and add-on payments reduce net share. Pending or processing refunds block sharing.

Pending positive adjustments are not paid and not effective, so they must block completion rather than producing a share bill.

### Refunds

Negative adjustment refunds are reservation-domain refunds, not takeout order refunds. Ordinary `order` partial-refund behavior is out of scope.

Current evidence that reservation refunds are already separate:

- Baofu refund callback owner selection treats `reservation` and `reservation_addon` as reservation owner in `locallife/api/baofu_callback.go:553-560`.
- `locallife/logic/payment_fact_application_service.go:633-672` applies reservation refund facts and subtracts prepaid amount only on transition to success.
- `locallife/db/sqlc/tx_baofu_profit_sharing.go:233-236` allows successful-refund net sharing only for `reservation` and `reservation_addon`.

### Existing `/v1/orders/:id/replace`

`ReplaceReservationOrderWithBaofu` is a separate and older replacement-order path. It should not be used as the primary reservation dish-change path after this refactor.

Recommendation:

- Keep ordinary takeout/order replacement out of this scope.
- For reservation dish changes, route Mini Program and backend callers to `modify-dishes`.
- Add tests or explicit guards so the old order replacement path cannot reintroduce reservation dish mutation before add-on payment success.
- If a future business path still needs replacement-order semantics, it should be redesigned on top of the same `reservation_adjustments` model.

## API Contract

Replace untyped `map[string]interface{}` responses with a typed response:

```json
{
  "outcome": "applied | payment_required | refund_initiated",
  "adjustment_id": 123,
  "delta_amount": 2500,
  "payment": {
    "payment_order_id": 456,
    "business_type": "reservation_addon",
    "amount": 2500,
    "pay_params": {}
  },
  "refunds": [
    {
      "refund_order_id": 789,
      "payment_order_id": 456,
      "amount": 1000
    }
  ]
}
```

For positive deltas, the response message should say payment is required and the reservation has not been modified yet.

For negative deltas, the response can say the reservation was modified and refund was initiated, because the effective item snapshot and local refund order(s) committed atomically.

## Mini Program Contract

The Mini Program already has a reusable payment workflow that supports `reservation_addon`:

- `weapp/miniprogram/services/payment-workflow.ts:31-37`
- `weapp/miniprogram/services/payment-workflow.ts:198-275`

The reservation modify page must use it:

1. Type `ReservationService.modifyDishes` response.
2. If `outcome='payment_required'`, convert the returned payment payload to `PaymentOrderResponse`.
3. Call `completePaymentWorkflow`.
4. Navigate back only after `status='paid'` or show a pending/failed/cancelled state.
5. For `outcome='refund_initiated'` or `outcome='applied'`, reload or navigate back as today.

The current behavior at `weapp/miniprogram/pages/reservation/modify/index.ts:315-318` is unsafe because it treats an API response as final business success.

## Security And Reliability Boundaries

The refactor must explicitly cover:

- Authorization: reservation owner can request dish changes; merchant-only actions remain merchant-bound.
- Tenant boundary: reservation merchant must match validated dishes/combos and inventory rows.
- Idempotency: duplicate Baofu success facts do not apply the same adjustment twice.
- Concurrency: one unresolved positive adjustment per reservation; stale target snapshots reject.
- Replay: callback/query facts remain deduped by existing external payment fact keys.
- Provider failure: Baofu create/close/refund failures do not leave effective items half-mutated.
- Timeout: payment timeout closes both payment order and pending adjustment.
- Unlimited-inventory dishes: when no `daily_inventory` row exists, pending adjustment holds and later release/close paths must not fail; absence of a row means there is no tracked finite stock to release.
- Payment-channel consistency: Baofu JSAPI creation must use the sub-merchant id returned by the same local payment/adjustment transaction that created the payment order and persisted `attach`, not a separate stale readiness read.
- Observability: impossible paid-but-not-applied states produce structured critical logs with payment order, adjustment, reservation, and fact application IDs.

## Non-Goals

- No post-share refund support in this refactor.
- No change to ordinary takeout/order partial-refund policy.
- No automatic historical data repair.
- No new Baofu provider contract surface.
- No Baofu HTTP call inside DB transactions.
- No separate 4-hour checked-in auto-share or reminder flow.
