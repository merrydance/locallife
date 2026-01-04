-- name: CreateTable :one
INSERT INTO tables (
    merchant_id,
    table_no,
    table_type,
    capacity,
    description,
    minimum_spend,
    qr_code_url,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetTable :one
SELECT * FROM tables
WHERE id = $1 LIMIT 1;

-- name: GetTableForUpdate :one
SELECT * FROM tables
WHERE id = $1 LIMIT 1
FOR UPDATE;

-- name: GetTableByMerchantAndNo :one
SELECT * FROM tables
WHERE merchant_id = $1 AND table_no = $2 LIMIT 1;

-- name: ListTablesByMerchant :many
SELECT * FROM tables
WHERE merchant_id = $1
ORDER BY table_type, table_no;

-- name: ListTablesByMerchantAndType :many
SELECT * FROM tables
WHERE merchant_id = $1 
  AND table_type = $2
ORDER BY table_no;

-- name: ListAvailableRooms :many
SELECT * FROM tables
WHERE merchant_id = $1 
  AND table_type = 'room' 
  AND status = 'available'
ORDER BY table_no;

-- name: UpdateTable :one
UPDATE tables
SET table_no = COALESCE(sqlc.narg(table_no), table_no),
    capacity = COALESCE(sqlc.narg(capacity), capacity),
    description = COALESCE(sqlc.narg(description), description),
    minimum_spend = COALESCE(sqlc.narg(minimum_spend), minimum_spend),
    qr_code_url = COALESCE(sqlc.narg(qr_code_url), qr_code_url),
    status = COALESCE(sqlc.narg(status), status),
    updated_at = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: UpdateTableStatus :one
UPDATE tables
SET status = $2,
    current_reservation_id = $3,
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteTable :exec
DELETE FROM tables
WHERE id = $1;

-- name: CountTablesByMerchant :one
SELECT COUNT(*) FROM tables
WHERE merchant_id = $1;

-- name: CountAvailableTablesByMerchant :one
SELECT COUNT(*) FROM tables
WHERE merchant_id = $1 
  AND status = 'available';

-- ============ Table Images ============

-- name: AddTableImage :one
INSERT INTO table_images (
    table_id,
    image_url,
    sort_order,
    is_primary
) VALUES (
    $1, $2, $3, $4
) RETURNING *;

-- name: ListTableImages :many
SELECT * FROM table_images
WHERE table_id = $1
ORDER BY is_primary DESC, sort_order ASC, created_at ASC;

-- name: GetPrimaryTableImage :one
SELECT * FROM table_images
WHERE table_id = $1 AND is_primary = TRUE
LIMIT 1;

-- name: UpdateTableImage :one
UPDATE table_images
SET sort_order = COALESCE(sqlc.narg('sort_order'), sort_order),
    is_primary = COALESCE(sqlc.narg('is_primary'), is_primary)
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteTableImage :exec
DELETE FROM table_images WHERE id = $1;

-- name: DeleteAllTableImages :exec
DELETE FROM table_images WHERE table_id = $1;

-- name: SetPrimaryTableImage :exec
-- 先清除所有主图标记，再设置新的主图
UPDATE table_images SET is_primary = FALSE WHERE table_id = $1;

-- name: SetTableImagePrimary :one
UPDATE table_images SET is_primary = TRUE WHERE id = $1 RETURNING *;

-- ============ Customer-side Room Queries (C端包间查询) ============

-- name: GetRoomDetailForCustomer :one
-- 获取包间详情（含商户信息、主图、月销量）供顾客查看
SELECT 
    t.id,
    t.merchant_id,
    t.table_no,
    t.capacity,
    t.description,
    t.minimum_spend,
    t.status,
    t.created_at,
    m.name as merchant_name,
    m.logo_url as merchant_logo,
    m.address as merchant_address,
    m.latitude as merchant_latitude,
    m.longitude as merchant_longitude,
    m.phone as merchant_phone,
    COALESCE((SELECT image_url FROM table_images WHERE table_id = t.id AND is_primary = TRUE LIMIT 1), '')::TEXT as primary_image,
    (SELECT COUNT(*) FROM table_reservations tr 
     WHERE tr.table_id = t.id 
       AND tr.status IN ('confirmed', 'completed')
       AND tr.created_at >= NOW() - INTERVAL '30 days') as monthly_reservations
FROM tables t
INNER JOIN merchants m ON t.merchant_id = m.id
WHERE t.id = $1 
  AND t.table_type = 'room'
  AND m.status = 'approved';

-- name: ListMerchantRoomsForCustomer :many
-- 获取商户的包间列表（含主图、月销量）供顾客查看
SELECT 
    t.id,
    t.merchant_id,
    t.table_no,
    t.capacity,
    t.description,
    t.minimum_spend,
    t.status,
    t.created_at,
    COALESCE(
        (SELECT image_url FROM table_images WHERE table_id = t.id AND is_primary = TRUE LIMIT 1),
        (SELECT image_url FROM table_images WHERE table_id = t.id ORDER BY sort_order ASC, created_at ASC LIMIT 1),
        ''
    )::TEXT as primary_image,
    (SELECT COUNT(*) FROM table_reservations tr 
     WHERE tr.table_id = t.id 
       AND tr.status IN ('confirmed', 'completed')
       AND tr.created_at >= NOW() - INTERVAL '30 days') as monthly_reservations
FROM tables t
WHERE t.merchant_id = $1 
  AND t.table_type = 'room'
ORDER BY t.capacity, t.table_no;

-- name: ListAvailableRoomsForCustomer :many
-- 获取商户的可用包间列表（含主图）供顾客查看
SELECT 
    t.id,
    t.merchant_id,
    t.table_no,
    t.capacity,
    t.description,
    t.minimum_spend,
    t.status,
    COALESCE((SELECT image_url FROM table_images WHERE table_id = t.id AND is_primary = TRUE LIMIT 1), '')::TEXT as primary_image
FROM tables t
WHERE t.merchant_id = $1 
  AND t.table_type = 'room'
  AND t.status = 'available'
ORDER BY t.capacity, t.table_no;

-- ============ Table Tags ============

-- name: AddTableTag :one
INSERT INTO table_tags (
    table_id,
    tag_id
) VALUES (
    $1, $2
) RETURNING *;

-- name: RemoveTableTag :exec
DELETE FROM table_tags
WHERE table_id = $1 AND tag_id = $2;

-- name: RemoveAllTableTags :exec
DELETE FROM table_tags
WHERE table_id = $1;

-- name: ListTableTags :many
SELECT 
    tt.id,
    tt.table_id,
    tt.tag_id,
    tt.created_at,
    t.name as tag_name,
    t.type as tag_type
FROM table_tags tt
INNER JOIN tags t ON tt.tag_id = t.id
WHERE tt.table_id = $1
ORDER BY t.name;

-- name: ListTablesByTag :many
SELECT tb.* FROM tables tb
INNER JOIN table_tags tt ON tb.id = tt.table_id
WHERE tt.tag_id = $1
ORDER BY tb.table_no;

-- ============ Room Search ============

-- name: SearchRooms :many
-- 搜索包间：按日期、时段、人数、菜系（商户标签）等条件过滤
-- 返回可用包间及其商户信息
SELECT 
    t.id,
    t.merchant_id,
    t.table_no,
    t.table_type,
    t.capacity,
    t.description,
    t.minimum_spend,
    t.qr_code_url,
    t.status,
    t.created_at,
    m.name as merchant_name,
    m.logo_url as merchant_logo,
    m.address as merchant_address,
    m.latitude as merchant_latitude,
    m.longitude as merchant_longitude
FROM tables t
INNER JOIN merchants m ON t.merchant_id = m.id
WHERE t.table_type = 'room'
  AND t.status = 'available'
  AND m.status = 'approved'
  -- 按区域筛选（可选）
  AND (sqlc.narg(region_id)::BIGINT IS NULL OR m.region_id = sqlc.narg(region_id))
  -- 按人数筛选
  AND (sqlc.narg(min_capacity)::SMALLINT IS NULL OR t.capacity >= sqlc.narg(min_capacity))
  AND (sqlc.narg(max_capacity)::SMALLINT IS NULL OR t.capacity <= sqlc.narg(max_capacity))
  -- 按最低消费筛选
  AND (sqlc.narg(max_minimum_spend)::BIGINT IS NULL OR t.minimum_spend IS NULL OR t.minimum_spend <= sqlc.narg(max_minimum_spend))
  -- 排除已在指定日期时段被预定的包间
  AND NOT EXISTS (
    SELECT 1 FROM table_reservations tr
    WHERE tr.table_id = t.id
      AND tr.reservation_date = sqlc.narg(reservation_date)::DATE
      AND tr.reservation_time = sqlc.narg(reservation_time)::TIME
      AND tr.status IN ('pending', 'paid', 'confirmed')
  )
ORDER BY t.capacity, t.minimum_spend NULLS FIRST
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);

