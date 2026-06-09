WITH latest_applications AS (
    SELECT
        id,
        applicant_user_id,
        status,
        row_number() OVER (
            PARTITION BY applicant_user_id
            ORDER BY created_at DESC, id DESC
        ) AS application_rank
    FROM merchant_group_applications
),
draft_cleanup AS (
    SELECT
        draft.id
    FROM merchant_group_applications AS draft
    LEFT JOIN latest_applications AS latest
      ON latest.applicant_user_id = draft.applicant_user_id
     AND latest.application_rank = 1
     AND latest.id = draft.id
     AND latest.status = 'draft'
    WHERE draft.status = 'draft'
      AND latest.id IS NULL
)
UPDATE merchant_group_applications AS app
SET status = 'rejected',
    reject_reason = COALESCE(NULLIF(app.reject_reason, ''), 'superseded by latest application during draft uniqueness hardening'),
    updated_at = now()
FROM draft_cleanup
WHERE app.id = draft_cleanup.id;

CREATE UNIQUE INDEX IF NOT EXISTS merchant_group_applications_one_draft_per_applicant_uidx
ON merchant_group_applications(applicant_user_id)
WHERE status = 'draft';
