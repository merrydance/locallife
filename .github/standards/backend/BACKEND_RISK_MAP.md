# Backend Risk Map

> 作用：把 LocalLife 后端已经暴露出的高风险生产链路和常见失效模式，沉淀为实现与审查前的固定热路径，而不是只靠个人经验记忆。

## 1. 使用方式

当变更涉及以下任一情况时，先读本文件，再进入对应 domain docs、实现代码与测试：

- 资金、支付、退款、分账、提现
- 订单、配送、履约、预订、库存或其他状态机
- callback、worker、scheduler、recovery、补偿或超时清理
- 需要判断不变量应落在 API、logic、transaction 还是数据库约束层

配合阅读：

- `.github/standards/backend/AGENT.md`
- `.github/standards/backend/SYSTEM_PROMPT.md`
- `.github/standards/engineering/HIGH_RISK_CHANGE_CHECKLISTS.md`
- `.github/standards/backend/RUNTIME_ARCHITECTURE.md`
- `.github/standards/domains/wechat-payment/README.md`

## 2. Highest-Risk Areas

### 2.1 Order creation invariants

- 外卖地址归属必须由服务端校验，不能只依赖客户端传入地址 ID。
- 预订类“单个用户只允许一个活跃订单”之类的排他性规则，若只在事务外判断，容易并发失效。
- 创建订单会联动优惠券、会员余额、账单组、预订库存、超时任务等下游状态，不是单表写入问题。

### 2.2 Payment creation and callbacks

- 待支付单复用、关闭和重新创建是并发敏感路径。
- 本地支付状态与微信可支付状态如果原子边界不对，容易产生“本地已失效但第三方仍可支付”的漂移。
- 支付成功不是终点；callback、worker、recovery 和后续订单资金链路都必须一起看。

### 2.3 Refund, profit sharing, and abnormal refund

- 退款金额汇总、处理中/待处理退款统计、异常退款补偿都可能被并发或重复触发打穿。
- 分账回退和异常退款不是边角逻辑，而是资金状态闭环的一部分。
- 退款修复通常不止改 `logic/`，还需要检查 `db/sqlc/`、worker、callback handler 与恢复任务是否同步。

### 2.4 Delivery and fulfillment state machines

- 配送状态与订单状态必须协同推进，不能出现半成功状态。
- 抢单、取货、配送中、送达、完成等状态转换常常伴随押金、库存、通知或结算语义。
- 若事务失败时部分副作用已生效，就会形成难恢复的履约漂移。

### 2.5 Recovery and cleanup paths

- 超时关闭、补偿、重试、死信或定时清理任务是预期生产行为的一部分，不是“兜底可选项”。
- 只验证请求 happy path 而不检查 stale pending records、遗漏清理、重复恢复，不能算完成。
- 很多路径的最终收敛依赖 scheduler，而不是某个单次请求。

## 3. Invariants To Re-Check Before Merge

- 认证用户只能访问自己拥有或被角色/员工授权的对象。
- 排他性、唯一性、幂等和关键状态前置条件落在真实写边界内，而不是只在 handler 做前置判断。
- 支付、退款、分账、配送和订单记录不会无补偿路径地进入相互矛盾状态。
- callback、worker、scheduler 的重复执行语义是显式设计的，而不是“默认应该不会重复”。
- SQL、生成代码、mock、Swagger 与上游调用方保持同步，没有停在半路。

## 4. Review Bias

实现和审查这些区域时，默认偏向：

- transaction-level enforcement over API-only checks
- explicit conflict or request errors over silent best-effort behavior
- idempotent callback and worker processing
- targeted regression tests at the exact failure boundary
- 明确写出未验证 callback / retry / recovery 分支，而不是用“需要更多测试”笼统带过

## 5. Closeout Expectation

如果改动触及资金、状态机或 recovery 路径，交付时至少说明：

1. 真正改动了哪些层：`api`、`logic`、`db/sqlc`、`worker`、`scheduler`、`webhook`
2. 哪些高风险分支已验证
3. 哪些分支未验证，残余风险落在哪个具体路径
4. 是否需要 `make sqlc`、`make mock`、`make swagger` 或 `make check-generated`

如果这些问题答不清，不应把改动描述为生产级闭环完成。

