-- ============================================
-- 食材管理查询 (Ingredient Queries)
-- ============================================

-- name: CreateIngredient :one
INSERT INTO ingredients (
  name,
  is_system,
  category,
  is_allergen,
  created_by
) VALUES (
  $1, $2, $3, $4, $5
) RETURNING *;

-- name: GetIngredient :one
SELECT * FROM ingredients
WHERE id = $1 LIMIT 1;

-- name: ListIngredients :many
SELECT * FROM ingredients
WHERE 
  (sqlc.narg('is_system')::boolean IS NULL OR is_system = sqlc.narg('is_system'))
  AND (sqlc.narg('category')::text IS NULL OR category = sqlc.narg('category'))
  AND (sqlc.narg('is_allergen')::boolean IS NULL OR is_allergen = sqlc.narg('is_allergen'))
ORDER BY is_system DESC, name ASC
LIMIT $1 OFFSET $2;

-- name: SearchIngredients :many
SELECT * FROM ingredients
WHERE name ILIKE '%' || $1 || '%'
ORDER BY is_system DESC, name ASC
LIMIT $2 OFFSET $3;

-- name: UpdateIngredient :one
UPDATE ingredients
SET
  name = COALESCE(sqlc.narg('name'), name),
  category = COALESCE(sqlc.narg('category'), category),
  is_allergen = COALESCE(sqlc.narg('is_allergen'), is_allergen)
WHERE id = $1
RETURNING *;

-- name: DeleteIngredient :exec
DELETE FROM ingredients
WHERE id = $1;

-- name: CountIngredients :one
SELECT COUNT(*) FROM ingredients
WHERE 
  (sqlc.narg('is_system')::boolean IS NULL OR is_system = sqlc.narg('is_system'))
  AND (sqlc.narg('category')::text IS NULL OR category = sqlc.narg('category'));
