INSERT INTO appeals (
    id,
    claim_id,
    appellant_type,
    appellant_id,
    reason,
    status,
    reviewer_id,
    review_notes,
    reviewed_at,
    compensation_amount,
    compensated_at,
    region_id,
    created_at
)
SELECT
    id,
    claim_id,
    appellant_type,
    appellant_id,
    reason,
    status,
    reviewer_id,
    review_notes,
    reviewed_at,
    compensation_amount,
    compensated_at,
    region_id,
    created_at
FROM recovery_disputes
ON CONFLICT (claim_id, appellant_type) DO UPDATE
SET reason = EXCLUDED.reason,
    status = EXCLUDED.status,
    reviewer_id = EXCLUDED.reviewer_id,
    review_notes = EXCLUDED.review_notes,
    reviewed_at = EXCLUDED.reviewed_at,
    compensation_amount = EXCLUDED.compensation_amount,
    compensated_at = EXCLUDED.compensated_at,
    region_id = EXCLUDED.region_id,
    created_at = EXCLUDED.created_at;

SELECT setval(
    pg_get_serial_sequence('appeals', 'id'),
    COALESCE((SELECT MAX(id) FROM appeals), 1),
    EXISTS (SELECT 1 FROM appeals)
);

DROP TABLE IF EXISTS recovery_disputes;