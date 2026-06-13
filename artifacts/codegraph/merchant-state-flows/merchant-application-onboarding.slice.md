# Merchant Application Onboarding Slice

Status: merchant-state flow slice created; owner-only approval lookup fixed 2026-06-07; read-only `/v1/merchants/applications/me` compatibility route verified 2026-06-11; mutating `GET /v1/merchant/application` submitted-reset race fixed 2026-06-11; explicit edit/reset queued-review supersede fixed 2026-06-11; stale pending OCR cleanup fixed 2026-06-11; submit retry active-review-run idempotency fixed 2026-06-11; approved re-edit reverification contract fixed 2026-06-11; post-approval review/credential repair fixed 2026-06-11; GPS duplicate-location hard reject narrowed 2026-06-13
Risk class: G3 - private identity documents, OCR async writeback, merchant creation/update, owner authorization, credential ledger activation, and review/recovery state
Scope: Mini Program merchant application page -> media/OCR job creation -> merchant application draft writes -> submit validation -> async/sync onboarding review -> merchant/profile/staff/user-role/credential durable truth

## Variant Coverage

This slice covers:

- Merchant Mini Program subject application page for draft edit, location save, document upload/delete, reset, and submit.
- Merchant application APIs under `/v1/merchant/application/**`.
- Generic OCR job API for `owner_type='merchant_application'`, merchant OCR workers, and stale OCR cleanup.
- Onboarding review run creation, queue fallback, worker processing, approval transaction, review summary, and credential ledger activation.
- Approved-application edit/reset behavior because it is part of the merchant-visible re-entry path.

This slice does not fully cover:

- Operator/rider onboarding variants except where they share review/OCR infrastructure.
- Baofu settlement-account onboarding after application approval; that is covered by `merchant-finance-withdrawal`.
- General media upload-session internals beyond the application OCR media binding boundary.
- Manual/operator application review UIs if later added; current merchant path is auto-review plus blocked-review summary.

## Product Invariant

Merchant onboarding must preserve a clear split between editable application draft truth and activated merchant truth:

- Draft edits and document OCR may only mutate the authenticated user's current editable `merchant_applications` row.
- OCR writeback must only apply to the current media asset and current OCR job while the application is still draft.
- Submit must validate required fields, OCR readiness, document validity, address/region/location, duplicate location, duplicate license, and duplicate legal person before creating or updating a merchant. Duplicate-location hard reject is now reserved for same-door/same-point GPS collisions within 5 meters; nearby but distinct merchants beyond that boundary are allowed to continue through the remaining review checks.
- Approval must atomically create/update merchant profile truth, ensure owner staff membership, ensure owner user role, and record review/credential evidence.
- Re-editing an approved application must be an intentional resubmission path, not a silent mutation of already-active merchant truth.

## Primary Forward Chain

1. Merchant application page gates entry with merchant console access, loads the current application draft, blocks pull refresh when dirty, and rehydrates local form state from backend truth.
   Evidence: `weapp/miniprogram/pages/merchant/settings/application/index.ts:110`, `weapp/miniprogram/pages/merchant/settings/application/index.ts:143`, `weapp/miniprogram/pages/merchant/settings/application/index.ts:152`, `weapp/miniprogram/pages/merchant/settings/application/index.ts:209`.

2. The page persists basic draft fields through `updateMerchantBasicInfo`, and submit first saves dirty draft state before calling `submitMerchantApplication`.
   Evidence: `weapp/miniprogram/pages/merchant/settings/application/index.ts:320`, `weapp/miniprogram/pages/merchant/settings/application/index.ts:328`, `weapp/miniprogram/pages/merchant/settings/application/index.ts:349`, `weapp/miniprogram/pages/merchant/settings/application/index.ts:365`, `weapp/miniprogram/pages/merchant/settings/application/index.ts:371`.

3. Location selection is saved immediately through the same basic-info endpoint with latitude/longitude; the backend may auto-match a `region_id`.
   Evidence: `weapp/miniprogram/pages/merchant/settings/application/index.ts:553`, `weapp/miniprogram/pages/merchant/settings/application/index.ts:576`, `weapp/miniprogram/pages/merchant/settings/application/index.ts:578`, `locallife/api/merchant_application.go:514`, `locallife/api/merchant_application.go:535`.

4. The frontend enforces submit-time completeness for merchant name, phone, address, business license number, legal person fields, four documents, location, region, and OCR block state.
   Evidence: `weapp/miniprogram/pages/merchant/settings/application/index.ts:296`, `weapp/miniprogram/pages/merchant/_utils/merchant-application-view.ts:201`, `weapp/miniprogram/pages/merchant/_utils/merchant-application-view.ts:218`, `weapp/miniprogram/pages/merchant/_utils/merchant-application-view.ts:222`, `weapp/miniprogram/pages/merchant/_utils/merchant-application-view.ts:230`.

