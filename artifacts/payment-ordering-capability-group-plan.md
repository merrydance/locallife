# 支付下单组实施计划

## 1. 目标

基于微信支付官方“支付下单组”文档，以及仓库中已落地的商户进件组、商户注销组实现模式，把当前支付下单能力收口为一个完整、可审计、可恢复、可验证的能力组闭环。

本计划不把支付下单组视为从零开发，而是把现有普通支付、合单支付、查单、关单、支付通知、超时恢复这些散点能力，按能力组方式重新对齐、补齐、验收。

当前建议按 G3 风险等级执行，因为该链路直接涉及支付创建、状态查询、关闭、回调重复投递、超时恢复和资金状态一致性。

## 2. 参考模式

本计划明确复用商户进件组和商户注销组已经验证过的实现方式。

- 能力组优先：先确认官方能力组边界，再落代码，不按单个接口零散修补。
- contracts 先行：请求、响应、状态、枚举、错误码先集中沉淀，再让调用链依赖这些沉淀。
- 本地 durable anchor 优先：先有本地持久化真值，再处理微信调用、回调、轮询、恢复。
- API 只做传输边界：handler 负责绑定、鉴权、错误映射、响应整形，不承载业务决策。
- 异步恢复闭环：提交后必须有 query、callback、timeout、recovery 的端到端路径，而不是只做 create。
- 契约漂移显式失败：微信返回结构、状态或错误码超出已知集合时，应视为可见风险，不静默吞掉。

## 3. 当前基线

当前仓库中，支付下单组已经存在较多实现基础，但还没有完全按能力组方式收口。

### 3.1 已有能力沉淀

- 支付 contracts 已按能力组拆分到 [locallife/wechat/contracts/partner_ordering_single.go](locallife/wechat/contracts/partner_ordering_single.go) 和 [locallife/wechat/contracts/combine_ordering.go](locallife/wechat/contracts/combine_ordering.go)
- ordering 错误码已集中到 [locallife/wechat/errorcodes/ordering.go](locallife/wechat/errorcodes/ordering.go)
- 平台收付通 client 已支持普通支付、合单支付、查单、关单、通知解密，实现在 [locallife/wechat/ecommerce.go](locallife/wechat/ecommerce.go)
- 普通支付业务编排在 [locallife/logic/payment_order_service.go](locallife/logic/payment_order_service.go)
- 合单支付业务编排在 [locallife/logic/combined_payment_service.go](locallife/logic/combined_payment_service.go)
- API 入口在 [locallife/api/payment_order.go](locallife/api/payment_order.go)
- 普通支付与合单支付回调入口在 [locallife/api/payment_callback.go](locallife/api/payment_callback.go)
- 合单超时恢复任务在 [locallife/worker/task_combined_payment_timeout.go](locallife/worker/task_combined_payment_timeout.go)

### 3.2 当前主要缺口

- 还没有像进件组、注销组那样先产出一份正式的能力矩阵，明确“官方文档页面 -> 当前代码入口 -> 缺口”
- 虽然 ordering 已有 contracts 和 errorcodes，但仍需按官方口径重新审计完整性，避免字段、状态、错误码只做到“能用”而没做到“可治理”
- 普通支付和合单支付虽然都可用，但 API 语义、本地状态真值、远端查询真值、回调恢复语义仍分散在多个层中，需要统一
- 普通支付与合单支付的异步恢复路径完整度不一致，需要按高风险链路统一验收口径
- 需要明确哪些场景只做审计与补齐，哪些场景才需要新增库表字段或 migration

## 4. 范围

### 4.1 本次纳入范围

- 平台收付通普通支付创建
- 平台收付通普通支付查询
- 平台收付通普通支付关闭
- 平台收付通普通支付支付通知
- 平台收付通合单创建
- 平台收付通合单查询
- 平台收付通合单关闭
- 平台收付通合单支付通知
- 与上述链路直接相关的本地持久化、状态收敛、回调幂等、超时恢复、错误码映射、测试覆盖

### 4.2 暂不纳入范围

- 分账、分账回退、退款、异常退款的重构
- 账单下载、资金账户、提现、投诉的实现扩展
- 直连支付链路的全面重构
- 前端页面大改

## 5. 实施阶段

## 阶段 0：能力矩阵与差异审计

### 目标

