# Rider Deposit Withdrawal Idempotency Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make rider deposit withdrawal/redeem requests idempotent end to end so client retries reuse the same WeChat `out_refund_no` records instead of creating a second withdrawal intent.

**Architecture:** Keep `refund_orders.out_refund_no` as the WeChat external contract key and add a rider-deposit-withdrawal request binding outside it for client request replay. The backend transaction owns request binding and refund-order creation; logic submits the already-created refund orders to WeChat; the Mini Program keeps one stable draft idempotency key per deposit withdrawal attempt.

**Tech Stack:** Go, gin, pgx/sqlc, PostgreSQL migrations, gomock, WeChat Pay direct refund, WeChat Mini Program TypeScript.

**Risk class:** G3. This touches refund/money movement, idempotency, external WeChat provider calls, retry/recovery semantics, and rider-facing funds UI.

---

## Scope Boundary

This plan covers only **rider deposit withdrawal/redeem**:

- Backend endpoint: `POST /v1/rider/withdraw`
- Backend service: `locallife/logic/rider_deposit_refund_service.go`
- Persistence transaction: `locallife/db/sqlc/tx_rider_refund.go`
- WeChat capability: retained direct WeChat refund, `POST /v3/refund/domestic/refunds`
- Mini Program page: `weapp/miniprogram/pages/rider/deposit/index.ts`

This plan does **not** cover rider income withdrawal:

- Do not modify `/v1/rider/income/baofu-withdrawal/**`.
- Do not modify `locallife/logic/baofu_withdraw_service.go`.
- Do not modify `weapp/miniprogram/pages/rider/income/withdrawals/**`.
- Do not change Baofoo/Baofu withdrawal behavior, request hashes, or frontend idempotency keys.

## Confirmed Problem

`refund_orders.out_refund_no` already exists, is unique, and is passed to WeChat. That is the correct WeChat-side idempotency key.

The real gap is one level earlier: `POST /v1/rider/withdraw` has no request-level replay key. A retry after a lost response or ambiguous WeChat create result can run `PrepareRiderDepositRefundTx` again, generate a fresh `out_refund_no`, reserve more rider deposit credit, and submit a second WeChat refund intent. WeChat's refund contract requires retrying a failed/uncertain refund application with the original merchant refund number.

## Provider Truth

Active provider/capability: WeChat Pay direct merchant refund.

Repo source:

- `.github/standards/domains/wechat-payment/README.md`
- `locallife/wechat/contracts/direct_payment_refund.go`

Official source:

- `https://pay.weixin.qq.com/doc/v3/merchant/4012791903.md`

Important contract points already verified in prior investigation:

- Same merchant refund number requested multiple times should refund only once.
- If a refund application fails and is retried, the original merchant refund number must be reused.

## Design Decisions

- Use request header `Idempotency-Key` for `POST /v1/rider/withdraw`, matching existing refund and Baofoo withdrawal entrypoints.
- Compute a canonical request hash from scope, actor user id, amount, and trimmed remark.
- Add `rider_deposit_withdrawal_requests`, not a generic central table, because one withdrawal request can map to multiple `refund_orders`.
- Store `refund_order_ids` as a JSONB array in the binding row. It is a replay snapshot, not the external contract key.
- Transaction behavior:
  - Same `(user_id, idempotency_key)` plus same request hash returns the original refund plans and sets `IdempotencyReplayed=true`.
  - Same key with a different hash returns conflict.
  - First writer creates the binding and refund orders in the same transaction.
  - Concurrent same-key writers serialize on the binding row or unique constraint and do not create a second refund batch.
- Logic behavior:
  - Replayed requests rebuild the response from existing refund orders.
  - Replayed requests do not create new refund orders.
  - Replayed requests do not submit another WeChat create call for refund orders that are already `processing`, `success`, `failed`, or `closed`.
  - Replayed requests may submit pending refund orders with the same stored `out_refund_no` only when the previous local attempt never reached an accepted, terminal, or explicit rejected command.
- Ambiguous WeChat create failures:
  - Timeout, 5xx, malformed/contract response, `SYSTEM_ERROR`, `FREQUENCY_LIMITED`, and unknown provider responses are treated as uncertain.
  - Uncertain failures keep the refund order `pending` or `processing`, record command status `unknown`, and return `202` with the same refund order ids.
  - Clear terminal business failures such as `NOT_ENOUGH`, `USER_ACCOUNT_ABNORMAL`, and `RESOURCE_NOT_EXISTS` can still resolve the refund order as failed/closed and unfreeze according to the existing compensation path.

