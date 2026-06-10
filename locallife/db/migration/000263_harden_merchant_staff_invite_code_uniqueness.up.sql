WITH ranked_bind_codes AS (
    SELECT
        id,
        row_number() OVER (
            PARTITION BY bind_code
            ORDER BY bind_code_expires_at DESC NULLS LAST, updated_at DESC NULLS LAST, id DESC
        ) AS bind_code_rank
    FROM merchants
    WHERE bind_code IS NOT NULL
      AND bind_code <> ''
      AND deleted_at IS NULL
)
UPDATE merchants AS merchant
SET bind_code = NULL,
    bind_code_expires_at = NULL,
    updated_at = now()
FROM ranked_bind_codes
WHERE merchant.id = ranked_bind_codes.id
  AND ranked_bind_codes.bind_code_rank > 1;

CREATE UNIQUE INDEX IF NOT EXISTS merchants_active_bind_code_uidx
ON merchants(bind_code)
WHERE bind_code IS NOT NULL
  AND bind_code <> ''
  AND deleted_at IS NULL;
