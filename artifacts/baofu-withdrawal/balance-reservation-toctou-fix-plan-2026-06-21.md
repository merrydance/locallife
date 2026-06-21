# 宝付提现余额 TOCTOU 生产级修复方案

日期：2026-06-21
范围：`locallife/` 宝付宝财通多角色提现创建、异步派发、回调/恢复终态应用、余额展示和资金审计。
风险等级：G3。原因：该链路涉及真实资金提现、三方余额查询、异步 provider 命令、回调/恢复重试、幂等键和多 owner 资金边界。

## 1. 背景与审计结论

当前提现创建链路为：

1. API 入口解析服务端 owner scope，不接受客户端传 owner：`locallife/api/baofu_withdrawal.go`。
2. `BaofuWithdrawService.CreateWithdrawal` 先按 `(owner_type, owner_id, idempotency_key)` 做重放查询。
3. 服务读取 active 宝付账户绑定，调用宝付 `QueryBalance`。
4. Go 逻辑比较 `input.AmountFen > balance.AvailableAmountFen`。
5. 本地事务 `CreateBaofuWithdrawalOrderWithSubmittedCommandTx` 只创建 `baofu_withdrawal_orders` 和 `external_payment_commands`。
6. worker 后续异步执行宝付 `CreateWithdraw`。
7. 宝付回调或恢复查询再推进提现单终态。

关键证据：

- 余额外呼和 Go 层校验位于 `locallife/logic/baofu_withdraw_service.go` 的 `CreateWithdrawal`。
- 本地事务位于 `locallife/db/sqlc/tx_baofu_withdrawal.go`，当前只落提现单和外部命令。
- 真正宝付提现外呼位于 `locallife/worker/task_baofu_withdrawal_command_dispatch.go`。
- 提现单当前只通过 `out_request_no` 和 `(owner_type, owner_id, idempotency_key)` 约束重复请求，不约束不同幂等键的合计提现金额。

审计结论：

- 这不是“HTTP 请求同步双外呼提现”的形态；当前设计是 HTTP 落本地命令、worker 异步派发。
- 但风险真实存在：宝付余额查询是事务外的三方快照，本地数据库没有冻结/预占资金，也没有在同一 owner/account 下对处理中提现金额建立强一致不变量。
- 不同 `Idempotency-Key` 是合法的不同提现请求，现有幂等机制不会也不应该拦截它们。
- 宝付后续拒绝其中一笔只能降低损失概率，不能作为本地资金不超额的强一致保证。

## 2. 修复目标

必须建立以下生产不变量：

- 同一宝付结算账户在本地任意时刻的 `reserved_amount_fen` 不得超过最近可用余额快照允许的提现额度。
- 提现创建、外部命令创建和本地资金预占必须在同一个数据库事务里提交或回滚。
- 同一个幂等键重放不得重复预占资金。
- 不同幂等键的并发提现必须被 owner/account 级本地锁串行化，后到请求必须看到先到请求的预占结果。
- provider 同步拒绝、回调失败、恢复查询失败、退回状态都必须释放本地预占；成功状态必须消耗本地预占。
- 回调重复投递、worker 重试、恢复任务重复查询必须幂等，不得重复释放或重复消耗。
- 余额展示必须扣除本地已预占金额，避免用户看到仍可提现但提交后被本地拒绝。
- 所有 owner 边界继续以服务端解析的 `owner_type + owner_id + account_binding_id` 为准，不允许客户端影响资金作用域。

## 3. 推荐方案

采用“本地提现预占账户 + 单提现预占记录”的两层模型。

### 3.1 为什么不只做 `SUM(processing)`

只在创建时 `SELECT SUM(amount) FROM baofu_withdrawal_orders WHERE status='processing'` 可以挡住一部分并发，但它缺少三个生产能力：

- 没有每笔预占的生命周期记录，难以证明某次终态是否已释放/消耗。
- 回调和恢复重复执行时很难天然幂等，容易出现重复释放或漏释放。
- 余额展示、审计和异常修复只能重新扫描提现单，无法形成明确资金控制面。

因此不推荐作为最终方案。可以把 `processing` 合计用于迁移回填和监控校验，但不作为核心不变量。

### 3.2 新增数据模型

新增 `baofu_withdrawal_account_guards`：

- `id`
- `owner_type`
- `owner_id`
- `account_binding_id`
- `provider_available_amount_fen`
- `provider_pending_amount_fen`
- `provider_ledger_amount_fen`
- `provider_frozen_amount_fen`
- `provider_balance_observed_at`
- `reserved_amount_fen`
- `consumed_withdraw_amount_fen`
- `created_at`
- `updated_at`

约束：

- `UNIQUE(owner_type, owner_id, account_binding_id)`
- `reserved_amount_fen >= 0`
- provider 金额字段均 `>= 0`
- owner/account 字段不得为空；`platform` owner 使用固定 `owner_id=0`，其他 owner 必须大于 0

