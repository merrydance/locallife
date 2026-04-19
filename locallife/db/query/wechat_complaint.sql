-- name: UpsertWechatComplaint :one
-- 插入或更新投诉记录（幂等，微信每日同步时使用 ON CONFLICT 更新）
INSERT INTO wechat_complaints (
    complaint_id,
    complaint_time,
    payer_openid,
    complaint_detail,
    complaint_state,
    transaction_id,
    out_trade_no,
    sub_mch_id,
    merchant_id,
    payer_complaint_full_info,
    amount,
    wxpay_update_time,
    last_synced_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now()
)
ON CONFLICT (complaint_id) DO UPDATE SET
    complaint_state         = EXCLUDED.complaint_state,
    complaint_detail        = EXCLUDED.complaint_detail,
    payer_complaint_full_info = EXCLUDED.payer_complaint_full_info,
    amount                  = EXCLUDED.amount,
    wxpay_update_time       = EXCLUDED.wxpay_update_time,
    last_synced_at          = now(),
    updated_at              = now()
RETURNING *;

-- name: GetWechatComplaint :one
SELECT id, complaint_id, complaint_time, payer_openid, complaint_detail, complaint_state, transaction_id, out_trade_no, sub_mch_id, merchant_id, payer_complaint_full_info, amount, response_content, responded_at, media_ids, completed_at, last_synced_at, wxpay_update_time, created_at, updated_at FROM wechat_complaints
WHERE id = $1 LIMIT 1;

-- name: GetWechatComplaintByComplaintID :one
SELECT id, complaint_id, complaint_time, payer_openid, complaint_detail, complaint_state, transaction_id, out_trade_no, sub_mch_id, merchant_id, payer_complaint_full_info, amount, response_content, responded_at, media_ids, completed_at, last_synced_at, wxpay_update_time, created_at, updated_at FROM wechat_complaints
WHERE complaint_id = $1 LIMIT 1;

-- name: GetWechatComplaintByComplaintIDForUpdate :one
SELECT id, complaint_id, complaint_time, payer_openid, complaint_detail, complaint_state, transaction_id, out_trade_no, sub_mch_id, merchant_id, payer_complaint_full_info, amount, response_content, responded_at, media_ids, completed_at, last_synced_at, wxpay_update_time, created_at, updated_at FROM wechat_complaints
WHERE complaint_id = $1
LIMIT 1
FOR UPDATE;

-- name: ListWechatComplaintsByMerchant :many
-- 商户侧查看自己收到的投诉（按投诉时间倒序）
SELECT id, complaint_id, complaint_time, payer_openid, complaint_detail, complaint_state, transaction_id, out_trade_no, sub_mch_id, merchant_id, payer_complaint_full_info, amount, response_content, responded_at, media_ids, completed_at, last_synced_at, wxpay_update_time, created_at, updated_at FROM wechat_complaints
WHERE merchant_id = $1
    AND (NULLIF($2::text, '') IS NULL OR complaint_state = $2)
ORDER BY complaint_time DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: ListWechatComplaintsBySubMchID :many
-- 通过 sub_mch_id 查询（运营商/平台使用）
SELECT id, complaint_id, complaint_time, payer_openid, complaint_detail, complaint_state, transaction_id, out_trade_no, sub_mch_id, merchant_id, payer_complaint_full_info, amount, response_content, responded_at, media_ids, completed_at, last_synced_at, wxpay_update_time, created_at, updated_at FROM wechat_complaints
WHERE sub_mch_id = $1
    AND (NULLIF($2::text, '') IS NULL OR complaint_state = $2)
ORDER BY complaint_time DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: ListPendingWechatComplaints :many
-- 运营商查看所有待处理投诉（按投诉时间倒序）
SELECT id, complaint_id, complaint_time, payer_openid, complaint_detail, complaint_state, transaction_id, out_trade_no, sub_mch_id, merchant_id, payer_complaint_full_info, amount, response_content, responded_at, media_ids, completed_at, last_synced_at, wxpay_update_time, created_at, updated_at FROM wechat_complaints
WHERE complaint_state IN ('PENDING_RESPONSE', 'PROCESSING')
ORDER BY complaint_time ASC, id ASC
LIMIT $1 OFFSET $2;

-- name: UpdateWechatComplaintResponse :one
-- 记录我方回复内容
UPDATE wechat_complaints
SET
    response_content = $2,
    responded_at     = now(),
    complaint_state  = 'PROCESSING',
    updated_at       = now()
WHERE id = $1
RETURNING *;

-- name: UpdateWechatComplaintCompleted :one
-- 标记投诉已完结
UPDATE wechat_complaints
SET
    complaint_state = 'PROCESSED',
    completed_at    = now(),
    updated_at      = now()
WHERE id = $1
RETURNING *;

-- name: UpdateWechatComplaintState :one
-- 同步状态变更（由微信通知或轮询驱动）
UPDATE wechat_complaints
SET
    complaint_state   = $2,
    wxpay_update_time = $3,
    last_synced_at    = now(),
    updated_at        = now()
WHERE complaint_id = $1
RETURNING *;

-- name: CountWechatComplaintsByMerchant :one
SELECT COUNT(*) FROM wechat_complaints
WHERE merchant_id = $1
    AND (NULLIF($2::text, '') IS NULL OR complaint_state = $2);