## Task 1: Plan Artifact And Scope Guard

**Files:**

- Create: `artifacts/rider-deposit-withdrawal-idempotency-plan-2026-06-13.md`

- [ ] Write this plan with the boundary above.
- [ ] Review the plan for scope creep into rider income/Baofu withdrawal.
- [ ] Commit only this artifact.

Run:

```bash
git status --short
git add artifacts/rider-deposit-withdrawal-idempotency-plan-2026-06-13.md
git commit -m "docs: plan rider deposit withdrawal idempotency"
```

Expected:

- Only the new artifact is committed.
- Existing unrelated `artifacts/codegraph/codegraph-tool-crosscheck-2026-06-13.md` remains untouched and untracked unless it was already user-owned.

## Task 2: Persistence Binding Schema And Queries

**Files:**

- Create: `locallife/db/migration/000XXX_add_rider_deposit_withdrawal_idempotency.up.sql`
- Create: `locallife/db/migration/000XXX_add_rider_deposit_withdrawal_idempotency.down.sql`
- Modify: `locallife/db/query/refund_order.sql`
- Regenerate: `locallife/db/sqlc/refund_order.sql.go`
- Regenerate if interface changes: `locallife/db/sqlc/querier.go`
- Test: add the transaction replay tests in Task 3 after the generated query types exist.

Schema:

```sql
CREATE TABLE rider_deposit_withdrawal_requests (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id),
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL,
    requested_amount BIGINT NOT NULL,
    accepted_amount BIGINT NOT NULL DEFAULT 0,
    refund_order_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT rider_deposit_withdrawal_requests_user_key_uq UNIQUE (user_id, idempotency_key),
    CONSTRAINT rider_deposit_withdrawal_requests_requested_amount_check CHECK (requested_amount > 0),
    CONSTRAINT rider_deposit_withdrawal_requests_accepted_amount_check CHECK (accepted_amount >= 0),
    CONSTRAINT rider_deposit_withdrawal_requests_refund_order_ids_array_check CHECK (jsonb_typeof(refund_order_ids) = 'array')
);

CREATE INDEX rider_deposit_withdrawal_requests_user_id_idx
    ON rider_deposit_withdrawal_requests(user_id);
```

Queries to add to `locallife/db/query/refund_order.sql`:

```sql
-- name: CreateRiderDepositWithdrawalRequest :one
INSERT INTO rider_deposit_withdrawal_requests (
    user_id,
    idempotency_key,
    request_hash,
    requested_amount,
    accepted_amount,
    refund_order_ids
) VALUES (
    sqlc.arg(user_id),
    sqlc.arg(idempotency_key),
    sqlc.arg(request_hash),
    sqlc.arg(requested_amount),
    COALESCE(sqlc.narg(accepted_amount), 0),
    COALESCE(sqlc.narg(refund_order_ids), '[]'::jsonb)
)
RETURNING *;

-- name: GetRiderDepositWithdrawalRequestForUpdate :one
SELECT id, user_id, idempotency_key, request_hash, requested_amount, accepted_amount, refund_order_ids, created_at, updated_at
FROM rider_deposit_withdrawal_requests
WHERE user_id = sqlc.arg(user_id)
  AND idempotency_key = sqlc.arg(idempotency_key)
LIMIT 1
FOR UPDATE;

-- name: UpdateRiderDepositWithdrawalRequestRefundOrders :one
UPDATE rider_deposit_withdrawal_requests
SET accepted_amount = sqlc.arg(accepted_amount),
    refund_order_ids = sqlc.arg(refund_order_ids),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;
```

Steps:

- [ ] Add the migration and SQL queries.
- [ ] Run `cd locallife && make sqlc`.
- [ ] Add a focused migration/query smoke test only if sqlc generation or migration coverage needs it.
- [ ] Commit the migration, query source, and generated code only after `make sqlc` succeeds.

Validation:

```bash
cd locallife
make sqlc
```

## Task 3: Transaction-Level Idempotent Prepare

**Files:**

- Modify: `locallife/db/sqlc/tx_rider_refund.go`
- Modify: `locallife/db/sqlc/store.go`
- Test: `locallife/db/sqlc/tx_rider_refund_test.go`

Required type changes:

```go
type PrepareRiderDepositRefundTxParams struct {
    RiderID                int64
    UserID                 int64
    Amount                 int64
    Remark                 string
    IdempotencyKey         string
    IdempotencyRequestHash string
}

type PrepareRiderDepositRefundTxResult struct {
    Rider                 Rider
    RefundPlans           []RiderDepositRefundPlan
    FrozenAmount          int64
    IdempotencyReplayed   bool
    WithdrawalRequestID   int64
}
```