新增 `baofu_withdrawal_reservations`：

- `id`
- `withdrawal_order_id`
- `owner_type`
- `owner_id`
- `account_binding_id`
- `amount_fen`
- `status`：`reserved`、`consumed`、`released`
- `release_reason`：可空，失败/退回/人工修复时写入稳定枚举
- `reserved_at`
- `consumed_at`
- `released_at`
- `created_at`
- `updated_at`

约束：

- `UNIQUE(withdrawal_order_id)`
- `status IN ('reserved','consumed','released')`
- `amount_fen > 0`
- `released_at` 仅在 `released` 时非空
- `consumed_at` 仅在 `consumed` 时非空

说明：

- guard 表负责 owner/account 级串行锁和聚合预占金额。
- reservation 表负责每笔提现的幂等终态结算和审计。
- 这不是替代宝付官方余额的完整资金账本；它是 LocalLife 对“已提交但未终态提现意图”的强一致控制阀门。

## 4. 创建提现事务设计

新增事务方法：

`CreateBaofuWithdrawalOrderWithReservationAndSubmittedCommandTx(ctx, params)`

输入包含：

- 原 `CreateBaofuWithdrawalOrderParams`
- `BusinessOwner`
- `SubmittedAt`
- provider balance 快照金额和 `ObservedAt`

事务步骤：

1. Upsert 并锁定 `baofu_withdrawal_account_guards` 当前 owner/account 行。
2. 如果传入 provider 快照时间更新，则刷新 guard 的 provider 金额字段。
3. 计算 `local_available = provider_available_amount_fen - reserved_amount_fen`。
4. 若 `amount_fen > local_available`，返回 `ErrBaofuWithdrawInsufficientBalance`，不创建提现单，不创建命令，不预占资金。
5. 创建 `baofu_withdrawal_orders`，状态仍为 `processing`。
6. 创建 `baofu_withdrawal_reservations`，状态为 `reserved`。
7. 将 guard 的 `reserved_amount_fen += amount_fen`。
8. 创建 `external_payment_commands`，状态为 `submitted`。
9. 返回提现单、命令、reservation 和 guard 快照。

时序要求：

- 宝付 `QueryBalance` 仍可在事务外执行，避免长事务持有 DB 锁等待三方网络。
- 但本地强一致判断必须在事务内基于锁定后的 guard 和已预占金额执行。
- 如果 provider 查询失败，创建提现 fail closed，沿用当前 `ErrBaofuWithdrawBalanceUnavailable` 到 502 语义；第一阶段不做“无 provider 快照也允许提现”的降级。

并发效果：

- 两个不同幂等键并发查询到同一个宝付可用余额快照时，进入 DB 事务后会按同一 guard 行串行。
- 第一个请求预占成功后，第二个请求会看到更新后的 `reserved_amount_fen`，合计超额时返回“可提现金额不足”。

## 5. 幂等设计

保持当前 request-level 幂等：

- 同一 `(owner_type, owner_id, idempotency_key)` 且请求 hash 一致：直接返回已有提现单，不再次查询宝付，不再次预占。
- 同一幂等键但请求 hash 不一致：返回 409 conflict。
- 不同幂等键：视为不同提现意图，但受本地 guard 余额预占约束。

事务内仍需处理唯一冲突：

- `out_request_no` 冲突：按现有 unique-conflict replay 逻辑处理，不得重复预占。
- `baofu_withdrawal_reservations.withdrawal_order_id` 冲突：视为异常幂等保护，回滚并重新查询已有订单。

## 6. 终态应用与释放设计

新增事务方法：

`ApplyBaofuWithdrawalTerminalStatusTx(ctx, params)`

覆盖以下入口：

- worker 派发 `CreateWithdraw` 后 provider 同步拒绝。
- 宝付提现回调 fact application。
- recovery scheduler 查询到终态后派发的 fact application。
- 后续如有人工修复入口，也必须复用该事务。

事务步骤：

1. 锁定 `baofu_withdrawal_orders` 目标行。
2. 如果提现单已是终态：
   - 查询 reservation。
   - 若 reservation 已是对应终态结算状态，直接 no-op 返回。
   - 若发现订单终态和 reservation 状态不一致，返回可观测错误并触发人工修复告警。
3. 如果提现单仍是 `processing`，锁定对应 reservation。
4. 更新提现单状态、`baofu_withdraw_no`、`raw_snapshot`、`finished_at`。
5. 若状态为 `succeeded`：
   - reservation `reserved -> consumed`
   - guard `reserved_amount_fen -= amount_fen`
   - guard `consumed_withdraw_amount_fen += amount_fen`
6. 若状态为 `failed` 或 `returned`：
   - reservation `reserved -> released`
   - guard `reserved_amount_fen -= amount_fen`
   - 写入 `release_reason`
