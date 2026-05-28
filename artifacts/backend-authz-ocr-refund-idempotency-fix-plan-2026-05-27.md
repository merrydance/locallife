# Backend Authz and Idempotency Fix Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the reviewed backend authorization and idempotency gaps in risk-management, OCR, and refund creation paths without weakening existing payment, media, OCR, or delivery invariants.

**Architecture:** Keep HTTP authz and request parsing in `locallife/api/**`, business replay semantics in `locallife/logic/**`, and persistent uniqueness or transaction guards in `locallife/db/**`. Casbin route policy is not global in this server; each sensitive route must be protected by the actual Gin middleware or by an explicit handler/logic authorization check.

**Tech Stack:** Go, gin, pgx/sqlc, gomock, sqlc, LocalLife backend standards, Baofu main-business refund flow, OCR job table.

**Risk class:** G3. This work touches authorization, tenant boundaries, OCR sensitive document jobs, refund/money movement, idempotency, and external provider side effects.

---

## Why This Document Exists

This document captures the security review context so a future session, compacted agent, or fresh engineer can continue without relying on chat memory.

The review found three concrete issues:

1. Admin-only risk endpoints are reachable by any authenticated user.
2. `POST /v1/ocr/jobs` accepts an arbitrary `idempotency_key` and returns the existing row on conflict without checking that the existing job belongs to the same canonical request.
3. `POST /v1/refunds` has strong over-refund protection but no request-level replay guard, so a retried partial refund can create another refund order with a new `out_refund_no`.

Do not broaden this plan into a general route rewrite, Casbin migration, OCR refactor, or payment-domain redesign. Fix the three invariants below with focused tests.

## Context Reconstruction

Before implementation, from repo root `/home/sam/locallife`, read:

1. `.github/copilot-instructions.md`
2. `.github/README.md`
3. `locallife/AGENTS.md`
4. `.github/instructions/backend-locallife.instructions.md`
5. `.github/instructions/backend-api.instructions.md`
6. `.github/instructions/backend-logic.instructions.md`
7. `.github/prompts/backend-bugfix.prompt.md`
8. `.github/standards/backend/README.md`
9. `.github/standards/backend/RUNTIME_ARCHITECTURE.md`
10. `.github/standards/backend/IDEMPOTENCY_STANDARDS.md`
11. `.github/standards/domains/baofu-payment/README.md` only for the refund task

Then read only the source files named in the active task. Do not bulk-load all backend standards or all artifacts.

## Global Non-Negotiables

- Treat `policy.csv` as policy data, not proof that a route is protected. In this server, `authGroup.Use(authMiddleware(...))` only validates login. Role protection must be visible in `server.go` or enforced by the called handler/logic.
- Do not turn on global Casbin middleware as a quick fix. That changes the whole route surface and can break routes whose object ownership is intentionally handled in handlers.
- Do not make admin bypasses for owner-only private document/OCR data unless a task explicitly designs that boundary and tests it.
- Do not replace `out_refund_no` with request-level idempotency. `out_refund_no` remains the external contract key for provider query/callback/reconciliation.
- Do not add a central idempotency table unless the task proves it is already part of the local architecture. For this plan, prefer a narrow refund request binding or local resource table pattern.
- Public errors must remain stable and not expose SQL, pgx, raw provider messages, stack traces, or internal object ownership details.
- If SQL query files or migrations change, run `make sqlc` and the relevant generated checks.
- If route middleware or Swagger annotations change, run `make swagger` only if annotations changed.

## Finding 1: Admin-Only Risk Endpoints Missing Actual Admin Middleware

### Background

Routes:

- `locallife/api/server.go:1448-1452`
  - `foodSafetyGroup := authGroup.Group("/food-safety")`
  - `foodSafetyGroup.PATCH("/merchants/:id/suspend", server.SuspendMerchant)`
- `locallife/api/server.go:1454-1458`
  - `fraudGroup := authGroup.Group("/fraud")`
  - `fraudGroup.POST("/detect", server.TriggerFraudDetection)`

Policy declares both admin-only:

- `locallife/casbin/policy.csv:82-84`

Handlers do not check role:

- `locallife/api/risk_management.go:1156-1199` (`TriggerFraudDetection`)
- `locallife/api/risk_management.go:1222-1249` (`SuspendMerchant`)