5. Application API wrappers call `GET /v1/merchant/application`, `PUT /basic`, `DELETE /documents/:type`, `POST /submit`, and `POST /reset`.
   Evidence: `weapp/miniprogram/pages/merchant/_main_shared/api/onboarding.ts:604`, `weapp/miniprogram/pages/merchant/_main_shared/api/onboarding.ts:756`, `weapp/miniprogram/pages/merchant/_main_shared/api/onboarding.ts:779`.

6. Backend merchant application routes are authenticated but not staff-gated, because this flow is used before a merchant exists and after approved re-entry.
   Evidence: `locallife/api/server.go:699`, `locallife/api/server.go:702`, `locallife/api/server.go:703`, `locallife/api/server.go:705`, `locallife/api/server.go:706`, `locallife/api/server.go:707`.

7. Fixed 2026-06-11: `getOrCreateMerchantApplicationDraft` returns the latest draft/submitted/rejected/approved application for the user, creates an empty draft if none exists, and does not reset `submitted` applications on GET. Submitted/rejected/approved applications still enter draft through write paths that first confirm edit intent.
   Evidence: `locallife/api/merchant_application.go:379`, `locallife/api/merchant_application.go:381`, `locallife/api/merchant_application.go:392`, `locallife/api/merchant_application.go:395`, `locallife/api/merchant_application.go:413`, `locallife/api/merchant_application_test.go:435`.

8. Basic, image, document delete, and OCR pending writes all call `checkApplicationEditable`; `rejected`, `approved`, and `submitted` are editable after `ResetMerchantApplicationTx` changes the application back to draft. Fixed/current 2026-06-11: that reset transaction now cancels active merchant onboarding review runs as `superseded_by_edit`, maps the summary to `needs_resubmit` for current client compatibility, refreshes `merchant_applications.review_summary`, and resolves only the applicant's owned merchant when preserving or downgrading merchant status.
   Evidence: `locallife/api/merchant_application.go:161`, `locallife/api/merchant_application.go:167`, `locallife/api/merchant_application.go:452`, `locallife/api/merchant_application.go:459`, `locallife/api/merchant_application.go:620`, `locallife/api/ocr.go:310`, `locallife/db/sqlc/tx_merchant_application.go:241`, `locallife/db/query/onboarding_review.sql:115`, `locallife/db/sqlc/tx_merchant_application_test.go:200`.

9. Basic-info SQL only updates rows with `status='draft'`; the reset transaction updates application status, fails closed if the reset result does not belong to the supplied user, ignores staff-associated merchants, keeps an owned active/approved merchant live with current profile/image truth until reapproval, and only changes an owned non-active/non-approved merchant status to pending.
   Evidence: `locallife/db/query/merchant_application.sql:26`, `locallife/db/query/merchant_application.sql:41`, `locallife/db/sqlc/tx_merchant_application.go:225`, `locallife/db/sqlc/tx_merchant_application.go:240`, `locallife/db/sqlc/tx_merchant_application.go:270`, `locallife/db/sqlc/tx_merchant_application_test.go:253`, `locallife/db/sqlc/tx_merchant_application_test.go:318`, `locallife/db/sqlc/tx_merchant_application_test.go:358`.

10. Document upload first creates media, then creates a generic OCR job with owner type `merchant_application`, document type, media asset id, owner id, and optional side.
    Evidence: `weapp/miniprogram/pages/merchant/settings/application/index.ts:446`, `weapp/miniprogram/pages/merchant/_main_shared/api/onboarding.ts:612`, `weapp/miniprogram/pages/merchant/_main_shared/api/onboarding.ts:617`, `weapp/miniprogram/pages/merchant/_main_shared/api/onboarding.ts:620`, `weapp/miniprogram/pages/merchant/_main_shared/api/onboarding.ts:623`, `weapp/miniprogram/pages/merchant/_main_shared/api/onboarding.ts:625`.

11. OCR job creation checks the requester owns the merchant application, enforces expected media category/side, writes a pending OCR payload plus media asset id, and enqueues the corresponding merchant OCR task.
    Evidence: `locallife/api/ocr.go:251`, `locallife/api/ocr.go:255`, `locallife/api/ocr_media_authz.go:57`, `locallife/api/ocr.go:265`, `locallife/api/ocr.go:273`, `locallife/api/ocr.go:304`, `locallife/api/ocr.go:338`, `locallife/api/ocr.go:353`.

12. Merchant OCR workers guard job payload, current draft status, current media asset id, and current OCR job id before provider execution and again before writeback.
    Evidence: `locallife/worker/task_merchant_application_ocr.go:117`, `locallife/worker/ocr_writeback_guard.go:96`, `locallife/worker/ocr_writeback_guard.go:110`, `locallife/worker/ocr_writeback_guard.go:114`, `locallife/worker/ocr_writeback_guard.go:119`, `locallife/worker/ocr_writeback_guard.go:124`, `locallife/worker/task_merchant_application_ocr.go:164`.

