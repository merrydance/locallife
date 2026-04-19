# 商家转账企业赔付能力组实施计划

## 1. 目标

基于微信支付官方“直连支付-商家转账”能力组文档，以及当前平台“顾客索赔进入免举证平台兜底赔付后，由平台先向顾客赔付，再向责任方生成追偿单，责任方后续向平台付款”的新业务边界，建立一套完整、可审计、可恢复、可验证的商家转账能力实现计划。

本计划明确分离两条资金链：

- 平台向顾客赔付：改用商家转账能力组，不再沿用原有余额赔付或旧版 batch 转账语义。
- 责任方向平台付款：继续沿用现有直连支付小程序支付链路，不在本计划内推翻。

当前建议按 G3 风险等级执行，因为该链路直接涉及平台出资、异步终态、重复通知、用户确认收款、失败原因处理、恢复任务和后续追偿的资金归责。

## 2. 本次设计纠偏结论

当前仓库里已经存在一套“平台赔付到零钱”的转账抽象，但它不是这次业务可继续依赖的项目内真值。

### 2.1 当前已有基础

- 转账接口边界已存在于 [locallife/wechat/interface.go](locallife/wechat/interface.go)
- 当前转账实现位于 [locallife/wechat/payment.go](locallife/wechat/payment.go)
- 当前赔付 worker 链位于 [locallife/worker/task_claim_refund.go](locallife/worker/task_claim_refund.go)
- 运行时 wiring 已存在于 [locallife/main.go](locallife/main.go) 和 [locallife/api/server.go](locallife/api/server.go)

### 2.2 当前实现为什么不能继续作为真值

当前实现仍是旧版 batch 语义，核心特征是：

- 使用 out_batch_no、batch_id、batch_status
- 使用路径 /v3/transfer/batches
- 以“批次”和“明细单”视角建模

但这次业务所依据的官方升级版商家转账能力组真值是：

- 发起转账使用 out_bill_no、transfer_bill_no、state、package_info
- 企业赔付场景固定要求 transfer_scene_id = 1011
- 企业赔付场景报备信息固定为一条，info_type 必须是“赔付原因”
- 用户收款感知只允许“退款”或“商家赔付”
- 转账结果通知的终态和失败原因以升级版单据语义返回

因此，当前 batch 风格转账实现必须被完整删除并由升级版商家转账实现原地替换，不能继续保留为遗留路径、旁路实现或回退方案。

## 3. 官方能力组真值范围

本计划以 [README.md](README.md) 和 [.github/standards/domains/wechat-payment/README.md](.github/standards/domains/wechat-payment/README.md) 中已登记的官方链接为仓库内能力组入口，以微信官方页面为唯一外部真值。

### 3.1 首批必须对齐的官方页面

- 产品介绍
- 开发指引
- 发起转账
- JSAPI 调起用户确认收款
- 商户单号查询转账单
- 微信单号查询转账单
- 商家转账回调通知
- 订单失败原因说明
- 转账场景报备信息字段传参说明-企业赔付

### 3.2 二期再接入的官方页面

- 撤销转账
- 商户单号申请电子回单
- 商户单号查询电子回单
- 微信单号申请电子回单
- 微信单号查询电子回单
- 下载电子回单

说明：二期页面仍需要在 contracts 层登记能力边界，但不要求在第一批业务闭环中全部实现到 caller 链路。

## 4. 实施原则

本能力组必须沿用当前仓库其他支付能力组已经验证过的实现顺序，不能再走“先写 client，再靠业务代码猜契约”的路径。

### 4.1 固定顺序

1. 先映射 contracts
2. 再做 official audit
3. 再收口 errorcodes
4. 再实现 client
5. 再实现 callback、query、scheduler recovery
6. 最后接入业务流程

### 4.2 强约束