Impact:

- Any authenticated user can trigger internal fraud scans.
- Any authenticated user can suspend a merchant. `CircuitBreakMerchant` writes `merchant_profiles.is_suspended=true`, sets `suspend_until`, emits a notification, and cancels future reservations:
  - `locallife/algorithm/food_safety_handler.go:218-272`
  - `locallife/db/query/trust_score.sql:66-73`

### Correct Invariant

Only platform admins may call:

- `PATCH /v1/food-safety/merchants/:id/suspend`
- `POST /v1/fraud/detect`

`POST /v1/food-safety/report` must remain available to normal authenticated users.

### Files

- Modify: `locallife/api/server.go`
- Test: `locallife/api/risk_management_authz_test.go` or existing nearby API test file if there is already a risk-management route test
- Optional Test: `locallife/api/casbin_enforcer_test.go` only if extending middleware coverage

### Tasks

- [ ] Add route-level regression tests for both endpoints as a non-admin user.
  - Build requests with a valid bearer token for a normal user.
  - `PATCH /v1/food-safety/merchants/123/suspend` with JSON body:
    - `merchant_id: 123`
    - `reason: "security regression test"`
    - `duration_hours: 1`
    - `admin_id: <normal user id>`
  - `POST /v1/fraud/detect` with JSON body:
    - `claim_id: 1`
  - Expected: HTTP 403.
  - Expected: `CircuitBreakMerchant` / fraud detector dependency is not reached. If the current test harness cannot observe that directly, assert the store methods that would be called by those handlers have no expectations.

- [ ] Run the new tests before implementation.

```bash
cd locallife
go test ./api -run 'TestRiskManagement(AdminOnly|Authz)|TestFoodSafetySuspendAdminOnly|TestFraudDetectAdminOnly' -count=1
```

Expected before fix: at least one test fails because a non-admin request reaches the handler path instead of returning 403.

- [ ] Protect the exact sensitive routes in `locallife/api/server.go`.

Recommended route shape:

```go
foodSafetyGroup := authGroup.Group("/food-safety")
{
    foodSafetyGroup.POST("/report", server.ReportFoodSafety)
    foodSafetyGroup.PATCH("/merchants/:id/suspend", server.CasbinRoleMiddleware(RoleAdmin), server.SuspendMerchant)
}

fraudGroup := authGroup.Group("/fraud")
{
    fraudGroup.POST("/detect", server.CasbinRoleMiddleware(RoleAdmin), server.TriggerFraudDetection)
}
```

- [ ] Add positive admin tests for both endpoints.
  - Use an admin token or role fixture consistent with existing `CasbinRoleMiddleware` tests.
  - It is enough to prove the admin request passes middleware and reaches the handler path; do not require a full fraud/suspend business success if mocking that would make the test brittle.

- [ ] Run focused validation.

```bash
cd locallife
go test ./api -run 'TestRiskManagement(AdminOnly|Authz)|TestFoodSafetySuspendAdminOnly|TestFraudDetectAdminOnly|TestCasbin' -count=1
```

### Acceptance

- Non-admin authenticated users receive 403 for both sensitive routes.
- Admin users are not blocked by the new middleware.
- `POST /v1/food-safety/report` remains authenticated-user accessible.
- No global Casbin behavior is introduced.

## Finding 2: OCR Idempotency Key Can Collide Across Canonical Requests

### Background

`POST /v1/ocr/jobs` currently:

- Accepts user-provided `idempotency_key`: `locallife/api/ocr.go:1117`.
- Builds a predictable default key only when absent: `locallife/api/ocr.go:1119`.
- Calls `UpsertOCRJob`: `locallife/api/ocr.go:1122-1133`.

The SQL query only conflicts on `idempotency_key` and returns the existing row:

- `locallife/db/query/ocr_job.sql:1-18`

Returned rows are later used to mark owner pending, enqueue OCR work, audit, and respond:

- `locallife/api/ocr.go:1138-1152`
- `locallife/api/ocr.go:1153-1173`
- `locallife/api/ocr.go:86-101`

Default key is predictable:

- `locallife/ocr/service.go:185-188`