13. OCR success writes recognized license, food permit, and ID-card fields back into the application draft; ID card front can fill legal person name/number, and business license can fill license number/scope.
    Evidence: `locallife/worker/task_merchant_application_ocr.go:171`, `locallife/worker/task_merchant_application_ocr.go:188`, `locallife/worker/task_merchant_application_ocr.go:193`, `locallife/worker/task_merchant_application_ocr.go:205`, `locallife/worker/task_merchant_application_ocr.go:295`, `locallife/worker/task_merchant_application_ocr.go:313`, `locallife/worker/task_merchant_application_ocr.go:419`.

14. OCR failure can write a failed OCR payload only if the job is still bound to the current media/document; stale failure writebacks are skipped.
    Evidence: `locallife/api/ocr.go:600`, `locallife/api/ocr.go:601`, `locallife/api/ocr.go:623`, `locallife/api/ocr.go:639`, `locallife/api/ocr.go:655`, `locallife/api/ocr.go:829`, `locallife/api/ocr.go:857`.

15. Fixed/current 2026-06-11: stale OCR cleanup marks application OCR JSON fields from `pending` or `processing` to `failed` if the draft has not been updated for more than one hour. It preserves the existing OCR payload metadata such as `queued_at`, `started_at`, `ocr_job_id`, and `error`, and leaves recent pending fields plus terminal `done`/`failed` fields unchanged.
    Evidence: `locallife/scheduler/data_cleanup.go:1680`, `locallife/scheduler/data_cleanup.go:1688`, `locallife/db/query/merchant_application.sql:223`, `locallife/db/query/merchant_application.sql:226`, `locallife/db/query/merchant_application.sql:231`, `locallife/db/sqlc/merchant_test.go:1187`, `locallife/db/sqlc/merchant_test.go:1247`.

16. Fixed/current 2026-06-11: Submit writes `merchant_application_submit_attempt_consent_confirmed` agreement consent audit after loading/backfilling the current application but before required-field validation. That audit is an intent/submit-attempt record with metadata `event_scope='submit_attempt'`, so a later 400 validation response does not imply successful submission or approval. The handler then validates required backend fields and runs document/address/duplicate checks before setting `status='submitted'`.
    Evidence: `locallife/api/merchant_application.go:877`, `locallife/api/merchant_application.go:908`, `locallife/api/merchant_application.go:914`, `locallife/api/merchant_application.go:917`, `locallife/api/merchant_application.go:923`, `locallife/api/merchant_application.go:938`, `locallife/api/agreement_consent.go:73`, `locallife/api/agreement_consent.go:78`, `locallife/api/merchant_application_test.go:2936`.

17. Backend submit validation requires merchant name, address, latitude/longitude, region, business license, food permit, ID card front, and ID card back.
    Evidence: `locallife/api/merchant_application.go:1006`, `locallife/api/merchant_application.go:1008`, `locallife/api/merchant_application.go:1014`, `locallife/api/merchant_application.go:1017`, `locallife/api/merchant_application.go:1020`, `locallife/api/merchant_application.go:1023`, `locallife/api/merchant_application.go:1026`.

18. Approval checks OCR payload readiness/validity, repairs some OCR payloads from raw/normalized job results, validates address match with reverse/geocode fallback, checks nearby merchant GPS duplicates, duplicate license number, and duplicate legal-person ID.
    Evidence: `locallife/api/merchant_application.go:1050`, `locallife/api/merchant_application.go:1060`, `locallife/api/merchant_application.go:1081`, `locallife/api/merchant_application.go:1123`, `locallife/api/merchant_application.go:1137`, `locallife/api/merchant_application.go:1141`, `locallife/api/merchant_application.go:1180`, `locallife/api/merchant_application.go:1207`, `locallife/api/merchant_application.go:1222`.

19. If approval checks fail before submission is approved, the handler records a blocked onboarding review summary and returns a 400 while the application remains editable.
    Evidence: `locallife/api/merchant_application.go:923`, `locallife/api/merchant_application.go:924`, `locallife/api/onboarding_review.go:37`, `locallife/api/onboarding_review.go:43`, `locallife/api/onboarding_review.go:58`.

20. Fixed/current 2026-06-11: if async review infrastructure is available, first submit creates an onboarding review run, enqueues `onboarding:review` with `asynq.Unique(30s)`, writes queued summary into the response, and returns without immediately approving. A retry while the application is already `submitted` first reuses the latest active merchant review run: queued runs re-enqueue the same run id and treat `ErrDuplicateTask` as already queued, while processing runs return the current review summary without enqueueing or creating a duplicate run.
    Evidence: `locallife/api/merchant_application.go:973`, `locallife/api/merchant_application.go:981`, `locallife/api/merchant_application.go:986`, `locallife/api/merchant_application.go:991`, `locallife/api/merchant_application.go:993`, `locallife/api/merchant_application.go:1002`, `locallife/db/query/onboarding_review.sql:61`, `locallife/api/merchant_application_test.go:3022`, `locallife/api/merchant_application_test.go:3100`, `locallife/api/merchant_application_test.go:3178`.

