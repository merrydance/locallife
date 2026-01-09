-- ==================== 商户入驻申请 ====================

-- name: CreateMerchantApplication :one
INSERT INTO merchant_applications (
  user_id,
  merchant_name,
  business_license_number,
  business_license_image_url,
  legal_person_name,
  legal_person_id_number,
  legal_person_id_front_url,
  legal_person_id_back_url,
  contact_phone,
  business_address,
  business_scope,
  longitude,
  latitude,
  region_id
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;

-- name: GetMerchantApplication :one
SELECT * FROM merchant_applications
WHERE id = $1 LIMIT 1;

-- name: GetUserMerchantApplication :one
SELECT * FROM merchant_applications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: ListMerchantApplications :many
SELECT * FROM merchant_applications
WHERE status = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListAllMerchantApplications :many
SELECT * FROM merchant_applications
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateMerchantApplicationStatus :one
UPDATE merchant_applications
SET
  status = $2,
  reject_reason = $3,
  reviewed_by = $4,
  reviewed_at = $5,
  updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetMerchantApplicationByLicenseNumber :one
SELECT * FROM merchant_applications
WHERE business_license_number = $1
LIMIT 1;

-- ==================== 商户管理 ====================

-- name: CreateMerchant :one
INSERT INTO merchants (
  owner_user_id,
  name,
  description,
  logo_url,
  phone,
  address,
  latitude,
  longitude,
  status,
  application_data,
  region_id
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: GetMerchant :one
SELECT * FROM merchants
WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetMerchantByOwner :one
-- 获取用户关联的商户（支持店主和员工）
-- 优先返回 owner_user_id 匹配的商户，其次返回 merchant_staff 关联的商户
SELECT m.* FROM merchants m
LEFT JOIN merchant_staff ms ON m.id = ms.merchant_id AND ms.status = 'active'
WHERE (m.owner_user_id = $1 OR ms.user_id = $1) AND m.deleted_at IS NULL
ORDER BY CASE WHEN m.owner_user_id = $1 THEN 0 ELSE 1 END
LIMIT 1;

-- name: ListMerchantsByOwner :many
-- 获取用户拥有的所有商户（用于多店铺切换）
SELECT * FROM merchants
WHERE owner_user_id = $1 AND deleted_at IS NULL
ORDER BY created_at ASC;

-- name: GetMerchantByBindCode :one
-- 通过邀请码获取商户
SELECT * FROM merchants
WHERE bind_code = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: ListMerchants :many
SELECT * FROM merchants
WHERE status = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListAllMerchants :many
SELECT * FROM merchants
WHERE deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateMerchant :one
-- ✅ P1-2: 使用乐观锁(version)防止并发更新丢失
UPDATE merchants
SET
  name = COALESCE(sqlc.narg('name'), name),
  description = COALESCE(sqlc.narg('description'), description),
  logo_url = COALESCE(sqlc.narg('logo_url'), logo_url),
  phone = COALESCE(sqlc.narg('phone'), phone),
  address = COALESCE(sqlc.narg('address'), address),
  latitude = COALESCE(sqlc.narg('latitude'), latitude),
  longitude = COALESCE(sqlc.narg('longitude'), longitude),
  region_id = COALESCE(sqlc.narg('region_id'), region_id),
  version = version + 1,
  updated_at = now()
WHERE id = sqlc.arg('id')
  AND version = sqlc.arg('version')
  AND deleted_at IS NULL
RETURNING *;

-- name: UpdateMerchantStatus :one
UPDATE merchants
SET
  status = $2,
  updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: UpdateMerchantBindCode :one
-- 更新商户邀请码
UPDATE merchants
SET
  bind_code = $2,
  bind_code_expires_at = $3,
  updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: DeleteMerchant :exec
-- 软删除商户
UPDATE merchants SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL;

-- name: SearchMerchants :many
SELECT * FROM merchants
WHERE status = 'active'
  AND deleted_at IS NULL
  AND name ILIKE '%' || $1 || '%'
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountSearchMerchants :one
SELECT COUNT(*) FROM merchants
WHERE status = 'active'
  AND deleted_at IS NULL
  AND name ILIKE '%' || $1 || '%';

-- ==================== 商户标签关联 ====================

-- name: AddMerchantTag :exec
INSERT INTO merchant_tags (
  merchant_id,
  tag_id
) VALUES (
  $1, $2
);

-- name: RemoveMerchantTag :exec
DELETE FROM merchant_tags
WHERE merchant_id = $1 AND tag_id = $2;

-- name: ListMerchantTags :many
SELECT t.* FROM tags t
INNER JOIN merchant_tags mt ON t.id = mt.tag_id
WHERE mt.merchant_id = $1
ORDER BY t.name;

-- name: ListMerchantsByTag :many
SELECT m.* FROM merchants m
INNER JOIN merchant_tags mt ON m.id = mt.merchant_id
WHERE mt.tag_id = $1
  AND m.status = 'active'
ORDER BY m.created_at DESC
LIMIT $2 OFFSET $3;

-- name: ClearMerchantTags :exec
DELETE FROM merchant_tags
WHERE merchant_id = $1;

-- ==================== 商户营业时间 ====================

-- name: CreateBusinessHour :one
INSERT INTO merchant_business_hours (
  merchant_id,
  day_of_week,
  open_time,
  close_time,
  is_closed,
  special_date
) VALUES (
  $1, $2, $3, $4, $5, $6
)
RETURNING *;

-- name: GetBusinessHour :one
SELECT * FROM merchant_business_hours
WHERE id = $1 LIMIT 1;

-- name: ListMerchantBusinessHours :many
SELECT * FROM merchant_business_hours
WHERE merchant_id = $1
  AND special_date IS NULL
ORDER BY day_of_week;

-- name: ListMerchantSpecialHours :many
SELECT * FROM merchant_business_hours
WHERE merchant_id = $1
  AND special_date IS NOT NULL
ORDER BY special_date;

-- name: GetBusinessHourByDate :one
SELECT * FROM merchant_business_hours
WHERE merchant_id = $1
  AND special_date = $2
LIMIT 1;

-- name: GetBusinessHourByDayOfWeek :one
SELECT * FROM merchant_business_hours
WHERE merchant_id = $1
  AND day_of_week = $2
  AND special_date IS NULL
LIMIT 1;

-- name: UpdateBusinessHour :one
UPDATE merchant_business_hours
SET
  open_time = $2,
  close_time = $3,
  is_closed = $4,
  updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteBusinessHour :exec
DELETE FROM merchant_business_hours
WHERE id = $1;

-- name: DeleteMerchantBusinessHours :exec
DELETE FROM merchant_business_hours
WHERE merchant_id = $1;

-- ==================== 高级查询（使用JOIN和聚合）====================

-- name: GetMerchantWithTags :one
SELECT 
  sqlc.embed(merchants),
  COALESCE(
    (
      SELECT json_agg(tags.*)
      FROM tags
      INNER JOIN merchant_tags ON tags.id = merchant_tags.tag_id
      WHERE merchant_tags.merchant_id = merchants.id
    ),
    '[]'::json
  )::json AS tags
FROM merchants
WHERE merchants.id = $1
LIMIT 1;

-- name: ListMerchantsWithTagCount :many
SELECT 
  m.*,
  COUNT(mt.tag_id) as tag_count
FROM merchants m
LEFT JOIN merchant_tags mt ON m.id = mt.merchant_id
WHERE m.status = $1
GROUP BY m.id
ORDER BY m.created_at DESC
LIMIT $2 OFFSET $3;

-- ============================================
-- 商户推荐查询
-- ============================================

-- name: GetPopularMerchants :many
-- 获取热门商户（基于整体销量和评分）
-- 销量 = 外卖 + 堂食 + 预定，高销量意味着回头客多，是最重要的排序因子
SELECT 
    m.id,
    m.name,
    m.description,
    m.logo_url,
    m.address,
    m.latitude,
    m.longitude,
    COALESCE(COUNT(o.id), 0)::int AS total_orders,
  0::numeric(3,2) AS avg_rating
FROM merchants m
LEFT JOIN orders o ON m.id = o.merchant_id 
  AND o.status IN ('completed')  -- 以已完成订单作为销量口径
WHERE m.status = 'active'
GROUP BY m.id
ORDER BY 
    total_orders DESC,  -- 销量优先：回头客多的商户排前面
    avg_rating DESC     -- 评分次之
LIMIT $1;

-- name: GetMerchantsByIDs :many
-- 批量获取商户详情
SELECT 
    id,
    name,
    description,
    logo_url,
    address,
    latitude,
    longitude,
    status
FROM merchants
WHERE id = ANY($1::bigint[])
  AND status = 'active';

-- name: GetMerchantsWithStatsByIDs :many
-- 批量获取商户详情及统计数据（用于推荐流展示）
SELECT 
    m.id,
    m.name,
    m.description,
    m.logo_url,
    m.address,
    m.latitude,
    m.longitude,
    m.region_id,
    m.status,
    m.is_open,
    COALESCE(mp.trust_score, 500) AS trust_score,
    COALESCE(
        (SELECT COUNT(*)
         FROM orders o 
         WHERE o.merchant_id = m.id 
           AND o.status IN ('completed')
           AND o.created_at >= NOW() - INTERVAL '30 days'
        ), 0
    )::int AS monthly_orders
FROM merchants m
LEFT JOIN merchant_profiles mp ON mp.merchant_id = m.id
WHERE m.id = ANY($1::bigint[])
  AND m.status = 'active';

-- ==================== 商户营业状态管理 ====================

-- name: UpdateMerchantIsOpen :one
-- 更新商户营业状态（手动开店/打烊）
UPDATE merchants
SET
  is_open = $2,
  auto_close_at = $3,
  updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetMerchantIsOpen :one
-- 获取商户营业状态
SELECT id, is_open, auto_close_at FROM merchants
WHERE id = $1;

-- name: ListOpenMerchants :many
-- 获取营业中的商户列表
SELECT * FROM merchants
WHERE status = 'active'
  AND is_open = true
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: AutoCloseMerchants :many
-- 自动打烊（用于定时任务）
UPDATE merchants
SET
  is_open = false,
  auto_close_at = NULL,
  updated_at = now()
WHERE is_open = true
  AND auto_close_at IS NOT NULL
  AND auto_close_at <= now()
RETURNING id;

-- ==================== 运营商管理商户 ====================

-- name: ListMerchantsByRegion :many
-- 按区域列出商户（供运营商管理使用）
SELECT * FROM merchants
WHERE region_id = $1
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountMerchantsByRegion :one
-- 统计区域内商户数量
SELECT COUNT(*) FROM merchants
WHERE region_id = $1
  AND deleted_at IS NULL;

-- name: ListMerchantsByRegionWithStatus :many
-- 按区域和状态列出商户
SELECT * FROM merchants
WHERE region_id = $1
  AND ($2::varchar IS NULL OR status = $2)
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountMerchantsByRegionWithStatus :one
-- 统计区域内指定状态的商户数量
SELECT COUNT(*) FROM merchants
WHERE region_id = $1
  AND ($2::varchar IS NULL OR status = $2)
  AND deleted_at IS NULL;

-- name: GetMerchantByBossBindCode :one
-- 通过 Boss 认领码获取商户
SELECT * FROM merchants
WHERE boss_bind_code = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: UpdateMerchantBossBindCode :one
-- 更新 Boss 认领码
UPDATE merchants
SET boss_bind_code = $2, boss_bind_code_expires_at = $3, updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: ClearMerchantBossBindCode :exec
-- 清除 Boss 认领码
UPDATE merchants
SET boss_bind_code = NULL, boss_bind_code_expires_at = NULL, updated_at = now()
WHERE id = $1;

