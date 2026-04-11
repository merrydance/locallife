# 微信支付改造执行计划（由当前代理执行）

## 1. 文档目的

本文件用于固定本次支付改造的目标、边界、任务顺序、验收标准和执行进度，避免在长上下文实现过程中出现目标漂移、阶段遗漏或中途跑偏。

约束如下：

- 本次改造由当前代理持续执行，不转交给其他代理完成实现。
- 本文件是本次改造期间的唯一执行基线，后续如有新增结论，必须先更新本文件再继续改代码。
- 所有任务必须满足“能勾选、能验收、能回滚”。
- 本次针对未上线业务做生产级净化改造，不保留旧模式、双路兜底或兼容性转账回退。

## 2. 最终目标

### 2.1 骑手押金链路

- 将骑手押金充值保留为微信直连支付。
- 将骑手押金提现从“微信零钱转账”改成“原支付单退款”。
- 建立骑手押金的可退款凭证模型，支持 365 天退款有效期管理。
- 支持到期提醒、过期状态管理、以及明确的过期后处理策略。

### 2.2 其他支付链路

- 订单支付、预订支付、预订加菜补差、会员充值、商户进件、商户提现、运营商提现、分账、分账回退、收付通退款继续走平台收付通。
- 接入独立的服务商配置，不能再默认复用直连商户配置。
- 清理订单支付 API 的对外歧义，保证接口语义与实际实现一致。

## 3. 当前确认的事实

- 当前骑手押金充值使用直连 JSAPI 支付。
- 当前骑手押金提现使用微信零钱转账，不是退款原支付单。
- 当前骑手提现不会创建 refund_order，也没有押金退款有效期管理。
- 当前订单和预订主支付虽然 API 仍暴露 `payment_type`，但实际已统一走平台收付通合单支付。
- 当前平台收付通客户端支持单独的 `SpMchID` 和 `SpAppID`，但服务初始化没有正式接入环境变量。

## 4. 不允许出现的歧义

- 不允许“先扣本地押金，再等外部退款结果”的账实错位逻辑。
- 不允许继续把“骑手提现”表述为退款式提现，但实际仍调用转账接口。
- 不允许让订单主支付接口继续在文档层面承诺 native 直连能力，除非代码真实支持。
- 不允许收付通在生产中继续依赖“默认复用直连商户号”这一隐式行为。
- 不允许为未上线的骑手押金提现保留旧转账模式、双写路径或运行期开关兜底。

## 5. 执行原则

- 先补配置和数据模型，再改主业务路径。
- 所有钱相关状态必须依赖可审计记录，不允许只改内存或只改一侧表。
- 能走回调落账的链路，不在主请求里伪造最终成功状态。
- 每个异步任务都要检查幂等性和失败补偿。
- 每个阶段完成后都要更新本文件中的进度状态。

## 6. 分阶段任务清单

### 阶段 A：固定业务规则

- [x] A1. 固定骑手押金退款有效期规则为：自原支付单支付成功时间起 365 天。
- [x] A2. 固定骑手押金退款顺序策略。
- [x] A3. 固定是否支持部分退款。
- [x] A4. 固定押金过期后的处理策略。
- [x] A5. 固定提醒节奏：30 天、7 天、1 天、当天。
- [x] A6. 固定订单支付 API 的对外语义，确认是否废弃 payment_type 的真实分流作用。

当前已固定规则：

- 退款有效期以原支付单 `paid_at` 为起点，统一为 365 天。
- 提现选券顺序按 `refundable_until ASC, id ASC` 执行，优先消耗最早到期的押金凭证。
- 支持部分退款：既支持单张押金凭证部分消耗，也支持一次提现拆分到多张原支付单退款。
- 提现前必须检查冻结状态：只要骑手存在冻结押金，则整笔不能提现，必须先解除冻结后再发起退款。
- 过期处理策略固定为：押金凭证进入 `expired` 后不再参与提现选择；历史 `legacy` 数据由阶段 G 单独治理，不通过旧转账路径兜底。
- 提醒节奏固定为 30 天、7 天、1 天、当天，并通过 `last_reminded_at` 控制幂等提醒。

当前已确认的冻结/解冻语义：

