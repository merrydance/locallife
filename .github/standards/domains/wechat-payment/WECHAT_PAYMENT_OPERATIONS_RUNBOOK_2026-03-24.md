# 微信支付主链路运行手册

## 1. 适用范围

本文覆盖以下生产主链路的日常巡检、异常对账与人工介入流程：

- 平台收付通支付
- 收付通退款
- 分账与分账回退
- 商户与运营商收付通提现回调、结果轮询与恢复
- 骑手押金退款式提现到期提醒与过期失效

目标不是替代代码逻辑，而是回答三个运维问题：

- 失败后系统会不会自动补偿
- 哪些异常必须人工跟进
- 发生问题后先看哪里

## 2. 自动恢复与定时任务矩阵

### 2.1 支付成功补偿

- payment callback 会把支付成功后续处理投递为 `payment:process_success`
- 若回调入队失败，当前会打指标并发布 `TASK_ENQUEUE_FAILURE` 平台告警
- 自动补偿依赖 `PaymentRecoveryScheduler`，默认每 5 分钟扫描 paid 但未完成后处理的 payment order 并重新入队

### 2.2 退款补偿

- 退款回调会把退款结果处理投递为 `payment:process_refund`
- 取消订单等链路若出现“应退未退”，自动补偿依赖 `RefundRecoveryScheduler`
- `RefundRecoveryScheduler` 默认每 5 分钟扫描已取消但未退款完成的订单并重新发起退款任务

### 2.3 分账补偿

- 支付成功后的分账主任务为 `payment:process_profit_sharing`
- 分账结果处理任务为 `payment:process_profit_sharing_result`
- 若结算回调触发分账任务入队失败，当前会发布 `TASK_ENQUEUE_FAILURE` 告警
- 自动补偿依赖 `ProfitSharingRecoveryScheduler`，默认每 10 分钟扫描失败或长时间未完成的分账单并重新入队

### 2.4 商户提现补偿

- 商户与运营商提现都会向微信上送专用回调地址 `/v1/webhooks/wechat-ecommerce/withdraw-notify`
- 微信 `MCHWITHDRAW.CHANGE` 回调会先验签、幂等 claim，并在关键状态与账户信息 durable 落库后再返回 success ack
- 商户提现结果查询任务为 `payment:process_merchant_withdraw_result`
- 自动补偿依赖 `MerchantWithdrawRecoveryScheduler`
- `MerchantWithdrawRecoveryScheduler` 默认每 3 分钟扫描 pending 提现并补发结果轮询任务，作为回调缺失、延迟或乱序时的兜底恢复
- 若查询重试耗尽或最终失败，当前会发布平台告警，extra 中会携带 withdrawal_record 级别的关键标识

### 2.5 骑手押金到期提醒与过期处理

- 每天 09:00 扫描 30 天、7 天、1 天、当天到期的押金凭证并发送提醒
- 每天 03:10 将已超过退款窗口的凭证标记为 expired
- 提醒优先走异步通知任务 `notification:send`，不可用时退化为直接写站内通知
- 当天到期与批量过期会额外发布 `RIDER_DEPOSIT_EXPIRY` 平台告警

### 2.6 每日对账

- 每天 10:10 自动执行 T-1 微信账单对账，给微信账单生成留出额外缓冲
- 覆盖 bill type：`trade`、`ecommerce_trade`、`refund`、`ecommerce_refund`
- 差异会写入 reconciliation reports，并发布 `BILL_MISMATCH` 平台告警

## 3. 日常巡检顺序

### 3.1 先看平台告警

优先关注以下告警类型：

- `TASK_ENQUEUE_FAILURE`
- `PROFIT_SHARING_FAILED`
- `REFUND_FAILED`
- `BILL_MISMATCH`
- `RIDER_DEPOSIT_EXPIRY`
- `SYSTEM_ERROR`

告警 extra 当前应优先检查这些字段：

- `payment_order_id`
- `refund_order_id`
- `profit_sharing_order_id`
- `withdrawal_record_id`
- `merchant_id`
- `out_trade_no`
- `out_refund_no`

### 3.2 再看对账报表

运营侧至少确认两类对账视图：

- 每日账单对账报告列表：平台账单与本地记录差异
- 分账对账汇总：确认分账单是否存在 pending、failed、closed 堆积

若当日出现支付投诉、退款异常或人工补单，优先看前一日和当日两天窗口。

### 3.3 最后看恢复调度器是否还在工作

若告警持续重复出现，需要确认恢复任务仍在持续推进，而不是调度器自身停摆：

- 支付恢复
- 退款恢复
- 分账恢复
- 商户提现恢复
- 每日对账
- 押金提醒与过期处理

## 4. 典型告警处理步骤

### 4.1 TASK_ENQUEUE_FAILURE

说明：回调已经收到，但后续异步处理任务没有成功进入队列。