21. If review run creation or enqueue fails, submit falls back to synchronous `ProcessSubmittedApplication`.
    Evidence: `locallife/api/merchant_application.go:958`, `locallife/api/merchant_application.go:973`, `locallife/api/merchant_application.go:977`.

22. Fixed/current 2026-06-11: the review worker validates run/application identity, skips cancelled runs and completed non-merchant runs, and deliberately lets completed merchant runs re-enter the merchant review service so approved applications can repair missing review summary or credential evidence.
    Evidence: `locallife/worker/task_onboarding_review.go:51`, `locallife/worker/task_onboarding_review.go:55`, `locallife/worker/task_onboarding_review.go:58`, `locallife/worker/task_onboarding_review.go:63`, `locallife/worker/task_onboarding_review.go:67`, `locallife/worker/task_onboarding_review.go:117`, `locallife/worker/task_onboarding_review_test.go:121`, `locallife/worker/task_onboarding_review_test.go:421`.

23. Fixed/current 2026-06-11: merchant onboarding review service still approves only `submitted` applications, but an approved application with an existing review run now enters a repair path. Queued/processing merchant runs are completed and summary-refreshed, completed merchant runs must already have `outcome='approved'` before they can refresh summary, and missing review service or completed non-approved evidence fails closed before credential repair. Other non-submitted statuses still cancel the run as `superseded_by_edit`, record `needs_resubmit`, and refresh the application review summary.
    Evidence: `locallife/logic/merchant_onboarding_review_service.go:106`, `locallife/logic/merchant_onboarding_review_service.go:111`, `locallife/logic/merchant_onboarding_review_service.go:175`, `locallife/logic/merchant_onboarding_review_service.go:181`, `locallife/logic/merchant_onboarding_review_service.go:189`, `locallife/logic/onboarding_review_service.go:103`, `locallife/logic/onboarding_review_service.go:116`, `locallife/db/query/onboarding_review.sql:85`, `locallife/db/query/onboarding_review.sql:94`.

24. Approval transaction changes the application to approved, creates or updates only a merchant owned by the application user, sets merchant closed, copies valid application storefront/environment image arrays into merchant live truth, reconciles system labels, ensures owner `merchant_staff`, and ensures the coarse `merchant_owner` user role.
    Evidence: `locallife/db/sqlc/tx_merchant_application.go:34`, `locallife/db/sqlc/tx_merchant_application.go:45`, `locallife/db/sqlc/tx_merchant_application.go:55`, `locallife/db/sqlc/tx_merchant_application.go:72`, `locallife/db/sqlc/tx_merchant_application.go:97`, `locallife/db/sqlc/tx_merchant_application.go:109`, `locallife/db/sqlc/tx_merchant_application.go:120`, `locallife/db/sqlc/tx_merchant_application.go:159`.

25. Fixed/current 2026-06-11: after approval or approved-application repair, credential governance activates business-license and food-permit ledgers only for missing `(document_type, media_asset_id, merchant_application_id, review_run_id)` entries, can release document-expiry suspension, and sends a restore notification when release occurs.
    Evidence: `locallife/logic/merchant_onboarding_review_service.go:154`, `locallife/logic/merchant_onboarding_review_service.go:161`, `locallife/logic/merchant_onboarding_review_service.go:207`, `locallife/logic/merchant_onboarding_review_service.go:222`, `locallife/logic/merchant_onboarding_review_service.go:246`, `locallife/logic/credential_governance_service.go:63`, `locallife/db/sqlc/tx_credential_ledger.go:54`, `locallife/db/sqlc/tx_credential_ledger.go:135`, `locallife/api/onboarding_review.go:145`.

## Reverse-Reference Findings