- 商家转账能力组的请求、响应、状态、枚举、错误码必须先沉淀到 [locallife/wechat/contracts](locallife/wechat/contracts) 和 [locallife/wechat/errorcodes](locallife/wechat/errorcodes)
- client 层不得继续维护一套只在实现文件内可见的匿名结构作为事实来源
- handler、logic、worker、scheduler 不得直接写微信返回状态字符串或错误码字符串
- business integration 只能依赖已审计过的 contracts 和 errorcodes
- 如果 audit 尚未确认 contracts 完整性，则实现阶段必须阻塞，不能宣称能力组已完成
- 不保留旧版 batch 转账实现，不保留兼容层，不保留双写或双读窗口，不保留过渡性 alias 或旁路状态解释
- 新 contracts 落地后，调用链必须一次性切到 canonical 商家转账语义，不能允许旧字段继续在 worker、logic、api 中存活

## 5. 当前仓库可复用模式

### 5.1 contracts 与 validation 模式

- 直连支付下单契约模式见 [locallife/wechat/contracts/direct_payment_ordering.go](locallife/wechat/contracts/direct_payment_ordering.go)
- 下单校验模式见 [locallife/wechat/contracts/direct_payment_validation.go](locallife/wechat/contracts/direct_payment_validation.go)
- 平台收付通下单契约模式见 [locallife/wechat/contracts/partner_ordering_single.go](locallife/wechat/contracts/partner_ordering_single.go)
- query 和 callback typed contract error 模式见 [locallife/wechat/contracts/ordering_validation.go](locallife/wechat/contracts/ordering_validation.go)

### 5.2 errorcodes 模式

- capability group canonical errorcodes 模式见 [locallife/wechat/errorcodes/ordering.go](locallife/wechat/errorcodes/ordering.go)
- 官方 audit 对齐说明见 [locallife/wechat/errorcodes/doc.go](locallife/wechat/errorcodes/doc.go)
- 使用 CONTRACT_NOT_CONFIRMED 作为未确认契约阶段占位的模式见 [locallife/wechat/errorcodes/fund_management.go](locallife/wechat/errorcodes/fund_management.go)

### 5.3 official audit 模式

- official extraction 审计测试模式见 [locallife/internal/wechatdoc/fund_management_audit_test.go](locallife/internal/wechatdoc/fund_management_audit_test.go)

### 5.4 运行时分层模式

- 传输边界和 client 注入模式见 [locallife/api/server.go](locallife/api/server.go)
- runtime wiring 和 recovery scheduler 注册模式见 [locallife/main.go](locallife/main.go)
- 当前赔付 outbox 与恢复模式见 [locallife/worker/task_claim_refund.go](locallife/worker/task_claim_refund.go)

## 6. 能力组范围划分

### 6.1 本次纳入范围

- 企业赔付场景 contracts 映射
- 企业赔付场景 official audit
- 企业赔付相关 errorcodes 审计与 canonicalization
- 发起转账 client 实现
- 商户单号查询 client 实现
- 微信单号查询 client 实现
- 商家转账回调通知解密与验签接入
- 赔付业务 durable anchor
- worker 和 scheduler 恢复
- 用户确认收款主链所需的 package_info 透传

### 6.2 首批业务闭环暂不纳入范围

- 撤销转账真实业务接入
- 电子回单申请、查询、下载
- 面向运营的完整财务工具页
- 对现有责任方向平台付款链路的重构

## 7. 阶段计划

## 阶段 0：能力组矩阵与现状差异审计

### 目标

先把官方商家转账能力组和当前仓库错误实现的差异完全拉平，形成唯一工作清单。

### 核心任务

- 对照官方页面逐项梳理 create、confirm、query、notify、cancel、receipt 的能力边界
- 建立“官方页面 -> contracts 文件 -> client 方法 -> caller 入口 -> recovery 入口 -> 测试覆盖点”的映射矩阵
- 显式标记当前 batch 版实现与升级版商家转账语义的字段漂移、状态漂移和 endpoint 漂移
- 确认第一批业务闭环只依赖哪些页面，哪些页面仅做 contracts 登记但延后 caller 接入

### 产出物

