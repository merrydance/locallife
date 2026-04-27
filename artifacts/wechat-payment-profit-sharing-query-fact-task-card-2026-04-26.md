# TASK-PAY-007B 分账 query fact/application 旁路记录任务卡

日期：2026-04-26

## 1. 目标

把 processing 分账单在 worker 恢复路径中通过 `QueryProfitSharing` 观察到的结果写入 `external_payment_facts`，并在终态事实上创建 `external_payment_fact_applications`，让 callback 与 query 逐步收敛到同一事实入口。

本段只覆盖已有 `reconcileProcessingProfitSharing` 查询分支，不新增 scheduler，不改变分账 create、现有业务状态推进、失败通知、自动重试或分账回退语义。

## 2. 范围

- 在 processing 分账单查询微信结果后、更新本地 `profit_sharing_orders` 前写入 `source=query` fact。
- 终态 query fact 创建 `profit_sharing_domain` application，目标对象为本地 `profit_sharing_order`。
- processing / unknown fact 只记录事实，不创建 application。
- fact 记录失败时返回 worker 错误，等待任务重试，避免 query 已推进本地终态但事实层缺失。
- raw resource 只保存稳定非敏字段，不保存 receiver account、receiver name、description、encrypted name 或完整微信原始 payload。

## 3. 不在本段处理

- 不把 `UpdateProfitSharingOrderToFinished` / `UpdateProfitSharingOrderToFailed` 移到 application consumer。
- 不消费 `external_payment_fact_applications`。
- 不迁移分账回调以外的独立手工 query/reconciliation 入口。
- 不迁移分账回退 `profit_sharing_return` fact/application。
- 不迁移补差 create/return/cancel fact/application。

## 4. 验收

- query fact 使用 `provider=wechat`、`channel=ecommerce`、`capability=profit_sharing`、`fact_source=query`、`external_object_type=profit_sharing`、`external_object_key=out_order_no`。
- query fact dedupe key 使用 `wechat:query:ecommerce:profit_sharing:<out_order_no>:<terminal_status>`。
- 终态 query fact 创建 application：`consumer=profit_sharing_domain`、`business_object_type=profit_sharing_order`、`business_object_id=profit_sharing_orders.id`。
- fact 写入在本地终态更新前执行；fact 写入失败不调用 finished/failed 更新。
- `raw_resource` 不包含 `receiver_account`、接收方姓名、接收方描述或加密姓名。

## 5. 验证

- `go -C /home/sam/locallife/locallife test ./worker -run 'TestProcessTaskProfitSharing_ProcessingOrder(Query|FactFailure)|TestProcessTaskProfitSharing_ProcessingOrderQueriesAndFinishes' -count=1`

## 6. Review 结论

风险等级：G3。原因是本段触及微信分账 query recovery、终态事实记录、重复执行和资金状态推进前置路径。

当前实现仍是旁路事实记录：现有业务状态推进仍在 worker 中执行，后续必须单独 review application consumer 迁移，不能把本段视为 TASK-PAY-007 完成。

残余风险：分账回退和补差尚未进入 fact/application；分账 application 目前只创建不消费；手工 reconciliation 来源尚未接入。