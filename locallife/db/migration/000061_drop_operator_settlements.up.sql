-- 删除冗余的 operator_settlements 表
-- 
-- 原因：
-- 1. 微信电商分账系统支持实时多方分账，每笔订单支付时自动分配给平台+运营商
-- 2. 实际分账数据已存储在 profit_sharing_orders 表（status='finished'）
-- 3. operator_settlements 只是一个手动创建的快照，"确认结算"只是改状态，不触发实际支付
-- 4. 运营商佣金统计可以从 profit_sharing_orders 实时计算，无需冗余表

DROP TABLE IF EXISTS operator_settlements;
