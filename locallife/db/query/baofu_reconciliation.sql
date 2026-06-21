-- name: GetBaofuDailyReconciliation :many
WITH baofu_reconciliation_parts AS (
    SELECT
        po.paid_at::date AS date,
        'baofu'::text AS provider,
        po.payment_channel AS channel,
        COALESCE(SUM(po.amount), 0)::bigint AS paid_amount,
        0::bigint AS payment_fee,
        0::bigint AS provider_payment_fee,
        0::bigint AS merchant_payment_fee,
        0::bigint AS rider_payment_fee,
        0::bigint AS platform_payment_fee_income,
        0::bigint AS platform_net_payment_fee_margin,
        0::bigint AS merchant_amount,
        0::bigint AS rider_amount,
        0::bigint AS platform_commission,
        0::bigint AS operator_commission,
        0::bigint AS withdraw_succeeded_amount,
        0::bigint AS withdraw_processing_amount,
        0::bigint AS unapplied_fact_count,
        0::bigint AS unknown_command_count,
        0::bigint AS fee_ledger_mismatch_count
    FROM payment_orders po
    WHERE po.payment_channel = 'baofu_aggregate'
      AND po.status = 'paid'
      AND po.paid_at >= sqlc.arg(start_at)
      AND po.paid_at <= sqlc.arg(end_at)
    GROUP BY po.paid_at::date, po.payment_channel

    UNION ALL

    SELECT
        COALESCE(pso.finished_at, pso.created_at)::date AS date,
        pso.provider,
        pso.channel,
        0::bigint AS paid_amount,
        COALESCE(SUM(CASE WHEN pso.calculation_version = 'baofu_fee_v2' THEN pso.provider_payment_fee ELSE pso.payment_fee END), 0)::bigint AS payment_fee,
        COALESCE(SUM(CASE WHEN pso.calculation_version = 'baofu_fee_v2' THEN pso.provider_payment_fee ELSE pso.payment_fee END), 0)::bigint AS provider_payment_fee,
        COALESCE(SUM(pso.merchant_payment_fee), 0)::bigint AS merchant_payment_fee,
        COALESCE(SUM(pso.rider_payment_fee), 0)::bigint AS rider_payment_fee,
        COALESCE(SUM(pso.merchant_payment_fee + pso.rider_payment_fee), 0)::bigint AS platform_payment_fee_income,
        COALESCE(SUM(pso.merchant_payment_fee + pso.rider_payment_fee - CASE WHEN pso.calculation_version = 'baofu_fee_v2' THEN pso.provider_payment_fee ELSE pso.payment_fee END), 0)::bigint AS platform_net_payment_fee_margin,
        COALESCE(SUM(pso.merchant_amount), 0)::bigint AS merchant_amount,
        COALESCE(SUM(pso.rider_amount), 0)::bigint AS rider_amount,
        COALESCE(SUM(pso.platform_commission), 0)::bigint AS platform_commission,
        COALESCE(SUM(pso.operator_commission), 0)::bigint AS operator_commission,
        0::bigint AS withdraw_succeeded_amount,
        0::bigint AS withdraw_processing_amount,
        0::bigint AS unapplied_fact_count,
        0::bigint AS unknown_command_count,
        0::bigint AS fee_ledger_mismatch_count
    FROM profit_sharing_orders pso
    WHERE pso.provider = 'baofu'
      AND pso.channel = 'baofu_aggregate'
      AND COALESCE(pso.finished_at, pso.created_at) >= sqlc.arg(start_at)
      AND COALESCE(pso.finished_at, pso.created_at) <= sqlc.arg(end_at)
    GROUP BY COALESCE(pso.finished_at, pso.created_at)::date, pso.provider, pso.channel

    UNION ALL

    SELECT
        COALESCE(bwo.finished_at, bwo.created_at)::date AS date,
        'baofu'::text AS provider,
        'baofu_aggregate'::text AS channel,
        0::bigint AS paid_amount,
        0::bigint AS payment_fee,
        0::bigint AS provider_payment_fee,
        0::bigint AS merchant_payment_fee,
        0::bigint AS rider_payment_fee,
        0::bigint AS platform_payment_fee_income,
        0::bigint AS platform_net_payment_fee_margin,
        0::bigint AS merchant_amount,
        0::bigint AS rider_amount,
        0::bigint AS platform_commission,
        0::bigint AS operator_commission,
        COALESCE(SUM(CASE WHEN bwo.status = 'succeeded' THEN bwo.amount ELSE 0 END), 0)::bigint AS withdraw_succeeded_amount,
        COALESCE(SUM(CASE WHEN bwo.status = 'processing' THEN bwo.amount ELSE 0 END), 0)::bigint AS withdraw_processing_amount,
        0::bigint AS unapplied_fact_count,
        0::bigint AS unknown_command_count,
        0::bigint AS fee_ledger_mismatch_count
    FROM baofu_withdrawal_orders bwo
    WHERE bwo.status IN ('succeeded', 'processing')
      AND COALESCE(bwo.finished_at, bwo.created_at) >= sqlc.arg(start_at)
      AND COALESCE(bwo.finished_at, bwo.created_at) <= sqlc.arg(end_at)
    GROUP BY COALESCE(bwo.finished_at, bwo.created_at)::date

    UNION ALL

    SELECT
        epf.observed_at::date AS date,
        epf.provider,
        epf.channel,
        0::bigint AS paid_amount,
        0::bigint AS payment_fee,
        0::bigint AS provider_payment_fee,
        0::bigint AS merchant_payment_fee,
        0::bigint AS rider_payment_fee,
        0::bigint AS platform_payment_fee_income,
        0::bigint AS platform_net_payment_fee_margin,
        0::bigint AS merchant_amount,
        0::bigint AS rider_amount,
        0::bigint AS platform_commission,
        0::bigint AS operator_commission,
        0::bigint AS withdraw_succeeded_amount,
        0::bigint AS withdraw_processing_amount,
        COUNT(*)::bigint AS unapplied_fact_count,
        0::bigint AS unknown_command_count,
        0::bigint AS fee_ledger_mismatch_count
    FROM external_payment_facts epf
    WHERE epf.provider = 'baofu'
      AND epf.channel = 'baofu_aggregate'
      AND epf.processing_status = 'failed'
      AND epf.observed_at >= sqlc.arg(start_at)
      AND epf.observed_at <= sqlc.arg(end_at)
    GROUP BY epf.observed_at::date, epf.provider, epf.channel

    UNION ALL

    SELECT
        epc.submitted_at::date AS date,
        epc.provider,
        epc.channel,
        0::bigint AS paid_amount,
        0::bigint AS payment_fee,
        0::bigint AS provider_payment_fee,
        0::bigint AS merchant_payment_fee,
        0::bigint AS rider_payment_fee,
        0::bigint AS platform_payment_fee_income,
        0::bigint AS platform_net_payment_fee_margin,
        0::bigint AS merchant_amount,
        0::bigint AS rider_amount,
        0::bigint AS platform_commission,
        0::bigint AS operator_commission,
        0::bigint AS withdraw_succeeded_amount,
        0::bigint AS withdraw_processing_amount,
        0::bigint AS unapplied_fact_count,
        COUNT(*)::bigint AS unknown_command_count,
        0::bigint AS fee_ledger_mismatch_count
    FROM external_payment_commands epc
    WHERE epc.provider = 'baofu'
      AND epc.channel = 'baofu_aggregate'
      AND epc.command_status = 'unknown'
      AND epc.submitted_at >= sqlc.arg(start_at)
      AND epc.submitted_at <= sqlc.arg(end_at)
    GROUP BY epc.submitted_at::date, epc.provider, epc.channel

    UNION ALL

    SELECT
        COALESCE(pso.finished_at, pso.created_at)::date AS date,
        pso.provider,
        pso.channel,
        0::bigint AS paid_amount,
        0::bigint AS payment_fee,
        0::bigint AS provider_payment_fee,
        0::bigint AS merchant_payment_fee,
        0::bigint AS rider_payment_fee,
        0::bigint AS platform_payment_fee_income,
        0::bigint AS platform_net_payment_fee_margin,
        0::bigint AS merchant_amount,
        0::bigint AS rider_amount,
        0::bigint AS platform_commission,
        0::bigint AS operator_commission,
        0::bigint AS withdraw_succeeded_amount,
        0::bigint AS withdraw_processing_amount,
        0::bigint AS unapplied_fact_count,
        0::bigint AS unknown_command_count,
        COUNT(*)::bigint AS fee_ledger_mismatch_count
    FROM profit_sharing_orders pso
    LEFT JOIN baofu_fee_ledger bfl
      ON bfl.fee_type = 'payment_fee'
     AND bfl.business_object_type = 'payment_order'
     AND bfl.business_object_id = pso.payment_order_id
    WHERE pso.provider = 'baofu'
      AND pso.channel = 'baofu_aggregate'
      AND COALESCE(pso.finished_at, pso.created_at) >= sqlc.arg(start_at)
      AND COALESCE(pso.finished_at, pso.created_at) <= sqlc.arg(end_at)
      AND (bfl.id IS NULL OR bfl.amount <> pso.payment_fee)
    GROUP BY COALESCE(pso.finished_at, pso.created_at)::date, pso.provider, pso.channel
),
baofu_reconciliation AS (
    SELECT
        date,
        provider,
        channel,
        COALESCE(SUM(paid_amount), 0)::bigint AS paid_amount,
        COALESCE(SUM(payment_fee), 0)::bigint AS payment_fee,
        COALESCE(SUM(provider_payment_fee), 0)::bigint AS provider_payment_fee,
        COALESCE(SUM(merchant_payment_fee), 0)::bigint AS merchant_payment_fee,
        COALESCE(SUM(rider_payment_fee), 0)::bigint AS rider_payment_fee,
        COALESCE(SUM(platform_payment_fee_income), 0)::bigint AS platform_payment_fee_income,
        COALESCE(SUM(platform_net_payment_fee_margin), 0)::bigint AS platform_net_payment_fee_margin,
        COALESCE(SUM(merchant_amount), 0)::bigint AS merchant_amount,
        COALESCE(SUM(rider_amount), 0)::bigint AS rider_amount,
        COALESCE(SUM(platform_commission), 0)::bigint AS platform_commission,
        COALESCE(SUM(operator_commission), 0)::bigint AS operator_commission,
        COALESCE(SUM(withdraw_succeeded_amount), 0)::bigint AS withdraw_succeeded_amount,
        COALESCE(SUM(withdraw_processing_amount), 0)::bigint AS withdraw_processing_amount,
        COALESCE(SUM(unapplied_fact_count), 0)::bigint AS unapplied_fact_count,
        COALESCE(SUM(unknown_command_count), 0)::bigint AS unknown_command_count,
        COALESCE(SUM(fee_ledger_mismatch_count), 0)::bigint AS fee_ledger_mismatch_count
    FROM baofu_reconciliation_parts
    WHERE date IS NOT NULL
    GROUP BY date, provider, channel
),
historical_failed_refund_reconciliation AS (
    SELECT
        epc.submitted_at::date AS date,
        epc.provider,
        epc.channel,
        COUNT(*) FILTER (
            WHERE upper(trim(epc.last_error_code)) IN ('REQUEST_FAILED', 'SYSTEM_BUSY', 'SYSTEM_ERROR', 'SYSTEM_INNER_ERROR', 'TIMEOUT', 'BF0005', 'BF0002', '2')
        )::bigint AS historical_retryable_failed_refund_count,
        COALESCE(SUM(ro.refund_amount) FILTER (
            WHERE upper(trim(epc.last_error_code)) IN ('REQUEST_FAILED', 'SYSTEM_BUSY', 'SYSTEM_ERROR', 'SYSTEM_INNER_ERROR', 'TIMEOUT', 'BF0005', 'BF0002', '2')
        ), 0)::bigint AS historical_retryable_failed_refund_amount,
        COUNT(*) FILTER (
            WHERE upper(trim(epc.last_error_code)) IN ('ORDER_EXIST', 'REPEATED_REQUEST', 'BF0013', 'TRADE_UNCONFIRMED', 'PROCESSING', 'ABNORMAL')
        )::bigint AS historical_queryable_failed_refund_count,
        COALESCE(SUM(ro.refund_amount) FILTER (
            WHERE upper(trim(epc.last_error_code)) IN ('ORDER_EXIST', 'REPEATED_REQUEST', 'BF0013', 'TRADE_UNCONFIRMED', 'PROCESSING', 'ABNORMAL')
        ), 0)::bigint AS historical_queryable_failed_refund_amount
    FROM refund_orders ro
    JOIN payment_orders po ON po.id = ro.payment_order_id
    JOIN external_payment_commands epc
      ON epc.provider = 'baofu'
     AND epc.channel = 'baofu_aggregate'
     AND epc.capability = 'baofu_refund'
     AND epc.command_type = 'create_refund'
     AND epc.business_object_type = 'refund_order'
     AND epc.business_object_id = ro.id
     AND epc.external_object_type = 'refund'
     AND epc.external_object_key = ro.out_refund_no
    WHERE ro.status = 'failed'
      AND po.payment_channel = 'baofu_aggregate'
      AND po.business_type = 'order'
      AND po.order_id IS NOT NULL
      AND epc.command_status IN ('rejected', 'unknown')
      AND epc.submitted_at >= sqlc.arg(start_at)
      AND epc.submitted_at <= sqlc.arg(end_at)
      AND upper(trim(epc.last_error_code)) IN (
          'REQUEST_FAILED', 'SYSTEM_BUSY', 'SYSTEM_ERROR', 'SYSTEM_INNER_ERROR', 'TIMEOUT', 'BF0005', 'BF0002', '2',
          'ORDER_EXIST', 'REPEATED_REQUEST', 'BF0013', 'TRADE_UNCONFIRMED', 'PROCESSING', 'ABNORMAL'
      )
    GROUP BY epc.submitted_at::date, epc.provider, epc.channel
)
SELECT
    COALESCE(br.date, hfr.date) AS date,
    COALESCE(br.provider, hfr.provider) AS provider,
    COALESCE(br.channel, hfr.channel) AS channel,
    COALESCE(br.paid_amount, 0)::bigint AS paid_amount,
    COALESCE(br.payment_fee, 0)::bigint AS payment_fee,
    COALESCE(br.provider_payment_fee, 0)::bigint AS provider_payment_fee,
    COALESCE(br.merchant_payment_fee, 0)::bigint AS merchant_payment_fee,
    COALESCE(br.rider_payment_fee, 0)::bigint AS rider_payment_fee,
    COALESCE(br.platform_payment_fee_income, 0)::bigint AS platform_payment_fee_income,
    COALESCE(br.platform_net_payment_fee_margin, 0)::bigint AS platform_net_payment_fee_margin,
    COALESCE(br.merchant_amount, 0)::bigint AS merchant_amount,
    COALESCE(br.rider_amount, 0)::bigint AS rider_amount,
    COALESCE(br.platform_commission, 0)::bigint AS platform_commission,
    COALESCE(br.operator_commission, 0)::bigint AS operator_commission,
    COALESCE(br.withdraw_succeeded_amount, 0)::bigint AS withdraw_succeeded_amount,
    COALESCE(br.withdraw_processing_amount, 0)::bigint AS withdraw_processing_amount,
    COALESCE(br.unapplied_fact_count, 0)::bigint AS unapplied_fact_count,
    COALESCE(br.unknown_command_count, 0)::bigint AS unknown_command_count,
    COALESCE(br.fee_ledger_mismatch_count, 0)::bigint AS fee_ledger_mismatch_count,
    COALESCE(hfr.historical_retryable_failed_refund_count, 0)::bigint AS historical_retryable_failed_refund_count,
    COALESCE(hfr.historical_retryable_failed_refund_amount, 0)::bigint AS historical_retryable_failed_refund_amount,
    COALESCE(hfr.historical_queryable_failed_refund_count, 0)::bigint AS historical_queryable_failed_refund_count,
    COALESCE(hfr.historical_queryable_failed_refund_amount, 0)::bigint AS historical_queryable_failed_refund_amount
