-- Rollback agreements v1.2.0-legal

-- 1) Remove the newly inserted versions
DELETE FROM agreements
WHERE version = 'v1.2.0-legal'
  AND type IN (
    'MERCHANT_AGREEMENT',
    'USER_AGREEMENT',
    'CONSUMER_RIGHTS',
    'RIDER_AGREEMENT',
    'PICKUP_PROXY_AGREEMENT',
    'OPERATOR_AGREEMENT',
    'PRIVACY_POLICY'
  );

-- 2) Re-activate previous versions for the original types (best-effort)
UPDATE agreements
SET is_active = true,
    updated_at = CURRENT_TIMESTAMP
WHERE version = 'v1.1.0-legal'
  AND type IN (
    'MERCHANT_AGREEMENT',
    'USER_AGREEMENT',
    'CONSUMER_RIGHTS',
    'RIDER_AGREEMENT',
    'PICKUP_PROXY_AGREEMENT',
    'OPERATOR_AGREEMENT'
  );

DROP INDEX IF EXISTS idx_agreements_one_active_per_type;
