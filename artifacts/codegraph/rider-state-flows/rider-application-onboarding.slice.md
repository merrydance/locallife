# Rider Application Onboarding Slice

Status: rider-state flow slice created 2026-06-08
Risk class: G3 - identity documents, OCR async writeback, health certificate validity, rider role activation, credential ledger lifecycle/restore/suspension, and review/recovery state
Scope: Mini Program rider registration page -> rider application routes -> OCR jobs/workers -> submit validation -> async/sync onboarding review -> riders/user_roles/credential/rider_profile durable truth

## Variant Coverage

This slice covers:

- Mini Program rider registration entry `pages/register/rider/index`, including draft load, basic save, ID card front/back upload, health certificate upload, health certificate OCR correction, reset, agreement consent, and submit.
- Rider application APIs under `/v1/rider/application/**`.
- Rider document media boundary: upload session/complete, private preview URL, application document delete, and media soft-delete side effect for the bound asset.
- Generic OCR job creation for `owner_type='rider_application'` plus rider-specific ID-card and health-certificate OCR workers.
- Async onboarding review run creation, `onboarding:review` worker path, synchronous fallback path, approval transaction, reject-to-draft path, credential ledger activation, 7-day expiry reminders, expired-credential suspension, suspension-slot collision, and suspension restore after renewed approval.

This slice does not fully cover:

- Operator/admin rider management pages after rider approval.
- Object storage/provider internals behind the shared media registry; this slice follows the rider document boundary through `media_assets`.
- Full OCR provider contracts; this slice follows local OCR job/readiness/writeback state only.
- Baofu settlement account onboarding after rider activation; that is covered by `rider-income-and-baofu-withdrawal`.

## Product Invariant

Rider onboarding must preserve the split between editable application truth and activated rider truth:

- Draft fields and document bindings may mutate only the authenticated user's `rider_applications` row while it is editable.
- OCR writeback must stay bound to the current media asset and current OCR job; stale document results must not overwrite newer uploads.
- Submit must reject incomplete profile fields, missing documents, pending/failed/incomplete OCR, expired ID card, expired/too-short health certificate, and name mismatches.
- Approval must atomically approve the application, create `riders`, and create `user_roles(role='rider')`.
- Credential governance must activate renewed rider credentials, remind before expiry, suspend expired rider credentials through `rider_profiles`, and only restore a suspension that it owns.
- Review rejection/needs-resubmit must return the application to `draft` with a durable reason instead of creating a rider.

## Primary Forward Chain

1. The Mini Program registration entry is declared as `pages/register/rider/index` in `weapp/miniprogram/app.json:64` through `weapp/miniprogram/app.json:72`.

2. The registration page loads the current application, normalizes draft/status state, and keeps upload previews tied to backend asset ids.
   Evidence: `weapp/miniprogram/pages/register/rider/index.ts:95`, `weapp/miniprogram/pages/register/rider/index.ts:117`, `weapp/miniprogram/pages/register/rider/index.ts:214`, `weapp/miniprogram/pages/register/rider/index.ts:226`.

3. Existing document previews call `POST /v1/media/private-access`, which loads `media_assets`, enforces owner-only private document access unless admin, and returns a short-lived download URL.
   Evidence: `weapp/miniprogram/pages/register/rider/index.ts:9`, `weapp/miniprogram/pages/register/rider/index.ts:214`, `weapp/miniprogram/pages/register/_main_shared/utils/image-security.ts:15`, `weapp/miniprogram/pages/register/_main_shared/utils/image-security.ts:24`, `locallife/api/server.go:658`, `locallife/api/media.go:189`, `locallife/api/media.go:199`, `locallife/api/media.go:209`, `locallife/api/media.go:228`.

4. The page uploads ID-card front/back and health certificate documents, starts OCR through shared OCR job wrappers, and ignores stale frontend request versions.
   Evidence: `weapp/miniprogram/pages/register/rider/index.ts:159`, `weapp/miniprogram/pages/register/rider/index.ts:254`, `weapp/miniprogram/pages/register/rider/index.ts:266`, `weapp/miniprogram/pages/register/rider/_api/rider-application.ts:248`, `weapp/miniprogram/pages/register/rider/_api/rider-application.ts:253`, `weapp/miniprogram/pages/register/rider/_api/rider-application.ts:277`, `weapp/miniprogram/pages/register/rider/_api/rider-application.ts:281`, `weapp/miniprogram/utils/media.ts:112`, `weapp/miniprogram/utils/media.ts:209`, `locallife/api/server.go:660`, `locallife/api/server.go:661`.

5. The page can manually patch health certificate OCR fields before submit when OCR needs user correction.
   Evidence: `weapp/miniprogram/pages/register/rider/index.ts:334`, `weapp/miniprogram/pages/register/rider/index.ts:436`, `weapp/miniprogram/pages/register/rider/_api/rider-application.ts:239`.

