# LocalLife 批次 B 修复执行跟踪

## 说明

- 本文用于跟踪批次 B 的实际推进，后续每一次进展都继续追加到本文。
- 批次 B 的来源文档：
  - [production-robustness-review-report.md](./production-robustness-review-report.md)
  - [production-robustness-p0-remediation-review.md](./production-robustness-p0-remediation-review.md)
- 本批次覆盖问题 2、3、6、7。

## 状态总览

| 任务 | 对应问题 | 当前状态 | 下一步 |
| --- | --- | --- | --- |
| B-1 | 问题 2：预订活跃订单唯一性只在事务外校验 | 已完成 | 进入批次 B 下一项问题 3 |
| B-2 | 问题 3：支付创建并发窗口 | 已完成 | 进入批次 B 下一项问题 6 |
| B-3 | 问题 6：抢单事务与订单状态分裂 | 已完成 | 进入批次 B 下一项问题 7 |
| B-4 | 问题 7：配送状态吞错提交与源状态缺失 | 已完成 | 批次 B 完成，转入后续批次 |

## 固定实施顺序

1. B-1
2. B-2
3. B-3
4. B-4

说明：问题 2 是后续支付与履约链围绕“单一预订订单”推进的前置约束，因此保持最先处理。

## 进展记录

### 2026-04-02 第 1 次推进

完成内容：

- 已确认问题 2 当前的真实缺陷不在“是否有前置检查”，而在“唯一性只在事务外检查，无法抵御并发”。
- 已确认仓库里已有 `GetTableReservationForUpdate`，因此问题 2 可以采用“事务内锁预订行，再复查活跃订单”的修复路线。
- 已明确本轮不考虑历史数据和向后兼容，因此无需为历史双活订单设计上线兼容策略。

当前决策：

- 问题 2 先不引入数据库级唯一索引。
- 优先采用事务内锁定 `table_reservations` 行并复查 `GetLatestOrderByReservation` 的方案，把修复范围限定在押金模式预订订单。
- 应用层前置 `EnsureReservationSingleActiveOrder` 继续保留，用于提前返回友好冲突；真正的一致性保障下沉到事务内。

原因：

- 这样可以保持修复精准命中 `payment_mode = deposit` 场景，不把约束误扩到 `payment_mode = full`。
- 现有仓库已经有预订行锁能力，可复用，不需要为了问题 2 额外上新的全局 schema 约束。

### 2026-04-02 第 2 次推进

完成内容：

- 已在 `CreateOrderTx` 增加 `EnforceSingleActiveReservationOrder` 开关，用于把押金模式预订订单的唯一性保护下沉到事务内。
- 当该开关打开且存在 `reservation_id` 时，事务会先锁定对应预订行，再复查当前最新未替换订单是否仍处于活跃态。
- 若事务内复查命中活跃订单，当前实现会返回明确的冲突哨兵错误；logic 层已把该错误映射回 409。

当前判断：

- 问题 2 的正确性保障已经从“事务外检查”移动到“事务内锁 + 复查”。
- 应用层前置检查保留不变，只承担提前失败和更早返回的作用，不再承担最终一致性责任。

### 2026-04-02 第 3 次推进

完成内容：

- 已在 `db/sqlc/tx_create_order.go` 引入事务内冲突哨兵错误 `ErrReservationActiveOrderConflict`。
- 已在 `logic/order_service.go` 将该冲突映射为 409，避免事务内命中唯一性冲突时向上冒成 500。
- 已补充 `db/sqlc/tx_create_order_test.go`，验证同一押金模式预订在首笔活跃订单创建后，第二笔建单会被事务内拦截。

验证结果：

- 已运行问题 2 对应的事务测试与现有 `EnsureReservationSingleActiveOrder` 单测，结果通过。
- 已运行 `TestCreateOrderAPI`，确认当前建单主路径未出现回归。

当前结论：

- 问题 2 可以视为已完成。
- 在“无历史数据、无需向后兼容”的前提下，当前方案已经把风险从正确性层面关闭。

### 2026-04-02 第 4 次推进

完成内容：

- 已重新核对问题 3 的真实窗口：第一个请求先在本地落出 pending 支付单，第二个并发请求在事务内把它本地关掉并重建，随后第一个请求仍可能继续去微信创建可支付子单。
- 已确认当前风险根源在 `CreateCombinedPaymentTx`：事务内一旦读到已有 pending `payment_orders`，会直接关掉旧单并创建新单，但微信 `CreateCombineOrder` 发生在事务外。
- 已确认应用层首次 `GetLatestPaymentOrderByOrder` 幂等检查不能覆盖这个窗口，因为第二个请求可能在首次检查时还看不到第一个请求刚提交的 pending 支付单。

