# TASK-PAY-003 支付命令返回与业务终态拆分任务卡

日期：2026-04-25

## 1. 目标

把微信支付相关 create/submit 类调用的同步返回，统一收口为 command 结果，而不是业务终态。

本任务承接 TASK-PAY-002 的事实模型：

- `external_payment_commands` 记录本系统向微信提交过什么命令，以及同步返回是否被受理或拒绝。
- `external_payment_facts` 记录 callback、query、manual reconciliation 观察到的外部状态。
- 业务状态只能由业务域消费 terminal fact 后推进。

核心目标不是把所有微信客户端合并，而是清理“提交返回即成功”的语义漂移。

## 2. 当前输入

### 2.1 已完成前置

- TASK-PAY-001 已输出支付调用点矩阵：`artifacts/wechat-payment-callsite-matrix-2026-04-25.md`。
- TASK-PAY-002A 已新增 command/fact/application/outbox schema 和 sqlc 查询。
- TASK-PAY-002B 已新增 `PaymentFactService` skeleton。
- TASK-PAY-002C 已在骑手押金 direct payment/refund callback/query 路径写入只读 fact。

### 2.2 已提前完成的 003 预备切片

骑手押金提现 `SubmitWithdrawal` 已将 direct refund create 返回 `SUCCESS` 改为 accepted/processing，不再同步结算押金余额。

这属于 TASK-PAY-003 的方向，但还不是完整落地：

- 尚未写入 `external_payment_commands`。
- direct refund create 返回 `CLOSED` / `ABNORMAL` 的同步终态分支仍需收口。
- Swagger 生成物需要与接口注释同步。

## 3. 设计原则

### 3.1 三类状态必须分开

| 层级 | 表达 | 允许来源 | 禁止行为 |
| --- | --- | --- | --- |
| Command result | 本系统请求已提交、已受理、被同步拒绝或未知 | create/submit API 同步返回、本地请求错误 | 推进业务成功、退款完成、分账完成、进件完成 |
| External fact | 微信侧或外部通道侧观察到的状态 | callback、query、manual reconciliation | 直接改业务表而不经过业务域幂等 handler |
| Business transition | 本系统业务状态变化 | domain handler 消费 terminal fact | 由微信 adapter 或 submit service 直接决定终态 |

### 3.2 create/submit 返回语义

- 同步参数校验失败、合同校验失败、微信明确拒绝：可以记录 `rejected` 或回滚本地 prepare 状态。
- 同步返回受理、处理中、可调起支付、已创建申请：只能记录 `accepted` 或 `submitted`。
- 同步返回看似终态的状态值，例如 refund create 返回 `SUCCESS`，第一阶段也不得直接推进业务终态；应记录 command accepted，并等待 callback/query/manual fact。
- 已存在的上游已完成错误，例如“订单已全额退款”，只有在它代表本地历史漂移对账时，才允许作为 manual reconciliation 风格的窄例外，并必须有测试保护。

### 3.3 command 表边界

`external_payment_commands` 不是 request-level idempotency guard，也不是业务状态表。

- 它记录外部命令的契约键、业务 owner、同步返回状态和脱敏响应。
- 它不能替代 `out_trade_no`、`out_refund_no` 等领域表外部契约键。
- 它不能创建 `external_payment_facts`，除非后续明确进入 callback/query/manual 事实入口。
- 它不能创建 `external_payment_fact_applications`。

### 3.4 API 与前端语义

- 提交类接口返回给前端的状态应使用 `submitted`、`accepted`、`processing`、`pay_params_ready` 等词。
- 只有业务域已经确认终态时，才能返回或展示 `success`、`completed`、`refunded`、`finished`。
- Swagger、Mini Program 文案和接口字段含义必须同步，避免文档仍宣称“已完成”。

## 4. 阶段拆解

### TASK-PAY-003A 提交返回语义盘点

范围：

- 基于 TASK-PAY-001 矩阵，列出所有 create/submit 调用。
- 标记同步返回当前写入的本地状态。
- 标记是否存在 `success`、`finished`、`completed`、`refunded` 等误导性状态或文案。
- 标记是否已有 callback/query/manual terminal path。

首批盘点对象：

