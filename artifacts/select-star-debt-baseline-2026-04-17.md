# SELECT * 治理状态

日期：2026-04-17

## 当前状态

- 当前 `SELECT *` 门禁已经升级为“全仓扫描 + baseline allowlist”，不再只依赖 touched block。
- 历史债务台账位于 [.github/sqlguard/select_star_baseline.txt](../.github/sqlguard/select_star_baseline.txt)，当前剩余 0 条 query block，分布在 0 个 SQL 文件中。
- bare `SELECT *` 历史债务已完成清零；后续新增违规将直接由 guard 拦截。

## 已完成治理

- 已升级 [.github/scripts/backend_sql_guard.sh](../.github/scripts/backend_sql_guard.sh) 为全仓 bare `SELECT *` baseline 校验，并保留其余 diff-based 规则。
- 已补齐 [.github/scripts/test_backend_sql_guard.sh](../.github/scripts/test_backend_sql_guard.sh) 的 baseline 自测场景。
- 已更新 [.github/standards/backend/SQL_STANDARDS.md](../.github/standards/backend/SQL_STANDARDS.md)，使标准描述与当前 guard 行为一致。
- 已按“只做机械显式列展开、不改 SQL 语义”的原则，完成历史裸 `SELECT *` 债务批量清理。
- 最终清零批次包含：
   - [locallife/db/query/abnormal_stats.sql](../locallife/db/query/abnormal_stats.sql)
   - [locallife/db/query/agreement.sql](../locallife/db/query/agreement.sql)
   - [locallife/db/query/auto_tag.sql](../locallife/db/query/auto_tag.sql)
   - [locallife/db/query/combo.sql](../locallife/db/query/combo.sql)
   - [locallife/db/query/order_status_log.sql](../locallife/db/query/order_status_log.sql)
   - [locallife/db/query/platform_alert_event.sql](../locallife/db/query/platform_alert_event.sql)
   - [locallife/db/query/reservation_inventory.sql](../locallife/db/query/reservation_inventory.sql)
   - [locallife/db/query/reservation_item.sql](../locallife/db/query/reservation_item.sql)
   - [locallife/db/query/reservation_payment.sql](../locallife/db/query/reservation_payment.sql)
   - [locallife/db/query/wechat_access_token.sql](../locallife/db/query/wechat_access_token.sql)
- 清零前最后几轮收缩批次包含：
   - [locallife/db/query/merchant_payment_config.sql](../locallife/db/query/merchant_payment_config.sql)
   - [locallife/db/query/merchant_settlement_adjustment.sql](../locallife/db/query/merchant_settlement_adjustment.sql)
   - [locallife/db/query/merchant_staff.sql](../locallife/db/query/merchant_staff.sql)
   - [locallife/db/query/merchant_system_label.sql](../locallife/db/query/merchant_system_label.sql)
   - [locallife/db/query/operator_region_application.sql](../locallife/db/query/operator_region_application.sql)
   - [locallife/db/query/operator.sql](../locallife/db/query/operator.sql)
   - [locallife/db/query/order_display_config.sql](../locallife/db/query/order_display_config.sql)
   - [locallife/db/query/order_item.sql](../locallife/db/query/order_item.sql)
   - [locallife/db/query/rule_hits.sql](../locallife/db/query/rule_hits.sql)
   - [locallife/db/query/web_login_session.sql](../locallife/db/query/web_login_session.sql)
   - [locallife/db/query/merchant_application.sql](../locallife/db/query/merchant_application.sql)
   - [locallife/db/query/merchant_boss.sql](../locallife/db/query/merchant_boss.sql)
   - [locallife/db/query/merchant_membership_settings.sql](../locallife/db/query/merchant_membership_settings.sql)
   - [locallife/db/query/operator_core.sql](../locallife/db/query/operator_core.sql)
   - [locallife/db/query/operator_region.sql](../locallife/db/query/operator_region.sql)

## 已验证结果

- `make sqlc` 在最终清零批次后通过。
- 最终清零批次相关文件 grep 已确认不再包含 bare `SELECT *`。
- 最终清零批次相关 SQL 源文件与 sqlc 生成文件 `get_errors` 均为无错误。
- 最终定向测试通过，合计 171 个用例，覆盖：`api/agreement_test.go`、`db/sqlc/combo_test.go`、`api/combo_test.go`、`db/sqlc/tx_order_status_test.go`、`api/notification_test.go`、`db/sqlc/tx_reservation_inventory_test.go`、`db/sqlc/tx_reservation_test.go`、`db/sqlc/wechat_access_token_test.go`。
- 对缺少直测的 singleton 查询（如 abnormal_stats、auto_tag），已使用 sqlc 生成链路、静态无错和仓库级 guard 回放作为收口验证。

## 结论

- bare `SELECT *` 历史债务已从 62 条 query block / 35 个 SQL 文件收缩至 0 / 0。
- 当前 baseline 已清空，后续无需继续做历史债务收缩，只需保持 guard 生效并阻止新增违规。