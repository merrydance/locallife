# 后端审查问题核实与修复计划

- 日期：2026-01-16
- 范围：locallife 后端 Go 服务（Gin API、sqlc Store/Tx、worker、外部集成等）
- 目标：核实审查结论的真实性，并形成分类修复方案与可回归的测试清单

---

## 1. 工作流总览（核实 → 分类 → 修复）

### 阶段 A：核实清单（先验证事实）
对每条风险建立“问题账本”，字段包括：
- 模块/接口
- 严重级
- 代码入口（路由 → handler → store/tx）
- SQL/迁移约束
- 复现步骤/最小测试
- 结论（真实/误报/需进一步验证）

核实方法：
- 路径验证（从路由到 DB 的调用链）
- 约束验证（迁移/索引/检查约束）
- 行为验证（最小化单测/请求模拟）

判定标准：
- 可复现 + 代码路径确证 + 约束/实现一致或缺失 → 真实问题
- 仅文档/注释层面不影响行为 → 契约漂移类问题
- 现有防线可证明不成立 → 误报或已修复

### 阶段 B：问题分类（结构化归档）
按“风险面 + 修复类型”划分：
1) 资金与幂等
2) 权限/越权
3) 状态机/一致性
4) 上传/资源访问安全
5) 错误映射与契约
6) 可观测与日志合规

### 阶段 C：修复方案（按优先级）
- P0：资金幂等/状态机漂移/上传高风险/默认地址原子性
- P1：ErrNoRows 统一/权限收敛/日志脱敏/日期口径统一
- P2：契约一致性/性能与测试覆盖

输出物：
- 核实结果表（问题账本）
- 分类修复清单（按模块拆分任务）
- 回归测试清单（覆盖幂等/越权/回滚/上传安全）

---

## 2. 立即执行的核实顺序（从 P0 开始）

- [x] 资金入账幂等（支付回调/worker）
- [x] 商户申请状态机漂移 + reset 降级风险
- [x] 高风险图片读取/上传链路（LFI/SSRF/DoS）
- [x] uploads 匿名直出面（评价图片）
- [x] 默认地址原子性缺口
- [x] 骑手申请通过但未创建 rider 的原子性缺口
- [x] 会员充值 attach JSON 生成错误
- [x] Casbin fail-open 风险
- [x] Billing Group 越权读取/加入
- [x] dining session open 缺少桌台校验
- [x] 库存 ErrNoRows → unlimited 的语义冲突
- [x] 推荐配置接口权限缺失
- [x] 优惠券核销回滚缺失

---

## 3. 下一步行动（开始核实）

- 建立问题账本：按以上顺序逐条核实。
- 每条核实产出：
  - 入口函数与路由
  - 关键代码片段位置（文件/函数）
  - 迁移/约束证据
  - 复现/测试建议
  - 结论（真实/误报/待证）

完成每条后即归类，并同步形成修复任务拆解与回归测试列表。

---

## 4. P0 核实记录（已完成）

> 记录格式：结论 + 证据路径（文件/行）。

