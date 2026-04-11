# LocalLife 后端生产级鲁棒性审查报告

## 说明

- 本报告只记录代码实现层面的风险、漏洞、行为回归隐患与一致性问题。
- 本报告不包含修复方案，目的是为后续深入评估是否需要修复提供基线。
- 当前已完成全部链路审查。
- 后续每完成一个链路审查，继续在本文件追加。

## 审查进度

| 审查项 | 状态 |
| --- | --- |
| 下单建单链路 | 已完成 |
| 订单支付链路 | 已完成 |
| 订单履约链路 | 已完成 |
| 退款与逆向链路 | 已完成 |
| 充值提现分账链路 | 已完成 |
| 赔付追偿流水链路 | 已完成 |

## 已确认问题

### 1. 外卖建单缺少地址归属校验，存在地址越权引用风险

- 严重级别：高危
- 审查范围：下单建单链路
- 影响面：外卖订单地址选择、配送目的地、订单收货信息

证据：

- [logic/order_service.go](../logic/order_service.go#L154) 在外卖建单流程中直接按 `address_id` 调用 `GetUserAddress` 读取地址。
- [db/query/user_address.sql](../db/query/user_address.sql#L15) 的 `GetUserAddress` 查询只按 `id` 取数，不带 `user_id` 条件。
- 对照已有地址详情接口 [api/user_address.go](../api/user_address.go#L210) 到 [api/user_address.go](../api/user_address.go#L221)，相同查询在别处会显式校验 `address.UserID` 是否等于当前登录用户。

风险判断：

- 当前建单链路没有复用这个归属校验。
- 如果攻击者获得其他用户的地址 ID，则可能把他人地址挂到自己的外卖订单上。
- 这属于对象级访问控制缺失，既是越权问题，也是隐私和履约安全问题。

可能后果：

- 订单被配送到错误地址。
- 用户收货信息发生串单暴露。
- 后续订单详情、售后、赔付链路都可能建立在错误的配送归属上。

触发条件：

- 攻击者或误操作用户持有他人的 `address_id`。
- 外卖建单请求直接携带该 `address_id`，且服务端仅按主键读取地址。
- 下单链路未在读取后补做 `address.user_id == current_user_id` 的归属校验。

影响范围细化：

- 影响对象限定为外卖建单链路，不直接影响到店自取、堂食或无需地址的订单类型。
- 影响边界不止于下单瞬间，配送单创建、订单详情展示、售后取证都会继承这份错误地址归属。
- 若地址中包含联系人、手机号、门牌号，经由后续查询或履约操作还会放大为隐私泄露问题。

修复优先级建议：

- 建议优先级：P0。
- 理由：这是直接的对象级越权与隐私泄露风险，触发门槛低，且一旦发生会污染后续整条履约链路的数据基础。

最小修复面：

- 在外卖建单读取地址后立即补充“地址归属必须属于当前用户”的服务端强校验。
- 同步检查是否还有其他直接按 `address_id` 取地址并参与下单或履约写入的入口，避免只修一处留下旁路。

涉及模块：

- `logic/order_service.go`
- `db/query/user_address.sql`
- `api/user_address.go`

回归验证点：

- 当前用户使用自己的地址下单仍应成功。
- 当前用户使用他人的 `address_id` 下单必须被稳定拒绝。
- 订单详情、配送目的地与售后查询在合法地址场景下不应回归异常。

### 2. 预订押金模式的“单预订唯一活跃订单”约束仅在事务外校验，存在并发绕过风险

- 严重级别：高危
- 审查范围：下单建单链路
- 影响面：预订订单唯一性、支付前状态一致性、后续押金抵扣与替单流程

证据：

- [logic/order_service.go](../logic/order_service.go#L135) 在建单事务开始前调用 `EnsureReservationSingleActiveOrder`。
- [logic/order_post.go](../logic/order_post.go#L11) 的实现是先查最新订单，再基于状态做应用层判断。
- [db/query/order.sql](../db/query/order.sql#L144) 的 `GetLatestOrderByReservation` 只是普通查询，没有加锁。
- 代码库内可见 migration 未体现“每个 reservation 仅允许一个活跃 order”的订单级唯一约束；当前能确认的唯一索引只覆盖 `order_no` 等其他字段。

风险判断：

- 两个并发请求可以同时通过事务外校验，然后分别进入 [db/sqlc/tx_create_order.go](../db/sqlc/tx_create_order.go#L42) 创建订单。
- 这会破坏“一个预订只能有一个活跃订单”的业务不变量。

可能后果：

- 同一预订产生多个活跃订单。
- 后续支付入口可能同时指向多笔订单。
- 押金抵扣、替单、预订改菜和退款链路的语义变得不确定。

触发条件：

- 同一 `reservation_id` 在短时间内收到两个及以上并发建单请求。
- 这些请求都在事务开启前通过 `EnsureReservationSingleActiveOrder` 的只读校验。
- 数据库层没有额外唯一约束或锁语义把第二个活跃订单挡住。

影响范围细化：

- 影响对象集中在预订押金模式，不是所有普通订单都会触发。
- 一旦形成双活订单，后续支付、替单、取消、退款都可能围绕不同订单继续推进，属于源头级业务不变量破坏。
- 该问题会把后续多个资金动作变成“各自局部看起来合法，但整体指向同一预订”的复杂坏状态。

修复优先级建议：

- 建议优先级：P0。
- 理由：这是核心业务约束失守，触发后会同时影响订单唯一性和后续资金语义，后补修复成本明显高于前置拦截。

最小修复面：

- 把“单预订唯一活跃订单”约束下沉到事务或数据库约束层，不能继续只依赖事务外只读检查。
- 明确活跃订单的判定状态集合，避免后续替单、取消、支付中状态与唯一约束口径再次漂移。

涉及模块：

- `logic/order_service.go`
- `logic/order_post.go`
- `db/sqlc/tx_create_order.go`
- 相关 migration 与订单索引定义

回归验证点：

- 同一预订串行建单时，合法首单不受影响。
- 同一预订并发双击建单时，最多只能成功一笔活跃订单。
- 替单、取消后再次建单应符合业务预期，不应被新的唯一约束误伤。

### 3. 并发发起支付时，本地关单与微信下单不在同一原子边界内，可能生成“本地已关闭但微信仍可支付”的陈旧预支付单

- 严重级别：高危
- 审查范围：订单支付链路
- 影响面：订单支付幂等性、重复发起支付、异常到账退款路径、用户真实扣款体验

证据：

- [logic/payment_order_service.go](../logic/payment_order_service.go#L142) 到 [logic/payment_order_service.go](../logic/payment_order_service.go#L160) 会先读取“最新支付单”决定复用或关闭，但这一步发生在真正创建新支付单事务之前。
- 对订单主支付，后续会进入 [logic/payment_order_service.go](../logic/payment_order_service.go#L451) 到 [logic/payment_order_service.go](../logic/payment_order_service.go#L481)，先执行 `CreateCombinedPaymentTx`，事务提交后才调用微信 `CreateCombineOrder`。
- 在事务内部，[db/sqlc/tx_create_combined_payment.go](../db/sqlc/tx_create_combined_payment.go#L94) 到 [db/sqlc/tx_create_combined_payment.go](../db/sqlc/tx_create_combined_payment.go#L111) 会把同订单最新的 pending 支付单直接更新为 `closed`，然后创建新的子支付单。
- 合单回调里已经显式处理了“子支付单本地已是 closed/failed，但微信仍回调支付成功”的场景：[api/payment_callback.go](../api/payment_callback.go#L1631) 到 [api/payment_callback.go](../api/payment_callback.go#L1656)。当前处理方式不是阻止该状态出现，而是走异常到账退款补偿。

风险判断：

- 如果两个支付请求并发进入，同一订单的前一个本地支付单可能被后一个请求在事务内先关闭。
- 但前一个请求在事务提交后仍可能继续调用微信创建合单，生成一个微信侧仍可支付的旧 `prepay`。
- 这说明“本地支付单状态切换”和“微信侧真实可支付状态”之间存在竞争窗口，系统当前是靠回调后的异常退款来兜底，而不是在支付创建阶段把该窗口关闭。

可能后果：

- 用户可能对一个已经在本地被关闭的旧支付单完成真实扣款。
- 系统随后只能走异常到账退款，产生额外退款时延、客服噪音和对账复杂度。
- 当并发点击、弱网重试或客户端多次拉起支付较多时，这条链更容易暴露。

触发条件：

- 同一订单在短时间内被重复点击支付、客户端重试或多端同时发起支付。
- 较早请求在本地支付单被后续请求关闭后，仍继续向微信成功创建合单。
- 用户最终在微信侧完成了这笔已经被本地逻辑视为失效的旧预支付单付款。

影响范围细化：

- 影响对象主要是订单主支付，尤其是并发发起支付概率高的移动端场景。
- 该问题不一定导致资金丢失，但会把正常支付链路降级为“先异常收款，再走退款补偿”的高成本闭环。
- 影响边界会延伸到异常到账退款、客服申诉、财务对账和用户支付体验，而不是停留在支付创建瞬间。

修复优先级建议：

- 建议优先级：P0。
- 理由：真实扣款已经可能发生，现有补偿路径只是事后兜底，不足以把该问题视为低优先级一致性瑕疵。

最小修复面：

- 收敛“关闭旧支付单”和“创建新可支付单”的并发窗口，避免旧单在本地失效后仍能成功获得微信侧可支付资格。
- 明确旧预支付单复用、关闭、重建的唯一判定顺序，减少不同请求之间的交叉覆盖。

涉及模块：

- `logic/payment_order_service.go`
- `db/sqlc/tx_create_combined_payment.go`
- `api/payment_callback.go`

回归验证点：

- 同一订单重复点击支付时，最终只应存在一笔真实可支付的有效支付尝试。
- 被本地关闭的旧支付单不应再出现微信侧成功扣款回调。
- 弱网重试、多端并发拉起支付时，不应再依赖异常到账退款作为主路径收敛手段。

### 4. 过期合单主记录缺少已接通的批量兜底清理路径，超时任务缺失时可能长期停留在 pending

- 严重级别：中危
- 审查范围：订单支付链路
- 影响面：合单状态一致性、查询展示、对账与恢复判断

证据：

- 单支付和合单支付创建后，超时任务入队失败都只记录日志，不阻断主流程：[api/payment_order.go](../api/payment_order.go#L293) 到 [api/payment_order.go](../api/payment_order.go#L299)、[api/payment_order.go](../api/payment_order.go#L402) 到 [api/payment_order.go](../api/payment_order.go#L408)。
- 仓库里确实定义了“查出已过期 pending 合单”的 SQL：[db/query/combined_payment.sql](../db/query/combined_payment.sql#L60)。
- 但当前可见生产代码没有使用这条查询做批量清理；相反，已接通的 data cleanup 只调用 [scheduler/data_cleanup.go](../scheduler/data_cleanup.go#L661) 到 [scheduler/data_cleanup.go](../scheduler/data_cleanup.go#L667) 的 `CloseExpiredPaymentOrders`。
- 而 [db/query/payment_order.sql](../db/query/payment_order.sql#L190) 到 [db/query/payment_order.sql](../db/query/payment_order.sql#L193) 只会关闭 `payment_orders`，不会关闭 `combined_payment_orders` 主记录。

风险判断：

- 一旦合单超时任务没有成功入队、worker 长时间不可用，或者相关 timeout 链在局部失败，子支付单可以被其他清理路径关闭，但合单主记录仍可能长期保持 `pending`。
- 当前仓库内没有看到一个已经接入调度器的“批量关闭过期合单主记录”兜底流程。

可能后果：

- `combined_payment_orders` 主状态与子支付单状态漂移。
- 合单查询、报表、恢复判断可能持续把已经失效的支付尝试视为待支付。
- 后续人工排障时会看到“主合单 pending、子单已 closed”的脏状态，增加定位成本。

触发条件：

- 创建合单后，超时任务入队失败或后续超时任务未被消费。
- worker、调度器或局部超时链发生停摆，导致过期合单没有按预期关闭。
- 子支付单已被其他路径关闭，但主合单记录没有被批量兜底收敛。

影响范围细化：

- 影响对象是合单主记录与其子单之间的状态一致性，不一定直接造成重复扣款。
- 更偏向状态治理、查询语义和恢复判断层面的坏数据累积。
- 若坏状态积累较多，会干扰运维对“真实待支付”和“历史脏数据”的区分，间接放大排障和对账成本。

修复优先级建议：

- 建议优先级：P1。
- 理由：问题主要体现在状态长期漂移和运维可观测性恶化，风险真实存在，但直接资金损失链路弱于前几项。

最小修复面：

- 为过期 `combined_payment_orders` 主记录补上一条已接通的批量关闭兜底路径。
- 明确主合单与子支付单的状态收敛关系，避免后续清理只动子单、不动主单。

涉及模块：

- `db/query/combined_payment.sql`
- `scheduler/data_cleanup.go`
- 合单超时任务调度与相关 worker

回归验证点：

- 超时任务正常执行时，主合单和子支付单都应按预期关闭。
- 超时任务丢失或 worker 不可用后，批量清理仍能最终关闭过期主合单。
- 查询和报表侧不再出现“主合单 pending、子单已 closed”的长期脏状态。

### 5. 围栏自动确认送达使用固定解冻金额 5000，和正常履约解冻口径不一致，存在骑手押金冻结金额漂移风险

- 严重级别：高危
- 审查范围：订单履约链路
- 影响面：骑手押金冻结/解冻、骑手余额、后续接单资格

证据：

- 正常人工确认送达路径会先按订单金额计算应解冻额度：[logic/delivery_status.go](../logic/delivery_status.go#L331) 到 [logic/delivery_status.go](../logic/delivery_status.go#L338)。
- 自动围栏确认送达入口 [api/rider_location_events.go](../api/rider_location_events.go#L281) 到 [api/rider_location_events.go](../api/rider_location_events.go#L289) 调用 `AutoConfirmDelivery` 时把 `unfreezeAmount` 固定传为 `5000`。
- `AutoConfirmDelivery` 会直接把这个入参传给 [db/sqlc/tx_delivery.go](../db/sqlc/tx_delivery.go#L257) 的 `CompleteDeliveryTx` 执行实际解冻。

风险判断：

- 正常路径的解冻金额来源于 `OrderFreezeAmount(order)`，而自动路径使用常量值，两条履约终态路径的资金口径并不一致。
- 当订单冻结金额不等于 5000 分时，自动确认送达会导致骑手冻结押金与实际应解冻额度发生偏差。

可能后果：

- 订单金额高于 5000 分时，骑手会被少解冻，冻结余额长期残留。
- 订单金额低于 5000 分时，可能出现超额解冻，影响押金账实一致性。
- 后续骑手接单资格、押金流水和人工对账都可能被污染。

触发条件：

- 订单通过围栏事件自动确认送达，而不是走人工确认送达路径。
- 该订单对应的实际冻结金额不等于固定值 `5000` 分。
- 自动确认路径成功执行 `CompleteDeliveryTx`，将固定金额写入实际解冻动作。

影响范围细化：

- 影响对象限定为骑手配送且命中自动围栏确认的订单，不是所有送达订单都会触发。
- 风险直接落在骑手押金冻结余额和押金流水账实一致性上，会进一步影响骑手可接单状态。
- 若系统后续再基于冻结余额做资格或风控判断，影响会从单笔订单扩散到骑手账户维度。

修复优先级建议：

- 建议优先级：P0。
- 理由：这是直接的资金口径错误，自动化路径一旦命中就会真实落账，且会持续污染骑手账户状态。

最小修复面：

- 自动确认送达路径必须复用与人工确认一致的解冻金额来源，不能继续依赖固定常量。
- 顺带排查其他自动履约入口是否也绕过了统一冻结/解冻口径计算。

涉及模块：

- `api/rider_location_events.go`
- `logic/delivery_status.go`
- `db/sqlc/tx_delivery.go`

回归验证点：

- 自动确认送达和人工确认送达在同一订单上应产出一致的解冻金额。
- 订单金额高于、低于、等于 `5000` 分的场景都应覆盖。
- 骑手冻结余额、押金流水和接单资格在自动确认后应保持账实一致。

### 6. 骑手抢单把配送分配、移出抢单池、冻结押金放在事务内，但订单状态同步放在事务外，存在履约状态分裂风险

- 严重级别：高危
- 审查范围：订单履约链路
- 影响面：订单状态、配送状态、押金冻结、抢单池一致性

证据：

- [logic/delivery_grab.go](../logic/delivery_grab.go#L133) 到 [logic/delivery_grab.go](../logic/delivery_grab.go#L149) 先执行 `GrabOrderTx`，事务成功后再单独调用 `UpdateOrderToCourierAccepted`。
- [db/sqlc/tx_delivery.go](../db/sqlc/tx_delivery.go#L162) 的 `GrabOrderTx` 在同一事务内完成：分配配送单、移出 `delivery_pool`、冻结押金、写押金流水。
- 订单状态更新 `courier_accepted` 不在这个事务里，而是事务后单独写入；状态日志也是后续单独补写。

风险判断：

- 一旦 `GrabOrderTx` 成功提交，而后续 `UpdateOrderToCourierAccepted` 因并发状态变化、短暂数据库故障或其他错误失败，系统就会留下“配送已分配、押金已冻结、订单池已移除，但订单状态仍未推进”的分裂状态。
- 当前代码对这种情况直接返回错误，但前序副作用已经落库，属于典型的跨事务不一致风险。

可能后果：

- 骑手端看到已接单，订单主状态却仍停留在 `ready`。
- 商户、用户、调度补偿和后续配送动作会基于不同状态视图工作。
- 押金已经冻结，但订单链路可能被误判为“尚未接单”或再次进入异常处理。

触发条件：

- 骑手抢单事务 `GrabOrderTx` 已经成功提交。
- 随后的 `UpdateOrderToCourierAccepted` 因数据库瞬时错误、并发状态变化或调用失败未能成功写入。
- 调用方收到错误返回，但前序配送分配、移池、冻结押金副作用已经不可回滚。

影响范围细化：

- 影响对象是抢单成功这一跳的跨事务一致性，而不是整个配送全链路都天然失效。
- 一旦触发，订单主状态、配送单状态、抢单池状态、押金状态会形成多视图分裂，后续任何依赖单一状态机的模块都可能误判。
- 该问题尤其影响高并发抢单、数据库抖动或事务提交后单独写状态失败的生产场景。

修复优先级建议：

- 建议优先级：P0。
- 理由：抢单是履约入口关键跳点，分裂后既影响资金冻结又影响状态机推进，恢复通常需要人工介入。

最小修复面：

- 把订单状态推进和抢单核心副作用放进同一收敛边界，至少不能再允许“前序副作用已提交、主订单状态失败”这种半成功结果落库。
- 若短期内无法合并事务，也需要提供明确的补偿或重放机制，而不是仅返回错误。

涉及模块：

- `logic/delivery_grab.go`
- `db/sqlc/tx_delivery.go`
- 订单状态更新与状态日志相关逻辑

回归验证点：

- 抢单成功后，订单状态、配送状态、押金冻结、抢单池移除必须同时一致。
- 模拟事务后半段失败时，不应留下“已分配但订单仍 ready”的分裂状态。
- 高并发抢单下，同一订单不应出现重复冻结或多次移池副作用。

### 7. 配送状态事务对订单同步失败采取“吞错提交”，且开始取餐 SQL 缺少源状态约束，存在状态漂移与回退风险

- 严重级别：高危
- 审查范围：订单履约链路
- 影响面：配送状态机、订单状态机、自动围栏推进、异常恢复判断

证据：

- `UpdateDeliveryToPickupTx` 在 [db/sqlc/tx_delivery.go](../db/sqlc/tx_delivery.go#L62) 先推进配送单，再在 [db/sqlc/tx_delivery.go](../db/sqlc/tx_delivery.go#L70) 同步订单；若订单更新返回 `ErrRecordNotFound`，在 [db/sqlc/tx_delivery.go](../db/sqlc/tx_delivery.go#L72) 直接 `return nil` 提交事务。
- `UpdateDeliveryToPickedTx` 与 `UpdateDeliveryToDeliveringTx` 也分别在 [db/sqlc/tx_delivery.go](../db/sqlc/tx_delivery.go#L99)、[db/sqlc/tx_delivery.go](../db/sqlc/tx_delivery.go#L141) 后吞掉订单同步失败。
- 更严重的是，开始取餐 SQL [db/query/delivery.sql](../db/query/delivery.sql#L44) 到 [db/query/delivery.sql](../db/query/delivery.sql#L48) 只校验 `id` 和 `rider_id`，没有校验原状态必须是 `assigned`。

风险判断：

- 当前实现允许“配送状态更新成功，但订单状态未同步”这种半成功事务被提交。
- `startPickup` 这一跳甚至没有在 SQL 层保护原状态，意味着只要应用层读到的状态与实际落库状态存在竞态，就可能把一个同骑手持有的配送单从意外状态重新写回 `picking`。

可能后果：

- 订单状态与配送状态长期不一致，例如配送单已 `picked`，订单仍停留在 `courier_accepted`。
- 自动围栏推进与人工操作叠加时，状态机可能发生回退或跳跃。
- 后续通知、超时判断、赔付和客服排障都会基于错误状态运行。

触发条件：

- 配送状态推进事务执行成功，但对应订单状态同步命中 `ErrRecordNotFound` 或其他被吞掉的失败路径。
- 或者同骑手配送单在并发/重试场景下重复触发开始取餐，SQL 由于缺少源状态约束而接受了不符合预期的状态推进。
- 自动围栏、人工点击、弱网重试等多源操作叠加时更容易暴露。

影响范围细化：

- 影响对象是配送状态机和订单状态机之间的长期一致性，不仅是一条日志缺失问题。
- 该问题会污染通知发送、超时判断、赔付判责、客服排障等所有依赖订单主状态或配送状态的下游模块。
- 因为事务本身会提交，所以坏状态可能长期存在，不会靠简单重试自然恢复。

修复优先级建议：

- 建议优先级：P0。
- 理由：这是核心履约状态机的系统性一致性风险，并且已经存在“吞错提交”和缺少源状态保护两层缺陷。

最小修复面：

- 取消“订单同步失败仍提交配送状态”的吞错语义，至少要把失败显式纳入可补偿路径。
- 为开始取餐等关键状态迁移补上数据库侧源状态保护，避免并发重试把配送状态写回旧阶段。

涉及模块：

- `db/sqlc/tx_delivery.go`
- `db/query/delivery.sql`
- 相关配送状态推进调用方

回归验证点：

- 配送状态推进时，如果订单状态未成功同步，不应留下已提交的半成功事务。
- `assigned -> picking -> picked -> delivering` 迁移应严格单向，不允许重复点击或重试造成回退。
- 自动围栏、人工推进、弱网重试组合场景下，订单状态与配送状态应保持一致。

### 8. 外卖配送地址在“出餐完成后创建配送单”时从地址簿现读，未固化订单地址快照，存在已支付订单目的地可变风险

- 严重级别：中危
- 审查范围：订单履约链路
- 影响面：配送目的地、配送时效、距离与费用语义、历史订单可追溯性

证据：

- 订单创建时 [db/query/order.sql](../db/query/order.sql#L1) 到 [db/query/order.sql](../db/query/order.sql#L23) 只保存 `address_id`，没有把地址文本、联系人、经纬度固化到订单表。
- 外卖单在商户出餐后才由 [db/sqlc/tx_takeout_order.go](../db/sqlc/tx_takeout_order.go#L123) 的 `ensureTakeoutDeliveryCreated` 创建配送单，并在 [db/sqlc/tx_takeout_order.go](../db/sqlc/tx_takeout_order.go#L146) 现查 `GetUserAddress` 读取最新地址内容。
- 用户可以通过 [api/user_address.go](../api/user_address.go#L282) 修改同一地址记录。
- 订单详情查询也仍然通过 [db/query/order.sql](../db/query/order.sql#L54) 连接 `user_addresses` 读取展示地址，而不是读取订单快照。

风险判断：

- 已支付订单的真实配送目标依赖一条可被用户后续修改的地址簿记录，而不是下单当时的不可变快照。
- 这会让履约目标、展示地址和下单时用于报价的地址语义发生漂移。

可能后果：

- 用户在支付后、商户出餐前修改地址簿记录，配送单会落到变更后的地址。
- 实际配送距离、服务范围和配送费可能与支付时依据不一致。
- 历史订单详情和后续申诉取证无法稳定还原下单时的真实地址信息。

触发条件：

- 用户下单后但配送单真正创建前，修改了同一 `address_id` 对应的地址簿记录。
- 订单履约进入“出餐完成后创建配送单”路径，系统重新按 `address_id` 读取最新地址。
- 下单时没有把联系人、电话、门牌号、经纬度等地址快照固化到订单。

影响范围细化：

- 影响对象主要是外卖订单，尤其是支付完成到配送创建之间存在时间窗口的场景。
- 该问题同时影响履约目标、历史取证和费用语义，但不一定每次都会直接转化为资金损失。
- 如果用户恶意利用，问题会从普通数据漂移升级为“支付后换地址”的履约规则绕过。

修复优先级建议：

- 建议优先级：P1。
- 理由：风险会直接影响履约目标与历史可追溯性，但其爆炸半径通常取决于用户是否在特定时间窗内修改地址，低于直接越权或直接资金错账问题。

最小修复面：

- 在下单时固化订单地址快照，后续配送创建和订单详情展示应读取订单快照而不是地址簿现值。
- 明确快照字段口径，至少覆盖联系人、手机号、详细地址和经纬度等履约关键字段。

涉及模块：

- `db/query/order.sql`
- `db/sqlc/tx_takeout_order.go`
- `api/user_address.go`
- 订单详情查询与展示相关代码

回归验证点：

- 用户下单后修改地址簿，不应影响已支付订单的配送目的地和详情展示。
- 新下单仍应使用最新地址成功报价和建单。
- 历史订单查询、申诉取证和配送创建应读取同一份不可变快照。

### 9. 主退款事务只统计 success 退款，pending/processing 期间仍可继续创建新退款单，累计退款保护实际失效

- 严重级别：高危
- 审查范围：退款与逆向链路
- 影响面：人工退款、商户拒单退款、预订取消退款、恢复调度补偿退款

证据：

- [db/sqlc/tx_refund.go](../db/sqlc/tx_refund.go#L35) 到 [db/sqlc/tx_refund.go](../db/sqlc/tx_refund.go#L69) 的 `CreateRefundOrderTx` 通过 `GetPaymentOrderForUpdate` 加锁后，调用 `GetTotalRefundedByPaymentOrder` 做累计退款校验。
- 但底层 SQL [db/query/refund_order.sql](../db/query/refund_order.sql#L74) 到 [db/query/refund_order.sql](../db/query/refund_order.sql#L77) 只统计 `status = 'success'` 的退款单，未把 `pending` / `processing` 算入占用额度。
- 这套事务已经被主退款入口复用，例如 [logic/refund_service.go](../logic/refund_service.go#L115)、[logic/merchant_reject_refund.go](../logic/merchant_reject_refund.go#L83)、[logic/reservation.go](../logic/reservation.go#L559)。

风险判断：

- 当前行锁只能串行化“同时进入”的退款创建请求，但第一笔退款一旦建单后进入 `pending` 或 `processing`，后续退款请求仍会因为累计额查询只看 `success` 而再次通过校验。
- 这意味着代码自认为已经关闭的“超退窗口”实际上仍然存在，只是从并发超退退化成了“异步回调完成前的顺序超退”。

可能后果：

- 同一支付单在第一笔退款尚未完成时还能继续生成第二笔、第三笔退款单。
- 多笔退款最终陆续成功回调后，真实退款总额可能超过原支付金额。
- 对账、客服、资金追溯都会面对“每笔单独合法，但合计超退”的隐蔽坏账场景。

触发条件：

- 同一支付单上第一笔退款已经建单，但状态仍停留在 `pending` 或 `processing`。
- 在第一笔退款最终成功回调前，又有后续退款请求进入同一支付单。
- 后续退款创建时累计退款查询仍只统计 `success`，因此继续放行。

影响范围细化：

- 影响对象覆盖所有复用该主退款事务的退款入口，不是单一业务分支的局部问题。
- 风险本质是“可退款额度占用”口径错误，会直接扩散到人工退款、拒单退款、预订取消退款等多条链路。
- 只有在异步回调完成后才会显性暴露，属于前台不易察觉、后台代价很高的资金坏账类型。

修复优先级建议：

- 建议优先级：P0。
- 理由：这是直接的超额退款风险，而且问题位于通用事务能力层，影响面广、资金后果重。

最小修复面：

- 累计可退款额度校验必须把 `pending`、`processing` 等已占用退款额度纳入统一口径，而不是只统计 `success`。
- 统一退款事务的所有调用方都应继承这套口径，避免不同退款入口出现分叉规则。

涉及模块：

- `db/sqlc/tx_refund.go`
- `db/query/refund_order.sql`
- `logic/refund_service.go`
- `logic/merchant_reject_refund.go`
- `logic/reservation.go`

回归验证点：

- 第一笔退款进入 `pending` 或 `processing` 后，后续退款请求应按剩余额度被正确拦截或放行。
- 多入口退款并发或顺序发起时，总退款额都不应超过原支付额。
- 正常单笔退款与分批合法退款场景不应被误伤。

### 10. 预订改菜退款把全部预付金额压到原始预订支付单，忽略 reservation_addon 支付记录，存在退款打错交易和超原单金额失败风险

- 严重级别：高危
- 审查范围：退款与逆向链路
- 影响面：预订全款改菜后的部分退款、预订加菜补差后的逆向退款

证据：

- `reservation_addon` 支付成功后会把补差金额并入预付余额：[db/sqlc/tx_payment_success.go](../db/sqlc/tx_payment_success.go#L166) 到 [db/sqlc/tx_payment_success.go](../db/sqlc/tx_payment_success.go#L189)。
- 改菜退款时，退款额度上限直接取 [logic/reservation_dishes.go](../logic/reservation_dishes.go#L212) 的 `reservation.PrepaidAmount`，也就是包含了 addon 补差后的总预付额。
- 但真正选取退款原支付单时，[logic/reservation_dishes.go](../logic/reservation_dishes.go#L219) 到 [logic/reservation_dishes.go](../logic/reservation_dishes.go#L221) 强制只查 `BusinessType: businessTypeReservation`，明确排除了 `reservation_addon` 支付单。
- 随后退款请求又直接把这笔原始预订支付单的 `OutTradeNo` 送给微信：[logic/reservation_dishes.go](../logic/reservation_dishes.go#L246) 到 [logic/reservation_dishes.go](../logic/reservation_dishes.go#L251)。

风险判断：

- 一旦用户先做过加菜补差，后续改菜退款的可退金额就可能已经包含 addon 交易金额，但系统仍然只对原始预订支付单发起退款。
- 这会把多笔交易形成的预付余额错误地映射到单一微信交易上，资金归因和退款执行对象都发生漂移。

可能后果：

- 退款金额超过原始预订支付单总额时，微信退款请求可能直接失败。
- 即使金额未超限，实际被退回的也是“原单资金”，而不是后续 addon 补差对应的那笔交易。
- 多次加菜、改菜后，预订账务会越来越依赖人工解释，难以证明每笔退款到底冲销了哪一笔入账。

触发条件：

- 预订订单先经历过一次或多次 `reservation_addon` 补差支付。
- 之后发生改菜退款，且退款额度计算包含了 addon 并入后的总预付余额。
- 退款执行时系统仍只选取原始 `reservation` 支付单作为微信退款目标交易。

影响范围细化：

- 影响对象限定在预订类的“加菜补差后再改菜退款”场景，不是所有预订退款都会触发。
- 一旦触发，问题会落在资金归因层面，即便用户最终收到了钱，系统也难以证明退款精确冲销了哪一笔入账。
- 当 addon 次数增多、金额组合复杂时，风险会从单笔失败放大为整条预订账务解释困难。

修复优先级建议：

- 建议优先级：P0。
- 理由：这是把多笔入账错误映射到单笔原交易上的资金对象错配问题，直接关系退款成功率和账务可证明性。

最小修复面：

- 改菜退款的退款来源必须与真实入账交易集合一致，不能再默认把总预付余额全部映射到原始预订支付单。
- 需要明确多笔 `reservation_addon` 入账后的退款分摊策略，至少保证退款对象和金额口径可追溯。

涉及模块：

- `logic/reservation_dishes.go`
- `db/sqlc/tx_payment_success.go`
- 预订支付与退款相关账务模型

回归验证点：

- 无 addon 的普通预订改菜退款行为应保持兼容。
- 存在一笔或多笔 addon 时，退款应命中正确的原交易集合，不再错误压到单一原始支付单。
- 微信退款成功率、退款金额上限和本地账务映射应能相互印证。

### 11. 预订改菜退款没有接入完整异步闭环，processing 成功后不会回写预付余额，接口临时失败也不会被恢复调度重试

- 严重级别：高危
- 审查范围：退款与逆向链路
- 影响面：预订预付余额、后续可退款额度、预订改菜/取消的资金一致性

证据：

- 改菜退款分支直接调用 [logic/reservation_dishes.go](../logic/reservation_dishes.go#L234) 的 `CreateRefundOrder`，没有复用统一退款事务。
- 当微信接口报错时，[logic/reservation_dishes.go](../logic/reservation_dishes.go#L255) 会立即把退款单改成 `failed`，而不是像主退款链那样保留 `pending` 进入恢复调度。
- 只有“同步返回 success”时才会在 [logic/reservation_dishes.go](../logic/reservation_dishes.go#L261) 到 [logic/reservation_dishes.go](../logic/reservation_dishes.go#L266) 扣减 `reservation.prepaid_amount`。
- 通用退款回调处理 [worker/task_process_payment.go](../worker/task_process_payment.go#L871) 到 [worker/task_process_payment.go](../worker/task_process_payment.go#L934) 在退款成功后只更新 `refund_orders` / `payment_orders`，没有任何预订预付余额回写逻辑。
- 批量恢复查询 [db/query/payment_order.sql](../db/query/payment_order.sql#L213) 到 [db/query/payment_order.sql](../db/query/payment_order.sql#L222) 只覆盖 `business_type = 'reservation'` 的取消预订退款，没有覆盖 `reservation_addon` 这条改菜退款支线。

风险判断：

- 如果微信先返回 `processing`，后续即使回调成功，系统也不会再补扣预订预付余额。
- 如果微信接口只是临时失败，这条支线会被直接打成 `failed`，而当前恢复调度又不会重新捞起 `reservation_addon` 相关退款。

可能后果：

- 用户真实已经收到退款，但预订 `prepaid_amount` 仍维持旧值，后续可退款额度被高估。
- 后续再次改菜或取消预订时，系统可能基于漂移后的预付余额继续计算退款，放大资金风险。
- 临时网络抖动、微信短暂故障会把本可自动恢复的退款直接变成需要人工排查的遗留单。

触发条件：

- 预订改菜退款走的是 `reservation_addon` 这条特化分支，而不是主退款闭环。
- 微信同步返回 `processing`，或者第一次调用因网络抖动、短暂故障未能稳定返回成功。
- 后续即便微信实际退款成功，也没有统一回调逻辑把 `prepaid_amount` 补写回正确值。

影响范围细化：

- 影响对象集中在预订改菜退款，不直接等同于普通订单退款问题。
- 风险既体现在单笔退款能否自愈，也体现在预订账户余额是否继续作为后续退款和改菜计算的可信输入。
- 由于这是一条状态和余额双漂移链路，越晚发现，后续叠加的改菜、取消、退款动作越难还原。

修复优先级建议：

- 建议优先级：P0。
- 理由：该问题会同时造成真实退款结果与本地预付余额脱节，而且当前恢复闭环本身就不完整，属于高概率沉积遗留单的资金风险。

最小修复面：

- 预订改菜退款必须接入与主退款链一致的异步状态收敛与余额回写机制，不能继续依赖同步成功分支单点更新 `prepaid_amount`。
- `reservation_addon` 相关退款也需要进入统一恢复调度覆盖面，避免接口瞬时失败直接变终态死单。

涉及模块：

- `logic/reservation_dishes.go`
- `worker/task_process_payment.go`
- `db/query/payment_order.sql`
- 预订预付余额更新相关逻辑

回归验证点：

- 微信同步返回 `success`、`processing`、临时失败三种路径下，最终退款结果与 `prepaid_amount` 都应收敛一致。
- `reservation_addon` 改菜退款在回调丢失、任务重试、恢复调度场景下应仍能闭环。
- 后续再次改菜或取消预订时，可退款额度应以真实已退款结果为准。

### 12. 商户/运营商提现先落本地 pending 记录再调微信，提交前进程中断会生成无法自愈的僵尸提现单

- 严重级别：中危
- 审查范围：充值提现分账链路
- 影响面：商户提现、运营商提现、提现状态可追踪性、恢复调度噪音

证据：

- 商户提现路径先在 [api/merchant_finance.go](../api/merchant_finance.go#L1378) 创建 `withdrawal_record`，随后才在 [api/merchant_finance.go](../api/merchant_finance.go#L1395) 调用 `CreateEcommerceWithdraw`。
- 运营商提现路径也一样：先在 [api/operator_finance.go](../api/operator_finance.go#L269) 创建 `withdrawal_record`，再在 [api/operator_finance.go](../api/operator_finance.go#L286) 调微信提现。
- 后续恢复链 [worker/task_merchant_withdraw_result.go](../worker/task_merchant_withdraw_result.go#L111) 只会按 `out_request_no` 去微信查询；如果连续查询失败，最终在 [worker/task_merchant_withdraw_result.go](../worker/task_merchant_withdraw_result.go#L131) 直接停止重试，并把记录继续保留在 `pending`。

风险判断：

- 当前实现没有“已本地建单但尚未真正提交到微信”的中间状态，也没有在恢复链里区分“微信确实处理中”和“请求根本没打出去”。
- 一旦在 `CreateWithdrawalRecord` 成功后、`CreateEcommerceWithdraw` 发出前发生进程崩溃、请求超时或调用链中断，本地就会留下一个永远查不到微信侧结果的 pending 提现单。

可能后果：

- 商户或运营商会在本地看到一笔长期 pending 的提现记录，但微信侧根本不存在对应提现。
- 恢复调度只能不断告警，无法自动收敛状态。
- 财务排障需要人工甄别“真实已提交”与“本地假单”，提高运营成本。

触发条件：

- 本地 `withdrawal_record` 已创建成功。
- 在真正调用微信发起提现前后发生进程崩溃、链路超时、请求未发出或调用结果未知。
- 恢复任务后续只能依据 `out_request_no` 查微信结果，但微信侧从未收到这笔申请。

影响范围细化：

- 影响对象覆盖商户提现和运营商提现两条链路，因为二者采用相同的“先落本地单、后调微信”模式。
- 问题更偏向状态真实性和恢复能力缺失，不一定直接造成资金损失，但会造成长期待处理假单。
- 当提现量增长时，这类僵尸单会持续侵蚀财务运营对“真实处理中单量”的判断准确性。

修复优先级建议：

- 建议优先级：P1。
- 理由：风险真实且会造成运营成本上升，但相较于直接错付、超退或错账，主要损害集中在状态收敛与人工排障层面。

最小修复面：

- 为提现链补出“本地已建单但上游提交未知/未提交”的可识别状态或等效恢复语义。
- 恢复链不能只会查微信结果，还需要能区分“微信处理中”与“本地僵尸单”。

涉及模块：

- `api/merchant_finance.go`
- `api/operator_finance.go`
- `worker/task_merchant_withdraw_result.go`
- 提现记录状态模型

回归验证点：

- 模拟本地建单成功后进程中断，系统后续应能识别并收敛这类未真正提交的提现单。
- 正常已提交微信提现不应被错误标记为僵尸单。
- 商户提现和运营商提现两条链路都应覆盖同样的恢复语义。

### 13. 预订与预订加菜分账失败后缺少恢复入口，恢复器和失败重试都只接受 order_id

- 严重级别：高危
- 审查范围：充值提现分账链路
- 影响面：预订分账、预订加菜补差分账、商户收入确认、平台/运营商分账统计

证据：

- 预订与 `reservation_addon` 支付成功时，系统明确按 `ReservationID` 入队分账任务：[worker/task_process_payment.go](../worker/task_process_payment.go#L565) 到 [worker/task_process_payment.go](../worker/task_process_payment.go#L569)。
- 但恢复调度器在 [worker/profit_sharing_recovery_scheduler.go](../worker/profit_sharing_recovery_scheduler.go#L121) 遇到 `payment_order.order_id` 为空就直接跳过，并且只在 [worker/profit_sharing_recovery_scheduler.go](../worker/profit_sharing_recovery_scheduler.go#L125) 用 `OrderID` 重新入队。
- 分账失败后的回调重试也沿用同样限制：[worker/task_process_payment.go](../worker/task_process_payment.go#L2576) 到 [worker/task_process_payment.go](../worker/task_process_payment.go#L2590) 只有 `paymentOrder.OrderID.Valid` 时才会重试。

风险判断：

- 预订和预订加菜的分账主键本来就是 `reservation_id` 维度，但当前所有恢复与失败重试代码都默认“必须有 order_id”。
- 这使得预订分账一旦首轮失败、回调失败或任务丢失，就缺少真正能把它重新送回执行路径的补偿入口。

可能后果：

- 预订类支付可能已经成功，但对应分账单长期停在 `pending` / `failed`。
- 商户收入、平台抽成、运营商抽成统计会漏计或延迟到人工介入。
- 预订链路和普通订单链路在财务层出现不同步恢复语义，增加线上排查复杂度。

触发条件：

- 预订或 `reservation_addon` 支付成功后，分账首轮执行失败、回调丢失，或异步任务未成功消费。
- 对应支付单没有 `order_id`，只有 `reservation_id` 维度信息。
- 恢复器与失败重试逻辑因 `order_id` 为空直接跳过，导致补偿链未真正触发。

影响范围细化：

- 影响对象限定在预订及预订加菜分账，不直接影响普通订单分账。
- 该问题会直接推迟或阻断商户、平台、运营商的分账收敛，属于结算层资金确认缺口。
- 一旦线上预订业务占比提高，分账恢复语义的不一致会让财务监控呈现“普通订单正常、预订单异常堆积”的割裂现象。

修复优先级建议：

- 建议优先级：P0。
- 理由：这是明确的恢复入口缺失，影响真实分账闭环和收入确认，不是单纯的告警噪音问题。

最小修复面：

- 分账恢复器和失败重试入口必须支持 `reservation_id` 维度，不能把分账恢复路径硬编码到 `order_id`。
- 统一预订与普通订单的分账恢复模型，避免不同支付业务类型共享同一分账框架却使用不同主键假设。

涉及模块：

- `worker/task_process_payment.go`
- `worker/profit_sharing_recovery_scheduler.go`
- 预订支付成功后的分账任务投递逻辑

回归验证点：

- 预订与 `reservation_addon` 分账首轮失败后，应能被恢复器重新拉起。
- 普通订单分账恢复行为不应回归。
- 分账最终状态、商户收入确认和平台统计在预订场景下应能自动收敛。

### 14. processing 状态的分账单被恢复器重新入队后会立即短路返回，回调丢失时没有真实自愈能力

- 严重级别：高危
- 审查范围：充值提现分账链路
- 影响面：分账最终完成、分账状态收敛、结算后收入确认

证据：

- 恢复查询 [db/query/profit_sharing_order.sql](../db/query/profit_sharing_order.sql#L79) 到 [db/query/profit_sharing_order.sql](../db/query/profit_sharing_order.sql#L83) 会把 `processing` 状态的分账单一起扫出来。
- 调度器拿到这些记录后，仍然在 [worker/profit_sharing_recovery_scheduler.go](../worker/profit_sharing_recovery_scheduler.go#L125) 重新分发 `ProcessTaskProfitSharing`。
- 但实际处理函数看到已有分账单状态是 `processing` 时，会在 [worker/task_process_payment.go](../worker/task_process_payment.go#L1228) 到 [worker/task_process_payment.go](../worker/task_process_payment.go#L1233) 直接判定“already processed, skip” 并返回。

风险判断：

- 也就是说，恢复器虽然把 `processing` 分账单重新捞出来了，但投递到的任务并不会查询结果，也不会补轮询，只会原地跳过。
- 一旦微信分账回调丢失、结果任务没有入队成功，`processing` 分账单就缺少一条真正有效的自动收敛路径。

可能后果：

- 分账单会长期卡在 `processing`，但不会再有任何后台任务推进到 `finished` 或 `failed`。
- 商户到账通知、平台财务统计、异常告警都会基于一条永远未闭环的分账记录工作。
- 这种问题只会在 webhook 丢失或异步任务局部失败时暴露，属于典型的生产环境长尾坏状态。

触发条件：

- 分账请求已成功发出，分账单进入 `processing`。
- 微信回调丢失、结果查询任务未成功入队，或相关异步链路局部失败。
- 恢复调度重新捞起该记录后，处理函数因检测到 `processing` 状态而直接短路返回。

影响范围细化：

- 影响对象是所有会进入 `processing` 并依赖异步结果回收的分账单，不局限于某个单一商户或订单类型。
- 该问题不会在主干成功场景下出现，但一旦命中长尾异常，系统没有真正自动收敛能力。
- 影响边界会延伸到收入确认、结算报表、商户到账感知和异常告警信噪比。

修复优先级建议：

- 建议优先级：P0。
- 理由：这是恢复器“表面在跑、实际上无法推进状态”的设计缺陷，会让长尾异常永久化。

最小修复面：

- 对 `processing` 分账单的恢复动作必须真正执行结果查询或推进逻辑，不能只是重新投递到会立即短路的处理函数。
- 明确 `pending`、`processing`、`finished`、`failed` 各状态的恢复职责，避免恢复器与执行器语义相互打架。

涉及模块：

- `worker/profit_sharing_recovery_scheduler.go`
- `worker/task_process_payment.go`
- `db/query/profit_sharing_order.sql`

回归验证点：

- 分账进入 `processing` 后，即便回调丢失，也应能通过恢复链最终收敛到 `finished` 或 `failed`。
- 恢复器重新捞起 `processing` 记录时，不应再出现“重新入队但立即 skip”的空转现象。
- 商户到账通知、分账报表和异常告警应与最终收敛结果一致。

### 15. 索赔追偿支付没有自身超时收敛路径，过期 pending 支付单在清理延迟时会反复被复用并卡住真实付款人

- 严重级别：中危
- 审查范围：赔付追偿流水链路
- 影响面：商户/骑手追偿支付、追偿单关闭时效、支付重试体验

证据：

- 追偿支付逻辑在 [logic/claim_recovery_payment.go](../logic/claim_recovery_payment.go#L198) 查找旧支付单时，只按 `business_type + attach` 取最新记录，不校验 `expires_at`。
- 查询 SQL [db/query/payment_order.sql](../db/query/payment_order.sql#L55) 到 [db/query/payment_order.sql](../db/query/payment_order.sql#L59) 也没有任何过期或状态活跃窗口过滤。
- 只要旧支付单状态还是 `pending`，`getExistingClaimRecoveryPayment` 就会在 [logic/claim_recovery_payment.go](../logic/claim_recovery_payment.go#L210) 到 [logic/claim_recovery_payment.go](../logic/claim_recovery_payment.go#L215) 直接复用它。
- 但追偿支付 API [api/claim_recovery.go](../api/claim_recovery.go#L224) 到 [api/claim_recovery.go](../api/claim_recovery.go#L296) 并没有像常规支付入口那样调用 `scheduleTimeoutForPaymentOrder`；当前只能依赖全局清理 [scheduler/data_cleanup.go](../scheduler/data_cleanup.go#L661) 到 [scheduler/data_cleanup.go](../scheduler/data_cleanup.go#L665) 的 `CloseExpiredPaymentOrders` 被动收敛。
- 与此同时，唯一索引 [db/migration/000170_add_claim_recovery_payment_order_uniqueness.up.sql](../db/migration/000170_add_claim_recovery_payment_order_uniqueness.up.sql#L1) 到 [db/migration/000170_add_claim_recovery_payment_order_uniqueness.up.sql](../db/migration/000170_add_claim_recovery_payment_order_uniqueness.up.sql#L4) 明确把 `pending` 与 `paid` 的追偿支付单都视为活跃占位。

风险判断：

- 这意味着追偿支付是否能重新发起，完全依赖后台清理任务是否已经及时把过期单改成 `closed`。
- 一旦清理调度延迟、worker 停摆或局部失败，用户再次进入支付入口时仍会命中过期的 pending 支付单，拿到一个已经不可支付但仍被系统当作“可复用”的旧单。

可能后果：

- 商户或骑手会看到追偿支付单一直是 `pending`，但前端拿到的 `pay_params` 可能为空或对应已过期预支付单。
- 新的追偿支付单无法及时创建，真实付款人被旧单卡住。
- 追偿从 `overdue` 回到 `paid` 的链路会对全局清理任务形成隐式强依赖，降低这条资金支线的独立鲁棒性。

触发条件：

- 索赔追偿支付单已过期，但后台批量清理尚未及时把它关闭。
- 付款人再次进入追偿支付入口时，系统按 `business_type + attach` 命中了这笔旧的 `pending` 支付单。
- 旧支付单对应的预支付信息已失效或不可用，但唯一占位仍阻止新支付单创建。

影响范围细化：

- 影响对象集中在索赔追偿这条资金支线，不会直接扩散到普通订单支付。
- 问题核心是支付重试体验和支付闭环收敛能力，对真实付款人感知非常直接。
- 如果全局清理任务延迟较大，问题会从偶发卡单升级为持续阻断新的追偿支付尝试。

修复优先级建议：

- 建议优先级：P1。
- 理由：问题会直接阻塞追偿付款重试，但通常依赖过期与清理延迟共同成立，影响面相对聚焦于特定业务支线。

最小修复面：

- 追偿支付入口需要具备自身的过期识别和超时收敛能力，不能完全依赖全局清理任务放行下一次支付。
- 旧 `pending` 追偿支付单的复用判定应把是否过期、是否仍可支付纳入条件。

涉及模块：

- `logic/claim_recovery_payment.go`
- `api/claim_recovery.go`
- `db/query/payment_order.sql`
- `scheduler/data_cleanup.go`

回归验证点：

- 追偿支付单过期后，用户再次进入支付入口应能稳定获得新的有效支付尝试。
- 全局清理任务延迟时，追偿支付链也不应被旧 `pending` 单长期卡住。
- 正常未过期的追偿支付单复用逻辑应保持兼容，不引入重复建单。

## 备注

- 以上问题均基于当前仓库内可见代码、SQL 与 migration 结论。
- 如果线上数据库存在未纳入仓库的额外约束，第二项需要结合真实库结构重新校验。