- `merchant_applications` is pre-onboarding draft truth and post-approval credential/application-history truth. Approved merchant live storefront/environment images are copied to `merchants.storefront_images/environment_images`; profile-image edits no longer write application rows.
- Fixed 2026-06-11: Approved applications are explicitly editable in both frontend and backend as a reverification draft. Any basic/image/document/OCR edit resets the application to draft, but `ResetMerchantApplicationTx` fails closed on application/user owner mismatch, deliberately leaves the applicant's owned active/approved merchant status and live profile/image truth unchanged until reapproval, ignores staff-associated merchants, and returns the preserved merchant in the transaction result. The Mini Program status summary now tells merchants that the online shop keeps the current data and updates only after the new review passes.
- Fixed 2026-06-11: `getOrCreateMerchantApplicationDraft` no longer auto-resets `submitted` applications on GET, so opening the application page while an async review is queued no longer silently changes the application to draft.
- Fixed 2026-06-11: explicit editing/reset of a `submitted` application now resets the application to draft and cancels active merchant onboarding review runs in the same reset transaction with `run_status='cancelled'`, `outcome='needs_resubmit'`, and `reason_code='superseded_by_edit'`; the application review summary is refreshed so merchant readers no longer see the old queued state. The worker defensive cancel path now records the same `needs_resubmit` outcome and refreshes `merchant_applications.review_summary` when it converges a legacy non-submitted queued run.
- Fixed 2026-06-11: retrying submit for an already `submitted` merchant application no longer creates a new onboarding review run when an active merchant run exists. It reuses a queued run with a unique asynq enqueue, treats duplicate-task enqueue as already queued, returns a processing run summary without re-enqueueing, and only creates a new run when no active queued/processing run remains.
- Fixed/current 2026-06-11: the Mini Program fallback wrapper `getMyApplication()` calls the registered read-only backend route `GET /v1/merchants/applications/me`. `getMyMerchantApplication` returns the latest current-user application without creating a draft or resetting a `submitted` application.
- OCR writeback guards are strong: job owner, media id, OCR job id, document type, side, and draft status are checked before writes.
- OCR pending/writeback uses full-field updates on one application row, not a shared JSON blob, so it avoids the group OCR sibling-overwrite problem.
- Fixed 2026-06-11: `ResetStaleMerchantOCRStatus` now treats both `pending` and `processing` application OCR JSON fields as stale cleanup candidates, matching the API pending marker and the OCR worker/job-table processing boundary.
- Submit can persist repaired business-license or food-permit OCR before final approval checks. A failed submit can therefore still mutate OCR JSON on the draft.
- Fixed 2026-06-11: Agreement consent audit on merchant submit is explicitly a submit-attempt intent record. Failed validation still keeps `merchant_application_submit_attempt_consent_confirmed` with `event_scope='submit_attempt'`, and tests prove it does not mean the application was submitted or approved.
- Fixed 2026-06-11: approval transaction still writes application/merchant/staff/user-role before review summary and credential-ledger activation, but post-approval retry now repairs the approved application through the existing merchant review run. The repair path refreshes approved review summary, completes queued/processing runs, rejects completed non-approved runs, activates only missing credential ledger entries, and re-runs credential restore eligibility without duplicating already-active ledgers.
- Fixed 2026-06-07: `ApproveMerchantApplicationTx` now uses owner-only `GetMerchantOwnedByUser`, so an applicant who is merely staff on another merchant creates or updates only their own merchant during application approval.
- Duplicate license and duplicate legal-person checks query `merchant_applications`, not credential ledger or merchant `application_data`; historical approved drafts and resubmitted drafts therefore define the uniqueness boundary.
- Storefront/environment image arrays are saved to application JSON URL lists for onboarding/draft material, not media asset ids. On approval they are copied into merchant live image fields, and later approved-merchant profile edits use the merchant-owned fields covered by `merchant-profile-update`.

## SQL And Durable State Boundaries

- `merchant_applications`: draft/status truth, OCR JSON, document asset ids, location/region, review summary, storefront/environment image URL lists.
- `media_assets`: uploaded private/public document/image ownership, moderation, visibility, and object-key truth.
- `ocr_jobs`: OCR job ownership, media binding, provider status, raw/normalized result, audit trail.
- `onboarding_review_runs`: queued/processing/completed/cancelled review evidence, rule hits, OCR job refs, requested/reviewed actors.
- `merchants`: activated merchant profile/status/open-state truth after approval, including live storefront/environment image arrays copied from the approved application.
- `merchant_staff`: merchant-specific owner membership created or repaired on approval.
- `user_roles`: coarse `merchant_owner` capability created if missing.
- `merchant_system_labels`: system labels reconciled after merchant creation/update.
- `credential_ledgers`: active business-license and food-permit credential evidence after approval.
- `merchant_profiles`: document-expiry takeout suspension can be released after eligible credential reactivation.
- Notification tables/tasks: restore notification is sent synchronously in API fallback or distributed by worker after async restore release.

## Trust, Authorization, And Tenant Checks

- Application routes are authenticated-user scoped, not staff scoped. Handlers use `authPayload.UserID`, `GetMerchantApplicationDraft`, `GetUserMerchantApplication`, or OCR owner checks.
- OCR creation allows access if the requester created the job, owns the OCR owner application, or is OCR admin.
- OCR media category/side checks are centralized in `ocr_media_authz.go` for merchant application documents.
- Document delete soft-deletes media through `mediaRegistry.SoftDelete(assetID, authPayload.UserID)` after clearing the draft binding; errors are logged and not fatal after draft mutation.
- Submit duplicate-location check excludes the same application owner's merchant but blocks nearby coordinates owned by other users.
- Approval transaction creates/updates a merchant owned by the application user id and ensures the user has owner-level merchant staff and user role.
- Reset transaction also checks the reset application owner and uses owner-only merchant lookup, so a mismatched caller cannot reset someone else's application and an applicant who is only staff on another merchant cannot downgrade or otherwise touch that merchant during application re-edit/reset.

