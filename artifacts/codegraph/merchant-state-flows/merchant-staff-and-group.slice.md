# Merchant Staff And Group Slice

Status: merchant-state flow slice created
Risk class: G3 - merchant staff authorization, reusable invite credentials, group membership boundaries, private identity documents, OCR async writeback, and multi-store affiliation transitions
Scope: Merchant Mini Program staff page and employee bind page -> merchant staff routes -> group onboarding and join pages -> group routes -> OCR worker -> durable staff/group truth

## Variant Coverage

This slice covers:

- Merchant staff list, invite-code generation, role assignment, and staff removal.
- Customer-side employee scan/manual bind entry because it consumes the merchant-generated invite code.
- Merchant-side group application draft, document upload, OCR polling, and submit.
- Merchant-side group search and join-request submission.
- Backend group review and join-request approve/reject/cancel paths where they determine merchant-visible state.

This slice does not fully cover:

- Group policy, menu-template, brand-template, and group administration UIs after a merchant has joined a group.
- Operator/platform group management surfaces except where their review handlers are required to close the merchant-side application path.
- Generic OCR provider internals beyond the group-application owner variant.

## Product Invariant

- A merchant invite code must not grant operational access until the owner assigns a real staff role.
- Removing or reactivating a staff member must converge both `merchant_staff` membership truth and the coarse `user_roles.merchant_staff` capability without partial writes.
- Group application documents and OCR results must remain bound to the applicant's current draft and current media assets.
- A merchant can belong to at most one group. Group join approval must not overwrite an existing affiliation or regress a terminal request.
- Every user-visible success or failure around join-request transitions must match the durable request and audit-log state.

## Primary Forward Chain

1. Merchant staff page loads backend staff truth, generates a reusable invite code, updates roles, and soft-removes staff.
   Evidence: `weapp/miniprogram/pages/merchant/staff/index.ts:165`, `weapp/miniprogram/pages/merchant/staff/index.ts:233`, `weapp/miniprogram/pages/merchant/staff/index.ts:324`, `weapp/miniprogram/pages/merchant/staff/index.ts:377`.

2. Staff wrappers call `GET /v1/merchant/staff`, `POST /invite-code`, `PATCH /:id/role`, and `DELETE /:id`.
   Evidence: `weapp/miniprogram/pages/merchant/_api/merchant-staff.ts:62`, `weapp/miniprogram/pages/merchant/_api/merchant-staff.ts:72`, `weapp/miniprogram/pages/merchant/_api/merchant-staff.ts:79`, `weapp/miniprogram/pages/merchant/_api/merchant-staff.ts:87`.

3. Staff list and invite-code routes allow owner/manager; direct add, role update, and remove routes are owner-only.
   Evidence: `locallife/api/server.go:725`, `locallife/api/server.go:733`.

4. Invite-code generation reuses an unexpired `merchants.bind_code`; otherwise it creates a random 32-character code and stores a 24-hour expiry.
   Evidence: `locallife/api/staff.go:337`, `locallife/api/staff.go:344`, `locallife/api/staff.go:354`, `locallife/api/staff.go:362`, `locallife/api/staff.go:366`.

5. Employee bind page accepts QR/manual code, calls `POST /v1/bind-merchant`, invalidates cached console identity, and redirects pending staff away from the merchant dashboard.
   Evidence: `weapp/miniprogram/pages/user/bind-merchant/index.ts:259`, `weapp/miniprogram/pages/user/bind-merchant/index.ts:269`, `weapp/miniprogram/pages/user/bind-merchant/index.ts:297`, `weapp/miniprogram/api/personal.ts:530`.

6. Bind route resolves the merchant by bind code, verifies expiry, rejects an existing membership row, and inserts `merchant_staff(role='pending', status='active')`. It deliberately does not grant the global `merchant_staff` user role yet.
   Evidence: `locallife/api/server.go:722`, `locallife/api/staff.go:400`, `locallife/api/staff.go:409`, `locallife/api/staff.go:420`, `locallife/api/staff.go:426`, `locallife/api/staff.go:440`.

