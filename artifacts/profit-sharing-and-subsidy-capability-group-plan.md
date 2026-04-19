# 补差与分账组实施计划

## 1. 目标

基于微信支付官方“补差与分账组”文档，以及仓库中已落地的商户进件组、商户注销组、支付下单组实施方式，把当前分账、分账回退、分账结果通知、接收方管理、完结解冻、剩余可分账金额查询，以及项目内依附该资金链路的补差能力，收口为一个完整、可审计、可恢复、可验证的能力组闭环。

本计划不把补差与分账组视为从零开发，而是把当前已经存在的分账 worker、分账回退、分账回调、分账配置、补差 API、补差表结构、退款前回退逻辑等散点能力，按能力组方式重新对齐、补齐、验收。

当前建议按 G3 风险等级执行，因为该链路直接涉及资金二次分配、资金解冻、营销补贴出资、退款前资金回收、接收方身份、回调重复投递、异步恢复和本地财务真值一致性。

## 2. 参考模式

本计划明确复用商户进件组、商户注销组和支付下单组已经验证过的实现方式。

- 能力组优先：先确认官方能力组边界，再落代码，不按单个接口零散修补。
- contracts 先行：请求、响应、状态、枚举、错误码先集中沉淀，再让调用链依赖这些沉淀。
- canonical 直连：调用链直接依赖能力组的 canonical contracts 与 errorcodes，不引入 type alias 兼容层。
- 本地 durable anchor 优先：先有本地持久化真值，再处理微信调用、回调、轮询、恢复。
- API 只做传输边界：handler 负责绑定、鉴权、错误映射、响应整形，不承载业务决策。
- 异步恢复闭环：提交后必须有 query、callback、polling、scheduler、recovery 的端到端路径，而不是只做 create。
- 契约漂移显式失败：微信返回结构、状态或错误码超出已知集合时，应视为可见风险，不静默吞掉。
- 错误语义明确：所有失败都要落日志，并给前端或运营端明确语义，不做模糊降级处理。

## 3. 当前基线

当前仓库中，补差与分账组已经存在较多实现基础，但还没有完全按能力组方式收口。

### 3.1 已有能力沉淀

- 官方分账相关 client 能力已在 [locallife/wechat/ecommerce.go](locallife/wechat/ecommerce.go) 实现，包括请求分账、查询分账结果、查询剩余待分账金额、完结分账、添加接收方、删除接收方、请求分账回退、查询分账回退结果、分账通知解密。
- 平台收付通接口定义已在 [locallife/wechat/interface.go](locallife/wechat/interface.go) 暴露，包括分账与补差方法集合。
- 支付下单链路已经能在 [locallife/wechat/contracts/partner_ordering_single.go](locallife/wechat/contracts/partner_ordering_single.go) 和 [locallife/wechat/contracts/combine_ordering.go](locallife/wechat/contracts/combine_ordering.go) 透传 profit_sharing 与 subsidy_amount 结算字段。
- 分账主执行链路已经在 [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go) 落地，包括分账金额计算、接收方补齐、请求分账、完结分账、结果处理、失败告警、自动重试。
- 分账恢复调度器已经在 [locallife/worker/profit_sharing_recovery_scheduler.go](locallife/worker/profit_sharing_recovery_scheduler.go) 落地，包括 failed 或 stale 分账单重试、已完成订单缺失分账单补偿、processing 分账回退卡死补偿。
- 分账结果回调入口已经在 [locallife/api/payment_callback.go](locallife/api/payment_callback.go) 落地，并已接入 [locallife/api/server.go](locallife/api/server.go) webhook 路由。
- 分账回退与退款联动已经在 [locallife/logic/refund_service.go](locallife/logic/refund_service.go) 落地，包括先回退分账再退款、模糊处理中转为轮询、个人接收方回退阻断。
- 分账规则配置已经在 [locallife/api/profit_sharing_config.go](locallife/api/profit_sharing_config.go) 和 [locallife/db/query/profit_sharing_config.sql](locallife/db/query/profit_sharing_config.sql) 落地。
- 分账持久化真值已经存在于 [locallife/db/query/profit_sharing_order.sql](locallife/db/query/profit_sharing_order.sql) 与 [locallife/db/query/profit_sharing_return.sql](locallife/db/query/profit_sharing_return.sql)。
- 补差 client 已在 [locallife/wechat/subsidy.go](locallife/wechat/subsidy.go) 实现 create、return、cancel。
- 补差 API 已在 [locallife/api/subsidy.go](locallife/api/subsidy.go) 落地，持久化真值已在 [locallife/db/query/subsidy_order.sql](locallife/db/query/subsidy_order.sql) 落地。

