UPDATE wechat_notifications
SET processed_at = NOW()
WHERE processed_at IS NULL;

ALTER TABLE wechat_notifications
    ALTER COLUMN processed_at SET DEFAULT NOW(),
    ALTER COLUMN processed_at SET NOT NULL;
