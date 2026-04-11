-- Relax notifications.type / related_type constraints
--
-- Rationale:
-- - Application code uses event-style types such as: order_completed, delivery_delay, delivery_exception, appeal, etc.
-- - The original schema only allowed a small fixed set, causing SQLSTATE 23514 at runtime.
--
-- Production-grade approach:
-- - Keep basic validation (non-empty, predictable format)
-- - Avoid tight enums that require frequent migrations for new event types

ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_type_check;

ALTER TABLE notifications
    ADD CONSTRAINT notifications_type_check
    CHECK (type ~ '^[a-z][a-z0-9_]*$');

ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_related_type_check;

ALTER TABLE notifications
    ADD CONSTRAINT notifications_related_type_check
    CHECK (related_type IS NULL OR related_type ~ '^[a-z][a-z0-9_]*$');

COMMENT ON COLUMN notifications.type IS '通知类型（事件标识，小写字母/数字/下划线，如 order_completed, delivery_delay 等）';
COMMENT ON COLUMN notifications.related_type IS '关联实体类型（小写字母/数字/下划线，如 order, delivery, appeal 等）';