7. 所有 guard 扣减必须带 `reserved_amount_fen >= amount_fen` 条件，防止重复释放造成负数。

状态映射：

- `processing`：不结算 reservation。
- `succeeded`：消耗预占。
- `failed`：释放预占。
- `returned`：释放预占。

## 7. 余额展示调整

`BaofuWithdrawService.QueryBalance` 保留宝付实时查询，但返回给前端的可提现金额需要扣除本地预占：

- `provider_available = upstream.AvailableAmountFen`
- `local_reserved = guard.reserved_amount_fen`
- `available_amount = max(provider_available - local_reserved, 0)`

`pending_amount` 的第一阶段建议保守处理：

- 对外仍返回宝付 `PendingAmountFen`，避免将 provider pending 和本地 reserved 重复相加。
- 内部结构可增加 `LocalReservedAmountFen`，用于日志、测试和后续 API 扩展。
- 如果产品侧需要展示“本地已提交提现中”，再单独扩展响应字段，不在本次高风险修复中顺手改公开契约。

## 8. 越权与租户边界

创建、查询和终态释放均使用服务端解析出的 owner scope：

- merchant：`merchant.ID`
- rider：`rider.ID`
- operator：`operator.ID`
- platform：固定 platform owner

guard 和 reservation 必须同时保存：

- `owner_type`
- `owner_id`
- `account_binding_id`

所有查询、锁定和回填必须验证 `account_binding_id` 属于同一 owner。不得只按 `account_binding_id` 更新资金预占，也不得接受客户端传入 owner/account 字段。

## 9. 迁移与回填

新增 migration，当前分支下一号为 `000278_add_baofu_withdrawal_reservations`；执行前需以最新 `main` 重新确认编号。

Up migration：

1. 创建 `baofu_withdrawal_account_guards`。
2. 创建 `baofu_withdrawal_reservations`。
3. 为现有 `status='processing'` 的 `baofu_withdrawal_orders` 回填 reservation，状态为 `reserved`。
4. 按 owner/account 聚合 processing 金额回填 guard 的 `reserved_amount_fen`。
5. provider 金额字段初始为 0，`provider_balance_observed_at` 可空；后续首次 balance/query create 会刷新。

Down migration：

- 删除 reservation 表。
- 删除 guard 表。

上线窗口注意：

- 如果生产有多个旧版本实例并行，必须在迁移到新代码期间短暂禁止提现创建，或先排空旧实例，避免旧代码在回填后继续创建无 reservation 的 processing 订单。
- 新代码终态事务应对“历史 processing 订单缺失 reservation” fail closed 并打可观测错误，不要静默更新终态导致预占账不一致。
- 上线后运行一次审计 SQL：检查所有 `processing` 提现单均有 `reserved` reservation，所有 terminal 提现单没有 `reserved` reservation。

## 10. 实施任务拆分

### Task 1：DB schema 与 sqlc

文件：

- 新增：`locallife/db/migration/000278_add_baofu_withdrawal_reservations.up.sql`
- 新增：`locallife/db/migration/000278_add_baofu_withdrawal_reservations.down.sql`
- 修改：`locallife/db/query/baofu_withdrawal_order.sql`
- 修改：`locallife/db/sqlc/tx_baofu_withdrawal.go`
- 生成：`locallife/db/sqlc/*.sql.go`

验收：

- `make sqlc`
- `go test ./db/sqlc -run 'TestBaofuWithdrawalReservation|TestCreateBaofuWithdrawalOrderWithReservation'`
- `make check-generated`

### Task 2：创建提现预占事务接入

文件：

- 修改：`locallife/logic/baofu_withdraw_service.go`
- 修改：`locallife/logic/baofu_withdraw_service_test.go`
- 如 store/mock 接口变化，运行 `make mock`

验收：

- 两个不同幂等键并发创建、合计超额时只能成功一笔。
- 同一幂等键 replay 不重复预占。
- provider QueryBalance 失败时不落单、不预占。
- `go test ./logic -run 'TestBaofuWithdrawServiceCreateWithdrawal'`

### Task 3：终态事务接入 worker/callback/recovery

文件：

- 修改：`locallife/worker/task_baofu_withdrawal_command_dispatch.go`
- 修改：`locallife/worker/task_baofu_withdrawal_fact_application.go`
- 修改：`locallife/worker/*baofu_withdrawal*_test.go`
- 必要时修改 `locallife/api/baofu_callback*.go` 的 payload/错误承接，但 callback ACK 语义不变。

验收：

- provider 同步拒绝释放预占。
- 回调 `succeeded` 消耗预占。
- 回调 `failed/returned` 释放预占。
- 重复回调 no-op，不重复释放/消耗。
- `go test ./worker -run 'TestProcessTaskBaofuWithdrawal'`

