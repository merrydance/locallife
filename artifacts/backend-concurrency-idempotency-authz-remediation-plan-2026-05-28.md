# Backend Concurrency Idempotency Authz Remediation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the reviewed backend concurrency, idempotency, and object-level authorization gaps without weakening existing money, recovery, group, merchant, or risk-management invariants.

**Architecture:** Keep transport authorization and client-facing error semantics in `locallife/api/**`, business permission and replay semantics in `locallife/logic/**`, and concurrency correctness in SQL/transaction boundaries under `locallife/db/**`. Route middleware can prevent broad access, but state-machine correctness must be enforced by row locks, conditional updates, constraints, and typed conflicts at the database transaction layer.

**Tech Stack:** Go, gin, pgx/sqlc, PostgreSQL, gomock, LocalLife backend standards, structured zerolog logs, audit log writer, sqlc generated code.

**Risk class:** G3. This work touches authorization, tenant boundaries, recovery/payment-adjacent merchant operations, group affiliation state machines, idempotency, and privacy-sensitive risk flags.

---

## 1. Review Scope And Re-Review Verdict

This document rechecks the five findings from the focused backend review on current `main` as of 2026-05-28.

All five findings are confirmed real. None should be downgraded, deferred, or handled as “frontend-only”. The implementation must close the backend invariant at the smallest durable layer:

| ID | Finding | Re-review verdict | Treatment |
| --- | --- | --- | --- |
| F1 | Merchant claim/recovery routes accept any active merchant staff | True positive | G3 authz fix |
| F2 | `/merchants/me` write routes accept any active merchant staff | True positive | G3 authz fix |
| F3 | Concurrent group application approval can create duplicate groups | True positive | G3 concurrency/idempotency fix |
| F4 | Concurrent group join approvals can approve multiple requests and overwrite affiliation | True positive | G3 concurrency/idempotency fix |
| F5 | Merchant risk lookup can enumerate arbitrary user risk flags | True positive | G3 object-scope authz/privacy fix |

## 1.1 Implementation Closeout 2026-05-28

All five confirmed findings have been implemented as backend fixes. No finding was downgraded to frontend-only or documentation-only handling.

| ID | Implemented backend guard | User-facing response | Observability |
| --- | --- | --- | --- |
| F1 | Merchant claim/recovery routes now require owner/manager, and recovery payment requires owner. | Stable 403 `APIError` messages for recovery read/action and payment-only denial. | Structured security rejection log plus audit metadata for denied staff attempts. |
| F2 | `/merchants/me` write routes now require owner/manager while read routes remain separate; membership settings remains owner-only. | Stable 403 `APIError` messages for missing merchant association, inactive account, missing region, low-privilege role, and owner-only membership settings. | Shared merchant-staff rejection logging/audit helper records actor, merchant, role, endpoint, and reason. |
| F3 | Group application review uses row locking and submitted-only conditional review before creating group side effects. | Stable 409 `ErrGroupApplicationReviewConflict`. | Conflict paths emit structured warning/audit records. |
| F4 | Group join approval locks the join request and merchant affiliation, approves pending rows only, and attaches only unassigned merchants. Reject uses pending-only conditional update. | Stable 409 `ErrGroupJoinRequestReviewConflict` or `ErrMerchantAlreadyJoinedGroup`. | Conflict paths emit structured warning/audit records. |
| F5 | Merchant risk lookup now requires owner/manager and verifies the target user has ordered from the current merchant before checking blocklist state. | Stable 403 `ErrMerchantRiskAccessDenied` for role or relationship denial. | Relationship denials emit structured warning/audit records. |

Additional payment-domain idempotency guard discovered during validation:

- Baofu `bind_sub_config` repeat errors are treated as idempotent success only when a matching local `baofu_bind_sub_config` command exists for the same `subMchID`.
- Missing local command evidence is not downgraded to success; the caller receives the existing safe provider-facing message and the provider error remains loggable.
- Baofu account-open callback state application no longer blocks callback ACK on synchronous merchant-report continuation; recovery remains explicit through the onboarding/report flow.

Validation evidence from `locallife/`:

- `go test ./api -count=1`
- `go test ./logic -count=1`
- `go test ./db/sqlc -count=1`
- `make check-generated`

## 2. Global Non-Negotiables