7. Owner role assignment changes `merchant_staff.role`, keeps the membership active, and then creates or reactivates `user_roles(role='merchant_staff')`.
   Evidence: `locallife/api/staff.go:183`, `locallife/api/staff.go:215`, `locallife/api/staff.go:228`, `locallife/api/staff.go:237`, `locallife/api/staff.go:468`, `locallife/db/query/merchant_staff.sql:48`.

8. Owner removal soft-deletes the membership and then disables the coarse user role when no real active merchant role remains.
   Evidence: `locallife/api/staff.go:266`, `locallife/api/staff.go:292`, `locallife/api/staff.go:304`, `locallife/api/staff.go:311`, `locallife/api/staff.go:500`.

9. Group application page loads or creates the current user's latest draft, uploads private documents, starts OCR jobs, polls refreshed draft truth, and submits with agreement consent.
   Evidence: `weapp/miniprogram/pages/merchant/group/application/index.ts:350`, `weapp/miniprogram/pages/merchant/group/application/index.ts:366`, `weapp/miniprogram/pages/merchant/group/application/index.ts:404`, `weapp/miniprogram/pages/merchant/group/application/index.ts:537`, `weapp/miniprogram/pages/merchant/group/application/index.ts:586`.

10. Group application wrappers call `GET /v1/groups/applications/me`, `PUT /basic`, `DELETE /documents/:type`, and `POST /submit`; OCR upload binds `owner_type='group_application'`.
    Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:128`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:138`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:146`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:158`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:183`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:210`.

11. Group application handlers scope drafts to the authenticated applicant. Update and delete reset a rejected application to draft; submitted/approved applications are not editable.
    Evidence: `locallife/api/server.go:742`, `locallife/api/group.go:336`, `locallife/api/group.go:379`, `locallife/api/group.go:397`, `locallife/api/group.go:438`, `locallife/api/group.go:458`.

12. Generic OCR creation recognizes `group_application`, checks owner identity and media-category constraints, records the current OCR job binding in `merchant_group_applications.application_data`, then enqueues the group OCR worker.
    Evidence: `locallife/api/ocr.go:197`, `locallife/api/ocr.go:240`, `locallife/api/ocr.go:293`, `locallife/api/ocr.go:461`, `locallife/api/ocr_media_authz.go:62`.

13. Group OCR workers guard job payload, draft status, media asset, and current OCR job id before provider execution and before writeback.
    Evidence: `locallife/worker/ocr_writeback_guard.go:240`, `locallife/worker/ocr_writeback_guard.go:256`, `locallife/worker/ocr_writeback_guard.go:269`, `locallife/worker/ocr_writeback_guard.go:273`, `locallife/worker/task_group_application_ocr.go:69`, `locallife/worker/task_group_application_ocr.go:157`.

14. Group application submit requires draft status plus group name and contact phone, writes consent audit, and updates status to `submitted`.
    Evidence: `locallife/api/group.go:542`, `locallife/api/group.go:560`, `locallife/api/group.go:564`, `locallife/api/group.go:569`, `locallife/api/group.go:571`.

15. Admin approval uses a transaction: lock submitted application, conditionally mark approved, create `merchant_groups`, create owner `merchant_group_members`, and insert a group audit log.
    Evidence: `locallife/api/group.go:643`, `locallife/db/sqlc/tx_group.go:22`, `locallife/db/sqlc/tx_group.go:26`, `locallife/db/sqlc/tx_group.go:35`, `locallife/db/sqlc/tx_group.go:49`, `locallife/db/sqlc/tx_group.go:63`, `locallife/db/sqlc/tx_group.go:77`.

16. Merchant group-join page searches active groups and posts a join request with an optional reason.
    Evidence: `weapp/miniprogram/pages/merchant/group/join/index.ts:71`, `weapp/miniprogram/pages/merchant/group/join/index.ts:117`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:221`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:232`.

17. Join-request create route is merchant-owner-only, rejects merchants already attached to a group, requires an active target group, inserts a pending request, then writes an audit log.
    Evidence: `locallife/api/server.go:762`, `locallife/api/group.go:1178`, `locallife/api/group.go:1191`, `locallife/api/group.go:1201`, `locallife/api/group.go:1222`, `locallife/api/group.go:1237`.