### 3.2 当前主要缺口

- 还没有像进件组、注销组、支付下单组那样先产出一份正式的能力矩阵，明确“官方文档页面 -> 当前代码入口 -> 缺口”。
- 分账 contracts、状态、错误码仍主要内嵌在 [locallife/wechat/ecommerce.go](locallife/wechat/ecommerce.go) 中，尚未像支付下单组那样收口为独立 capability-group contracts 与 errorcodes 包。
- 分账主链虽然已可运行，但“请求分账 / 查询分账 / 回调 / 完结解冻 / 查询剩余金额 / 失败恢复”的真值边界仍分散在 worker、api、refund 和 db 多层中，需要统一口径。
- 接收方管理已经存在，但 operator 与 rider 的接收方关系、receiver name 来源、是否需要预注册、失败后恢复方式仍需按官方分账能力组重新审计。
- 分账回调当前已接入，但 QueryProfitSharing 作为真值、回调缺字段时的 FAIL 重试语义、结果任务与 scheduler 的边界仍需能力组级验收。
- 分账 finish/unfreeze 语义已经存在，但哪些场景应请求真正分账、哪些场景只应 finish 解冻、哪些场景需要查询剩余待分账金额后再决策，仍需统一治理。
- 补差能力已经有 API 和 DB，但目前更像项目内扩展功能，尚未纳入完整能力组治理：没有正式矩阵、没有独立 contracts 或 errorcodes、没有 recovery 闭环、没有测试面、与退款和取消的顺序要求尚未完整审计。
- 当前没有发现补差测试文件，说明该链路尚未形成与风险等级相匹配的验证保护。
- 分账配置表与统计接口已经存在，但它们不能替代对微信资金链路闭环本身的能力组验收。

## 4. 范围

### 4.1 本次纳入范围

- 分账开发指引及业务示例与现有实现映射
- 请求分账
- 查询分账结果
- 请求分账回退
- 查询分账回退结果
- 完结分账并解冻剩余资金
- 查询订单剩余待分账金额
- 添加分账接收方
- 删除分账接收方
- 分账动账通知
- 分账失败处理与自动恢复
- 与上述链路直接相关的本地持久化、状态收敛、接收方关系、回调幂等、scheduler 恢复、告警与测试覆盖
- 项目内补差 create、return、cancel，以及其与 payment order settle_info、退款链路、分账时序的收口

### 4.2 暂不纳入范围

- 平台与商户财务报表页面的大改
- 账单下载能力的新扩展，只校验当前分账账单下载入口是否与主链一致
- 直连支付链路重构
- 对现有业务分润比例做产品策略调整

## 5. 实施阶段

## 阶段 0：能力矩阵与差异审计

### 目标

先把官方分账能力和项目内补差扩展拆清楚，形成和支付下单组同样的“能力组矩阵 + 缺口清单”。

### 核心任务

- 对照微信支付“补差与分账组”文档，逐项梳理分账 create、query、return、return query、finish、amount query、receiver add、receiver delete、notify。
- 明确补差不是凭印象并入分账，而是作为当前项目实际依附在分账结算链路上的本地能力扩展，单独标注其入口、状态机和缺口。
- 为每个官方接口建立映射：官方文档、client 方法、worker 或 logic 调用点、api 入口、scheduler 恢复点、测试覆盖点。
- 标记每条链路的 durable anchor 是什么，分别确认 payment_orders、profit_sharing_orders、profit_sharing_returns、subsidy_orders 各自承担的真值角色。
- 标记当前补差链路与支付下单 settle_info subsidy_amount 的关系是否闭环，是否存在“下单声明补差、后续没有本地真值”的漂移。

### 涉及文件

- [locallife/.github/standards/domains/wechat-payment/README.md](locallife/.github/standards/domains/wechat-payment/README.md)
- [locallife/wechat/interface.go](locallife/wechat/interface.go)
- [locallife/wechat/ecommerce.go](locallife/wechat/ecommerce.go)
- [locallife/wechat/subsidy.go](locallife/wechat/subsidy.go)
- [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go)
- [locallife/worker/profit_sharing_recovery_scheduler.go](locallife/worker/profit_sharing_recovery_scheduler.go)
- [locallife/api/payment_callback.go](locallife/api/payment_callback.go)
- [locallife/api/subsidy.go](locallife/api/subsidy.go)
- [locallife/logic/refund_service.go](locallife/logic/refund_service.go)

### 产出物

