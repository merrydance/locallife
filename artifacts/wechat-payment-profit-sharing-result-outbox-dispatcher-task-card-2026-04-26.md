# TASK-PAY-007I profit sharing result outbox dispatcher 任务卡

日期：2026-04-26

状态：历史阶段。已由 TASK-PAY-007M 直接切换到 outbox 执行面；当前不再保持分账结果双轨。

## 1. 目标

让 `payment:process_domain_outbox` worker 具备处理 `profit_sharing_result_ready` 真实事件的能力，为后续切换分账结果通知链路做准备。

本段仍保持双轨：旧 `payment:process_profit_sharing_result` 任务继续存在，`PaymentDomainOutboxScheduler` 仍只扫描 probe event，不自动扫描真实分账 outbox。

## 2. 范围

- 在 `dispatchPaymentDomainOutbox` 中支持 `profit_sharing_result_ready`。
- 解码 outbox payload 为 `ProfitSharingResultPayload`，校验 aggregate type、aggregate id、out order no、result、merchant id。
- 成功分账事件沿用旧结果任务的商户通知内容，并在通知入队成功后标记 outbox published。
- 通知入队失败、dispatcher 缺失或 payload 不完整时标记 outbox failed，并设置 durable retry。
- 旧 `ProcessTaskProfitSharingResult` 继续走非严格模式，保持旧链路对通知/重排入队失败的宽松行为。

## 3. 不在本段处理

- 不让 scheduler 扫描 `profit_sharing_result_ready`。
- 不删除 callback/query 中旧的 `DistributeTaskProcessProfitSharingResult`。
- 不删除旧 `ProcessTaskProfitSharingResult`。
- 不改变分账 fact application 写 outbox 的时机。

## 4. 验收

- probe event 仍可正常 published。
- `profit_sharing_result_ready` success event 能发送商户财务通知并标记 published。
- 通知入队失败时 outbox 进入 failed retry，而不是误标记 published。
- distributor 缺失时 outbox 进入 failed retry。
- scheduler 仍只查询 `payment_domain_outbox_dispatcher_probe`。

## 5. 验证

- `go -C /home/sam/locallife/locallife test ./worker -run 'TestProcessTaskPaymentDomainOutbox|TestPaymentDomainOutboxScheduler|TestProcessTaskProfitSharingResult' -count=1`
- `go -C /home/sam/locallife/locallife test . ./logic ./worker -count=1`

## 6. Review 结论

风险等级：G3。原因是该 worker 后续会承载分账结果通知发布链路。本段只新增 worker 处理能力，不打开真实事件扫描，不切换旧结果任务，因此不会改变当前生产自动触发路径。

Review 中发现并修复：outbox 严格模式下 distributor 缺失时不能把事件标记 published，否则会造成通知未发送但 outbox 终态成功。现已改为 failed retry，并补充 focused 测试。