先把支付下单组官方能力拆清楚，形成和进件组、注销组同样的“能力组矩阵 + 缺口清单”。

### 核心任务

- 对照支付下单组官方文档，逐项梳理普通支付与合单支付的 create、pay sign、query、close、notify
- 为每个官方接口建立映射：官方文档、client 方法、logic 调用点、api 入口、worker 恢复点、测试覆盖点
- 标记每个接口的状态枚举、错误码集合、条件必填项是否已在 contracts 和 errorcodes 中落地
- 标记每条链路的本地 durable anchor 是什么，是否足以支持 callback、query、close 和 config drift 后恢复

### 涉及文件

- [locallife/wechat/interface.go](locallife/wechat/interface.go)
- [locallife/wechat/ecommerce.go](locallife/wechat/ecommerce.go)
- [locallife/wechat/contracts/partner_ordering_single.go](locallife/wechat/contracts/partner_ordering_single.go)
- [locallife/wechat/contracts/combine_ordering.go](locallife/wechat/contracts/combine_ordering.go)
- [locallife/wechat/errorcodes/ordering.go](locallife/wechat/errorcodes/ordering.go)
- [locallife/logic/payment_order_service.go](locallife/logic/payment_order_service.go)
- [locallife/logic/combined_payment_service.go](locallife/logic/combined_payment_service.go)
- [locallife/api/payment_order.go](locallife/api/payment_order.go)
- [locallife/api/payment_callback.go](locallife/api/payment_callback.go)

### 产出物

- 一份支付下单组能力矩阵
- 一份缺口清单，按“契约缺口 / 状态缺口 / 异步恢复缺口 / 持久化缺口 / API 语义缺口”分类

### 验收标准

- 能明确回答每个官方接口当前在仓库中的入口和责任层
- 能明确指出哪些点是“已实现但未能力组化”，哪些点是“真正缺失”
- 后续实现阶段不再依赖口头判断，而是以矩阵作为唯一工作清单

## 阶段 1：contracts 与 errorcodes 收口

### 目标

把支付下单组所有官方契约和错误码沉淀补齐到和进件组、注销组同等治理水平。

### 核心任务

- 审核普通支付 contracts 是否完整表达官方请求和响应字段
- 审核合单 contracts 是否完整表达官方请求、查询、关闭、通知字段
- 审核所有 query 和 callback 中的状态、金额、payer、scene、promotion 等字段是否为显式命名类型，而不是实现内部匿名结构
- 审核 ordering 错误码集合是否按 create、query、close、combine create、combine query、combine close 分组完整收口
- 把历史兼容错误码保留在 canonicalization 层，避免 logic 和 api 层继续写魔法字符串

### 涉及文件

- [locallife/wechat/contracts/partner_ordering_single.go](locallife/wechat/contracts/partner_ordering_single.go)
- [locallife/wechat/contracts/combine_ordering.go](locallife/wechat/contracts/combine_ordering.go)
- [locallife/wechat/ordering_contract_aliases.go](locallife/wechat/ordering_contract_aliases.go)
- [locallife/wechat/combine_ordering_contract_aliases.go](locallife/wechat/combine_ordering_contract_aliases.go)
- [locallife/wechat/errorcodes/ordering.go](locallife/wechat/errorcodes/ordering.go)
- [locallife/wechat/ecommerce.go](locallife/wechat/ecommerce.go)

### 验收标准

- 官方下单组每个接口的请求和响应结构都能在 contracts 包中找到稳定定义
- 所有 ordering 错误码都通过 errorcodes 包统一引用
- wechat client 内不再维护分散、重复、不可复用的 ordering 结构定义
- 合同漂移会在 client 测试或查询逻辑中显式暴露，而不是静默容忍

## 阶段 2：普通支付主链收口

### 目标

把普通支付 create、query、close 的业务主链按能力组方式统一，确保本地状态锚点、微信查询、回调归属、商户配置变化后的恢复逻辑一致。

### 核心任务

- 审核普通支付创建路径的幂等、并发重试、prepay_id 落库、失败回滚和旧单 supersede 语义
- 明确 payment_orders 中哪些字段是本地真值，哪些字段仅作远端快照
- 固化普通支付 query 逻辑使用 transaction_id 还是 out_trade_no 的优先顺序
- 核对 close 调用时 sub_mchid 解析来源是否稳定，避免 merchant payment config 变化后 query 或 close 漂移
- 检查 attach 中的 sub_mchid、业务标识、归属校验字段是否满足 callback 和恢复需要

