# 系统全链路代码与业务流程审计清单

> **创建时间**：2026-02-06
> **目标**：对系统代码进行全链路审计，发现潜在的代码问题和业务流程设计缺陷。
> **执行方式**：逐条审计，完成后勾选 `[x]`，发现问题记录在对应条目下。

### 核心风险总结 (2026-02-06 审计发现)

| 编号       | 风险等级 | 类别        | 描述                                                                                                                                                                                                       |
| :--------- | :------- | :---------- | :--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **P1-001** | Resolved | 基础设施    | ~~`app.env.example` 缺失关键微信配置，生产环境部署风险。~~ (**已修复**: Verified `WECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH` and other keys are present)                                                         |
| **P1-002** | Resolved | 日志        | ~~`main.go` 默认日志级别过高，可能掩盖底层数据库错误。~~ (**已修复**: Added `LOG_LEVEL` config defaulting to `info`)                                                                                       |
| **P1-003** | Resolved | 安全/配送   | ~~`grabOrder` 缺失骑手物理距离校验，允许异地抢单。~~ (**已修复**: `MaxGrabOrderDistanceMeters` 5公里距离校验)                                                                                              |
| **P1-004** | Resolved | 安全/索赔   | ~~`SubmitClaim` 在规则引擎故障时缺乏可靠的降级保护。~~ (**已修复**: Implemented fallback to manual review with error logging.)                                                                             |
| **P1-005** | Resolved | 安全/配送   | ~~`confirmDelivery` 缺失 LBS 地理围栏校验~~ (**已修复**: `DeliveryConfirmRadiusMeters` 500米围栏校验)                                                                                                      |
| **P1-006** | Critical | 架构约束    | 违反 Rule 3：`api/*.go` 堆积了大量核心业务逻辑，未解耦至 logic 层。                                                                                                                                        |
| **P1-007** | Resolved | 事务/SSOT   | ~~`adjustMemberBalance` 余额变动与流水记录未在同一事务内~~ (**已修复**: `AdjustMemberBalanceTx` 原子事务)                                                                                                  |
| **P1-008** | Medium   | 业务逻辑    | `promotion_engine` 暂未支持真实的本金/赠额拆分汇总。                                                                                                                                                       |
| **P1-009** | Resolved | 事务/SSOT   | ~~`createCombinedPaymentOrder` 跨子单操作缺乏原子性保证。~~ (**已修复**: `CreateCombinedPaymentTx` 原子事务)                                                                                               |
| **P1-010** | Resolved | 业务逻辑    | ~~`Appeal` (申诉) 接口缺失时效窗口限制~~ (**已修复**: `AppealWindowDays` 7天窗口期检查)                                                                                                                    |
| **P1-011** | Resolved | 一致性      | ~~`createOrder` 在事务外预计算余额，存在高并发下的脏读风险。~~ (**已修复**: `CreateOrderTx` 内使用 `FOR UPDATE` 锁校验)                                                                                    |
| **P1-012** | Resolved | 并发/SSOT   | ~~`renewAccessToken` 刷新令牌时未对 Session 加锁，存在竞态风险。~~ (**已修复**: `RefreshSessionTx` + `FOR UPDATE`)                                                                                         |
| **P1-013** | Low      | 性能        | RBAC 中间件多次重复查询相同实体（商户/骑手），增加 DB 压力。                                                                                                                                               |
| **P1-014** | Resolved | 安全        | ~~权限校验链路不统一，部分路由未接入全局 Casbin Enforcer。~~ (**已修复**: Applied `MerchantStaffMiddleware` to critical routes)                                                                            |
| **P1-015** | Low      | 业务逻辑    | 购物车预览阶段缺失阶梯优惠与代金券试算功能。                                                                                                                                                               |
| **P1-016** | Resolved | 并发/业务   | ~~`addCartItem` 数量上限校验与数据库写入非原子操作~~ (**已修复**: SQL `WHERE quantity + amount <= 99` 原子保护)                                                                                            |
| **P1-017** | Resolved | 安全        | ~~运费校验虽然记录了差异但主要依赖前端传入，防篡改策略相对宽松。~~ (**已修复**: Enforced server-side distance & fee calculation)                                                                           |
| **P1-018** | Medium   | 业务逻辑    | 营销引擎缺乏“互斥/叠加”规则的可视化配置，当前代码逻辑较硬。                                                                                                                                                |
| **P1-019** | Resolved | 安全/频控   | ~~`urgeOrder` (催单) 完全缺失速率限制~~ (**已修复**: 5分钟窗口最多3次限频)                                                                                                                                 |
| **P1-020** | Resolved | 事务/SSOT   | ~~`replaceOrder` 跨表操作（创建新单并标记旧单）未在同一个事务内执行~~ (**已修复**: `ReplaceOrderTx`)                                                                                                       |
| **P1-021** | Resolved | 性能/成本   | ~~骑手推荐列表实时调用 OSM 骑行路径计算且无缓存，高并发下成本/延时极高。~~ (**已修复**: Removed `enrichWithRealDistance` in favor of cached `RouteService.EnrichOrders` with ~100m precision and 30m TTL.) |
| **P1-022** | Resolved | 业务逻辑    | ~~`scanTable` 仅校验商户审核状态，未校验实时营业状态 (`is_open`)~~ (**已修复**: 增加 `IsOpen` 检查)                                                                                                        |
| **P1-023** | Resolved | 魔数/Rule 5 | ~~堂食/预订签到窗口（30分钟）为硬编码魔法数字~~ (**已修复**: 改用 `ReservationCheckInEarlyMinutes` 常量)                                                                                                   |
| **P1-024** | Resolved | 可靠性      | ~~`SendNotification` 在非 Worker 模式下同步执行 I/O，可能阻塞 API 响应。~~ (**已修复**: Wrapped fallback in detached goroutine with timeout.)                                                              |
| **P1-025** | Resolved | 逻辑泄露    | ~~`cleanupStaleDeliveries` 自动取消订单时未显式触发退款 Worker 任务。~~ (**已修复**: Added refund task trigger upon cancellation.)                                                                         |
| **P1-026** | Resolved | 维护性      | ~~购物车 (`carts` 表) 缺失定时清理机制，长期运行可能导致数据膨胀。~~ (**已修复**: `cleanupExpiredCarts` daily job removes +7d old carts)                                                                   |
| **P1-027** | Resolved | 一致性      | ~~`adjustMemberBalance` 未使用 DB 事务包裹“余额更新”与“流水记录”~~ (**已修复**: 同 P1-007)                                                                                                                 |
| **P1-028** | Resolved | 并发/死锁   | ~~`CancelOrderTx` 还原库存时未按 `dish_id` 排序~~ (**已修复**: `sort.Slice` 按 DishID 排序)                                                                                                                |
| **P1-029** | Resolved | 事务/一致性 | ~~预定取消/爽约与库存释放非原子操作~~ (**已修复**: `CancelReservationTx` 事务内释放库存)                                                                                                                   |
| **P1-030** | Resolved | 事务/SSOT   | ~~`grabOrder` 抢单逻辑分解为多次独立事务~~ (**已修复**: `GrabOrderTx` 原子事务)                                                                                                                            |
| **P1-031** | Resolved | 事务/资金   | ~~余额下单模式下，订单创建与支付确认非原子~~ (**已修复**: `CreateOrderTx` 集成扣库存逻辑，全原子操作)                                                                                                      |
| **P1-032** | Resolved | 可靠性      | ~~`cancelOrder` 分步执行状态变更与退款任务入队，入队失败会导致退款丢失。~~ (**已修复**: `RefundRecoveryScheduler` 自动扫描补偿)                                                                            |
| **P1-033** | Accepted | 并发/一致性 | Casbin 策略更新缺失多实例同步机制。(用户确认: 两年内仅单实例部署，暂不修复)                                                                                                                                |
| **P1-034** | Resolved | 可靠性/资金 | ~~`rejectOrder` 直接同步调用微信退款 API，失败无任务补偿机制，易丢单。~~ (**已修复**: `RefundRecoveryScheduler` 自动扫描补偿)                                                                              |
| **P1-035** | Resolved | 并发/一致性 | ~~`kitchen` 状态变更未像 `api/order` 一样加锁~~ (**已修复**: `UpdateOrderToPreparing`/`UpdateOrderToReady` 带状态条件)                                                                                     |
| **P1-036** | Resolved | 事务/资金   | ~~`confirmDelivery` 将送达确认、订单完结、押金解冻拆为多笔事务~~ (**已修复**: `CompleteDeliveryTx` 原子事务)                                                                                               |
| **P1-037** | Resolved | 逻辑/一致性 | ~~`submitMerchantApplication` 提交时未校验 OCR 任务是否全部完成~~ (**已修复**: 审核时检查所有 OCR 数据)                                                                                                    |
| **P1-038** | Resolved | 安全/防欺诈 | ~~自动审核系统缺失针对营业执照、身份证的历史库查重，易被恶意 P 图绕过。~~ (**已修复**: Added duplicate checks for License and ID Card)                                                                     |
| **P1-039** | Medium   | 逻辑/数据   | `CheckMerchantAddressExists` 字符串匹配逻辑太初级，易被“XX路1号”与“XX路一号”绕过。                                                                                                                         |
| **P1-040** | Resolved | 安全/CSRF   | ~~WebSocket `CheckOrigin` 在配置为空时默认为 `*`~~ (**已修复**: 使用 `isOriginAllowed()` 白名单校验)                                                                                                       |
| **P1-041** | Accepted | 架构/可用性 | WebSocketHub 缺失 Redis Pub/Sub 订阅。(用户确认: 两年内仅单实例部署，暂不修复)                                                                                                                             |
| **P1-042** | Resolved | 逻辑/健壮性 | ~~`Hub.unregisterClient` 在 platform 类型下缺失 client 实例校验~~ (**已修复**: 所有类型都有 `existing == client` 校验)                                                                                     |
| **P1-043** | Accepted | 架构/并发   | `robfig/cron` 缺失分布式锁。(用户确认: 两年内仅单实例部署，暂不修复)                                                                                                                                       |
| **P1-044** | Resolved | 逻辑/原子性 | ~~异步任务（如 OCR）缺乏最终一致性对齐，若任务执行中系统崩溃，申请可能永久卡在 pending。~~ (**已修复**: `cleanupStaleOCRTasks` recovery job)                                                               |
| **P1-045** | Accepted | 安全/合规   | `TOKEN_SYMMETRIC_KEY` 等核心秘钥缺乏自动轮转机制。(用户确认: 运维手动管理，Accept风险)                                                                                                                     |
| **P1-046** | Resolved | 逻辑/可靠性 | ~~`confirmOrder` 异步分发分账任务失败无补偿，可能导致商户无法收到资金。~~ (**已修复**: `ProfitSharingRecoveryScheduler` 自动扫描补偿)                                                                      |
| **P1-047** | Resolved | 事务/资金   | ~~余额支付在 `CreateOrderTx` 扣除但在后续失败时无回滚~~ (**已修复**: 全原子操作，无需回滚)                                                                                                                 |
| **P1-058** | Medium   | 逻辑/并发   | `tryWebSocketPush` 对每一条通知都进行角色查询，在高并发下可能导致 DB 压力。                                                                                                                                |
| **P1-059** | Resolved | 事务/资金   | ~~`CancelOrderTx` (订单取消) 缺失会员余额回滚逻辑~~ (**已修复**: 事务内原子回滚余额+创建退款流水)                                                                                                          |
| **P1-060** | Resolved | 逻辑/并发   | ~~`ProcessOrderPaymentTx` 在 inventory 扣减后未对 `PaymentOrder` 状态做最终强制校验。~~ (**已修复**: Added mandatory check for `OrderStatusPending` in `ProcessOrderPaymentTx`.)                           |
| **P1-061** | Resolved | 逻辑/同步   | ~~`getMerchantFinanceOverview` 跨日期统计未加缓存，且未对超大时间跨度做严格限制。~~ (**已修复**: Restricted date range to 90 days)                                                                         |
| **P1-062** | Resolved | 安全/身份   | ~~`bindPhone` 接口允许绑定任意手机号~~ (已移除该功能，因业务不需要且微信接口收费)。                                                                                                                        |
| **P1-063** | Resolved | 逻辑/资金   | ~~爽约时定金（Deposit）逻辑缺失，未显式处理没收/结算资金流。~~ (**已修复**: Integrated into ProfitSharing system)                                                                                          |
| **P1-064** | Resolved | 安全/风控   | ~~爽约时未记录用户风控信用分，无法防止恶意爽约。~~ (**已修复**: Automatically create behavior decision for no-show)                                                                                        |
| **P1-065** | Medium   | 业务逻辑    | 取消预订仅支持全额/无退款，不支持部分退款配置（如退50%）。                                                                                                                                                 |