- 一份商家转账能力矩阵
- 一份差异清单，按“契约缺口 / 状态缺口 / 错误码缺口 / callback 缺口 / durable anchor 缺口 / 前端确认收款缺口”分类

### 验收标准

- 能明确回答每个官方接口当前在仓库中的落点或缺口
- 能明确说明哪些点必须在 contracts 阶段完成，哪些点可延后到业务 integration 阶段

## 阶段 1：contracts 映射

### 目标

先把商家转账能力组所需的 canonical contracts 落到项目内，作为后续所有实现的唯一真值来源。

### 计划文件

- 新增 [locallife/wechat/contracts/direct_merchant_transfer.go](locallife/wechat/contracts/direct_merchant_transfer.go)
- 新增 [locallife/wechat/contracts/direct_merchant_transfer_validation.go](locallife/wechat/contracts/direct_merchant_transfer_validation.go)
- 新增 [locallife/wechat/contracts/direct_merchant_transfer_validation_test.go](locallife/wechat/contracts/direct_merchant_transfer_validation_test.go)

### 必须覆盖的 contracts

- 发起转账 caller-shaped request
- 发起转账官方 wire request body
- 发起转账 response
- 商户单号查询 response
- 微信单号查询 response
- 回调外层 envelope
- 回调解密后 resource
- 企业赔付场景枚举与常量
- state 枚举
- fail_reason 字段枚举边界占位
- package_info 与 WAIT_USER_CONFIRM 相关字段

### 必须固化的常量

- transfer_scene_id = 1011
- 默认用户收款感知 = 退款
- 可选用户收款感知 = 商家赔付
- 企业赔付报备 info_type = 赔付原因
- 官方状态集合
- 官方回调 event_type 与 resource_type

### contracts 设计要求

- 先有 caller-shaped contract，再在同文件或同能力组文件中补官方 wire contract
- query、notify 必须返回 typed contract error，而不是普通 error string
- 任何金额、时间、状态、枚举都不能留在 client 层隐式解析
- package_info 作为业务主链关键字段，必须进入 canonical response contract

### 验收标准

- 后续 client 实现不需要再自定义本地匿名结构
- 业务层和 worker 层可以只依赖 contracts 包完成状态分支
- contracts 本身能表达企业赔付场景的固定规则，不依赖上层散落拼接

## 阶段 2：official audit 与 errorcodes 收口

### 目标

确认 contracts 确实已经覆盖官方真值，并把错误码集合沉淀成 canonical errorcodes。

### 计划文件

- 新增 [locallife/internal/wechatdoc/merchant_transfer_audit_test.go](locallife/internal/wechatdoc/merchant_transfer_audit_test.go)
- 新增 [locallife/wechat/errorcodes/merchant_transfer.go](locallife/wechat/errorcodes/merchant_transfer.go)

### audit 范围

- 发起转账 endpoint、字段、状态、错误码
- 商户单号查询 endpoint、字段、状态、错误码
- 微信单号查询 endpoint、字段、状态、错误码
- 商家转账回调 resource 字段
- 企业赔付场景字段要求
- 失败原因说明页面的 fail_reason 集合

### errorcodes 收口要求

- 以官方文档列出的错误码为 DocumentedCodes
- merchant transfer errorcodes 只保留官方确认过的 canonical codes，不引入兼容拼写映射，不为了“兼容历史调用”扩散旧字符串
- 对于尚未完成官方确认的子接口，阶段内应继续补 audit 直到确认；不允许以长期占位方式跳过契约确认

### 阶段 gate

这个阶段通过之前，不允许进入 client 接口实现。

### 验收标准

- internal wechatdoc audit 能明确证明 contracts 与官方页面对齐
- errorcodes 包成为项目内唯一错误码真值来源
- 不再需要在 payment.go、worker、logic 里手写转账状态或错误码字符串

## 阶段 3：client 接口实现替换

### 目标

在 contracts 和 audit 确认后，直接删除当前错误的 batch 风格转账实现，并以升级版商家转账 client 原地替换。

### 涉及文件