处理顺序：

1. 先根据告警 extra 定位 payment order、refund order、profit sharing order 或 withdrawal record。
2. 确认对应主记录当前状态是否已经被后续恢复任务修复。
3. 若状态仍停滞，检查 Asynq/Redis 是否可用，以及应用实例是否存在持续入队失败。
4. 若恢复调度器会自动覆盖该场景，优先等待一个调度周期后复查。
5. 若超过两个调度周期仍无推进，按主记录手工补发任务或执行人工事务修复。

人工介入时的原则：

- 不直接修改 success/failed 终态字段掩盖问题
- 优先补发任务，让既有事务逻辑落账
- 若必须修数，先记录 payment_order_id 或 refund_order_id 与修复原因

### 4.2 PROFIT_SHARING_FAILED

说明：微信侧或本地分账处理进入失败或关闭状态。

处理顺序：

1. 检查 extra 中的 `profit_sharing_order_id`、`payment_order_id`、`merchant_id`。
2. 确认失败原因是微信返回明确拒绝，还是本地重试耗尽。
3. 若属于可重试问题，确认 `ProfitSharingRecoveryScheduler` 是否已在下一周期重新拉起。
4. 若微信侧已明确关闭且不可重试，需要按订单金额决定是否人工补偿或补差。

### 4.3 REFUND_FAILED

说明：退款发起成功后，结果处理阶段发现失败、关闭或异常状态。

处理顺序：

1. 检查 `refund_order_id`、`payment_order_id`、`out_refund_no`。
2. 判断是否属于骑手押金提现退款、订单取消退款或异常退款。
3. 确认冻结金额是否已经自动解冻，避免重复退款或余额错扣。
4. 若退款单长期停留在 processing，优先查微信侧结果与退款回调是否到达。

### 4.4 BILL_MISMATCH

说明：微信账单与本地订单金额、数量或存在性不一致。

处理顺序：

1. 先看差异 bill type，是 trade、ecommerce_trade、refund 还是 ecommerce_refund。
2. 对 missing record，先判断是微信有本地无，还是本地有微信无。
3. 对 amount mismatch，检查支付单金额、退款单金额以及是否存在拆单、部分退款、分账回退。
4. 若属于回调延迟造成的暂时不一致，待相关 recovery scheduler 跑完后重查。
5. 若次日仍存在差异，进入人工对账工单，禁止仅通过删报表方式“消告警”。

## 5. 人工补救优先级

出现多个问题时，按以下顺序处理：

1. 用户资金未到账或错误退款
2. 分账失败导致商户或骑手应收未结算
3. 提现结果长时间未知
4. 对账差异但资金最终一致
5. 押金到期提醒类告警

原因很简单：先保证真实资金流，再处理报表与提醒一致性。

## 6. 发布后观察点

本轮重构上线后的重点观察窗口为首个完整自然日与首个完整自然周。

首日重点看：

- `TASK_ENQUEUE_FAILURE` 是否持续出现
- payment recovery 与 refund recovery 是否实际有补偿命中
- 分账是否出现异常积压
- 提现回调失败、重试或 pending 积压是否异常升高
- 骑手押金提醒任务是否产生日志与通知

首周重点看：

- 每日对账报告是否稳定生成
- `ecommerce_refund` 是否出现持续差异
- 商户提现失败告警是否能携带完整排障上下文

## 7. 测试环境闭环联调清单

本节用于阶段 J6 的现场执行，不代表当前已在仓库内自动完成。

推荐按以下顺序在测试环境完成一次完整闭环：

1. 校验测试环境已加载最新 `WECHAT_PAY_*`、`WECHAT_ECOMMERCE_*`、`REDIS_*` 配置，尤其确认 `WECHAT_ECOMMERCE_WITHDRAW_NOTIFY_URL` 指向当前环境可访问域名。
2. 执行 `make migrateup`，确认新增 migration 已全部落库且 `migratestatus` 无 dirty 标记。
3. 部署后端新版本，确认 `/health`、`/ready` 正常，scheduler 与 task processor 初始化日志正常。
4. 验证订单/预订支付仍走收付通合单链路。
5. 验证骑手押金充值仍走微信直连链路。
6. 验证骑手押金提现触发退款式提现，且生成 refund order、freeze 流水与 credit 变更。
7. 人工触发或等待退款结果回调，确认 payment order、refund order、rider balance、rider deposit credit 最终状态闭环一致。
8. 人工执行一次到期提醒任务，确认通知与平台告警都能产出。
9. 人工检查 payment/refund/profit-sharing/merchant-withdraw recovery scheduler 至少各有一次正常扫描日志。
10. 对账入口执行一次最近账单拉取，确认 reconciliation report 可生成。

联调通过标准：

