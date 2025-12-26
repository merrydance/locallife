-- 删除M12相关表和索引

DROP INDEX IF EXISTS idx_order_items_dish_created;
DROP INDEX IF EXISTS idx_orders_user_merchant_created;
DROP INDEX IF EXISTS idx_orders_merchant_created_status;

DROP TABLE IF EXISTS operator_settlements;