1) 资金入账幂等（支付回调/worker）— 结论：真实
- 支付成功任务仅判断 `payment_order.status == paid`，无幂等门闩，重复任务仍会入账：[locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go#L305-L416)
- 押金流水表无与支付单绑定的唯一约束：[locallife/db/migration/000012_add_riders_and_deliveries.up.sql](locallife/db/migration/000012_add_riders_and_deliveries.up.sql#L47-L64)

2) 商户申请状态机漂移 + reset 降级风险 — 结论：真实
- 非 draft 申请可编辑且自动 reset：[locallife/api/merchant_application.go](locallife/api/merchant_application.go#L135-L152)
- reset 时强制把商户状态改为 pending：[locallife/db/sqlc/tx_merchant_application.go](locallife/db/sqlc/tx_merchant_application.go#L144-L170)
- approve 时强制把商户状态改为 approved（可能降级 active）：[locallife/db/sqlc/tx_merchant_application.go](locallife/db/sqlc/tx_merchant_application.go#L72-L89)

3) 高风险图片读取/上传链路（LFI/SSRF/DoS）— 结论：真实
- 外部 URL 直接 `http.Get`，无超时/白名单/IP 限制：[locallife/api/ecommerce_applyment.go](locallife/api/ecommerce_applyment.go#L500-L511)
- `io.ReadAll` 直接读入内存，缺少流式上限：[locallife/api/ecommerce_applyment.go](locallife/api/ecommerce_applyment.go#L511-L533)
- 本地路径 `os.Open` 未做 `..` 清理：[locallife/api/ecommerce_applyment.go](locallife/api/ecommerce_applyment.go#L516-L525)、[locallife/api/public_url.go](locallife/api/public_url.go#L42-L60)

4) uploads 匿名直出面（评价图片）— 结论：真实
- `/uploads/*filepath` 为公共路由：[locallife/api/server.go](locallife/api/server.go#L216-L220)
- `uploads/reviews/` 被标记为 public 直出：[locallife/api/upload_signed.go](locallife/api/upload_signed.go#L134-L153)

5) 默认地址原子性缺口 — 结论：真实
- 创建/设置默认地址采用“两次写入”，无事务：[locallife/api/user_address.go](locallife/api/user_address.go#L164-L189)、[locallife/api/user_address.go](locallife/api/user_address.go#L457-L474)
- DB 无唯一约束兜底：[locallife/db/migration/000003_add_regions_and_addresses.up.sql](locallife/db/migration/000003_add_regions_and_addresses.up.sql#L16-L41)

6) 骑手申请通过但未创建 rider 的原子性缺口 — 结论：真实
- approve 后再创建 rider，失败仅记录日志不回滚：[locallife/api/rider_application.go](locallife/api/rider_application.go#L579-L605)
- `createRiderFromApplication` 失败会留下“已通过无档案”：[locallife/api/rider_application.go](locallife/api/rider_application.go#L727-L756)

7) 会员充值 attach JSON 生成错误 — 结论：真实
- `fmt.Sprintf` 生成 JSON，nil 会产生 `<nil>` 非法 JSON：[locallife/api/membership.go](locallife/api/membership.go#L636-L642)
- worker 端 `json.Unmarshal` 失败即 `SkipRetry`：[locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go#L347-L370)

8) Casbin fail-open 风险 — 结论：真实
- enforcer nil 时直接放行：[locallife/api/casbin_enforcer.go](locallife/api/casbin_enforcer.go#L200-L203)、[locallife/api/casbin_enforcer.go](locallife/api/casbin_enforcer.go#L271-L279)
- 初始化失败仅 warn 不阻断启动：[locallife/api/server.go](locallife/api/server.go#L153-L157)

9) Billing Group 越权读取/加入 — 结论：真实
- list/join/orders 均无成员校验：[locallife/api/billing_group.go](locallife/api/billing_group.go#L134-L230)
- SQL 已提供 `GetActiveBillingGroupMember` 但未使用：[locallife/db/query/billing_group.sql](locallife/db/query/billing_group.sql#L39-L45)

10) dining session open 缺少桌台校验 — 结论：真实
- open 仅校验 table/reservation，不校验桌台口令/参与者证明：[locallife/api/dining_session.go](locallife/api/dining_session.go#L215-L334)

11) 库存 ErrNoRows → unlimited 语义冲突 — 结论：真实
- ErrNoRows 被当成无限库存返回：[locallife/api/inventory.go](locallife/api/inventory.go#L379-L394)
- SQL 语义为 `total_quantity=-1` 才是无限：[locallife/db/query/inventory.sql](locallife/db/query/inventory.sql#L1-L22)

12) 推荐配置接口权限缺失 — 结论：未发现问题（疑似已修复）
- 路由已加 operator 角色 + 区域校验：[locallife/api/server.go](locallife/api/server.go#L1017-L1024)
- `ValidateOperatorRegionMiddleware` 做区域归属校验：[locallife/api/casbin_enforcer.go](locallife/api/casbin_enforcer.go#L468-L516)

13) 优惠券核销回滚缺失 — 结论：真实
- 订单创建时即核销优惠券：[locallife/db/sqlc/tx_create_order.go](locallife/db/sqlc/tx_create_order.go#L124-L139)
- 取消订单未回滚优惠券状态/计数：[locallife/db/sqlc/tx_order_status.go](locallife/db/sqlc/tx_order_status.go#L126-L219)

---

## 5. P1 核实清单与记录（进行中）

- [x] ErrNoRows 类型不统一导致错误码漂移
- [x] 权限边界偏宽（财务/统计/PII）
- [x] 上传内容校验不一致（扩展名 vs magic number）
- [x] 证照访问策略与“敏感图片”目标不一致
- [x] 统计日期口径少算一天
- [x] OCR/请求日志可能泄露敏感信息
- [x] 地图编码体系不匹配导致 region 精确匹配失效
- [x] access token 可能落日志（WebSocket query token + RawQuery 记录）
- [x] 权限矩阵多来源导致漂移
- [x] WebSocket CheckOrigin 恒 true
- [x] worker→Redis→API server 推送协议不一致
- [x] webhooks 未限制 body 大小且存在明文落日志点
- [x] 标签创建接口未实现管理员权限
- [x] public 搜索/扫码返回面偏大
- [x] public 搜索接口触发外部地图调用（成本/DoS 风险）
- [x] 折扣规则 list/get 权限与归属校验不足
- [x] 收藏接口重复写入幂等语义缺口

### 5.1 ErrNoRows 类型不统一导致错误码漂移 — 结论：真实
- 业务 handler 使用 `sql.ErrNoRows` 判定 404，但 store 基于 pgx（ErrNoRows 类型不同）：[locallife/api/region.go](locallife/api/region.go#L66-L82)
- db 统一 ErrNoRows 常量为 `pgx.ErrNoRows`，与 handler 不一致：[locallife/db/sqlc/error.go](locallife/db/sqlc/error.go#L15-L19)
- 修复：新增统一判定 `isNotFoundError` 并替换 handler 的 `sql.ErrNoRows` 判断：[locallife/api/not_found.go](locallife/api/not_found.go#L1-L14)

### 5.2 权限边界偏宽（财务/统计/PII）— 结论：真实
- 财务接口仅使用 `authGroup`，未加 owner/manager 限制：[locallife/api/server.go](locallife/api/server.go#L780-L789)
- 财务接口通过 `GetMerchantByOwner`，该查询允许 active staff 命中：[locallife/api/merchant_finance.go](locallife/api/merchant_finance.go#L87-L106)、[locallife/db/query/merchant.sql](locallife/db/query/merchant.sql#L85-L92)

### 5.3 上传内容校验不一致（扩展名 vs magic number）— 结论：真实
- OCR 上传使用 magic number 校验：[locallife/util/upload.go](locallife/util/upload.go#L144-L188)
- 非 OCR 上传仅做扩展名白名单校验（且使用 `strings.Contains`）：[locallife/util/upload.go](locallife/util/upload.go#L200-L247)、[locallife/util/upload.go](locallife/util/upload.go#L288-L345)

### 5.4 证照访问策略与“敏感图片”目标不一致 — 结论：真实
- 证照路径被允许任意已登录用户签名访问（business_license/food_permit）：[locallife/api/upload_signed.go](locallife/api/upload_signed.go#L189-L207)

### 5.5 统计日期口径少算一天 — 结论：真实
- 仅解析 `end_date` 为 00:00:00，未统一到当日结束，SQL 使用 `created_at <= end_date` 会漏掉当日大部分数据：[locallife/api/merchant_stats.go](locallife/api/merchant_stats.go#L53-L91)、[locallife/db/query/merchant_stats.sql](locallife/db/query/merchant_stats.sql#L6-L17)

### 5.6 OCR/请求日志可能泄露敏感信息 — 结论：真实（请求日志）
- 基础信息更新直接把请求体打印到日志，包含手机号、地址、经纬度等敏感字段：[locallife/api/merchant_application.go](locallife/api/merchant_application.go#L309-L346)

### 5.7 地图编码体系不匹配导致 region 精确匹配失效 — 结论：真实
- 匹配逻辑直接使用地图服务返回的 `adcode` 去查 `regions.code`：[locallife/api/location.go](locallife/api/location.go#L187-L199)
- OSM/Nominatim 逆地理编码将 `Adcode` 赋值为 `Postcode`（邮编），与 `regions.code` 的“行政区划代码”口径不一致：[locallife/maps/osm.go](locallife/maps/osm.go#L205-L230)、[locallife/db/migration/000003_add_regions_and_addresses.up.sql](locallife/db/migration/000003_add_regions_and_addresses.up.sql#L36-L45)

### 5.8 access token 可能落日志（WebSocket query token + RawQuery 记录）— 结论：真实
- WebSocket 场景在未携带 Authorization 时从 query `token` 取 access token：[locallife/api/middleware.go](locallife/api/middleware.go#L21-L36)
- 请求日志中间件无脱敏记录完整 `RawQuery`，可能把 `token` 直接写入日志：[locallife/api/middleware_tracing.go](locallife/api/middleware_tracing.go#L41-L79)

### 5.9 权限矩阵多来源导致漂移 — 结论：真实
- 路由层同时存在基于数据库角色的 `RoleMiddleware` 与 Casbin 策略的 `CasbinRoleMiddleware`，并在不同路由组混用：[locallife/api/rbac_middleware.go](locallife/api/rbac_middleware.go#L33-L79)、[locallife/api/casbin_enforcer.go](locallife/api/casbin_enforcer.go#L271-L330)、[locallife/api/server.go](locallife/api/server.go#L738-L812)
- Casbin 初始化失败会回退到“非 Casbin”路径，权限来源不唯一，容易产生矩阵漂移：[locallife/api/server.go](locallife/api/server.go#L153-L157)

### 5.10 WebSocket CheckOrigin 恒 true — 结论：真实
- WebSocket 升级器 `CheckOrigin` 始终返回 true，未进行来源校验：[locallife/api/notification.go](locallife/api/notification.go#L452-L468)

### 5.11 worker→Redis→API server 推送协议不一致 — 结论：真实
- Worker 通过 Redis 发布的消息为 `{type,data,timestamp}` 结构：[locallife/worker/task_send_notification.go](locallife/worker/task_send_notification.go#L134-L178)
- API server 的 Pub/Sub 订阅端期待 `NotificationPushMessage{entity_type,entity_id,message}`，不匹配会直接解包失败：[locallife/websocket/pubsub.go](locallife/websocket/pubsub.go#L28-L109)

### 5.12 webhooks 未限制 body 大小且存在明文落日志点 — 结论：真实
- 回调处理使用 `io.ReadAll` 直接读取请求体，无大小限制：[locallife/api/payment_callback.go](locallife/api/payment_callback.go#L43-L52)
- 解析失败时把完整 `body` 打到日志，包含支付回调明文内容：[locallife/api/payment_callback.go](locallife/api/payment_callback.go#L72-L76)

### 5.13 标签创建接口未实现管理员权限 — 结论：真实
- 路由仅在 `authGroup` 下注册，未绑定管理员/运营权限中间件：[locallife/api/server.go](locallife/api/server.go#L439-L444)
- `createTag` 仅校验已登录，注释也提示“后续可添加管理员权限检查”，实际未做权限判断：[locallife/api/tag.go](locallife/api/tag.go#L73-L110)

### 5.14 public 搜索/扫码返回面偏大 — 结论：真实
- 搜索与扫码为公开路由（无需认证）：[locallife/api/server.go](locallife/api/server.go#L299-L312)
- 搜索商户返回包含地址、经纬度、电话等敏感字段：[locallife/api/search.go](locallife/api/search.go#L63-L76)
- 扫码接口返回商户电话与地址等敏感信息：[locallife/api/scan.go](locallife/api/scan.go#L28-L36)
- 备注：小程序实际默认携带 token（静默登录），但服务端路由仍允许匿名访问，风险取决于网关与客户端约束。

### 5.15 public 搜索接口触发外部地图调用（成本/DoS 风险）— 结论：真实
- 公开搜索接口在带经纬度时触发距离/运费计算：[locallife/api/search.go](locallife/api/search.go#L237-L279)
- 距离计算调用 `mapClient.GetDistanceMatrix`（外部地图服务），可被公开请求放大成本：[locallife/api/search.go](locallife/api/search.go#L803-L826)
- 备注：若所有客户端均携带 token，可通过网关与鉴权频控降低风险。

### 5.16 折扣规则 list/get 权限与归属校验不足 — 结论：真实
- 路由仅挂在 `authGroup`，未绑定商户归属中间件：[locallife/api/server.go](locallife/api/server.go#L1092-L1105)
- `getDiscountRule`/`listMerchantDiscountRules` 仅按传入 `merchant_id`/`id` 查询，未校验当前用户是否属于该商户：[locallife/api/discount.go](locallife/api/discount.go#L101-L162)

### 5.17 收藏接口重复写入幂等语义缺口 — 结论：真实
- 收藏写入采用 `ON CONFLICT DO NOTHING RETURNING *`，重复请求会导致“无返回行”：[locallife/db/query/favorite.sql](locallife/db/query/favorite.sql#L1-L13)
- API 未对该场景做幂等处理，重复收藏会进入 `500` 分支：[locallife/api/favorite.go](locallife/api/favorite.go#L66-L92)

---

## 6. P1 修复建议（简版）

- 5.1 统一 ErrNoRows：统一使用 db.ErrRecordNotFound/pgx.ErrNoRows；封装 `notFound()` 判定。
- 5.2 收敛财务权限：对财务/统计接口统一加 `MerchantStaffMiddleware("owner","manager")` 或 casbin 规则，并在 handler 内核验归属。
- 5.3 上传校验统一：所有上传统一 magic number + content-type + size 上限；移除 `strings.Contains` 的扩展名判断。
- 5.4 证照访问：签名 URL 绑定资源归属（owner/operator），并限制公开角色。
- 5.5 统计日期：`end_date` 归一到当日 23:59:59 或改为 `< end_date+1day`。
- 5.6 日志脱敏：移除 `req_payload`/body 明文；仅记录字段白名单或 hash。
- 5.7 地图编码：OSM `postcode` 不用于行政区匹配；引入 adcode 映射或仅使用最近区域兜底。
- 5.8 token 日志：WebSocket 使用 header/短期 ticket，且日志中剥离 `query` 或脱敏。
- 5.9 权限矩阵：统一到单一 RBAC（Casbin 或 RoleMiddleware），移除 fallback 分支。
- 5.10 CheckOrigin：允许来源白名单校验。
- 5.11 推送协议：统一 Redis payload 为 `NotificationPushMessage` 或兼容旧结构。
- 5.12 Webhooks：限制 body 大小，禁止记录 `body` 明文（改为长度/hash）。
- 5.13 标签创建：路由增加管理员/运营商权限中间件。
- 5.14 公共返回面：公开搜索/扫码移除电话、地址、精确经纬度；必要时下沉到鉴权接口。
- 5.15 地图调用：公开搜索改为异步/缓存或加限流，避免外部调用放大。
- 5.16 折扣规则：list/get 增加商户归属校验或绑定 merchant staff 中间件。
- 5.17 收藏幂等：重复收藏返回 200 并复用已有记录（`ON CONFLICT DO UPDATE/RETURNING` 或先查后插）。

---

## 7. P2 核实清单与记录（进行中）

- [x] public/需登录契约漂移
- [x] regions 后备接口契约漂移（鉴权/多区域模型）
- [x] BI 字段语义错配（order_count vs dish_count）
- [x] 测试覆盖缺口（location 基础用例）

### 7.1 public/需登录契约漂移 — 结论：真实
- 路由命名为 `/public/*` 但挂在 `authGroup`（需登录），与“public”语义不一致：[locallife/api/server.go](locallife/api/server.go#L314-L325)
- 备注：小程序静默登录默认携带 token，命名歧义仍可能导致文档/权限矩阵误导。

### 7.2 regions 后备接口契约漂移（鉴权/多区域模型）— 结论：真实
- Swagger 标注 `BearerAuth`，但路由实际为 public（无需认证）：[locallife/api/region.go](locallife/api/region.go#L320-L328)、[locallife/api/server.go](locallife/api/server.go#L292-L297)
- 区域占用判断仍使用 `operators.region_id` 的单区域查询：[locallife/api/region.go](locallife/api/region.go#L354-L369)、[locallife/db/query/operator.sql](locallife/db/query/operator.sql#L24-L31)
- 多区域模型已引入 `operator_regions` 与 `GetActiveOperatorByRegion`，与现行判断口径不一致：[locallife/db/query/operator_region.sql](locallife/db/query/operator_region.sql#L55-L60)

### 7.3 BI 字段语义错配（order_count vs dish_count）— 结论：真实
- 响应字段 `order_count` 实际填充 `dish_count`：[locallife/api/merchant_stats.go](locallife/api/merchant_stats.go#L880-L888)
- SQL 的 `dish_count` 为 `COUNT(DISTINCT oi.dish_id)`（菜品数而非订单数）：[locallife/db/query/merchant_stats.sql](locallife/db/query/merchant_stats.sql#L194-L200)

### 7.4 测试覆盖缺口（location 基础用例）— 结论：真实
- 文档核对指出 `api/location.go` 未见直接单测/集成测，并建议补齐基础用例与 `matchRegionID` 线路测试：[locallife/docs/backend_review_tasks.md](locallife/docs/backend_review_tasks.md#L228-L234)

---

## 8. 修复任务拆分（按优先级）

### 8.1 P0 修复任务
- T0.1 支付入账幂等门闩：`worker/task_process_payment.go` + DB 唯一约束（押金/入账）+ 幂等测试。
- T0.2 商户申请状态机保护：`tx_merchant_application.go` 防降级；`merchant_application.go` reset 条件收敛。
- T0.3 上传链路防护：`ecommerce_applyment.go`/`public_url.go` 增加 URL 白名单/IP 拦截/超时/大小上限。
- T0.4 uploads 公共直出收敛：`upload_signed.go` + 路由权限调整。
- T0.5 默认地址原子性：新增事务或局部唯一索引。
- T0.6 rider/merchant 关键写入原子性：`rider_application.go`、`tx_create_order.go`/`tx_order_status.go`。
- T0.7 Casbin fail-open：初始化失败直接拒绝或启动失败。
- T0.8 用餐会话 open 参与者证明：桌台口令/参与者校验（防止越权开桌）。
- T0.9 申诉幂等与审核可靠性：幂等粒度统一、重复提交防重、审核后状态变更的可靠投递。
- T0.10 认证/会话治理：refresh token 轮换、会话清理、token 哈希存储。

### 8.2 P1 修复任务
- T1.1 统一 NotFound 映射：`db/sqlc/error.go` + handler 判定修正 + 测试 stub 修正。
- T1.2 权限矩阵收敛：统一 Casbin 或 RoleMiddleware；移除 fallback。
- T1.3 日志脱敏与请求体上限：`middleware_tracing.go` + `payment_callback.go`。
- T1.4 地图编码一致：OSM `Adcode` 取值修正或停止使用；region 匹配策略调整。
- T1.5 WebSocket 安全：`CheckOrigin` 白名单；WebSocket token 仅 header。
- T1.6 Redis 推送协议统一：worker 与 api 使用同一 schema。
- T1.7 标签创建加管理员权限：`tag.go` + 路由中间件。
- T1.8 public 搜索/扫码脱敏与限流：`search.go`/`scan.go`。
- T1.9 折扣规则归属校验：`discount.go` + 路由加 merchant staff 校验。
- T1.10 收藏幂等：`favorite.sql` + handler 对冲突返回 200。
- T1.11 上传归属规则统一：上传/签名下载按资源归属与角色校验一致化。
- T1.12 支付单 business_type 维度：明确定义并统一使用，防止错配导致越权或错账。
- T1.13 会员链路治理：支付方式枚举、交易类型约束、out_trade_no 碰撞风险、充值规则权限/契约一致。
- T1.14 openid/avatar URL 风险治理：外部 URL 拉取白名单/校验与存储策略。

### 8.3 P2 修复任务
- T2.1 public/需登录契约对齐：/public 路由与 swagger 统一。
- T2.2 regions 后备接口对齐多区域模型：改用 `GetActiveOperatorByRegion`。
- T2.3 BI 字段语义修正：`order_count` 与 `dish_count` 口径对齐。
- T2.4 补齐 location 回归测试：参数校验、mapClient nil、错误/成功路径。
- T2.5 架构/运行期治理闭环：Redis 依赖明确化、响应封装策略落地、DB 指标更新。

### 8.4 任务清单（文件级落地）
- T0.1 支付入账幂等：
  - 代码：[locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go)
  - 迁移：[locallife/db/migration/000012_add_riders_and_deliveries.up.sql](locallife/db/migration/000012_add_riders_and_deliveries.up.sql)
  - 测试：[locallife/worker/task_process_payment_test.go](locallife/worker/task_process_payment_test.go)
  - Owner：TBD（支付/账务）
- T0.2 商户申请状态机保护：
  - 代码：[locallife/db/sqlc/tx_merchant_application.go](locallife/db/sqlc/tx_merchant_application.go)、[locallife/api/merchant_application.go](locallife/api/merchant_application.go)
  - 测试：[locallife/api/merchant_application_test.go](locallife/api/merchant_application_test.go)
  - Owner：TBD（商户申请）
- T0.3 上传链路防护：
  - 代码：[locallife/api/ecommerce_applyment.go](locallife/api/ecommerce_applyment.go)、[locallife/api/public_url.go](locallife/api/public_url.go)、[locallife/util/upload.go](locallife/util/upload.go)
  - 测试：[locallife/api/ecommerce_applyment_test.go](locallife/api/ecommerce_applyment_test.go)
  - Owner：TBD（上传安全）
- T0.4 uploads 直出收敛：
  - 代码：[locallife/api/upload_signed.go](locallife/api/upload_signed.go)、[locallife/api/server.go](locallife/api/server.go)
  - 测试：[locallife/api/upload_signed_test.go](locallife/api/upload_signed_test.go)
  - Owner：TBD（资源访问）
- T1.1 NotFound 统一：
  - 代码：[locallife/db/sqlc/error.go](locallife/db/sqlc/error.go) + 相关 handler
  - 测试：相关 `*_test.go` stub 统一 ErrNoRows
  - Owner：TBD（基础设施）
- T1.3 日志脱敏与体积限制：
  - 代码：[locallife/api/middleware_tracing.go](locallife/api/middleware_tracing.go)、[locallife/api/payment_callback.go](locallife/api/payment_callback.go)
  - 测试：[locallife/api/payment_callback_test.go](locallife/api/payment_callback_test.go)
  - Owner：TBD（可观测性）
- T1.5 WebSocket 安全：
  - 代码：[locallife/api/notification.go](locallife/api/notification.go)、[locallife/api/middleware.go](locallife/api/middleware.go)
  - 测试：[locallife/api/notification_test.go](locallife/api/notification_test.go)
  - Owner：TBD（通知/实时）
- T1.6 推送协议统一：
  - 代码：[locallife/worker/task_send_notification.go](locallife/worker/task_send_notification.go)、[locallife/websocket/pubsub.go](locallife/websocket/pubsub.go)
  - 测试：[locallife/worker/task_send_notification_test.go](locallife/worker/task_send_notification_test.go)
  - Owner：TBD（消息推送）
- T1.7 标签权限：
  - 代码：[locallife/api/tag.go](locallife/api/tag.go)、[locallife/api/server.go](locallife/api/server.go)
  - 测试：[locallife/api/tag_test.go](locallife/api/tag_test.go)
  - Owner：TBD（运营后台）
- T1.9 折扣规则归属校验：
  - 代码：[locallife/api/discount.go](locallife/api/discount.go)、[locallife/api/server.go](locallife/api/server.go)
  - 测试：[locallife/api/discount_test.go](locallife/api/discount_test.go)
  - Owner：TBD（营销/商户）
- T1.10 收藏幂等：
  - 代码：[locallife/db/query/favorite.sql](locallife/db/query/favorite.sql)、[locallife/api/favorite.go](locallife/api/favorite.go)
  - 测试：[locallife/api/favorite_test.go](locallife/api/favorite_test.go)
  - Owner：TBD（消费者）

### 8.5 子步骤与验收标准（简版）
- T0.1 支付入账幂等门闩
  - 子步骤：新增支付/押金唯一约束；支付成功处理加幂等锁或幂等表；补幂等单测与并发用例。
  - 验收：重复回调只入账一次；重复任务返回成功且无多条流水。
- T0.2 商户申请状态机保护
  - 子步骤：Reset/Approve 前校验当前商户状态；禁止对 active 降级；补状态机测试。
  - 验收：active 商户不会被 reset/approve 降级；状态机回归测试通过。
- T0.3 上传链路防护
  - 子步骤：URL 白名单+内网拦截+超时+大小限制；替换 io.ReadAll 为限制读取；统一 magic 校验。
  - 验收：超大文件/内网 URL 被拒；OCR 与非 OCR 上传均做一致校验。
- T0.4 uploads 直出收敛
  - 子步骤：收敛 public 路径；签名访问绑定归属；补鉴权用例。
  - 验收：未授权用户无法访问证照/敏感资源；历史公开路径不可直出。
- T1.1 NotFound 统一
  - 子步骤：统一 NotFound 判断函数；替换 handler 与测试 stub。
  - 验收：线上/测试对 NotFound 均返回 404；不再出现 404→500 漂移。
- T1.3 日志脱敏与体积限制
  - 子步骤：移除 req_payload 与 body 明文；记录 hash/长度；webhook 加 body 上限。
  - 验收：日志中不含 token/手机号/地址/证件/回调明文；超限 body 返回 413/400。
- T1.5 WebSocket 安全
  - 子步骤：CheckOrigin 白名单；token 仅 header；移除 query token。
  - 验收：跨站 origin 被拒；query token 不再被接受与记录。
- T1.6 推送协议统一
  - 子步骤：统一 Redis payload schema；API 端向后兼容旧结构（如需）。
  - 验收：worker 推送可被 API 正常解包并送达；异常 payload 有明确日志。
- T1.7 标签权限
  - 子步骤：路由增加管理员中间件；handler 验权。
  - 验收：非管理员创建标签返回 403；管理员可成功创建。
- T1.9 折扣规则归属校验
  - 子步骤：list/get 增加商户归属校验；路由使用 MerchantStaffMiddleware。
  - 验收：越权访问返回 403；商户自身可读写。
- T1.10 收藏幂等
  - 子步骤：改为 UPSERT 或冲突时返回已有记录；处理 no rows 返回。
  - 验收：重复收藏返回 200；无 500。

---

## 10. 已完成修复（记录）

- 10.1 请求日志脱敏：移除 `req_payload` 明文日志；`RawQuery` 进行 token 脱敏。
  - [locallife/api/merchant_application.go](locallife/api/merchant_application.go#L342-L346)
  - [locallife/api/middleware_tracing.go](locallife/api/middleware_tracing.go#L1-L112)
- 10.2 Webhooks body 上限与脱敏：统一限流读取，移除 body 明文日志。
  - [locallife/api/payment_callback.go](locallife/api/payment_callback.go#L26-L110)
- 10.3 标签创建权限：创建标签需管理员角色。
  - [locallife/api/server.go](locallife/api/server.go#L439-L444)
- 10.4 折扣规则归属权限：折扣路由加商户 staff 角色，handler 校验商户归属。
  - [locallife/api/server.go](locallife/api/server.go#L1092-L1100)
  - [locallife/api/discount.go](locallife/api/discount.go#L40-L220)
- 10.5 Redis 推送协议统一：worker 发布 `NotificationPushMessage` 结构。
  - [locallife/worker/task_send_notification.go](locallife/worker/task_send_notification.go#L134-L186)
- 10.6 WebSocket 安全加固：移除 query token；升级时校验 Origin 白名单。
  - [locallife/api/middleware.go](locallife/api/middleware.go#L24-L41)
  - [locallife/api/notification.go](locallife/api/notification.go#L452-L520)
- 10.7 收藏幂等：冲突返回已有记录。
  - [locallife/db/query/favorite.sql](locallife/db/query/favorite.sql#L1-L13)
- 10.8 public 接口鉴权收敛：search/scan/regions/promotions 统一纳入鉴权路由组。
  - [locallife/api/server.go](locallife/api/server.go#L270-L340)
- 10.9 统计日期口径修复：end_date 统一到当日 23:59:59。
  - [locallife/api/merchant_stats.go](locallife/api/merchant_stats.go#L1-L220)
- 10.10 BI 字段语义修正：菜品分类统计返回订单数口径。
  - [locallife/db/query/merchant_stats.sql](locallife/db/query/merchant_stats.sql#L194-L214)
  - [locallife/api/merchant_stats.go](locallife/api/merchant_stats.go#L880-L909)
- 10.11 上传链路防护：外部URL白名单/内网拦截/超时/大小限制与本地路径清理。
  - [locallife/api/ecommerce_applyment.go](locallife/api/ecommerce_applyment.go#L520-L760)
- 10.12 uploads 直出收敛：reviews 允许公共直出；营业执照/食品经营许可证对登录用户可签名访问；身份证仅归属者与平台管理员可签名访问。
  - [locallife/api/upload_signed.go](locallife/api/upload_signed.go#L80-L250)
- 10.13 默认地址原子性：创建/设置默认地址改为事务。
  - [locallife/db/sqlc/tx_user_address.go](locallife/db/sqlc/tx_user_address.go#L1-L80)
  - [locallife/api/user_address.go](locallife/api/user_address.go#L150-L490)
- 10.14 Billing Group 权限收敛：加入/列表/订单需会话归属或成员权限。
  - [locallife/api/billing_group.go](locallife/api/billing_group.go#L60-L220)
- 10.15 库存 ErrNoRows 语义修复：无库存记录不再视为无限库存。
  - [locallife/api/inventory.go](locallife/api/inventory.go#L360-L405)
- 10.16 会员充值 attach JSON 修复：改用 json.Marshal，避免 <nil>。
  - [locallife/api/membership.go](locallife/api/membership.go#L620-L690)
- 10.17 Casbin fail-open 修复：初始化失败直接阻断启动；中间件未初始化时拒绝请求。
  - [locallife/api/server.go](locallife/api/server.go#L150-L168)
  - [locallife/api/casbin_enforcer.go](locallife/api/casbin_enforcer.go#L190-L305)
- 10.18 商户申请状态机保护：提交后禁止编辑；reset/approve 不降级 active/suspended。
  - [locallife/api/merchant_application.go](locallife/api/merchant_application.go#L135-L190)
  - [locallife/db/sqlc/tx_merchant_application.go](locallife/db/sqlc/tx_merchant_application.go#L36-L160)
- 10.19 骑手申请通过原子性：审核通过与创建骑手记录同事务。
  - [locallife/api/rider_application.go](locallife/api/rider_application.go#L560-L650)
  - [locallife/db/sqlc/tx_rider_application.go](locallife/db/sqlc/tx_rider_application.go)
- 10.20 支付成功幂等强化：统一事务处理+业务幂等记录（押金/会员/预订）+订单支付幂等。
  - [locallife/worker/task_process_payment.go](locallife/worker/task_process_payment.go#L305-L380)
  - [locallife/db/sqlc/tx_payment_success.go](locallife/db/sqlc/tx_payment_success.go)
  - [locallife/db/query/rider_deposit.sql](locallife/db/query/rider_deposit.sql#L9-L15)
  - [locallife/db/query/membership.sql](locallife/db/query/membership.sql#L139-L173)
  - [locallife/db/query/reservation_payment.sql](locallife/db/query/reservation_payment.sql)
  - [locallife/db/sqlc/tx_create_order.go](locallife/db/sqlc/tx_create_order.go#L200-L470)
- 10.21 订单取消回滚优惠券：取消订单恢复 user_voucher 状态并回退 used 计数。
  - [locallife/db/sqlc/tx_order_status.go](locallife/db/sqlc/tx_order_status.go#L126-L219)
  - [locallife/db/query/voucher.sql](locallife/db/query/voucher.sql#L54-L123)
- 10.22 地图编码兜底：adcode 匹配失败时按城市/区县名称回退匹配。
  - [locallife/api/location.go](locallife/api/location.go#L192-L240)
  - [locallife/db/query/region.sql](locallife/db/query/region.sql#L9-L28)
- 10.23 regions 后备接口对齐多区域模型：区域占用检查改为 operator_regions。
  - [locallife/api/region.go](locallife/api/region.go#L330-L379)
  - [locallife/db/query/operator_region.sql](locallife/db/query/operator_region.sql#L37-L52)
- 10.24 location 回归测试补齐：覆盖 adcode 与名称回退匹配。
  - [locallife/api/location_test.go](locallife/api/location_test.go)
- 10.25 财务/统计权限收敛：商户财务与统计接口仅限 owner/manager。
  - [locallife/api/server.go](locallife/api/server.go#L760-L792)
- 10.26 权限矩阵收敛：角色鉴权统一走 Casbin（标签创建与运营商骑手审核）。
  - [locallife/api/server.go](locallife/api/server.go#L439-L745)
  - [locallife/casbin/policy.csv](locallife/casbin/policy.csv#L35-L83)
- 10.27 public 路由契约对齐：Swagger 标注为需登录访问。
  - [locallife/api/dish.go](locallife/api/dish.go#L800-L812)
  - [locallife/api/merchant.go](locallife/api/merchant.go#L1357-L1774)
- 10.28 public 搜索/扫码脱敏：不返回商户电话/地址/精确经纬度。
  - [locallife/api/search.go](locallife/api/search.go#L63-L520)
  - [locallife/api/scan.go](locallife/api/scan.go#L28-L315)
- 10.29 public 搜索/扫码限流：对搜索与扫码接口增加更严格限流。
  - [locallife/api/server.go](locallife/api/server.go#L304-L322)
- 10.30 搜索/扫码契约对齐：Swagger 标注为需登录访问。
  - [locallife/api/search.go](locallife/api/search.go#L97-L580)
  - [locallife/api/scan.go](locallife/api/scan.go#L100-L113)
- 10.31 CORS 响应封装头补齐：允许并暴露 X-Response-Envelope。
  - [locallife/api/middleware_security.go](locallife/api/middleware_security.go#L1-L120)
- 10.32 RateLimiter 内存增长修复：敏感接口复用主限流访客表，统一清理。
  - [locallife/api/middleware_ratelimit.go](locallife/api/middleware_ratelimit.go#L1-L220)
- 10.33 退出清理完善：关闭 WebSocket Hub/PubSub/RateLimiter。
  - [locallife/api/server.go](locallife/api/server.go#L1-L220)
  - [locallife/main.go](locallife/main.go#L1-L220)
- 10.34 自动迁移开关：生产环境需显式开启 AUTO_MIGRATE。
  - [locallife/util/config.go](locallife/util/config.go#L1-L220)
  - [locallife/main.go](locallife/main.go#L1-L220)
  - [locallife/app.env.example](locallife/app.env.example#L1-L120)
- 10.35 加密 key 强制：生产环境缺少 DATA_ENCRYPTION_KEY 阻断启动。
  - [locallife/api/server.go](locallife/api/server.go#L1-L220)
- 10.36 用餐会话参与者证明：桌台口令校验 + 口令哈希存储。
  - [locallife/api/dining_session.go](locallife/api/dining_session.go#L140-L340)
  - [locallife/api/table.go](locallife/api/table.go#L1-L620)
  - [locallife/db/query/table.sql](locallife/db/query/table.sql)
- 10.37 申诉幂等与审核后处理可靠性：重复提交返回既有申诉；异步投递失败走同步兜底。
  - [locallife/api/appeal.go](locallife/api/appeal.go#L320-L1260)

---

## 9. 回归测试清单（按风险面）

### 9.1 资金/幂等
- 支付成功重复回调不重复入账（支付单、押金流水、通知）。
- 订单取消回滚优惠券状态/计数。

### 9.2 权限/越权
- Billing Group 成员校验、折扣规则归属校验、标签创建管理员权限。
- public 接口脱敏字段校验。

### 9.3 上传/资源访问
- 外部 URL 拉取白名单/超时/大小限制。
- uploads/public 访问权限与签名访问校验。

### 9.4 状态机/一致性
- 商户申请 reset/approve 不降级 active。
- rider/merchant 创建失败回滚与事务一致性。

### 9.5 日志合规
- request payload、token、webhook body 不落日志。