- 骑手押金充值 `CreateJSAPIOrder`。
- 骑手押金提现 `CreateRefund`。
- 追偿付款 `CreateJSAPIOrder`。
- 订单/预订支付 `CreatePartnerJSAPIOrder` / `CreateCombineOrder`。
- 订单/预订退款 `CreateEcommerceRefund`。
- 分账 `CreateProfitSharing` / `FinishProfitSharing`。
- 进件 `SubmitApplyment` / query applyment。
- 商户提现 `CreateEcommerceWithdraw`。
- 商户注销提现 `CreateEcommerceCancelWithdraw`。
- 商家转账 `Transfer`。

验收：

- 每个 submit path 有明确 owner、external object key、command status、terminal fact source。
- 每个误用终态返回的路径有后续任务编号。
- 不修改业务代码。

验证：

- `rg "CreateJSAPIOrder|CreatePartnerJSAPIOrder|CreateCombineOrder|CreateRefund|CreateEcommerceRefund|CreateProfitSharing|FinishProfitSharing|Applyment|Withdraw|Transfer" locallife`

### TASK-PAY-003B command writer skeleton

范围：

- 新增 `logic/payment_command_service.go` 或等价小服务。
- 提供 `RecordExternalPaymentCommand(ctx, input)`。
- 校验 provider、channel、capability、command_type、business_owner、external_object_type、external_object_key、command_status。
- 使用 `CreateExternalPaymentCommand`，重复时返回既有 command。
- 对 request/response snapshot 做脱敏边界说明，禁止存身份证号、银行卡号、证件图片、签名、密钥、完整原始 payload。

边界：

- 不调用微信客户端。
- 不写 fact。
- 不推进业务状态。
- 不设计 request-level idempotency records。

验收：

- accepted/rejected/submitted/unknown 均可记录。
- 重复 external command key 不产生多条 command。
- 无效枚举或空 external object key 不写库。

验证：

- `go test ./logic -run TestPaymentCommandService -count=1`
- `go test ./db/sqlc -run TestCreateExternalPaymentCommand -count=1`

### TASK-PAY-003C 骑手押金 direct refund create 接入 command 表

范围：

- `RiderDepositRefundService.SubmitWithdrawal` 在调用 direct refund create 前后记录 command。
- create request 发起前可记录 `submitted`，同步受理后记录或保持 `accepted`，同步拒绝记录 `rejected`。
- 记录能力：provider `wechat`、channel `direct`、capability `direct_refund`、command_type `create_refund`、business_owner `rider_deposit`、business_object_type `refund_order`、external_object_type `refund`、external_object_key `out_refund_no`。
- accepted command 可以记录 `refund_id` 作为 external secondary key。

边界：

- 不创建 fact。
- 不改变 callback/query 事实写入。
- 不改变当前 prepare refund transaction 的冻结语义。
- 不把 command accepted 暴露为提现成功。

验收：

- `PROCESSING` 和 create-response `SUCCESS` 都只记录 accepted command，并返回 202/processing。
- 同一个 out_refund_no 重放不产生重复 command。
- create 同步拒绝时记录 rejected command，并保留现有本地补偿/错误映射行为。

验证：

- `go test ./logic -run 'TestRiderDepositRefundService_SubmitWithdrawal_.*Command' -count=1`
- `go test ./api -run TestWithdrawRiderAPI -count=1`

### TASK-PAY-003D 收口骑手押金提现同步终态残留

范围：

- 修正 `SubmitWithdrawal` 中 direct refund create 返回 `CLOSED` / `ABNORMAL` 时直接调用 `ResolveRefund` 的残留。
- 明确这类同步返回应被视为同步拒绝、未知 accepted 后待 query，或 contract drift；选择必须有测试证明。
- 保留“订单已全额退款” stale credit reconciliation 的窄例外，并把它标记为 manual reconciliation 风格的本地对账行为。
- 同步 Swagger 生成物。

边界：

- 不迁移 `ProcessTaskRefundResult`。
- 不改 callback/query 已存在的终态处理路径。
- 不引入 fact application 消费。

验收：

- Submit path 不再直接推进押金终态成功、关闭或异常回滚，除 stale credit 对账例外。
- CLOSED/ABNORMAL 分支有明确 API 错误或 processing 语义。
- Swagger 文档中 200/202 和响应说明与代码一致。

验证：

- `go test ./logic -run TestRiderDepositRefundService_SubmitWithdrawal -count=1`
- `go test ./api -run TestWithdrawRiderAPI -count=1`
- `make swagger`

### TASK-PAY-003E API / Mini Program 提交状态文案收口

范围：

- 后端 Swagger 和响应字段说明。
- Mini Program 骑手提现页面。
- 后续按域扩展到订单退款、商户提现、进件提交等页面。

边界：