---

## 方案 A：全链路代码级审计

### A0. 基础设施核心配置

- [x] `main.go` / `util/config.go` - 核心配置 ✅
  - [x] **发现 P1-001**: ~~`app.env.example` 环境变量缺失。~~ (**已修复**)
  - [x] **发现 P1-002**: ~~默认日志级别配置缺失。~~ (**已修复**: Added
        `LOG_LEVEL`)
  - [x] 数据库连接池配置 ✅
  - [x] Redis 连接配置 ✅

### A1. 用户认证与授权链路

#### A1.1 微信登录链路

- [x] `api/wechat.go` - 微信小程序登录 ✅
  - [x] code2session 调用失败时的错误处理 ✅
  - [x] ~~**已检查**: `session_key` 未存储 (Not Stored)，导致 `bindPhone`
        无法解密微信手机号 (**P1-062**)~~。**已移除 `bindPhone` 接口**。
  - [x] 首次登录用户创建的事务完整性 ✅ `CreateUserTx` + `wechat_openid`
        唯一索引
  - [x] token 生成与过期时间设置 ✅

#### A1.2 Web 登录链路

- [x] `api/web_login.go` - 商户/运营商/骑手 Web 端登录
  - [x] 密码校验机制 -> N/A (使用扫码登录，无密码)
  - [x] 登录失败次数限制 -> 扫码模式下，二维码有过期时间，且验证签名 ✅
  - [x] session 管理与过期 -> `webLoginSessionTTL` 控制，状态流转完善 ✅