18. Join-request approval requires group owner/admin and uses a transaction: lock request, require pending, lock merchant affiliation, require unassigned, conditionally approve request, attach merchant only if still unassigned, and write audit log.
    Evidence: `locallife/api/group.go:1302`, `locallife/api/group.go:1309`, `locallife/db/sqlc/tx_group.go:108`, `locallife/db/sqlc/tx_group.go:112`, `locallife/db/sqlc/tx_group.go:123`, `locallife/db/sqlc/tx_group.go:132`, `locallife/db/sqlc/tx_group.go:144`, `locallife/db/sqlc/tx_group.go:160`.

19. Reject uses a conditional pending update but writes audit afterward. Cancel checks pending in the handler, then uses an unconditional status update and writes audit afterward.
    Evidence: `locallife/api/group.go:1407`, `locallife/api/group.go:1444`, `locallife/api/group.go:1463`, `locallife/api/group.go:1487`, `locallife/api/group.go:1511`, `locallife/api/group.go:1536`, `locallife/api/group.go:1547`, `locallife/api/group.go:1558`, `locallife/db/query/group.sql:189`, `locallife/db/query/group.sql:206`.

## Reverse-Reference Findings

- `merchant_staff` is the merchant-specific authorization truth, while `user_roles(role='merchant_staff')` is a coarse global capability. Direct add, role assignment, and remove mutate these in separate calls without a transaction or recovery job.
- A disabled `merchant_staff` row cannot rejoin through invite binding or direct add because both paths treat any existing row as a conflict. `UpdateMerchantStaffStatus` exists but no runtime reactivation caller was found.
- Invite codes are reusable for up to 24 hours and there is no revoke/rotate action in the traced Mini Program page. This may be intentional for team onboarding, but it is a bearer credential with a wider blast radius than a one-person invite.
- Bind creates `role='pending', status='active'`. `MerchantStaffMiddleware` later blocks roles not explicitly allowed, but `CheckUserHasMerchantAccess` checks only active status. Pending staff can therefore be treated as authorized by role-agnostic downstream checks such as dining-session access.
- Migration `000070_add_staff_pending_status` introduced a `pending` status, but runtime bind uses a pending role plus active status. `UpdateMerchantStaffStatus`, hard delete queries, and count helpers are reverse-reference drift candidates.
- Group OCR is protected by stale asset/job guards before and after provider execution. However, group documents share one `application_data` JSON blob, and writebacks replace that blob after a read/merge cycle. Parallel document OCR completion can lose a sibling write without a JSON merge SQL update, version check, or row lock.
- Group update basic accepts `license_image_asset_id` from the client. The OCR create path validates owner/category binding, but the direct basic-info update path does not visibly apply the same media ownership/category validation.
- Group submit only requires `group_name` and `contact_phone`; backend does not require license, identity documents, successful OCR, address, or region. If those are required for approval, enforcement currently lives only in UI expectations or manual review.
- Group application GET can create a draft and there is no unique active-draft constraint per applicant. Concurrent get-or-create requests can create multiple drafts; all later handlers operate on whichever row is latest.
- Join-request approval is the strongest state transition in this slice. Its transaction prevents affiliation overwrite and duplicate terminal review.
- Join-request create, reject, and cancel write audit logs outside their state writes. An audit-log failure returns `500` after the durable state changed, so retry semantics become misleading.
- Cancel is vulnerable to terminal-state regression: it reads `pending`, then calls unconditional `UpdateGroupJoinRequestStatus`. A concurrent approve or reject can be overwritten to `cancelled`.
- `merchant_group_join_requests` has `UNIQUE(group_id, merchant_id, status)`. This blocks duplicate pending rows, but it also allows only one historical rejected or cancelled row for the same merchant/group. A later reject/cancel can fail on a terminal-history uniqueness collision.
- Fixed 2026-06-09: `isDuplicateKeyError` now uses typed PostgreSQL error-code classification through `db.ErrorCode(err) == db.UniqueViolation`; short ordinary errors no longer panic and non-unique driver errors are not misclassified by message text.
- Mini Program join page has no local submitting guard and no status/list rehydration entry. The database blocks duplicate pending rows, but weak-network retry and post-submit recovery are not modeled in the page.
- Group application review is registered both as `/v1/groups/applications/:id/review` and `/v1/admin/groups/applications/:id/review`. Both are admin-protected, but the alias pair should be treated as a contract-drift candidate.

