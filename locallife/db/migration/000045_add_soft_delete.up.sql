-- 软删除机制：为核心业务表添加 deleted_at 字段
-- 这些表被订单等历史数据引用，不能硬删除

-- 1. 菜品表（被 order_items.dish_id 引用）
ALTER TABLE dishes ADD COLUMN deleted_at TIMESTAMPTZ DEFAULT NULL;
CREATE INDEX idx_dishes_deleted_at ON dishes(deleted_at) WHERE deleted_at IS NULL;

-- 2. 套餐表（被 order_items.combo_id 引用）
ALTER TABLE combo_sets ADD COLUMN deleted_at TIMESTAMPTZ DEFAULT NULL;
CREATE INDEX idx_combo_sets_deleted_at ON combo_sets(deleted_at) WHERE deleted_at IS NULL;

-- 3. 商户表（被大量表引用）
ALTER TABLE merchants ADD COLUMN deleted_at TIMESTAMPTZ DEFAULT NULL;
CREATE INDEX idx_merchants_deleted_at ON merchants(deleted_at) WHERE deleted_at IS NULL;

-- 4. 菜品分类表（被 dishes.category_id 引用）
ALTER TABLE dish_categories ADD COLUMN deleted_at TIMESTAMPTZ DEFAULT NULL;
CREATE INDEX idx_dish_categories_deleted_at ON dish_categories(deleted_at) WHERE deleted_at IS NULL;

-- 5. 优惠券模板表（被 user_vouchers 引用）
ALTER TABLE vouchers ADD COLUMN deleted_at TIMESTAMPTZ DEFAULT NULL;
CREATE INDEX idx_vouchers_deleted_at ON vouchers(deleted_at) WHERE deleted_at IS NULL;

-- 6. 满减规则表（可能被订单快照引用）
ALTER TABLE discount_rules ADD COLUMN deleted_at TIMESTAMPTZ DEFAULT NULL;
CREATE INDEX idx_discount_rules_deleted_at ON discount_rules(deleted_at) WHERE deleted_at IS NULL;