#### A1.3 Token 刷新链路

- [x] `api/token.go` - 访问令牌刷新逻辑 ✅
  - [x] refresh_payload 校验 ✅
  - [x] session.is_revoked 校验 ✅
  - [x] 令牌滚动更新（New RT for Each Refresh）✅
  - [ ] **发现 P1-012**: 缺少 `FOR UPDATE` 锁定会话，存在高并发刷新冲突。

#### A1.4 权限控制链路

- [x] `api/rbac_middleware.go` - RBAC 权限系统 ✅
  - [x] 角色动态加载 ✅
  - [x] 实体（商户/骑手/运营商）关联校验 ✅
  - [ ] **发现 P1-013**: 多重中间件导致冗余的 DB 查询。
  - [x] **发现 P1-014**: ~~部分路由未覆盖全局 Casbin 规则。~~ (**已修复**: Added
        `MerchantStaffMiddleware` to merchant management APIs)
- [x] `api/casbin_enforcer.go` - Casbin 策略执行 ✅
  - [x] 策略加载失败的降级处理 ✅ 中间件检测 globalCasbinEnforcer == nil 返回
        503
  - [x] 策略热更新机制 ✅ 支持 ReloadPolicy 但缺乏多实例自动触发。
  - [x] **发现 P1-033**: 缺少分布式集群下的策略同步机制。

---

### A2. 购物车链路

#### A2.1 购物车操作

- [x] `api/cart.go` - 购物车增删改查 ✅
  - [x] `addCartItem` - 加购 ✅
    - [x] 菜品是否在线/可售校验 ✅
    - [x] 库存预检查 ⚠️ 仅校验上架状态，无库存扣减（软购物车）
    - [x] 商户是否营业校验 ✅
    - [x] 加购数量上限校验 ✅ 限制 1-99
    - [x] 跨商户购物车隔离 ✅
  - [x] `updateCartItem` - 更新数量 ✅
    - [x] 数量为0的处理 ⚠️ 需显式调用 remove
    - [x] 负数数量拒绝 ✅
  - [x] `removeCartItem` - 删除 ✅
    - [x] 幂等性保护 ✅
  - [x] `clearCart` - 清空 ✅
    - [x] 并发清空的竞态 ✅
  - [x] `getCart` - 查询 ✅
    - [x] 过期商品/下架商品的处理 ✅ 返回时重新校验 IsAvailable
    - [x] 价格变化的提示 ⚠️ 无显式提示，始终返回最新价

#### A2.2 购物车试算

- [x] `api/cart.go` - `calculateCart` ✅
  - [x] 优惠券叠加规则 ✅
  - [x] 会员折扣计算 ✅
  - [x] 满减规则 ✅
  - [x] 配送费计算 ✅
  - [x] 包装费计算 ✅
  - [x] 金额精度处理（分 vs 元）✅
  - [ ] **发现 P1-015**: 购物车预览阶段缺失阶梯优惠与代金券试算功能。
  - [ ] **发现 P1-016**: `addCartItem`
        数量上限校验与数据库写入非原子操作，可绕过 99 件限制。
  - [x] **发现 P1-017**: ~~运费校验宽松。~~ (**已修复**: Ignored client
        distance/fee, always recalculate on server)

  - [ ] **发现 P1-018**:
        营销引擎缺乏“互斥/叠加”规则的可视化配置，当前代码逻辑较硬。

---

### A3. 订单链路

#### A3.1 订单创建

- [x] `api/order.go` - `createOrder` ✅
  - [x] 订单类型校验（堂食/外卖/打包）✅
  - [x] 商户营业状态校验 ✅
  - [x] 菜品可售性校验 ✅
  - [x] 库存扣减原子性（`db/sqlc/tx_create_order.go`）✅ 使用
        `GetDailyInventoryForUpdate` 加锁
  - [x] 地址归属校验（外卖）✅
  - [x] 桌台归属校验（堂食）✅
  - [x] 金额一致性校验（前端传入 vs 后端计算）✅
  - [x] **发现 P1-011**: `createOrder` 在 Tx
        前预计算余额支付金额，存在脏读风险。
  - [x] **发现 P1-031**:
        余额下单模式下，创建订单与执行支付（扣库存）分为两个独立事务，存在资金风险。
  - [x] 订单号生成唯一性 ✅
  - [x] 超时取消任务调度 ✅ 修复：已添加 `DistributeTaskOrderPaymentTimeout`
  - [x] 并发下单的库存竞态 ✅ 按 dish_id 排序避免死锁

#### A3.2 订单状态变更（商户侧）

- [x] `api/order.go` - `acceptOrder` ✅ 使用 `GetOrderForUpdate`
  - [x] 前置状态校验（必须为 paid）✅
  - [x] 幂等性保护（重复接单）✅ 状态条件检查
  - [x] 超时通知 ✅ 接单后异步发送商家接单通知
- [x] `api/order.go` - `rejectOrder` ✅
  - [x] 前置状态校验 ✅ 仅 `paid` 允许
  - [x] 自动发起退款 ✅ 实现
  - [x] 退款失败的回滚 ⚠️ **发现 P1-034**: 直接调用 API
        失败仅记录日志，无补偿流。
- [x] `api/order.go` - `markOrderReady` ✅
  - [x] 前置状态校验 ✅ 仅 `preparing` 允许

#### A3.3 订单状态变更（厨房侧）

- [x] `api/kitchen.go` - `startPreparing` ✅
  - [x] 前置状态校验（必须为 paid）✅
  - [x] **发现 P1-035**: 缺失锁定机制，存在并发更新竞态。
- [x] `api/kitchen.go` - `markKitchenOrderReady` ✅
  - [x] 前置状态校验（paid 或 preparing）✅
  - [x] 外卖订单触发配送池 ⚠️ 逻辑发现偏差：`kitchen` 中未显式入池，依赖
        `order.go` 的 logic。
  - [x] 出餐通知触发 ✅

#### A3.4 订单确认收货

- [x] `api/order.go` - `confirmOrder`
  - [x] 仅外卖订单允许 ✅
  - [x] 前置状态校验（rider_delivered 或 user_delivered）✅
  - [x] 幂等性保护 ✅
  - [x] 分账触发 -> 无直接触发，依赖后续任务或状态变更日志触发 ✅

#### A3.5 订单取消

