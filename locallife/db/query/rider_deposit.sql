-- name: CreateRiderDeposit :one
INSERT INTO rider_deposits (
    rider_id,
    amount,
    type,
    related_order_id,
    payment_order_id,
    balance_after,
    remark
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
) RETURNING *;

-- name: GetRiderDeposit :one
SELECT id, rider_id, amount, type, related_order_id, balance_after, remark, created_at, payment_order_id FROM rider_deposits
WHERE id = $1 LIMIT 1;

-- name: GetRiderDepositByPaymentOrderID :one
SELECT id, rider_id, amount, type, related_order_id, balance_after, remark, created_at, payment_order_id FROM rider_deposits
WHERE payment_order_id = $1 AND type = 'deposit' LIMIT 1;

-- name: ListRiderDeposits :many
SELECT id, rider_id, amount, type, related_order_id, balance_after, remark, created_at, payment_order_id FROM rider_deposits
WHERE rider_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: CountRiderDeposits :one
SELECT COUNT(*)::bigint
FROM rider_deposits
WHERE rider_id = $1;

-- name: ListRiderDepositsByType :many
SELECT id, rider_id, amount, type, related_order_id, balance_after, remark, created_at, payment_order_id FROM rider_deposits
WHERE rider_id = $1 AND type = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: GetRiderDepositStats :one
SELECT 
    COALESCE(SUM(CASE WHEN type = 'deposit' THEN amount ELSE 0 END), 0) AS total_deposit,
    COALESCE(SUM(CASE WHEN type = 'withdraw' THEN amount ELSE 0 END), 0) AS total_withdraw,
    COALESCE(SUM(CASE WHEN type = 'deduct' THEN amount ELSE 0 END), 0) AS total_deduct
FROM rider_deposits
WHERE rider_id = $1;

