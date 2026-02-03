# Phase0 性能/索引核对清单（草案）

## 目标
- 明确关键查询与索引覆盖，降低慢查询风险。

## 关键表与查询（初版）
- 订单相关：order, order_items, order_status_log
- 支付相关：payment_orders, refunds
- 配送相关：deliveries, delivery_orders
- 商户/菜品：merchants, dishes, inventory
- 平台统计：platform_stats

## 核对项
- [ ] 订单检索条件（merchant_id/region_id/created_at）索引覆盖
- [ ] 支付与退款按订单号/外部单号检索索引覆盖
- [ ] 配送按 rider_id/状态/时间范围检索索引覆盖
- [ ] 菜品/库存按 merchant_id/状态检索索引覆盖
- [ ] 平台统计按 region_id/日期范围检索索引覆盖

## 慢查询治理（建议）
- 对 top N 慢查询添加 explain 与索引建议记录
- 对高基数条件（region_id + status + created_at）优先复合索引
- 对分页查询确保使用可索引排序字段

## 备注
- 本清单用于 Phase0 基线梳理，实际索引需结合生产慢查询日志与压测结果调整。
