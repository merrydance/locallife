-- ==================== 商户入驻申请 ====================

-- name: CreateMerchantApplication :one
INSERT INTO merchant_applications (
  user_id,
  merchant_name,
  business_license_number,
  legal_person_name,
  legal_person_id_number,
  contact_phone,
  business_address,
  business_scope,
  longitude,
  latitude,
  region_id
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: GetMerchantApplication :one
SELECT id, user_id, merchant_name, business_license_number, legal_person_name, legal_person_id_number, contact_phone, business_address, business_scope, status, reject_reason, reviewed_by, reviewed_at, created_at, updated_at, longitude, latitude, region_id, food_permit_ocr, business_license_ocr, id_card_front_ocr, id_card_back_ocr, storefront_images, environment_images, business_license_media_asset_id, food_permit_media_asset_id, id_card_front_media_asset_id, id_card_back_media_asset_id, review_summary FROM merchant_applications
WHERE id = $1 LIMIT 1;

-- name: GetUserMerchantApplication :one
SELECT id, user_id, merchant_name, business_license_number, legal_person_name, legal_person_id_number, contact_phone, business_address, business_scope, status, reject_reason, reviewed_by, reviewed_at, created_at, updated_at, longitude, latitude, region_id, food_permit_ocr, business_license_ocr, id_card_front_ocr, id_card_back_ocr, storefront_images, environment_images, business_license_media_asset_id, food_permit_media_asset_id, id_card_front_media_asset_id, id_card_back_media_asset_id, review_summary FROM merchant_applications
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT 1;

-- name: ListMerchantApplications :many
SELECT id, user_id, merchant_name, business_license_number, legal_person_name, legal_person_id_number, contact_phone, business_address, business_scope, status, reject_reason, reviewed_by, reviewed_at, created_at, updated_at, longitude, latitude, region_id, food_permit_ocr, business_license_ocr, id_card_front_ocr, id_card_back_ocr, storefront_images, environment_images, business_license_media_asset_id, food_permit_media_asset_id, id_card_front_media_asset_id, id_card_back_media_asset_id, review_summary FROM merchant_applications
WHERE status = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: ListAllMerchantApplications :many
SELECT id, user_id, merchant_name, business_license_number, legal_person_name, legal_person_id_number, contact_phone, business_address, business_scope, status, reject_reason, reviewed_by, reviewed_at, created_at, updated_at, longitude, latitude, region_id, food_permit_ocr, business_license_ocr, id_card_front_ocr, id_card_back_ocr, storefront_images, environment_images, business_license_media_asset_id, food_permit_media_asset_id, id_card_front_media_asset_id, id_card_back_media_asset_id, review_summary FROM merchant_applications
ORDER BY created_at DESC, id DESC
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
SELECT id, user_id, merchant_name, business_license_number, legal_person_name, legal_person_id_number, contact_phone, business_address, business_scope, status, reject_reason, reviewed_by, reviewed_at, created_at, updated_at, longitude, latitude, region_id, food_permit_ocr, business_license_ocr, id_card_front_ocr, id_card_back_ocr, storefront_images, environment_images, business_license_media_asset_id, food_permit_media_asset_id, id_card_front_media_asset_id, id_card_back_media_asset_id, review_summary FROM merchant_applications
WHERE business_license_number = $1
LIMIT 1;

-- name: GetLatestApprovedMerchantApplicationByUser :one
SELECT id, user_id, merchant_name, business_license_number, legal_person_name, legal_person_id_number, contact_phone, business_address, business_scope, status, reject_reason, reviewed_by, reviewed_at, created_at, updated_at, longitude, latitude, region_id, food_permit_ocr, business_license_ocr, id_card_front_ocr, id_card_back_ocr, storefront_images, environment_images, business_license_media_asset_id, food_permit_media_asset_id, id_card_front_media_asset_id, id_card_back_media_asset_id, review_summary FROM merchant_applications
WHERE user_id = $1
  AND status = 'approved'
  AND (
    SELECT COUNT(*)
    FROM merchants
    WHERE owner_user_id = $1
      AND deleted_at IS NULL
  ) = 1
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- ==================== 商户管理 ====================

-- name: CreateMerchant :one
INSERT INTO merchants (
  owner_user_id,
  name,
  description,
  phone,
  address,
  latitude,
  longitude,
  status,
  application_data,
  region_id,
  storefront_images,
  environment_images
) VALUES (
  $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
RETURNING *;

-- name: GetMerchant :one
SELECT id, owner_user_id, name, description, phone, address, latitude, longitude, status, application_data, created_at, updated_at, version, region_id, is_open, auto_close_at, deleted_at, pending_owner_bind, bind_code, bind_code_expires_at, group_id, brand_id, logo_media_asset_id, auto_open_by_business_hours, storefront_images, environment_images, manual_open_status_until FROM merchants
WHERE id = $1 AND deleted_at IS NULL LIMIT 1;

-- name: GetMerchantOwnedByUser :one
SELECT id, owner_user_id, name, description, phone, address, latitude, longitude, status, application_data, created_at, updated_at, version, region_id, is_open, auto_close_at, deleted_at, pending_owner_bind, bind_code, bind_code_expires_at, group_id, brand_id, logo_media_asset_id, auto_open_by_business_hours, storefront_images, environment_images, manual_open_status_until FROM merchants
WHERE owner_user_id = $1 AND deleted_at IS NULL
ORDER BY created_at ASC, id ASC
LIMIT 1;

-- name: LockMerchantForUpdate :one
SELECT id FROM merchants
WHERE id = $1 AND deleted_at IS NULL
FOR UPDATE;

-- name: GetMerchantByOwner :one
-- 获取用户关联的商户（支持店主和员工）
-- 优先返回 owner_user_id 匹配的商户，其次返回 merchant_staff 关联的商户
SELECT m.id, m.owner_user_id, m.name, m.description, m.phone, m.address, m.latitude, m.longitude, m.status, m.application_data, m.created_at, m.updated_at, m.version, m.region_id, m.is_open, m.auto_close_at, m.deleted_at, m.pending_owner_bind, m.bind_code, m.bind_code_expires_at, m.group_id, m.brand_id, m.logo_media_asset_id, m.auto_open_by_business_hours, m.storefront_images, m.environment_images, m.manual_open_status_until FROM merchants m
LEFT JOIN merchant_staff ms ON m.id = ms.merchant_id AND ms.status = 'active' AND ms.role <> 'pending'
WHERE (m.owner_user_id = $1 OR ms.user_id = $1) AND m.deleted_at IS NULL
ORDER BY CASE WHEN m.owner_user_id = $1 THEN 0 ELSE 1 END
LIMIT 1;

-- name: ListMerchantsByOwner :many
-- 获取用户拥有的所有商户（用于多店铺切换）
SELECT id, owner_user_id, name, description, phone, address, latitude, longitude, status, application_data, created_at, updated_at, version, region_id, is_open, auto_close_at, deleted_at, pending_owner_bind, bind_code, bind_code_expires_at, group_id, brand_id, logo_media_asset_id, auto_open_by_business_hours, storefront_images, environment_images, manual_open_status_until FROM merchants
WHERE owner_user_id = $1 AND deleted_at IS NULL
ORDER BY created_at ASC;

-- name: GetMerchantByBindCode :one
-- 通过邀请码获取商户
SELECT id, owner_user_id, name, description, phone, address, latitude, longitude, status, application_data, created_at, updated_at, version, region_id, is_open, auto_close_at, deleted_at, pending_owner_bind, bind_code, bind_code_expires_at, group_id, brand_id, logo_media_asset_id, auto_open_by_business_hours, storefront_images, environment_images, manual_open_status_until FROM merchants
WHERE bind_code = $1 AND deleted_at IS NULL
LIMIT 1;

-- name: ListMerchants :many
SELECT id, owner_user_id, name, description, phone, address, latitude, longitude, status, application_data, created_at, updated_at, version, region_id, is_open, auto_close_at, deleted_at, pending_owner_bind, bind_code, bind_code_expires_at, group_id, brand_id, logo_media_asset_id, auto_open_by_business_hours, storefront_images, environment_images, manual_open_status_until FROM merchants
WHERE status = $1 AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListAllMerchants :many
SELECT id, owner_user_id, name, description, phone, address, latitude, longitude, status, application_data, created_at, updated_at, version, region_id, is_open, auto_close_at, deleted_at, pending_owner_bind, bind_code, bind_code_expires_at, group_id, brand_id, logo_media_asset_id, auto_open_by_business_hours, storefront_images, environment_images, manual_open_status_until FROM merchants
WHERE deleted_at IS NULL
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateMerchant :one
-- ✅ P1-2: 使用乐观锁(version)防止并发更新丢失
UPDATE merchants
SET
  name = COALESCE(sqlc.narg('name'), name),
  description = COALESCE(sqlc.narg('description'), description),
  logo_media_asset_id = COALESCE(sqlc.narg('logo_media_asset_id'), logo_media_asset_id),
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

-- name: ClearMerchantLogo :one
UPDATE merchants
SET
  logo_media_asset_id = NULL,
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

-- name: UpdateMerchantShopImages :one
-- 更新商户 live 门头照和环境照（已通过审核后的店铺展示图）
UPDATE merchants
SET
  storefront_images = COALESCE(sqlc.narg('storefront_images'), storefront_images),
  environment_images = COALESCE(sqlc.narg('environment_images'), environment_images),
  updated_at = now()
WHERE id = $1 AND deleted_at IS NULL
RETURNING *;

-- name: ActivateApprovedMerchant :one
UPDATE merchants
SET
  status = 'active',
  updated_at = now()
WHERE id = $1
  AND status = 'approved'
  AND deleted_at IS NULL
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

-- name: CreateMerchantBindCodeWhenInactive :one
-- 仅在商户当前没有有效邀请码时创建邀请码，避免并发打开邀请弹窗互相覆盖
UPDATE merchants
SET
  bind_code = $2,
  bind_code_expires_at = $3,
  updated_at = now()
WHERE id = $1
  AND deleted_at IS NULL
  AND (
    bind_code IS NULL
    OR bind_code = ''
    OR bind_code_expires_at IS NULL
    OR bind_code_expires_at <= now()
  )
RETURNING *;

-- name: DeleteMerchant :exec
-- 软删除商户
UPDATE merchants SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL;

-- name: SearchMerchants :many
SELECT m.id, m.owner_user_id, m.name, m.description, m.phone, m.address, m.latitude, m.longitude, m.status, m.application_data, m.created_at, m.updated_at, m.version, m.region_id, m.is_open, m.auto_close_at, m.deleted_at, m.pending_owner_bind, m.bind_code, m.bind_code_expires_at, m.group_id, m.brand_id, m.logo_media_asset_id, m.auto_open_by_business_hours, COALESCE(mp.total_orders, 0)::int AS total_orders,
  COALESCE(
    (SELECT json_agg(t.name)
     FROM tags t
     INNER JOIN merchant_tags mt ON t.id = mt.tag_id
     WHERE mt.merchant_id = m.id
       AND t.type = 'merchant'
       AND t.status = 'active'
    ), '[]'::json) AS tags,
  COALESCE(
    (SELECT json_agg(t.name ORDER BY t.sort_order ASC, t.name ASC)
     FROM tags t
     INNER JOIN merchant_system_labels msl ON t.id = msl.tag_id
     WHERE msl.merchant_id = m.id
       AND t.type = 'system'
       AND t.status = 'active'
    ), '[]'::json) AS system_labels,
  COALESCE(m.storefront_images, ma.storefront_images) AS storefront_images,
  COALESCE((SELECT AVG(d.repurchase_rate)
     FROM dishes d
     WHERE d.merchant_id = m.id
       AND d.deleted_at IS NULL
       AND d.is_online = true), 0)::float8 AS avg_repurchase_rate
  , COALESCE(earth_distance(ll_to_earth(m.latitude::float8, m.longitude::float8), ll_to_earth($4::float8, $5::float8)), 0)::bigint AS distance_meters
FROM merchants m
  LEFT JOIN merchant_profiles mp ON m.id = mp.merchant_id
  LEFT JOIN LATERAL (
    SELECT selected_ma.storefront_images
    FROM merchant_applications selected_ma
    WHERE selected_ma.user_id = m.owner_user_id
      AND selected_ma.status = 'approved'
      AND (
        SELECT COUNT(*)
        FROM merchants owned_m
        WHERE owned_m.owner_user_id = m.owner_user_id
          AND owned_m.deleted_at IS NULL
      ) = 1
    ORDER BY selected_ma.created_at DESC, selected_ma.id DESC
    LIMIT 1
  ) ma ON true
WHERE m.status = 'active'
  AND m.deleted_at IS NULL
  AND COALESCE(mp.is_takeout_suspended, false) = false
  AND m.region_id = sqlc.narg('region_id')
  AND (
    $3::text IS NULL
    OR m.name ILIKE '%' || $3 || '%'
  )
ORDER BY
    m.is_open DESC,
    CASE WHEN sqlc.arg('sort_by') = 'distance'
      THEN earth_distance(ll_to_earth(m.latitude::float8, m.longitude::float8), ll_to_earth($4::float8, $5::float8))
    END ASC NULLS LAST,
    CASE WHEN sqlc.arg('sort_by') = 'distance' THEN COALESCE((SELECT AVG(d.repurchase_rate)
     FROM dishes d
     WHERE d.merchant_id = m.id
       AND d.deleted_at IS NULL
       AND d.is_online = true), 0) END DESC NULLS LAST,
  CASE WHEN sqlc.arg('sort_by') = 'distance' THEN COALESCE(mp.total_orders, 0) END DESC NULLS LAST,
    CASE WHEN sqlc.arg('sort_by') = 'distance' THEN NULL ELSE COALESCE((SELECT AVG(d.repurchase_rate)
     FROM dishes d
     WHERE d.merchant_id = m.id
       AND d.deleted_at IS NULL
       AND d.is_online = true), 0) END DESC NULLS LAST,
  CASE WHEN sqlc.arg('sort_by') = 'distance' THEN NULL ELSE COALESCE(mp.total_orders, 0) END DESC NULLS LAST,
  m.id ASC
LIMIT $2
OFFSET $1;

-- name: CountSearchMerchants :one
SELECT COUNT(*) FROM merchants m
LEFT JOIN merchant_profiles mp ON m.id = mp.merchant_id
WHERE m.status = 'active'
  AND m.deleted_at IS NULL
  AND COALESCE(mp.is_takeout_suspended, false) = false
  AND m.region_id = sqlc.narg('region_id')
  AND m.name ILIKE '%' || $1 || '%';

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
SELECT t.id, t.name, t.type, t.sort_order, t.status, t.created_at, t.icon FROM tags t
INNER JOIN merchant_tags mt ON t.id = mt.tag_id
WHERE mt.merchant_id = $1
ORDER BY t.name;

-- name: ListMerchantsByTag :many
SELECT m.id, m.owner_user_id, m.name, m.description, m.phone, m.address, m.latitude, m.longitude, m.status, m.application_data, m.created_at, m.updated_at, m.version, m.region_id, m.is_open, m.auto_close_at, m.deleted_at, m.pending_owner_bind, m.bind_code, m.bind_code_expires_at, m.group_id, m.brand_id, m.logo_media_asset_id, m.auto_open_by_business_hours, m.storefront_images, m.environment_images, m.manual_open_status_until FROM merchants m
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
SELECT id, merchant_id, day_of_week, open_time, close_time, is_closed, special_date, created_at, updated_at FROM merchant_business_hours
WHERE id = $1 LIMIT 1;

-- name: ListMerchantBusinessHours :many
SELECT id, merchant_id, day_of_week, open_time, close_time, is_closed, special_date, created_at, updated_at FROM merchant_business_hours
WHERE merchant_id = $1
  AND special_date IS NULL
ORDER BY day_of_week, open_time;

-- name: ListMerchantBusinessHoursAll :many
SELECT id, merchant_id, day_of_week, open_time, close_time, is_closed, special_date, created_at, updated_at FROM merchant_business_hours
WHERE merchant_id = $1
ORDER BY special_date NULLS FIRST, day_of_week, open_time;

-- name: ListMerchantSpecialHours :many
SELECT id, merchant_id, day_of_week, open_time, close_time, is_closed, special_date, created_at, updated_at FROM merchant_business_hours
WHERE merchant_id = $1
  AND special_date IS NOT NULL
ORDER BY special_date;

-- name: GetBusinessHourByDate :one
SELECT id, merchant_id, day_of_week, open_time, close_time, is_closed, special_date, created_at, updated_at FROM merchant_business_hours
WHERE merchant_id = $1
  AND special_date = $2
LIMIT 1;

-- name: GetBusinessHourByDayOfWeek :one
SELECT id, merchant_id, day_of_week, open_time, close_time, is_closed, special_date, created_at, updated_at FROM merchant_business_hours
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

-- name: UpdateMerchantAutoOpenByBusinessHours :exec
UPDATE merchants
SET
  auto_open_by_business_hours = $2,
  auto_close_at = CASE WHEN $2 THEN NULL ELSE auto_close_at END,
  manual_open_status_until = NULL,
  updated_at = now()
WHERE id = $1;

-- ==================== 高级查询（使用JOIN和聚合）====================

-- name: GetMerchantWithTags :one
SELECT 
  sqlc.embed(merchants),
  COALESCE(
    (
      SELECT json_agg(
        json_build_object(
          'id', tags.id,
          'name', tags.name,
          'type', tags.type,
          'sort_order', tags.sort_order,
          'status', tags.status,
          'created_at', tags.created_at
        )
      )
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
  m.id, m.owner_user_id, m.name, m.description, m.phone, m.address, m.latitude, m.longitude, m.status, m.application_data, m.created_at, m.updated_at, m.version, m.region_id, m.is_open, m.auto_close_at, m.deleted_at, m.pending_owner_bind, m.bind_code, m.bind_code_expires_at, m.group_id, m.brand_id, m.logo_media_asset_id, m.auto_open_by_business_hours, m.storefront_images, m.environment_images, m.manual_open_status_until,
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
    m.logo_media_asset_id,
    m.address,
    m.latitude,
    m.longitude,
    COALESCE(COUNT(o.id), 0)::int AS total_orders,
  0::numeric(3,2) AS avg_rating
FROM merchants m
LEFT JOIN orders o ON m.id = o.merchant_id 
  AND o.status IN ('completed')  -- 以已完成订单作为销量口径
WHERE m.status = 'active'
  AND m.is_open = true
GROUP BY m.id
ORDER BY 
    m.is_open DESC,
    total_orders DESC,  -- 销量优先：回头客多的商户排前面
    avg_rating DESC,     -- 评分次之
    earth_distance(ll_to_earth(m.latitude, m.longitude), ll_to_earth($2::float8, $3::float8)) ASC
LIMIT $1;

-- name: GetMerchantsByIDs :many
-- 批量获取商户详情
SELECT 
    id,
    name,
    description,
    logo_media_asset_id,
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
    m.logo_media_asset_id,
    m.address,
    m.latitude,
    m.longitude,
    m.region_id,
    m.status,
    m.is_open,
    COALESCE(
        (SELECT COUNT(*)
         FROM orders o 
         WHERE o.merchant_id = m.id 
           AND o.status IN ('completed')
           AND o.created_at >= NOW() - INTERVAL '30 days'
        ), 0
    )::int AS monthly_orders
FROM merchants m
WHERE m.id = ANY($1::bigint[])
  AND m.status = 'active';

-- ==================== 商户营业状态管理 ====================

-- name: UpdateMerchantIsOpen :one
-- 更新商户营业状态（手动开店/打烊）
UPDATE merchants
SET
  is_open = $2,
  auto_close_at = $3,
  manual_open_status_until = $4,
  updated_at = now()
WHERE id = $1
RETURNING *;

-- name: GetMerchantIsOpen :one
-- 获取商户营业状态
SELECT id, is_open, auto_close_at, manual_open_status_until, auto_open_by_business_hours FROM merchants
WHERE id = $1;

-- name: ListOpenMerchants :many
-- 获取营业中的商户列表
SELECT id, owner_user_id, name, description, phone, address, latitude, longitude, status, application_data, created_at, updated_at, version, region_id, is_open, auto_close_at, deleted_at, pending_owner_bind, bind_code, bind_code_expires_at, group_id, brand_id, logo_media_asset_id, auto_open_by_business_hours, storefront_images, environment_images, manual_open_status_until FROM merchants
WHERE status = 'active'
  AND is_open = true
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetDatabaseLocalClock :one
SELECT
  EXTRACT(YEAR FROM CURRENT_DATE)::int AS current_year,
  EXTRACT(MONTH FROM CURRENT_DATE)::int AS current_month,
  EXTRACT(DAY FROM CURRENT_DATE)::int AS current_day,
  (
    EXTRACT(HOUR FROM LOCALTIME)::bigint * 3600 * 1000000 +
    EXTRACT(MINUTE FROM LOCALTIME)::bigint * 60 * 1000000 +
    FLOOR(EXTRACT(SECOND FROM LOCALTIME) * 1000000)::bigint
  )::bigint AS local_time_micros,
  current_setting('TIMEZONE')::text AS time_zone;

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

-- name: SyncMerchantOpenStatusByBusinessHours :many
WITH merchants_with_hours AS (
  SELECT DISTINCT merchant_id
  FROM merchant_business_hours
),
today_rows AS (
  SELECT
    mbh.merchant_id,
    mbh.is_closed,
    mbh.open_time,
    mbh.close_time,
    mbh.special_date
  FROM merchant_business_hours mbh
  JOIN merchants_with_hours mwh ON mwh.merchant_id = mbh.merchant_id
  WHERE mbh.special_date = CURRENT_DATE
     OR (mbh.special_date IS NULL AND mbh.day_of_week = EXTRACT(DOW FROM CURRENT_DATE)::int)
),
mode_by_merchant AS (
  SELECT
    merchant_id,
    BOOL_OR(special_date IS NOT NULL) AS has_special
  FROM today_rows
  GROUP BY merchant_id
),
effective_rows AS (
  SELECT
    tr.merchant_id,
    tr.is_closed,
    tr.open_time,
    tr.close_time,
    tr.special_date
  FROM today_rows tr
  JOIN mode_by_merchant mm ON mm.merchant_id = tr.merchant_id
  WHERE (mm.has_special AND tr.special_date IS NOT NULL)
     OR (NOT mm.has_special AND tr.special_date IS NULL)
),
desired_state AS (
  SELECT
    mwh.merchant_id,
    CASE
      WHEN COUNT(er.merchant_id) = 0 THEN false
      WHEN BOOL_OR(er.is_closed) THEN false
      WHEN BOOL_OR((NOT er.is_closed) AND (LOCALTIME >= er.open_time AND LOCALTIME < er.close_time)) THEN true
      ELSE false
    END AS should_open
  FROM merchants_with_hours mwh
  LEFT JOIN effective_rows er ON er.merchant_id = mwh.merchant_id
  GROUP BY mwh.merchant_id
),
updated AS (
  UPDATE merchants m
  SET
    is_open = CASE
      WHEN ds.should_open
        AND COALESCE(mpc.sub_mch_id, '') <> ''
        AND mpc.status = 'active' THEN true
      ELSE false
    END,
    auto_close_at = NULL,
    manual_open_status_until = NULL,
    updated_at = now()
  FROM desired_state ds
  LEFT JOIN merchant_payment_configs mpc ON mpc.merchant_id = ds.merchant_id
  WHERE m.id = ds.merchant_id
    AND m.status = 'active'
    AND m.auto_open_by_business_hours = true
    AND (
      m.manual_open_status_until IS NULL
      OR m.manual_open_status_until <= now()
      OR COALESCE(mpc.sub_mch_id, '') = ''
      OR mpc.status IS DISTINCT FROM 'active'
    )
    AND m.is_open IS DISTINCT FROM CASE
      WHEN ds.should_open
        AND COALESCE(mpc.sub_mch_id, '') <> ''
        AND mpc.status = 'active' THEN true
      ELSE false
    END
  RETURNING m.id
)
SELECT id
FROM updated
ORDER BY id;

-- name: ClearExpiredMerchantManualOpenStatusOverrides :execrows
UPDATE merchants
SET
  manual_open_status_until = NULL,
  updated_at = now()
WHERE auto_open_by_business_hours = true
  AND manual_open_status_until IS NOT NULL
  AND manual_open_status_until <= now();

-- ==================== 运营商管理商户 ====================

-- name: ListMerchantsByRegion :many
-- 按区域列出商户（供运营商管理使用）
SELECT id, owner_user_id, name, description, phone, address, latitude, longitude, status, application_data, created_at, updated_at, version, region_id, is_open, auto_close_at, deleted_at, pending_owner_bind, bind_code, bind_code_expires_at, group_id, brand_id, logo_media_asset_id, auto_open_by_business_hours, storefront_images, environment_images, manual_open_status_until FROM merchants
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
SELECT id, owner_user_id, name, description, phone, address, latitude, longitude, status, application_data, created_at, updated_at, version, region_id, is_open, auto_close_at, deleted_at, pending_owner_bind, bind_code, bind_code_expires_at, group_id, brand_id, logo_media_asset_id, auto_open_by_business_hours, storefront_images, environment_images, manual_open_status_until FROM merchants
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


-- name: CheckBusinessLicenseExists :one
-- 检查营业执照号是否已被其他已通过的申请占用
SELECT COUNT(*) FROM merchant_applications
WHERE business_license_number = $1
  AND status = 'approved'
  AND id != $2;

-- name: CheckLegalPersonIDExists :one
-- 检查法人身份证号是否已被其他已通过的申请占用
SELECT COUNT(*) FROM merchant_applications
WHERE legal_person_id_number = $1
  AND status = 'approved'
  AND id != $2;

-- name: SearchMerchantsByTag :many
-- 按标签（菜系）过滤商户，支持区域和位置排序
SELECT m.id, m.owner_user_id, m.name, m.description, m.phone, m.address, m.latitude, m.longitude, m.status, m.application_data, m.created_at, m.updated_at, m.version, m.region_id, m.is_open, m.auto_close_at, m.deleted_at, m.pending_owner_bind, m.bind_code, m.bind_code_expires_at, m.group_id, m.brand_id, m.logo_media_asset_id, m.auto_open_by_business_hours, COALESCE(mp.total_orders, 0)::int AS total_orders,
  COALESCE(
    (SELECT json_agg(t.name)
     FROM tags t
     INNER JOIN merchant_tags mt ON t.id = mt.tag_id
     WHERE mt.merchant_id = m.id
       AND t.type = 'merchant'
       AND t.status = 'active'
    ), '[]'::json) AS tags,
  COALESCE(
    (SELECT json_agg(t.name ORDER BY t.sort_order ASC, t.name ASC)
     FROM tags t
     INNER JOIN merchant_system_labels msl ON t.id = msl.tag_id
     WHERE msl.merchant_id = m.id
       AND t.type = 'system'
       AND t.status = 'active'
    ), '[]'::json) AS system_labels,
  COALESCE(m.storefront_images, ma.storefront_images) AS storefront_images,
  COALESCE((SELECT AVG(d.repurchase_rate)
     FROM dishes d
     WHERE d.merchant_id = m.id
       AND d.deleted_at IS NULL
       AND d.is_online = true), 0)::float8 AS avg_repurchase_rate
  , COALESCE(earth_distance(ll_to_earth(m.latitude::float8, m.longitude::float8), ll_to_earth(sqlc.arg('user_lat')::float8, sqlc.arg('user_lng')::float8)), 0)::bigint AS distance_meters
FROM merchants m
  LEFT JOIN merchant_profiles mp ON m.id = mp.merchant_id
  LEFT JOIN LATERAL (
    SELECT selected_ma.storefront_images
    FROM merchant_applications selected_ma
    WHERE selected_ma.user_id = m.owner_user_id
      AND selected_ma.status = 'approved'
      AND (
        SELECT COUNT(*)
        FROM merchants owned_m
        WHERE owned_m.owner_user_id = m.owner_user_id
          AND owned_m.deleted_at IS NULL
      ) = 1
    ORDER BY selected_ma.created_at DESC, selected_ma.id DESC
    LIMIT 1
  ) ma ON true
  INNER JOIN merchant_tags mt_filter ON m.id = mt_filter.merchant_id
WHERE m.status = 'active'
  AND m.deleted_at IS NULL
  AND COALESCE(mp.is_takeout_suspended, false) = false
  AND m.region_id = sqlc.narg('region_id')
  AND mt_filter.tag_id = sqlc.arg('tag_id')
ORDER BY
    m.is_open DESC,
    CASE WHEN sqlc.arg('sort_by') = 'distance'
      THEN earth_distance(ll_to_earth(m.latitude::float8, m.longitude::float8), ll_to_earth(sqlc.arg('user_lat')::float8, sqlc.arg('user_lng')::float8))
    END ASC NULLS LAST,
    CASE WHEN sqlc.arg('sort_by') = 'distance' THEN COALESCE((SELECT AVG(d.repurchase_rate)
     FROM dishes d
     WHERE d.merchant_id = m.id
       AND d.deleted_at IS NULL
       AND d.is_online = true), 0) END DESC NULLS LAST,
  CASE WHEN sqlc.arg('sort_by') = 'distance' THEN COALESCE(mp.total_orders, 0) END DESC NULLS LAST,
    CASE WHEN sqlc.arg('sort_by') = 'distance' THEN NULL ELSE COALESCE((SELECT AVG(d.repurchase_rate)
     FROM dishes d
     WHERE d.merchant_id = m.id
       AND d.deleted_at IS NULL
       AND d.is_online = true), 0) END DESC NULLS LAST,
  CASE WHEN sqlc.arg('sort_by') = 'distance' THEN NULL ELSE COALESCE(mp.total_orders, 0) END DESC NULLS LAST,
  m.id ASC
LIMIT sqlc.arg('limit')
OFFSET sqlc.arg('offset');

-- name: CountSearchMerchantsByTag :one
-- 统计指定标签在区域内的商户数量（用于分页）
SELECT COUNT(*) FROM merchants m
LEFT JOIN merchant_profiles mp ON m.id = mp.merchant_id
INNER JOIN merchant_tags mt ON m.id = mt.merchant_id
WHERE m.status = 'active'
  AND m.deleted_at IS NULL
  AND COALESCE(mp.is_takeout_suspended, false) = false
  AND m.region_id = sqlc.narg('region_id')
  AND mt.tag_id = sqlc.arg('tag_id');
