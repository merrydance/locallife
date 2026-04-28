# TASK-PAY-003F-10 收付通补差命令记录任务卡

## 1. 目标

把补差 API 中的收付通 `CreateSubsidy`、`ReturnSubsidy`、`CancelSubsidy` 同步返回记录为 command 结果，避免补差资金写操作游离在 TASK-PAY-003F 的命令审计之外。

本任务只记录外部命令同步结果，不把补差 create、return、cancel 的业务终态迁入 fact/application，不整改既有补差对象级授权 deferred finding，也不改变当前补差 API 的返回语义。

## 2. 范围

纳入：

- `api/subsidy.go` 中 `CreateSubsidy` 成功且本地 `subsidy_orders` 更新为 success 后记录 `create_subsidy` accepted command。
- `api/subsidy.go` 中 `ReturnSubsidy` 成功且本地 return 状态更新为 `return_success` 后记录 `return_subsidy` accepted command。
- `api/subsidy.go` 中 `CancelSubsidy` 成功且本地补差单更新为 canceled 后记录 `cancel_subsidy` accepted command。
- 补差 command 常量、白名单和 API 单测。

不纳入：

- 补差事实模型、终态 application、恢复调度或对账。
- 补差对象级授权整改。
- 分账接收方生命周期拆分。
- 投诉、违规通知、结算账户修改等 operational command/fact。

## 3. 语义边界

- command accepted 只表示微信补差命令同步返回成功，并且本地 durable state 已经持久化。
- 当前失败路径不写 rejected/unknown，因为 create/return 失败后允许用同一外部单号重试；提前写入会被 command 去重键锁住后续成功记录。
- snapshot 只保留稳定契约键、微信二级 ID、金额和 result，不保存 description、退款说明、商户输入文本或原始微信 payload。

## 4. 验收

- create/return/cancel 三个成功路径都有 `external_payment_commands` accepted 写入。
- 失败路径仍保持现有业务行为，不新增 command 去重副作用。
- focused API tests 覆盖三类 command 写入。