- 不改变业务状态机。
- 不用全局 notice bar 解释状态，优先使用就地状态和按钮旁提示。
- 不把 `accepted` 翻译成“成功到账”或“已退款”。

验收：

- 用户看到“已提交处理 / 等待微信确认 / 处理中”。
- 只有终态 fact 处理完成后才展示“已完成 / 已到账 / 已退款”。
- 空态、错误态和重试态不误导。

验证：

- 后端：`go test ./api -run TestWithdrawRiderAPI -count=1`
- Mini Program：按变更范围运行 `npm run compile` 或聚焦页面检查。

### TASK-PAY-003F 逐域迁移其他 submit path

细化任务卡：

- 003F-1 直连支付下单命令记录：`artifacts/wechat-payment-direct-payment-command-task-card-2026-04-25.md`
- 003F-2 追偿直连支付下单命令记录：`artifacts/wechat-payment-claim-recovery-command-task-card-2026-04-25.md`
- 003F-3a 商户拒单收付通退款命令记录：`artifacts/wechat-payment-merchant-reject-refund-command-task-card-2026-04-25.md`
- 003F-3b 预订换菜收付通退款命令记录：`artifacts/wechat-payment-replace-reservation-refund-command-task-card-2026-04-25.md`
- 003F-3c 预订取消收付通退款命令记录：`artifacts/wechat-payment-cancel-reservation-refund-command-task-card-2026-04-25.md`
- 003F-3d 通用退款服务收付通退款命令记录：`artifacts/wechat-payment-refund-service-command-task-card-2026-04-25.md`
- 003F-3e Worker 收付通退款命令记录：`artifacts/wechat-payment-worker-refund-command-task-card-2026-04-25.md`
- 003F-4a 收付通单笔 JSAPI 支付下单命令记录：`artifacts/wechat-payment-partner-jsapi-payment-command-task-card-2026-04-25.md`
- 003F-4b 预订换菜补款单笔支付命令记录：`artifacts/wechat-payment-replace-reservation-payment-command-task-card-2026-04-25.md`
- 003F-4c 收付通合单支付下单命令记录：`artifacts/wechat-payment-combine-payment-command-task-card-2026-04-25.md`
- 003F-4d 预订加菜合单支付下单命令记录：`artifacts/wechat-payment-reservation-addon-combine-payment-command-task-card-2026-04-25.md`
- 003F-5a 收付通分账命令记录：`artifacts/wechat-payment-profit-sharing-command-task-card-2026-04-25.md`
- 003F-5b 收付通分账回退命令记录：`artifacts/wechat-payment-profit-sharing-return-command-task-card-2026-04-25.md`
- 003F-6 收付通商户提现命令记录：`artifacts/wechat-payment-merchant-withdraw-command-task-card-2026-04-25.md`
- 003F-7 收付通商户注销提现命令记录：`artifacts/wechat-payment-merchant-cancel-withdraw-command-task-card-2026-04-25.md`
- 003F-8 收付通商户进件命令记录：`artifacts/wechat-payment-applyment-command-task-card-2026-04-25.md`
- 003F-9 直连商家转账赔付命令记录：`artifacts/wechat-payment-merchant-transfer-command-task-card-2026-04-25.md`
- 003F-10 收付通补差命令记录：`artifacts/wechat-payment-subsidy-command-task-card-2026-04-25.md`

收尾审计记录（2026-04-25）：

- 已覆盖的资金/长流程 submit path：direct JSAPI payment、direct refund、partner JSAPI payment、combine order、ecommerce refund、profit sharing create/finish/return、merchant withdraw、merchant cancel withdraw、applyment、merchant transfer。
- 审计发现补差 create/return/cancel 是 G3 资金写路径，属于 TASK-PAY-007 能力组但仍应先纳入 TASK-PAY-003F command 记录，因此拆出 TASK-PAY-003F-10。
- 暂不纳入 003F：`ModifySubMerchantSettlement` 归 TASK-PAY-008；`CreateViolationNotification`、`RespondComplaint`、`CompleteComplaint` 归 TASK-PAY-010；`AddProfitSharingReceiver`、`DeleteProfitSharingReceiver` 归 TASK-PAY-005。

范围：

- 订单/预订支付和退款。
- 分账和分账完结。
- 进件提交。
- 商户提现与注销提现。
- 商家转账。

迁移顺序建议：

