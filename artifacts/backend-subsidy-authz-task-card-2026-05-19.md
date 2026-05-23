# TASK-SUBSIDY-AUTHZ-001：补差入口增加支付单对象级授权

日期：2026-05-19
范围：仅 `locallife/` 后端。
风险等级：G3。原因：涉及补差资金流、运营商权限和支付单对象级授权。

## 状态备注

2026-05-19：微信平台收付通支付链路当前停用，本卡暂缓，不推进实现。相关风险已记录到 [.github/review/open-findings.md](../.github/review/open-findings.md)，待链路重启后再恢复。

## 目标

`createSubsidy`、`returnSubsidy`、`cancelSubsidy` 只能处理当前 operator 管辖范围内的支付单。`merchant_id` 不能再当作权限边界。

## 这张卡的边界

- 只改 `locallife/api/subsidy.go` 和对应测试。
- 如果需要新增一个很小的授权 helper，可放在 `locallife/api/subsidy_auth.go`，但不要把权限逻辑搬进一个大 service。
- 不改微信补差 provider 协议，不改 operator 登录态，不改支付单 schema。

## 先读这些文件

- `locallife/api/subsidy.go`：`createSubsidy`, `returnSubsidy`, `cancelSubsidy`
- `locallife/api/delivery_fee.go`：`checkOperatorManagesRegion`, `listManagedOperatorRegionIDs`
- `locallife/logic/claim_recovery.go`：operator 区域归属的现成模式
- `locallife/db/query/payment_order.sql`：支付单可用字段
- `locallife/db/query/merchant.sql`：商户区域字段
- `locallife/db/query/operator_region.sql`：operator-region 归属关系
- `locallife/api/subsidy_test.go`：现有成功与失败测试

## 当前问题

- 路由只校验了 operator 身份，没有做支付单对象级授权。
- `createSubsidy` 直接信任 body 里的 `merchant_id` 去取支付配置。
- `returnSubsidy` 和 `cancelSubsidy` 只按 `payment_order_id` 找补差单，没有再校验 operator 是否对这笔支付单有管理权。
- 当前实现里，body merchant 能影响授权来源，这个边界是错的。

## 必须实现的规则

1. 补差操作的授权来源必须是支付单自身的真实归属，而不是请求体。
2. `createSubsidy` 先加载支付单，再从支付单解析出真实业务对象：
   - `paymentOrder.OrderID.Valid` 时，加载 `order`，用 `order.MerchantID` 作为真实归属。
   - `paymentOrder.ReservationID.Valid` 时，加载 `reservation`，用 `reservation.MerchantID` 作为真实归属。
   - 如果两者都没有，直接拒绝。
3. 解析出真实商户后，再比对当前 operator 是否管理该商户所在区域。
4. `req.MerchantID` 只能作为“请求目标商户”参与一致性校验，不能作为授权来源。
5. `returnSubsidy` 和 `cancelSubsidy` 也要先回溯到同一支付单真实归属，再做相同的 operator 区域校验。
6. 任一越权场景必须返回稳定的 403 或等价拒绝，不得继续调用微信补差接口。
7. 不得把授权失败降级成空结果、成功结果或静默 no-op。

## 明确不做

- 不改微信补差 provider 协议。
- 不改 operator 登录态本身。
- 不把 body merchant 当成新的权限边界。
- 不把这张卡和幂等卡合并。

## 测试要求

- `createSubsidy`：管辖区域内成功。
- `createSubsidy`：跨区域 / 跨商户拒绝。
- `createSubsidy`：body merchant 与支付单真实归属不一致时拒绝。
- `returnSubsidy` / `cancelSubsidy`：同样覆盖越权拒绝。

建议新增/调整这些测试名：

- `TestCreateSubsidy_RetryFailedOrderReusesExistingRecord`
- `TestCreateSubsidy_AcceptsEmptyWechatBody`
- `TestCreateSubsidy_RejectsCrossRegionMerchant`
- `TestReturnSubsidy_RejectsCrossRegionOperator`
- `TestCancelSubsidy_RejectsCrossRegionOperator`

## 验证命令

```bash
cd locallife
go test ./api -run 'Test(CreateSubsidy|ReturnSubsidy|CancelSubsidy)' -count=1
```

## 停止条件

- 如果支付单归属必须从 order/reservation 两条链路回溯，先把这条授权链路闭合，再决定是否需要新增一个小 helper。
- 不要因为看到了补差幂等问题，就顺手改 create/return/cancel 的 claim 语义。