-- name: SearchRoomsByMerchantTag :many
-- 按商户标签（菜系）搜索包间
SELECT 
    t.id,
    t.merchant_id,
    t.table_no,
    t.table_type,
    t.capacity,
    t.description,
    t.minimum_spend,
    t.qr_code_url,
    t.status,
    t.created_at,
    m.name as merchant_name,
    m.logo_url as merchant_logo,
    m.address as merchant_address,
    m.latitude as merchant_latitude,
    m.longitude as merchant_longitude
FROM tables t
INNER JOIN merchants m ON t.merchant_id = m.id
INNER JOIN merchant_tags mt ON m.id = mt.merchant_id
WHERE t.table_type = 'room'
  AND t.status = 'available'
  AND m.status = 'approved'
  AND mt.tag_id = sqlc.arg(tag_id)
  -- 按区域筛选（可选）
  AND (sqlc.narg(region_id)::BIGINT IS NULL OR m.region_id = sqlc.narg(region_id))
  -- 按人数筛选
  AND (sqlc.narg(min_capacity)::SMALLINT IS NULL OR t.capacity >= sqlc.narg(min_capacity))
  AND (sqlc.narg(max_capacity)::SMALLINT IS NULL OR t.capacity <= sqlc.narg(max_capacity))
  -- 排除已在指定日期时段被预定的包间
  AND NOT EXISTS (
    SELECT 1 FROM table_reservations tr
    WHERE tr.table_id = t.id
      AND tr.reservation_date = sqlc.narg(reservation_date)::DATE
      AND tr.reservation_time = sqlc.narg(reservation_time)::TIME
      AND tr.status IN ('pending', 'paid', 'confirmed')
  )