- 一份补差与分账组能力矩阵
- 一份缺口清单，按“契约缺口 / 状态缺口 / 接收方缺口 / 异步恢复缺口 / 持久化缺口 / API 语义缺口 / 测试缺口”分类

### 验收标准

- 能明确回答每个官方分账接口当前在仓库中的入口和责任层
- 能明确说明补差能力在当前项目中是如何附着到支付与退款链路上的
- 后续实现阶段不再依赖口头判断，而是以矩阵作为唯一工作清单

## 阶段 1：contracts、errorcodes 与 client 收口

### 目标

把分账与补差所有正式契约和错误码沉淀补齐到与支付下单组同等治理水平。

### 核心任务

- 将分账请求、响应、接收方、查询结果、回调结构、回退结构从 [locallife/wechat/ecommerce.go](locallife/wechat/ecommerce.go) 中收口为 capability-group contracts。
- 为分账与补差建立独立 errorcodes 收口，避免 logic、worker、api 继续通过 err.Error 或散落字符串分支。
- 审核补差 create、return、cancel 的请求响应结构是否需要迁入 capability-group contracts，而不是继续只保留在 [locallife/wechat/subsidy.go](locallife/wechat/subsidy.go)。
- 统一 client 层的字段校验、空 body 容错、状态枚举解析与未知状态显式失败策略。
- 不引入 type alias 兼容层，调用链直接切到 canonical contracts。

### 涉及文件

- [locallife/wechat/ecommerce.go](locallife/wechat/ecommerce.go)
- [locallife/wechat/subsidy.go](locallife/wechat/subsidy.go)
- [locallife/wechat/interface.go](locallife/wechat/interface.go)
- [locallife/wechat/contracts](locallife/wechat/contracts)
- [locallife/wechat/errorcodes](locallife/wechat/errorcodes)

### 验收标准

- 官方分账组每个接口的请求和响应结构都能在 contracts 包中找到稳定定义
- 分账与补差错误码都通过 errorcodes 包统一引用
- wechat client 内不再维护散落、重复、只在实现文件可见的结构定义
- 契约漂移会在 client 测试或调用链中显式暴露，而不是静默容忍

## 阶段 2：分账主链与接收方关系收口

### 目标

把请求分账主链按能力组方式统一，确保本地计算真值、接收方关系、幂等重试和 finish=true 收尾语义一致。

### 核心任务

- 审核 [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go) 中分账金额计算的真值来源，确认 total_amount、delivery_fee、distributable_amount、platform/operator/rider/merchant 金额与官方语义一致。
- 审核 order_source、reservation、takeout、dine_in、takeaway 的分账策略分支，确认哪些场景真的发起分账，哪些场景只应 finish 解冻。
- 审核 rider、operator、merchant、platform 接收方关系，确认 receiver type、relation type、receiver name 来源和预注册策略符合当前项目主链路。
- 审核 existing profit_sharing_order 的重试语义，确认 pending、processing、failed、finished 之间不会产生重复分账或错误覆盖。
- 审核订单已支付但缺失 profit_sharing_order 的补偿创建路径，确认不会因为局部失败而永久漏分账。

### 涉及文件

- [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go)
- [locallife/worker/profit_sharing_recovery_scheduler.go](locallife/worker/profit_sharing_recovery_scheduler.go)
- [locallife/algorithm/profit_sharing_calculator.go](locallife/algorithm/profit_sharing_calculator.go)
- [locallife/db/query/profit_sharing_order.sql](locallife/db/query/profit_sharing_order.sql)
- [locallife/db/query/profit_sharing_config.sql](locallife/db/query/profit_sharing_config.sql)

### 验收标准

- 每个订单类型的分账决策路径可解释且有单一真值来源
- 接收方关系创建失败、已存在、缺实名、缺 openid 等分支都有明确处理语义
- 同一 payment_order 不会因重试或回调重复而重复创建有效分账结果
- 无实际接收方的链路会调用 finish/unfreeze，而不是伪造 receiver 回流

## 阶段 3：查询、通知、完结与恢复闭环

### 目标

把分账 query、notify、finish、remaining amount query、failure recovery 拉齐为可恢复、可对账、可审计的闭环。

### 核心任务

- 审核 QueryProfitSharing 与分账回调的真值边界，明确本地是否以 query 结果为准、回调缺字段时如何返回 FAIL 触发微信重试。
- 审核 finish-order 调用时机，确保“不需要继续分账”与“微信侧仍冻结资金”之间的关系被正确建模。
- 审核 QueryProfitSharingAmounts 在本项目中的使用位置，明确是否需要在 finish、retry、补差取消等场景前引入剩余可分账金额检查。
- 审核分账结果任务与 recovery scheduler 的协作边界，确保 FAILED、CLOSED、PROCESSING、SUCCESS 各状态都能走到稳定收敛。
- 审核告警与日志字段，确保资金链路错误可以直接定位 payment_order、profit_sharing_order、out_order_no、receiver 和 fail_reason。