- Do not downgrade any finding to documentation-only, frontend-only, operational training, or “acceptable risk”.
- Do not rely on frontend role hiding, button disabling, or double-click prevention.
- Do not rely on handler-level “check then update” for concurrent state transitions. The durable guard must live in SQL or inside a transaction with row locks and conditional updates.
- Do not introduce global Casbin behavior as a broad shortcut. Protect exact route groups or handlers and keep object-level checks explicit.
- Do not return internal SQL, pgx, provider, stack, or ownership details to clients.
- Every new error path in this remediation must have both:
  - a structured log or audit trail appropriate for the layer; and
  - a stable, semantic frontend message, preferably via `APIError` constants.
- Avoid duplicate logging for the same unexpected 5xx. Use `internalError(ctx, err)` or `loggedServerError(...)` at the HTTP boundary.
- For expected 4xx authz/conflict denials introduced by this plan, add a `Warn`-level security/business log or `writeAuditLog` record with `request_id`, actor id, merchant/group/request ids, action, and reason.
- If `locallife/db/query/*.sql` changes, run `make sqlc` and generated checks.
- If route Swagger annotations or public API shape change, run `make swagger`.

## 3. Existing Error And Logging Contract

Use the existing contract from `.github/standards/backend/ERROR_HANDLING.md`:

- Business rule violations in `logic/` return `logic.NewRequestError(4xx, errors.New("semantic public message"))`.
- Unexpected infrastructure failures are wrapped with context and returned as plain errors.
- HTTP handlers call `writeLogicRequestError(ctx, err)` for `logic.RequestError`; otherwise call `internalError(ctx, err)`.
- API-layer unexpected failures use `internalError(ctx, err)`.
- Public `4xx` messages must be stable and meaningful for frontend display.
- Public `5xx` messages must be safe and generic, while logs keep internal details.

For this remediation, expected permission and concurrency denials are security-relevant, so add explicit audit/log entries in addition to the normal response path.

## 4. Context To Read Before Implementation

From repo root `/home/sam/locallife`, read these first:

1. `.github/copilot-instructions.md`
2. `.github/README.md`
3. `locallife/AGENTS.md`
4. `.github/instructions/backend-locallife.instructions.md`
5. `.github/instructions/backend-api.instructions.md`
6. `.github/instructions/backend-logic.instructions.md`
7. `.github/instructions/backend-db-query.instructions.md`
8. `.github/instructions/backend-db-sqlc.instructions.md`
9. `.github/prompts/backend-bugfix.prompt.md`
10. `.github/prompts/backend-sql-review.prompt.md`
11. `.github/standards/backend/ERROR_HANDLING.md`
12. `.github/standards/backend/IDEMPOTENCY_STANDARDS.md`
13. `.github/standards/backend/GO_PRACTICES.md`
14. `.github/standards/backend/SQL_STANDARDS.md`

Then read only the files named in the active task below.

## 5. Shared Implementation Building Blocks

### 5.1 Stable API Errors

Add explicit `APIError` constants in `locallife/api/apierrors.go` for new frontend-visible cases. Suggested messages:

- `ErrMerchantStaffPermissionDenied`: `当前员工角色无权执行该商户操作，请联系店主或管理员处理`
- `ErrMerchantRecoveryOwnerRequired`: `仅店主或授权管理员可查看或处理索赔追偿`
- `ErrMerchantRecoveryPaymentOwnerRequired`: `仅店主可发起追偿支付`
- `ErrMerchantRiskAccessDenied`: `仅店主或管理员可查看与本店交易相关顾客的风险提示`
- `ErrGroupApplicationReviewConflict`: `该集团申请状态已变化，请刷新后查看最新审核结果`
- `ErrGroupJoinRequestReviewConflict`: `该加入申请状态已变化，请刷新后查看最新审核结果`
- `ErrMerchantAlreadyJoinedGroup`: `该门店已加入其他集团，请刷新后查看最新归属`

Use unused code ranges in the existing 403/409 sections. Do not reuse duplicate numeric codes.

### 5.2 Security/Audit Logging Helper

Prefer small, explicit helpers over scattered ad hoc log fields. Add a helper only if it reduces repetition.

Candidate file: `locallife/api/security_audit.go`

Responsibilities:

