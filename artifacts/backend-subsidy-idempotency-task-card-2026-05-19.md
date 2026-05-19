# TASK-SUBSIDY-IDEMPOTENCY-001：补差创建/退回/取消改为原子 claim + 终态收敛

日期：2026-05-19
范围：仅 `locallife/` 后端。
风险等级：G3。原因：涉及补差外部副作用、重复提交、并发点击和终态回写。

## 状态备注

2026-05-19：微信平台收付通支付链路当前停用，本卡暂缓，不推进实现。相关风险已记录到 [.github/review/open-findings.md](../.github/review/open-findings.md)，待链路重启后再恢复。

## 目标

补差相关流程必须避免“先查再调外部接口”的竞态，保证重复提交、并发点击和重试都不会造成重复外呼或 500。

## 这张卡的边界

- 只改 `locallife/api/subsidy.go`、`locallife/db/query/subsidy_order.sql`、必要的 migration 和测试。
- 如果 SQL 或表结构变化，必须生成 `locallife/db/sqlc/subsidy_order.sql.go` 和 `locallife/db/sqlc/querier.go`。
- 不把这张卡和补差授权卡混在一起；先完成授权边界，再做 claim 语义。

## 先读这些文件

- `locallife/api/subsidy.go`：`createSubsidy`, `returnSubsidy`, `cancelSubsidy`
- `locallife/db/query/subsidy_order.sql`：现有 create / return / cancel 查询
- `locallife/db/migration/000161_create_subsidy_orders.up.sql`：现有表结构和状态机
- `locallife/api/subsidy_test.go`：现有重试、失败、成功测试
- `locallife/db/sqlc/models.go`：`SubsidyOrder` 字段

## 当前问题

- create 是 check-then-insert，没有原子 claim。
- return / cancel 先读，再直接调微信，再回写。
- `GetSubsidyOrderForUpdate` 已存在，但生产路径没用上。
- `cancelSubsidy` 没有单独的“取消中” claim 状态。
- `out_subsidy_no` 现在虽然唯一，但不足以表达“同一支付单 + 同一目标商户只有一条有效补差单”的完整 claim 语义。

## 必须实现的规则

1. create 必须变成原子 upsert / claim：
   - 同一支付单 + 同一目标商户只能形成一条有效补差单。
   - 竞争失败方应读回既有单，而不是直接 500。
2. return 必须先 claim `pending_return`，再调用外部接口。
3. cancel 必须有自己的 claim 语义。
   - 如果现有 schema 无法表达，请补最小必要字段或状态迁移。
4. claim 成功的那个请求才允许继续调用微信接口。
5. 外部接口成功/失败后的本地终态更新必须和外部单号保持一致。
6. 不允许把外部副作用放到纯读之后的“碰碰运气”流程里。

## 推荐的最小持久化方向

- 为 `subsidy_orders` 增加能够表达有效唯一性的约束，使 `(payment_order_id, sub_mch_id)` 成为一条有效补差单的稳定边界。
- create 走单条写入 + 冲突回读，不要先 `GetSubsidyOrderByOutSubsidyNo` 再插入。
- return 走条件更新或 `FOR UPDATE` 语义，只有成功 claim 的请求才允许调用微信。
- cancel 如果现有 `status` 不能表达 claim，请补最小必要的 `cancel_status` 或等价字段，不要把 final `canceled` 当 claim state 用。

## 明确不做

- 不把并发问题甩给前端重复确认。
- 不允许“失败后直接空返回”。
- 不把未知状态伪装成成功。
- 不把 create / return / cancel 的失败语义直接透出 provider raw。

## 测试要求

- 并发同 key 只形成一个有效补差单。
- return / cancel 重试不会重复外呼。
- 失败后可按既定状态重放，但不能把未知状态伪装成成功。
- 失败 / 冲突返回应是稳定中文，不得直接暴露 provider 原始错误。

建议新增/调整这些测试名：

- `TestCreateSubsidy_RetryFailedOrderReusesExistingRecord`
- `TestCreateSubsidy_AcceptsEmptyWechatBody`
- `TestReturnSubsidy_RetryFailedReturnReusesOriginalOutOrderNo`
- `TestCancelSubsidy_NilResponseDoesNotPanic`
- `TestCancelSubsidy_RecordsAcceptedCommand`

## 验证命令

```bash
cd locallife
make sqlc
go test ./api -run 'Test(CreateSubsidy|ReturnSubsidy|CancelSubsidy)' -count=1
make check-generated
```

## 停止条件

- 如果 create 需要新增唯一约束或 claim 列，先把 migration + sqlc 闭合，再做 API 逻辑。
- 不要在同一轮里顺手把 command 审计或前端文案也改掉。