- 配送接单冻结：骑手抢单成功时，按订单冻结金额冻结押金，当前实现为优先取订单实付金额，否则回退到订单总额。
- 配送完成解冻：骑手确认送达或围栏自动确认送达后，按该单冻结金额原额解冻。
- 提现退款冻结：骑手发起押金退款提现时，先冻结本次申请金额，再按原支付单拆分创建 `refund_order`。
- 提现退款结算：退款成功时，冻结金额转为实际提现扣减；退款失败、异常或关闭时，冻结金额原路解冻并恢复押金凭证可退余额。
- 冻结互斥约束：当前业务规则已收紧为“只要存在冻结押金，就不能再发起新的押金提现”。
- 取消解冻边界：只有未取餐阶段的取消事务才允许同步撤销配送并解冻该单押金；一旦进入 `picked` / `delivering` 及之后阶段，不允许再通过取消订单来释放骑手押金。
- 微信退款 365 天规则本质是“原支付单退款 API 的可用时间窗口”，不是微信侧代商户冻结资金；平台侧仍可在本地账务中冻结、解冻、扣减押金，只是超过窗口后不能再对该原支付单发起原路退款。
- 当前骑手责任索赔主路径不是“自动从冻结押金扣款”，而是生成 `claim_recovery` 追偿单并暂停骑手接单，待骑手确认支付后恢复接单。
- 旧的“直接扣骑手押金并给用户退款/余额补偿”事务已移除；骑手/商户责任赔付统一走“平台先赔付 + `claim_recovery` 追偿 + 逾期暂停合作”的单一路径。

当前已识别风险：

- 仍需继续梳理取消链路以外的异常结束场景，但取消解冻现在只覆盖未取餐阶段；已取餐/配送中场景明确禁止通过取消订单释放押金，并已补充事务测试覆盖。
- 索赔/申诉维度当前主路径已收敛为追偿单，不再保留自动直扣骑手押金的生产事务；后续只需继续完善追偿闭环与异常回滚审计。

验收标准：

- 所有边界条件都有唯一规则，不存在“后面再看”的空白决策。

### 阶段 B：收付通配置接线

- [x] B1. 在 `util.Config` 中新增独立的收付通服务商配置字段。
- [x] B2. 在环境模板中补充正式配置说明。
- [x] B3. 在服务初始化中显式将 `SpMchID` 和 `SpAppID` 传给收付通客户端。
- [x] B4. 增加启动阶段配置校验。
- [ ] B5. 为配置接线增加测试。

验收标准：

- 收付通客户端运行时使用明确的服务商配置。
- 配置缺失时应用能在启动阶段失败。

### 阶段 C：骑手押金可退款凭证模型

- [x] C1. 设计骑手押金凭证表结构。
- [x] C2. 编写数据库迁移。
- [x] C3. 增加 sqlc 查询与更新语句。
- [x] C4. 固定凭证状态枚举。
- [x] C5. 为历史数据兼容预留 `legacy` 状态。

建议表字段：

- `id`
- `rider_id`
- `payment_order_id`
- `original_amount`
- `refundable_amount`
- `refunded_amount`
- `status`
- `paid_at`
- `refundable_until`
- `last_reminded_at`
- `expired_at`
- `created_at`
- `updated_at`

验收标准：

- 可以按骑手查有效押金凭证。
- 可以原子扣减可退款余额。
- 可以识别即将过期和已过期凭证。

### 阶段 D：充值成功后生成押金凭证

- [x] D1. 修改 `rider_deposit` 支付成功处理逻辑。
- [x] D2. 支付成功时同步创建押金凭证记录。
- [x] D3. 确保重复消费支付成功任务不会重复建凭证。
- [ ] D4. 为该逻辑增加事务和幂等测试。

验收标准：

- 每笔骑手押金成功支付后，都有对应的可退款凭证。

### 阶段 E：骑手提现改为退款式提现

- [x] E1. 提炼独立的骑手押金退款服务。
- [x] E2. 实现可退款凭证选择策略。
- [x] E3. 提现时创建 `refund_order` 而不是直接转账。
- [x] E4. 调用直连 `CreateRefund` 而不是 `CreateTransfer`。
- [x] E5. 成功或 processing 后通过退款状态驱动余额更新。
- [x] E6. 对 processing 状态接入退款回调落账。
- [x] E7. 删除旧零钱转账提现残留实现，不保留双模式兜底。
- [x] E8. 调整 API 返回语义，准确区分 success 与 processing。