ORDER BY t.capacity, t.minimum_spend NULLS FIRST
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);

-- name: CountSearchRooms :one
-- 统计搜索包间结果数量
SELECT COUNT(*) FROM tables t
INNER JOIN merchants m ON t.merchant_id = m.id
WHERE t.table_type = 'room'
  AND t.status = 'available'
  AND m.status = 'approved'
  AND (sqlc.narg(region_id)::BIGINT IS NULL OR m.region_id = sqlc.narg(region_id))
  AND (sqlc.narg(min_capacity)::SMALLINT IS NULL OR t.capacity >= sqlc.narg(min_capacity))
  AND (sqlc.narg(max_capacity)::SMALLINT IS NULL OR t.capacity <= sqlc.narg(max_capacity))
  AND (sqlc.narg(max_minimum_spend)::BIGINT IS NULL OR t.minimum_spend IS NULL OR t.minimum_spend <= sqlc.narg(max_minimum_spend))
  AND NOT EXISTS (
    SELECT 1 FROM table_reservations tr
    WHERE tr.table_id = t.id
      AND tr.reservation_date = sqlc.narg(reservation_date)::DATE
      AND tr.reservation_time = sqlc.narg(reservation_time)::TIME
      AND tr.status IN ('pending', 'paid', 'confirmed')
  );

-- name: ExploreNearbyRooms :many
-- 探索附近包间（无需指定预订日期时段），用于本地包间浏览流
-- 返回包间信息 + 商户信息 + 主图 + 近30天预订量
SELECT 
    t.id,
    t.merchant_id,
    t.table_no,
    t.table_type,
    t.capacity,
    t.description,
    t.minimum_spend,
    t.status,
    t.created_at,
    m.name as merchant_name,
    m.logo_url as merchant_logo,
    m.address as merchant_address,
    m.latitude as merchant_latitude,
    m.longitude as merchant_longitude,
    m.phone as merchant_phone,
    COALESCE((SELECT ti.image_url FROM table_images ti WHERE ti.table_id = t.id AND ti.is_primary = true LIMIT 1), '')::TEXT as primary_image,
    (SELECT COUNT(*) FROM table_reservations tr 
     WHERE tr.table_id = t.id 
       AND tr.reservation_date >= CURRENT_DATE - INTERVAL '30 days'
       AND tr.status IN ('paid', 'confirmed', 'completed')
    )::INT as monthly_reservations
