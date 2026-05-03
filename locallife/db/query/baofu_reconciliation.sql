-- name: GetBaofuDailyReconciliation :many
WITH baofu_reconciliation_parts AS (
    SELECT
        po.paid_at::date AS date,
        'baofu'::text AS provider,
        po.payment_channel AS channel,
        COALESCE(SUM(po.amount), 0)::bigint AS paid_amount,
        0::bigint AS payment_fee,
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
        COALESCE(SUM(pso.payment_fee), 0)::bigint AS payment_fee,
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
)
SELECT
    date,
    provider,
    channel,
    COALESCE(SUM(paid_amount), 0)::bigint AS paid_amount,
    COALESCE(SUM(payment_fee), 0)::bigint AS payment_fee,
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
ORDER BY date DESC, provider ASC, channel ASC;