Existing access checks on `getOCRJob`, `getOCRJobResult`, `retryOCRJob`, and `batchQueryOCRJobs` call `canAccessOCRJob`, but create-time conflict return bypasses that by returning the conflicted job immediately.

### Correct Invariant

An OCR idempotency hit is valid only when the existing row matches the same canonical request:

- `document_type`
- `provider`
- `media_asset_id`
- `owner_type`
- `owner_id`
- `side`
- request actor or an explicitly designed actor scope

Same key with different canonical fields must return 409 Conflict and must not mark pending, enqueue, or return the old job.

### Files

- Modify: `locallife/api/ocr.go`
- Modify: `locallife/db/query/ocr_job.sql`
- Regenerate: `locallife/db/sqlc/ocr_job.sql.go` via `make sqlc`
- Test: `locallife/api/ocr_test.go`
- Optional Logic Test: `locallife/ocr/service_test.go` only if `ocr.Service.CreateJob` needs the same guard

### Preferred Fix Shape

Keep the natural OCR resource key in `ocr_jobs.idempotency_key`, but make the SQL conflict conditional.

Replace the current no-op conflict update with a guarded conflict that only returns a row when canonical fields match. One workable shape is:

```sql
ON CONFLICT (idempotency_key) DO UPDATE
SET updated_at = ocr_jobs.updated_at
WHERE ocr_jobs.document_type = EXCLUDED.document_type
  AND ocr_jobs.provider = EXCLUDED.provider
  AND ocr_jobs.media_asset_id = EXCLUDED.media_asset_id
  AND ocr_jobs.owner_type = EXCLUDED.owner_type
  AND ocr_jobs.owner_id = EXCLUDED.owner_id
  AND ocr_jobs.side = EXCLUDED.side
  AND ocr_jobs.requested_by = EXCLUDED.requested_by
RETURNING *;
```

Then map the resulting no-row condition for an insert conflict to a stable 409 error at the API/logic boundary.

Implementation note:

- PostgreSQL `ON CONFLICT DO UPDATE ... WHERE` can return zero rows when the conflict exists but the `WHERE` predicate is false.
- sqlc `:one` will surface that as no rows. Treat that branch as "idempotency key conflict", not "OCR job not found".
- If `ocr.Service.CreateJob` also calls `UpsertOCRJob`, ensure it returns a typed conflict error rather than leaking `pgx.ErrNoRows`.

### Tasks

- [ ] Add a failing API test: `TestCreateOCRJob_IdempotencyKeyConflictDifferentOwnerReturnsConflict`.
  - Current user owns a valid application and media asset.
  - Request includes `idempotency_key: "shared-key"`.
  - Mock/store returns the equivalent of an existing OCR row with the same key but a different `owner_id` or `media_asset_id`, or after SQL change returns `db.ErrRecordNotFound`/`pgx.ErrNoRows` from `UpsertOCRJob`.
  - Expected response: HTTP 409.
  - Expected no call to `markOCRPending` side effects and no task enqueue.

- [ ] Add a failing API test: `TestCreateOCRJob_IdempotencyKeyReplaySameRequestReturnsExistingJob`.
  - Same key and same canonical fields.
  - Existing job is returned.
  - Expected response: HTTP 200 with that job.
  - If existing job status is `succeeded`, do not enqueue.
  - If existing job status is `pending`, preserve current pending behavior only for the same canonical request.

- [ ] Add or update SQL-backed test in `locallife/db/sqlc` if practical:
  - Insert OCR job with key `ocr-key-1`.
  - Call `UpsertOCRJob` with the same key and same canonical fields: returns existing row.
  - Call `UpsertOCRJob` with the same key and different `owner_id` or `media_asset_id`: returns no rows / conflict branch expected by upper layer.

- [ ] Modify `locallife/db/query/ocr_job.sql` to guard the conflict update with canonical field equality.

- [ ] Run SQL regeneration.

```bash
cd locallife
make sqlc
```

- [ ] Map the guarded-conflict no-row branch to a stable API 409.
  - Preferred public error: `idempotency key conflicts with a different OCR request`.
  - Do not return `ocr job not found`.
  - Do not log this as an internal server error; it is a caller conflict.

- [ ] Run focused validation.