当前决策：

- 不把微信下单调用放进数据库事务。
- 在 `CreateCombinedPaymentTx` 内把“命中已有 pending 支付单”改成明确的并发哨兵错误，禁止事务内直接本地关单重建。
- 在 `PaymentOrderService` 外层收到该哨兵后，走“重新读取最新 pending 支付单 -> 同金额复用/短暂等待 prepay_id -> 不同金额先 supersede 再重试”的补偿路径。
- 同时收紧 `UpdatePaymentOrderPrepayId`，只允许对仍处于 pending 且尚未写入 prepay_id 的支付单落 prepay，阻断旧请求把已失效支付单重新写活的路径。

原因：

- 这样可以直接关闭“本地已失效但微信仍可支付”的并发窗口，同时不把外部网络调用放进事务持锁区间。

### 2026-04-02 第 5 次推进

完成内容：

- 已在 `db/sqlc/tx_create_combined_payment.go` 引入并发哨兵 `ErrOrderPendingPaymentConflict`，当事务内发现同一订单已有 pending 支付单时不再直接本地关单重建。
- 已在 `logic/payment_order_service.go` 为订单支付增加并发补偿路径：命中该哨兵后重新读取最新 pending 支付单；同金额时优先复用并短暂等待 `prepay_id`，不同金额时先按 supersede 语义关闭旧单再重试。
- 已把已有 pending 支付单的重新签名逻辑抽成统一 helper，避免正常幂等路径与并发补偿路径分别维护。
- 已把 supersede 关闭逻辑拆成“有 `prepay_id` 走正常微信关单语义、无 `prepay_id` 仅本地关闭 payment/combined 主记录”，避免尚未真正下发到微信的旧单在本地被关闭后又卡在远端关单调用。
- 已收紧 `UpdatePaymentOrderPrepayId`，现在只有 `status = pending` 且 `prepay_id IS NULL` 的支付单才能落 prepay，阻断旧请求在支付单被后续请求 supersede 后重新写活。
- 已补充 `db/sqlc/tx_create_combined_payment_test.go`，验证事务内命中已有 pending 支付单时返回并发冲突且不会把旧单状态改成 closed。
- 已补充 `api/payment_order_test.go`，验证首轮幂等读取 miss、事务内命中并发冲突后，API 会回退到复用最新 pending 支付单并重新生成 `pay_params`。

验证结果：

- 已执行 `make sqlc`，生成代码与 mock 已同步更新。
- 已执行 `go test ./db/sqlc -run 'TestCreateCombinedPaymentTx'`，通过。
- 已执行 `go test ./api -run 'TestCreatePaymentOrderAPI'`，通过。

当前结论：

- 问题 3 可以视为已完成。
- 目前并发支付路径已经从“事务内本地关旧单、事务外再去微信建新单”调整为“事务内拒绝 supersede，事务外按最新 pending 支付单做复用或显式 supersede”，原先的陈旧预支付单窗口已关闭。

### 2026-04-02 第 6 次推进

完成内容：

- 已重新核对问题 6 的真实断点：`GrabOrderTx` 只提交配送分配、移池、押金冻结和押金流水，`UpdateOrderToCourierAccepted` 与 `CreateOrderStatusLog` 仍在事务外补写。
- 已确认这不是单纯“日志晚写”问题，而是抢单成功这一跳在代码上被拆成了“事务内副作用已提交”和“事务外主状态再补一刀”的两阶段提交。
- 已确认最小修复面可以收敛在 `GrabOrderTx` 本身，无需先重构整条抢单入口或改动 API 合约。

当前决策：

- 把订单状态推进 `courier_accepted` 与对应状态日志一并并入 `GrabOrderTx`。
- 抢单逻辑层在事务成功后不再额外调用 `UpdateOrderToCourierAccepted` 或 `CreateOrderStatusLog`，直接消费事务结果。
- 状态日志继续保持 `operator_type = rider` 语义，因此事务参数会补充 `RiderUserID`，避免日志从“用户维度操作人”退化成“骑手表主键”。

原因：

- 这样可以在不扩大业务面和不引入额外补偿器的前提下，直接关闭“配送与押金副作用已提交，但订单主状态还没推进”的半成功窗口。

### 2026-04-02 第 7 次推进

完成内容：