- [locallife/wechat/interface.go](locallife/wechat/interface.go)
- [locallife/wechat/payment.go](locallife/wechat/payment.go)
- [locallife/wechat/payment_test.go](locallife/wechat/payment_test.go)
- [locallife/wechat/mock/payment_client.go](locallife/wechat/mock/payment_client.go)

### 关键任务

- 重写 TransferClientInterface 的 request 和 response 语义
- 实现发起转账 client
- 实现按商户单号查询 client
- 实现按微信单号查询 client
- 实现商家转账回调资源解密与 contract validation
- 保留与直连 JSAPI 支付接口的边界隔离，不能把 TransferClientInterface 再并回 PaymentClientInterface
- 删除 payment.go 中旧版 /v3/transfer/batches 相关 endpoint、request、response、状态解释与测试

### 注意事项

- 运行时仍可共用同一个商户 payment runtime client，但 interface 视图必须分离
- 不允许继续使用 out_batch_no、batch_status、batch_id 作为商家转账业务真值
- 旧测试必须同步重写，不允许为了保测试通过而保留兼容层或旧语义分支

### 验收标准

- client 只依赖 contracts 和 errorcodes
- payment.go 中不存在新的匿名转账结构定义
- mock 生成物与接口边界同步

## 阶段 4：赔付 durable anchor 与异步恢复闭环

### 目标

把平台向顾客赔付的本地真值从“仅有一条行为动作 + 错误 batch 字段”彻底替换为可支撑升级版商家转账状态机的 durable anchor。

### 关键判断

当前 [locallife/worker/task_claim_refund.go](locallife/worker/task_claim_refund.go) 的 outbox 和恢复模式可复用，但 detail 结构和状态解释不能继续沿用 batch 语义。

这一阶段不是在旧字段外面再包一层映射，而是直接删除旧字段语义，统一切到新单据语义。

### 核心任务

- 重新定义 payout action detail 中的商家转账字段
- 持久化 out_bill_no、transfer_bill_no、state、package_info、fail_reason、scene id、scene report infos
- 重写 running、success、failed、wait_user_confirm、processing 之间的状态收敛逻辑
- 用商户单号查询作为默认 recovery 主键，用微信单号查询作为补充入口
- 调整 scheduler 恢复逻辑，确保未收到回调时可以兜底查询

### 涉及文件

- [locallife/worker/task_claim_refund.go](locallife/worker/task_claim_refund.go)
- [locallife/worker/claim_refund_recovery_scheduler.go](locallife/worker/claim_refund_recovery_scheduler.go)
- [locallife/worker/processor.go](locallife/worker/processor.go)
- [locallife/main.go](locallife/main.go)

### 验收标准

- 同一笔赔付不会因重复创建、重复通知、重复恢复而重复出资
- WAIT_USER_CONFIRM 和 SUCCESS、FAIL、CANCELLED 等终态可以被正确区分和持久化
- 未收到回调时，query 路径能完成状态收敛

## 阶段 5：回调、确认收款与业务流程接入

### 目标

把商家转账能力真正接入“免举证平台兜底赔付”业务流程。

### 核心任务

- 新增商家转账回调 handler，完成验签、解密、幂等、快速应答与异步落库
- 在赔付创建流程里返回 package_info，并向上游调用方暴露 WAIT_USER_CONFIRM 语义
- 为小程序或上游调用链预留“调起用户确认收款”所需 contract，而不是把 package_info 留在底层 worker 中沉没
- 把赔付成功后的“生成平台向责任方追偿单”与转账成功终态严格绑定
- 保留原责任方向平台付款的直连支付链路，不与本能力组合并重构

### 业务接入要求

- 顾客索赔进入平台兜底赔付后，只能先创建商家转账单
- 只有商家转账进入成功终态后，才允许把平台赔付视为已完成并触发后续追偿单生成
- 若停留在 WAIT_USER_CONFIRM，不得提前生成“已赔付完成”语义
- 若失败，应根据 fail_reason 给出清晰运营语义，而不是简单标记系统失败

