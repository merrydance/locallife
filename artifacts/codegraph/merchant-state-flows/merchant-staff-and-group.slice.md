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
- Product decision 2026-06-10: a disabled/removed staff member may re-apply through a valid invite code, but only into `pending`; the owner must confirm the role again before operational access is restored.
- Product decision 2026-06-10: staff invite codes must support revoke/rotate, and old codes become invalid immediately after rotation/revocation.
- Removing or reactivating a staff member must converge both `merchant_staff` membership truth and the coarse `user_roles.merchant_staff` capability without partial writes.
- Group application documents and OCR results must remain bound to the applicant's current draft and current media assets.
- Fixed 2026-06-10: group application submit is backend-gated on the minimum required documents: business license, legal representative ID card front/back, and trademark registration certificate.
- Group onboarding can have at most one editable `draft` application per applicant.
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

6. Bind route resolves the merchant by bind code, verifies expiry, rejects an existing membership row, and calls `AddMerchantStaffTx` to insert `merchant_staff(role='pending', status='active')`. It deliberately does not grant the global `merchant_staff` user role yet.
   Evidence: `locallife/api/server.go:722`, `locallife/api/staff.go:395`, `locallife/api/staff.go:408`, `locallife/db/sqlc/tx_merchant_staff.go:45`, `locallife/db/sqlc/tx_merchant_staff.go:60`.

7. Owner role assignment uses `AssignMerchantStaffRoleTx`: it locks the staff row, verifies merchant ownership and non-owner mutation, changes `merchant_staff.role`, keeps membership active, and creates/reactivates `user_roles(role='merchant_staff')` in the same transaction.
   Evidence: `locallife/api/staff.go:177`, `locallife/api/staff.go:199`, `locallife/db/sqlc/tx_merchant_staff.go:73`, `locallife/db/sqlc/tx_merchant_staff.go:77`, `locallife/db/sqlc/tx_merchant_staff.go:88`, `locallife/db/sqlc/tx_merchant_staff.go:96`, `locallife/db/query/user_role.sql:11`.

8. Owner removal uses `RemoveMerchantStaffTx`: it locks the staff row, verifies merchant ownership and non-owner mutation, soft-deletes the membership, then disables the coarse user role only when no assigned active merchant role remains.
   Evidence: `locallife/api/staff.go:246`, `locallife/api/staff.go:262`, `locallife/db/sqlc/tx_merchant_staff.go:111`, `locallife/db/sqlc/tx_merchant_staff.go:115`, `locallife/db/sqlc/tx_merchant_staff.go:126`, `locallife/db/sqlc/tx_merchant_staff.go:151`, `locallife/db/query/merchant_staff.sql:53`.

9. Group application pages load or create the current user's latest draft, upload required private documents (business license, legal representative ID front/back, and trademark certificate), start OCR jobs where applicable, poll refreshed draft truth, require persisted backend asset ids before advancing, and submit with agreement consent.
   Evidence: `weapp/miniprogram/pages/merchant/group/application/index.ts:362`, `weapp/miniprogram/pages/merchant/group/application/index.ts:416`, `weapp/miniprogram/pages/merchant/group/application/index.ts:432`, `weapp/miniprogram/pages/merchant/group/application/index.ts:574`, `weapp/miniprogram/pages/merchant/group/application/index.ts:624`, `weapp/miniprogram/pages/register/merchant/group/index.ts:386`, `weapp/miniprogram/pages/register/merchant/group/index.ts:402`, `weapp/miniprogram/pages/register/merchant/group/index.ts:544`, `weapp/miniprogram/pages/register/merchant/group/index.ts:595`.