- 已在 `db/sqlc/tx_delivery.go` 扩展 `GrabOrderTx`，把订单行锁定、`courier_accepted` 状态推进和对应 `order_status_log` 写入一并并入同一事务。
- 已为 `GrabOrderTxParams` 补充 `RiderUserID`，确保事务内写出的状态日志继续以用户维度记录 `operator_id`，不改变既有语义。
- 已让 `GrabOrderTxResult` 直接返回事务内更新后的订单与状态日志，`logic/delivery_grab.go` 不再在事务外重复调用 `UpdateOrderToCourierAccepted` 与 `CreateOrderStatusLog`。
- 已同步更新事务测试、logic 测试和 API 测试，确保新的事务边界成为默认实现而不是只改主逻辑不改验证。
- 已修正 `db/sqlc/delivery_test.go` 中配送池辅助订单的状态基座，使其与真实“ready 后才能进入抢单”语义保持一致。

验证结果：

- 已执行 `go test ./db/sqlc -run 'TestGrabOrderTx'`，通过。
- 已执行 `go test ./logic -run 'TestGrabDeliveryOrder'`，通过。
- 已执行 `go test ./api -run 'TestGrabOrderAPI'`，通过。

当前结论：

- 问题 6 可以视为已完成。
- 抢单成功这一跳已经从“事务内提交配送与押金副作用，事务外再补订单状态”收敛为单事务完成，不再存在原报告里的半成功提交窗口。

### 2026-04-02 第 8 次推进

完成内容：

- 已重新核对问题 7 的两个根因：一是 `UpdateDeliveryToPickupTx` / `UpdateDeliveryToPickedTx` / `UpdateDeliveryToDeliveringTx` 把订单同步失败吞掉后直接提交事务，二是 `UpdateDeliveryToPickup` SQL 缺少 `assigned` 源状态保护。
- 已确认这三条事务的正确修复面应保持一致，不能只修开始取餐，否则确认取餐和开始配送仍会保留“配送状态提交了、订单状态没同步”的半成功行为。

当前决策：

- 为三条配送状态事务引入统一的并发状态冲突哨兵，任何“配送推进 no rows”或“订单同步 no rows”都视为状态竞争，事务整体回滚。
- 在 logic 层把该哨兵统一映射成可重试的 409，而不是把并发状态变化向上冒成 500。
- 在 `UpdateDeliveryToPickup` SQL 级别补上 `status = 'assigned'`，把开始取餐的单向状态约束真正下沉到数据库写入条件。

原因：

- 这样可以同时关闭“吞错提交”和“重复点击/并发重试把配送单写回旧阶段”两类问题，不需要额外引入补偿器也能把最危险的坏状态挡在事务边界内。

### 2026-04-02 第 9 次推进

完成内容：

- 已在 `db/query/delivery.sql` 为 `UpdateDeliveryToPickup` 补上 `status = 'assigned'` 源状态保护，把开始取餐从应用层前置检查下沉到数据库写入条件。
- 已在 `db/sqlc/tx_delivery.go` 引入统一并发冲突哨兵 `ErrDeliveryStateTransitionConflict`，三条配送状态事务现在一旦命中“配送推进 no rows”或“订单同步 no rows”，都会整体回滚，不再吞错提交。
- 已在 `logic/delivery_status.go` 把该哨兵统一映射成 409，用户侧会拿到“配送状态已变化，请刷新后重试”，不再把并发状态变化向上冒成 500。
- 已补充 `db/sqlc/delivery_test.go`，分别验证开始取餐的 SQL 单向性，以及订单同步失败时 `UpdateDeliveryToPickupTx` 会回滚而不是提交半成功事务。
- 已补充 `logic/delivery_status_test.go`，验证冲突哨兵会被映射成明确的 409 请求错误。
- 已执行 `make sqlc`，同步更新 delivery 相关生成代码和 mock。

验证结果：

- 已执行 `go test ./db/sqlc -run 'TestUpdateDeliveryToPickup|TestUpdateDeliveryToPickupTx'`，通过。
- 已执行 `go test ./logic -run 'Test(StartPickup|ConfirmPickup|StartDelivery)'`，通过。
- 已执行 `go test ./api -run 'Test(ConfirmPickupAPI|StartPickupAPI|StartDeliveryAPI)'`，通过。

当前结论：

- 问题 7 可以视为已完成。
- 批次 B 四项问题（2、3、6、7）现已全部完成；配送状态推进链不再接受“配送状态提交了但订单状态没同步”的半成功结果，开始取餐也具备数据库侧单向状态保护。

## 本批次完成标准

- 每个问题的实施和验证都继续追加到本文。
- 若设计路线发生变化，必须先在本文记录原因，再改代码。
- 每个问题至少要有一条直接对应原风险的自动化验证。