### 涉及文件

- [locallife/logic/payment_order_service.go](locallife/logic/payment_order_service.go)
- [locallife/api/payment_order.go](locallife/api/payment_order.go)
- [locallife/db/query/payment_order.sql](locallife/db/query/payment_order.sql)
- [locallife/db/sqlc/tx_create_ecommerce_payment.go](locallife/db/sqlc/tx_create_ecommerce_payment.go)
- [locallife/wechat/ecommerce.go](locallife/wechat/ecommerce.go)

### 验收标准

- 普通支付 create/query/close 的 sub_mchid 来源单一且可恢复
- payment_order 在 pending、paid、closed、failed 之间的状态迁移可解释且幂等
- 并发创建不会让旧请求把已关闭本地单重新“救活”
- 商户支付配置变化后，旧 payment_order 仍能完成 query、close、callback ownership 校验

## 阶段 3：合单支付主链收口

### 目标

把合单支付 create、query、close、timeout recovery 整体拉齐到可恢复、可对账、可审计状态。

### 核心任务

- 审核合单创建事务与微信创建调用的边界，确认并发冲突和 prepay_id 回填失败时的本地补偿动作
- 审核 combined_payment_orders 与 payment_orders 的关系是否足以支撑回调处理和远端补查
- 检查合单 query 结果的状态归类是否覆盖 paid、partial、closed、failed、refunded、mixed 等分支
- 检查超时任务是否在远端已支付、部分支付、异常状态时做正确分支处理，而不是一律关单
- 检查 close combine order 时对子单列表、sub_mchid、out_trade_no 的构造是否严格符合官方要求

### 涉及文件

- [locallife/logic/combined_payment_service.go](locallife/logic/combined_payment_service.go)
- [locallife/worker/task_combined_payment_timeout.go](locallife/worker/task_combined_payment_timeout.go)
- [locallife/db/sqlc/tx_create_ecommerce_payment.go](locallife/db/sqlc/tx_create_ecommerce_payment.go)
- [locallife/wechat/ecommerce.go](locallife/wechat/ecommerce.go)
- [locallife/api/payment_order.go](locallife/api/payment_order.go)

### 验收标准

- 合单创建失败、本地 prepay 更新失败、超时未支付、远端部分支付、远端已支付但本地未落账，均有明确处理路径
- 超时任务不会误关已经远端支付成功的合单
- 合单 query 返回结果可以直接支撑前端恢复支付与后端对账判断
- 合单与子单的状态收敛规则可被测试覆盖，不依赖人工经验

## 阶段 4：支付通知与异步恢复闭环

### 目标

把普通支付与合单支付的通知链路、重复回调幂等、落库事务、失败重试和恢复任务统一到同一风险口径。

### 核心任务

- 审核普通支付通知的签名校验、归属校验、重复投递 claim、支付成功事务处理
- 审核合单支付通知的 combine_mchid、combine_appid、sub_order 归属校验
- 检查回调先到、本地 query 后到，和本地下单成功、回调失败重试之间的时序安全性
- 检查普通支付 timeout 任务与合单 timeout 任务在远端已支付时的补偿逻辑是否一致
- 审核任何 callback 失败分支是否都有可恢复入口，而不是只记日志

### 涉及文件

- [locallife/api/payment_callback.go](locallife/api/payment_callback.go)
- [locallife/worker/task_payment_timeout.go](locallife/worker/task_payment_timeout.go)
- [locallife/worker/task_combined_payment_timeout.go](locallife/worker/task_combined_payment_timeout.go)
- [locallife/db/sqlc/tx_payment_success.go](locallife/db/sqlc/tx_payment_success.go)
- [locallife/logic/payment_order_service.go](locallife/logic/payment_order_service.go)
- [locallife/logic/combined_payment_service.go](locallife/logic/combined_payment_service.go)

### 验收标准

- callback 重复投递不会造成重复落账、重复状态迁移或重复业务副作用
- ownership 校验失败会显式告警并返回可重试响应，不会误记成功
- timeout、query、callback 三条路径处理同一笔单时，最终状态不会互相覆盖出错
- 所有关键分支都有单测或集成测试证明

## 阶段 5：API 契约与用户恢复语义统一

### 目标