### 涉及文件

- [locallife/api/payment_callback.go](locallife/api/payment_callback.go)
- [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go)
- [locallife/worker/profit_sharing_recovery_scheduler.go](locallife/worker/profit_sharing_recovery_scheduler.go)
- [locallife/db/query/profit_sharing_order.sql](locallife/db/query/profit_sharing_order.sql)
- [locallife/wechat/ecommerce.go](locallife/wechat/ecommerce.go)

### 验收标准

- 回调、查询、scheduler、人工重试不会对同一分账单产生互相覆盖的错误状态
- finish/unfreeze 不会被遗漏，也不会在需要实际分账时误提前执行
- unknown result 或 unknown receiver result 会显式失败并可见告警
- 分账回调重复投递、query 先后乱序、scheduler 重复扫描都能幂等收敛

## 阶段 4：分账回退与退款联动收口

### 目标

把退款前分账回退链路拉齐到与主分账链路同等强度的 durable、polling、failure handling 水平。

### 核心任务

- 审核 [locallife/logic/refund_service.go](locallife/logic/refund_service.go) 中先回退分账再退款的执行顺序，确保 refund order 与 profit_sharing_return 的真值关系明确。
- 审核个人接收方回退阻断、模糊错误转 processing、结果轮询和 max retries 行为，确保同步错误不会错误地终结高风险链路。
- 审核 stuck processing return 的补偿轮询逻辑，确认 DB 已 processing 但任务丢失时可以恢复。
- 审核回退成功后再触发退款、回退失败时 refund order 的失败语义和告警语义。
- 审核多接收方回退的拆分规则、out_return_no 幂等策略和 return_mchid 真值来源。

### 涉及文件

- [locallife/logic/refund_service.go](locallife/logic/refund_service.go)
- [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go)
- [locallife/worker/profit_sharing_recovery_scheduler.go](locallife/worker/profit_sharing_recovery_scheduler.go)
- [locallife/db/query/profit_sharing_return.sql](locallife/db/query/profit_sharing_return.sql)
- [locallife/wechat/ecommerce.go](locallife/wechat/ecommerce.go)

### 验收标准

- 回退处理中、成功、失败的本地状态迁移可解释且可恢复
- 回退轮询任务丢失不会造成永久 processing 卡死
- 退款不会绕过应先回退的分账资金
- 个人接收方不支持回退的场景会显式失败并告警，而不是静默跳过

## 阶段 5：补差主链与退款、取消联动收口

### 目标

把项目内补差能力从“有 API 可调用”收口为“有契约、有状态机、有恢复边界、有退款顺序要求”的正式能力面。

### 核心任务

- 审核 [locallife/api/subsidy.go](locallife/api/subsidy.go) 的幂等键、状态迁移、失败映射和日志语义，明确 pending、success、failed、canceled、pending_return、return_success、return_failed 的可见规则。
- 审核补差与支付下单 settle_info subsidy_amount 的关系，明确哪些场景应在下单时声明 subsidy_amount，哪些场景是在支付后单独创建 subsidy_order。
- 审核补差 return 与 refund 的顺序约束，确认“先回补差，再退分账，再退交易”是否需要被显式固化为统一流程。
- 审核补差 cancel 的前置条件，确认“尚未分账前可取消”在本地是否有足够的状态与查询支撑，而不是只依赖人工约定。
- 为补差能力补齐测试和必要的恢复路径；若官方没有完整 query 或 notify 接口，则明确残余风险与人工处理策略，而不是假装链路天然闭环。

### 涉及文件

- [locallife/api/subsidy.go](locallife/api/subsidy.go)
- [locallife/wechat/subsidy.go](locallife/wechat/subsidy.go)
- [locallife/db/query/subsidy_order.sql](locallife/db/query/subsidy_order.sql)
- [locallife/logic/refund_service.go](locallife/logic/refund_service.go)
- [locallife/logic/payment_order_service.go](locallife/logic/payment_order_service.go)
- [locallife/logic/combined_payment_service.go](locallife/logic/combined_payment_service.go)

### 验收标准

- 补差 create、return、cancel 的状态机有清晰本地真值和用户侧语义
- 补差不会与分账、退款顺序发生隐式冲突
- 补差 API 失败都能落日志并返回明确运营语义
- 补差至少具备与当前风险等级相匹配的 handler 或 logic 测试覆盖

