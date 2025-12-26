-- M12: 商户与运营商BI系统
-- 设计思路: 统计数据全部实时计算,避免数据同步复杂度
-- 只保留operator_settlements表,因为它是业务实体(涉及结算状态)

-- 1. 运营商结算表
CREATE TABLE operator_settlements (
    id BIGSERIAL PRIMARY KEY,
    region_id BIGINT NOT NULL REFERENCES regions(id) ON DELETE CASCADE,
    month TEXT NOT NULL,
    
    -- 结算统计
    total_orders INTEGER NOT NULL,
    total_gmv BIGINT NOT NULL,
    platform_commission BIGINT NOT NULL,
    operator_commission BIGINT NOT NULL,
    
    -- 结算状态
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'settled', 'paid')),
    settled_at TIMESTAMPTZ,
    
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ,
    
    CONSTRAINT operator_settlements_region_month_unique UNIQUE (region_id, month)
);

CREATE INDEX idx_operator_settlements_region_id ON operator_settlements(region_id);
CREATE INDEX idx_operator_settlements_status ON operator_settlements(status);
CREATE INDEX idx_operator_settlements_month ON operator_settlements(month);

-- 自动更新updated_at触发器
CREATE TRIGGER update_operator_settlements_updated_at
    BEFORE UPDATE ON operator_settlements
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- 2. 优化orders表索引,支持BI实时查询
-- 确保以下索引已存在(如果已有则跳过)
CREATE INDEX IF NOT EXISTS idx_orders_merchant_created_status 
    ON orders(merchant_id, created_at, status);

CREATE INDEX IF NOT EXISTS idx_orders_user_merchant_created 
    ON orders(user_id, merchant_id, created_at) 
    WHERE status IN ('delivered', 'completed');

-- 3. 优化order_items表索引,支持菜品销量统计
CREATE INDEX IF NOT EXISTS idx_order_items_dish_created 
    ON order_items(dish_id, created_at);
