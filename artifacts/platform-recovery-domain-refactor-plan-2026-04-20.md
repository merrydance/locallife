# 平台追偿域拆分与重构实施计划

## 1. 文档目的

本文不是继续审计现状，而是把已经确认的业务边界、技术债与改造顺序收敛成一份可执行计划。

目标只有四个：

1. 把顾客索赔和平台向责任方追偿从语义和实现上拆开。
2. 把“先赔后追”变成模型事实，而不是入口校验。
3. 把现有 `behavior_actions` 和 `claim_recovery_events` 从半成品状态扶正为正式执行与审计底座。
4. 用分阶段方式实施，避免一次性大改导致资金链、回调链和恢复链失控。

配套参考：

1. [artifacts/abnormal-order-claim-redesign-intent-and-refactor-plan-2026-04-20.md](artifacts/abnormal-order-claim-redesign-intent-and-refactor-plan-2026-04-20.md)
2. [artifacts/abnormal-order-customer-claim-business-chain-design-2026-04-20.md](artifacts/abnormal-order-customer-claim-business-chain-design-2026-04-20.md)
3. [artifacts/merchant-transfer-compensation-capability-group-plan.md](artifacts/merchant-transfer-compensation-capability-group-plan.md)

## 2. 问题定义

当前实现的主要问题不是“追偿功能不存在”，而是它被包裹在顾客索赔主链内部，形成了三类结构性耦合：

1. 语义耦合
顾客向平台索赔，与平台向责任方追偿，是两个方向相反、参与方不同、状态机不同的业务对象；当前却共用 `claim` / `appeal` / `compensation` 语义壳。

2. 写边界耦合
平台赔付动作和追偿单创建都在同一批 claim compensation 事务里生成，导致“先赔后追”只能靠运行时校验兜底，而不是靠对象生命周期天然成立。

3. 副作用耦合
一部分后处理走 `behavior_actions`，一部分直接在 logic / tx / worker 里写 SQL，造成 action 表与真实执行链分裂，event 表与真实状态迁移分裂。

## 3. 必须锁定的不变量

后续所有阶段都不得违反以下规则：

1. 顾客索赔不是平台追偿，二者不共享主状态机。
2. 平台赔付完成之前，不允许责任方向平台付款。
3. 平台赔付完成之前，不应生成正式可支付的追偿单。
4. 支付完成是责任方恢复服务能力的正常主路径。
5. 申诉通过不是独立“解禁业务”，而是追偿状态变更，随后由统一封禁判定收敛结果。
6. 是否应封禁商户或骑手，必须由追偿域统一判定，不允许各入口直接解封。
7. `behavior_actions` 是副作用的唯一执行面。
8. `claim_recovery_events` 是追偿生命周期的唯一事件账本。
9. 所有高风险状态迁移都必须具备幂等、恢复与可审计语义。

## 4. 目标领域模型

## 4.1 顾客索赔域

建议对象：`CustomerClaimCase`

职责：

1. 顾客异常反馈提交
2. 订单、时效、归属校验
3. 判责输入与顾客可见状态
4. 顾客与平台之间的主张记录

不负责：

1. 责任方追偿支付
2. 责任方封禁与解封
3. 责任方争议处理

## 4.2 平台赔付域

建议对象：`CompensationCase`

职责：

1. 消费判责结果
2. 创建平台赔付 durable anchor
3. 驱动 payout action、回调、query recovery、scheduler recovery
4. 收敛赔付终态

不负责：

1. 责任方付款入口
2. 责任方逾期治理
3. 责任方争议状态机

## 4.3 平台追偿域

建议对象：`RecoveryOrder`

建议未来命名：`claim_recovery_orders`

职责：

1. 记录被追偿主体
2. 记录追偿依据、金额、来源赔付
3. 驱动责任方支付
4. 驱动逾期治理与封禁
5. 维护自身状态机与可运营状态投影

这是本次重构的核心领域对象。

## 4.4 追偿争议域

建议对象：`RecoveryDispute`

职责：

1. 责任方对追偿的异议
2. 追偿复核
3. 追偿豁免、恢复、补偿
4. 面向责任方和平台的争议审计

它不应继续以“顾客索赔申诉”的语义出现。

## 5. 目标状态机

## 5.1 CustomerClaimCase

建议主状态：

1. `pending_platform_review`
2. `adjudicated`
3. `awaiting_compensation`
4. `compensating`
5. `compensated`
6. `warned_waiting_customer_confirmation`
7. `restricted_compensated`
8. `closed`

## 5.2 CompensationCase

建议主状态：

1. `created`
2. `queued`
3. `running`
4. `wait_user_confirm`
5. `succeeded`
6. `failed`
7. `cancelled`

关键约束：只有 `succeeded` 才能触发 RecoveryOrder 创建。

## 5.3 RecoveryOrder

建议主状态：

1. `created`
2. `payable`
3. `pending_payment`
4. `overdue`
5. `paid`
6. `disputed`
7. `waived`
8. `closed`