## Idempotency And Duplicate-Submit Checks

- Frontend blocks duplicate save/submit/upload/reset with local flags.
- Basic-info PUT is last-write-wins partial update on the draft row; no version/idempotency key.
- Document upload creates a new media asset and OCR job each time. Current-binding guards make stale OCR result writeback idempotent at the application field level.
- Asynq duplicate OCR enqueue is suppressed by task options, but durable OCR job ids are still unique per upload/request.
- Fixed/current 2026-06-11: Submit permits `submitted` status for retry, and async submit now reuses the latest active merchant onboarding review run instead of creating a duplicate run. Queued-run retry re-enqueues the same run id with `asynq.Unique(30s)` and treats `ErrDuplicateTask` as already queued; processing-run retry returns the current review summary without reprocessing.
- Review worker skips cancelled runs and completed non-merchant runs; approval SQL only approves from `submitted`, so duplicate workers after approval do not re-approve. Fixed/current 2026-06-11: if a worker sees an existing merchant review run whose application has already been reset away from `submitted`, it treats the run as superseded/cancelled, records `needs_resubmit`, and refreshes the review summary instead of returning a retryable error. If the application is already approved, completed merchant runs can re-enter the repair path to reconcile summary and credentials without duplicate ledger activation.
- Approval transaction is atomic for application, merchant, owner staff, labels, and owner role. Review summary and credential ledger activation remain post-transaction work, with a retryable repair path for approved merchant applications.

## Recovery And Async Convergence Paths

- Frontend OCR wrapper waits for OCR job terminal state and then polls `GET /v1/merchant/application` until writeback is visible.
- OCR worker retries provider failures via asynq and writes failed OCR payloads while the job remains bound.
- Data cleanup scheduler resets long-running merchant OCR `pending`/`processing` fields after one hour.
- Submit can run async review via `onboarding:review` or fallback synchronously if queue creation/enqueue fails.
- Fixed/current 2026-06-11: explicit reset/edit cancels active merchant review runs as superseded in the reset transaction, and the worker has a defensive terminal skip plus summary refresh for existing runs whose application is already non-submitted. Approved applications with an existing run now have a repair/reconciliation path for review summary and credential ledgers. There is still no broad stale queued-run scanner for unrelated abandoned runs.
- Credential-governance restore can release takeout suspension after new active ledgers satisfy the matrix and can notify the owner.

## Frontend Draft And Backend Rehydration

- Page builds form/initialForm from backend response and OCR fallback fields.
- `hasChanges` blocks pull refresh and causes submit to save the draft before final submit.
- Document upload merges the latest backend draft after OCR writeback and keeps fallback local preview paths while public/private URLs resolve.
- `approved` status is editable/submittable in the frontend, matching backend reset-on-edit behavior; the status summary labels the path as "重新认证" and explains that the online shop keeps current data until the new review passes.
- Submit response is authoritative: queued async review, approved result, blocked review, or validation errors all flow back to page state/toasts.
- The `getMyApplication()` compatibility fallback is now backed by the read-only `/v1/merchants/applications/me` route, so conflict/status polling can fetch current application truth without triggering draft creation or submitted-to-draft reset.

## Test Coverage Signals

Observed tests:

- `locallife/api/merchant_application_test.go` covers submit success/validation branches, address matching, duplicate checks, nearby-but-distinct GPS locations beyond the hard-reject distance, queue-when-async-available, and sync fallback when enqueue fails.
- `locallife/api/ocr_test.go` covers merchant OCR pending write, owner/media auth, enqueue behavior, moderation-pending retry, and stale failure writeback.
- `locallife/api/ocr_async_response_test.go` covers async OCR fields in the application draft response.
- `locallife/worker/task_merchant_application_ocr_test.go` covers merchant OCR job execution, stale asset/status guards, readiness payloads, provider variants, and non-draft skip.
- `locallife/worker/task_onboarding_review_test.go` covers merchant review worker approval, credential activation, superseded non-submitted run cancellation with summary refresh, completed non-merchant run skip, and completed approved merchant-run repair.
- `locallife/logic/onboarding_review_service_test.go` and `logic/merchant_onboarding_review_service_test.go` cover review summary creation/completion, merchant superseded-run summary refresh, approved application post-approval repair, duplicate credential activation skip, completed non-approved repair fail-closed behavior, missing review-service fail-closed behavior, merchant credential activation/restore attempts, and approval param sanitization for invalid legacy application image JSON.
- `locallife/db/sqlc/tx_merchant_application_test.go` covers approval transaction owner role, owner staff behavior, application-image copy to merchant live truth, staff-associated merchant collision avoidance through owner-only lookup, reset preserving owned active/approved merchant status and live images, reset ignoring staff-associated merchants, reset failing closed on application/user owner mismatch, and approval fail-closed application/user owner mismatch. `locallife/db/sqlc/merchant_test.go` covers stale `pending`/`processing` merchant OCR cleanup while preserving recent pending and terminal payloads.
- `weapp/scripts/check-merchant-application-reverification-contract.test.js` covers approved application re-edit copy and page rendering for the reverification boundary.
- Fixed/current 2026-06-11: `locallife/api/merchant_application_test.go` covers `/v1/merchants/applications/me` returning a submitted application without reset and returning an empty state without creating a draft. It also covers `GET /v1/merchant/application` returning `submitted` without calling `ResetMerchantApplicationTx`, while adjacent basic-info tests prove explicit edits still reset submitted/approved applications to draft.
- Fixed/current 2026-06-11: `locallife/api/merchant_application_test.go` covers submitted retry reusing an existing queued run, using an asynq unique enqueue option, treating `ErrDuplicateTask` as an already queued response, returning an existing processing run without enqueueing, and falling back to synchronous review with the same queued run id when enqueue infrastructure fails. `locallife/db/sqlc/onboarding_review_test.go` covers the active-run lookup ignoring newer terminal runs.
- Fixed/current 2026-06-11: `locallife/api/merchant_application_test.go` covers failed required-field validation still writing a submit-attempt agreement consent audit with explicit action and metadata scope.

Missing high-value tests:

- Fixed/current 2026-06-11: approved application re-edit/reset contract is covered at the transaction and Mini Program copy/rendering boundary. Owned active/approved merchant truth remains live until reapproval, reset fails closed on application/user owner mismatch, staff-associated merchants are ignored, and merchants see the reverification explanation.
- Fixed/current 2026-06-11: explicit edit/reset of a submitted application cancels queued/processing merchant review runs as superseded and updates the application review summary; worker race coverage proves non-submitted existing runs do not retry and refresh the visible summary.
- Fixed/current 2026-06-11: stale pending OCR cleanup is covered at the SQL/sqlc boundary and matches the API pending marker.
- Fixed/current 2026-06-11: idempotent/retry submit semantics are covered for queued, duplicate queued-task, processing, and sync-fallback active review-run branches.
- Fixed/current 2026-06-11: agreement-consent audit semantics on failed validation are covered as a submit-attempt intent record, not successful submission evidence.
- Fixed/current 2026-06-11: post-approval review/credential repair is covered for processing-run completion, completed approved run idempotent credential skip, completed non-approved run rejection, missing review-service rejection, completed merchant worker re-entry, and completed rider worker skip.
- Fixed 2026-06-13: GPS duplicate-location hard reject is covered so merchants just beyond the same-point threshold can continue, while the rejection copy tells merchants to re-position at their own storefront rather than a neighboring storefront.

## Gaps And Refactor Notes

- Fixed 2026-06-11: `GET /v1/merchant/application` no longer resets `submitted` to draft; the route is now a read/create entry, while write paths still reset only after explicit edit intent.
- Fixed 2026-06-11: explicit edit/reset now marks active merchant review runs superseded/cancelled in `ResetMerchantApplicationTx`, and the worker treats existing non-submitted merchant runs as terminal superseded skips with `needs_resubmit` summary refresh instead of retryable failures.
- Fixed 2026-06-11: submitted retry no longer creates a second active merchant onboarding review run. The submit handler queries the latest queued/processing merchant run, reuses queued runs with queue-level uniqueness, returns processing summaries without duplicate enqueue, and keeps sync fallback pinned to the existing run id.
- Fixed 2026-06-11: approved re-edit is now a documented and tested reverification draft contract. Reset fails closed on application/user owner mismatch, uses owner-only merchant lookup, preserves owned active/approved merchant status and live images until reapproval, ignores staff-associated merchants, and the Mini Program renders the "线上店铺继续使用当前资料，重新审核通过后更新" guidance near the status summary.
- Fixed 2026-06-11: failed submit validation keeps a durable agreement-consent audit only as `merchant_application_submit_attempt_consent_confirmed` with `event_scope='submit_attempt'`, so compliance can prove consent was confirmed during a submission attempt while product/reporting does not confuse it with submitted/approved application state.
- Fixed 2026-06-07: `ApproveMerchantApplicationTx` lookup is restricted to merchants owned by `arg.UserID`, not staff-associated merchants, and the approval status update requires the application owner to match `arg.UserID`.
- Fixed/current 2026-06-11: frontend fallback wrapper `getMyApplication()` is aligned with registered read-only backend route `GET /v1/merchants/applications/me`.
- Fixed 2026-06-11: stale OCR cleanup now includes `pending` application OCR payloads as well as `processing`.
- Fixed 2026-06-11: approved merchant applications now have a post-approval repair/reconciliation path for review-run summary and credential ledgers. The design keeps review/credential work outside the approval transaction but makes worker/API retries repair queued, processing, or completed-approved merchant runs and dedupe existing active credential ledgers.
- Fixed 2026-06-13: duplicate-location approval validation now uses a 5-meter hard-reject boundary for effectively identical GPS points instead of rejecting nearby storefronts in the broader address validation radius; the rejection message gives stable merchant-facing recovery guidance.