### Task 4：余额读取扣除本地预占

文件：

- 修改：`locallife/logic/baofu_withdraw_service.go`
- 修改：`locallife/api/baofu_withdrawal.go`
- 修改：`locallife/api/baofu_withdrawal_contract_test.go`

验收：

- provider 可用 100、本地 reserved 80 时，GET balance 返回 `available_amount=20`。
- `can_withdraw` 基于扣除后的金额判断。
- 不改变未开户/未激活账户的 empty-state 行为。
- `go test ./api -run 'Test.*BaofuWithdrawal.*Balance'`

### Task 5：审计、监控与异常修复入口

文件：

- 新增或修改：`locallife/db/query/baofu_reconciliation.sql`
- 可选新增：只读审计查询，例如 `ListBaofuWithdrawalReservationDrifts`
- 修改：相关 admin/operator 财务审计测试

验收：

- 能查出 processing 订单无 reserved reservation。
- 能查出 terminal 订单仍有 reserved reservation。
- 能查出 guard 聚合金额与 reservation 聚合不一致。
- 不自动修复不一致，先暴露为 operator/audit 信号；人工修复必须单独走受控脚本或后续任务。

## 11. 测试矩阵

必须覆盖：

- DB 并发：两个 goroutine、不同 idempotency key、同一 owner/account、provider 快照可用金额不足以覆盖两笔，最终只有一笔 reservation。
- DB 幂等：同一 idempotency key 重放不新增 reservation，不增加 guard reserved。
- DB 终态：`reserved -> consumed`、`reserved -> released`、重复应用 no-op。
- Logic：provider balance 不足、provider balance 足够但本地 reserved 后不足。
- API：用户看到 409 “可提现金额不足”，而不是 500/502。
- Worker：provider 明确拒绝、未知结果、accepted processing、回调终态、恢复终态。
- Migration：已有 processing 订单回填 reservation 和 guard reserved。

建议命令：

```bash
cd locallife
make sqlc
go test ./db/sqlc -run 'TestBaofuWithdrawalReservation|TestCreateBaofuWithdrawalOrderWithReservation|TestApplyBaofuWithdrawalTerminalStatus'
go test ./logic -run 'TestBaofuWithdrawServiceCreateWithdrawal|TestBaofuWithdrawServiceQueryBalance'
go test ./worker -run 'TestProcessTaskBaofuWithdrawal'
go test ./api -run 'Test.*BaofuWithdrawal.*Balance|TestCreate.*BaofuWithdrawal'
make check-baofu-contract
make check-generated
make test-safety
git diff --check
```

## 12. 上线与回滚

上线前：

- 确认最近 24 小时 `baofu_withdrawal_orders status='processing'` 数量和金额。
- 确认是否存在多实例旧代码并发创建提现；如存在，安排短暂提现创建维护窗口或先排空旧实例。
- 备份 migration 前 schema 状态和关键聚合查询结果。

上线步骤：

1. 关闭或排空旧版本提现创建流量。
2. 发布包含 migration 和新代码的版本。
3. 启动后执行审计 SQL，确认 processing 订单都有 reservation。
4. 人工发起一笔小额沙箱/测试 owner 提现，验证创建预占、worker 派发、回调/恢复释放或消耗。
5. 恢复提现创建流量。

回滚策略：

- 如果新代码未开始处理真实提现，可回滚代码并执行 down migration。
- 如果已有新 reservation 参与真实提现，不建议直接 down migration；应先停止提现创建，等待 processing 订单终态收敛或导出 reservation 状态，再由人工脚本对账后回滚。
- 任意回滚都不得删除还在 `reserved` 的资金预占记录而不留审计备份。

## 13. Review 检查清单

每完成一个任务必须 review 一次：

- 是否真的把不变量放进 DB 事务，而不是只在 Go 层判断。
- 是否没有把外部 provider 调用放进持锁事务里。
- 是否同一幂等键 replay 不重复预占。
- 是否不同幂等键并发会被本地 guard 串行化。
- 是否终态成功/失败/退回都能正确结算 reservation。
- 是否重复回调、worker retry、recovery retry 都是幂等。
- 是否 owner/account 作用域全部来自服务端上下文和本地 binding。
- 是否日志没有泄露完整合同号、银行卡、证件、手机号、原始 provider 报文或密钥。
- 是否所有 SQL/source 变化都已 `make sqlc`，生成物通过 `make check-generated`。

## 14. 当前不纳入本次修复的事项

- 不建立完整宝付结算账户本地收入总账。
- 不改变宝付官方回调 ACK 规则。
- 不把提现创建同步受理结果解释为提现终态。
- 不新增前端公开字段，除非产品明确要求展示本地预占金额。
- 不自动修复历史账实不一致；本次先建立强一致新写入路径和审计暴露。