状态语义建议：

1. `created`：追偿事实已经成立，但尚未开放付款入口。
2. `payable`：平台赔付成功，正式开放责任方付款。
3. `pending_payment`：责任方已创建支付单但尚未完成。
4. `overdue`：已超期未结清，触发封禁治理。
5. `paid`：责任方支付完成。
6. `disputed`：责任方已发起争议，暂挂追偿推进。
7. `waived`：平台撤销追偿。
8. `closed`：终态归档。

## 5.4 RecoveryDispute

建议主状态：

1. `submitted`
2. `under_review`
3. `approved`
4. `rejected`
5. `closed`

## 6. 保留与重写决策

## 6.1 应保留并扶正的底座

1. `behavior_decision`
2. `behavior_trace_snapshots`
3. `behavior_decision_effects`
4. `behavior_actions`
5. `claim_recovery`
6. `claim_recovery_events`

保留理由不是因为它们已经设计完毕，而是因为它们已经把“判责事实”“赔付动作”“追偿对象”“事件痕迹”分成了正式持久化对象，适合继续演进。

## 6.2 必须重写的部分

1. RecoveryOrder 的创建时机
2. 追偿付款入口对 claim 语义的依赖
3. 申诉与追偿争议的命名和边界
4. 直接 SQL 驱动副作用的路径
5. 只写 `created` 的事件账本实现

## 6.3 不应继续扩散的旧模式

1. 在 claim handler / logic 中继续增加追偿状态分支
2. 在各 tx / worker / api 里直接调用 `UnsuspendMerchantTakeout` 或 `UnsuspendRider`
3. 继续把追偿争议建模为 claim appeal 的附属物
4. 继续把 payout 和 recovery 同时创建成正式执行对象

## 7. 分阶段实施计划

## 阶段 0：边界冻结与术语收口

目标：先冻结不变量和术语，防止后续边改边漂。

核心任务：

1. 确认顾客索赔、平台赔付、平台追偿、追偿争议四个对象边界。
2. 明确旧命名到新对象的映射关系。
3. 确认“支付完成自动解禁”是主路径；申诉通过只是状态变更，不再被建模为独立解禁主业务。

产出：

1. 本计划文档
2. 重构术语表
3. 需要保留的旧对象清单

完成标准：

1. 不再出现“追偿是不是算索赔的一部分”的歧义。
2. 后续 PR 可以用同一套术语描述改动目标。

## 阶段 1：RecoveryOrder 创建后移

目标：让“先赔后追”变成对象生命周期事实。

核心任务：

1. 将正式 RecoveryOrder 的创建从 `CreateClaimCompensationTx` 中移出。
2. 让 `FinalizeClaimCompensationAfterPayoutTx` 或 payout success 后置事务成为 RecoveryOrder 的唯一创建点。
3. 在平台赔付未成功前，最多只能存在“待创建追偿事实”或 decision snapshot，不得存在正式 payable recovery。

涉及层：

1. `db/sqlc/tx_claim_behavior.go`
2. `worker/task_claim_refund.go`
3. payout callback / query recovery 路径

完成标准：

1. 平台赔付失败、取消、等待用户确认时，不会出现正式可支付追偿单。
2. payout success 与 RecoveryOrder 创建之间的因果关系被稳定持久化。

## 阶段 2：追偿状态机独立化

目标：让 RecoveryOrder 从 claim 语义外壳中独立出来。

核心任务：

1. 逐步把 `GetClaimForAppeal` 语义替换为 RecoveryOrder 自有读模型。
2. 明确 RecoveryOrder 的查询、权限与支付前置条件。
3. 把 claim 侧状态与 recovery 侧状态分离，不再把 recovery 可支付性建立在 claim status 名称上。

涉及层：

1. `logic/claim_recovery.go`
2. `logic/claim_recovery_payment.go`
3. `api/claim_recovery.go`
4. `db/query/appeal.sql`

完成标准：

1. 责任方支付追偿时，不再依赖 claim appeal 语义查询。
2. RecoveryOrder 自身成为权限、状态和支付的真值对象。

## 阶段 3：副作用执行面统一到 behavior_actions

目标：把 `behavior_actions` 从装饰性记录变成唯一副作用执行面。

核心任务：

1. 定义追偿域 action 类型集合。
2. 把逾期封禁、提醒通知、状态后处理、争议通过后的补偿/恢复全部改成 action 驱动。
3. 删除或下沉直接在 logic / tx / worker 中做副作用的路径。

建议 action 类型：

1. `recovery_open`
2. `recovery_notify`
3. `recovery_block_target`
4. `recovery_release_target`
5. `recovery_compensate`

完成标准：

1. 副作用执行与重试只有一套路径。
2. `behavior_actions` 不再只是“写出来但没人消费”的对象。

## 阶段 4：事件账本补齐到 claim_recovery_events

