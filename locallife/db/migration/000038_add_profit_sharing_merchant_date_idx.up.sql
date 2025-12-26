-- 为商户财务查询添加联合索引
-- 高频查询: 按商户ID + 日期范围查询分账订单
CREATE INDEX IF NOT EXISTS idx_profit_sharing_orders_merchant_created 
    ON profit_sharing_orders(merchant_id, created_at);

-- 为按状态+日期范围查询添加索引（用于结算记录筛选）
CREATE INDEX IF NOT EXISTS idx_profit_sharing_orders_merchant_status_created 
    ON profit_sharing_orders(merchant_id, status, created_at);