- Log expected authz/conflict denials with `log.Warn()`.
- Include `request_id`, path, method, actor user id, action, target type/id, selected merchant id or group id where relevant, and machine-readable reason.
- For tenant/object boundary denials, also call `server.writeAuditLog(...)` when the actor is authenticated and the target id is meaningful.
- Do not call this helper for unexpected 5xx; those go through `internalError`.

Suggested helper shape:

```go
func (server *Server) logSecurityRejection(ctx *gin.Context, input securityRejectionInput) {
    log.Warn().
        Str("request_id", GetRequestID(ctx)).
        Str("path", ctx.Request.URL.Path).
        Str("method", ctx.Request.Method).
        Int64("actor_user_id", input.ActorUserID).
        Str("action", input.Action).
        Str("target_type", input.TargetType).
        Int64("target_id", input.TargetID).
        Int64("merchant_id", input.MerchantID).
        Int64("group_id", input.GroupID).
        Str("reason", input.Reason).
        Msg("security request rejected")

    if input.Audit {
        server.writeAuditLog(ctx, AuditLogInput{
            ActorUserID: input.ActorUserID,
            ActorRole:   input.ActorRole,
            Action:      input.Action,
            TargetType:  input.TargetType,
            TargetID:    int64Ptr(input.TargetID),
            Metadata: map[string]any{
                "reason":      input.Reason,
                "path":        ctx.Request.URL.Path,
                "method":      ctx.Request.Method,
                "merchant_id": input.MerchantID,
                "group_id":    input.GroupID,
            },
        })
    }
}
```

If existing helpers already provide `int64Ptr`, reuse them. Otherwise create a local unexported helper or inline the pointer assignment.

## 6. Finding F1: Merchant Claim/Recovery Routes Over-Authorize Active Staff

### Re-Review Verdict

True positive.

Evidence:

- `locallife/api/server.go:1006-1018` creates `merchantClaimsGroup := authGroup.Group("/merchant")` and registers claim/recovery routes without `MerchantStaffMiddleware`.
- `locallife/api/recovery_dispute.go:27-28` resolves merchant via `resolveMerchantForUser`.
- `locallife/api/claim_recovery.go:230-239` allows payment creation after only resolving the associated merchant.
- `locallife/api/permission_helpers.go:70-99` treats owned merchants and all active staff merchants as accessible.
- `locallife/db/query/merchant_staff.sql:38-46` confirms `ListMerchantsByStaff` returns any active staff and `GetUserMerchantRole` exists but is not used by these routes.

### Impact

Low-privilege active staff such as `chef` or `cashier` can:

- list and inspect merchant claims;
- view recovery records;
- initiate recovery payment creation;
- create recovery disputes;
- view dispute details.

This is financial/recovery-adjacent and may expose claim details, recovery amounts, behavior summaries, and platform adjudication data.

### Required Invariant

- Read-only merchant claim/recovery list/detail endpoints require at least `owner` or `manager`.
- Dispute creation requires at least `owner` or `manager`.
- Recovery payment creation requires `owner` unless product explicitly defines a `finance` merchant staff role. Current merchant staff roles are `owner`, `manager`, `chef`, `cashier`, so use `owner` for payment.
- Denied staff receive stable 403 with a frontend-meaningful message.
- Denied attempts are logged/audited with actor, merchant, endpoint, role, and action.

### Files

- Modify: `locallife/api/server.go`
- Modify: `locallife/api/apierrors.go`
- Modify or create: `locallife/api/security_audit.go`
- Test: `locallife/api/claim_recovery_authz_test.go`
- Test: `locallife/api/recovery_dispute_authz_test.go`

### Tasks

- [ ] Add API errors for claim/recovery staff denials.

- [ ] Add or reuse a security rejection logging helper.

- [ ] Add failing route-level tests proving `chef` and `cashier` cannot reach:
  - `GET /v1/merchant/claims`
  - `GET /v1/merchant/recoveries/:id`
  - `POST /v1/merchant/recoveries/:id/pay`
  - `POST /v1/merchant/recovery-disputes`

Expected before fix: requests currently pass route middleware and reach handler/store expectations.

- [ ] Split `merchantClaimsGroup` into role-scoped subgroups.

Recommended shape:

```go
merchantClaimsReadGroup := authGroup.Group("/merchant")
merchantClaimsReadGroup.Use(server.MerchantStaffMiddleware("owner", "manager"))
{
    merchantClaimsReadGroup.GET("/claims", server.listMerchantClaims)
    merchantClaimsReadGroup.GET("/claims/summary", server.listMerchantClaimsSummary)
    merchantClaimsReadGroup.GET("/claims/:id", server.getMerchantClaimDetail)
    merchantClaimsReadGroup.GET("/claims/:id/decision", server.getMerchantClaimDecision)
    merchantClaimsReadGroup.GET("/claims/behavior-summary", server.getMerchantClaimBehaviorSummary)
    merchantClaimsReadGroup.GET("/recoveries/:id", server.getMerchantClaimRecovery)
    merchantClaimsReadGroup.POST("/recovery-disputes", server.createMerchantRecoveryDispute)
    merchantClaimsReadGroup.GET("/recovery-disputes", server.listMerchantRecoveryDisputes)
    merchantClaimsReadGroup.GET("/recovery-disputes/summary", server.listMerchantRecoveryDisputesSummary)
    merchantClaimsReadGroup.GET("/recovery-disputes/:id", server.getMerchantRecoveryDisputeDetail)
}

merchantClaimsPaymentGroup := authGroup.Group("/merchant")
merchantClaimsPaymentGroup.Use(server.MerchantStaffMiddleware("owner"))
{
    merchantClaimsPaymentGroup.POST("/recoveries/:id/pay", server.payMerchantClaimRecovery)
}
```

- [ ] If `MerchantStaffMiddleware` returns generic `ErrPermissionDenied`, ensure frontend receives the new claim/recovery-specific error for these routes. The simplest safe pattern is a route-local wrapper middleware, for example:

```go
func (server *Server) MerchantStaffMiddlewareWithError(publicErr error, allowedRoles ...string) gin.HandlerFunc
```

Keep it small and do not alter existing route behavior globally unless tests cover all affected route groups.

- [ ] Add tests for owner/manager access to read/dispute routes and owner access to payment route.

- [ ] Add tests that denied attempts produce either an audit log entry or a structured security rejection log. If log capture is brittle, inject/spy on `AuditWriter` for route-level tests and require log behavior in helper unit tests.

### Validation

Run from `locallife/`:

```bash
go test ./api -run 'TestMerchantClaimRecoveryAuthz|TestRecoveryDisputeAuthz|TestMerchantStaffMiddleware' -count=1
```

If route annotations change:

```bash
make swagger
make check-generated
```

## 7. Finding F2: `/merchants/me` Write Routes Over-Authorize Active Staff

### Re-Review Verdict

True positive.

Evidence:

- `locallife/api/server.go:667-676` registers `/merchants/me` write/config routes directly on `authGroup`.
- `locallife/api/merchant.go:286`, `:525`, and `:784` resolve merchant with `resolveMerchantForUser`.
- `locallife/api/tag.go:250` does the same for merchant category tags.
- `resolveMerchantForUser` uses the same active-staff-accessible path described in F1.
- `updateMerchantMembershipSettings` already uses owner-only logic, so it is not the vulnerable write route in this group.

### Impact

Any active staff member can update high-impact merchant configuration:

- merchant profile fields and logo;
- open/closed status;
- business hours and auto-open behavior;
- merchant category tags.

That can change storefront visibility, order acceptance, search/category placement, and customer behavior.

### Required Invariant

- Merchant profile, open status, business hours, and tags require `owner` or `manager`.
- Owner-only flows remain owner-only and are not weakened.
- Denied staff receive stable 403 with clear frontend text.
- Denied attempts are logged/audited.
- Existing optimistic lock on profile update remains intact.

### Files

- Modify: `locallife/api/server.go`
- Modify: `locallife/api/apierrors.go`
- Modify or create: `locallife/api/security_audit.go`
- Test: `locallife/api/merchant_authz_test.go`
- Test: `locallife/api/tag_authz_test.go`

### Tasks

- [ ] Add failing tests for a `chef` or `cashier` attempting:
  - `PATCH /v1/merchants/me`
  - `PATCH /v1/merchants/me/status`
  - `PUT /v1/merchants/me/business-hours`
  - `PUT /v1/merchants/me/tags`