验收标准：

- 骑手提现主路径不再依赖零钱转账。
- 每次提现都能追溯到 refund_order 和原支付单。

### 阶段 F：365 天到期提醒与过期管理

- [x] F1. 为每条押金凭证计算 `refundable_until`。
- [x] F2. 新建每日扫描任务。
- [x] F3. 在 30 天、7 天、1 天、当天发送提醒。
- [x] F4. 接入站内通知。
- [x] F5. 视情况接入订阅消息或运营告警。
- [x] F6. 过期后自动标记 `expired`。
- [x] F7. 实现过期后的提现策略。

验收标准：

- 即将到期押金能被稳定识别并提醒。
- 过期后系统行为一致且可审计。

### 阶段 G：历史数据回填

注：当前任务是上线前重构与生产级鲁棒性优化，不是线上存量系统升级；现阶段没有历史生产数据，因此阶段 G 默认不执行。

- [ ] G1. 梳理历史骑手押金支付单与当前余额的映射规则。
- [ ] G2. 编写可重复执行的回填脚本。
- [ ] G3. 无法精确映射的历史数据标记为 `legacy`。
- [ ] G4. 为 `legacy` 数据定义提现策略。
- [ ] G5. 完成回填结果校验。

验收标准：

- 回填后骑手押金余额和可退款凭证总额一致。

### 阶段 H：清理订单支付 API 歧义

- [x] H1. 调整订单支付 API 请求语义。
- [x] H2. 更新 Swagger 和设计文档。
- [x] H3. 对旧客户端兼容请求增加日志统计。

验收标准：

- 接口语义与真实实现一致。

### 阶段 I：收付通主链路鲁棒性补强

- [x] I1. 检查并补齐分账、退款、提现结果处理的统一告警字段。
- [x] I2. 审查任务入队失败后的补偿机制。
- [x] I3. 完善对账说明与运维操作说明。
- [x] I4. 补充异常到账、金额不一致等关键测试。

验收标准：

- 关键失败场景有日志、告警、补偿或人工介入路径。

### 阶段 J：测试、灰度与回滚

- [x] J1. 完成数据库层测试。
- [x] J2. 完成服务层测试。
- [x] J3. 完成退款回调落账集成测试。
- [x] J4. 完成到期提醒任务测试。
- [x] J5. 完成配置接线测试。
- [ ] J6. 在测试环境完成一次完整闭环联调。
- [x] J7. 准备生产发布顺序。
- [x] J8. 准备回滚方案和开关。

验收标准：

- 新旧链路可灰度切换。
- 回滚不依赖紧急手工修数。

## 7. 生产链路必须保持正确的业务归类

以下链路维持平台收付通：

- 订单支付
- 预订支付
- 预订加菜补差
- 会员充值
- 商户进件
- 商户提现
- 运营商提现
- 分账
- 分账回退
- 收付通退款

以下链路维持微信直连：

- 骑手押金充值
- 骑手押金退款式提现

## 8. 当前执行状态

- [x] 已确认现状与目标存在偏差。
- [x] 已确认骑手提现当前使用 `CreateTransfer`。
- [x] 已确认订单和预订主支付已统一走收付通。
- [x] 已确认收付通服务商配置尚未正式接线。
- [x] 阶段 A 已完成 A1-A6 的规则固定。
- [x] 阶段 B 已完成 B1-B4 的配置接线，B5 测试待补。
- [x] 阶段 C 已完成 C1-C5 的表结构、迁移、sqlc 查询和状态枚举落地。
- [x] 阶段 D 已完成 D1-D3 的支付成功凭证创建与事务幂等保护，D4 测试已编写但受测试库脏迁移阻塞。
- [x] 阶段 E 已完成 E1-E8，旧转账主路径与事务入口已清理。
- [x] 阶段 F 已完成 F1-F7；当前选择复用既有异步通知任务与平台运营告警通道，不额外新建微信订阅消息基础设施。
- [ ] 阶段 G 未开始编码。
- [x] 阶段 H 已完成 H1-H3。
- [x] 阶段 I 已完成 I1-I4。
- [ ] 阶段 J 已完成 J1-J5、J7-J8，J6 待测试环境执行。

## 9. 执行日志

### 2026-03-24