```bash
cd locallife
go test ./api -run 'TestCreateOCRJob_IdempotencyKey' -count=1
go test ./db/sqlc -run 'TestUpsertOCRJob|TestOCRJob' -count=1
```

- [ ] Run generated check if available in this repo state.

```bash
cd locallife
make check-generated
```

### Acceptance

- Same-key same-request replay returns the same OCR job or equivalent response.
- Same-key different-request collision returns 409.
- Collision does not enqueue OCR work, mark an unrelated owner pending, write misleading audit metadata, or leak another user's OCR job fields.
- `getOCRJob`, `getOCRJobResult`, `retryOCRJob`, and `batchQueryOCRJobs` existing access checks remain intact.

## Finding 3: Refund Creation Lacks Request-Level Replay Guard

### Background

`POST /v1/refunds`:

- Handler has no idempotency key input: `locallife/api/payment_order.go:1183-1236`.
- `CreateRefundOrderInput` has no replay key: `locallife/logic/interfaces.go:205-212`.
- `RefundService.CreateRefundOrder` generates a fresh `out_refund_no` on every call: `locallife/logic/refund_service.go:123-133`.
- `CreateRefundOrderTx` locks the payment order and counts `pending/processing/success` refunds to prevent over-refund: `locallife/db/sqlc/tx_refund.go:35-92`.
- `refund_orders.out_refund_no` is unique and must remain the external provider contract key: `locallife/db/migration/000011_add_payment_orders.up.sql:141`.

The existing over-refund guard is good. The missing piece is request replay semantics: a client retry of the same partial refund request can create another refund order with a new `out_refund_no` as long as remaining refundable amount allows it.

Project standards already classify this as a candidate:

- `.github/standards/backend/IDEMPOTENCY_STANDARDS.md:43-62`
- `artifacts/idempotency-scope-inventory-2026-04-25.md:46`

### Correct Invariant

For merchant-created refunds, the same actor and same `Idempotency-Key` for the same canonical refund request must return the same refund order or equivalent response.

Same actor and same `Idempotency-Key` with different canonical refund fields must return 409 Conflict.

The request-level replay guard must not replace:

- `refund_orders.out_refund_no`
- provider command/fact records
- callback/query/recovery idempotency
- `CreateRefundOrderTx` over-refund protection

### Files

Expected files, unless implementation discovers a narrower existing helper:

- Modify: `locallife/api/payment_order.go`
- Modify: `locallife/logic/interfaces.go`
- Modify: `locallife/logic/refund_service.go`
- Modify/Create SQL: `locallife/db/query/refund_order.sql`
- Create migration: `locallife/db/migration/<next>_add_refund_request_idempotency.up.sql`
- Create migration: `locallife/db/migration/<next>_add_refund_request_idempotency.down.sql`
- Modify generated sqlc after `make sqlc`
- Test: `locallife/api/payment_order_test.go`
- Test: `locallife/logic/refund_service_test.go`
- Test: `locallife/db/sqlc/tx_refund_test.go` or a new SQL test for the request binding

### Preferred Data Model

Use a narrow binding table instead of overloading `refund_orders.out_refund_no`:

```sql
CREATE TABLE refund_request_idempotency (
    id              bigserial PRIMARY KEY,
    operation_scope text        NOT NULL,
    actor_user_id   bigint      NOT NULL REFERENCES users(id),
    idempotency_key text        NOT NULL,
    request_hash    text        NOT NULL,
    refund_order_id bigint      NOT NULL REFERENCES refund_orders(id),
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT refund_request_idempotency_scope_key_uq
        UNIQUE (operation_scope, actor_user_id, idempotency_key)
);

CREATE INDEX refund_request_idempotency_refund_order_id_idx
    ON refund_request_idempotency(refund_order_id);
```

Canonical request hash fields for `POST /v1/refunds`:

- `payment_order_id`
- `refund_type`
- `refund_amount`
- trimmed `refund_reason`
- `actor_user_id`

Recommended `operation_scope`:

- `merchant_refund_create`

Response reconstruction:

- Store only `refund_order_id`, not full response snapshots.
- On replay, load the refund order through existing ownership-aware service path or direct internal lookup after confirming actor/scope/hash.

### Tasks

