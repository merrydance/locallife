-- Rollback: restore original strict enums for notifications
-- Warning: this may fail if existing rows contain non-enumerated values.

ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_type_check;

ALTER TABLE notifications
    ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('order', 'payment', 'delivery', 'system', 'food_safety'));

ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_related_type_check;

ALTER TABLE notifications
    ADD CONSTRAINT notifications_related_type_check
    CHECK (related_type IN ('order', 'payment', 'delivery', 'reservation', 'merchant'));

COMMENT ON COLUMN notifications.type IS '通知类型：order/payment/delivery/system/food_safety';
COMMENT ON COLUMN notifications.related_type IS '关联实体类型，用于跳转';