FROM tables t
INNER JOIN merchants m ON t.merchant_id = m.id
WHERE t.table_type = 'room'
  AND t.status = 'available'
  AND m.status = 'approved'
  -- 按区域筛选
  AND (sqlc.narg(region_id)::BIGINT IS NULL OR m.region_id = sqlc.narg(region_id))
  -- 按人数筛选
  AND (sqlc.narg(min_capacity)::SMALLINT IS NULL OR t.capacity >= sqlc.narg(min_capacity))
  AND (sqlc.narg(max_capacity)::SMALLINT IS NULL OR t.capacity <= sqlc.narg(max_capacity))
  -- 按最低消费筛选
  AND (sqlc.narg(max_minimum_spend)::BIGINT IS NULL OR t.minimum_spend IS NULL OR t.minimum_spend <= sqlc.narg(max_minimum_spend))
ORDER BY monthly_reservations DESC, t.capacity
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);

-- name: CountExploreNearbyRooms :one
-- 统计可探索包间总数
SELECT COUNT(*) FROM tables t
INNER JOIN merchants m ON t.merchant_id = m.id
WHERE t.table_type = 'room'
  AND t.status = 'available'
  AND m.status = 'approved'
  AND (sqlc.narg(region_id)::BIGINT IS NULL OR m.region_id = sqlc.narg(region_id))
  AND (sqlc.narg(min_capacity)::SMALLINT IS NULL OR t.capacity >= sqlc.narg(min_capacity))
  AND (sqlc.narg(max_capacity)::SMALLINT IS NULL OR t.capacity <= sqlc.narg(max_capacity))
  AND (sqlc.narg(max_minimum_spend)::BIGINT IS NULL OR t.minimum_spend IS NULL OR t.minimum_spend <= sqlc.narg(max_minimum_spend));

-- name: SearchRoomsWithImage :many
-- 搜索包间（带主图），增强版 SearchRooms
SELECT 
    t.id,
    t.merchant_id,
    t.table_no,
    t.table_type,
    t.capacity,
    t.description,
    t.minimum_spend,
    t.qr_code_url,
    t.status,
    t.created_at,
    m.name as merchant_name,
    m.logo_url as merchant_logo,
    m.address as merchant_address,
    m.latitude as merchant_latitude,
    m.longitude as merchant_longitude,
    COALESCE((SELECT ti.image_url FROM table_images ti WHERE ti.table_id = t.id AND ti.is_primary = true LIMIT 1), '')::TEXT as primary_image
FROM tables t
INNER JOIN merchants m ON t.merchant_id = m.id
WHERE t.table_type = 'room'
  AND t.status = 'available'
  AND m.status = 'approved'
  -- 按区域筛选（可选）
  AND (sqlc.narg(region_id)::BIGINT IS NULL OR m.region_id = sqlc.narg(region_id))
  -- 按人数筛选
  AND (sqlc.narg(min_capacity)::SMALLINT IS NULL OR t.capacity >= sqlc.narg(min_capacity))
  AND (sqlc.narg(max_capacity)::SMALLINT IS NULL OR t.capacity <= sqlc.narg(max_capacity))
  -- 按最低消费筛选
  AND (sqlc.narg(max_minimum_spend)::BIGINT IS NULL OR t.minimum_spend IS NULL OR t.minimum_spend <= sqlc.narg(max_minimum_spend))
  -- 排除已在指定日期时段被预定的包间
  AND NOT EXISTS (
    SELECT 1 FROM table_reservations tr
    WHERE tr.table_id = t.id
      AND tr.reservation_date = sqlc.narg(reservation_date)::DATE
      AND tr.reservation_time = sqlc.narg(reservation_time)::TIME
      AND tr.status IN ('pending', 'paid', 'confirmed')
  )
ORDER BY t.capacity, t.minimum_spend NULLS FIRST
LIMIT sqlc.arg(page_size)
OFFSET sqlc.arg(page_offset);