-- name: ListRiderDepositLedgerAnomalies :many
WITH rider_scope AS (
    SELECT id, user_id, deposit_amount, frozen_deposit
    FROM riders
    WHERE sqlc.narg('rider_id')::bigint IS NULL OR id = sqlc.narg('rider_id')::bigint
), credit_totals AS (
    SELECT rdc.rider_id, COALESCE(SUM(rdc.refundable_amount), 0)::bigint AS refundable_credit_amount
    FROM rider_deposit_credits rdc
    JOIN rider_scope rs ON rs.id = rdc.rider_id
    WHERE rdc.status IN ('active', 'partially_refunded', 'legacy')
    GROUP BY rdc.rider_id
), pending_refunds AS (
    SELECT po.user_id, COALESCE(SUM(ro.refund_amount), 0)::bigint AS pending_refund_amount
    FROM refund_orders ro
    JOIN payment_orders po ON po.id = ro.payment_order_id
    JOIN rider_scope rs ON rs.user_id = po.user_id
    WHERE po.business_type = 'rider_deposit'
      AND ro.refund_type = 'rider_deposit'
      AND ro.status IN ('pending', 'processing')
    GROUP BY po.user_id
), success_refunds AS (
    SELECT po.user_id, po.id AS payment_order_id, COALESCE(SUM(ro.refund_amount), 0)::bigint AS success_refund_amount
    FROM refund_orders ro
    JOIN payment_orders po ON po.id = ro.payment_order_id
        JOIN rider_scope rs ON rs.user_id = po.user_id
    WHERE po.business_type = 'rider_deposit'
      AND ro.refund_type = 'rider_deposit'
      AND ro.status = 'success'
    GROUP BY po.user_id, po.id
), withdraw_logs AS (
        SELECT rd.rider_id, rd.payment_order_id, COALESCE(SUM(rd.amount), 0)::bigint AS withdraw_log_amount
        FROM rider_deposits rd
        JOIN rider_scope rs ON rs.id = rd.rider_id
        WHERE rd.type IN ('withdraw', 'deduct')
            AND rd.payment_order_id IS NOT NULL
        GROUP BY rd.rider_id, rd.payment_order_id
), duplicate_deposit_logs AS (
        SELECT rd.rider_id, rd.payment_order_id, COUNT(*)::bigint AS duplicate_count, COALESCE(SUM(rd.amount), 0)::bigint AS duplicate_amount
        FROM rider_deposits rd
        JOIN rider_scope rs ON rs.id = rd.rider_id
        WHERE rd.type = 'deposit'
            AND rd.payment_order_id IS NOT NULL
        GROUP BY rd.rider_id, rd.payment_order_id
    HAVING COUNT(*) > 1
), anomalies AS (
    SELECT
        rs.id AS rider_id,
        rs.user_id,
        'deposit_amount_gt_refundable_credit'::text AS anomaly_type,
        NULL::bigint AS payment_order_id,
        rs.deposit_amount,
        rs.frozen_deposit,
        COALESCE(ct.refundable_credit_amount, 0)::bigint AS refundable_credit_amount,
        COALESCE(pr.pending_refund_amount, 0)::bigint AS pending_refund_amount,
        0::bigint AS success_refund_amount,
        0::bigint AS ledger_amount,
        (rs.deposit_amount - COALESCE(ct.refundable_credit_amount, 0) - COALESCE(pr.pending_refund_amount, 0))::bigint AS anomaly_amount
    FROM rider_scope rs
    LEFT JOIN credit_totals ct ON ct.rider_id = rs.id
    LEFT JOIN pending_refunds pr ON pr.user_id = rs.user_id
    WHERE rs.deposit_amount > COALESCE(ct.refundable_credit_amount, 0) + COALESCE(pr.pending_refund_amount, 0)

    UNION ALL

    SELECT
        rs.id AS rider_id,
        rs.user_id,
        'refundable_credit_gt_deposit_amount'::text AS anomaly_type,
        NULL::bigint AS payment_order_id,
        rs.deposit_amount,
        rs.frozen_deposit,
        COALESCE(ct.refundable_credit_amount, 0)::bigint AS refundable_credit_amount,
        COALESCE(pr.pending_refund_amount, 0)::bigint AS pending_refund_amount,
        0::bigint AS success_refund_amount,
        0::bigint AS ledger_amount,
        (COALESCE(ct.refundable_credit_amount, 0) + COALESCE(pr.pending_refund_amount, 0) - rs.deposit_amount)::bigint AS anomaly_amount
    FROM rider_scope rs
    LEFT JOIN credit_totals ct ON ct.rider_id = rs.id
    LEFT JOIN pending_refunds pr ON pr.user_id = rs.user_id
    WHERE COALESCE(ct.refundable_credit_amount, 0) + COALESCE(pr.pending_refund_amount, 0) > rs.deposit_amount

    UNION ALL

    SELECT
        rs.id AS rider_id,
        rs.user_id,
        'frozen_less_than_pending_refund'::text AS anomaly_type,
        NULL::bigint AS payment_order_id,
        rs.deposit_amount,
        rs.frozen_deposit,
        COALESCE(ct.refundable_credit_amount, 0)::bigint AS refundable_credit_amount,
        COALESCE(pr.pending_refund_amount, 0)::bigint AS pending_refund_amount,
        0::bigint AS success_refund_amount,
        0::bigint AS ledger_amount,
        (COALESCE(pr.pending_refund_amount, 0) - rs.frozen_deposit)::bigint AS anomaly_amount
    FROM rider_scope rs
    LEFT JOIN credit_totals ct ON ct.rider_id = rs.id
    LEFT JOIN pending_refunds pr ON pr.user_id = rs.user_id
    WHERE COALESCE(pr.pending_refund_amount, 0) > rs.frozen_deposit

    UNION ALL

    SELECT
        rs.id AS rider_id,
        rs.user_id,
        'duplicate_deposit_log'::text AS anomaly_type,
        ddl.payment_order_id,
        rs.deposit_amount,
        rs.frozen_deposit,
        COALESCE(ct.refundable_credit_amount, 0)::bigint AS refundable_credit_amount,
        COALESCE(pr.pending_refund_amount, 0)::bigint AS pending_refund_amount,
        0::bigint AS success_refund_amount,
        ddl.duplicate_amount::bigint AS ledger_amount,
        ddl.duplicate_count::bigint AS anomaly_amount
    FROM duplicate_deposit_logs ddl
    JOIN rider_scope rs ON rs.id = ddl.rider_id
    LEFT JOIN credit_totals ct ON ct.rider_id = rs.id
    LEFT JOIN pending_refunds pr ON pr.user_id = rs.user_id

    UNION ALL

    SELECT
        rs.id AS rider_id,
        rs.user_id,
        'success_refund_not_settled'::text AS anomaly_type,
        sr.payment_order_id,
        rs.deposit_amount,
        rs.frozen_deposit,
        COALESCE(ct.refundable_credit_amount, 0)::bigint AS refundable_credit_amount,
        COALESCE(pr.pending_refund_amount, 0)::bigint AS pending_refund_amount,
        sr.success_refund_amount::bigint AS success_refund_amount,
        COALESCE(wl.withdraw_log_amount, 0)::bigint AS ledger_amount,
        (sr.success_refund_amount - COALESCE(wl.withdraw_log_amount, 0))::bigint AS anomaly_amount
    FROM success_refunds sr
    JOIN rider_scope rs ON rs.user_id = sr.user_id
    LEFT JOIN withdraw_logs wl ON wl.rider_id = rs.id AND wl.payment_order_id = sr.payment_order_id
    LEFT JOIN credit_totals ct ON ct.rider_id = rs.id
    LEFT JOIN pending_refunds pr ON pr.user_id = rs.user_id
    WHERE sr.success_refund_amount > COALESCE(wl.withdraw_log_amount, 0)
)
SELECT rider_id, user_id, anomaly_type, payment_order_id, deposit_amount, frozen_deposit, refundable_credit_amount, pending_refund_amount, success_refund_amount, ledger_amount, anomaly_amount
FROM anomalies
ORDER BY rider_id, anomaly_type, payment_order_id NULLS LAST
LIMIT sqlc.arg('limit')::int;
