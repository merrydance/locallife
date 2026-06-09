ALTER TABLE merchant_group_join_requests
DROP CONSTRAINT IF EXISTS merchant_group_join_requests_group_id_merchant_id_status_key;

WITH ranked_pending AS (
    SELECT
        id,
        row_number() OVER (
            PARTITION BY group_id, merchant_id
            ORDER BY created_at DESC, id DESC
        ) AS pending_rank
    FROM merchant_group_join_requests
    WHERE status = 'pending'
)
UPDATE merchant_group_join_requests AS req
SET status = 'cancelled',
    reviewed_at = COALESCE(req.reviewed_at, now())
FROM ranked_pending
WHERE req.id = ranked_pending.id
  AND ranked_pending.pending_rank > 1;

CREATE UNIQUE INDEX IF NOT EXISTS merchant_group_join_requests_one_pending_per_pair_uidx
ON merchant_group_join_requests(group_id, merchant_id)
WHERE status = 'pending';