1. 骑手押金 direct refund。
2. 骑手押金 direct payment / 追偿 direct payment。
3. 订单/预订 ecommerce refund。
4. 订单/预订 payment create。
5. 分账 create/finish。
6. 商户提现和商户注销提现。
7. 进件和商家转账。

每个域必须独立任务卡，不允许跨域大改。

验收：

- submit 返回语义不再冒充终态。
- command 记录、fact 记录、业务状态推进三层可单独审计。
- 每个高风险金额路径都有重复提交、callback 重放、query 补偿测试。

## 5. 风险与发布门禁

### 5.1 风险

- 把 create-response `SUCCESS` 改成 processing 可能改变前端即时提示，但这是架构预期；前端必须同步文案。
- 如果移除 submit path 的同步终态处理，而 callback/query 覆盖不足，会导致业务停留 processing；迁移前必须确认终态恢复路径存在。
- command 表重复写入必须依赖外部契约键，不得引入新的 request-level 幂等误判。
- response snapshot 必须脱敏，不能为了排障直接存原始微信 payload。

### 5.2 发布门禁

- 每个改动路径必须有“create 返回 accepted/processing 不推进业务终态”的测试。
- 每个移除同步终态的路径必须有 callback 或 query 终态补偿测试。
- Swagger 或 API 注释变更必须同步生成物。
- 小程序文案变更必须避免终态误导。
- 金额状态变更必须保留幂等事务测试。

## 6. 当前推荐下一步

003B、003C、003D、003E 与 003F-1 至 003F-10 已按分段任务推进。003F-10 完成并复审后，TASK-PAY-003 可以进入 closeout；后续不在 003F 内继续扩展 operational path，而转入 TASK-PAY-005、TASK-PAY-008、TASK-PAY-009、TASK-PAY-010。

## 7. Closeout 记录（2026-04-25）

TASK-PAY-003 当前可以按“命令返回与业务终态拆分的 command 记录层”收口。

已完成：

- `PaymentCommandService` 已成为 command 写入的统一边界。
- direct payment、direct refund、partner JSAPI payment、combine payment、ecommerce refund、profit sharing create/finish/return、merchant withdraw、merchant cancel withdraw、applyment、merchant transfer、subsidy create/return/cancel 已按外部契约键写入 `external_payment_commands`。
- 各切片均保持 command/fact/application/business transition 分层：本阶段不从 command 记录创建 fact，不写 `external_payment_fact_applications`，不推进业务终态。
- response snapshot 均按切片保留稳定契约键和必要状态，不保存原始微信 payload、调起支付签名、银行卡/证件/实名信息、receiver name、openid、转账备注或补差描述。

验证证据：

- `go test ./api -run 'Test.*Subsidy' -count=1`
- `go test ./logic -run 'TestPaymentCommandService' -count=1`
- 003F-7 已通过 merchant cancel withdraw focused API/logic tests。
- 003F-8 已通过 applyment focused logic/API tests。
- 003F-9 已通过 focused worker tests 与 `go test ./worker -count=1`。
- `git diff --check` 与 `get_errors` 已在最新切片通过。

未在 TASK-PAY-003 内解决的剩余工作：

- TASK-PAY-005：分账接收方生命周期独立化，包括 `AddProfitSharingReceiver` / `DeleteProfitSharingReceiver` 的 owner、重试和恢复边界。
- TASK-PAY-007：分账剩余异步链路与补差能力组后续收口；补差 command 层已完成，但后续只按真正需要 callback/query/recovery 的子项逐项推进，不把 `CreateSubsidy` 这种同步终态契约强行迁入 fact/application。
- TASK-PAY-008：进件、结算账户、提现、注销提现的长流程 fact 化；`ModifySubMerchantSettlement` 不在 003F 中处理。
- TASK-PAY-009：支付、退款、分账、进件、提现、转账等处理中对象的统一 query scheduler 与对账恢复。
- TASK-PAY-010：投诉、违规通知、结算事件等 operational fact/observability/release gate。

残余风险：

- 本阶段只保证 create/submit/write 命令同步返回可审计，不保证 callback/query terminal fact 已统一消费。
- 仍有部分旧路径会在 domain worker 或 handler 内直接消费 callback/query 结果推进业务状态；这些属于后续 fact application 迁移范围。
- 补差能力已有 deferred 对象级授权 finding，本阶段没有整改该 authz 边界，只记录命令审计层。
- 2026-04-26 文档同步：`CreateSubsidy` 上游契约返回 `result` 与 `success_time`，当前实现只修正空 body 成功容错，不再把它视为下一主线的异步终态迁移入口。