- [ ] Group the write routes under `MerchantStaffMiddleware("owner", "manager")`.

Recommended route shape:

```go
merchantProfileWriteGroup := authGroup.Group("/merchants/me")
merchantProfileWriteGroup.Use(server.MerchantStaffMiddleware("owner", "manager"))
{
    merchantProfileWriteGroup.PATCH("", server.updateCurrentMerchant)
    merchantProfileWriteGroup.PATCH("/status", server.updateMerchantOpenStatus)
    merchantProfileWriteGroup.PUT("/business-hours", server.setMerchantBusinessHours)
    merchantProfileWriteGroup.PUT("/tags", server.setMerchantTags)
}
```

Keep these routes still reachable at the same public paths.

- [ ] Keep read-only routes separate:
  - `GET /merchants/me`
  - `GET /merchants/my`
  - `GET /merchants/me/status`
  - `GET /merchants/me/business-hours`
  - `GET /merchants/me/tags`

- [ ] Decide whether `PATCH /merchants/me/shop-images` is onboarding/application-owner behavior or merchant staff behavior. Do not silently broaden it. If unchanged, document why in the implementation PR. If fixed, add tests and route it to owner/manager as well.

- [ ] Ensure denied attempts have a structured log or audit record.

- [ ] Add positive tests for owner and manager on the four write routes.

### Validation

Run from `locallife/`:

```bash
go test ./api -run 'TestMerchant.*Authz|TestMerchantTagsAuthz|TestMerchantStaffMiddleware' -count=1
```

## 8. Finding F3: Group Application Approval Can Create Duplicate Groups

### Re-Review Verdict

True positive.

Evidence:

- `locallife/api/group.go:614-627` checks application status before entering the transaction.
- `locallife/db/sqlc/tx_group.go:26-31` reads the application inside the transaction without `FOR UPDATE`.
- `locallife/db/sqlc/tx_group.go:34-56` creates a merchant group and owner member before final status update.
- `locallife/db/query/group.sql:15-17` is a plain select.
- `locallife/db/query/group.sql:85-93` updates by `id` only, without `AND status = 'submitted'`.
- `locallife/db/migration/000093_add_group_multi_store.up.sql` has no application-to-group uniqueness anchor.

### Impact

Concurrent admin approvals can both observe `submitted`, create multiple active groups, create multiple owner memberships, and then both mark the application approved. An approve/reject race can also leave an inconsistent “rejected application with group side effects” pattern.

### Required Invariant

- Exactly one terminal review operation can claim a submitted group application.
- A group is created only after the approval operation successfully claims the submitted application.
- A repeated approval after the application is already approved returns a stable conflict or idempotent existing result. For this codebase, use stable `409 Conflict` unless product explicitly wants replay.
- A reject racing an approval must not overwrite an already-approved application.
- All conflicts are logged with application id, reviewer id, requested status, and current state when available.
- Frontend receives `409` with `ErrGroupApplicationReviewConflict`.

### Files

- Modify: `locallife/db/query/group.sql`
- Modify: `locallife/db/sqlc/tx_group.go`
- Modify: `locallife/api/group.go`
- Modify: `locallife/api/apierrors.go`
- Modify or create: `locallife/api/security_audit.go`
- Test: `locallife/db/sqlc/tx_group_test.go`
- Test: `locallife/api/group_test.go`

### Tasks

- [ ] Add SQL query `GetGroupApplicationForUpdate`.

```sql
-- name: GetGroupApplicationForUpdate :one
SELECT id, applicant_user_id, group_name, contact_phone, license_number, address, region_id, status, reject_reason, reviewed_by, reviewed_at, application_data, created_at, updated_at, license_media_asset_id
FROM merchant_group_applications
WHERE id = $1
FOR UPDATE;
```

- [ ] Replace `GetGroupApplication` inside `ApproveGroupApplicationTx` with `GetGroupApplicationForUpdate`.

- [ ] Change `ReviewGroupApplication` or add `ReviewSubmittedGroupApplication` so terminal review updates require `status = 'submitted'`.

Recommended new query:

```sql
-- name: ReviewSubmittedGroupApplication :one
UPDATE merchant_group_applications
SET status = $2,
    reject_reason = $3,
    reviewed_by = $4,
    reviewed_at = $5,
    updated_at = now()
WHERE id = $1
  AND status = 'submitted'
RETURNING *;
```