FROM baofu_reconciliation br
FULL JOIN historical_failed_refund_reconciliation hfr
  ON hfr.date = br.date
 AND hfr.provider = br.provider
 AND hfr.channel = br.channel
ORDER BY date DESC, provider ASC, channel ASC;

-- name: ListBaofuWithdrawalReservationDrifts :many
WITH reserved_reservation_totals AS (
    SELECT
        owner_type,
        owner_id,
        account_binding_id,
        COUNT(*)::bigint AS reserved_reservation_count,
        COALESCE(SUM(amount_fen), 0)::bigint AS reserved_reservation_amount_fen,
        MIN(id)::bigint AS sample_reservation_id,
        MAX(updated_at) AS latest_reserved_at
    FROM baofu_withdrawal_reservations
    WHERE status = 'reserved'
    GROUP BY owner_type, owner_id, account_binding_id
),
drifts AS (
    SELECT
        'processing_missing_reserved_reservation'::text AS drift_type,
        bwo.owner_type,
        bwo.owner_id,
        bwo.account_binding_id,
        bwo.id::bigint AS withdrawal_order_id,
        COALESCE(bwr.id, 0)::bigint AS reservation_id,
        COALESCE(bwag.id, 0)::bigint AS guard_id,
        bwo.status::text AS withdrawal_status,
        COALESCE(bwr.status, '')::text AS reservation_status,
        COALESCE(bwag.reserved_amount_fen, 0)::bigint AS guard_reserved_amount_fen,
        0::bigint AS reserved_reservation_amount_fen,
        bwo.amount::bigint AS amount_delta_fen,
        bwo.updated_at AS observed_at
    FROM baofu_withdrawal_orders bwo
    LEFT JOIN baofu_withdrawal_reservations bwr
      ON bwr.withdrawal_order_id = bwo.id
    LEFT JOIN baofu_withdrawal_account_guards bwag
      ON bwag.owner_type = bwo.owner_type
     AND bwag.owner_id = bwo.owner_id
     AND bwag.account_binding_id = bwo.account_binding_id
    WHERE bwo.status = 'processing'
      AND (bwr.id IS NULL OR bwr.status <> 'reserved')

    UNION ALL

    SELECT
        'terminal_reserved_reservation'::text AS drift_type,
        bwo.owner_type,
        bwo.owner_id,
        bwo.account_binding_id,
        bwo.id::bigint AS withdrawal_order_id,
        bwr.id::bigint AS reservation_id,
        COALESCE(bwag.id, 0)::bigint AS guard_id,
        bwo.status::text AS withdrawal_status,
        bwr.status::text AS reservation_status,
        COALESCE(bwag.reserved_amount_fen, 0)::bigint AS guard_reserved_amount_fen,
        bwr.amount_fen::bigint AS reserved_reservation_amount_fen,
        bwr.amount_fen::bigint AS amount_delta_fen,
        GREATEST(bwo.updated_at, bwr.updated_at) AS observed_at
    FROM baofu_withdrawal_orders bwo
    JOIN baofu_withdrawal_reservations bwr
      ON bwr.withdrawal_order_id = bwo.id
     AND bwr.status = 'reserved'
    LEFT JOIN baofu_withdrawal_account_guards bwag
      ON bwag.owner_type = bwo.owner_type
     AND bwag.owner_id = bwo.owner_id
     AND bwag.account_binding_id = bwo.account_binding_id
    WHERE bwo.status IN ('succeeded', 'failed', 'returned')

    UNION ALL

    SELECT
        'guard_reserved_mismatch'::text AS drift_type,
        COALESCE(bwag.owner_type, rrt.owner_type)::text AS owner_type,
        COALESCE(bwag.owner_id, rrt.owner_id)::bigint AS owner_id,
        COALESCE(bwag.account_binding_id, rrt.account_binding_id)::bigint AS account_binding_id,
        0::bigint AS withdrawal_order_id,
        COALESCE(rrt.sample_reservation_id, 0)::bigint AS reservation_id,
        COALESCE(bwag.id, 0)::bigint AS guard_id,
        ''::text AS withdrawal_status,
        ''::text AS reservation_status,
        COALESCE(bwag.reserved_amount_fen, 0)::bigint AS guard_reserved_amount_fen,
        COALESCE(rrt.reserved_reservation_amount_fen, 0)::bigint AS reserved_reservation_amount_fen,
        (COALESCE(bwag.reserved_amount_fen, 0) - COALESCE(rrt.reserved_reservation_amount_fen, 0))::bigint AS amount_delta_fen,
        COALESCE(bwag.updated_at, rrt.latest_reserved_at) AS observed_at
    FROM baofu_withdrawal_account_guards bwag
    FULL OUTER JOIN reserved_reservation_totals rrt
      ON rrt.owner_type = bwag.owner_type
     AND rrt.owner_id = bwag.owner_id
     AND rrt.account_binding_id = bwag.account_binding_id
    WHERE COALESCE(bwag.reserved_amount_fen, 0) <> COALESCE(rrt.reserved_reservation_amount_fen, 0)
)
SELECT
    drift_type,
    owner_type,
    owner_id,
    account_binding_id,
    withdrawal_order_id,
    reservation_id,
    guard_id,
    withdrawal_status,
    reservation_status,
    guard_reserved_amount_fen,
    reserved_reservation_amount_fen,
    amount_delta_fen,
    observed_at
FROM drifts
WHERE (sqlc.narg('owner_type')::text IS NULL OR owner_type = sqlc.narg('owner_type')::text)
  AND (sqlc.narg('owner_id')::bigint IS NULL OR owner_id = sqlc.narg('owner_id')::bigint)
ORDER BY observed_at DESC, drift_type ASC, owner_type ASC, owner_id ASC, account_binding_id ASC, withdrawal_order_id NULLS LAST, reservation_id NULLS LAST, guard_id NULLS LAST
LIMIT sqlc.arg(limit_count)::int;
