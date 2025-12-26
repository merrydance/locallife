-- 逆向操作，顺序相反

-- 删除订单展示配置表
DROP TABLE IF EXISTS order_display_configs;

-- 删除打印日志表
DROP TABLE IF EXISTS print_logs;

-- 删除云打印机配置表
DROP TABLE IF EXISTS cloud_printers;

-- 删除订单状态变更日志表
DROP TABLE IF EXISTS order_status_logs;

-- 删除订单明细表
DROP TABLE IF EXISTS order_items;

-- 删除订单主表
DROP TABLE IF EXISTS orders;