- [ ] In `ApproveGroupApplicationTx`, claim the submitted row before creating side effects, or keep the row locked and perform the conditional review before `CreateMerchantGroup`. Do not create group records unless the submitted transition succeeds.

Preferred sequence:

1. `GetGroupApplicationForUpdate`.
2. If not `submitted`, return typed conflict.
3. `ReviewSubmittedGroupApplication(... approved ...)`.
4. `CreateMerchantGroup`.
5. `CreateGroupMember`.
6. `CreateGroupAuditLog`.

- [ ] Add a typed db sentinel error, for example `ErrGroupApplicationReviewConflict`, instead of returning bare `errors.New(...)`.

- [ ] Update reject path in `api/group.go` to use the same conditional review query, not the broad `ReviewGroupApplication`.

- [ ] Map the db sentinel to HTTP `409` and `ErrGroupApplicationReviewConflict`; log/audit the conflict.

- [ ] Add transaction tests:
  - approving a submitted application creates exactly one group and owner member;
  - second approval returns conflict and does not create another group;
  - approve followed by reject returns conflict on reject and does not change approved state;
  - reject followed by approve returns conflict on approve and does not create a group.

- [ ] Add an API test that conflict returns `409` with stable frontend message.

### Validation

Run from `locallife/`:

```bash
make sqlc
go test ./db/sqlc -run 'TestApproveGroupApplicationTx|TestReviewGroupApplication' -count=1
go test ./api -run 'TestReviewGroupApplication' -count=1
make check-generated
```

## 9. Finding F4: Group Join Approval Can Approve Multiple Requests And Overwrite Merchant Affiliation

### Re-Review Verdict

True positive.

Evidence:

- `locallife/db/sqlc/tx_group.go:109-118` reads join request without `FOR UPDATE`.
- `locallife/db/sqlc/tx_group.go:121-126` updates request status by id only.
- `locallife/db/sqlc/tx_group.go:131-135` unconditionally writes `merchants.group_id` and `brand_id`.
- `locallife/db/query/group.sql:160-174` has plain read and broad status update.
- `locallife/db/query/group.sql:177-180` updates merchant affiliation by id only.
- `locallife/db/migration/000093_add_group_multi_store.up.sql:77` only constrains `(group_id, merchant_id, status)`, not “one approved group per merchant”.

### Impact

Concurrent approvals of pending join requests for the same merchant can both return success. The final `merchants.group_id` is last-writer-wins, while multiple join request rows and audit logs say approved.

### Required Invariant

- A join request can move from `pending` to `approved` exactly once.
- A merchant can be affiliated to at most one group at a time.
- Approval of a request for a merchant already in a group returns `409` with `ErrMerchantAlreadyJoinedGroup`.
- Re-approving or racing an already-reviewed request returns `409` with `ErrGroupJoinRequestReviewConflict`.
- Conflicts and denials are logged with request id, merchant id, group id, reviewer id, and reason.

### Files

- Modify: `locallife/db/query/group.sql`
- Modify: `locallife/db/query/merchant.sql`
- Modify: `locallife/db/sqlc/tx_group.go`
- Modify: `locallife/api/group.go`
- Modify: `locallife/api/apierrors.go`
- Modify or create: `locallife/api/security_audit.go`
- Test: `locallife/db/sqlc/tx_group_test.go`
- Test: `locallife/api/group_test.go`

### Tasks

- [ ] Add SQL query `GetGroupJoinRequestForUpdate`.

```sql
-- name: GetGroupJoinRequestForUpdate :one
SELECT id, group_id, merchant_id, applicant_user_id, status, reason, reviewed_by, reviewed_at, created_at
FROM merchant_group_join_requests
WHERE id = $1
FOR UPDATE;
```

- [ ] Add SQL query `GetMerchantForUpdate` or a narrow `GetMerchantGroupAffiliationForUpdate`.

```sql
-- name: GetMerchantGroupAffiliationForUpdate :one
SELECT group_id, brand_id
FROM merchants
WHERE id = $1
FOR UPDATE;
```

- [ ] Add conditional join request update.