10. Group application wrappers call `GET /v1/groups/applications/me`, `PUT /basic`, `DELETE /documents/:type`, and `POST /submit`; OCR upload binds `owner_type='group_application'`, business-license upload uses the group-license category, and trademark upload writes `trademark_certificate_asset_id`.
    Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:133`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:143`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:151`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:163`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:188`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:201`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:228`.

11. Group application handlers scope drafts to the authenticated applicant. Update and delete reset a rejected application to draft; submitted/approved applications are not editable.
    Evidence: `locallife/api/server.go:742`, `locallife/api/group.go:336`, `locallife/api/group.go:379`, `locallife/api/group.go:397`, `locallife/api/group.go:438`, `locallife/api/group.go:458`.

12. Generic OCR creation recognizes `group_application`, checks owner identity and media-category constraints, records the current OCR job binding in `merchant_group_applications.application_data`, then enqueues the group OCR worker.
    Evidence: `locallife/api/ocr.go:197`, `locallife/api/ocr.go:240`, `locallife/api/ocr.go:293`, `locallife/api/ocr.go:461`, `locallife/api/ocr_media_authz.go:62`.

13. Group OCR workers guard job payload, draft status, media asset, and current OCR job id before provider execution and before writeback.
    Evidence: `locallife/worker/ocr_writeback_guard.go:240`, `locallife/worker/ocr_writeback_guard.go:256`, `locallife/worker/ocr_writeback_guard.go:269`, `locallife/worker/ocr_writeback_guard.go:273`, `locallife/worker/task_group_application_ocr.go:69`, `locallife/worker/task_group_application_ocr.go:157`.

14. Group application submit requires draft status, group name, contact phone, and the backend minimum document asset ids before it writes consent audit or updates status to `submitted`.
    Evidence: `locallife/api/group.go:518`, `locallife/api/group.go:523`, `locallife/api/group.go:532`, `locallife/api/group.go:665`, `locallife/api/group.go:673`, `locallife/api/group.go:682`, `locallife/api/group.go:684`.

15. Admin approval uses a transaction: lock submitted application, conditionally mark approved, create `merchant_groups`, create owner `merchant_group_members`, and insert a group audit log.
    Evidence: `locallife/api/group.go:643`, `locallife/db/sqlc/tx_group.go:22`, `locallife/db/sqlc/tx_group.go:26`, `locallife/db/sqlc/tx_group.go:35`, `locallife/db/sqlc/tx_group.go:49`, `locallife/db/sqlc/tx_group.go:63`, `locallife/db/sqlc/tx_group.go:77`.

16. Merchant group-join page reads merchant-scoped applicant history, searches active groups, blocks duplicate pending submissions, posts a join request with an optional reason, and re-reads durable history after success or stable-code `40980` duplicate-pending conflict. The old register join route redirects to this single merchant-owner page owner, so there is no second stale search/submit implementation.
    Evidence: `weapp/miniprogram/pages/merchant/group/join/index.ts:229`, `weapp/miniprogram/pages/merchant/group/join/index.ts:266`, `weapp/miniprogram/pages/merchant/group/join/index.ts:292`, `weapp/miniprogram/pages/merchant/group/join/index.ts:315`, `weapp/miniprogram/pages/merchant/group/join/index.ts:347`, `weapp/miniprogram/pages/merchant/group/join/index.ts:378`, `weapp/miniprogram/pages/merchant/group/join/index.ts:381`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:222`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:233`, `weapp/miniprogram/pages/merchant/_main_shared/api/group-application.ts:241`, `weapp/miniprogram/pages/register/merchant/index.ts:50`, `weapp/miniprogram/pages/register/merchant/join-group/index.ts:1`.

17. Join-request create route is merchant-owner-only, rejects merchants already attached to a group with stable joined-merchant conflict semantics, requires an active target group, then uses `CreateGroupJoinRequestTx` to lock merchant affiliation, insert the pending request, and write the audit log atomically.
    Evidence: `locallife/api/server.go:793`, `locallife/api/group.go:1212`, `locallife/api/group.go:1225`, `locallife/api/group.go:1235`, `locallife/api/group.go:1256`, `locallife/db/sqlc/tx_group.go:119`, `locallife/db/sqlc/tx_group.go:123`, `locallife/db/sqlc/tx_group.go:131`, `locallife/db/sqlc/tx_group.go:146`.

18. Join-request approval requires group owner/admin and uses a transaction: lock request, require pending, lock merchant affiliation, require unassigned, conditionally approve request, attach merchant only if still unassigned, and write audit log.
    Evidence: `locallife/api/group.go:1336`, `locallife/api/group.go:1378`, `locallife/db/sqlc/tx_group.go:165`, `locallife/db/sqlc/tx_group.go:169`, `locallife/db/sqlc/tx_group.go:180`, `locallife/db/sqlc/tx_group.go:189`, `locallife/db/sqlc/tx_group.go:201`, `locallife/db/sqlc/tx_group.go:217`.

19. Join-request reject and cancel now use transaction helpers. Reject locks the request, requires group match and pending status, conditionally marks rejected, and writes audit. Cancel locks the request, requires group/applicant match and pending status, conditionally marks cancelled through `CancelPendingGroupJoinRequest`, and writes audit. Terminal-state conflicts now return 409 instead of regressing status.
    Evidence: `locallife/api/group.go:1441`, `locallife/api/group.go:1464`, `locallife/api/group.go:1479`, `locallife/api/group.go:1529`, `locallife/api/group.go:1547`, `locallife/db/sqlc/tx_group.go:247`, `locallife/db/sqlc/tx_group.go:251`, `locallife/db/sqlc/tx_group.go:263`, `locallife/db/sqlc/tx_group.go:279`, `locallife/db/sqlc/tx_group.go:308`, `locallife/db/sqlc/tx_group.go:312`, `locallife/db/sqlc/tx_group.go:327`, `locallife/db/sqlc/tx_group.go:342`, `locallife/db/query/group.sql:210`.

## Reverse-Reference Findings

- Fixed 2026-06-09: `merchant_staff` is still the merchant-specific authorization truth and `user_roles(role='merchant_staff')` remains a coarse global capability, but direct add, invite bind, role assignment, and remove now mutate both through transaction helpers. Assigned roles create/reactivate the coarse role via `UpsertUserRoleActive`; pending staff does not grant it; removal disables it only when no assigned active staff remains.
- Product decision 2026-06-10: disabled/removed staff rejoin is allowed only by re-applying with a valid invite into pending status, followed by owner role confirmation. Current runtime still treats any existing staff row as a bind/direct-add conflict, so implementation must align invite binding, direct add, and owner confirmation with this contract.
- Product decision 2026-06-10: invite codes must be revocable/rotatable, and any old code must become invalid immediately. Current runtime still reuses an unexpired `merchants.bind_code` for up to 24 hours and has no traced revoke/rotate UI/API.
- Fixed 2026-06-09: bind still creates `role='pending', status='active'` for pending workbench display, but role-agnostic access is now role-aware. `CheckUserHasMerchantAccess`, `CountMerchantStaff`, `GetMerchantByOwner`, logic `resolveMerchantForUser`, and default accessible-merchant resolution exclude pending staff; `MerchantStaffMiddleware` resolves pending associations only for explicit denial/audit and defaults to granted staff merchants when granted and pending associations coexist.
- Migration `000070_add_staff_pending_status` introduced a `pending` status, but runtime bind uses a pending role plus active status. `UpdateMerchantStaffStatus` and hard-delete queries remain reverse-reference drift candidates; count/access helpers are now role-aware.
- Fixed 2026-06-09: group OCR writeback is concurrency-safe at the shared `application_data` JSON boundary. `UpdateGroupApplicationLicense` now JSONB-merges patches, and group OCR API/worker callers pass only current document keys instead of stale full blobs; DB/API/worker tests prove sibling document state is preserved and callers remain patch-only.
- Fixed 2026-06-09: group application basic update now validates a submitted `license_image_asset_id` before draft mutation. The asset must belong to the applicant, be `confirmed`, and use one of the group business-license categories accepted by OCR (`business_license` or `group_license`); missing, cross-user, wrong-category, unconfirmed, and infrastructure-error branches are covered.
- Fixed 2026-06-10: group submit enforces the backend minimum required document set before consent audit and status transition: business license, legal representative ID card front/back, and trademark registration certificate. The basic-update path validates the trademark certificate asset, document deletion supports `trademark_certificate`, and both traced Mini Program group-application entries require persisted backend asset ids before advancing.
- Fixed 2026-06-09: group application active-draft uniqueness is enforced at the database boundary. Migration `000256` cleans historical non-latest draft rows to `rejected`, adds a partial unique index on `(applicant_user_id) WHERE status='draft'`, `CreateGroupApplicationDraft` is idempotent through upsert, and `ResetGroupApplicationToDraft` returns an existing draft instead of creating a second one.
- Fixed 2026-06-09: join-request create/reject/cancel now use transaction helpers for durable state and audit-log writes. Create also locks merchant affiliation before inserting so a concurrently joined merchant is rejected inside the transaction, and both precheck and transaction joined-merchant branches return the stable API error.
- Fixed 2026-06-09: join-request cancel no longer calls a broad status update. `CancelPendingGroupJoinRequest` updates only `status='pending'`, and stale approved/rejected/cancelled requests return `ErrGroupJoinRequestReviewConflict` / HTTP 409 without overwriting terminal state.
- Fixed 2026-06-09: group join request uniqueness is pending-only at the database boundary. Migration `000257` drops the old all-status `(group_id, merchant_id, status)` unique constraint, cleans duplicate historical pending rows by keeping the newest pending request and marking older pending rows cancelled, and adds a partial unique index on `(group_id, merchant_id) WHERE status='pending'`. Repeated rejected/cancelled history rows are now allowed while duplicate pending writes remain rejected.
- Fixed 2026-06-09: `isDuplicateKeyError` now uses typed PostgreSQL error-code classification through `db.ErrorCode(err) == db.UniqueViolation`; short ordinary errors no longer panic and non-unique driver errors are not misclassified by message text.
- Fixed 2026-06-09: Mini Program group-join recovery is now modeled in the merchant page. The page hydrates applicant request history from `GET /v1/merchants/me/group-join-requests`, shows the latest pending request on entry/re-entry, disables already-pending search results, keeps page/row submitting state during POST, treats backend duplicate-pending conflict as recovery only when the API error code is the stable `40980`, and re-reads durable history after success or conflict. This change is applicant-side only and does not change group owner/admin approval transaction semantics.
- Group application review is registered both as `/v1/groups/applications/:id/review` and `/v1/admin/groups/applications/:id/review`. Both are admin-protected, but the alias pair should be treated as a contract-drift candidate.

## SQL And Durable State Boundaries

- `merchants.bind_code`, `merchants.bind_code_expires_at`: current staff invite bearer credential. Product decision 2026-06-10 requires revoke/rotate semantics where old codes become invalid immediately.
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

- Current invite generation is replay-friendly because an existing unexpired code is reused; product decision 2026-06-10 requires explicit revoke/rotate semantics instead of relying only on expiry.
- Binding the same active or disabled membership currently returns conflict. Product decision 2026-06-10 changes the disabled/removed rejoin path to create a pending re-application that still requires owner role confirmation.
- Role update and soft remove now use transaction helpers, so the membership row and coarse `user_roles` propagation commit or roll back together.
- Group get-or-create is protected by a partial unique active-draft constraint and idempotent create/reset SQL.
- Group application approval and all join-request create/approve/reject/cancel transitions use transaction helpers for their state/audit boundaries.
- Join-request create relies on the pending-only database unique index for active pending dedupe, while terminal rejected/cancelled history can accumulate for audit/debugging.
- `ListGroupJoinRequestsByMerchant` reads applicant history scoped by merchant id and joins `merchant_groups.name` for display; it is exposed only through merchant-owner middleware.
- Reject and cancel conditionally change only pending rows; stale terminal states are conflicts.

## Recovery And Async Convergence Paths

- Staff membership/user-role propagation is transaction-owned for add, invite bind, role assignment, and removal. There is no separate scheduler or reconciliation worker; current recovery relies on the transaction boundary.
- Group OCR uses `ocr_jobs`, asynq workers, polling from the Mini Program wrapper, and stale-binding guards before and after provider execution.
- Group application review and group-join review are synchronous admin actions.
- Merchant join page exposes applicant-side request history and status refresh for duplicate-submit/re-entry recovery. It still does not expose cancellation controls; group owner/admin review remains the synchronous backend approval path.

## Frontend Draft And Backend Rehydration

- Staff page uses backend list truth, applies a local optimistic patch after role/remove success, then reloads.
- Employee bind page uses backend bind response. Pending employees are redirected away from merchant dashboard.
- Group application page rehydrates uploaded asset ids, OCR status, recognized fields, and application status from backend draft responses. Private preview URLs are resolved separately.
- Group join page rehydrates join-request history on load/show, derives the latest pending request from backend truth, reconciles current search results against pending state, and reloads durable history after create success or stable-code `40980` duplicate-pending conflict before showing the submitted modal/redirecting.

## Test Coverage Signals

Observed tests:

- `locallife/api/staff_test.go` proves invite bind creates pending staff through `AddMerchantStaffTx` without granting the coarse merchant-staff role, and proves add/update/delete handlers use the atomic staff-role tx entrypoints instead of split writes.
- `locallife/db/sqlc/merchant_staff_test.go` proves pending staff is excluded from role-agnostic access/counts, assigned staff grants/reactivates coarse `user_roles.merchant_staff`, pending staff does not grant it, removal disables it only when no assigned active staff remains, pending-only remaining staff disables it, and owner staff cannot be mutated through staff tx helpers.
- `locallife/db/sqlc/merchant_test.go` and `locallife/logic/service_support_test.go` prove `GetMerchantByOwner` excludes pending staff and logic `resolveMerchantForUser` does not fall back to a pending-only merchant association.
- `locallife/api/rbac_middleware_test.go`, `locallife/api/merchant_access_test.go`, and `locallife/api/device_access_test.go` cover pending role denial, pending exclusion from default accessible merchants, mixed granted+pending default selection, and device-access denial with visible `staff_role=pending`.
- `locallife/api/kitchen_test.go` and `locallife/api/inventory_test.go` prove representative real high-level merchant routes (`GET /v1/kitchen/orders` and `POST /v1/inventory`) deny `role='pending'` staff at the registered `MerchantStaffMiddleware` boundary before kitchen reads or inventory writes can run.
- `locallife/api/group_test.go` covers group draft create/update/submit/review, submit rejection for missing required documents before consent/state mutation, trademark asset validation, trademark document delete, join create via transaction, joined-merchant create precheck/transaction conflict mapping, approve conflict, reject via transaction, cancel via transaction, and cancel conflict API paths.
- `locallife/api/group_test.go` also covers the merchant applicant-history API returning group display names without review-conflict leakage.
- `locallife/db/sqlc/tx_group_test.go` proves group application terminal review conflict, create request transaction writes audit and rejects already-joined merchants, duplicate pending join requests are rejected by the database, approval prevents affiliation overwrite, applicant-history listing is scoped to the merchant and returns group names in newest-first order, reject writes audit and rejects non-pending replay, repeated rejected history can be stored, cancel does not overwrite approved requests, and repeated cancelled history can be stored.
- `locallife/api/ocr_test.go` covers group OCR create ownership/enqueue paths and patch-only pending/failure writebacks.
- `locallife/db/sqlc/group_test.go` proves `UpdateGroupApplicationLicense` merges sibling `application_data` patches instead of replacing the full JSON blob, `UpdateGroupApplicationBasic` merges/removes the trademark certificate asset key without dropping sibling data, `CreateGroupApplicationDraft` is idempotent for an existing active draft, and resetting an older rejected application returns the existing draft without creating a second draft.
- `locallife/worker/task_group_application_ocr_test.go` covers group OCR execution, stale asset/status/malformed-data guards, and patch-only success/failure writebacks.
- `weapp/scripts/check-merchant-group-join-recovery.test.js` proves the Mini Program page uses applicant history, models pending/error/submitting state, refreshes durable history on load/show and after submit/stable-code `40980` duplicate conflict, disables already-pending or in-flight actions, and redirects the old register join page to the single merchant group-join owner.
- Mini Program TypeScript/compile/lint validation proves both merchant and register group-application entries consume the new trademark field/upload/delete contract without WXML or type drift.

Missing high-value tests:

- Disabled staff rejoin/reactivation contract per the 2026-06-10 decision: valid invite -> pending re-application -> owner role confirmation.
- Invite-code revoke/rotation and disabled-merchant bind behavior, including immediate invalidation of old codes.

## Gaps And Refactor Notes

- Product decision 2026-06-10: disabled/removed staff may re-apply through a valid invite into pending, and owner role confirmation is required before access. Implement this across invite binding, direct add, owner confirmation, and tests.
- Fixed 2026-06-09: staff membership/coarse-role updates now use transaction helpers for add, invite bind, role assignment, and removal.
- Fixed 2026-06-09: pending staff semantics are role-aware in backend access helpers, `GetMerchantByOwner`, and logic merchant fallback while preserving `role='pending', status='active'` as the pending workbench model.
- Fixed 2026-06-09: representative high-level pending-staff consumers now have real route proof. `GET /v1/kitchen/orders` and `POST /v1/inventory` return 403 for pending staff through the registered middleware, with gomock expectations ensuring downstream kitchen reads and inventory writes are not reached. A full route matrix can be expanded when new merchant-staff surfaces are added, but the prior high-value proof gap is closed for traced representative consumers.
- Product decision 2026-06-10: staff invite codes must support revoke/rotate on demand; old codes are invalid immediately after rotation/revocation.
- Fixed 2026-06-09: group OCR writeback is concurrency-safe at the durable JSON boundary.
- Fixed 2026-06-09: apply media ownership/category/upload-status checks to direct group `license_image_asset_id` draft updates.
- Fixed 2026-06-10: group submission completeness moved into backend validation for business license, legal representative ID card front/back, and trademark registration certificate; both traced Mini Program entries now upload the required trademark certificate and require persisted backend asset ids before advancing.
- Fixed 2026-06-09: add active-draft uniqueness for group onboarding through migration `000256`, idempotent draft creation, deterministic latest-application ordering, and reset-to-existing-draft behavior.
- Fixed 2026-06-09: join create/reject/cancel state and audit writes are atomic, and cancel uses pending-only SQL.
- Fixed 2026-06-09: replace the all-status join-request uniqueness rule with migration `000257` pending-only uniqueness; duplicate pending rows remain blocked and repeated terminal rejected/cancelled history is preserved.
- Fixed 2026-06-09: duplicate-error string slicing was replaced with typed database error classification and focused regression coverage.
- Fixed 2026-06-09: Mini Program group-join duplicate-submit/re-entry recovery is applicant-history driven through `GET /v1/merchants/me/group-join-requests` and stable duplicate-pending API error code `40980`; the legacy register join page redirects to the same merchant-owner page owner.

## Branch Exhaustion

- Entry branches checked: Mini Program staff list/invite/role/remove flows, staff invite binding, pending staff redirect, group application draft/update/document/OCR/submit, group join request page, group management/review backend paths, and staff/group dashboard/config entries. Flutter App has no staff/group management entry in `merchant_app/lib/features/**`. Web/admin UI is out of current scope except backend review effects.
- Request branches checked: staff list/invite/bind/update-role/remove, group application get-or-create/update/delete/submit/review aliases, group OCR create/poll, group document binding/delete, group member/role routes, merchant group join create/cancel/reject/approve/list, and group audit-log writes.
- Backend state branches checked: reusable staff invite code, pending/active/disabled `merchant_staff`, coarse `user_roles`, staff role update/removal, group application draft/submitted/approved/rejected states, group OCR JSON writeback, group creation, group membership roles, join request pending/approved/rejected/cancelled states, merchant `group_id/brand_id` affiliation, and audit logs.
- Async branches checked: group OCR asynq worker and polling; staff propagation is transaction-owned with no separate repair scheduler; group application review and join review are synchronous admin actions; group join Mini Program durable re-entry/status refresh now reads applicant history.
- Failure/retry branches checked: invite-code reuse, disabled staff bind conflict, fixed transaction-owned staff/user-role propagation, fixed group draft duplicate get-or-create, OCR stale asset guard, fixed group submit backend completeness enforcement, pending-only join duplicate constraint, repeated rejected/cancelled terminal history, fixed cancel-after-approve conflict handling, fixed join create/reject/cancel transaction-owned audit writes, typed duplicate-key classification, and Mini Program stable-code `40980` duplicate-pending recovery.
- Reader/consumer branches checked: merchant staff page, pending staff access checks, representative kitchen/inventory high-level route denial, merchant dashboard/config access, group application page, group join page/history recovery, group membership authorization, merchant affiliation readers, OCR result readers, and audit log consumers.
- Authorization/tenant branches checked: owner/manager staff list/invite, owner-only role/remove, bind by server-side invite-code lookup, group applicant user ownership, OCR owner/media/category checks, group role authorization, merchant owner-only join create/cancel, review request/group ownership, optional brand-in-group validation, and transaction-protected one-group affiliation on approval.
- Zombie/unreachable branches checked: reusable invite code currently has no revoke/rotation UI despite the 2026-06-10 revoke/rotate decision; disabled staff reactivation currently conflicts instead of pending re-applying; group review has two admin route aliases; group join backend cancel path is still not represented as a Mini Program applicant cancellation control.
- Test-proof gaps checked: existing tests cover invite pending semantics, pending role-aware access helpers, `GetMerchantByOwner`/logic fallback filtering, atomic staff/coarse-role propagation, typed duplicate-key classification, RBAC role selection, representative real kitchen/inventory route denial for pending staff, group draft/review/join basics, group active-draft uniqueness/reset idempotency, direct group license media validation, group OCR ownership/worker, group OCR JSON patch merging, group submit minimum-document completeness, join create/reject/cancel transaction paths, stable joined-merchant create conflict mapping, pending-only join uniqueness, repeated terminal join history, applicant-history readback, Mini Program join recovery, and terminal transaction conflicts. Missing proof remains for the 2026-06-10 disabled rejoin contract and invite revoke/rotate invalidation.
