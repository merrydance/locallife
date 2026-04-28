# TASK-PAY-007A 分账 callback fact/application 旁路记录任务卡

日期：2026-04-25

## 1. 目标

把收付通分账结果回调在现有业务状态推进前写入 `external_payment_facts`，并在终态事实上创建 `external_payment_fact_applications`，为后续把分账业务推进迁移到 fact application consumer 做准备。

本段只覆盖分账 callback + query 得到的结果事实，不迁移现有业务终态写入，不改变分账重试策略，不处理分账回退或补差。

## 2. 范围

- 增加分账状态到统一 terminal status 的 normalizer。
- 在 `handleProfitSharingNotify` 查询分账结果并计算 `SUCCESS` / `FAILED` / `PROCESSING` 后记录 fact。
- 终态 fact 创建 `profit_sharing_domain` application，目标对象为本地 `profit_sharing_order`。
- fact raw resource 只保存稳定非敏字段，不保存接收方账号、姓名、加密姓名或完整微信原始 payload。
- fact 记录失败时释放 notification claim 并返回 `FAIL`，等待微信重试，避免分账终态已经推进但事实层缺失。

## 3. 不在本段处理

- 不把 `UpdateProfitSharingOrderToFinished` / `UpdateProfitSharingOrderToFailed` 移到 application consumer。
- 不消费 `external_payment_fact_applications`。
- 不迁移 `QueryProfitSharing` recovery scheduler 产生的 query fact。
- 不迁移分账回退 `profit_sharing_return` fact/application。
- 不迁移补差 create/return/cancel fact/application。

## 4. 验收

- 分账 callback 重复投递仍由原有 notification claim 与 fact dedupe key 双层保护。
- `SUCCESS` / `FINISHED` 映射为 `success`，`FAILED` 映射为 `failed`，`PROCESSING` / `PENDING` 映射为 `processing`。
- 终态分账 fact 使用 `provider=wechat`、`channel=ecommerce`、`capability=profit_sharing`、`external_object_type=profit_sharing`、`external_object_key=out_order_no`。
- 终态 fact 创建 application：`consumer=profit_sharing_domain`、`business_object_type=profit_sharing_order`、`business_object_id=profit_sharing_orders.id`。
- `raw_resource` 不包含 `receiver_account`、接收方姓名或加密姓名。

## 5. 验证

- `go -C /home/sam/locallife/locallife test ./logic ./api -run 'TestPaymentFactServiceRecordExternalPaymentFact|TestNormalizeProfitSharingTerminalStatus|TestHandlePaymentNotify_RiderDepositRecordsPaymentFact|TestHandleRefundNotify_RiderDepositRecordsRefundFact|TestHandleProfitSharingNotify' -count=1`
- `git --no-pager diff --check -- locallife/api/payment_callback.go locallife/api/payment_callback_profit_sharing_fact.go locallife/api/payment_callback_test.go locallife/logic/payment_fact_service.go locallife/logic/payment_fact_service_test.go`
- `rg -n '[ \t]+$' locallife/api/payment_callback.go locallife/api/payment_callback_profit_sharing_fact.go locallife/api/payment_callback_test.go locallife/logic/payment_fact_service.go locallife/logic/payment_fact_service_test.go`

## 6. Review 结论

风险等级：G3。原因是本段触及微信支付分账回调、终态事实记录、重复投递和资金状态推进前置路径。

当前实现是旁路事实记录：现有业务状态推进仍在 `handleProfitSharingNotify` 中执行，后续必须单独 review application consumer 迁移，不能把本段视为 TASK-PAY-007 完成。

残余风险：query/recovery 产生的分账终态尚未写 fact；分账回退和补差尚未进入 fact/application；分账 application 目前只创建不消费。