CREATE TABLE IF NOT EXISTS merchant_offline_customers (
    id BIGSERIAL PRIMARY KEY,
    merchant_id BIGINT NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
    contact_name TEXT NOT NULL,
    contact_phone TEXT NOT NULL,
    source TEXT NOT NULL DEFAULT 'merchant',
    created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ,
    CONSTRAINT merchant_offline_customers_contact_name_chk CHECK (btrim(contact_name) <> ''),
    CONSTRAINT merchant_offline_customers_contact_phone_chk CHECK (contact_phone = btrim(contact_phone) AND contact_phone <> ''),
    CONSTRAINT merchant_offline_customers_source_chk CHECK (source IN ('phone', 'walkin', 'merchant'))
);

CREATE UNIQUE INDEX IF NOT EXISTS merchant_offline_customers_merchant_phone_uidx
    ON merchant_offline_customers(merchant_id, contact_phone);

CREATE INDEX IF NOT EXISTS merchant_offline_customers_merchant_idx
    ON merchant_offline_customers(merchant_id);

ALTER TABLE table_reservations
    ADD COLUMN IF NOT EXISTS offline_customer_id BIGINT REFERENCES merchant_offline_customers(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS created_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS table_reservations_offline_customer_id_idx
    ON table_reservations(offline_customer_id);

UPDATE table_reservations
SET contact_phone = btrim(contact_phone),
    contact_name = COALESCE(NULLIF(btrim(contact_name), ''), 'offline customer')
WHERE source IS NOT NULL
  AND btrim(source) <> ''
  AND btrim(source) <> 'online'
  AND btrim(contact_phone) <> ''
  AND (
      contact_phone <> btrim(contact_phone)
      OR contact_name <> COALESCE(NULLIF(btrim(contact_name), ''), 'offline customer')
  );

WITH historical_offline_rows AS (
    SELECT
        id,
        merchant_id,
        COALESCE(NULLIF(btrim(contact_name), ''), 'offline customer') AS contact_name,
        btrim(contact_phone) AS contact_phone,
        CASE
            WHEN btrim(source) IN ('phone', 'walkin', 'merchant') THEN btrim(source)
            ELSE 'merchant'
        END AS source,
        user_id AS operator_user_id,
        created_at,
        COALESCE(updated_at, created_at) AS seen_at
    FROM table_reservations
    WHERE source IS NOT NULL
      AND btrim(source) <> ''
      AND btrim(source) <> 'online'
      AND btrim(contact_phone) <> ''
),
historical_offline_seen AS (
    SELECT
        merchant_id,
        contact_phone,
        MIN(created_at) AS first_seen_at,
        MAX(seen_at) AS last_seen_at
    FROM historical_offline_rows
    GROUP BY merchant_id, contact_phone
),
historical_offline_first AS (
    SELECT DISTINCT ON (merchant_id, contact_phone)
        merchant_id,
        contact_phone,
        operator_user_id
    FROM historical_offline_rows
    ORDER BY merchant_id, contact_phone, created_at ASC, id ASC
),
historical_offline_latest AS (
    SELECT DISTINCT ON (merchant_id, contact_phone)
        merchant_id,
        contact_name,
        contact_phone,
        source,
        operator_user_id
    FROM historical_offline_rows
    ORDER BY merchant_id, contact_phone, seen_at DESC, created_at DESC, id DESC
)
INSERT INTO merchant_offline_customers (
    merchant_id,
    contact_name,
    contact_phone,
    source,
    created_by_user_id,
    updated_by_user_id,
    first_seen_at,
    last_seen_at,
    created_at,
    updated_at
)
SELECT
    latest.merchant_id,
    latest.contact_name,
    latest.contact_phone,
    latest.source,
    first.operator_user_id,
    latest.operator_user_id,
    seen.first_seen_at,
    seen.last_seen_at,
    seen.first_seen_at,
    seen.last_seen_at
FROM historical_offline_latest latest
JOIN historical_offline_seen seen
  ON seen.merchant_id = latest.merchant_id
 AND seen.contact_phone = latest.contact_phone
JOIN historical_offline_first first
  ON first.merchant_id = latest.merchant_id
 AND first.contact_phone = latest.contact_phone
ON CONFLICT (merchant_id, contact_phone) DO UPDATE
SET contact_name = EXCLUDED.contact_name,
    source = EXCLUDED.source,
    created_by_user_id = COALESCE(merchant_offline_customers.created_by_user_id, EXCLUDED.created_by_user_id),
    updated_by_user_id = EXCLUDED.updated_by_user_id,
    first_seen_at = LEAST(merchant_offline_customers.first_seen_at, EXCLUDED.first_seen_at),
    last_seen_at = GREATEST(merchant_offline_customers.last_seen_at, EXCLUDED.last_seen_at),
    updated_at = now();

UPDATE table_reservations tr
SET offline_customer_id = moc.id,
    created_by_user_id = COALESCE(tr.created_by_user_id, tr.user_id)
FROM merchant_offline_customers moc
WHERE tr.source IS NOT NULL
  AND btrim(tr.source) <> ''
  AND btrim(tr.source) <> 'online'
  AND tr.offline_customer_id IS NULL
  AND moc.merchant_id = tr.merchant_id
  AND moc.contact_phone = btrim(tr.contact_phone);
