-- name: CreateOrderPaymentFeeLedger :one
INSERT INTO order_payment_fee_ledgers (
    provider,
    channel,
    payment_order_id,
    profit_sharing_order_id,
    fee_type,
    payer_type,
    payer_id,
    payee_type,
    base_amount,
    rate_bps,
    amount,
    amount_source,
    external_payment_fact_id,
    status,
    calculation_version
) VALUES (
    sqlc.arg(provider),
    sqlc.arg(channel),
    sqlc.arg(payment_order_id),
    sqlc.narg(profit_sharing_order_id),
    sqlc.arg(fee_type),
    sqlc.arg(payer_type),
    sqlc.narg(payer_id),
    sqlc.arg(payee_type),
    sqlc.arg(base_amount),
    sqlc.arg(rate_bps),
    sqlc.arg(amount),
    sqlc.arg(amount_source),
    sqlc.narg(external_payment_fact_id),
    sqlc.arg(status),
    sqlc.arg(calculation_version)
) RETURNING id, provider, channel, payment_order_id, profit_sharing_order_id, fee_type, payer_type, payer_id, payee_type, base_amount, rate_bps, amount, amount_source, external_payment_fact_id, status, calculation_version, created_at, updated_at;

-- name: UpsertOrderPaymentFeeLedgerActual :one
INSERT INTO order_payment_fee_ledgers (
    provider,
    channel,
    payment_order_id,
    profit_sharing_order_id,
    fee_type,
    payer_type,
    payer_id,
    payee_type,
    base_amount,
    rate_bps,
    amount,
    amount_source,
    external_payment_fact_id,
    status,
    calculation_version
) VALUES (
    sqlc.arg(provider),
    sqlc.arg(channel),
    sqlc.arg(payment_order_id),
    sqlc.narg(profit_sharing_order_id),
    sqlc.arg(fee_type),
    sqlc.arg(payer_type),
    sqlc.narg(payer_id),
    sqlc.arg(payee_type),
    sqlc.arg(base_amount),
    sqlc.arg(rate_bps),
    sqlc.arg(amount),
    sqlc.arg(amount_source),
    sqlc.narg(external_payment_fact_id),
    sqlc.arg(status),
    sqlc.arg(calculation_version)
) ON CONFLICT (payment_order_id, fee_type, payer_type, COALESCE(payer_id, 0))
DO UPDATE SET
    profit_sharing_order_id = COALESCE(EXCLUDED.profit_sharing_order_id, order_payment_fee_ledgers.profit_sharing_order_id),
    base_amount = EXCLUDED.base_amount,
    rate_bps = EXCLUDED.rate_bps,
    amount = EXCLUDED.amount,
    amount_source = EXCLUDED.amount_source,
    external_payment_fact_id = COALESCE(EXCLUDED.external_payment_fact_id, order_payment_fee_ledgers.external_payment_fact_id),
    status = EXCLUDED.status,
    calculation_version = EXCLUDED.calculation_version,
    updated_at = now()
RETURNING id, provider, channel, payment_order_id, profit_sharing_order_id, fee_type, payer_type, payer_id, payee_type, base_amount, rate_bps, amount, amount_source, external_payment_fact_id, status, calculation_version, created_at, updated_at;

-- name: UpsertOrderPaymentFeeLedgerCalculated :one
INSERT INTO order_payment_fee_ledgers (
    provider,
    channel,
    payment_order_id,
    profit_sharing_order_id,
    fee_type,
    payer_type,
    payer_id,
    payee_type,
    base_amount,
    rate_bps,
    amount,
    amount_source,
    external_payment_fact_id,
    status,
    calculation_version
) VALUES (
    sqlc.arg(provider),
    sqlc.arg(channel),
    sqlc.arg(payment_order_id),
    sqlc.narg(profit_sharing_order_id),
    sqlc.arg(fee_type),
    sqlc.arg(payer_type),
    sqlc.narg(payer_id),
    sqlc.arg(payee_type),
    sqlc.arg(base_amount),
    sqlc.arg(rate_bps),
    sqlc.arg(amount),
    sqlc.arg(amount_source),
    sqlc.narg(external_payment_fact_id),
    sqlc.arg(status),
    sqlc.arg(calculation_version)
) ON CONFLICT (payment_order_id, fee_type, payer_type, COALESCE(payer_id, 0))
DO UPDATE SET
    profit_sharing_order_id = COALESCE(EXCLUDED.profit_sharing_order_id, order_payment_fee_ledgers.profit_sharing_order_id),
    base_amount = CASE
        WHEN order_payment_fee_ledgers.amount_source IN ('actual_callback', 'actual_query') THEN order_payment_fee_ledgers.base_amount
        ELSE EXCLUDED.base_amount
    END,
    rate_bps = CASE
        WHEN order_payment_fee_ledgers.amount_source IN ('actual_callback', 'actual_query') THEN order_payment_fee_ledgers.rate_bps
        ELSE EXCLUDED.rate_bps
    END,
    amount = CASE
        WHEN order_payment_fee_ledgers.amount_source IN ('actual_callback', 'actual_query') THEN order_payment_fee_ledgers.amount
        ELSE EXCLUDED.amount
    END,
    amount_source = CASE
        WHEN order_payment_fee_ledgers.amount_source IN ('actual_callback', 'actual_query') THEN order_payment_fee_ledgers.amount_source
        ELSE EXCLUDED.amount_source
    END,
    external_payment_fact_id = COALESCE(order_payment_fee_ledgers.external_payment_fact_id, EXCLUDED.external_payment_fact_id),
    status = CASE
        WHEN order_payment_fee_ledgers.amount_source IN ('actual_callback', 'actual_query') THEN order_payment_fee_ledgers.status
        ELSE EXCLUDED.status
    END,
    calculation_version = EXCLUDED.calculation_version,
    updated_at = now()
RETURNING id, provider, channel, payment_order_id, profit_sharing_order_id, fee_type, payer_type, payer_id, payee_type, base_amount, rate_bps, amount, amount_source, external_payment_fact_id, status, calculation_version, created_at, updated_at;

-- name: ListOrderPaymentFeeLedgersByPayer :many
SELECT id, provider, channel, payment_order_id, profit_sharing_order_id, fee_type, payer_type, payer_id, payee_type, base_amount, rate_bps, amount, amount_source, external_payment_fact_id, status, calculation_version, created_at, updated_at
FROM order_payment_fee_ledgers
WHERE payer_type = sqlc.arg(payer_type)
  AND (sqlc.narg(payer_id)::bigint IS NULL OR payer_id = sqlc.narg(payer_id))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(limit_count) OFFSET sqlc.arg(offset_count);