- [x] `api/order.go` - `cancelOrder` ✅ 使用 `GetOrderForUpdate`
  - [x] 各状态下取消规则（pending/paid/preparing）✅ 状态条件检查
  - [x] 库存恢复 ✅ 使用 `GetDailyInventoryForUpdate` 加锁
  - [x] **发现 P1-028**: `CancelOrderTx` 还原库存时循环内未进行 `dish_id`
        排序，死锁高发。
  - [x] 退款触发 ⚠️ **发现 P1-032**: 异步任务入队非原子，失败需介入。
  - [x] 配送单处理 ✅ `paid` 状态下尚未入池或已被清理。

#### A3.6 订单催单/改单

- [x] `api/order.go` - `urgeOrder` ✅
  - [ ] 催单频率限制
  - [ ] 通知推送
  - [ ] **发现 P1-019**: `urgeOrder` (催单)
        完全缺失速率限制，易被恶意刷接口污染日志。
- [x] `api/order.go` - `replaceOrder` (改单) ✅
  - [ ] 改单后金额变化处理
  - [ ] 已支付订单改单的差价处理
  - [ ] **发现 P1-020**: `replaceOrder`
        跨表操作（创建新单并标记旧单）未在同一个事务内执行。

---

### A4. 支付链路

#### A4.1 支付单创建

- [x] `api/payment_order.go` - `createPaymentOrder` ✅
  - [x] 业务单状态校验 ✅
  - [x] 幂等性保护（已有 pending 支付单）✅
  - [x] 金额一致性校验 ✅
  - [x] 超时时间设置 ✅ 30分钟
  - [x] 微信支付参数生成 ✅

#### A4.2 合单支付

- [x] `api/payment_order.go` - `createCombinedPaymentOrder` ✅
  - [x] 多商户子单拆分 ✅
  - [x] 子单金额校验 ✅
  - [x] 分账比例预设 ✅
  - [x] **发现 P1-009**: 跨子单操作缺乏全局事务原子性。

#### A4.3 支付回调处理

- [x] `api/payment_callback.go` - `handlePaymentNotify` ✅ 设计良好
  - [x] 验签正确性 ✅ `VerifyNotificationSignature`
  - [x] 幂等性保护（notification_id）✅ `CheckNotificationExists`
  - [x] 金额校验（支付金额 vs 订单金额）✅ **发现 P0-2 Risk**
  - [x] 金额不匹配的处理 ⚠️ 需人工介入，仅记录日志
  - [x] 支付单状态更新 ✅ `WHERE status = 'pending'` 条件更新
  - [x] 异步任务入队失败的处理 ⚠️ `DistributeTask` 失败仅日志
  - [x] 响应超时导致的重复回调 ✅ 幂等性检查保护

#### A4.4 支付成功后处理

- [x] `worker/task_process_payment.go` - `ProcessTaskPaymentSuccess` ✅
  - [x] 事务边界完整性（`db/sqlc/tx_payment_success.go`）✅ 单一事务处理
  - [x] 订单/预订状态推进 ✅
  - [x] 通知推送 ✅
  - [x] 任务失败时的重试策略 ✅ Asynq 自动重试
  - [x] 重试导致的重复处理 ✅ `GetPaymentOrderForUpdate` + `processed_at`
        幂等检查
  - [x] **发现 P1-060**: `ProcessOrderPaymentTx` 缺乏对 `OrderStatus`
        的最终一致性校验。 (已修复)

#### A4.5 支付超时处理

- [x] `worker/task_payment_timeout.go` ✅
  - [x] 关闭支付单 ✅ `DistributeTaskPaymentOrderTimeout`
  - [x] 业务单状态回滚 ✅ `DistributeTaskOrderPaymentTimeout` 负责
  - [x] 库存恢复 ✅ 订单取消自动恢复 (`CancelOrderTx`)
- [x] `worker/task_order_timeout.go` ✅ 使用 `GetOrderForUpdate`
  - [x] 订单取消 ✅ 状态+时间检查后取消
  - [x] 资源释放 ✅ `CancelOrderTx`

---

### A5. 退款链路

#### A5.1 退款发起

- [x] `api/payment_order.go` - 退款相关接口 ✅
  - [x] 退款条件校验 ✅
  - [x] 退款金额上限 ✅
  - [x] 部分退款支持 ✅
  - [x] 分账回退触发 ✅ 优先执行分账回退
  - [x] **发现 P1-034**: 失败仅记录日志，无补偿流。

#### A10.1 索赔追偿与核销

- [x] `api/claim_recovery.go` -
      `payMerchantClaimRecovery`/`payRiderClaimRecovery` ⚠️ **发现 P2-001**
  - [x] 支付与解封的原子性 ⚠️ **P2-001:
        状态更新、资金流水、解封操作分步执行，非原子事务**
  - [x] 权限校验 ✅
  - [x] 幂等性 ✅ 依赖状态机流转 (pending -> paid)
  - [x] 退款金额上限 ✅ (By DB Constraint)
  - [x] 部分退款支持 ⚠️ No, 全额追偿
  - [x] 分账回退触发 ⚠️ Explicitly created adjustment, not direct profit sharing
        return.

#### A5.2 退款回调

- [x] `api/payment_callback.go` - `handleRefundNotify` ✅
  - [x] 验签 ✅ `VerifyNotificationSignature`
  - [x] 幂等性 ✅ `CheckNotificationExists` + `CreateWechatNotification`
  - [x] 退款状态更新 ✅ 通过 `ProcessTaskRefundResult` 异步处理
  - [ ] 业务单状态同步

#### A5.3 分账回退

- [x] 分账回退逻辑（收付通场景）✅
  - [x] 回退金额正确性 -> 平台和运营商佣金分别回退 ✅
  - [x] 回退失败的处理 -> `ProcessTaskProfitSharingReturnResult` 异步重试 ✅
  - [x] 回退超时的处理 -> 依赖微信 API 超时和 Asynq 重试 ✅

---

### A6. 分账链路

#### A6.1 分账触发

- [x] 订单完成后分账触发 ✅ `confirmOrder` 中检测
  - [x] 触发条件正确性 ✅
  - [x] 入队失败的兜底 ⚠️ 仅日志记录，无重试（依赖人工）

#### A6.2 分账执行

- [x] `worker/task_process_payment.go` - `ProcessTaskProfitSharing` ✅
  - [x] 分账比例计算 ✅
  - [x] 各方金额正确性（商户/骑手/运营商/平台）✅
  - [x] 分账请求失败重试 ✅ Asynq 自动重试 + 状态检查

#### A6.3 分账回调

- [x] `api/payment_callback.go` - `handleProfitSharingNotify` ✅
  - [x] 验签 ✅
  - [x] 状态更新 ✅
  - [x] 失败重试 ✅ 触发 `ProcessTaskProfitSharingResult`，失败则 Re-queue
        Sharing Task

#### A6.4 分账恢复