```sql
-- name: ApprovePendingGroupJoinRequest :one
UPDATE merchant_group_join_requests
SET status = 'approved',
    reviewed_by = $2,
    reviewed_at = $3
WHERE id = $1
  AND status = 'pending'
RETURNING *;
```

Add equivalent `RejectPendingGroupJoinRequest` for reject path.

- [ ] Add conditional merchant affiliation update.

```sql
-- name: AttachMerchantToGroupIfUnassigned :execrows
UPDATE merchants
SET group_id = $2,
    brand_id = $3,
    updated_at = now()
WHERE id = $1
  AND group_id IS NULL;
```

Use `:execrows` so 0 rows becomes a conflict, not a silent success.

- [ ] Update `ApproveGroupJoinRequestTx` sequence:
  1. Lock join request.
  2. Verify group id.
  3. Verify status pending.
  4. Lock merchant affiliation.
  5. If `group_id` is already set, return `ErrMerchantAlreadyJoinedGroup`.
  6. Conditionally approve request.
  7. Attach merchant if unassigned.
  8. Create audit log.

- [ ] Update reject handler to use conditional `RejectPendingGroupJoinRequest`, so reject cannot overwrite an approved request.

- [ ] Add db sentinel errors:
  - `ErrGroupJoinRequestReviewConflict`
  - `ErrMerchantAlreadyJoinedGroup`

- [ ] Map sentinels to `409` with stable API errors and structured log/audit entries.

- [ ] Add db tests:
  - approve pending request succeeds and attaches merchant;
  - second approval of same request conflicts and does not duplicate audit side effects;
  - approval of a different pending request for already-affiliated merchant conflicts;
  - reject after approval conflicts;
  - approve after reject conflicts.

- [ ] Add API tests for `409` responses and frontend messages.

### Validation

Run from `locallife/`:

```bash
make sqlc
go test ./db/sqlc -run 'TestApproveGroupJoinRequestTx|TestGroupJoinRequestReview' -count=1
go test ./api -run 'TestApproveGroupJoinRequest|TestRejectGroupJoinRequest' -count=1
make check-generated
```

## 10. Finding F5: Merchant Risk Lookup Can Enumerate Arbitrary User Risk Flags

### Re-Review Verdict

True positive.

Evidence:

- `locallife/api/server.go:1021-1024` registers `/merchant/risk/users/:id` without `MerchantStaffMiddleware`.
- `locallife/api/behavior_trace.go:44-52` only checks that caller resolves to some merchant, then queries risk blocklist for arbitrary path user id.
- `locallife/db/query/order.sql:132-137` already has `HasUserOrderedFromMerchant`, which can help enforce customer relationship.
- `locallife/db/migration/000019_add_membership_marketing_system.up.sql:13-28` has `merchant_memberships(merchant_id, user_id)` for an optional membership relationship if product wants membership-based lookup too.

### Impact

Any active staff member associated with any merchant can probe whether arbitrary user ids are on the active behavior blocklist and retrieve reason/block-until-style risk hints. This leaks privacy-sensitive risk and claim behavior state.

### Required Invariant

- Risk lookup requires at least `owner` or `manager`.
- Target user must have a legitimate relationship with the selected merchant. Minimum required relationship: `HasUserOrderedFromMerchant(userID, merchantID)` is true.
- Optional allowed relationship if product confirms: active merchant membership in `merchant_memberships`.
- Unknown target user, unrelated user, and blocked-by-authz cases must not reveal whether the user exists or whether a risk flag exists.
- Frontend receives stable semantic 403, for example `ErrMerchantRiskAccessDenied`.
- Denied attempts are logged/audited with actor id, merchant id, target user id, and reason.

### Files

- Modify: `locallife/api/server.go`
- Modify: `locallife/api/behavior_trace.go`
- Modify: `locallife/api/apierrors.go`
- Modify or create: `locallife/api/security_audit.go`
- Optional Modify: `locallife/db/query/membership.sql` if membership relationship is accepted
- Test: `locallife/api/behavior_trace_authz_test.go`

### Tasks

- [ ] Protect `merchantRiskGroup` with `MerchantStaffMiddleware("owner", "manager")`.

```go
merchantRiskGroup := authGroup.Group("/merchant/risk")
merchantRiskGroup.Use(server.MerchantStaffMiddleware("owner", "manager"))
{
    merchantRiskGroup.GET("/users/:id", server.getMerchantUserRisk)
}
```

