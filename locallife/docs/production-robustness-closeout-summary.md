# LocalLife 生产级鲁棒性整改总收尾摘要

## 说明

- 本文用于汇总 [production-robustness-review-report.md](./production-robustness-review-report.md) 中 15 个问题在 batch A-E 的整改结果。
- 本文同时回答两个收尾问题：
  - 当前风险报告中是否还存在需要单独再开的非 P0 批次。
  - batch A-E 已修项、验证面与剩余风险分别是什么。
- 本文不替代各批次执行跟踪；批次级过程记录仍以 batch A-E tracker 为准。

## 总体结论

- 当前风险报告共记录 15 个问题。
- 其中 P0 问题为 1、2、3、5、6、7、9、10、11、13、14，已分别在 batch A-D 完成修复并记录。
- 非 P0 问题为 4、8、12、15，仅有这四项，已在 batch E 全部完成。
- 以当前 [production-robustness-review-report.md](./production-robustness-review-report.md) 为边界，当前没有剩余需要新开的独立非 P0 批次。
- 当前进入收尾阶段的重点不再是继续拆批，而是历史数据排查、外部依赖路径观察和上线后监控确认。

## 批次收口总览

| 批次 | 覆盖问题 | 当前状态 | 核心收口结果 |
| --- | --- | --- | --- |
| A | 1、5、9 | 已完成 | 封住外卖地址越权建单；自动送达解冻金额改为订单口径；退款额度占用纳入 pending/processing/success |
| B | 2、3、6、7 | 已完成 | 预订活跃订单唯一性下沉到事务内；支付并发窗口改为事务内拒绝冲突、事务外复用或显式 supersede；抢单与配送推进统一为原子状态流转 |
| C | 10、11 | 已完成 | 预订改菜退款按真实支付交易集合分摊；改菜退款接入统一异步退款闭环与 pending 恢复 |
| D | 13、14 | 已完成 | 预订与加菜分账恢复入口补齐；processing 分账改为查询微信结果后收敛，不再空转 |
| E | 15、12、4、8 | 已完成 | 追偿支付具备入口侧过期自收敛；提现 zombie pending 可识别远端不存在并失败收口；过期合单主记录接入 scheduler 兜底清理；外卖订单配送地址改为建单快照 |

## 已修项摘要

### Batch A

- 问题 1：在外卖建单路径补上地址归属强校验，当前用户不能再用他人的 `address_id` 建单。
- 问题 5：自动确认送达改为在 logic 层统一按 `OrderFreezeAmount(order)` 计算解冻金额，不再依赖固定值 `5000`。
- 问题 9：退款额度校验改为统计 `pending`、`processing`、`success` 三类占用状态，阻断第一笔退款挂起时继续放出超额退款。

### Batch B

- 问题 2：预订押金模式的“单预订唯一活跃订单”约束下沉到事务内，借助预订行锁和事务内复查关闭并发绕过窗口。
- 问题 3：支付创建并发改为事务内返回冲突哨兵，事务外按最新 pending 支付单复用、等待 prepay 或显式 supersede，不再允许本地旧单已关而微信侧仍创建出旧可支付单。
- 问题 6：抢单成功这一跳改为在 `GrabOrderTx` 内一并提交配送分配、订单状态推进和状态日志，关闭两阶段半提交窗口。
- 问题 7：开始取餐、确认取餐、开始配送三条状态事务改为“任一写入 no rows 都整体回滚”，并补上数据库侧源状态保护。

### Batch C

- 问题 10：预订改菜退款不再错误压回单一原始支付单，而是按真实 `reservation` 与 `reservation_addon` 支付交易集合逆序分摊。
- 问题 11：改菜退款改为先建退款单，再走统一异步退款任务；退款成功后再回写 `prepaid_amount`，并补上 pending 退款恢复调度。

### Batch D

- 问题 13：预订与 `reservation_addon` 分账恢复路径改为从 `payment_order` 自动构造 payload，不再依赖 `order_id` 假设。
- 问题 14：已有 `processing` 分账单被恢复时，不再直接 skip，而是查询微信分账结果后把本地状态推进到 `finished`、`failed` 或继续保持 `processing`。

### Batch E

- 问题 15：索赔追偿支付入口新增“过期 pending 旧单不再复用”的自收敛逻辑，命中过期旧单时先本地关闭再创建新支付尝试。
- 问题 12：提现结果轮询现在能区分“微信处理中”和“远端根本不存在该申请单”，后者在重试耗尽后会从 `pending` 收敛到 `failed`。
- 问题 4：过期 `combined_payment_orders` 主记录已接入 `scheduler/data_cleanup.go` 的批量兜底关闭路径，不再只清子支付单。
- 问题 8：新增订单配送快照字段，建单时固化联系人、手机号、地址和经纬度；后续订单详情展示与出餐后创建配送单都优先使用订单快照。
- batch E 后续收尾兼容：由于 `GetOrderWithDetailsRow` 的配送字段类型从可空文本收敛为字符串，已同步修正 API 层响应映射与测试夹具，避免批次 E 改动在上层留下编译断点。