- 初始文档创建。
- 已固定总体目标和阶段划分。
- 已完成阶段 B 的配置结构接线、服务初始化传参、环境模板说明同步。
- 已将当前 `app.env` 中的收付通服务商配置从占位注释切换为正式配置。
- 已完成阶段 C 的完整基础落地：新增骑手押金可退款凭证表迁移、sqlc 查询、生成代码和状态枚举接入。
- 已完成阶段 D 的主事务改造：骑手押金支付成功后会在同一事务中幂等创建押金流水和可退款凭证。
- 已新增阶段 D 的数据库测试，覆盖凭证创建与幂等行为。
- `go test ./db/sqlc -run 'TestProcessPaymentSuccessTx_RiderDeposit(CreatesCredit|IsIdempotent)$'` 曾被本地测试库脏迁移阻塞：`Dirty database version 141`。
- `go test -c ./db/sqlc` 已通过，说明当前 `db/sqlc` 包和新增测试代码可以正常编译。
- 已明确本次为未上线业务的生产级净化改造，不保留旧转账兜底模式。
- 已提炼独立的骑手押金退款服务，统一 API 提现提交与退款结果结算入口。
- 已修复“请求内立即退款成功”场景下支付单 refunded 状态仅依赖回调补齐的问题。
- 已删除旧零钱转账提现事务入口与对应测试资产，不再保留双模式兜底。
- 已确认当前押金冻结存在两类来源：配送接单冻结与提现退款冻结；解冻出口分别依赖配送完成和退款结果结算。
- 已收紧取消订单事务边界：只有未取餐阶段才允许取消并同步解冻押金，已取餐/配送中场景明确禁止通过取消订单释放押金。
- 已删除旧的“直接扣骑手押金并给用户退款/余额补偿”事务，骑手责任赔付统一保留 `claim_recovery` 追偿单主路径。
- 已修复本地 `locallife_test` 测试库的迁移元数据错位：确认 141/142 等媒体迁移存在“DDL 已落库但 `schema_migrations` 脏/失真”的历史状态后，补跑缺失的幂等迁移、对齐版本线，并恢复 `make migrateup` 为稳定 `no change`。
- 已在 `Makefile` 增加 `migratestatus` 与 `migrateforce version=<n>` 辅助目标，便于后续诊断和修复本地迁移状态。
- 已在 `DataCleanupScheduler` 中接入骑手押金退款窗口每日扫描：当前会按 30 天、7 天、1 天、当天四个窗口发送提醒，并在到期后自动将押金凭证标记为 `expired`。
- 阶段 F5 已确定采用现有生产通道落地：用户提醒统一走异步 `TaskSendNotification` 链路，今日到期与已过期批次额外发布平台运营告警；当前不额外引入微信订阅消息发送体系。
- 已完成阶段 I1：统一退款、分账、商户提现失败告警的关键标识字段，当前告警 extra 会稳定携带 payment_order / refund_order / profit_sharing_order / withdrawal_record 级别的排障上下文；商户提现失败与查询重试耗尽也已补发平台告警。
- 已新增 worker 层测试，覆盖统一告警字段 helper 与商户提现失败告警发布。
- 已完成阶段 I2：支付回调主链路已补齐关键任务入队失败的指标与平台告警，覆盖合单子单金额不一致退款入队失败、合单支付成功任务入队失败、结算触发分账任务入队失败三类高风险场景；对应恢复仍依赖 payment/refund/profit-sharing/merchant-withdraw recovery scheduler 或人工介入。
- 已补充支付运行手册 `../WECHAT_PAYMENT_OPERATIONS_RUNBOOK_2026-03-24.md`，明确自动恢复链路、对账入口、核心告警类型与人工排障步骤，作为阶段 I3 交付物。
- 已完成阶段 I4：为支付回调补充关键回归测试，覆盖合单异常到账自动异常退款、合单金额不一致自动退款、合单支付成功任务入队失败仍返回 SUCCESS、结算触发分账任务入队失败仍返回 SUCCESS 四类高风险分支；`go test ./api -run 'TestHandleCombinePaymentNotify_(ClosedOrderEnqueuesAnomalyRefund|AmountMismatchEnqueuesRefund|PaymentSuccessEnqueueFailureStillReturnsSuccess)|TestHandleOrderSettlementNotify_ProfitSharingEnqueueFailureStillReturnsSuccess'` 已通过。
- 已完成阶段 H：订单支付 API 的 `payment_type` 已降为兼容保留字段，请求可省略；Swagger 已改为说明“按业务类型自动选择真实支付链路”；后端会对显式传入旧 `payment_type` 的请求记录兼容日志，小程序支付调用也已去掉该冗余字段。
- 已进一步收紧取消订单后的退款任务处理：当退款任务调度失败或未配置调度器时，`OrderService.CancelOrder` 会额外写入审计记录，明确标记依赖 refund recovery scheduler 继续补偿；新增 `go test ./logic -run 'TestOrderServiceCancelOrder_(RefundScheduleFailureWritesAudit|MissingSchedulerWritesAudit)'` 已通过。
- 已完成阶段 J1：新增骑手押金退款事务数据库测试，覆盖 `PrepareRiderDepositRefundTx` 的冻结预留/跨凭证拆分以及 `ResolveRiderDepositRefundTx` 的成功结算/关闭回滚；测试过程中暴露 `refund_orders.refund_type` 约束未放开 `rider_deposit` 的真实缺口，已通过迁移 `000163_expand_refund_order_types` 修复；`go test ./db/sqlc -run 'TestPrepareRiderDepositRefundTx_(ReservesCreditAndFreezesBalance|SplitsAcrossCredits)$|TestResolveRiderDepositRefundTx_(SuccessSettlesFrozenBalance|ClosedRestoresCreditAndUnfreezesBalance)$'` 已通过。
- 已完成阶段 J2：新增 `RiderDepositRefundService.SubmitWithdrawal` 服务层测试，覆盖“微信退款同步成功后更新 accepted/refunded 状态”和“微信退款申请失败后执行补偿回滚”两条关键编排路径；`go test ./logic -run 'TestRiderDepositRefundService_SubmitWithdrawal_(SynchronousSuccess|RefundRequestFailureCompensates)$'` 已通过。
- 已完成阶段 J3：新增骑手押金退款回调落账集成测试，覆盖“押金支付成功 -> 提现冻结 -> 退款结果任务成功结算 -> 支付单/refund order/credit/rider balance/rider deposit logs 全量落账”闭环；在补测过程中进一步暴露并修复两处真实 schema 约束缺口：`rider_deposits.related_order_id` 不能承载 `refund_order.id`，现已改为退款流水统一关联 `payment_order_id`；`rider_deposits.payment_order_id` 唯一索引已收窄为仅约束 `type='deposit'`，并同步将 `GetRiderDepositByPaymentOrderID` 查询限定为充值流水；`make sqlc` 已重新生成代码，`go test ./integration -run 'TestRiderDepositRefundCallbackAccountingIntegration$' -count=1 -p 1` 已通过。
- 已完成阶段 J4：现有 `DataCleanupScheduler` 已具备骑手押金到期提醒测试覆盖，包含“异步通知任务分发”“无 distributor 时直接通知 fallback 并发布平台告警”“到期凭证批量标记 expired 并发布告警”三类关键场景；`go test ./scheduler -run 'TestDataCleanupScheduler_(RemindExpiringRiderDepositCredits_DistributesNotificationTask|RemindExpiringRiderDepositCredits_FallbackDirectNotificationAndPublishAlert|MarkExpiredRiderDepositCredits)$'` 已通过。
- 已完成阶段 J5：新增 `util.LoadConfig` 配置接线测试，覆盖默认值、Redis 密码引号清洗、数据库连接池默认参数以及微信支付/收付通新增配置字段读取，确保 `WECHAT_PAY_*`、`WECHAT_ECOMMERCE_*`、`REDIS_REQUIRED` 等关键变量能稳定进入运行时配置对象；`go test ./util -run 'TestLoadConfig_(DefaultsAndTrimQuotes|ReadsWechatPaymentAndEcommerceConfig)$'` 已通过。
- 已完成阶段 J7/J8：已在 `../WECHAT_PAYMENT_OPERATIONS_RUNBOOK_2026-03-24.md` 中补充测试环境闭环执行清单、生产发布顺序、回滚优先级、数据库回滚原则与回滚触发条件，后续上线按 runbook 执行即可。
- 阶段 J6 仍需在真实测试环境执行一次闭环联调；阶段 G 因无历史数据暂不执行。
