-- name: UpsertWechatMerchantViolation :one
INSERT INTO wechat_merchant_violations (
    record_id,
    sub_mch_id,
    merchant_id,
    company_name,
    event_type,
    risk_type,
    risk_description,
    punish_plan,
    punish_time,
    punish_description,
    latest_notification_id,
    latest_notify_time,
    last_received_at
) VALUES (
    sqlc.arg('record_id'),
    sqlc.arg('sub_mch_id'),
    sqlc.narg('merchant_id'),
    sqlc.arg('company_name'),
    sqlc.arg('event_type'),
    sqlc.arg('risk_type'),
    sqlc.arg('risk_description'),
    sqlc.arg('punish_plan'),
    sqlc.narg('punish_time'),
    sqlc.arg('punish_description'),
    sqlc.arg('latest_notification_id'),
    sqlc.arg('latest_notify_time'),
    now()
)
ON CONFLICT (record_id) DO UPDATE SET
    sub_mch_id = COALESCE(NULLIF(EXCLUDED.sub_mch_id, ''), wechat_merchant_violations.sub_mch_id),
    merchant_id = COALESCE(EXCLUDED.merchant_id, wechat_merchant_violations.merchant_id),
    company_name = COALESCE(NULLIF(EXCLUDED.company_name, ''), wechat_merchant_violations.company_name),
    event_type = EXCLUDED.event_type,
    risk_type = COALESCE(NULLIF(EXCLUDED.risk_type, ''), wechat_merchant_violations.risk_type),
    risk_description = COALESCE(NULLIF(EXCLUDED.risk_description, ''), wechat_merchant_violations.risk_description),
    punish_plan = COALESCE(NULLIF(EXCLUDED.punish_plan, ''), wechat_merchant_violations.punish_plan),
    punish_time = COALESCE(EXCLUDED.punish_time, wechat_merchant_violations.punish_time),
    punish_description = COALESCE(NULLIF(EXCLUDED.punish_description, ''), wechat_merchant_violations.punish_description),
    latest_notification_id = EXCLUDED.latest_notification_id,
    latest_notify_time = EXCLUDED.latest_notify_time,
    last_received_at = now(),
    updated_at = now()
RETURNING
    id,
    record_id,
    sub_mch_id,
    merchant_id,
    company_name,
    event_type,
    risk_type,
    risk_description,
    punish_plan,
    punish_time,
    punish_description,
    latest_notification_id,
    latest_notify_time,
    last_received_at,
    created_at,
    updated_at;

-- name: GetWechatMerchantViolationByRecordID :one
SELECT
    id,
    record_id,
    sub_mch_id,
    merchant_id,
    company_name,
    event_type,
    risk_type,
    risk_description,
    punish_plan,
    punish_time,
    punish_description,
    latest_notification_id,
    latest_notify_time,
    last_received_at,
    created_at,
    updated_at
FROM wechat_merchant_violations
WHERE record_id = sqlc.arg('record_id')
LIMIT 1;

-- name: ListWechatMerchantViolations :many
SELECT
    id,
    record_id,
    sub_mch_id,
    merchant_id,
    company_name,
    event_type,
    risk_type,
    risk_description,
    punish_plan,
    punish_time,
    punish_description,
    latest_notification_id,
    latest_notify_time,
    last_received_at,
    created_at,
    updated_at
FROM wechat_merchant_violations
WHERE (sqlc.narg('merchant_id')::bigint IS NULL OR merchant_id = sqlc.narg('merchant_id'))
  AND (sqlc.narg('sub_mch_id')::text IS NULL OR sub_mch_id = sqlc.narg('sub_mch_id'))
  AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type'))
  AND (sqlc.narg('risk_type')::text IS NULL OR risk_type = sqlc.narg('risk_type'))
ORDER BY COALESCE(punish_time, latest_notify_time) DESC, id DESC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: CountWechatMerchantViolations :one
SELECT COUNT(*)
FROM wechat_merchant_violations
WHERE (sqlc.narg('merchant_id')::bigint IS NULL OR merchant_id = sqlc.narg('merchant_id'))
  AND (sqlc.narg('sub_mch_id')::text IS NULL OR sub_mch_id = sqlc.narg('sub_mch_id'))
  AND (sqlc.narg('event_type')::text IS NULL OR event_type = sqlc.narg('event_type'))
  AND (sqlc.narg('risk_type')::text IS NULL OR risk_type = sqlc.narg('risk_type'));