Transaction behavior:

- Lock `riders` by `RiderID`.
- If idempotency metadata is missing, return a typed request error; the API should normally catch this first.
- Try `GetRiderDepositWithdrawalRequestForUpdate`.
- If a row exists:
  - Different hash -> conflict.
  - Same hash -> decode `refund_order_ids`, load the refund orders with `ListRiderDepositWithdrawalRefundOrdersByIDs`, and return existing plans with `IdempotencyReplayed=true`.
- If no row exists:
  - Create a placeholder request row with `[]`.
  - Run the existing credit reservation and refund-order creation path.
  - Store the resulting refund order ids and accepted amount back to the request row.

Tests:

- [ ] Add failing DB test `TestPrepareRiderDepositRefundTx_ReplaysSameIdempotencyKey`.
  - First call creates refund plans.
  - Second call with same user, key, hash, amount, and remark returns the same refund order ids and `IdempotencyReplayed=true`.
  - Assert the second call does not create extra `refund_orders`.
- [ ] Run the focused test and confirm it fails before transaction implementation.
- [ ] Add `TestPrepareRiderDepositRefundTx_RejectsConflictingIdempotencyKey`.
- [ ] Add `TestPrepareRiderDepositRefundTx_ReplayLoadsSplitRefundPlans`.
- [ ] Add `TestPrepareRiderDepositRefundTx_ConcurrentSameKeyCreatesOneBatch` if the local DB test harness can run a small goroutine race reliably.

Validation:

```bash
cd locallife
go test ./db/sqlc -run 'TestPrepareRiderDepositRefundTx_(ReplaysSameIdempotencyKey|RejectsConflictingIdempotencyKey|ReplayLoadsSplitRefundPlans|ConcurrentSameKeyCreatesOneBatch)' -count=1
```

Review gate:

- Confirm no external I/O occurs inside the transaction.
- Confirm `out_refund_no` remains on `refund_orders` and stays unique.
- Confirm replay does not consume additional rider deposit credit or increase `riders.frozen_deposit`.

## Task 4: Logic And API Request-Level Contract

**Files:**

- Modify: `locallife/logic/rider_deposit_refund_service.go`
- Modify: `locallife/api/rider.go`
- Regenerate if Swagger annotations change: `locallife/docs/docs.go`, `locallife/docs/swagger.json`, `locallife/docs/swagger.yaml`
- Test: `locallife/logic/rider_deposit_refund_service_test.go`
- Test: `locallife/api/rider_test.go`

Logic changes:

- Add `IdempotencyKey string` to `SubmitRiderDepositWithdrawalInput`.
- Compute request hash in logic from:
  - version `v1`
  - scope `rider_deposit_withdrawal`
  - actor user id
  - amount in fen
  - trimmed remark
- Pass user id, idempotency key, and hash into `PrepareRiderDepositRefundTx`.
- If `PrepareRiderDepositRefundTx` returns a replayed result, rebuild the response from existing refund plans and avoid duplicate WeChat create calls for refund orders that are no longer `pending`.

API changes:

- Require header `Idempotency-Key`.
- Trim whitespace.
- Enforce max length 256.
- Forward it to logic.
- Add Swagger header annotation to `withdrawRider`.

Tests:

- [ ] Add API failing test `TestWithdrawRiderAPIRequiresIdempotencyKey`.
- [ ] Add API test that whitespace is trimmed and forwarded in `PrepareRiderDepositRefundTxParams`.
- [ ] Add logic failing test for same-key replay with a processing refund order and assert no `CreateRefund` call.
- [ ] Add logic failing test for same-key conflict mapped from transaction request error to HTTP 409.

Validation:

```bash
cd locallife
go test ./api -run 'TestWithdrawRiderAPI' -count=1
go test ./logic -run 'TestRiderDepositRefundService_SubmitWithdrawal' -count=1
```

If Swagger annotations changed:

```bash
cd locallife
make swagger
```

Review gate:

- Confirm only `POST /v1/rider/withdraw` changed.
- Confirm no rider income/Baofu withdrawal route or Mini Program income page changed.

## Task 5: WeChat Refund Create Uncertain Failure Handling

**Files:**

- Modify: `locallife/logic/refund_error_mapping.go`
- Modify: `locallife/logic/rider_deposit_refund_service.go`
- Test: `locallife/logic/rider_deposit_refund_service_test.go`
- Optional Test: `locallife/logic/refund_error_mapping_test.go`

Required behavior:

- Add a direct-refund create failure classifier for WeChat direct refunds.
- Clear business failures still use the existing compensation path:
  - `NOT_ENOUGH`
  - `USER_ACCOUNT_ABNORMAL`
  - `RESOURCE_NOT_EXISTS`
  - local validation errors before the provider call
- Uncertain failures do not call `ResolveRiderDepositRefundTx(...FAILED...)`.
- Uncertain failures record `external_payment_commands.command_status='unknown'` for the same `out_refund_no`.
- The API returns `202` with the already-created refund order ids so the Mini Program can poll status and retry with the same idempotency key if needed.

Tests:

- [ ] Add `TestRiderDepositRefundService_SubmitWithdrawal_WechatSystemErrorKeepsRefundPending`.
  - WeChat returns `SYSTEM_ERROR`.
  - Assert `ResolveRiderDepositRefundTx` is not called.
  - Assert command is recorded as `unknown`.
  - Assert response has accepted amount and processing status.
- [ ] Add `TestRiderDepositRefundService_SubmitWithdrawal_ContractResponseErrorKeepsRefundPending`.
- [ ] Keep existing `NOT_ENOUGH` compensation tests green.

Validation:

```bash
cd locallife
go test ./logic -run 'TestRiderDepositRefundService_SubmitWithdrawal|TestMapDirectRefundCreateError|TestDirectRefund' -count=1
```

Review gate:

- Confirm no ambiguous WeChat create result unfreezes rider deposit.
- Confirm terminal provider failures still unblock the rider through the existing failed-refund compensation path.

## Task 6: Mini Program Deposit Withdrawal Idempotency Key

**Files:**

- Modify: `weapp/miniprogram/pages/rider/_main_shared/api/rider.ts`
- Modify: `weapp/miniprogram/pages/rider/_services/rider-deposit-withdrawal.ts`
- Modify: `weapp/miniprogram/pages/rider/deposit/index.ts`
- Test or script: add a small focused script under `weapp/scripts/` if no existing one covers this page.

Required behavior:

- `RiderService.withdrawDeposit` accepts an options object with `idempotencyKey`.
- The request sends header `Idempotency-Key`.
- The deposit page creates a stable draft key before submit, for example:

```ts
function buildDepositWithdrawalIdempotencyKey() {
  return `rider-deposit-withdrawal:${Date.now()}:${Math.random().toString(36).slice(2, 10)}`
}
```

- The page stores the draft key in page data and reuses it while the withdrawal dialog/request is in flight.
- On accepted response with pending refund orders, persist the key together with pending withdrawal context so re-entry/retry can use the same key.
- Clear the key only after terminal success/failure is confirmed or the user explicitly cancels before submission.
- Do not alter rider income withdrawal key generation.

Validation:

```bash
cd weapp
npm run compile
```

If adding a focused script:

```bash
cd weapp
node scripts/check-rider-deposit-withdrawal-idempotency-contract.test.js
```

Review gate:

- Confirm the Mini Program change is limited to rider deposit withdrawal.
- Confirm no Baofu withdrawal API wrapper changed.
- Confirm weak-network retry/re-entry uses the stored key and not a new key.

## Task 7: Final Generated Checks, Safety Tests, Review, And Push

**Files:**

- Review all changed files.
- No new prompt templates.
- No `.github` changes unless implementation discovers a durable standard gap.

Validation checklist:

```bash
cd locallife
make sqlc
make check-generated
go test ./db/sqlc -run 'TestPrepareRiderDepositRefundTx|TestResolveRiderDepositRefundTx' -count=1
go test ./logic -run 'TestRiderDepositRefundService_SubmitWithdrawal|TestMapDirectRefundCreateError|TestDirectRefund' -count=1
go test ./api -run 'TestWithdrawRiderAPI' -count=1
make test-safety
```

```bash
cd weapp
npm run compile
```

Review checklist:

- [ ] Verify every task has a dedicated commit.
- [ ] Verify unrelated user/untracked files were not committed.
- [ ] Verify `out_refund_no` remains the WeChat refund contract key.
- [ ] Verify `Idempotency-Key` is a request replay key, scoped to rider deposit withdrawal.
- [ ] Verify no rider income/Baofu withdrawal files changed except read-only inspection.
- [ ] Verify uncertain WeChat create results do not mark local refunds failed.
- [ ] Verify final response clearly reports validations run, validations not run, regeneration, residual risks, and risk class.

Push:

```bash
git status --short
git log --oneline --decorate origin/main..HEAD
git push -u origin fix/rider-deposit-withdrawal-idempotency
```