- 收付通主链路与骑手押金直连链路分类正确。
- 退款与分账异常不会遗留需要手工修数的半状态。
- 关键告警在失败场景下能带出足够排障上下文。

## 8. 生产发布顺序

推荐发布顺序固定为：

1. 配置变更预检查。
2. 数据库 migration。
3. 后端服务发布。
4. Web 发布。
5. 小程序发布。
6. 上线后首轮验收。

不得颠倒为“先发前端/小程序，再发后端和 migration”。

### 8.1 配置变更预检查

必须确认：

1. `WECHAT_PAY_*` 直连商户配置完整。
2. `WECHAT_ECOMMERCE_*` 服务商配置完整。
3. `REDIS_ADDRESS`、`REDIS_PASSWORD` 已配置，生产环境不会退化为 Noop task distributor。
4. 回调 URL 指向即将发布的新环境域名。
5. `WECHAT_ECOMMERCE_WITHDRAW_NOTIFY_URL` 已配置，且与 `/v1/webhooks/wechat-ecommerce/withdraw-notify` 路由一致。
6. 生产 `ALLOWED_ORIGINS` 不为空且不包含 `*`。

### 8.2 数据库 migration

按现有 Makefile 执行：

```bash
make migrateup
```

发布前至少确认以下 migration 已在目标库可见：

- `000162_create_rider_deposit_credits`
- `000163_expand_refund_order_types`
- `000164_relax_rider_deposit_payment_order_uniqueness`

### 8.3 后端发布

发布后立刻确认：

1. `/health` 与 `/ready` 正常。
2. `ecommerce client created for profit sharing` 或对应支付初始化日志符合预期。
3. `payment-recovery`、`refund-recovery`、`profit-sharing-recovery`、`merchant-withdraw-recovery`、`data-cleanup`、`bill-reconciliation` scheduler 已注册并启动。
4. 没有出现 Redis 未配置、task distributor 退化、支付客户端初始化失败等 fatal/warn。

### 8.4 Web 与小程序发布

发布后至少抽检：

1. 订单支付创建正常。
2. 预订支付创建正常。
3. 骑手押金充值创建正常。
4. 骑手提现入口返回的状态与新退款式提现语义一致。

### 8.5 上线后首轮验收

上线后首个观察窗口内必须确认：

1. 没有持续性的 `TASK_ENQUEUE_FAILURE`。
2. 退款 recovery 与 payment recovery 有正常扫描日志。
3. 骑手押金提醒任务可以正常扫描。
4. 对账任务次日能正常生成报表。

## 9. 回滚策略

### 9.1 回滚原则

本轮改造属于支付主链路净化，不建议回滚到“旧转账路径 + 新 schema 并存”的混乱状态。

回滚优先级固定为：

1. 配置回滚。
2. 后端版本回滚。
3. 前端/小程序版本回滚。
4. 数据库 schema 回滚仅在刚上线且未产生新业务数据时评估。

### 9.2 可优先回滚的层

优先回滚：

1. Web/小程序版本。
2. 后端服务版本。
3. 配置开关与回调地址。

尽量不回滚：

1. 已执行的 rider deposit credit 相关 migration。
2. 已执行的 refund type 扩容与 rider deposit 流水索引调整。

原因：这些 schema 已与当前资金语义对齐，盲目 down migration 反而更容易制造资金状态不一致。

### 9.3 数据库回滚策略

除非满足以下两个条件，否则不执行 `migratedown`：

1. 刚上线且尚未产生新的押金退款、收付通退款或分账业务数据。
2. 已确认通过回滚代码/配置无法恢复服务。

数据库问题的优先处理方式：

1. 热修后端代码兼容当前 schema。
2. 增补 forward migration，而不是直接 down。
3. 借助 recovery scheduler 让业务状态重新收敛。

### 9.4 回滚触发条件

满足任一条件即可进入回滚评估：

1. 收付通支付创建或支付回调大面积失败。
2. 骑手押金提现连续出现冻结不解或退款状态无法收敛。
3. 分账失败大面积堆积且 recovery scheduler 无法推进。
4. 对账出现系统性金额差异，而非单笔偶发。
5. Redis/Asynq 异常导致关键支付后处理任务长时间无法恢复。

### 9.5 回滚后必须复查的点

1. 已产生的 refund order 是否仍可继续靠 refund callback 或 recovery scheduler 收敛。
2. 骑手押金 credit 是否存在 `refundable_amount` / `refunded_amount` 不一致。
3. payment order 是否存在 `paid`、`refunded`、`processed_at` 终态漂移。
4. 平台告警是否已恢复到可控水平。

## 10. 手册维护原则

- 新增支付类恢复调度器时，同步补充到本手册
- 新增平台告警类型时，必须写清楚人工处理顺序
- 若某个场景已经不再依赖人工处理，应删除对应手工说明，避免手册落后于代码