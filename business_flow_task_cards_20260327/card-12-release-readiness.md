# CARD-12 发布前回归与上线评审复核

状态：进行中

优先级：P0

所属阶段：Phase 4

## 问题目标

在全部整改完成后，统一执行回归、复核和上线前放行检查，避免“代码改了但发布门槛没收口”。

## 任务内容

- [x] 核对 CARD-01 到 CARD-11 的完成状态。
- [x] 汇总自动化测试结果和未覆盖风险。
- [ ] 手工回归 P0 主场景：押金抵扣、预约确认、开台转台、支付回调重试。
- [ ] 手工回归 P1 主场景：账单组金额展示、预检身份展示。
- [x] 输出最终评审结论：可上线 / 有条件上线 / 不可上线。

## 放行条件

- [x] CARD-01 到 CARD-06 全部完成。
- [x] 若 CARD-07 到 CARD-09 未完成，则堂食分账相关入口必须明确关闭。
- [x] 若 CARD-10 到 CARD-11 需要联动前端，则前后端版本必须一起发。

## 验证要求

- [x] 汇总测试命令和结果。
- [ ] 按 [business_flow_task_cards_20260327/manual-regression-checklist.md](business_flow_task_cards_20260327/manual-regression-checklist.md) 执行并记录手工回归人、时间和结论。
- [x] 对剩余风险给出上线后监控点。

## 当前核对结果

- CARD-01 到 CARD-08、CARD-10、CARD-11 已完成。
- CARD-09 代码回归已完成，但手工场景回归未完成，因此仍处于进行中。
- 堂食分账相关入口未关闭，因为 CARD-07 到 CARD-08 已完成，CARD-09 剩余项仅为手工验证而非功能缺口。

## 自动化验证汇总

- `go test ./db/sqlc -run 'TestCreateOrderTx_BillingGroupAggregation|TestGetBillingGroupAmounts_ExcludesCancelledAndReplacedOrders|TestReplaceOrderTx_ReLinksReplacementOrderToBillingGroup'`：通过。
- `go test ./api -run 'TestCreateBillingGroupAPI_UsesAggregatedAmounts|TestListBillingGroupsAPI_UsesAggregatedAmounts|TestJoinBillingGroupAPI_UsesAggregatedAmounts'`：通过。
- `go test ./api -run 'TestPrecheckDiningSessionAPI|TestOpenDiningSessionAPI_UsesAggregatedBillingGroupAmounts'`：通过。
- `go test ./logic -run 'TestReplaceReservationOrder_'`：通过。
- 本轮之前已完成并通过的关键回归包括：
	- `go test ./db/sqlc -run 'TestConfirmReservationTx|TestCompleteReservationTx|TestCancelReservationTx|TestMarkReservationNoShowTx|TestConfirmReservationTxDoesNotReserveTableNearReservationTime'`
	- `go test ./db/sqlc -run 'TestTransferDiningSessionTableTx'`
	- `go test ./logic -run 'TestResolveReservationDepositDeduction|TestComputeOrderTotals'`
	- `go test ./logic -run 'TestPrecheckDiningSession'`
	- `go test ./api -run 'TestPaymentCallback'`
	- `go test ./worker -run 'TestWechatNotificationRecoveryScheduler'`

补充说明：

- `TestPrecheckDiningSessionAPI` 已覆盖预检接口的 transport 语义，包括“本人预约”判定与商户查看时 `is_reservation_owner=false` 的返回口径。
- `TestOpenDiningSessionAPI_UsesAggregatedBillingGroupAmounts` 已覆盖开台接口响应读取运行时聚合账单组金额，而非回退到 `billing_groups.total_amount/paid_amount` 持久化字段。
- `TestJoinBillingGroupAPI_UsesAggregatedAmounts` 已覆盖拼桌/加入账单组接口返回聚合金额，减少“加入后金额仍显示旧值”的合同风险。

## 剩余风险

- 尚未完成手工回归，仍需串行确认押金抵扣、预约确认、开台/转台、支付回调重试，以及账单组金额展示、预检身份展示在真实页面和端到端链路上的表现。
- 预检身份语义与开台账单组聚合金额的 API 合同已补齐自动化回归，剩余未验证部分主要集中在小程序/Web 展示层和联调路径。
- 账单组创建、列表、拼桌加入三个主要返回入口已补齐 API 自动化回归，剩余未验证部分主要集中在“真实多人拼桌 + 部分支付 + 关台”的端到端串联表现。
- 手工执行步骤已经固化到 [business_flow_task_cards_20260327/manual-regression-checklist.md](business_flow_task_cards_20260327/manual-regression-checklist.md)，当前风险主要在于尚未实际执行并回填结果。
- `weapp` 当前执行 `npm run lint` 失败，但失败项位于既有未修复文件，不是本轮 `is_reservation_owner` 注释调整引入的问题。

## 上线后监控点

- 监控支付回调重复认领、查单失败与 release fail 告警是否仍出现高频重试。
- 监控堂食账单组金额异常投诉，重点看换单后金额展示与已付金额口径。
- 监控预检接口投诉或埋点，确认“本人预约”与“商户查看”没有再出现身份混淆。

## 当前评审结论

- 当前建议：有条件上线。
- 条件：上线前必须完成 CARD-12 中列出的 P0/P1 手工回归，并记录回归人、时间、结论。
- 若手工回归无法在上线窗口前完成，则当前结论自动降级为“暂不放行”。

## 完成记录

- [ ] 回归完成
- [x] 评审完成
- [x] 发布建议输出