6. The frontend application wrappers call `GET /v1/rider/application`, `PUT /basic`, `PATCH /documents/health_cert/ocr-fields`, `POST /submit`, `POST /reset`, and document delete routes.
   Evidence: `weapp/miniprogram/pages/register/rider/_api/rider-application.ts:219`, `weapp/miniprogram/pages/register/rider/_api/rider-application.ts:229`, `weapp/miniprogram/pages/register/rider/_api/rider-application.ts:239`, `weapp/miniprogram/pages/register/rider/_api/rider-application.ts:304`, `weapp/miniprogram/pages/register/rider/_api/rider-application.ts:315`, `weapp/miniprogram/pages/register/rider/_api/rider-application.ts:324`.

7. Backend rider application routes are registered under the authenticated rider group before rider middleware is required, because the user may not yet be a rider.
   Evidence: `locallife/api/server.go:1125`, `locallife/api/server.go:1126`, `locallife/api/server.go:1127`, `locallife/api/server.go:1128`, `locallife/api/server.go:1129`, `locallife/api/server.go:1130`, `locallife/api/server.go:1131`.

8. `GET /v1/rider/application` returns the existing application as-is or creates a new `draft`; unlike merchant onboarding, it does not silently reset a `submitted` rider application on read.
   Evidence: `locallife/api/rider_application.go:123`, `locallife/api/rider_application.go:129`, `locallife/api/rider_application.go:140`, `locallife/db/query/rider_application.sql:1`, `locallife/db/query/rider_application.sql:13`.

9. Basic info update reloads the authenticated user's application, calls `ensureEditableRiderApplication`, then updates only `status='draft'` rows.
   Evidence: `locallife/api/rider_application.go:168`, `locallife/api/rider_application.go:185`, `locallife/api/rider_application.go:193`, `locallife/db/query/rider_application.sql:198`, `locallife/db/query/rider_application.sql:204`.

10. Reset is explicit and only allowed from `submitted`; it clears submitted/review traces and returns the application to `draft`.
   Evidence: `locallife/api/rider_application.go:236`, `locallife/api/rider_application.go:251`, `locallife/api/rider_application.go:256`, `locallife/db/query/rider_application.sql:177`, `locallife/db/query/rider_application.sql:183`.

11. Document delete accepts `id_card_front`, `id_card_back`, and `health_cert` only on editable draft applications; it clears the matching asset id and OCR payload from `rider_applications`, then best-effort soft-deletes the previously bound media asset.
    Evidence: `weapp/miniprogram/pages/register/rider/index.ts:273`, `weapp/miniprogram/pages/register/rider/index.ts:279`, `weapp/miniprogram/pages/register/rider/_utils/rider-application-document-workflow.ts:194`, `weapp/miniprogram/pages/register/rider/_api/rider-application.ts:329`, `locallife/api/rider_application_document_delete.go:15`, `locallife/api/rider_application_document_delete.go:35`, `locallife/api/rider_application_document_delete.go:62`, `locallife/api/rider_application_document_delete.go:77`, `locallife/api/rider_application_document_delete.go:97`, `locallife/api/rider_application_document_delete.go:119`, `locallife/api/rider_application_document_delete.go:136`.

12. OCR workers execute rider ID-card and health-certificate jobs, reload the application, verify the media asset is still bound, and then write success or failure payloads.
    Evidence: `locallife/worker/task_rider_application_ocr.go:164`, `locallife/worker/task_rider_application_ocr.go:181`, `locallife/worker/task_rider_application_ocr.go:183`, `locallife/worker/task_rider_application_ocr.go:222`, `locallife/worker/task_rider_application_ocr.go:226`, `locallife/worker/task_rider_application_ocr.go:338`, `locallife/worker/task_rider_application_ocr.go:352`, `locallife/worker/task_rider_application_ocr.go:383`.

13. ID-card OCR merges front-side name/number/gender/nation/address and back-side validity into one JSON payload with readiness flags.
    Evidence: `locallife/worker/task_rider_application_ocr.go:241`, `locallife/worker/task_rider_application_ocr.go:255`, `locallife/worker/task_rider_application_ocr.go:265`, `locallife/worker/task_rider_application_ocr.go:268`, `locallife/worker/task_rider_application_ocr.go:295`, `locallife/db/query/rider_application.sql:41`.

14. Health certificate OCR extracts or parses name, certificate number, ID number, and validity, then stores readiness flags for submit validation.
    Evidence: `locallife/worker/task_rider_application_ocr.go:387`, `locallife/worker/task_rider_application_ocr.go:398`, `locallife/worker/task_rider_application_ocr.go:400`, `locallife/worker/task_rider_application_ocr.go:404`, `locallife/worker/task_rider_application_ocr.go:407`, `locallife/worker/task_rider_application_ocr.go:433`.