让用户侧和前端消费到的支付状态语义统一，不再把业务决策散落在多个 handler 或前端推断里。

### 核心任务

- 统一 create/query/close 接口对 pending、paid、closed、failed、expired 的表达
- 明确什么时候返回 pay_params，什么时候只返回远端状态，什么时候返回“请重试”
- 审核普通支付与合单支付 response 结构是否足够对称，便于前端恢复支付
- 收敛兼容字段语义，避免 payment_type 这类历史字段继续误导调用方
- 若接口语义变化，补 Swagger 和 API 测试说明

### 涉及文件

- [locallife/api/payment_order.go](locallife/api/payment_order.go)
- [locallife/api/server.go](locallife/api/server.go)
- [locallife/api/payment_order_test.go](locallife/api/payment_order_test.go)

### 验收标准

- 前端不需要依赖隐式规则就能恢复普通支付或合单支付
- 同一类状态在普通支付和合单支付上的 API 语义一致
- handler 中不存在未下沉到 logic 的支付业务判断

## 阶段 6：持久化补强与生成链路

### 目标

只有在前五个阶段确认存在 durable gap 时，才补 migration、query、sqlc 生成物。

### 决策原则

- 默认不新增字段
- 只有本地无法稳定保存 callback ownership、query routing、幂等恢复、配置漂移后追踪所需信息时，才加字段
- 新字段必须能直接解释某个恢复缺口，不为“可能以后有用”而加

### 可能涉及文件

- [locallife/db/migration](locallife/db/migration)
- [locallife/db/query](locallife/db/query)
- [locallife/db/sqlc](locallife/db/sqlc)

### 验收标准

- 每个新增字段都有明确缺口对应关系
- 所有 SQL 变更都完成 sqlc 生成和回归验证
- 不引入只写不用、只存不查的漂移字段

## 阶段 7：验证、发布与回归

### 最小验证集合

- wechat client 契约测试
- logic 服务测试
- API handler 测试
- 支付回调测试
- timeout / recovery 测试
- 至少一条普通支付主链集成验证
- 至少一条合单支付主链集成验证

### 优先执行的测试文件

- [locallife/wechat/ecommerce_test.go](locallife/wechat/ecommerce_test.go)
- [locallife/logic/payment_order_service_test.go](locallife/logic/payment_order_service_test.go)
- [locallife/logic/combined_payment_service_test.go](locallife/logic/combined_payment_service_test.go)
- [locallife/api/payment_order_test.go](locallife/api/payment_order_test.go)
- [locallife/api/payment_callback_test.go](locallife/api/payment_callback_test.go)
- [locallife/worker/task_combined_payment_timeout_test.go](locallife/worker/task_combined_payment_timeout_test.go)
- [locallife/integration/takeout_journey_integration_test.go](locallife/integration/takeout_journey_integration_test.go)

### 推荐命令

- make test-unit
- make test-integration
- make swagger
- make check-generated

### 发布前验收标准

- 下单组能力矩阵中的高优先级缺口已关闭
- create/query/close/callback/timeout recovery 形成完整闭环
- 普通支付与合单支付的 API 语义一致性得到确认
- 没有新增未验证的高风险状态分支

## 6. 实施顺序建议

建议按以下顺序推进，避免一开始就改库或改 API，扩大爆炸半径。

1. 先完成阶段 0，产出能力矩阵和缺口清单
2. 再完成阶段 1，锁定 contracts 与 errorcodes
3. 然后并行推进阶段 2 和阶段 3，分别清普通支付和合单支付主链
4. 主链清完后统一处理阶段 4，把 callback 和 recovery 拉齐
5. 最后做阶段 5 和阶段 6，收敛 API 语义与持久化缺口
6. 阶段 7 作为每阶段穿插执行的验证门，不要拖到最后一次性补测

## 7. 完成定义

支付下单组完成，不以“接口能调通”为标准，而以以下条件全部满足为准。

- 官方下单组接口都有清晰的仓库内实现映射
- contracts、errorcodes、client、logic、api、worker 之间职责边界清晰
- 普通支付和合单支付都具备 create、query、close、callback、timeout recovery 完整闭环
- 关键状态迁移有测试保护
- 配置漂移、重复回调、远端先成功本地后补偿等高风险场景有明确处理
- 新增结构、字段、SQL 或 API 语义都能说明“为什么现在必须存在”