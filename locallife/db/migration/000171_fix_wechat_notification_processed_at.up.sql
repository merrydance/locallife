ALTER TABLE wechat_notifications
    ALTER COLUMN processed_at DROP DEFAULT,
    ALTER COLUMN processed_at DROP NOT NULL;