目标：让 `claim_recovery_events` 成为追偿生命周期真相。

核心任务：

1. 定义事件集。
2. 把所有关键状态迁移都写 event。
3. 明确事件 payload 结构，避免后续又回到当前 status 猜历史。

最低事件集建议：

1. `created`
2. `payable`
3. `payment_started`
4. `paid`
5. `overdue`
6. `disputed`
7. `waived`
8. `resumed`
9. `closed`

完成标准：

1. 任何一笔追偿都可以从 event 流还原生命周期。
2. 运营履历、排障、审计都不再依赖分散状态猜测。

## 阶段 5：追偿争议从 claim appeal 语义中抽离

目标：不再把责任方争议挂在顾客索赔申诉语义下。

核心任务：

1. 抽出 RecoveryDispute 读写入口。
2. 明确争议提交、审核、通过、驳回的状态迁移。
3. 保留 claim appeal 仅用于顾客索赔域，不再继续承载责任方争议。

完成标准：

1. 商户/骑手发起的是“追偿争议”，不是“顾客索赔申诉”。
2. 命名、路由、表结构、worker 语义保持一致。

## 阶段 6：收口旧入口与遗留逻辑

目标：删掉会继续制造边界混乱的旧实现。

需要收口的内容：

1. claim appeal 语义下残留的 recovery 写入口
2. 直接解封路径
3. 无执行器的 recovery / merchant / rider block action 旧残留
4. 旧文案与旧状态字段映射

完成标准：

1. 新旧路径不再并行写同一状态。
2. 代码阅读时不会再误把顾客索赔和平台追偿视为同一链。

## 8. 数据与迁移策略

## 8.1 命名迁移策略

不建议在第一阶段就立即重命名数据库表；优先先改业务边界和写路径，再在边界稳定后做 schema rename 或包装。

原因：

1. 当前高风险点在行为语义，不在表名本身。
2. 先大改表名会放大迁移面和回滚难度。

## 8.2 双读单写策略

建议采用“双读单写，逐阶段收口”：

1. 旧读模型在短期内保留只读兼容。
2. 新状态推进只允许写入新规则。
3. 等新 worker、回调、恢复链稳定后，再删除旧写入口。

## 8.3 存量数据回补

至少评估以下回补：

1. 已有 recovery 是否需要补 `payable` 事件。
2. 已有 overdue / appealed / waived / paid recovery 是否需要补 event 流。
3. 历史被直接解封的主体是否有残留错误状态，需要脚本校正。

## 9. 回归与验证策略

## 9.1 每阶段都要覆盖的验证面

1. request path
2. transaction path
3. payment callback
4. worker retry
5. scheduler recovery
6. idempotency
7. terminal failure handling

## 9.2 关键回归集

至少要有以下测试：

1. 平台赔付未成功时不能创建正式 payable recovery。
2. payout success 后才创建 RecoveryOrder。
3. 责任方不能在 payout success 前付款。
4. 同一主体存在多笔阻塞性追偿时，支付一笔不会误解封。
5. `behavior_actions` 失败、重试、重复投递不会导致重复封禁或重复解封。
6. `claim_recovery_events` 能完整覆盖生命周期。
7. 争议通过不会重复补偿或重复恢复。

## 9.3 建议命令

1. `make sqlc`
2. `make mock`
3. `go test ./logic`
4. `go test ./worker`
5. `go test ./db/sqlc`
6. `go test ./api -run '^$'`
7. 在阶段 1 和阶段 3 后补 `make test-unit`

## 10. PR 拆分建议

建议按以下顺序拆 PR，而不是一次性大改：

1. PR-1：阶段 0 文档、术语、不变量收口
2. PR-2：RecoveryOrder 创建后移
3. PR-3：追偿状态机独立化
4. PR-4：behavior_actions 执行面统一
5. PR-5：claim_recovery_events 事件账本补齐
6. PR-6：追偿争议域抽离与旧入口清理

每个 PR 的关闭条件都必须包含：

1. 新路径可独立运行
2. 旧写入口不再继续漂移
3. 有明确回归集
4. 没有新增半成品对象

## 11. 非目标

本计划当前不包含以下内容：

1. 一次性更名所有已有表和代码包名
2. 把整个异常订单售后体系拆成独立服务
3. 顺手重构所有顾客索赔判责算法
4. 在未完成 recovery 域拆分前就删除现有 payout 底座

这些内容不是永远不做，而是不应抢在 recovery 域边界稳定之前做。

## 12. 实施顺序结论

正确顺序是：

1. 先冻结不变量和术语
2. 再后移 RecoveryOrder 创建时机
3. 再独立 RecoveryOrder 状态机
4. 再统一副作用执行面
5. 再补齐事件账本
6. 最后抽离争议域并删除旧入口

如果顺序反过来，例如先删旧 action、先删申诉逻辑、先改表名，都会提高状态漂移和运行时断链风险。

这份计划应作为后续平台追偿域重构的唯一执行基线。