- [ ] Update `getMerchantUserRisk` to use merchant from context if middleware has already resolved it. Do not re-resolve through broad active-staff helper.

- [ ] Before querying blocklist, verify target relation:

```go
hasOrdered, err := server.store.HasUserOrderedFromMerchant(ctx, db.HasUserOrderedFromMerchantParams{
    UserID:     userID,
    MerchantID: merchant.ID,
})
if err != nil {
    ctx.JSON(http.StatusInternalServerError, internalError(ctx, fmt.Errorf("check merchant user relationship: %w", err)))
    return
}
if !hasOrdered {
    server.logSecurityRejection(ctx, ...)
    ctx.JSON(http.StatusForbidden, errorResponse(ErrMerchantRiskAccessDenied))
    return
}
```

- [ ] If membership relationship is allowed, add a narrowly named query such as `HasMerchantMembershipByUser` and treat `hasOrdered || hasMembership` as authorized. This requires `make sqlc`. If product does not explicitly ask for membership relation, do not add it.

- [ ] Ensure the “no active blocklist record” response remains `200 { has_block: false }` only after relationship is confirmed.

- [ ] Add tests:
  - cashier/chef denied at middleware;
  - owner/manager with no order relationship gets 403 and no blocklist query;
  - owner/manager with order relationship and no blocklist gets `200 has_block=false`;
  - owner/manager with order relationship and active block gets `200 has_block=true`;
  - DB error while checking relationship uses `internalError` and logs once.

### Validation

Run from `locallife/`:

```bash
go test ./api -run 'TestMerchantUserRiskAuthz|TestMerchantUserRiskRelationship' -count=1
```

If SQL changes:

```bash
make sqlc
make check-generated
```

## 11. Cross-Cutting Tests And Validation

After all tasks are implemented, run from `locallife/`:

```bash
go test ./api -run 'TestMerchantClaimRecoveryAuthz|TestRecoveryDisputeAuthz|TestMerchant.*Authz|TestMerchantTagsAuthz|TestMerchantUserRiskAuthz|TestReviewGroupApplication|TestApproveGroupJoinRequest|TestRejectGroupJoinRequest' -count=1
go test ./db/sqlc -run 'TestApproveGroupApplicationTx|TestApproveGroupJoinRequestTx|TestGroup.*Review' -count=1
make check-generated
```

If route annotations or Swagger docs change:

```bash
make swagger
make check-generated
```

If SQL changed:

```bash
make sqlc
make check-generated
```

If time and local DB availability permit, add:

```bash
make test-safety
```

## 12. Acceptance Checklist

- [x] F1 claim/recovery read and dispute routes reject `chef` and `cashier`.
- [x] F1 recovery payment route rejects everyone except `owner`.
- [x] F2 `/merchants/me` write/config routes reject low-privilege staff.
- [x] F3 group application review is claimed once and cannot create duplicate groups.
- [x] F3 approve/reject races produce stable `409`.
- [x] F4 group join approval is claimed once and cannot overwrite existing affiliation.
- [x] F4 multiple pending join requests for one merchant cannot both become effective approvals.
- [x] F5 risk lookup requires owner/manager and a merchant relationship to the target user.
- [x] Every new 4xx denial has a stable APIError message for frontend display.
- [x] Every new authz/conflict denial has structured log or audit coverage.
- [x] Every new unexpected 5xx path uses `internalError`/`loggedServerError`, not `errorResponse`.
- [x] No raw internal, SQL, or provider errors are returned to clients.
- [x] SQL changes regenerated with `make sqlc`.
- [x] Generated artifacts checked with `make check-generated`.

## 13. Residual Risk To Call Out After Implementation

- This plan does not redesign merchant staff roles. If future product needs a finance role, add it explicitly through staff role schema, middleware, UI, and tests before allowing non-owner recovery payments.
- This plan treats risk lookup as a merchant-customer relationship query. If support, platform, or operator users need cross-merchant risk views, create separate admin/operator endpoints with explicit role and audit controls.
- This plan does not add a historical data cleanup for duplicate groups or conflicting group join approvals that may already exist. After code fixes, run a data audit query and handle any existing duplicates through a separate migration/ops plan.
