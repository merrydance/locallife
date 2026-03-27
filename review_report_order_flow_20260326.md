# 订单业务线 Review Report

日期：2026-03-26

范围：
- 堂食开台、换桌、关台
- 预订创建、确认、到店、加菜与支付联动
- 外卖订单与配送、自动完单的交叉链路

本次 review 重点检查了 handler → logic → transaction → SQL 的完整调用链，优先关注权限、状态流转、金额结算、桌台占用与预订联动。

## Findings

### 1. High: 预订订单允许挂在 pending 预订上创建，并直接抵扣未支付定金

证据：
- [locallife/logic/order_session.go](locallife/logic/order_session.go#L54) 对 reservation 订单放行了 pending 状态
- [locallife/logic/order_service.go](locallife/logic/order_service.go#L196) 只要是 deposit 模式就直接把 deposit_amount 当作抵扣额
- [locallife/logic/order_payment.go](locallife/logic/order_payment.go#L35) 会把该抵扣额直接从应付金额里减掉

影响：
- 用户可以在预订定金尚未支付时创建 reservation 订单，并提前享受定金抵扣。
- 这会导致订单实付金额被低估，形成资金对账缺口。
- 对 deposit 模式而言，这相当于把“未到账定金”当成“已到账余额”使用。

建议：
- reservation 订单在 deposit 模式下至少要求预订已进入 paid、confirmed 或 checked_in。
- 或者仅在确认存在已支付的 reservation payment 后，才允许写入 depositDeduction。

### 2. High: 商户确认预订时会立即把桌台写成 reserved，没有校验时段或当前占用

证据：
- [locallife/logic/reservation.go](locallife/logic/reservation.go#L380) 到 [locallife/logic/reservation.go](locallife/logic/reservation.go#L404) 的确认逻辑只校验归属与状态
- [locallife/db/sqlc/tx_reservation.go](locallife/db/sqlc/tx_reservation.go#L197) 先把预订改成 confirmed
- [locallife/db/sqlc/tx_reservation.go](locallife/db/sqlc/tx_reservation.go#L205) 再无条件把桌台写成 reserved 并设置 current_reservation_id

影响：
- 未来时段的预订一旦确认，就会立刻改变桌台当前状态，而不是在预约冲突窗口内生效。
- 如果桌台当下仍在营业中，这次确认会直接覆盖桌台状态，造成运营态和实际占用态不一致。
- 后续转台逻辑对 reserved 和 current_reservation_id 很敏感，容易把未来预订错误地当成当前不可用，从而放大业务阻塞。

建议：
- confirm 只更新预订状态，不应立即占用桌台当前状态。
- 桌台 reserved/current_reservation_id 应在接近预约时段或到店 check-in 时再写入。
- 若确实需要提前占位，也必须同时校验当前是否已有 open dining session，以及预订时间窗口是否已进入占用期。

### 3. Medium: 预订支付成功后只更新 status，未写 paid_at

证据：
- [locallife/db/sqlc/tx_payment_success.go](locallife/db/sqlc/tx_payment_success.go#L150) 在 reservation 支付成功路径里调用的是通用的 UpdateReservationStatus
- [locallife/db/query/table_reservation.sql](locallife/db/query/table_reservation.sql#L128) 已经存在专门的 UpdateReservationToPaid 语句，会同时写 paid_at

影响：
- 预订虽然进入 paid，但 paid_at 可能一直为空。
- 预订详情接口、统计报表、支付时序审计和后续超时/提醒逻辑会失去准确支付时间。
- 这类问题通常不会立刻报错，但会在运营核账和用户投诉回溯时暴露。

建议：
- reservation 主支付成功后统一走 UpdateReservationToPaid，而不是通用状态更新。
- 补一个回归测试，明确断言 paid_at 已写入。

### 4. Medium: 账单组 total_amount 和 paid_amount 当前没有被维护，对外返回值会长期漂移

证据：
- [locallife/db/sqlc/tx_dining_session.go](locallife/db/sqlc/tx_dining_session.go#L74) 到 [locallife/db/sqlc/tx_dining_session.go](locallife/db/sqlc/tx_dining_session.go#L75) 创建账单组时把 total_amount 和 paid_amount 初始化为 0
- [locallife/db/sqlc/tx_create_order.go](locallife/db/sqlc/tx_create_order.go#L111) 下单时只新增 billing_group_orders 关联记录，没有更新 billing_groups 聚合字段
- [locallife/db/query/billing_group.sql](locallife/db/query/billing_group.sql#L1) 到 [locallife/db/query/billing_group.sql](locallife/db/query/billing_group.sql#L69) 只有创建、查询、状态更新，没有金额更新 SQL
- [locallife/api/billing_group.go](locallife/api/billing_group.go#L260) 到 [locallife/api/billing_group.go](locallife/api/billing_group.go#L268) 列表接口直接返回 billing_groups 表中的 total_amount 和 paid_amount

影响：
- 堂食拼桌/分账场景下，接口返回的账单组金额很可能长期停留在 0 或旧值。
- 前端如果直接依赖该字段做结账展示、待付金额展示或已付金额判断，会出现明显误导。
- 账单组与 billing_group_orders 之间形成数据双写但只写一侧，后续越跑越难修复。

建议：
- 明确选择一种单一事实来源。
- 若 billing_groups 继续保留聚合字段，则在建单、支付成功、取消、替换订单等路径里原子维护。
- 若改为运行时聚合，则应停止对外暴露表内 total_amount 和 paid_amount 原值。

### 5. Medium: 堂食权限模型前后不一致，商户员工可转台但不能关台

证据：
- [locallife/logic/dining_session.go](locallife/logic/dining_session.go#L360) 到 [locallife/logic/dining_session.go](locallife/logic/dining_session.go#L374) 的转台逻辑允许 owner 或通过 CheckUserHasMerchantAccess 的商户成员操作
- [locallife/logic/dining_session.go](locallife/logic/dining_session.go#L469) 开始的关台逻辑只通过 GetMerchantByOwner 查 owner，不接受商户员工权限

影响：
- 同一条堂食业务链里，商户员工能参与开台/转台，但无法执行关台结账，权限模型前后断裂。
- 这会把关台能力意外收缩到 owner，营业高峰时容易形成操作瓶颈。
- 如果前端默认把“有商户权限”视为可管理堂食，会在运行时产生 403 回退。

建议：
- 统一堂食链路的商户权限判断方式。
- 若产品要求只有 owner 能关台，应在其他堂食操作上同步收紧，并在接口契约里明确写出。

## 其他观察

- [locallife/api/order.go](locallife/api/order.go#L475) 到 [locallife/api/order.go](locallife/api/order.go#L490) 还保留了 DEBUG 日志文案，属于生产路径调试残留，建议顺手清理。
- 本次没有把外卖自动完单列为正式 finding，因为当前证据还不足以证明一定存在状态撕裂；但建议后续补一个覆盖 order 与 delivery 双状态一致性的集成测试。

## 建议优先级

1. 先修 Finding 1 和 Finding 2。这两项会直接影响金额正确性与桌台运营态。
2. 再修 Finding 3。它会影响支付时间审计与后续统计。
3. Finding 4 和 Finding 5 可作为堂食账单与权限治理的第二批修复项。

## 建议补测

- deposit 模式预订在 pending 状态下创建 reservation 订单，应失败。
- 未来时段预订 confirm 后，不应立刻把当前桌台状态改成 reserved。
- reservation 主支付成功后，paid_at 应被写入。
- billing group 在下单、支付、取消、替换订单后，金额字段应与订单集合一致。
- 商户员工在具备 merchant access 时，对开台、转台、关台的权限应表现一致。