15. Submit parses agreement consent, requires `draft`, writes consent audit, checks real name, phone, ID-card front/back media, health certificate media, and OCR readiness before setting `status='submitted'`.
    Evidence: `locallife/api/rider_application_submit.go:32`, `locallife/api/rider_application_submit.go:47`, `locallife/api/rider_application_submit.go:52`, `locallife/api/rider_application_submit.go:56`, `locallife/api/rider_application_submit.go:75`, `locallife/api/rider_application_submit.go:83`, `locallife/db/query/rider_application.sql:149`.

16. Submit rejects pending/processing/failed OCR and readiness gaps for ID-card name, ID number, ID validity, health certificate name, and health certificate validity.
    Evidence: `locallife/api/rider_application_submit.go:155`, `locallife/api/rider_application_submit.go:164`, `locallife/api/rider_application_submit.go:168`, `locallife/api/rider_application_submit.go:190`, `locallife/api/rider_application_submit.go:199`, `locallife/api/rider_application_submit.go:203`.

17. If onboarding review infrastructure is available, submit creates a rider review run and enqueues `onboarding:review`; otherwise it falls back to synchronous review.
    Evidence: `locallife/api/rider_application_submit.go:91`, `locallife/api/rider_application_submit.go:99`, `locallife/api/rider_application_submit.go:111`, `locallife/api/rider_application_submit.go:118`, `locallife/api/rider_application_submit.go:127`.

18. Rider review service only processes `submitted` applications, evaluates document validity/name rules, approves or returns to draft, and completes or records an onboarding review run.
    Evidence: `locallife/logic/rider_onboarding_review_service.go:81`, `locallife/logic/rider_onboarding_review_service.go:87`, `locallife/logic/rider_onboarding_review_service.go:91`, `locallife/logic/rider_onboarding_review_service.go:105`, `locallife/logic/rider_onboarding_review_service.go:118`, `locallife/logic/rider_onboarding_review_service.go:130`.

19. Approval transaction changes the application to `approved`, creates a rider, and creates the coarse `user_roles` rider capability in one transaction.
    Evidence: `locallife/db/sqlc/tx_rider_application.go:27`, `locallife/db/sqlc/tx_rider_application.go:34`, `locallife/db/sqlc/tx_rider_application.go:43`, `locallife/db/sqlc/tx_rider_application.go:55`.

20. After approval, credential governance activates rider credential ledgers and can restore a rider suspended for expired credentials by releasing only the credential-owned rider suspension and marking ledgers resumed.
    Evidence: `locallife/logic/rider_onboarding_review_service.go:143`, `locallife/logic/rider_onboarding_review_service.go:153`, `locallife/logic/rider_onboarding_review_service.go:161`, `locallife/logic/rider_onboarding_review_service.go:424`, `locallife/db/sqlc/tx_credential_ledger.go:162`, `locallife/db/query/trust_score.sql:208`.

21. Credential lifecycle schedulers send 7-day expiry reminders, claim rider suspension for expired active credentials when the suspension slot is free or already owned by credential expiry, mark credential ledgers suspended, and notify the rider; if another flow owns the suspension slot, the scheduler records the collision and does not overwrite it.
    Evidence: `locallife/scheduler/data_cleanup.go:825`, `locallife/scheduler/data_cleanup.go:833`, `locallife/scheduler/data_cleanup.go:924`, `locallife/scheduler/data_cleanup.go:932`, `locallife/scheduler/data_cleanup.go:983`, `locallife/scheduler/data_cleanup.go:1003`, `locallife/scheduler/data_cleanup.go:1017`, `locallife/db/query/credential_ledger.sql:154`, `locallife/db/query/credential_ledger.sql:161`, `locallife/db/query/trust_score.sql:192`, `locallife/db/query/notification.sql:1`.

## SQL And Durable State Boundaries

- `rider_applications`: draft/submitted/approved state, real name/phone, document asset ids, OCR JSON, reject reason, review summary, and submitted/review timestamps.
- `media_assets`: private document ownership, upload completion, short-lived private preview access, and soft-delete truth for ID-card and health certificate images.
- `ocr_jobs`: provider execution state, normalized result, media binding, and audit trail.
- `onboarding_review_runs`: queued/processing/completed rider review evidence and rule decision snapshot.
- `riders`: activated rider profile, status, deposit, region, location, online flag, stats, and application link.
- `rider_profiles`: credential-expiry suspension state claimed/restored by credential governance.
- `user_roles`: coarse rider capability attached to the approved rider id.
- `credential_ledgers`: active rider ID-card and health-certificate evidence used by credential governance, reminder, suspension, and restore.
- `notifications`: rider credential reminder/suspension messages when the scheduler cannot or does not use the async notification task path.