## Branch Exhaustion

- Entry branches checked: Mini Program merchant application/settings page, group-independent merchant application draft, basic info save, document upload/delete, OCR polling/writeback, agreement consent, submit, approved-application edit/reset, storefront/environment image draft reads, and read-only `getMyApplication()` compatibility fallback. Flutter App has no merchant onboarding/OCR entry in `merchant_app/lib/features/**`. Web/operator review UI is out of current scope except backend review effects.
- Request branches checked: `GET/PUT/POST /v1/merchant/application`, read-only `GET /v1/merchants/applications/me`, document bind/delete routes, media upload session/complete/read, OCR create/poll, agreement consent, async onboarding review enqueue, synchronous review fallback, and credential restore notifications.
- Backend state branches checked: draft/submitted/approved/rejected/reset transitions, pending and processing OCR payloads in application JSON, current-asset stale guards, duplicate license/legal-person checks, address/geofence validation, narrow GPS duplicate-location hard reject, agreement audit write, async review-run creation/completion, owner-only approval transaction, merchant/profile/staff/user-role creation or update, merchant live image copy, system label reconciliation, credential-ledger activation, and takeout suspension release.
- Async branches checked: OCR asynq worker and provider retry, frontend OCR terminal polling plus application rehydration, stale OCR cleanup scheduler, onboarding review asynq worker, sync fallback when queue unavailable, credential restore notification worker/path, read-path no-reset behavior for submitted applications, explicit edit/reset superseded-review cancellation with worker race skip/summary refresh, submitted retry reuse of queued/processing onboarding review runs, and completed approved merchant-run repair.
- Failure/retry branches checked: duplicate frontend save/submit/upload guards, stale OCR result after asset change, stale pending/processing OCR cleanup, submit validation failure after submit-attempt agreement consent audit, nearby-but-distinct GPS location continuation, same-point duplicate location rejection copy, submit mutating OCR JSON before later failure, async enqueue fallback, explicit submitted/approved edit causing draft reset, review summary/credential activation failure after approval transaction, approved application repair for queued/processing/completed-approved review runs, completed non-approved repair fail-closed behavior, missing review-service fail-closed behavior, submitted retry with existing queued/processing review run, duplicate queued-task enqueue, and fallback using the same existing queued run id.
- Reader/consumer branches checked: application page form state, OCR result readers, approved merchant profile readers, merchant profile-image flow reading application images, operator/backend review readers, credential governance, and merchant open/status readiness that depends on activated merchant state.
- Authorization/tenant branches checked: application routes are authenticated-user scoped; OCR checks creator/application owner/OCR admin; media category/side checks apply for merchant application documents; document delete soft-deletes by user owner; approval creates/updates applicant-owned merchant truth through owner-only lookup, no longer follows staff-associated merchant rows, and fails closed if the transaction application id does not belong to the supplied user id; reset now also fails closed on application/user owner mismatch and no longer follows staff-associated merchant rows.
- Zombie/unreachable branches checked: the frontend fallback route is now registered/read-only, so it is no longer a zombie branch; stale pending/processing OCR cleanup now converges application OCR payloads; duplicate group/web review paths are separate flows; application image URL truth overlaps with merchant profile images only as onboarding/draft material and approved-only compatibility fallback.
- Test-proof gaps checked: existing tests cover application submit branches, nearby GPS hard-reject threshold behavior, stable duplicate-location recovery copy, failed-validation submit-attempt consent audit semantics, read-only current-application compatibility route behavior, `GET /v1/merchant/application` submitted no-reset behavior, explicit edit reset behavior, explicit edit/reset queued-review supersede cancellation, generic cancel `needs_resubmit` outcome, worker non-submitted superseded skip with summary refresh, submitted retry with existing queued/processing review run, duplicate queued-task enqueue handling, sync fallback reusing the same queued run id, approved application repair for missing review summary/credentials, duplicate credential activation skip, completed non-approved repair rejection, missing review service rejection, completed merchant worker repair, completed rider worker skip, OCR owner/media auth, async OCR response/writeback, stale pending/processing OCR cleanup, review worker approval, credential activation, approval transaction basics, approval image copy, approved re-edit preserving owned active/approved merchant truth, reset ignoring staff-associated merchants, reset application/user owner mismatch rejection, Mini Program reverification copy/rendering, staff-merchant collision avoidance, and application/user owner mismatch rejection.
