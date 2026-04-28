# TASK-PAY-007C 分账 fact application consumer core 任务卡

日期：2026-04-26

状态：历史阶段。后续切换完成后，callback/query 不再直接更新 `profit_sharing_orders`，分账状态只由 fact application consumer 推进。

## 1. 目标

为 `profit_sharing_domain` 增加可测试的 `external_payment_fact_applications` 消费核心，让终态分账 fact 可以通过 application 边界推进本地 `profit_sharing_orders` 状态。

本段只建立 consumer core 和显式 worker task 入口，不把旧 callback/query 路径自动切到 application consumer，避免在旧路径仍会发分账结果通知时引入重复通知。

## 2. 范围

- `PaymentFactService.ApplyExternalPaymentFactApplication` claim 单个 application。
- 只支持 `consumer=profit_sharing_domain`、`business_object_type=profit_sharing_order`。
- 校验 fact 必须是微信电商分账终态 fact。
- `success` fact 只允许把 `processing` 本地分账单推进到 `finished`；已 `finished` 视为幂等成功。
- `failed` / `closed` fact 推进本地分账单到 `failed`；已 `failed` 视为幂等成功。
- 应用成功后标记 fact `processing_status=terminalized`，并标记 application `applied`。
- 应用失败时标记 application `failed`，设置 `last_error` 和下一次重试时间。
- 新增显式 worker task：`payment:process_fact_application`，payload 只包含 `application_id`。

## 3. 不在本段处理

- 不新增自动扫描 pending/failed application 的 scheduler。
- 不让 007A callback 或 007B query 路径自动 enqueue application task。
- 不迁移现有 `ProcessTaskProfitSharingResult` 通知链路。
- 不消费分账回退、补差、直连退款或押金类 fact application。
- 不改变现有 callback/query 对 `profit_sharing_orders` 的同步更新行为。

## 4. 验收

- application 无法 claim 时任务幂等 no-op，不触发重试。
- success fact 只从 processing 推进 finished，pending 等非预期状态会失败并进入 application 重试。
- failed/closed fact 可幂等落到 failed。
- 非微信电商分账、非终态、业务对象不匹配的 fact 不会被应用。
- 本段无 SQL/query 变更，因此不需要 `make sqlc`。

## 5. 验证

- `go -C /home/sam/locallife/locallife test ./logic ./worker -run 'TestPaymentFactServiceApplyExternalPaymentFactApplication|TestProcessTaskPaymentFactApplication' -count=1`

## 6. Review 结论

风险等级：G3。原因是本段触及支付事实消费、分账状态推进、worker 重试和幂等边界。

本段 consumer core 已被后续切换接入生产链路：callback/query 只记录 terminal fact 并创建 application，由 consumer 单源推进本地分账状态；结果通知和告警副作用由后续 outbox 执行面收口。