## Trust, Authorization, And Tenant Checks

- Rider application routes use authenticated `authPayload.UserID`; no rider role is required before approval.
- All application reads and writes resolve `rider_applications` by the authenticated user id, not by client-supplied application id.
- Private document previews are mediated by `/v1/media/private-access`, which allows the uploader or an admin, and owner-only categories reject other users.
- OCR owner/media checks bind document type and media asset to the application; worker writeback rechecks the current asset before success/failure writes.
- Health certificate OCR correction is server-side scoped to the current application document.
- Approval transaction derives the rider user id from the approved application row.
- Credential-expiry suspension uses `ClaimRiderSuspensionIfAvailable`, so it does not overwrite a rider suspension owned by another behavior or risk flow.
- Credential restore uses `ReleaseRiderSuspensionIfOwned`, so renewed approval only releases the credential-expiry suspension reason it owns.

## Idempotency And Duplicate-Submit Checks

- Frontend tracks request/upload versions and submit/loading flags.
- Basic info update is last-write-wins on the draft row.
- Each document upload can create a new media asset and OCR job; stale worker results skip writeback when the asset binding changed.
- `GET /v1/rider/application` is idempotent after the draft exists.
- Submit requires `draft`; duplicate submit after the first successful status change is rejected unless the application returns to draft.
- Async enqueue failure falls back to synchronous review; duplicate queued review runs remain possible if callers race before the status update boundary, but review service refuses non-submitted applications.
- Credential reminders are deduped by `credential_ledgers.last_reminded_at`; expired-credential suspension is idempotent when the ledger is already marked with the credential-expiry reason.

## Recovery And Async Convergence Paths

- Frontend OCR polling rehydrates application truth after job terminal state.
- OCR worker retries provider failures through the job/task layer and writes failed payloads only while the same asset is still bound.
- Submit can converge through async `onboarding:review` or synchronous fallback.
- Credential restore notification is best-effort after approved application and credential activation.
- Credential expiry reminder/suspension can converge through the scheduler's async notification task path or direct `notifications` insert fallback.

## Frontend Draft And Backend Rehydration

- The page keeps local upload previews but treats backend asset ids and OCR payloads as canonical after refresh.
- Preview URL failures return an empty URL and do not mutate backend truth; refreshed backend asset ids remain canonical.
- Health certificate valid-end correction is explicitly saved before confirmation/submit.
- Submit response can be queued/submitted, approved, or returned to draft with reason; the page rehydrates from response.

## Test Coverage Signals

Observed tests include rider application API/worker/review coverage in nearby files such as `locallife/api/rider_application*_test.go`, `locallife/worker/task_rider_application_ocr_test.go`, `locallife/logic/rider_onboarding_review_service_test.go`, and `locallife/db/sqlc/tx_rider_application_test.go` where present in the repository.

Missing high-value tests:

- Contract test proving `GET /v1/rider/application` does not reset `submitted`.
- Duplicate submit race while async review infrastructure is available.
- Health certificate manual correction plus OCR readiness submit branch.
- Credential restore notification failure after approved application and activated credentials.
- Credential expiry lifecycle: 7-day rider reminder, expired rider suspension, suspension-slot collision, direct notification fallback, and renewed-approval restore.

## Gaps And Refactor Notes

- Decide whether review-run cancellation is needed when a submitted application is explicitly reset to draft.
- Add a stale OCR cleanup policy for rider application pending/processing payloads if not already covered by a shared OCR cleanup path.
- Make the product copy explicit that submitted rider applications can be reset only by a deliberate user action.
- Add an operator-visible audit/report path for rider credential lifecycle suspensions and restore outcomes if this is not already covered by generic credential governance alerts.

## Branch Exhaustion

- Entry branches checked: rider registration load, private preview refresh, basic save, ID-card front/back upload/delete, health certificate upload/delete, health cert OCR correction, agreement consent, reset, and submit.
- Request branches checked: `GET/PUT/PATCH/DELETE/POST /v1/rider/application/**`, `/v1/media/upload-sessions`, `/v1/media/complete`, `/v1/media/private-access`, OCR job creation/polling, and async review enqueue/fallback.
- Backend state branches checked: no application, draft, submitted, approved, reject-to-draft, reset-to-draft, invalid document type, delete while non-draft, missing media, private media not owner/admin, soft-delete failure, pending/processing/failed/done OCR, ID-card expiry, health-certificate expiry/too-short validity, health-vs-ID name mismatch, approve transaction, rider role creation, credential ledger activation, credential expiry reminder, expired credential suspension, suspension slot occupied by another flow, and credential restore.
- Async branches checked: OCR success/failure/stale writeback, review run queued/worker path, sync fallback, credential reminder/suspension notification fallback, credential restore notification.
- Dead/orphan branches checked: no stale rider-application route was found in this slice.