### 涉及文件

- [locallife/api/server.go](locallife/api/server.go)
- [locallife/api](locallife/api)
- [locallife/logic](locallife/logic)
- [locallife/worker/task_claim_refund.go](locallife/worker/task_claim_refund.go)

### 验收标准

- payout create、callback、query、scheduler recovery 形成闭环
- 赔付完成与追偿单生成之间的因果关系被稳定建模
- 前端或调用侧可以消费 WAIT_USER_CONFIRM 和 package_info

## 阶段 6：二期能力补齐

### 二期范围

- 撤销转账
- 电子回单申请与查询
- 电子回单下载
- 面向运营的凭证查询与人工处理工具

### 原则

- 二期能力仍须先走 contracts -> audit -> errorcodes -> client -> business 的顺序
- 不因为首批已上线，就跳过 contracts 审计流程

## 8. 建议的 PR 拆分

为降低风险，建议按以下顺序拆分独立 PR：

1. PR-1：商家转账 capability matrix 与 contracts 映射
2. PR-2：merchant transfer official audit 与 errorcodes
3. PR-3：TransferClientInterface 和 client 实现替换
4. PR-4：赔付 durable anchor 与 recovery 重写
5. PR-5：callback 和业务流程接入
6. PR-6：确认收款前端接入和结果反馈

这样可以确保每个阶段都在 review 时有明确关闭条件，避免“一次性大改支付链路”。

每个 PR 的关闭条件都包含“删除被替换的旧实现”，而不是“新旧并存后再清理”。

## 9. 验证策略

### 9.1 contracts 阶段

- 运行商家转账 validation test
- 运行 internal wechatdoc audit test

### 9.2 client 阶段

- 运行 [locallife/wechat](locallife/wechat) 下的定向测试
- 如果修改接口定义，运行 make mock

### 9.3 业务集成阶段

- 运行 [locallife/worker](locallife/worker) 相关测试
- 运行 [locallife/api](locallife/api) 相关测试
- 至少补一条 DB-backed 集成测试，覆盖 create -> wait_user_confirm 或 success -> callback 或 query recovery -> payout success

### 9.4 生成链路判断

- 预计本能力组首批不需要 make sqlc，除非 durable anchor 最终需要新字段或新表
- 若新增 webhook 路由或 swagger 注释变化，则需要 make swagger
- 若接口变更影响 mocks，则需要 make mock

## 10. 提示词系统路由建议

这项工作应按你们当前提示词系统这样推进：

1. 用 [.github/prompts/backend-payment-domain.prompt.md](.github/prompts/backend-payment-domain.prompt.md) 发起阶段 0 到阶段 5 的总任务
2. 用 [.github/prompts/backend-phase-batch-implementation.prompt.md](.github/prompts/backend-phase-batch-implementation.prompt.md) 逐阶段实施
3. 每个 PR 或任务卡用 [.github/prompts/backend-task-card-implementation.prompt.md](.github/prompts/backend-task-card-implementation.prompt.md) 控制范围
4. 需要前端确认收款页时，再切到 [.github/prompts/weapp-implementation.prompt.md](.github/prompts/weapp-implementation.prompt.md)

## 11. 当前执行顺序建议

基于本计划，当前最合理的起手式是：

1. 先新增商家转账 contracts 和 validation 文件
2. 紧接着补 internal wechatdoc 审计与 errorcodes
3. 等 audit 通过后，再动 [locallife/wechat/payment.go](locallife/wechat/payment.go) 和 [locallife/wechat/interface.go](locallife/wechat/interface.go)
4. client 稳定后，再改 [locallife/worker/task_claim_refund.go](locallife/worker/task_claim_refund.go) 的 durable anchor 与业务流程

在这之前，不建议直接修改现有赔付 worker 逻辑，否则会把错误的 batch 语义继续扩散到业务层。

最终落地要求是：新能力上线时，仓库里不再保留任何旧 batch 商家转账路径、兼容字段解释或兼容测试分支。