- [x] `worker/task_process_payment.go` - `ProcessTaskProfitSharingResult` /
      `Retry` ✅
  - [x] 扫描条件正确性 ✅
  - [x] 重试逻辑 ✅ 30min 延迟重试
  - [x] 最大重试次数 ✅ Limit 5

---

### A7. 配送链路

#### A7.1 待接单池管理

- [x] `api/delivery.go` - `ListDeliveryPoolNearbyByRegion` ✅
  - [x] 区域过滤正确性 ✅
  - [x] 距离计算（Haversine）✅
  - [x] 状态转换保护 ✅
- [x] `api/delivery.go` - `getRecommendedOrders` (推荐算法集成) ✅
  - [x] 权重配置动态加载 ✅ 见 `algorithm.RecommendScorer`
  - [x] **发现 P1-021**: ~~骑行路径实时计算 N+1 无缓存，且未做 LBS 缓存。~~
        (**已修复**: `RouteService` implemented with local cache)

#### A7.2 骑手推荐

- [x] `api/delivery.go` - `getRecommendedOrders` (recommendDeliveries) ✅
  - [x] 距离计算 ✅ `algorithm.RecommendScorer`
  - [x] 区域过滤 ✅ `ListDeliveryPoolNearbyByRegion`
  - [x] 骑手在线状态 ✅
  - [x] 骑手押金余额 -> 间接通过 IsOnline 状态保证（上线需押金）✅

#### A7.3 骑手抢单

- [x] `api/delivery.go` - `grabDelivery` ✅
  - [x] 订单状态校验（paid/preparing/ready）✅
  - [x] 骑手资质校验 ✅
  - [x] 押金冻结原子性 ✅ 使用 `GetRiderForUpdate`
  - [x] **并发抢单竞态处理** -> **发现 P0-001**:
        配送池行缺少悲观锁（已在此前核查中标记）
  - [x] **发现 P1-003**: 接口完全缺失物理距离校验（Distance Check）。
  - [x] 配送单创建 ✅
  - [x] 订单状态推进 ✅

#### A7.4 配送状态推进

- [x] `api/delivery.go` - `startPickup` ✅
  - [x] 前置状态校验 ✅ 仅允许 `assigned` 状态操作
  - [x] 订单状态联动 ✅ 联动为 `preparing` (if not already)
- [x] `api/delivery.go` - `confirmPickup` ✅
  - [x] 前置状态校验 ✅ 仅允许 `picking`
  - [x] 订单状态联动 ✅ 联动为 `picked`
- [x] `api/delivery.go` - `startDelivery` ✅
  - [x] 前置状态校验 ✅ 仅允许 `picked`
  - [x] 订单状态联动 ✅ 联动为 `delivering`
- [x] `api/delivery.go` - `confirmDelivery` ✅
  - [x] 前置状态校验 ✅ 仅允许 `delivering`
  - [x] 订单状态联动（rider_delivered）✅
  - [x] **发现 P1-005**: 缺失 LBS 地理围栏（Geofencing）校验。
  - [x] 押金解冻 ✅ 调用 `UnlockDepositTx`
  - [x] **发现 P1-036**: 状态同步与解冻分属独立事务，存在原子性风险。

#### A7.5 骑手异常上报

- [x] `api/rider.go` - 延时/异常上报 ✅ (`reportDelay`/`reportException`)
  - [x] 风控规则触发 ⚠️ Log Only, 无同步规则引擎触发
  - [x] 索赔入口 ⚠️ 无自动索赔创建，仅通知

#### A7.6 配送费计算

- [x] `api/delivery_fee.go` ✅
  - [x] 基础配送费 ✅
  - [x] 距离加价 ✅
  - [x] 峰时加价 ✅ (Redis/DB Config)
  - [x] 天气加价 ✅ (Redis/DB Config)
  - [x] 会员减免 ⚠️ 仅商户满减 (`PromotionDiscount`)，未见平台/会员级减免

---

### A8. 堂食链路

#### A8.1 扫码入场

- [x] `api/scan.go` - `scanTable` 扫码获取菜单 ✅
  - [x] 商户开放状态校验 ✅
  - [x] 菜品规格树展开 ✅
  - [x] **发现 P1-022**: 忽略商户 `is_open` 标志。
  - [x] 返回信息正确性 ✅

#### A8.2 开台

- [x] `api/dining_session.go` - `openDiningSession` ⚠️ **发现 P1-001**
  - [x] 桌台占用校验 ⚠️ **P1-001: 事务外检查，无二次校验** (Resolved via Unique
        Index)
  - [x] 预订冲突处理 ✅ `findActiveReservationForTable`

- [x] 验证码校验（可选）✅ `CheckPassword` if configured
- [x] 会话创建 ✅ 事务内创建
- [x] 商户代客开台限制 ✅ 仅允许用户开台 (Except Merchant override)

#### A8.3 用餐会话 (Session)

- [x] `api/dining_session.go` - `openDiningSession` ✅
  - [x] 预订自动关联与签到 ✅
  - [x] **发现 P1-023**: 签到窗口期（30min）为硬编码魔法数字。
  - [x] 风险用户自动预警 (Risk Alert) ✅
- [x] `api/dining_session.go` - `transferDiningSessionTable` (转台) ✅
  - [x] 目标桌台锁定 (FOR UPDATE) ✅ via `TransferDiningSessionTableTx`
  - [x] 菜品/账单同步 ✅ (Session ID 不变，自然同步)

#### A8.4 结账离店

- [x] `api/dining_session.go` - `checkoutDiningSession` ✅
  - [x] 商户身份校验 ✅
  - [x] 会话归属校验 ✅
  - [x] 未结清订单处理 ⚠️API 层无显式检查，允许商户强制关台
  - [x] 桌台释放 ✅ `CloseDiningSessionTx`
  - [x] WS 通知 ✅

---

### A9. 预订链路

#### A9.1 可用性查询

- [x] `api/table.go` - `getRoomAvailability` ✅
  - [x] 时间段冲突检测 -> `AreReservationsConflictingWithConfig` 逻辑覆盖 ✅
  - [x] 容量校验 -> API 仅返回时间段，容量校验应在下单时 ✅
  - [x] 营业时间校验 -> `ListMerchantBusinessHours` 并处理特殊日期 ✅

#### A9.2 预订创建

- [x] `api/table_reservation.go` - `createReservation` ⚠️ **发现 P0-002**
  - [x] 房型校验（必须为包间）✅
  - [x] 冲突二次校验 ⚠️ **P0-002: 冲突检测在事务和锁之外，高并发会导致双重预订**
  - [x] 库存预占 -> 无，SaaS 系统不扣减库存，只占时间段 ✅
  - [x] 支付截止时间设置 ✅
  - [x] 超时取消任务调度 ✅ Asynq 延时任务
  - [x] 预点菜金额校验 ✅

#### A9.3 预订支付

