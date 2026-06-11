-- name: CreateMerchantOnboardingReviewRun :one
INSERT INTO onboarding_review_runs (
    application_type,
    merchant_application_id,
    run_status,
    stage,
    evidence,
    rule_hits,
    ocr_job_refs,
    snapshot,
    requested_by
) VALUES (
    'merchant',
    sqlc.arg(merchant_application_id),
    sqlc.arg(run_status),
    sqlc.arg(stage),
    sqlc.arg(evidence),
    sqlc.arg(rule_hits),
    sqlc.arg(ocr_job_refs),
    sqlc.arg(snapshot),
    sqlc.narg(requested_by)
)
RETURNING id, application_type, merchant_application_id, rider_application_id, run_status, stage, outcome, reason_code, reason_message, evidence, rule_hits, ocr_job_refs, snapshot, requested_by, reviewed_by, queued_at, started_at, finished_at, created_at, updated_at;

-- name: CreateRiderOnboardingReviewRun :one
INSERT INTO onboarding_review_runs (
    application_type,
    rider_application_id,
    run_status,
    stage,
    evidence,
    rule_hits,
    ocr_job_refs,
    snapshot,
    requested_by
) VALUES (
    'rider',
    sqlc.arg(rider_application_id),
    sqlc.arg(run_status),
    sqlc.arg(stage),
    sqlc.arg(evidence),
    sqlc.arg(rule_hits),
    sqlc.arg(ocr_job_refs),
    sqlc.arg(snapshot),
    sqlc.narg(requested_by)
)
RETURNING id, application_type, merchant_application_id, rider_application_id, run_status, stage, outcome, reason_code, reason_message, evidence, rule_hits, ocr_job_refs, snapshot, requested_by, reviewed_by, queued_at, started_at, finished_at, created_at, updated_at;

-- name: GetOnboardingReviewRun :one
SELECT id, application_type, merchant_application_id, rider_application_id, run_status, stage, outcome, reason_code, reason_message, evidence, rule_hits, ocr_job_refs, snapshot, requested_by, reviewed_by, queued_at, started_at, finished_at, created_at, updated_at
FROM onboarding_review_runs
WHERE id = $1;

-- name: GetLatestMerchantOnboardingReviewRun :one
SELECT id, application_type, merchant_application_id, rider_application_id, run_status, stage, outcome, reason_code, reason_message, evidence, rule_hits, ocr_job_refs, snapshot, requested_by, reviewed_by, queued_at, started_at, finished_at, created_at, updated_at
FROM onboarding_review_runs
WHERE merchant_application_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetLatestActiveMerchantOnboardingReviewRun :one
SELECT id, application_type, merchant_application_id, rider_application_id, run_status, stage, outcome, reason_code, reason_message, evidence, rule_hits, ocr_job_refs, snapshot, requested_by, reviewed_by, queued_at, started_at, finished_at, created_at, updated_at
FROM onboarding_review_runs
WHERE merchant_application_id = $1
  AND application_type = 'merchant'
  AND run_status IN ('queued', 'processing')
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: GetLatestRiderOnboardingReviewRun :one
SELECT id, application_type, merchant_application_id, rider_application_id, run_status, stage, outcome, reason_code, reason_message, evidence, rule_hits, ocr_job_refs, snapshot, requested_by, reviewed_by, queued_at, started_at, finished_at, created_at, updated_at
FROM onboarding_review_runs
WHERE rider_application_id = $1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: ListQueuedOnboardingReviewRuns :many
SELECT id, application_type, merchant_application_id, rider_application_id, run_status, stage, outcome, reason_code, reason_message, evidence, rule_hits, ocr_job_refs, snapshot, requested_by, reviewed_by, queued_at, started_at, finished_at, created_at, updated_at
FROM onboarding_review_runs
WHERE run_status = 'queued'
  AND ((sqlc.arg(application_type))::text = '' OR application_type = (sqlc.arg(application_type))::text)
ORDER BY queued_at ASC, id ASC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: MarkOnboardingReviewRunProcessing :one
UPDATE onboarding_review_runs
SET run_status = 'processing',
    started_at = COALESCE(started_at, NOW()),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND run_status = 'queued'
RETURNING id, application_type, merchant_application_id, rider_application_id, run_status, stage, outcome, reason_code, reason_message, evidence, rule_hits, ocr_job_refs, snapshot, requested_by, reviewed_by, queued_at, started_at, finished_at, created_at, updated_at;

-- name: CompleteOnboardingReviewRun :one
UPDATE onboarding_review_runs
SET run_status = 'completed',
    stage = sqlc.arg(stage),
    outcome = sqlc.arg(outcome),
    reason_code = sqlc.arg(reason_code),
    reason_message = COALESCE(sqlc.narg(reason_message), reason_message),
    evidence = COALESCE(sqlc.narg(evidence), evidence),
    rule_hits = COALESCE(sqlc.narg(rule_hits), rule_hits),
    ocr_job_refs = COALESCE(sqlc.narg(ocr_job_refs), ocr_job_refs),
    snapshot = COALESCE(sqlc.narg(snapshot), snapshot),
    reviewed_by = COALESCE(sqlc.narg(reviewed_by), reviewed_by),
    finished_at = NOW(),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND run_status IN ('queued', 'processing')
RETURNING id, application_type, merchant_application_id, rider_application_id, run_status, stage, outcome, reason_code, reason_message, evidence, rule_hits, ocr_job_refs, snapshot, requested_by, reviewed_by, queued_at, started_at, finished_at, created_at, updated_at;

-- name: CancelOnboardingReviewRun :one
UPDATE onboarding_review_runs
SET run_status = 'cancelled',
    outcome = 'needs_resubmit',
    reason_code = sqlc.arg(reason_code),
    reason_message = COALESCE(sqlc.narg(reason_message), reason_message),
    finished_at = NOW(),
    updated_at = NOW()
WHERE id = sqlc.arg(id)
  AND run_status IN ('queued', 'processing')
RETURNING id, application_type, merchant_application_id, rider_application_id, run_status, stage, outcome, reason_code, reason_message, evidence, rule_hits, ocr_job_refs, snapshot, requested_by, reviewed_by, queued_at, started_at, finished_at, created_at, updated_at;

-- name: CancelActiveMerchantOnboardingReviewRunsForApplication :many
UPDATE onboarding_review_runs
SET run_status = 'cancelled',
    outcome = sqlc.arg(outcome),
    reason_code = sqlc.arg(reason_code),
    reason_message = COALESCE(sqlc.narg(reason_message), reason_message),
    finished_at = NOW(),
    updated_at = NOW()
WHERE application_type = 'merchant'
  AND merchant_application_id = sqlc.arg(merchant_application_id)
  AND run_status IN ('queued', 'processing')
RETURNING id, application_type, merchant_application_id, rider_application_id, run_status, stage, outcome, reason_code, reason_message, evidence, rule_hits, ocr_job_refs, snapshot, requested_by, reviewed_by, queued_at, started_at, finished_at, created_at, updated_at;
