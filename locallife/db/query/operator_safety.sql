-- name: CreateSafetyReport :one
INSERT INTO safety_reports (
    reporter_id,
    region_id,
    title,
    description,
    level,
    merchant_ids,
    images,
    status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING *;

-- name: GetSafetyReport :one
SELECT * FROM safety_reports
WHERE id = $1 LIMIT 1;

-- name: ListSafetyReportsByRegion :many
SELECT * FROM safety_reports
WHERE region_id = $1
ORDER BY created_at DESC, id DESC
LIMIT $2 OFFSET $3;

-- name: CountSafetyReportsByRegion :one
SELECT COUNT(*) FROM safety_reports
WHERE region_id = $1;

-- name: ListSafetyReportsByRegionAndStatus :many
SELECT * FROM safety_reports
WHERE region_id = $1
    AND status = $2
ORDER BY created_at DESC, id DESC
LIMIT $3 OFFSET $4;

-- name: CountSafetyReportsByRegionAndStatus :one
SELECT COUNT(*) FROM safety_reports
WHERE region_id = $1
    AND status = $2;

-- name: UpdateSafetyReportStatus :one
UPDATE safety_reports
SET 
    status = $2,
    resolution_notes = COALESCE(sqlc.narg(resolution_notes), resolution_notes),
    updated_at = now()
WHERE id = $1
RETURNING *;

-- name: SuspendRegion :exec
UPDATE regions
SET status = 'suspended', updated_at = now()
WHERE id = $1;
