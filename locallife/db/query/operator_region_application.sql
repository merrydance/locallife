-- name: CreateOperatorRegionApplication :one
-- 运营商提交区域扩展申请
INSERT INTO operator_region_applications (
    operator_id,
    region_id
) VALUES ($1, $2)
RETURNING *;

-- name: GetOperatorRegionApplication :one
SELECT id, operator_id, region_id, status, reject_reason, created_at, updated_at FROM operator_region_applications
WHERE id = $1 LIMIT 1;

-- name: GetOperatorRegionApplicationByOperatorAndRegion :one
SELECT id, operator_id, region_id, status, reject_reason, created_at, updated_at FROM operator_region_applications
WHERE operator_id = $1 AND region_id = $2 LIMIT 1;

-- name: ListOperatorRegionApplicationsByOperator :many
-- 列出某运营商的所有区域扩展申请
SELECT ora.*, r.name AS region_name, r.code AS region_code
FROM operator_region_applications ora
JOIN regions r ON ora.region_id = r.id
WHERE ora.operator_id = $1
ORDER BY ora.created_at DESC;

-- name: ListPendingRegionApplications :many
-- 管理后台：列出所有待审核的区域扩展申请
SELECT ora.*, r.name AS region_name, r.code AS region_code,
       o.name AS operator_name, o.contact_name, o.contact_phone
FROM operator_region_applications ora
JOIN regions r ON ora.region_id = r.id
JOIN operators o ON ora.operator_id = o.id
WHERE ora.status = 'pending'
ORDER BY ora.created_at ASC
LIMIT $1 OFFSET $2;

-- name: CountPendingRegionApplications :one
SELECT COUNT(*) FROM operator_region_applications WHERE status = 'pending';

-- name: ListAllRegionApplicationsAdmin :many
-- 管理后台：列出所有区域扩展申请（支持状态过滤，NULL 表示不过滤）
SELECT ora.*, r.name AS region_name, r.code AS region_code,
       o.name AS operator_name, o.contact_name, o.contact_phone
FROM operator_region_applications ora
JOIN regions r ON ora.region_id = r.id
JOIN operators o ON ora.operator_id = o.id
WHERE (sqlc.narg(status)::text IS NULL OR ora.status = sqlc.narg(status))
ORDER BY ora.created_at DESC
LIMIT $1 OFFSET $2;

-- name: CountAllRegionApplicationsAdmin :one
-- 管理后台：统计区域扩展申请数量（支持状态过滤，NULL 表示不过滤）
SELECT COUNT(*) FROM operator_region_applications
WHERE (sqlc.narg(status)::text IS NULL OR status = sqlc.narg(status));

-- name: ApproveOperatorRegionApplication :one
-- 审批通过区域扩展申请
UPDATE operator_region_applications
SET status = 'approved', updated_at = now()
WHERE id = $1
RETURNING *;

-- name: RejectOperatorRegionApplication :one
-- 审批拒绝区域扩展申请
UPDATE operator_region_applications
SET status = 'rejected', reject_reason = $2, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: DeleteOperatorRegionApplication :exec
DELETE FROM operator_region_applications
WHERE id = $1;