- [x] 预订支付与订单支付的差异 ✅
  - [x] business_type 区分 -> `createPaymentOrder` 支持 `reservation` ✅
  - [x] 定金 vs 全款模式 -> 均支持，通过 `PaymentMode` 判断 ✅

#### A9.4 商户确认

- [x] `api/table_reservation.go` - `confirmReservation` ✅ 使用
      `GetTableReservationForUpdate`
  - [x] 前置状态校验（需已支付）✅
  - [x] 桌台占用标记 ✅ `ConfirmReservationTx` 事务内更新
  - [x] 未到店提醒任务 ✅

#### A9.5 签到

#### A9.5 签到 (Check-in)

- [x] `api/table_reservation.go` - `checkInReservation` ✅ 使用
      `GetTableReservationForUpdate`
  - [x] 签到窗口校验 ⚠️ 仅校验状态，未校验时间窗口
  - [x] 预订归属校验 ✅
  - [x] 开台联动 ⚠️ 无直接联动，仅状态变更

#### A9.6 完结

- [x] `api/table_reservation.go` - `completeReservation` ✅ 使用
      `GetTableReservationForUpdate`
  - [x] 状态校验 ✅
  - [x] 资源释放 ✅ `CompleteReservationTx` 事务内释放

#### A9.7 爽约处理

- [x] `api/table_reservation.go` - `noShowReservation` ✅
  - [x] 爽约判定标准 ✅ `MarkNoShowTx` 事务内处理
  - [x] **发现 P1-063**:
        ~~爽约时定金（Deposit）逻辑缺失，未显式处理没收/结算资金流。~~
        (**已修复**)
  - [x] **发现 P1-064**: ~~爽约时未记录用户风控信用分，无法防止恶意爽约。~~
        (**已修复**)

#### A9.8 预订取消

- [x] `api/table_reservation.go` - `cancelReservation` ✅ 使用
      `GetTableReservationForUpdate`
  - [x] 退款窗口（refund_deadline）✅
  - [x] **发现 P1-029**: 状态变更与库存释放 (`ReleaseReservationInventoryTx`)
        分属不同事务，存在库存泄露。(Resolved)
  - [x] **发现 P1-065**:
        取消预订仅支持全额/无退款，不支持部分退款配置（如退50%）。(Medium)

#### A9.9 预订超时

#### A9.9 预订超时

- [x] `worker/task_reservation_timeout.go` ✅
  - [x] 取消预订 ✅ 状态+时间双重检查
  - [x] 释放库存 ✅ `ReleaseReservationInventoryTx` 事务内释放

---

### A10. 售后与风控链路

#### A10.1 索赔提交

- [x] `api/risk_management.go` - `SubmitClaim` ✅
  - [x] 前置条件校验（订单 completed）✅
  - [x] 索赔类型校验 ✅
  - [x] 证据上传 ✅
  - [x] 规则引擎触发 ✅
  - [x] **发现 P1-004**:
        规则引擎故障时采用异步重试但主流程通过，存在拦截失效风险。(已修复:
        规则引擎故障时降级为 Manual Review)
  - [x] **并发/幂等性** -> **发现 P0-003**: `claims.order_id`
        无唯一约束，非原子操作。
  - [x] **退款可靠性** ⚠️ 异步 goroutine 执行退款，应使用 Task Queue。

#### A10.2 申诉 (Appeal)

- [x] `api/appeal.go` - `createMerchantAppeal` ✅
  - [x] 归属校验（MerchantID 检查）✅
  - [x] 状态同步 (MarkClaimRecoveryAppealed) ✅
  - [x] **发现 P1-010**: 申诉有效期窗口未实现（代码未见 CreateAt 检查）。

---

### A11. 商户管理链路

#### A11.1 商户入驻

- [x] `api/merchant_application.go` - 申请流程 (**Risk Identified**)
  - [x] OCR 识别任务异步触发 (Asynq) ✅
  - [x] **发现 P1-037**: 提交申请时未校验异步 OCR 任务状态。
  - [x] 申请编辑状态控制 ✅ 实现 `checkApplicationEditable`
  - [x] 行政区划自动匹配 ✅ 实现 `matchRegionID`
  - [x] **发现 P2-001**: `merchant_applications` 表存储法人身份证号
        (`LegalPersonIDNumber`) 为明文。
- [x] `api/merchant_application.go` - 自动审核 ✅ 实现
      `checkMerchantApplicationApproval`
  - [x] 证件有效期校验 ✅
  - [x] 经营范围关键词匹配 ✅
  - [x] 身份与法人一致性检查 ✅
  - [x] 地址重合度检查 ⚠️ **发现 P1-039**: 地址去重逻辑较弱。
  - [x] **发现 P1-038**: ~~缺失针对证件真伪、修图、多开账号的防欺诈校验。~~
        (**已修复**: Implemented duplicate license/ID checks)
- [x] `db/sqlc/tx_merchant_application.go` - 审核通过事务 ✅
      `ApproveMerchantApplicationTx` 原子化。
  - [x] 进件数据校验 ✅
  - [x] 状态回调 ✅

#### A11.2 商户信息管理

- [x] `api/merchant.go` ✅
  - [x] 营业状态切换 ✅
  - [x] 信息更新 ✅

#### A11.3 收付通进件

- [x] `api/ecommerce_applyment.go` ✅
  - [x] 进件数据校验 ✅
  - [x] 状态回调 ✅
  - [x] 敏感信息加密 ✅ (`util.EncryptSensitiveField`)

#### A11.4 商户财务

- [x] `api/merchant_finance.go` ✅
  - [x] 账单生成 ✅ (`getMerchantFinanceOverview`)
  - [x] 提现申请 ✅
  - [x] 结算调整 ✅ (`SumMerchantSettlementAdjustments`)

---

### A12. 骑手管理链路

#### A12.1 骑手入驻

- [x] `api/rider_application.go` (**Risk Identified**)
  - [x] 资料校验 ✅
  - [x] 审批流程 ✅
  - [x] **发现 P2-002**: `rider_applications` 表 `id_card_ocr` JSON
        列明文存储敏感信息。

#### A12.2 骑手状态

- [x] `api/rider.go` ✅
  - [x] 上下线 ✅
  - [x] 状态流转 ✅

#### A12.3 骑手位置事件

- [x] `api/rider_location_events.go` ✅
  - [x] 地理围栏触发 ✅
  - [x] 自动状态推进 ✅

---

### A13. 运营商管理链路

#### A13.1 运营商入驻

- [x] `api/operator_application.go` ✅
  - [x] 审批流程 ✅
  - [x] 区域分配 ✅

#### A13.2 运营商管理

- [x] `api/operator_merchant_rider.go` ✅
  - [x] 区域内商户/骑手管理 ✅ (ID Masking implemented)

#### A13.3 运营商规则

- [x] `api/operator_rules.go` ✅
  - [x] 规则配置 ✅ (Audit logs enabled)
  - [x] 规则生效范围 ✅

#### A13.4 分账配置