## 验证面汇总

### Batch A 验证面

- API：`TestCreateOrderAPI` 覆盖地址归属拒绝路径。
- logic：自动送达相关单测验证非 `5000` 金额时的解冻金额与订单口径一致。
- db/sqlc：退款统计与事务测试覆盖 pending、processing、success 混合退款占用口径。

### Batch B 验证面

- db/sqlc：`TestCreateOrderTx`、`TestCreateCombinedPaymentTx`、`TestGrabOrderTx` 覆盖预订唯一性、支付并发冲突和抢单事务原子性。
- logic：`TestGrabDeliveryOrder` 与配送状态相关测试覆盖状态冲突映射和事务后行为。
- API：`TestCreateOrderAPI`、`TestCreatePaymentOrderAPI`、`TestGrabOrderAPI`、`TestConfirmPickupAPI`、`TestStartPickupAPI`、`TestStartDeliveryAPI` 覆盖主入口不回归。

### Batch C 验证面

- logic：`TestBuildReservationRefundAllocations_SplitsAcrossReservationPayments` 覆盖多支付单退款分摊。
- worker：预订退款发起、退款成功回写余额和 pending 恢复相关 worker 测试已补齐。
- API：`TestCreateReservationAPI` 用于确认预订主链未被本轮改动打断。

### Batch D 验证面

- worker：恢复调度、`processing` 分账查询收敛、失败后重试 payload 复用均有定向测试。
- 静态与编译检查：批次 D 涉及的 worker 文件已完成 `gofmt` 与文件级错误检查。

### Batch E 验证面

- logic：追偿支付过期 pending 自收敛路径已有定向测试。
- worker：提现 request-not-found 收敛为 failed 的路径已有定向测试。
- scheduler：过期合单主记录兜底关闭路径已补充定向测试。
- db/sqlc：`TestGetOrderWithDetails_PrefersDeliverySnapshot` 与 `TestMarkTakeoutOrderReadyTx_UsesOrderDeliverySnapshotAfterAddressUpdate` 覆盖订单快照读取与配送创建。
- logic 与 worker 编译校验：`go test ./logic -run '^$' -count=1`、`go test ./worker -run '^$' -count=1` 已用于确认 issue 8 变更没有打断主链。
- API 兼容收尾：`go test ./api -run '^$' -count=1` 与 `go test ./api -run 'TestCreateOrderAPI' -count=1` 已通过，用于确认配送字段类型收敛后 API 层映射与测试夹具无回归。

## 剩余风险与收尾建议

### 1. 历史数据与存量坏状态未在本轮代码修复里自动清理

- 问题 9 修复后，历史长时间挂起的 `pending` / `processing` 退款单会真实占住退款额度；上线前仍建议排查是否存在需要人工确认或清理的历史退款单。
- 问题 8 只保证新订单从建单开始固化配送快照；历史订单若没有快照，当前仍会按兼容逻辑回退到地址簿现值。

### 2. 若干外部依赖链已补结构性闭环，但仍缺真实外部系统联调证据

- 问题 12 的“微信提现申请单远端不存在”收敛依赖微信错误码语义，建议上线后重点观察该类失败告警是否出现误报或漏报。
- 问题 14 的 `processing` 分账查询收敛依赖微信查询结果枚举，建议观察真实微信返回体是否存在当前测试未覆盖的状态组合。
- 问题 10、11 的预订退款异步闭环虽然已补 worker 与恢复路径，但真实回调时序、重复投递和长尾重试仍建议上线后做专项观察。

### 3. 个别问题当前采用的是“最小生产修复”，不是完整治理工程

- 问题 15 当前只在追偿支付入口自身识别并关闭过期 pending 旧单，尚未额外建设独立的追偿支付超时清理任务；如果用户不再重新进入支付入口，旧单主要仍依赖现有路径或后续专项治理。
- 问题 5 已修围栏自动确认送达路径，但其他自动履约入口是否还存在硬编码冻结/解冻金额，需要在后续巡检中继续确认。

### 4. 当前收尾结论以代码级风险报告为边界，不等于所有高风险链路都做过完整集成压测

- 当前 batch A-E 已完成代码修复和针对性自动化验证，但未在本文范围内补做全量并发压测、全链路微信沙箱联调或历史数据回放。
- 因此当前最合理的收尾方式是：代码整改结束，不再新开独立非 P0 批次；后续转入上线观察、运营排查和专项联调。

## 最终结论

- 以当前风险报告为准，15 个已登记问题在 batch A-E 已全部完成整改收口。
- 当前不存在新的独立非 P0 批次需求。
- 后续工作重点应从“继续拆批修代码”转为“观察存量数据、验证外部依赖长尾状态、根据线上信号决定是否开专项治理项”。