## 阶段 6：持久化、API 语义与生成链路收口

### 目标

只有在前五个阶段确认存在 durable gap 时，才补 migration、query、sqlc 生成物与 API 契约。

### 决策原则

- 默认不新增字段
- 只有当前字段无法稳定保存接收方真值、query routing、finish/unfreeze 决策、补差取消前置条件、退款前资金回收顺序时，才加字段
- 新字段必须能直接解释某个恢复缺口，不为“可能以后有用”而加
- 若需要新增 SQL，则同步完成 sqlc 生成；若需要新增路由或 Swagger 注解，则同步完成 swagger 生成

### 可能涉及文件

- [locallife/db/migration](locallife/db/migration)
- [locallife/db/query](locallife/db/query)
- [locallife/db/sqlc](locallife/db/sqlc)
- [locallife/api/server.go](locallife/api/server.go)
- [locallife/api/payment_callback.go](locallife/api/payment_callback.go)
- [locallife/api/subsidy.go](locallife/api/subsidy.go)

### 验收标准

- 每个新增字段都有明确缺口对应关系
- 所有 SQL 变更都完成 sqlc 生成和回归验证
- 不引入只写不用、只存不查的漂移字段
- API 语义变化都会同步反映到测试与文档生成链路

## 阶段 7：验证、发布与回归

### 最小验证集合

- wechat client 契约测试
- 分账 worker 主链测试
- 分账回调测试
- 分账恢复 scheduler 测试
- 分账回退与退款联动测试
- 补差 handler 或 logic 测试
- 至少一条支付成功后触发分账的集成验证
- 至少一条退款前回退分账的集成验证

### 优先执行的测试文件

- [locallife/wechat/ecommerce_test.go](locallife/wechat/ecommerce_test.go)
- [locallife/worker/task_process_payment_profit_sharing_return_test.go](locallife/worker/task_process_payment_profit_sharing_return_test.go)
- [locallife/worker/profit_sharing_recovery_scheduler_test.go](locallife/worker/profit_sharing_recovery_scheduler_test.go)
- [locallife/api/payment_callback_test.go](locallife/api/payment_callback_test.go)
- [locallife/logic/refund_service_test.go](locallife/logic/refund_service_test.go)
- [locallife/db/sqlc/profit_sharing_order_recovery_test.go](locallife/db/sqlc/profit_sharing_order_recovery_test.go)
- [locallife/api/profit_sharing_config_test.go](locallife/api/profit_sharing_config_test.go)
- [locallife/api/operator_profit_sharing_config_test.go](locallife/api/operator_profit_sharing_config_test.go)

### 推荐命令

- make test-unit
- make test-integration
- make sqlc
- make swagger
- make check-generated

### 发布前验收标准

- 能力矩阵中的高优先级缺口已关闭
- 分账 create/query/notify/finish/return/recovery 形成完整闭环
- 补差 create/return/cancel 与支付、分账、退款顺序关系得到确认
- 没有新增未验证的高风险状态分支
- 任何无法本地验证的补差或分账高风险路径都被明确标记为残余风险，而非默认视为安全

## 6. 实施顺序建议

建议按以下顺序推进，避免一开始就改库或改 API，扩大爆炸半径。

1. 先完成阶段 0，产出能力矩阵和缺口清单。
2. 再完成阶段 1，锁定 contracts、errorcodes 与 client 边界。
3. 然后先推进阶段 2 和阶段 3，把分账主链、通知、finish 与 recovery 拉齐。
4. 主分账链稳定后推进阶段 4，收紧退款前回退闭环。
5. 再推进阶段 5，把补差纳入同一资金链路治理口径。
6. 最后做阶段 6，补持久化与 API 缺口。
7. 阶段 7 作为每阶段穿插执行的验证门，不要拖到最后一次性补测。

## 7. 完成定义

补差与分账组完成，不以“接口能调通”为标准，而以以下条件全部满足为准。

- 官方分账组接口都有清晰的仓库内实现映射。
- contracts、errorcodes、client、worker、logic、api、scheduler 之间职责边界清晰。
- 分账具备 create、query、notify、finish、return、return query、recovery 完整闭环。
- 补差具备 create、return、cancel 和退款顺序约束的明确本地真值。
- 关键状态迁移与接收方分支有测试保护。
- 接收方关系、重复回调、模糊处理中间态、finish 解冻遗漏、补差退款顺序冲突等高风险场景有明确处理。
- 新增结构、字段、SQL 或 API 语义都能说明“为什么现在必须存在”。