- [x] `api/profit_sharing_config.go` ✅
  - [x] 比例配置 ✅
  - [x] 生效时间 ✅

---

### A14. 促销与优惠链路

#### A14.1 优惠券

- [x] `api/voucher.go` ✅
  - [x] 领取限制 ✅ User Limit Check
  - [x] 使用条件 ✅ Expire/MinAmount Check
  - [x] 核销逻辑 ✅ `UseVoucherTx` Atomic
  - [x] 过期处理 ✅ Date Check

#### A14.2 满减活动

- [x] `api/discount.go` ⚠️ 简单逻辑，不做深入并发审计
  - [x] 规则配置 ✅
  - [x] 叠加规则 ✅

#### A14.3 会员折扣

- [x] `api/membership.go` ✅
  - [x] 会员等级计算 ✅
  - [x] 折扣生效 ✅
  - [x] **发现 P1-007**: `adjustMemberBalance` 余额变动与流水写入非原子事务。
  - [x] **发现 P1-008**: 赠额与本金拆分逻辑不完整。

---

### A15. 通知系统

#### A15.1 实时推送 (WebSocket)

- [x] `api/notification.go` - `handleWebSocket` ✅
  - [x] 角色校验与连接鉴权 ✅
  - [x] **发现 P1-027**: 具备 CheckOrigin 防 CSRF。
  - [x] **发现 P1-026**: 具备 Sequence 回放机制 (Replay)。

#### A15.2 通知触发与偏好

- [x] `api/notification_helper.go` - `SendNotification` ✅
  - [x] DND 免打扰时段判断 ✅
  - [x] **发现 P1-024**: ~~同步逻辑可能阻塞，依赖 Asynq 解耦。~~ (**已修复**:
        Async fallback implementation)

#### A15.3 异步通知任务

- [x] `worker/task_send_notification.go` ✅
  - [x] 微信模板消息 ✅
  - [x] 短信通知 ✅
  - [x] 失败重试 ✅

---

### A16. 异步任务链路

#### A16.1 任务分发

- [x] `worker/distributor.go` ✅
  - [x] 入队失败处理 ✅
  - [x] 任务优先级 ✅
  - [x] 任务幂等性保护 ⚠️ **发现 P1-044**: ~~部分任务（如
        OCR）直接覆盖结果，未处理中间崩溃导致的状态不一致。~~ (**已修复**:
        `cleanupStaleOCRTasks` resets `processing` > 1h to `failed`)
  - [x] 异步结果通知 ✅ 通过 `pubSubPublisher`
- [x] `scheduler/manager.go` - 调度管理 ✅
  - [x] 定时任务生命周期管理 ✅
  - [x] `scheduler/order_timeout.go` - 订单超时 ✅
  - [x] `scheduler/takeout_auto_complete.go` - 自动完成 ✅
  - [x] **发现 P1-043**: 缺失分布式任务锁，不支持多机部署下的主备/分片竞争。

#### A16.2 任务处理

- [x] `worker/processor.go` - 处理器注册 ✅
- [x] `worker/task_risk_management.go` ✅
- [x] `worker/task_send_notification.go` ✅
- [x] `worker/task_merchant_application_ocr.go` ✅

---

---

### A17. 定时任务 (Scheduler)

#### A17.1 调度管理

- [x] `scheduler/manager.go` ✅
  - [x] 任务注册 ✅ (`Register`)
  - [x] 并发控制 ✅ (`StartAll` with `errgroup`)
  - [x] 错误处理 ✅ (Logs errors but continues)

#### A17.2 数据清理与业务调度

- [x] `scheduler/data_cleanup.go` - 清理策略 ✅
  - [x] **发现 P1-025**: ~~配送超时清理缺失退款联动。~~ (**已修复**: Refund task
        integration)
  - [x] **发现 P1-026**: ~~购物车未纳入清理计划。~~ (**已修复**: Added
        `cleanupExpiredCarts` daily job)
- [x] `scheduler/order_timeout.go` - 订单超时扫描 ✅
- [x] `scheduler/takeout_auto_complete.go` - 外卖自动完成 ✅
- [x] `worker/claim_recovery_scheduler.go` - 索赔恢复 ✅
  - [x] **发现 P2-001 (Variant)**: `MarkClaimRecoveryOverdue` 与
        `SuspendMerchant` 非原子操作，失败可能导致"已逾期但未处罚"的数据不一致。
- [x] `worker/profit_sharing_recovery_scheduler.go` - 分账恢复 ✅
- [x] `worker/payment_recovery_scheduler.go` - 支付恢复 ✅

---

### A18. 全局事务审计 (Global Transaction)

#### A18.1 原子性校验与关键事务

- [x] `db/sqlc/store.go` - 事务定义层 ✅
  - [x] 核心链路 `Tx` 方法覆盖 ✅
  - [x] **发现 P1-007**: 部分 API (如 `adjustMemberBalance`) 未调用 `Tx` 方法。
  - [x] **发现 P1-020**: `replaceOrder` 需收拢至 `Store` 事务。
- [x] `db/sqlc/tx_create_order.go` - 创建订单事务 ✅
  - [x] 库存扣减 ✅
  - [x] 优惠券核销 ✅
  - [x] 订单创建 ✅
  - [x] 失败回滚完整性 ✅
- [x] `db/sqlc/tx_payment_success.go` - 支付成功事务 ✅
- [x] `db/sqlc/tx_delivery.go` - 配送事务 ✅
  - [x] **发现 P1-030**: `GrabOrderTx` 缺失订单状态同步，非 100% 原子。
- [x] `db/sqlc/tx_reservation.go` - 预订事务 ✅
  - [x] 库存预占 ✅ `ReserveInventory`
  - [x] 冲突校验 ✅ `AfterLock` callback
  - [x] 库存同步 ✅ `ListReservationDishSummary` 排序有序，安全。
- [x] `db/sqlc/tx_claim_refund.go` - 索赔退款事务 ✅
- [x] `db/sqlc/tx_dining_session.go` - 就餐会话事务 ✅
- [x] `db/sqlc/tx_voucher.go` - 优惠券事务 ✅
- [x] `db/sqlc/tx_membership.go` - 会员事务 ✅

---

## 方案 B：状态机形式化验证

### B1. 订单状态机验证

- [ ] 提取 `api/order.go` 中所有状态转换代码
- [ ] 绘制实际状态机图
- [ ] 对比文档设计，发现差异
- [ ] 检查是否存在不可达状态
- [ ] 检查是否存在死锁状态

### B2. 配送状态机验证

- [ ] 提取 `api/delivery.go` 中所有状态转换
- [ ] 验证与订单状态联动正确性
- [ ] 检查完整闭环

### B3. 预订状态机验证

- [ ] 提取 `api/table_reservation.go` 中所有状态转换
- [ ] 验证各终态可达性
- [ ] 检查异常分支闭合

### B4. 支付状态机验证

- [ ] 提取 `api/payment_order.go` 和 `api/payment_callback.go` 中所有状态转换
- [ ] 验证回调与业务单联动

