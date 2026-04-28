# TASK-PAY-005A Receiver Lifecycle Matrix

日期：2026-04-25

## 1. 审计目标

盘点当前微信分账接收方 ensure/delete 调用点，明确 owner、触发事件、当前失败行为与后续迁移目标。

本文件只读记录，不要求本阶段改业务代码。

## 2. 调用矩阵

| 调用点 | Owner | 触发事件 | 当前外部调用 | 当前失败行为 | 目标边界 | 后续任务 |
| --- | --- | --- | --- | --- | --- | --- |
| `worker/task_process_payment.go` rider deposit success | Rider deposit lifecycle | 骑手押金支付成功后，`ProcessTaskPaymentSuccess` 进入押金业务分支 | `EnsureRiderReceiver` -> `AddProfitSharingReceiver` | 获取 rider 或 ensure receiver 失败会让支付成功处理返回错误，可能导致支付成功任务重试 | 支付成功业务状态先落库，receiver ensure 写入 rider receiver intent，由异步 worker 重试 | TASK-PAY-005D |
| `logic/rider_deposit_refund_service.go` refund success | Rider deposit lifecycle | 押金退款终态成功且押金/冻结押金归零后 | `DeleteRiderReceiver` -> `DeleteProfitSharingReceiver` | delete 失败会让 refund resolve 返回错误，可能影响退款终态消费 | 退款终态先落库，receiver delete 写入 rider receiver intent，由异步 worker 重试 | TASK-PAY-005D |
| `logic/operator_status_service.go` active | Operator lifecycle | operator 切换为 active | `EnsureOperatorReceiver` -> `AddProfitSharingReceiver` | receiver ensure 失败阻止 operator 状态切换；区域冲突校验在此之前同步执行 | 区域冲突仍同步；operator active state 成功后写 receiver ensure intent | TASK-PAY-005C |
| `logic/operator_status_service.go` suspended | Operator lifecycle | operator 切换为 suspended，包括合同到期失活复用路径 | `DeleteOperatorReceiver` -> `DeleteProfitSharingReceiver` | receiver delete 失败阻止 suspended 与 role 状态更新，可能阻塞 scheduler | operator suspended/role state 先落库，receiver delete intent 异步重试 | TASK-PAY-005C |
| `api/operator_application_admin.go` approve with tx store | Operator lifecycle | 运营商审核通过，事务型 store 分支 | 事务前 `EnsurePersonalOpenIDReceiver`，事务失败后用 background context rollback delete | 微信 ensure 成功、DB 事务失败时需要补偿删除；rollback 失败仅记录日志，存在跨系统半成功 | 审核事务先提交 operator/application/role/region，再写 receiver ensure intent；取消事务前微信调用 | TASK-PAY-005C |
| `api/operator_application_admin.go` approve fallback | Operator lifecycle | 运营商审核通过，非事务型 fallback 分支 | 创建 operator/region/role 后同步 `EnsureOperatorReceiver` | receiver ensure 失败会让 API 返回失败，但本地 operator 可能已创建/激活 | 写 receiver ensure intent；API 返回审核已受理/已完成本地状态，并暴露 receiver sync pending/failed 状态 | TASK-PAY-005C |
| `api/profit_sharing_capability.go` manual delete by payment order | Manual repair / operational capability | 运营商按 payment_order 上下文手动删除 receiver | `DeleteProfitSharingReceiver` | 直接同步返回，作为手工能力接口 | 保留为 manual repair，但 owner 应从 payment_order 上下文迁到 receiver owner 上下文 | TASK-PAY-005E |
| `logic/profit_sharing_receiver_sync_service.go` helper | Shared adapter | 被上述 owner 调用 | Add/Delete receiver；already-exists / not-exists 错误窄范围忽略 | 当前 helper 同时承担 adapter、owner 决策、错误语义 | 保留微信 adapter 能力；owner 决策迁入 lifecycle service/worker | TASK-PAY-005B |

## 3. 外部对象键建议

Receiver target key 建议不要依赖 payment order：

- provider: `wechat`
- channel: `ecommerce`
- receiver_type: `PERSONAL_OPENID` 或 `MERCHANT_ID`
- appid: ecommerce client `sp_appid`
- account: openid 或 merchant id
- owner_type: `rider` / `operator` / `manual`
- owner_id: rider id / operator id / manual request id
- desired_state: `present` / `absent`

## 4. 状态建议

Receiver lifecycle intent / attempt 可考虑：

- `pending`: 等待 worker 执行。
- `processing`: worker 已 claim。
- `synced`: 微信侧达到 desired state。
- `failed`: 可重试或待人工处理。
- `skipped`: 缺 openid、owner 不再需要 receiver、desired state 被新 intent 覆盖。

错误分类建议：

- 幂等成功：already exists / not exists。
- 配置错误：ecommerce client nil、sp_appid missing。
- 数据错误：openid missing、owner missing。
- 微信拒绝：PARAM_ERROR、NO_AUTH、receiver high risk 等。
- 暂时性失败：5xx、timeout、rate limit。

## 5. 首轮实现建议

先做 TASK-PAY-005B schema/design，不直接改调用点。

理由：当前调用点跨 API、logic、worker、scheduler，且 operator 审核路径包含事务前微信调用和事务后补偿删除。没有持久化 intent/attempt 之前直接把同步调用删掉，会造成 receiver 漂移不可恢复。

## 6. 验证建议

后续实现时至少覆盖：

- rider deposit success 不因 receiver ensure 失败而重试支付成功业务。
- rider refund success 不因 receiver delete 失败而阻塞退款终态。
- operator active/suspended 不因 receiver sync 失败回滚本地状态。
- operator approval 事务路径不再出现微信 ensure 成功但 DB 失败后只能 background rollback 的半成功模式。
- already exists / not exists 幂等成功。
- missing openid 进入 skipped 或 failed，并有可见错误原因。