- [ ] Add API tests for header behavior.
  - `TestCreateRefundOrderAPI_RequiresIdempotencyKey`:
    - Missing `Idempotency-Key` returns 400.
    - Or, if product decides optional rollout is safer, missing key keeps old behavior but logs/metrics are added. Choose one policy before coding and document it in this file.
  - `TestCreateRefundOrderAPI_PassesTrimmedIdempotencyKeyToLogic`:
    - Header `" refund-key-1 "` becomes `"refund-key-1"`.
    - Logic input includes the key.

- [ ] Add logic tests for same-key replay.
  - First call:
    - Creates refund order through `CreateRefundOrderTx`.
    - Calls `CreateBaofuRefund`.
    - Records idempotency binding to the refund order.
  - Second call with same actor/key/hash:
    - Returns the existing refund order.
    - Does not call `CreateRefundOrderTx`.
    - Does not call `CreateBaofuRefund`.

- [ ] Add logic test for same-key different-request conflict.
  - Existing binding has same actor/key but different `request_hash`.
  - Expected: HTTP 409 via `NewRequestError`.
  - Expected: no provider call.

- [ ] Add logic test for provider failure after local refund row creation.
  - If `CreateBaofuRefund` returns a provider/transport failure and the local refund is marked failed, decide the replay semantics deliberately:
    - Safer default: same key returns the same failed refund order and does not create another provider call; operator/user can use a separate explicit retry path.
    - If product requires same-key retry after failure, the binding must model status and retry ownership explicitly. Do not implement implicit duplicate create.

- [ ] Add migration and SQL queries.
  - Query by `(operation_scope, actor_user_id, idempotency_key)`.
  - Insert binding after refund order creation in the same transaction if possible.
  - If binding insertion races, load existing binding and compare hash.

- [ ] Consider transaction ownership.
  - Best option: extend `CreateRefundOrderTx` to accept optional idempotency fields and create `refund_orders` + binding in the same transaction.
  - Avoid creating the refund order, calling provider, then discovering the idempotency binding conflicts.
  - Continue locking the payment order and counting pending/processing/success refunds as today.

- [ ] Run SQL regeneration.

```bash
cd locallife
make sqlc
```

- [ ] Wire API header to logic.
  - Header name: `Idempotency-Key`.
  - Empty after trim: stable 400 if required.
  - Excessively long key: stable 400; recommended max 128 or 256 chars.
  - Same-key different-hash: stable 409.

- [ ] Run focused validation.

```bash
cd locallife
go test ./api -run 'TestCreateRefundOrderAPI' -count=1
go test ./logic -run 'TestCreateRefundOrder_.*Idempot|TestCreateRefundOrder_BaofuPreShareRefund' -count=1
go test ./db/sqlc -run 'TestCreateRefundOrderTx|TestRefundRequestIdempotency' -count=1
make check-generated
```

### Acceptance

- Same actor/key/same canonical refund request returns the original refund order.
- Same actor/key/different canonical request returns 409.
- Replayed request does not create a second `refund_orders` row, does not generate a second `out_refund_no`, and does not call Baofoo again.
- Existing over-refund guard still counts `pending/processing/success` refunds.
- `out_refund_no` remains present and unique in `refund_orders`.
- Callback/query/recovery behavior is not weakened.

## Suggested Execution Order

1. Fix Finding 1 first. It is the highest immediate authz exposure and should be a small route/test change.
2. Fix Finding 2 second. It closes cross-tenant OCR information leakage and wrong-job side effects.
3. Fix Finding 3 third. It may require migration/sqlc/API/logic coordination and should not be mixed with the first two.

## Validation Closeout Checklist

Before claiming this package fixed:

- [ ] State risk class G3 in the final handoff.
- [ ] List exact files changed.
- [ ] State whether `make sqlc`, `make mock`, `make swagger`, and `make check-generated` were required and run.
- [ ] Run the focused tests listed in each completed task.
- [ ] Run `git diff --check`.
- [ ] For any skipped integration or provider validation, name the exact unverified branch.
- [ ] Confirm no unrelated dirty worktree changes were reverted or absorbed.

## Existing Dirty Worktree Note

At planning time, the worktree already contained unrelated changes under `merchant_app/`, `weapp/`, and artifacts. Do not revert or clean those while implementing this backend plan. Only touch backend files and this plan's required migration/generated files.