### B5. 就餐会话状态机验证

- [ ] 提取 `api/dining_session.go` 中所有状态转换
- [ ] 验证桌台占用/释放完整性

### B6. 申诉状态机验证

- [ ] 提取 `api/appeal.go` 中所有状态转换
- [ ] 验证审核流程闭环

### B7. 索赔/追偿状态机验证

- [ ] 提取 `api/risk_management.go` 和 `api/claim_recovery.go` 中所有状态转换
- [ ] 验证追偿闭环

---

## 方案 C：边界条件测试用例清单

### C1. 并发场景

#### C1.1 抢单并发

- [ ] 两个骑手同时抢同一订单
- [ ] 抢单同时订单被取消

#### C1.2 库存并发

- [ ] 多用户同时下单抢同一商品最后库存
- [ ] 下单同时商户修改库存

#### C1.3 支付并发

- [ ] 支付回调与用户取消并发
- [ ] 同一订单重复回调

#### C1.4 预订并发

- [ ] 同一时段多用户同时预订

#### C1.5 会话并发

- [ ] 同一桌台多用户同时开台

### C2. 时间边界

#### C2.1 支付超时边界

- [ ] 刚好在超时点支付成功
- [ ] 超时任务执行时支付回调到达

#### C2.2 预订时间边界

- [ ] 刚好在签到窗口边界
- [ ] 刚好在退款截止边界

#### C2.3 自动完成边界

- [ ] 刚好满足 1 小时条件
- [ ] 有索赔的不自动完成

### C3. 金额边界

#### C3.1 零元场景

- [ ] 0 元订单（全额优惠）
- [ ] 0 元配送费

#### C3.2 精度场景

- [ ] 分账比例导致的精度问题
- [ ] 多商户合单的金额拆分

#### C3.3 退款边界

- [ ] 全额退款
- [ ] 超额退款拒绝
- [ ] 部分退款后再次退款

### C4. 数量边界

#### C4.1 购物车

- [ ] 单品数量上限
- [ ] 购物车商品种类上限

#### C4.2 订单

- [ ] 单订单金额上限
- [ ] 单订单商品数量上限

### C5. 跨主线场景

#### C5.1 堂食 + 预订

- [ ] 预订用户扫码开台
- [ ] 非预订用户在预订时段开台

#### C5.2 外卖 + 堂食

- [ ] 同一用户同时下外卖和堂食订单

#### C5.3 多订单场景

- [ ] 一个会话多笔订单
- [ ] 部分订单完成部分未完成

### C6. 异常场景

#### C6.1 外部服务故障

- [ ] 微信支付接口超时
- [ ] 地图服务不可用
- [ ] Redis 不可用

#### C6.2 回调丢失

- [ ] 支付成功但回调未到
- [ ] 退款成功但回调未到

#### C6.3 任务失败

- [ ] 异步任务持续失败
- [ ] 队列积压

---

## 审计发现汇总

> 执行审计过程中发现的问题记录在此处。

### 高危问题（P0）

| 编号   | 链路           | 问题描述                                                                                                       | 代码位置                   | 修复建议                                                                                |
| ------ | -------------- | -------------------------------------------------------------------------------------------------------------- | -------------------------- | --------------------------------------------------------------------------------------- |
| P0-001 | A7.3 骑手抢单  | **并发抢单竞态条件**：`GrabOrderTx` 中 `RemoveFromDeliveryPool` 无行锁，并发请求可能导致超卖或重复派单         | `db/sqlc/tx_delivery.go`   | ✅ **已修复**: 添加 `GetDeliveryPoolByOrderIDForUpdate` 锁定及 404 检查                 |
| P0-002 | A9.2 预订创建  | **预订时段并发双重预订**：冲突检测在事务外，`CreateReservationTx` 事务内无再次检查。                           | `api/table_reservation.go` | ✅ **已修复**: `CreateReservationTx` 增加 `GetTableForUpdate` 锁及 `AfterLock` 回调重查 |
| P0-003 | A10.1 索赔提交 | **索赔重复提交**：`SubmitClaim` 检查与写入均无锁，且 `claims` 表 `order_id` 无唯一约束，并发请求可导致一单多赔 | `db/migration/000126...`   | ✅ **已修复**: 添加 `claims(order_id)` 唯一约束                                         |

### 中危问题（P1）

| 编号   | 链路          | 问题描述                                                       | 代码位置                | 修复建议                                                                        |
| ------ | ------------- | -------------------------------------------------------------- | ----------------------- | ------------------------------------------------------------------------------- |
| P1-001 | A8.2 堂食开台 | **同桌多开台并发问题**：`openDiningSession` 在事务外检查状态。 | `api/dining_session.go` | ✅ **已修复**: 添加部分唯一索引 `dining_sessions(table_id) WHERE status='open'` |

### 低危问题（P2）

| 编号   | 链路           | 问题描述                                                                      | 代码位置                      | 修复建议                                                                |
| ------ | -------------- | ----------------------------------------------------------------------------- | ----------------------------- | ----------------------------------------------------------------------- |
| P2-001 | A10.1 索赔退款 | **退款可靠性不足**：使用 `go func` 异步执行退款事务，进程崩溃会导致退款丢失。 | `api/risk_management.go`      | ✅ **已修复**: 使用 `Asynq` 任务队列 (`TaskClaimRefund`) 替代 goroutine |
| P2-002 | A11.1 商户入驻 | **敏感信息明文存储**：法人身份证号在 merchant_applications 表中未加密存储。   | `api/merchant_application.go` | **建议**: 使用 AES 加密存储或仅存储脱敏/哈希值（如业务允许）            |
| P2-003 | A12.1 骑手入驻 | **敏感信息明文存储**：OCR 结果 JSON 直接存入数据库，包含完整身份证信息。      | `api/rider_application.go`    | **建议**: 对敏感字段进行加密处理或仅保存必要脱敏信息                    |

### 设计优化建议

| 编号    | 链路          | 建议描述                                                                                                               |
| ------- | ------------- | ---------------------------------------------------------------------------------------------------------------------- |
| OPT-001 | A3.1 订单创建 | ✅ **库存扣减已正确实现**：`ProcessOrderPaymentTx` 使用 `GetDailyInventoryForUpdate` 加锁，且按 `dish_id` 排序避免死锁 |
| OPT-002 | A4.3 支付回调 | ✅ **幂等性处理良好**：使用 notification_id 记录 + 状态条件更新 `WHERE status = 'pending'`                             |

### 🔵 优化建议 (P2 - Low)

1. **P2-001 (A10.1 索赔追偿)**: 已通过 Task Queue 修复。
2. **P2-002 (A11.1 商户入驻)**: 建议对身份证号等敏感字段加密存储 (P1-006合规)。
3. **P2-003 (A12.1 骑手入驻)**: 建议对 OCR JSON 中的敏感信息加密或脱敏。