## SQL And Durable State Boundaries

- `merchants.bind_code`, `merchants.bind_code_expires_at`: reusable staff invite bearer credential.
- `merchant_staff`: merchant-specific staff membership, role, and active/disabled truth.
- `user_roles`: coarse `merchant_staff` platform capability used by broader authorization surfaces.
- `merchant_group_applications`: group onboarding draft and review state; OCR document bindings/results share `application_data`.
- `ocr_jobs`: async OCR job truth and requested-by/owner binding.
- `merchant_groups`: approved group truth.
- `merchant_group_members`: group user membership and `owner/admin/finance/ops` authorization truth.
- `merchant_group_join_requests`: merchant-to-group affiliation request state.
- `merchants.group_id`, `merchants.brand_id`: final merchant group/brand affiliation truth.
- `merchant_group_audit_logs`: group application/join transition audit trail.

## Trust, Authorization, And Tenant Checks

- Staff list/invite use `MerchantStaffMiddleware("owner", "manager")`; role update/remove use owner-only middleware and then verify target staff belongs to the current merchant.
- Bind route derives merchant from server-side invite-code lookup, not client merchant id. It accepts any authenticated user and creates a pending membership.
- Group application draft/update/delete/submit handlers derive applicant from the authenticated user.
- OCR create validates supported group document type, applicant ownership, media ownership/category binding, and async job ownership.
- Group management uses `requireGroupRole` against active `merchant_group_members`.
- Merchant join-request creation derives current merchant from owner middleware. Approval validates optional brand belongs to group and transactionally protects one-group affiliation.
- Reject validates request belongs to group. Cancel validates request applicant matches current user.

## Idempotency And Duplicate-Submit Checks

- Generating an invite code is replay-friendly: an existing unexpired code is reused.
- Binding the same active or disabled membership returns conflict. Disabled membership is not reactivated.
- Role update and soft remove are convergent at the row level but not atomic with coarse `user_roles` propagation.
- Group get-or-create is not protected by a unique active-draft constraint.
- Group application approval and join-request approval use conditional transactional transitions.
- Join-request create relies on a database uniqueness constraint for pending dedupe, but that same constraint causes terminal-history collisions.
- Reject conditionally changes only pending rows. Cancel performs an unconditional status update after a non-locking read and can overwrite a concurrent terminal transition.

## Recovery And Async Convergence Paths

- Staff membership/user-role propagation has no scheduler, reconciliation worker, or retry path after a partial write.
- Group OCR uses `ocr_jobs`, asynq workers, polling from the Mini Program wrapper, and stale-binding guards before and after provider execution.
- Group application review and group-join review are synchronous admin actions.
- Merchant join page does not expose request history, cancel, or status refresh despite backend cancel/list capabilities.

## Frontend Draft And Backend Rehydration

- Staff page uses backend list truth, applies a local optimistic patch after role/remove success, then reloads.
- Employee bind page uses backend bind response. Pending employees are redirected away from merchant dashboard.
- Group application page rehydrates uploaded asset ids, OCR status, recognized fields, and application status from backend draft responses. Private preview URLs are resolved separately.
- Group join page submits a request and redirects to config after a success modal. It does not retain durable request id or reload status.

## Test Coverage Signals

Observed tests:

- `locallife/api/staff_test.go` proves invite bind creates pending staff without granting the coarse merchant-staff role, and covers role activation/disable helpers.
- `locallife/api/rbac_middleware_test.go` covers merchant staff middleware role selection.
- `locallife/api/group_test.go` covers group draft create/update/submit/review, document delete, join create, approve conflict, and cancel API paths.
- `locallife/db/sqlc/tx_group_test.go` proves group application terminal review conflict and prevents group-join approval from overwriting an existing merchant affiliation.
- `locallife/api/ocr_test.go` covers group OCR create ownership and enqueue paths.
- `locallife/worker/task_group_application_ocr_test.go` covers group OCR execution and stale asset/status/malformed-data guards.

Missing high-value tests:

- Disabled staff rejoin/reactivation contract.
- Atomic rollback or reconciliation when `merchant_staff` succeeds but `user_roles` propagation fails.
- Pending-role access test for every role-agnostic `CheckUserHasMerchantAccess` consumer.
- Invite-code revoke/rotation and disabled-merchant bind behavior.
- Group OCR concurrent sibling-document writeback test.
- Direct group `license_image_asset_id` ownership/category validation test.
- Group submit completeness contract for documents/OCR/address/region.
- Concurrent group draft get-or-create test.
- Join-request create/reject/cancel audit-log failure behavior.
- Cancel-versus-approve race and second reject/cancel history test.
- Mini Program duplicate-tap and re-entry recovery tests for group join.

## Gaps And Refactor Notes

- Replace split staff membership/coarse-role updates with a transaction or an explicit reconciliation capability.
- Define a reactivation path for disabled staff and decide whether invite bind should reactivate or require owner confirmation.
- Align pending semantics: use status, role, or a stricter role-aware access query consistently.
- Decide whether staff invite codes should be reusable, revocable, rotated on demand, or one-time.
- Make group OCR writeback concurrency-safe at the durable JSON boundary.
- Apply media ownership/category checks to any direct group document-asset binding path.
- Move group submission completeness rules into the backend contract.
- Add an active-draft uniqueness strategy for group onboarding.
- Make join create/reject/cancel state and audit writes atomic; use conditional cancel SQL.
- Replace the all-status join-request uniqueness rule with a pending-only uniqueness constraint if historical retries are expected.
- Fixed 2026-06-09: duplicate-error string slicing was replaced with typed database error classification and focused regression coverage.

## Branch Exhaustion

- Entry branches checked: Mini Program staff list/invite/role/remove flows, staff invite binding, pending staff redirect, group application draft/update/document/OCR/submit, group join request page, group management/review backend paths, and staff/group dashboard/config entries. Flutter App has no staff/group management entry in `merchant_app/lib/features/**`. Web/admin UI is out of current scope except backend review effects.
- Request branches checked: staff list/invite/bind/update-role/remove, group application get-or-create/update/delete/submit/review aliases, group OCR create/poll, group document binding/delete, group member/role routes, merchant group join create/cancel/reject/approve/list, and group audit-log writes.
- Backend state branches checked: reusable staff invite code, pending/active/disabled `merchant_staff`, coarse `user_roles`, staff role update/removal, group application draft/submitted/approved/rejected states, group OCR JSON writeback, group creation, group membership roles, join request pending/approved/rejected/cancelled states, merchant `group_id/brand_id` affiliation, and audit logs.
- Async branches checked: group OCR asynq worker and polling; staff propagation has no repair scheduler; group application review and join review are synchronous admin actions; group join Mini Program has no durable re-entry/status refresh despite backend list/cancel capability.
- Failure/retry branches checked: invite-code reuse, disabled staff bind conflict, partial staff/user-role propagation failure, group draft duplicate get-or-create, OCR stale asset guard, group submit without backend completeness enforcement, join duplicate pending constraint, all-status unique collision after terminal history, cancel-after-approve race, non-atomic audit-log writes, and typed duplicate-key classification.
- Reader/consumer branches checked: merchant staff page, pending staff access checks, merchant dashboard/config access, group application page, group join page, group membership authorization, merchant affiliation readers, OCR result readers, and audit log consumers.
- Authorization/tenant branches checked: owner/manager staff list/invite, owner-only role/remove, bind by server-side invite-code lookup, group applicant user ownership, OCR owner/media/category checks, group role authorization, merchant owner-only join create/cancel, review request/group ownership, optional brand-in-group validation, and transaction-protected one-group affiliation on approval.
- Zombie/unreachable branches checked: reusable invite code has no revoke/rotation UI; disabled staff reactivation path is undefined; pending staff semantics can drift through role-agnostic access helpers; group review has two admin route aliases; group join backend cancel/list paths are not represented as durable Mini Program status recovery.
- Test-proof gaps checked: existing tests cover invite pending semantics, typed duplicate-key classification, RBAC role selection, group draft/review/join basics, group OCR ownership/worker, and terminal transaction conflicts. Missing proof remains for disabled rejoin contract, atomic staff/coarse-role propagation, every pending-role consumer, invite revocation/disabled-merchant bind, concurrent group OCR JSON writes, direct document ownership validation, submit completeness, active draft uniqueness, join audit failure, and cancel/approve race.
