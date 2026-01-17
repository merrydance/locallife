# 后端代码审查任务清单（可勾选）

> 目标：从总体架构出发，逐组审查 API 接口实现，并对“接口 → 业务逻辑 → 数据库（含事务/一致性）→ 返回结构/错误处理”的完整链路做一致性与完成度评估。
>
> 使用方式：
> - 每完成一个模块（或一组接口）就在对应任务前打勾。
> - 每个模块审查时，按“链路核对清单”逐项记录结论与问题。
> - 全部完成后，产出《总体审查报告》（见文末输出物）。

---

## 0. 审查范围与产出物

### 范围
- 后端 Go 服务：路由/handler、业务逻辑、数据库访问（`db/sqlc`）、异步任务（`worker`）、外部集成（如 `wechat` / `weather`）。
- 接口定义与分组：以 `api/` 目录内文件为主。
- 文档：Swagger（`docs/swagger.yaml|json`）与实际实现一致性。

### 输出物（最终）
- 《总体审查报告》：
  - 项目完成度（模块维度、接口覆盖维度、测试覆盖维度）
  - 设计一致性/自相矛盾点
  - 冗余代码与可合并点
  - 缺失功能/未闭环链路
  - 风险清单（安全、并发、幂等、数据一致性、可观测性）
  - 具体改进建议（按优先级）

建议最终报告文件名：`docs/backend_review_report.md`

---

## 1. 总体架构审查（从入口到依赖边界）

- [x] 1.1 进程启动与依赖初始化（配置、DB 连接池、Redis、迁移、任务处理器、HTTP Server）
- [x] 1.2 模块边界与依赖方向（`api` / `db` / `worker` / `wechat` / `util` 等是否清晰）
- [x] 1.3 数据模型与存储层抽象（sqlc store：是否有一致的事务封装、是否泄露查询细节）
- [x] 1.4 并发与生命周期（graceful shutdown、goroutine 泄露、超时/取消传递）
- [x] 1.5 可观测性（日志、指标、Tracing、中间件一致性）
- [x] 1.6 配置与环境（`app.env`/Docker/Compose/Makefile，敏感配置管理）

**结论记录**
- 关键发现：
  - 架构形态清晰：单体 Gin API（`api/server.go` 路由聚合）+ sqlc Store/Tx（`db/sqlc`）+ 异步任务 Asynq（`worker`）+ WebSocket Hub + Redis Pub/Sub（`websocket`）+ 外部集成（`wechat`/`weather`/`maps`）。
  - 启动流程有基本健壮性：DB 连接池参数显式配置并 `Ping`；HTTP server 设置了 `ReadHeaderTimeout/ReadTimeout/WriteTimeout/IdleTimeout`；调度器（天气、自动标签）、任务处理器与 HTTP server 都接入了 `errgroup` 的优雅退出。
  - 存储层抽象质量较高：`db/sqlc/store.go` 通过 `Store` interface 聚合 Query + 多个 Tx（`tx_*.go`），并有统一 `execTx` 封装，便于对“跨表写入/状态流转”做一致性审查。
  - 横切中间件较完整：安全响应头、CORS、Request-ID、请求日志、Prometheus、限流、全局超时、统一响应封装（可选）。

- 风险：
  - 设计自相矛盾：
    - Redis 依赖：`main.go` 强制要求 `REDIS_ADDRESS`，但 `api/server.go` 在 WebSocket 队列/推送上实现了无 Redis 的内存降级路径；需要明确“Redis 是否为强依赖”，避免文档/实现互相打架。
    - 统一响应封装：路由注释写“统一 {code,message,data}”，现已调整为 **默认启用**，可通过 `X-Response-Envelope: 0` 显式关闭；同时 CORS 已允许/暴露该 header。
  - 生命周期/资源释放不完整：
    - WebSocket `Hub` 支持 `Shutdown()`、`PubSubManager` 支持 `Stop()`，但当前启动路径里未看到在进程退出时调用；存在 goroutine/Redis 连接泄露风险。
    - `RateLimiter` 启动了后台清理协程且有 `Stop()`，但未在退出时调用；另外 `SensitiveAPIMiddleware` 的 map 没有清理机制，长期运行可能增长。
  - 迁移策略风险：`runDBMigration()` 在每次启动都尝试 `migrate.Up()`；生产环境通常需要更强的控制（开关、权限、失败回滚策略、与发布流程配合）。
  - 安全策略不够“强约束”：`DATA_ENCRYPTION_KEY` 缺失仅告警继续运行（会明文存敏感字段）；如果确实存储身份证/银行卡等，建议在生产环境变为强制要求。
  - 可观测性“定义了但未落地”：Prometheus 里预留了 DB 连接池指标更新函数 `UpdateDBMetrics`，但当前未见周期性调用点；可能导致监控缺口。

- 建议：
  - 先统一“强依赖 vs 可降级”：明确 Redis、天气缓存、WebSocket 推送、异步任务是否可独立关闭；把开关写进配置与文档（并在启动时按开关行为一致）。
  - 补齐退出钩子：在 graceful shutdown 中显式调用 `wsHub.Shutdown()`、`wsPubSub.Stop()`、`rateLimiter.Stop()` 等，保证资源与 goroutine 可回收。
  - 明确响应协议：决定是否全局强制 envelope；若保留 opt-in，补齐 CORS header 白名单与文档说明，并确保前端可用。
  - 给迁移加策略开关：例如 `AUTO_MIGRATE=true/false` 或仅在 `development` 自动执行，生产走 CI/CD 迁移。
  - 加固敏感数据策略：生产环境缺失 `DATA_ENCRYPTION_KEY` 时拒绝启动，或至少对涉及敏感字段的写入接口拒绝。

---

## 2. 横切能力审查（全局一致性）

- [x] 2.1 认证/授权（JWT/Token、RBAC/Casbin、路由权限矩阵是否一致）
- [x] 2.2 请求校验（validator、参数边界、枚举/状态机约束）
- [x] 2.3 统一响应结构与错误码（`response_envelope`、错误映射是否一致）
- [x] 2.4 安全（鉴权绕过、越权、注入、上传安全、回调验签、敏感信息日志）
- [x] 2.5 限流与防刷（ratelimit、风控/风控点是否落地）
- [x] 2.6 幂等与重试（支付回调、下单、发券等关键链路）
- [x] 2.7 事务与一致性（跨表写入、资金/库存/订单状态变更）
- [x] 2.8 测试策略（单测/集成测、可测性、关键路径覆盖）

**结论记录**
- 关键发现：
  - 权限体系是“双轨”：既有 `MerchantStaffMiddleware/Operator...` 这类 DB 关系校验，也有 Casbin（`casbin/model.conf` + `policy.csv`）的路径/方法授权；另外提供了 `/v1/role-access` 元数据接口给客户端做“调用前自检”。
  - 统一错误处理具备“防泄露”思路：`internalError()` 记录结构化 pg error 并返回通用 `internal server error`；4xx 走 `errorResponse(err)` 直出错误信息。
  - 微信支付回调链路做了关键安全动作：验签 + 解密 + 幂等（通知 ID 去重）+ 金额校验 + 异步队列分发（失败会告警到平台 WebSocket）。
  - 上传访问策略有区分：`/uploads/*` 支持公开目录直出 + 私有目录签名访问（HMAC + TTL）。

- 风险：
  - 鉴权降级风险（高优先级）：Casbin 中间件在 `globalCasbinEnforcer == nil` 时会“跳过权限检查并放行”。如果生产环境 Casbin 初始化失败，部分路由组可能直接变成无授权保护（取决于是否依赖 CasbinRoleMiddleware）。建议至少“fail closed”。
  - 权限矩阵可能与实现漂移：`/v1/role-access` 是手写维护，Casbin policy 也是手写维护；两者与 `api/server.go` 路由装配存在重复来源，后续很容易不一致。
  - 统一响应封装存在协议不一致：路由注释强调统一 `{code,message,data}`，现已调整为 **默认启用**，可通过 `X-Response-Envelope: 0` 显式关闭。
  - 上传“归属校验”疑似与目录约定不一致：`isUploadPathOwnedByUser()` 用 `uid` 在路径里匹配（如 `/merchants/{uid}/`），但实际上传目录更像是 `uploads/merchants/{merchant_id}/...`；这会导致“合法用户被拒”或“越权放开（靠特殊 case）”的混乱。
  - 速率限制存在内存增长点：`RateLimiter` 有清理协程与 `Stop()`，但 `SensitiveAPIMiddleware` 的 map 无清理策略，长期运行可能无限增长。
  - 幂等建模风险：支付订单 `OrderID` 字段同时承载 order 与 reservation（从 `createPaymentOrder` 逻辑可见），但“查已存在 pending 支付单”的查询仅按 `OrderID` 查，未纳入 `BusinessType`；如果不同业务表 ID 可能碰撞，会出现错复用/串单风险。

- 建议：
  - 权限“单一事实来源”：建议把路由权限定义集中化（例如以路由组生成 policy/metadata），避免 Casbin policy 与 role-access 双维护。
  - Casbin 初始化失败改为拒绝启动或拒绝请求（fail closed），并在健康检查/ready 里反映依赖状态。
  - 响应封装协议定稿：统一为默认 envelope + `X-Response-Envelope: 0` opt-out，并在 Swagger/前端契约里写清楚。
  - 上传归属规则定稿并代码对齐：统一“uploads 路径的主键到底是 user_id 还是 merchant_id/rider_id”，把 `isUploadPathOwnedByUser` 与各模块上传路径约定统一。
  - 支付订单关联字段建议拆分：将 `PaymentOrder` 与 `order_id/reservation_id` 做结构化区分（或把查询显式加上 `business_type`），并补充相关约束/索引。

---

## 3. 接口分组审查（按模块勾选）

> 说明：每一组完成后勾选；组内可再按“接口链路核对清单”逐条抽查或全量审查。

### 3.1 用户与认证
- [x] Token：`api/token.go`
- [x] 用户：`api/user.go`
- [x] 用户地址：`api/user_address.go`
- [x] 会员：`api/membership.go`

**结论记录（Token·链路核对：登录→会话→刷新）**
- 端到端链路：
  - 登录：`POST /v1/auth/wechat-login`（`api/wechat.go`）用 code 换 openid → 查/建用户（`CreateUserTx`）→ 生成 access/refresh token（`tokenMaker.CreateToken`，type 区分）→ 写入 `sessions`（`CreateSession`，access_token/refresh_token 均为 UNIQUE）→ 返回 `session_id + access_token + refresh_token`。
  - 刷新：`POST /v1/auth/refresh`（`api/token.go`）先验 refresh token（必须为 refresh type）→ `GetSessionByRefreshToken` 查会话 → 校验 `is_revoked`、`session.user_id` 与 token payload 一致、`refresh_token_expires_at` 未过期 → 签发新 access token（不旋转 refresh token）。
  - 数据层：`sessions` 表在迁移 `000002_init_users_and_auth.up.sql` 创建；提供 `RevokeSession/RevokeUserSessions/DeleteExpiredSessions/ListUserActiveSessions` 等 SQL（但需要确认是否有后台任务实际调用 `DeleteExpiredSessions`）。

- 关键发现：
  - 认证链路“token 校验 + DB 会话校验”两层防线齐全：既要求 refresh token 自身合法，又要求数据库中存在且未撤销，降低纯 token 重放风险。
  - 路由装配清晰：`/v1/auth/*` 属于 public group，但仍受 `SensitiveAPIMiddleware(10/min)` 限流保护。
  - 测试覆盖到位：`api/token_test.go` 覆盖 OK/非法 token/过期 token/session 过期/session revoke/缺参等分支。

- 严重/重要问题（记录）：
  - **refresh token 不旋转**：刷新只返回新 access token，refresh token 仍可长期复用；一旦 refresh token 泄露，攻击窗口更大。建议至少支持“刷新即轮换 refresh token（旧 token 失效）”或提供显式登出/撤销会话能力给客户端。
  - **会话清理可能未落地**：虽有 `DeleteExpiredSessions` SQL，但当前未看到定时调用点；长期运行可能导致 sessions 表膨胀、索引变大、备份/查询成本上升。
  - **token 明文落库**：`sessions.refresh_token/access_token` 以明文存储并做 UNIQUE/查询键，数据库一旦泄露风险更高；最佳实践通常存 hash（查询时用哈希比对），或至少只存 refresh token 的 hash。

- 建议（待实现）：
  - 增加 refresh token 轮换与会话撤销接口（例如 logout / revoke current session），并在刷新时更新 session 记录（单条 session 维持“最新 refresh token”）。
  - 增加后台清理任务：周期性调用 `DeleteExpiredSessions`（或分区/TTL 策略），并补监控（sessions 行数、过期清理成功率）。
  - 将 refresh token 落库改为 hash 存储（若需要按 token 查 session，则改为 `refresh_token_hash`），降低 DB 泄露影响面。

  **结论记录（用户·链路核对：获取/更新当前用户）**
  - 端到端链路：
    - `GET /v1/users/me`（`api/user.go`）通过 `authMiddleware` 注入 `authorizationPayloadKey` → `GetUser(user_id)` → `ListUserRoles(user_id, status=active)` → 返回用户基础信息 + roles。
    - `PATCH /v1/users/me` 绑定 JSON（允许部分更新 `full_name/avatar_url`）→ 构造 `UpdateUserParams{ID, FullName?, AvatarUrl?}`（pgtype.Text + `sqlc.narg`）→ `UpdateUser` → `ListUserRoles` → 返回更新后信息。
    - URL 处理：返回时对 `avatar_url` 做 `normalizeUploadURLForClient`；写入时对输入做 `normalizeImageURLForStorage`。

  - 测试与覆盖：`api/user_test.go` 覆盖“OK/未授权/用户不存在/内部错误”，并覆盖更新接口的“全量更新/部分更新/未授权/非法 body”。

  - 关键发现：
    - “只允许操作当前用户（me）”的权限边界清晰：所有读写都基于 token payload 的 `UserID`，不存在通过 path 参数越权读取/更新他人资料的入口。
    - `UpdateUser` 采用 `COALESCE(sqlc.narg(...), ...)` 支持幂等的部分更新，避免“未传字段被清空”。

  - 风险/建议：
    - **返回 `wechat_openid` 的必要性存疑**：openid 通常属于服务端标识；若客户端业务不需要，建议不下发或改为仅下发内部 user_id，降低外泄与滥用面。
    - `avatar_url` 仅做长度校验与归一化：若前端可传任意 URL，可能引入内容安全/混合内容/钓鱼跳转风险；建议强约束为“上传系统产生的 URL/key”，或只允许设置为已上传文件的 key。

    **结论记录（用户地址·链路核对：增删改查+默认地址）**
    - 端到端链路（均需 BearerAuth）：
      - `POST /v1/addresses`：校验参数 → 坐标来源（优先 payload 坐标；缺失/非法则用 `mapClient.Geocode(detail_address)`）→ region 来源（显式 region_id 校验存在，否则 `matchRegionID(lat,lon)` 自动匹配）→ 若 `is_default=true` 先 `SetDefaultAddress(user_id)` 清空默认 → `CreateUserAddress` 落库。
      - `GET /v1/addresses/:id`：`GetUserAddress(id)`（SQL 过滤 `deleted_at IS NULL`）→ 校验 `address.user_id == token.user_id`，否则 403。
      - `GET /v1/addresses`：`ListUserAddresses(user_id)`（过滤未删除，按默认优先+时间倒序）。
      - `PATCH /v1/addresses/:id`：先 `GetUserAddress` 做存在性+归属校验 → `UpdateUserAddress(id,user_id,...)` 部分更新；若更新了经纬度且没带 region_id，会尝试基于新坐标自动匹配 region。
      - `PATCH /v1/addresses/:id/default`：先 `GetUserAddress` 校验归属 → `SetDefaultAddress(user_id)` 清空默认 → `SetAddressAsDefault(id,user_id)` 设置默认。
      - `DELETE /v1/addresses/:id`：先 `GetUserAddress` 校验归属 → `DeleteUserAddress(id,user_id)` 软删（`deleted_at=now()` 且 `is_default=false`）。

    - 数据层核对：
      - `user_addresses` 初始在迁移 `000003_add_regions_and_addresses.up.sql` 建表；软删除字段在 `000078_add_user_address_soft_delete.up.sql` 补齐。
      - 查询层在 `db/query/user_address.sql` 中统一以 `deleted_at IS NULL` 过滤（Update 通过 handler 先查再写间接保证“不可更新已删除记录”）。

    - 测试覆盖：`api/user_address_test.go` 覆盖创建/读取/列表/更新/设默认/删除的 OK 与部分鉴权分支（未覆盖：更新/删除 NotFound、Geocode fallback、自动匹配 region 的异常分支）。

    - 重要问题与建议：
      - **默认地址一致性非原子（P0 级别一致性缺口）**：`SetDefaultAddress` 与 `CreateUserAddress` / `SetAddressAsDefault` 目前是两次独立写操作，遇到并发或中途失败可能出现“多个默认”或“无默认”。建议用 DB 事务封装，或加部分唯一索引（例如 `(user_id) WHERE is_default=true AND deleted_at IS NULL`）从数据库层兜底。
      - **坐标解析存在静默降级风险**：`updateUserAddress` 对 `strconv.ParseFloat` 的错误忽略会导致解析失败时 `lon/lat=0` 仍被视为有效，进而可能错误匹配 region。建议在代码中显式校验 parse 错误并返回 400（或完全复用 `pgtype.Numeric.Scan` 的成功与否来决定有效性）。
      - 地址数量上限未使用：虽有 `CountUserAddresses` SQL，但 handler 未做限制；若需要防滥用可在创建时做上限校验。

      **结论记录（会员·链路核对：加入/充值/流水/规则/商户侧管理）**
      - 端到端链路（核心接口）：
        - 用户侧：
          - `POST /v1/memberships`：校验 merchant 存在 → `JoinMembershipTx(merchant_id,user_id)`（幂等返回已有会员）→ 返回会员账户。
          - `GET /v1/memberships`：`ListUserMemberships(user_id,limit,offset)` 返回用户所有会员卡（当前 handler 直接返回 sqlc row）。
          - `GET /v1/memberships/:id`：`GetMerchantMembership(id)` + owner 校验。
          - `POST /v1/memberships/recharge`：校验会员卡 owner → `GetMatchingRechargeRule`（按金额精确匹配，选赠送）→ `CreatePaymentOrder(business_type=membership_recharge, attach=充值参数)` → 调用微信 JSAPI 下单 → 前端拉起支付。
          - `GET /v1/memberships/:id/transactions`：owner 校验 → `ListMembershipTransactions` → 转换为 `transactionResponse`。
        - 商户侧：
          - 充值规则：`/v1/merchants/:id/recharge-rules` 的增删改（校验 merchant role + merchant_id 匹配）；`/active` 查询当前生效规则。
          - 会员设置：`GET/PUT /v1/merchants/me/membership-settings`（按 owner 获取商户）→ `Get/UpsertMerchantMembershipSettings`。
          - 商户会员管理：`/v1/merchants/:id/members` 列表/详情/余额调整（依赖 `GetUserRoleByType(role=merchant)` + related_entity_id）。
        - 支付回调落账：支付回调入队后由 worker `handleMembershipRechargePaid` 解析 `payment_orders.attach` → 调用 `RechargeTx`（`GetMembershipForUpdate` + 更新余额 + 写 `membership_transactions(type=recharge)`）。

      - 严重/重要问题（P0/P1）：
        - **P0：充值 attach 在“无匹配规则”时不是合法 JSON**：`rechargeMembership` 用 `fmt.Sprintf` 拼 JSON，`recharge_rule_id` 为 nil 时会生成 `<nil>`，worker 侧 `json.Unmarshal` 直接失败并 `SkipRetry`，导致支付成功但不入账（需要人工补偿）。建议必须改为 `json.Marshal` 生成 attach，并让 `recharge_rule_id` 为 `null` 或直接省略字段。
        - **支付方式参数与实现不一致**：请求允许 `payment_method=alipay`，但实现只走微信 JSAPI 下单；应收敛枚举或补齐分支。
        - **交易流水类型约束不一致**：迁移里 `membership_transactions.type` 仅允许 `('recharge','consume','refund','bonus')`，但商户“调整余额”接口写入 `adjustment_credit/adjustment_debit`，会因 check constraint 写入失败（当前代码吞掉错误，仅 stdout 打印）。
        - **幂等/唯一性风险**：充值 `out_trade_no` 由 `membership_id + Unix秒级时间` 拼接，同一秒内重复发起可能碰撞；建议改为纳秒/随机后缀。

      - 重要权限与设计问题：
        - `GET /v1/merchants/:id/recharge-rules` 当前无商户身份校验，且返回“所有状态”的规则；如果业务只需对用户展示可用规则，建议仅开放 `/active`，或对“全量规则”加商户权限。
        - `GET /v1/memberships` swagger 标注返回 `membershipResponse`，但 handler 直接返回 `ListUserMembershipsRow`（包含 merchant_name/logo_url 等字段），存在契约不一致风险。

### 3.2 商户侧（入驻/资料/运营）
- [x] 商户：`api/merchant.go`
- [x] 商户申请：`api/merchant_application.go`
- [x] 商户开户/进件：`api/ecommerce_applyment.go`（merchant applyment：bindbank/status）
- [x] 商户统计：`api/merchant_stats.go`
- [x] 商户财务：`api/merchant_finance.go`
- [x] 门店/位置：`api/location.go`

**结论记录（门店/位置·链路核对：逆地理编码→骑行路线→region 匹配）**
- 端到端链路（路由→handler→外部依赖→返回）：
  - 路由装配：`api/server.go` 中两条接口均挂在 `authGroup` 下（需要登录，设计意图是“避免滥用地图服务”）。
    - `GET /v1/location/reverse-geocode` → `server.reverseGeocode`
    - `GET /v1/location/direction/bicycling` → `server.getBicyclingRoute`
  - 逆地址解析：`reverseGeocode`
    - 参数：query `latitude/longitude`，`parseLatitudeLongitude` 做必填 + float 解析 + 范围校验（[-90,90], [-180,180]）。
    - 依赖：`server.mapClient.ReverseGeocode(ctx, maps.Location{Lat,Lng})`。
    - 返回：固定 `{code:0,message:"ok",data:{address/formatted_address/province/city/district/street/street_number}}`；失败时 400（参数）/500（地图 client 不可用或调用失败）。
  - 骑行路线：`getBicyclingRoute`
    - 参数：query `from/to`，格式 `lat,lng`；解析失败返回 400。
    - 依赖：`server.mapClient.GetBicyclingRoute(ctx, from, to)`。
    - 返回：固定 `{code:0,message:"ok",data:{distance,duration}}`；失败 400/500。

- 地图实现与坐标系口径（maps 包）：
  - `api/server.go` 初始化地图 client 时实际只创建自建 `OSMClient`（当 `OSMBaseURL` 配置存在时），实现接口类型为 `maps.TencentMapClientInterface`（语义上是“腾讯地图接口名，但可由 OSM 实现”）。
  - `maps/osm.go`：
    - 使用 `http.Client{Timeout:10s}`，并为 Nominatim 设置 `User-Agent`（避免 406）。
    - 坐标系转换：输入按“微信小程序 GCJ-02”理解，调用 OSRM/Nominatim 前转换为 WGS84（`GCJ02ToWGS84`），响应后（Geocode）再转回 GCJ-02。

- region 匹配（DB/兜底策略）：
  - `matchRegionID(ctx,lat,lon)` 的策略为：
    1) 若 `mapClient` 可用，则先 `ReverseGeocode` 拿到 `res.Adcode`，用 `store.GetRegionByCode(adcode)` 精确匹配；
    2) 失败则用 `store.GetClosestRegion(lat,lon)` 兜底（`db/query/region.sql`：仅在 `regions.level=3` 范围内按球面距离取最近）。
  - 该函数除了在本文件中定义外，还被用户地址与商户申请模块复用（用于“无显式 region_id 时的自动匹配”）。

- 测试覆盖：
  - 未看到 `api/location.go` 对应的直接单测/集成测；`matchRegionID` 也未见通过 mock mapClient 的显式测试用例。

- 风险与建议：
  - **P1：地图依赖的运行时不可用会导致 500**：当 `OSMBaseURL` 未配置时 `mapClient=nil`，两条 location 接口直接 500；同时 `matchRegionID` 会退化为纯 DB 最近邻（仍依赖 regions 数据质量）。建议明确该依赖在生产是否“强依赖”，并在健康检查/启动期暴露配置缺失。
  - **P1：`Adcode` 语义可能与 regions.code 不匹配**：OSM ReverseGeocode 当前把 `Adcode` 绑定为 `postcode`（邮编），而 `GetRegionByCode` 通常期望行政区划编码；这会导致“按编码精确匹配”基本失效，长期走最近邻兜底，既浪费一次 ReverseGeocode 调用，也可能在边界区域产生误匹配。建议统一 regions.code 的编码体系，或调整 OSM 侧映射/改为更可靠的字段。
  - **P2：缺少接口级回归测试**：建议至少补齐两条 location 接口的基础用例（参数校验、mapClient nil、mapClient 错误、成功返回结构），并补一条 `matchRegionID` 的“adcode 命中 vs fallback”测试，避免未来重构引入行为漂移。

**结论记录（商户·链路核对：入驻素材→商户资料→营业状态/时间→消费者端详情）**
- 端到端链路（路由→handler→DB）：
  - 入驻素材上传：`POST /v1/merchants/images/upload`（仅 `logo` 走 `wechat ImgSecCheck`）→ `util.FileUploader.UploadMerchantImage(user_id, category, ...)` → 返回 `image_url`（相对路径归一化）。
  - 入驻申请（旧版）：`POST /v1/merchants/applications` / `GET /v1/merchants/applications/me` → `CreateMerchantApplication/GetUserMerchantApplication`（见 `db/query/merchant.sql`）。
  - 管理员审核（旧版）：`GET /v1/admin/merchants/applications` / `POST /v1/admin/merchants/applications/review` → `GetUserRoleByType(role=admin)` → `List*MerchantApplications/GetMerchantApplication/UpdateMerchantApplicationStatus` → 审核通过时 `CreateMerchant(status=pending_bindbank)`。
  - 商户资料：`GET /v1/merchants/me`（当前用户关联商户，支持 owner + active staff）→ `GetMerchantByOwner`。
  - 多店铺列表：`GET /v1/merchants/my` → `ListMerchantsByOwner`（仅 owner 维度）。
  - 更新商户：`PATCH /v1/merchants/me` → 先读 `GetMerchantByOwner` → 版本号本地校验 → `UpdateMerchant(version 乐观锁)`。
  - 营业状态：`GET/PATCH /v1/merchants/me/status` → `GetMerchantByOwner` → （PATCH 时）若 `GetMerchantProfile().IsSuspended` 则拒绝 → `UpdateMerchantIsOpen/GetMerchantIsOpen`。
  - 营业时间：`GET/PUT /v1/merchants/me/business-hours` → `GetMerchantByOwner` → `SetBusinessHoursTx`（删除后全量重建，原子替换）/`ListMerchantBusinessHours`。
  - 促销聚合：`GET /v1/merchants/{id}/promotions` → `GetMerchant` 存在性校验 → 聚合读取配送费优惠/满减/券（读取失败会静默降级为空数组）。
  - “公开接口”商户详情：`GET /v1/public/merchants/{id}` + `/dishes` + `/combos` + `/rooms`（代码注释称“公开”，但路由装配在 `authGroup` 下，实际需要登录）。

- 数据层核对（迁移/SQL）：
  - 建表：`db/migration/000004_add_merchants_and_tags.up.sql` 创建 `merchants/merchant_applications/merchant_business_hours`。
  - 状态机演进：`db/migration/000050_enhance_merchant_applications.up.sql` 将 `merchant_applications.status` 约束改为 `draft/submitted/approved/rejected` 且默认 `draft`。
  - 员工体系：`db/query/merchant.sql` 的 `GetMerchantByOwner` 会把 `merchant_staff(status=active)` 纳入“当前用户关联商户”；员工表在 `000069_add_merchant_staff.up.sql` 引入，后续迁移增加 `pending` 角色/状态。
  - 商户更新使用 `version` 乐观锁（`db/query/merchant.sql: UpdateMerchant`）。

- 测试覆盖现状（`api/merchant_test.go`）：
  - 已覆盖：图片上传、入驻申请（旧版）创建/查询、`/merchants/me` 获取、`/merchants/me` 更新、管理员申请列表（部分）。
  - 未覆盖：营业状态 GET/PATCH、营业时间 GET/PUT、`/merchants/{id}/promotions`、`/public/merchants/*` 系列消费者端聚合接口。

- 严重/重要问题：
  - **P0：入驻申请状态机与数据库约束不一致**：`merchant_applications` 在迁移后仅允许 `draft/submitted/approved/rejected`，但 `api/merchant.go` 的旧版入驻/审核接口仍使用 `pending`（包含 query binding 枚举、审核前置校验、冲突判断、测试用例 status/filter）。在已执行 `000050` 的库上，这些接口大概率会出现“查不出来/无法审核/重复提交”等行为漂移。
  - **P1：ErrNoRows 类型不统一导致错误分支不可靠**：多个 handler 用 `errors.Is(err, pgx.ErrNoRows)` 判定 404/409，但 `api/merchant_test.go` 等测试普遍 stub `sql.ErrNoRows`。建议以 `db.ErrRecordNotFound`（已在 `db/sqlc/error.go` 定义为 `pgx.ErrNoRows`）作为单一判定源，或统一 mock/stub 的 ErrNoRows 类型。
  - **P1：商户敏感操作的角色边界不清晰**：`/merchants/me` 更新、`/merchants/me/status` 开关店、`/merchants/me/business-hours` 设置时间均仅依赖 `GetMerchantByOwner`（owner 或任意 active staff）而未通过 `MerchantStaffMiddleware(owner/manager)` 约束；如果期望“仅老板/店长可改营业状态/时间”，需在路由层或 handler 层补齐。
  - **P2：命名为 public 的接口实际需要登录**：`/v1/public/merchants/*` 当前装配在 `authGroup` 下，Swagger/注释称“公开接口”会误导客户端与权限矩阵。
  - **P2：消费者端聚合存在 N+1/排序不稳定**：`getPublicMerchantRooms` 每个 room 额外查询标签（N+1）；`getPublicMerchantDishes` 使用 map 聚合分类导致分类顺序不稳定（建议按 `sort_order` 排序输出）。

- 建议（按优先级）：
  - 优先收敛“入驻申请”两套实现（旧 `/merchants/applications*` 与新 `/merchant/application*`）：明确哪套对外，补齐/删除另一套，统一 status 枚举与迁移约束。
  - 统一 NotFound 错误判定与测试 stub（建议使用 `db.ErrRecordNotFound`）。
  - 明确 staff 权限模型：对“开关店/营业时间/商户资料”加 `MerchantStaffMiddleware`（owner/manager）或 Casbin 规则，避免过宽授权。
  - 若确实要对外公开 `/public/merchants/*`，将其移动到非认证 group 或将描述改为“需登录”。

  **结论记录（商户申请·链路核对：草稿→素材→OCR→提交→自动审核→通过/拒绝→重置）**
  - 端到端链路（路由→handler→DB→worker）：
    - 路由装配：`/v1/merchant/application/*` 均在 `authGroup` 下（需要登录，见 `api/server.go`）。
    - 草稿：`GET /v1/merchant/application`（`getOrCreateMerchantApplicationDraft`）→ `GetMerchantApplicationDraft`（允许 `draft/submitted/rejected/approved`）→ 若无记录则 `CreateMerchantApplicationDraft` → 返回 `merchantApplicationDraftResponse`。
    - 基础信息：`PUT /v1/merchant/application/basic`（`updateMerchantApplicationBasicInfo`）→ `GetMerchantApplicationDraft` → `checkApplicationEditable`（`submitted/rejected/approved` 会自动 `ResetMerchantApplicationTx` 到 `draft`）→ `UpdateMerchantApplicationBasicInfo(status='draft')`；可选自动 `matchRegionID(lat,lon)`。
    - 门头/环境照：`PUT /v1/merchant/application/images`（`updateMerchantApplicationImages`）→ 同样会在非 draft 时先 reset → `UpdateMerchantApplicationImages`（jsonb 数组）。
    - OCR 上传与异步处理：
      - `POST /v1/merchant/application/license/ocr`：上传/复用营业执照图片 → `UpdateMerchantApplicationBusinessLicense` 写入 `business_license_image_url` + `business_license_ocr=pending` → 分发 asynq 任务 `TaskMerchantApplicationBusinessLicenseOCR`。
      - `POST /v1/merchant/application/foodpermit/ocr`：上传/复用食品证 → `UpdateMerchantApplicationFoodPermit` 写入 `food_permit_url` + `food_permit_ocr=pending` → 分发 `TaskMerchantApplicationFoodPermitOCR`。
      - `POST /v1/merchant/application/idcard/ocr`：上传/复用身份证（Front/Back）→ `UpdateMerchantApplicationIDCardFront/Back` 写入图片 URL + 对应 `*_ocr=pending` → 分发 `TaskMerchantApplicationIDCardOCR`。
      - worker 处理（`worker/task_merchant_application_ocr.go`）：
        - 先把对应 `*_ocr` 更新为 `processing`，再调用 wechat OCR（先压缩到 `DefaultMaxOCRBytes`），成功后写入 `done` 的 OCR JSON，并同步回填结构化字段（如 `business_license_number/business_scope/legal_person_name/legal_person_id_number`）；失败写 `failed` 并给出 `user_hint`，部分错误走 `asynq.SkipRetry`。
    - 提交与自动审核：`POST /v1/merchant/application/submit`（`submitMerchantApplication`）→ `validateMerchantApplicationRequired`（含 `region_id` 必填）→ `SubmitMerchantApplication`（或已是 `submitted` 则直接重试审核）→ `checkMerchantApplicationApproval`（营业执照/食品证/身份证有效期、经营范围、地址模糊匹配、企业名一致、地址占用校验等）→
      - 通过：`ApproveMerchantApplicationTx`（事务：申请置 `approved` + 创建/更新 merchants + 赋予用户 merchant 角色）。
      - 拒绝：`RejectMerchantApplication(status='submitted'→'rejected', reject_reason=...)`。
    - 重置：`POST /v1/merchant/application/reset`（`resetMerchantApplication`）仅允许 `rejected` 状态 → `ResetMerchantApplicationTx`。

  - 数据层核对（SQL/迁移/事务）：
    - SQL：`db/query/merchant_application.sql` 覆盖草稿获取/创建、基础信息更新、图片数组更新、三类 OCR 字段更新、提交/通过/拒绝/重置、地址占用检查。
    - 状态机：迁移 `db/migration/000050_enhance_merchant_applications.up.sql` 将 `merchant_applications.status` 约束升级为 `draft/submitted/approved/rejected`。
    - 事务：`db/sqlc/tx_merchant_application.go` 的 `ApproveMerchantApplicationTx/ResetMerchantApplicationTx` 负责“申请状态 + merchants + user_roles”的一致性。

  - 测试覆盖现状（`api/merchant_application_test.go`）：
    - 已覆盖：草稿创建/获取、基础信息更新（含非 draft 自动 reset）、提交的通过/拒绝关键分支、reset（但走的是旧查询 `GetUserMerchantApplication`）、地址匹配算法单测。
    - 未覆盖：`/images`、三类 OCR 上传接口、OCR worker 处理（成功/失败/SkipRetry）、approved 后重复提交/重复编辑的“商户状态演进”风险分支。

  - 严重/重要问题：
    - **P0：重置/再提交可能“降级”已存在商户状态**：
      - `ResetMerchantApplicationTx` 在 reset 申请时会把已存在的 `merchants.status` 强制改为 `pending`；结合 `checkApplicationEditable`（`approved/submitted` 也会自动 reset），会导致“商户已经 active 仍被降为 pending”。
      - `ApproveMerchantApplicationTx` 在“已有商户”场景会把商户状态强制写为 `approved`（即使原本是 `active`）。这会带来线上可用性/权限/结算链路的严重漂移风险。
    - **P1：reset handler 混用旧/新查询**：`resetMerchantApplication` 读取的是 `GetUserMerchantApplication`（旧链路，来自 `db/query/merchant.sql`），而不是新链路的 `GetMerchantApplicationDraft`；可能导致“拿到不该被 reset 的记录/状态判断漂移”，同时测试用例也因此锁死在旧查询。
    - **P1：Swagger/注释与实现不一致**：
      - `getOrCreateMerchantApplicationDraft` 注释写“已通过的申请不会返回/409”，但实现允许返回 `approved` 并用于继续编辑。
      - `uploadMerchantFoodPermitOCR` 文档标注必须传 image，但实现支持“不传则复用已上传图片”。
    - **P1：OCR 文本日志可能泄露敏感信息**：`worker/task_merchant_application_ocr.go` 在食品证解析失败时会输出 `raw_text_preview`（最多 200 字），可能包含许可证号/地址等敏感字段；建议改为仅输出 hash/统计信息。
    - **P2：敏感字段请求日志**：`updateMerchantApplicationBasicInfo` 会记录 `req_payload`（含电话/地址/坐标），建议在生产环境做脱敏或降级为 debug。
    - **P2：无法“清空”图片数组**：`updateMerchantApplicationImages` 只有在 `len(...)>0` 才更新字段，客户端无法把数组显式置空（与 COALESCE 语义叠加会造成“删不掉图”）。

  - 建议（按优先级）：
    - 先定清“申请重置/再次申请”与“现网商户状态”的关系：至少避免把 `active` 商户降级为 `pending/approved`（加条件、引入新状态、或把“编辑申请”限制在未激活阶段）。
    - reset 统一使用 `GetMerchantApplicationDraft`（并补齐按状态筛选），同步调整测试用例，避免旧/新链路混用。
    - 对齐 Swagger/注释与真实行为（尤其是 approved 是否可编辑、foodpermit OCR 是否允许复用图片）。
    - 删除/降级 OCR 失败时的 `raw_text_preview` 日志，并对包含身份证号等字段的日志做统一脱敏策略。
    - 补齐关键测试：OCR handler（写 pending + enqueue）、worker 更新（processing/done/failed）、以及“已有商户 active 时重复提交/重置”的保护逻辑。

  **结论记录（商户开户/进件·链路核对：bindbank → 微信进件 → 状态回写 → 商户激活）**
  > 说明：本节仅覆盖商户端 `/v1/merchant/applyment/*` 两个接口；同文件 `api/ecommerce_applyment.go` 内的骑手/运营商进件链路后续在对应模块再审。

  - 端到端链路（路由→handler→DB→外部集成）：
    - 路由装配：`POST /v1/merchant/applyment/bindbank`、`GET /v1/merchant/applyment/status`（见 `api/server.go` 的 `merchantApplymentGroup`）。
    - 绑定银行卡并提交进件（`merchantBindBank`）：
      - 鉴权：从 `authorizationPayloadKey` 取 `UserID` → `GetMerchantByOwner(user_id)` 获取商户（owner/active staff 都可能命中，取决于该 SQL 的 join 逻辑）。
      - 前置状态：仅允许 `merchants.status in ('approved','pending_bindbank')`。
      - 幂等/防重：`GetLatestEcommerceApplymentBySubject(subject=merchant)`，若已有 `submitted/auditing/to_be_signed/signing` 则拒绝；若 `finish` 则拒绝。
      - 数据来源：读取 `GetUserMerchantApplication(user_id)`（旧版商户申请表）作为商户名称/证件号/证件图片/OCR 数据来源；解析身份证有效期 `parseIDCardValidTime`。
      - 本地落库：对身份证号/银行卡号等做 `util.EncryptSensitiveField` 后写入 `CreateEcommerceApplyment(status='pending')`（`db/query/ecommerce_applyment.sql`）。
      - 微信侧提交（可选）：
        - 未配置 `ecommerceClient`：仅更新 `merchants.status='bindbank_submitted'` 并返回。
        - 已配置：`uploadImageToWechat` 拉取/读取图片 → `ecommerceClient.UploadImage` 获取 MediaID；再 `EncryptSensitiveData` 加密敏感字段 → `CreateEcommerceApplyment`(wechat) 提交 → `UpdateEcommerceApplymentToSubmitted` 写入微信 `applyment_id` + `status='submitted'` → 更新 `merchants.status='bindbank_submitted'`。
    - 查询进件状态（`getMerchantApplymentStatus`）：
      - 读取 `GetLatestEcommerceApplymentBySubject(subject=merchant)`。
      - 若存在 `applyment_id` 且配置了 `ecommerceClient`：调用 `QueryEcommerceApplymentByID` 拉取微信状态 → `mapWechatApplymentStatus` 映射本地状态 →（仅当状态发生变化时）`UpdateEcommerceApplymentStatus` 回写 `status/reject_reason/sign_url/sign_state/sub_mch_id`。
      - 若微信返回 `sub_mch_id` 且商户未 `active`：写入 `merchant_payment_configs`（`CreateMerchantPaymentConfig`）并将 `merchants.status` 置为 `active`。

  - 数据层核对（SQL/约束）：
    - `ecommerce_applyments`：创建时固定 `status='pending'`；提交后用 `UpdateEcommerceApplymentToSubmitted` 置 `submitted`；状态回写走 `UpdateEcommerceApplymentStatus`（使用 `sqlc.narg` 做可选字段更新）。
    - `merchant_payment_configs`：当前激活时直接 `CreateMerchantPaymentConfig`（非 upsert）。

  - 测试覆盖现状：
    - `api/ecommerce_applyment_test.go` 已覆盖：
      - `bindbank` happy path（mock wechat：上传图片、加密敏感字段、提交进件、DB 状态更新、商户状态更新）。
      - `status` 查询的多状态分支（含“审核中/待签约/完成”等模拟）。
    - 未覆盖：异常/攻击面相关分支（外部 URL 拉取失败、超大文件/流式读取、路径穿越/SSRF、重复提交与 out_request_no 冲突、激活时 payment config 冲突等）。

  - 严重/重要问题（记录）：
    - **P0：`uploadImageToWechat` 存在高风险安全面（LFI/路径穿越 + SSRF + 无超时/无流式限流）**：
      - 本地路径分支直接 `os.Open(localPath)`，`normalizeUploadPath` 不阻止 `../`；若 `imageURL` 可被注入为 `uploads/../../../../etc/passwd` 之类路径，可能读取任意系统文件并上传到微信（数据外带）。
      - 外部 URL 分支使用 `http.Get`（无超时、无 ctx 取消传递）且 `io.ReadAll` 直接读入内存；若 `imageURL` 可控，存在 SSRF（访问内网/云元数据）与 DoS（大文件/慢速响应）风险。
    - **P1：商户进件依赖旧版商户申请数据源**：`merchantBindBank` 使用 `GetUserMerchantApplication`（旧 `/merchants/applications`）而非新版 `merchant_application` 草稿/审核链路；与前文“旧/新链路并存”的一致性风险叠加，可能导致“进件取不到数据/取到过期字段/字段口径不一致”。
    - **P1：接口契约/状态语义可能漂移**：
      - 未配置微信客户端时，响应里的 `ApplymentID` 实际是本地 `ecommerce_applyments.id`，但字段注释/语义是“微信申请单号”。
      - 同分支下本地记录仍为 `status='pending'`，响应却返回 `status='submitted'`，易造成客户端误判。
    - **P1：进件幂等键生成过弱**：`out_request_no` 由 `merchant_id + Unix秒级时间` 拼接；同秒内并发/重试可能碰撞，且未看到 DB 层唯一约束兜底（需结合迁移确认）。
    - **P2：状态回写只在“状态变化”时触发**：`getMerchantApplymentStatus` 仅在 `updateStatus != applyment.Status` 时更新 DB；若微信侧在同一状态下更新了 `sign_url/reject_reason`，本地可能长期保留旧值。
    - **P2：激活阶段一致性非事务且容错较弱**：`CreateMerchantPaymentConfig` 不是 upsert，若已存在配置会失败并仅打日志；随后仍可能把 `merchants.status` 置为 `active`，形成“商户已激活但支付配置未落库”的漂移。

  - 建议（按优先级）：
    - 先收敛图片来源与读写边界：只允许上传系统生成的 key/path；对 URL 拉取做强约束（禁内网/私网 IP、白名单域名、带超时与大小上限的流式读取）；本地读取必须做路径清理与“限制在 uploads 根目录”校验。
    - 统一进件数据源：明确以新版 `merchant_application` 的“approved 记录”作为唯一真源（并对缺字段给出明确错误/补材料入口），避免旧表导致的不可解释失败。
    - 对齐响应契约：明确 `ApplymentID` 是“微信 applyment_id”还是“本地记录 id”，并让本地 `status` 与返回一致；必要时新增字段区分 `local_applyment_id`。
    - 加固幂等：`out_request_no` 建议使用 UUID/纳秒+随机后缀，并在 DB 侧加唯一约束；重复提交应能返回“已有进行中申请”的幂等响应。
    - 状态回写改为“字段变化即回写”：即便状态不变也可更新 `sign_url/reject_reason/sign_state/sub_mch_id`（结合 `sqlc.narg`），避免客户端拿到陈旧签约链接。
    - 激活写入加事务/幂等：对 `merchant_payment_configs` 用 upsert（或先 `Get` 再 `Update`）并与 `UpdateMerchantStatus(active)` 放入同一事务，确保激活闭环一致。

  **结论记录（商户财务·链路核对：概览→明细→服务费→满返→日汇总→结算）**
  - 端到端链路（路由→handler→DB）：
    - 路由装配：`/v1/merchant/finance/*`（见 `api/server.go` 的 `merchantFinanceGroup`），包含：
      - `GET /overview`（财务概览）
      - `GET /orders`（订单收入明细，基于分账订单）
      - `GET /service-fees`（服务费明细，按日期+来源聚合）
      - `GET /promotions`（满返/运费优惠支出明细，基于订单）
      - `GET /daily`（每日财务汇总，按日聚合）
      - `GET /settlements`（结算记录/分账订单列表，支持 status 过滤）
    - 统一前置：
      - query 参数 `start_date/end_date` 必填（YYYY-MM-DD）→ `validateDateRange(..., 365)` 限制最大跨度 → `end_date` 归一化到当日 23:59:59.999...
      - `GetMerchantByOwner(user_id)` 获取商户，再用 `merchant.ID` 进行聚合查询。
    - 概览（`getMerchantFinanceOverview`）：
      - `GetMerchantFinanceOverview`（来自 `profit_sharing_orders`）统计完成/待处理订单数、GMV、商户收入、平台/运营商服务费、待结算收入。
      - `GetMerchantPromotionExpenses`（来自 `orders`）统计“有运费优惠”的订单数与优惠总额；计算 `NetIncome = TotalIncome - TotalPromotionExp`。
    - 明细（`listMerchantFinanceOrders`）：
      - `ListMerchantFinanceOrders`（JOIN `payment_orders` 补 `order_id`）+ `CountMerchantFinanceOrders`，分页返回。
    - 服务费/日汇总/结算：
      - 服务费与日汇总基于 `profit_sharing_orders(status='finished')` 做聚合；结算列表直接分页展示 `profit_sharing_orders`（可按 status 过滤），并额外查询 `GetMerchantProfitSharingStats`（仅 finished）做汇总。

  - 关键依赖（资金口径的上游来源）：
    - `profit_sharing_orders` 由 worker 在支付成功后创建（见 `worker/task_process_payment.go` 的分账订单创建与状态推进），商户财务模块本质上是“分账订单的聚合展示”。

  - 测试覆盖现状：
    - `api/merchant_finance_test.go` 覆盖 overview/orders/service-fees/promotions/daily/settlements 的 OK、未授权、日期参数错误等主要分支。

  - 严重/重要问题（记录）：
    - **P1：NotFound 错误判定类型不一致，真实环境可能 404 变 500**：多个 handler 使用 `errors.Is(err, sql.ErrNoRows)` 判断“商户不存在”，但 store 实际基于 pgx（通常返回 `pgx.ErrNoRows`）；单测通过 stub `sql.ErrNoRows` 掩盖了该问题。
    - **P1：财务数据权限边界可能过宽**：`GetMerchantByOwner` 会匹配任意 `merchant_staff(status=active)`，且 `/merchant/finance/*` 路由组未加 `MerchantStaffMiddleware(owner/manager)`；若业务期望“仅老板/店长可见财务”，当前实现会让普通员工也可访问 GMV/佣金等敏感数据。
    - **P2：状态/口径口语化可能引发误解**：
      - 概览里 `PendingOrders` 仅统计 `profit_sharing_orders.status='pending'`，未纳入 `processing` 等中间态；对“待结算/处理中”的定义需在产品侧明确。
      - `NetIncome = merchant_amount - delivery_fee_discount` 的口径需要确认：当前“收入”来自 `profit_sharing_orders.merchant_amount`（由支付后分账计算得出），而“满返支出”来自 `orders.delivery_fee_discount`（运费优惠）。若运费优惠由平台承担或已体现在订单实收里，可能导致重复扣减或口径漂移。

  - 建议（按优先级）：
    - 统一 ErrNoRows 判定：建议统一使用 `db.ErrRecordNotFound`（或 `pgx.ErrNoRows`）作为 NotFound 的单一来源，并同步修正测试 stub 类型，避免“测得过、线上错”。
    - 明确财务权限模型：若财务仅 owner/manager 可见，应在路由组加 `MerchantStaffMiddleware("owner","manager")` 或等价的 Casbin 规则；若允许员工可见，需在文档/前端权限矩阵里明确。
    - 财务口径出一页定义表：明确 GMV/收入/服务费/满返支出/净收入的计算公式与数据来源表，避免后续对账争议；必要时把“平台承担 vs 商户承担”的优惠成本拆分存储。

  **结论记录（商户统计·链路核对：日报/概览/排行/客户/多维分析）**
  - 端到端链路（路由→handler→DB）：
    - 路由装配：`/v1/merchant/stats/*`（见 `api/server.go` 的 `merchantStatsGroup`），包含：
      - `GET /daily`（商户日报：按日聚合订单/销售额/佣金，拆分外卖/堂食单量）
      - `GET /overview`（概览：总天数/订单数/销售额/佣金/日均）
      - `GET /dishes/top`（菜品销量排行：基于 `order_items` 聚合）
      - `GET /customers`、`GET /customers/{user_id}`（顾客列表与详情：含电话、金额、偏好菜品）
      - `GET /hourly`（按小时分布）、`GET /sources`（订单类型分布）、`GET /repurchase`（复购率）、`GET /categories`（分类销售统计）
    - 通用前置：大多数接口接收 `start_date/end_date`（YYYY-MM-DD）→ `validateDateRange(...,365)` → `GetMerchantByOwner(user_id)` → 以 `merchant.ID` 做查询；顾客列表不带日期范围。
    - 数据层：聚合主要基于 `orders/order_items/users/dishes/dish_categories`（见 `db/query/merchant_stats.sql`），订单状态口径普遍过滤 `status IN ('user_delivered','completed')`。

  - 测试覆盖现状：
    - `api/merchant_stats_test.go` 覆盖上述接口的 OK/未授权/参数错误/部分 NotFound 与内部错误分支；但大量 NotFound stub 使用 `sql.ErrNoRows`，与真实 pgx 行为存在偏差。

  - 严重/重要问题（记录）：
    - **P1：日期范围的 end_date 存在“少算一天”的风险**：多个接口将 `end_date` 解析为当天 00:00:00 并直接用于 `created_at <= end_date`（SQL 也是 `<= $3`）；若客户端仅传日期（无时间），会导致 end_date 当天除 0 点外的订单全部被排除。财务模块已采用“end_date 归一化到当日结束”，统计模块建议对齐同一口径。
    - **P1：NotFound 错误判定类型不一致，真实环境可能 404 变 500**：本模块同样使用 `errors.Is(err, sql.ErrNoRows)` 判断“商户/顾客不存在”，但 store 基于 pgx；测试用例通过 stub `sql.ErrNoRows` 掩盖了该问题。
    - **P1：顾客接口返回 PII（手机号）且权限边界可能过宽**：`/merchant/stats/customers*` 直接返回 `users.phone/full_name` 与消费统计；而 `GetMerchantByOwner` 支持任意 active staff 关联的商户，且该路由组未加 `MerchantStaffMiddleware(owner/manager)`。若业务不允许普通员工访问用户手机号，这属于高风险合规/隐私问题。
    - **P2：分类统计字段语义疑似错配**：`getMerchantDishCategoryStats` 响应里 `order_count` 实际填充的是 `dish_count`（“分类下不同菜品数”），容易造成 BI 口径误解。
    - **P2：订单状态口径可能漏算**：统计 SQL 多处只纳入 `('user_delivered','completed')`，若业务存在 `delivered` 或其他终态，会导致统计偏小（需与订单状态机最终口径对齐）。

  - 建议（按优先级）：
    - 统一日期口径：所有“按日期范围”接口都将 `end_date` 归一化到当日结束（或改为 `created_at < end_date+1day` 的半开区间），避免少算一天。
    - 统一 ErrNoRows 判定：同商户财务，改用 `db.ErrRecordNotFound/pgx.ErrNoRows` 并同步测试 stub。
    - 明确顾客数据权限与脱敏策略：若必须展示手机号，建议仅 owner/manager 可见；否则提供脱敏版本（如 `138****1234`）或仅展示内部 customer_id。
    - 修正字段语义：分类统计建议把“订单数/菜品数/销量”拆成明确字段，避免用 `order_count` 承载 `dish_count`。

### 3.3 平台侧/运营后台
- [x] Boss：`api/boss.go`
- [x] 平台统计：`api/platform_stats.go`
- [x] 运营申请：`api/operator_application.go`
- [x] 运营统计：`api/operator_stats.go`
- [x] 运营-商户-骑手关系：`api/operator_merchant_rider.go`
- [x] Billing Group：`api/billing_group.go`

**结论记录（平台侧/运营后台·第一轮：路由边界→handler→SQL→测试）**

- 路由与鉴权边界（入口事实）：
  - `api/server.go`：
    - Boss：`POST /v1/claim-boss`（任意登录用户）、`GET /v1/boss/merchants`（登录用户）；店主侧 Boss 管理挂在 `/v1/merchant/*` 并由 `MerchantStaffMiddleware("owner")` 保护。
    - 运营商统计与运营侧商户/骑手管理：`/v1/operator/*` 由 `CasbinRoleMiddleware(RoleOperator)` + `LoadOperatorMiddleware()` 保护；运营财务另走 `/v1/operators/me/*` 但同样叠加 operator role + load。
    - 平台统计：`/v1/platform/stats/*` 由 `CasbinRoleMiddleware(RoleAdmin)` 保护。
    - Billing Group：`/v1/billing-groups/*` 仅在 `authGroup`（登录）下，无额外成员/会话归属校验。

- 端到端链路（典型链路抽样）：
  - 平台统计：`GET /v1/platform/stats/*`（`api/platform_stats.go`）解析 `start_date/end_date`（YYYY-MM-DD）→ `validateDateRange(...,365)` → 调用 `db/query/platform_stats.sql`（orders 口径，普遍 `created_at >= $1 AND created_at <= $2` + 状态过滤）。
  - 运营 BI：`GET /v1/operator/*`（`api/operator_stats.go`）解析日期→ `validateDateRange` → 从 middleware 获取 operator/region → SQL `db/query/operator_stats.sql` 基于 `profit_sharing_orders(status='finished')` 实时统计佣金/GMV。
  - 运营商商户/骑手管理：`GET/POST /v1/operator/merchants|riders*`（`api/operator_merchant_rider.go`）从 context 获取 operator.region_id 进行 region 过滤或事后校验；骑手详情对身份证号做脱敏输出（`maskIDCard`）。
  - Billing Group：`POST /v1/billing-groups` 创建账单组并写 member；`POST /v1/billing-groups/:id/join` 加入；`GET /v1/billing-groups?dining_session_id=...` 列表；`GET /v1/billing-groups/:id/orders` 列订单（对应 `db/query/billing_group.sql`）。
  - Boss 认领：`POST /v1/merchant/boss-bind-code`（owner）生成/复用 24h bind code → `POST /v1/claim-boss` 校验 bind code → 创建 boss 关系并尝试补 user role。

- 测试覆盖现状：
  - `api/operator_stats_test.go`：覆盖区域统计/排行/趋势等的 OK/鉴权/参数错误/范围超限；但未覆盖“end_date 日期口径”的边界语义与少算一天风险。
  - `api/operator_merchant_rider_test.go`：覆盖 operator 侧商户/骑手管理主要接口的 OK/鉴权/部分 NotFound。
  - 未见：Boss/platform_stats/billing_group 的直接单测（或缺少关键权限/越权用例）。

- 严重/重要问题（记录）：
  - **P0：Billing Group 存在明显的越权读取/加入面**
    - 现象：`listBillingGroups` 仅按 `dining_session_id` 列出账单组，未校验“当前用户是否属于该 dining session 或该账单组”；`listBillingGroupOrders` 仅按 group_id 列出订单，同样缺少成员校验；`joinBillingGroup` 可对任意 id 尝试加入（只要 group 未关闭）。
    - 影响：任意已登录用户在知道/猜测 `dining_session_id` 或 `billing_group_id` 的情况下，可枚举账单组与订单信息（金额/状态等）；也可能加入不属于自己的账单组。
    - 证据链：`api/server.go` billing-groups 路由仅挂 `authGroup`；`db/query/billing_group.sql` 已存在 `GetActiveBillingGroupMember` 查询但 handler 未使用。

  - **P1：平台/运营统计日期范围存在“少算一天”的一致性风险（跨模块 BI 口径问题）**
    - 现象：多个统计接口把 `end_date` 按日期解析为当天 00:00:00，并在 SQL 中使用 `created_at <= end_date`（如 `db/query/platform_stats.sql`、`db/query/operator_stats.sql`）。
    - 影响：当客户端按“日期”理解 end_date（例如 2025-01-31 代表包含 1/31 全天）时，实际会漏掉 end_date 当天除 00:00 之外的所有数据；且与 `getOperatorFinanceOverview` 中“monthEnd=23:59:59”的口径不一致。

  - **P1：NotFound 错误类型判定与真实 pgx 行为可能不一致**
    - 现象：`api/operator_stats.go`、`api/operator_merchant_rider.go` 多处用 `errors.Is(err, sql.ErrNoRows)`；但部分模块/handler 又使用 `pgx.ErrNoRows`（例如 `api/billing_group.go`）。测试 stub 普遍以 `sql.ErrNoRows` 驱动分支，存在“测得过但线上漂移”的风险。

  - **P1：运营侧接口直接返回 PII 字段，需明确最小化/脱敏策略**
    - 现象：operator 侧商户列表/详情返回 `phone/address/owner_user_id`；骑手列表/详情返回 `phone`（身份证已脱敏）。
    - 影响：虽由 operator role 保护，但仍属于敏感数据面扩大；需要明确“运营账号的最小权限 + 审计”与脱敏策略。

  - **P2：Boss 认领链路存在一致性缺口与权限/角色语义漂移风险**
    - 现象：`claimBoss` 创建 boss 关系后尝试写 `user_roles`，但写入失败当前未阻断主流程（不回滚/不报错），可能造成“Boss 关系存在但角色列表不一致”。

- 建议（按优先级）：
  - Billing Group 强制成员校验：对 `listBillingGroups/listBillingGroupOrders/joinBillingGroup` 在 handler 前置校验“当前用户属于该 session 或已是 group member”，并用 `GetActiveBillingGroupMember` 作为 DB 兜底；必要时增加唯一约束/幂等键避免重复加入。
  - 统一日期口径：建议全局改为半开区间 `[start, end+1day)`（或把 end_date 归一化到当日结束），并在平台/运营统计与财务保持一致。
  - 统一 NotFound 判定与测试 stub：明确唯一 ErrNoRows 来源（例如 `pgx.ErrNoRows` 或 `db.ErrRecordNotFound`），并同步更新所有相关测试。
  - 运营侧 PII 最小化：按角色/审计需要决定是否返回手机号/地址；若必须返回，建议默认脱敏并对 owner/manager/operator 管理员进行差异化授权。

### 3.4 商品与资源管理（菜品/组合/库存/后厨/桌台）
- [x] 菜品：`api/dish.go`
- [x] 菜品上传：`api/dish_upload.go`
- [x] 组合套餐：`api/combo.go`
- [x] 标签：`api/tag.go`
- [x] 库存：`api/inventory.go`
- [x] 后厨：`api/kitchen.go`
- [x] 桌台：`api/table.go`
- [x] 桌台上传：`api/table_upload.go`
- [x] 桌台预订：`api/table_reservation.go`
- [x] 就餐会话：`api/dining_session.go`

**结论记录（商品与资源管理·第一轮：路由→鉴权→handler→SQL→测试）**

- 路由与鉴权边界（入口事实）：
  - 标签：`GET/POST /v1/tags`（`api/server.go` 的 `tagsGroup`）挂在 `authGroup` 下，仅要求登录；`createTag` 注释称“管理员”但未见权限校验。
  - 菜品：`/v1/dishes/*`（含分类、CRUD、上下架、定制、图片上传）均挂 `authGroup`。
  - 套餐：`/v1/combos/*` 挂 `authGroup`。
  - 库存：`/v1/inventory/*`（含 `POST /check` 扣减）挂 `authGroup`。
  - 桌台/包间：`/v1/tables/*`（含图片、标签、二维码）挂 `authGroup`；预订 `/v1/reservations/*` 与就餐会话 `/v1/dining-sessions/*` 同样挂 `authGroup`。
  - 扫码点餐：`GET /v1/scan/table`（`api/scan.go`）为 public（不要求登录）。
  - 搜索：`GET /v1/search/dishes|combos|merchants`（`api/search.go`）为 public（不要求登录）。

- 端到端链路要点（抽样到“可复核的安全/一致性点”）：
  - 上传一致性：`dish_upload/table_upload` 上传前调用 `wechatClient.ImgSecCheck`，通过后落盘；入库/更新图片 URL 时普遍要求 `uploads/public/merchants/{merchant_id}/...` 前缀白名单，并对返回给客户端做 `normalizeUploadURLForClient`。
  - 菜品：`dish.go` 侧大多以 `GetMerchantByOwner(user_id)` 绑定商户上下文，再对 `dish.merchant_id` 做归属校验；分类/定制/上下架走 sqlc（`db/query/dish*.sql`）并在 handler 层做参数与状态约束。
  - 套餐：`combo.go` 创建时逐个 `GetDish` 校验菜品归属，写入通过 `CreateComboSetTx`；读取 `GetComboSetWithDetails`（SQL `json_agg/jsonb_build_object` 聚合 dishes/tags）后在 handler 层再次校验 `combo.merchant_id`；更新/上下架/删除同样显式校验归属。
  - 桌台：`table.go` 多处使用 `db.ErrRecordNotFound`（更接近 pgx/sqlc 基线）；图片/标签/删除（含未来预订检查）走 tx（如 `DeleteTableTx`），并对 image_url 做 uploads 白名单校验。
  - 预订：`table_reservation.go` 创建含时段冲突检测（SQL `OVERLAPS` + 固定时长）、支付/退款窗口与“全款预点菜”校验；`getReservation` 有“用户本人 / 商户 owner”双路径鉴权。
  - 后厨（KDS）：`kitchen.go` 以 `GetMerchantByOwner` 获取商户，再通过 `order.merchant_id` 校验归属；状态推进仅允许 `paid→preparing→ready`。

- 严重/重要问题（记录）：
  - **P0：就餐会话 `open` 缺少“桌台口令/会话参与者”校验，可能被远程加入并联动 Billing Group 越权**
    - 现象：`POST /v1/dining-sessions/open` 仅要求登录；当桌台存在进行中的 session 时，调用方可在缺少“物理在场证明/桌台 secret”的情况下被加入默认 billing group（并可能进一步访问账单组/订单信息）。
    - 影响：与 3.3 已记录的 Billing Group 越权面叠加，形成“知道 table_id/merchant_id+table_no 即可远程入组/枚举账单与订单”的风险链路。
    - 证据链：`api/server.go` 的 `diningSessionsGroup` 仅挂 `authGroup`；`api/dining_session.go` 的 open 逻辑会 `getOrCreateDefaultBillingGroup` 并写 `billing_group_members`。

  - **P0：库存 `checkAndDecrementInventory` 在“无库存记录”分支返回 unlimited（Available=true, stock=-1）与 SQL 语义冲突**
    - 现象：SQL 语义为 `total_quantity=-1` 才代表无限库存（见 `db/query/inventory.sql`）；但 handler 在 ErrNoRows 分支会把“无记录”解释成“无限库存”。
    - 影响：可能导致超卖/错卖并引发资金/履约事故；同时会掩盖库存配置缺失。

  - **P1：标签创建接口未实现管理员权限（注释与实现不一致）**
    - 现象：`POST /v1/tags` 注释称管理员，但 `api/tag.go:createTag` 仅验证登录，直接 `CreateTag(status=active)`。
    - 影响：任意登录用户可污染标签体系（排序/类型），影响搜索/推荐/展示与运营治理。

  - **P1：ErrNoRows 类型不统一的新增证据（3.4 仍在延续）**
    - 现象：`kitchen.go`、`inventory.go` 等仍用 `errors.Is(err, sql.ErrNoRows)`；而 store 基于 pgx/sqlc，且项目已定义 `db.ErrRecordNotFound = pgx.ErrNoRows`。
    - 影响：线上“应返回 403/404/409”可能漂移为 500；单测 stub `sql.ErrNoRows` 会进一步掩盖。

  - **P1：public 扫码接口返回面偏大（枚举/爬取/联动风险）**
    - 现象：`GET /v1/scan/table` 为 public，响应包含 merchant phone/address、完整菜单/套餐/活动信息，并可返回 table 相关数据。
    - 影响：可被批量爬取；与“open dining session + billing group”链路叠加时，远程枚举与加入风险更高。

  - **P2：套餐更新的关联写入一致性非事务**
    - 现象：`updateComboSet` 采取“先 RemoveAll 再逐个 Add”的多步写入策略；若中途失败可能短暂不一致。
    - 影响：数据一致性与回滚成本提升（尤其在并发读场景）。

- 建议（按优先级）：
  - P0：为 `dining-sessions/open` 引入“桌台 secret/一次性码/带签名的扫码凭证”，并在服务端校验；同时在 billing group 相关接口强制成员校验（与 3.3 风险合并治理）。
  - P0：把“无库存记录”与“无限库存(total=-1)”严格区分；缺记录应返回 unavailable/需要配置（并补单测覆盖）。
  - P1：给 `POST /v1/tags` 加管理员权限（Casbin 或 admin role），或至少降为“仅 merchant/operator 可创建”。
  - P1：统一 NotFound 判定与测试 stub，收敛到 `db.ErrRecordNotFound`。
  - P1：评估 `scan/table` 返回字段最小化 + 限流/防爬（敏感字段脱敏/按需返回）。

### 3.5 交易链路（购物车/下单/支付/回调）
- [x] 购物车：`api/cart.go`
- [x] 订单：`api/order.go`
- [x] 支付下单：`api/payment_order.go`
- [x] 支付回调：`api/payment_callback.go`
- [x] 配送费：`api/delivery_fee.go`
- [x] 配送：`api/delivery.go`

**结论记录（交易链路·第一轮深挖：订单/支付/回调/支付成功任务）**
- 端到端链路（资金→状态机→履约）：
  - `POST /v1/orders`（`createOrder`）创建 `orders` + `order_items`，可选绑定 `billing_group_orders`，并在同一事务内核销优惠券/扣会员余额（`CreateOrderTx`）。
  - `POST /v1/payment-orders`（订单业务）创建 `payment_orders(status=pending)`，微信 JSAPI 下单；支付超时会入队关闭支付单任务。
  - 微信回调 `POST /v1/webhooks/wechat/pay`：验签/解密/幂等（notification_id 去重）/金额校验 → 将 `payment_orders` 置为 `paid` → 入队 `TaskPaymentSuccess`。
  - `TaskPaymentSuccess(business_type=order)`：事务性推进订单履约（扣库存、标记订单 paid、必要时创建配送单并推入配送池），再发送通知、可选触发分账任务。

  - 高优先级问题（发现）：
    - **订单支付履约推进的幂等性缺口**：`ProcessOrderPaymentTx` 在任务重试/重复执行时，可能重复扣减库存（尤其是存在 `daily_inventory` 行时），并可能导致重复通知/重复触发分账。
    - 建议修复方向：
      - 事务入口先对 `orders` 行 `FOR UPDATE` 加锁；若订单状态已是 `paid` 则幂等返回（不做扣库存/建配送单等副作用）。
      - worker 层对“是否本次真正推进 paid”做 gating：只有首次推进时才发送通知/触发分账。
      - 补充幂等单测：构造有库存行的订单，重复执行支付履约推进，断言 `sold_quantity` 只增长一次。

- 仍需确认/建议（待后续扩展到配送、购物车等）：
    - 订单状态日志：需要盘点并确认“支付成功（pending -> paid）”以及后续 `paid -> preparing/ready/...` 是否全链路一致写入状态日志（建议在 Tx 内原子写入）。
  - 退款闭环：worker 已识别“库存不足需要自动退款”的场景，但当前仅打日志（标注 pending），建议补齐“自动退款任务 + 状态回写 + 用户/商户通知 + 库存/配送池回滚策略”。
    - 分账幂等：建议在“任务层防重复”之外，在分账订单表侧再加唯一约束/幂等键（如 `payment_order_id` 唯一）作为最后防线。

**结论记录（购物车·链路核对：增/改/删/清空/计价/合单预览）**
- 端到端链路：
  - 获取购物车：`GET /v1/cart` → `GetCartByUserAndMerchant` → `ListCartItems`（JOIN dishes/combo_sets）→ 组装响应（含单项可售、价格、图片）。
  - 加购：`POST /v1/cart/items` → 校验 merchant/dish/combo 可售 → `CreateCart(ON CONFLICT…)` → 通过“dish+customizations / combo”查重并增量更新 → `AddCartItem/UpdateCartItem`。
  - 改数量/定制：`PATCH /v1/cart/items/{id}` → `GetCartItem` → `GetCart` 校验 `cart.user_id` → `UpdateCartItem`。
  - 删单品：`DELETE /v1/cart/items/{id}` → 同上校验所有权 → `DeleteCartItem`。
  - 清空：`POST /v1/cart/clear` → `GetCartByUserAndMerchant` → `ClearCart`。
  - 计价：`POST /v1/cart/calculate` → `GetCartByUserAndMerchant + ListCartItems` → 可选调地图算距离 → `calculateDeliveryFeeInternal` → 可选校验 `GetUserVoucher` 计算优惠 → 返回（含 ETA 估算）。
  - 合单预览：`POST /v1/cart/combined-checkout/preview` → `GetUserCartsByCartIDs + ListCartItems` → 对每个商户可选计算配送费 → 返回合计。

- 严重/重要问题（记录）：
  - `order_type/table_id/reservation_id` 组合缺少“枚举与关联约束”的强校验：当前接口允许任意 `order_type` 字符串进入 DB（unique key 包含该字段），容易产生不可预期的“同用户同商户多购物车分叉”，且堂食/预订场景对 `table_id/reservation_id` 的必填约束仅在注释里未强制执行。
  - 定制选项去重存在边界：同一菜品“未传 customizations（NULL）”与“传空对象 {}（JSONB）”会被视为不同，可能出现同款商品被拆成两条 cart_items 行（影响 UI 合并、结算校验与库存预扣的可解释性）。
  - `calculateCart` 在配送费计算失败/配送暂停时，会静默返回 `delivery_fee=0` 且无显式提示字段，存在“用户误以为免配送费/可配送”的产品与履约风险。
  - 代码里存在直接 `fmt.Printf` 的调试输出（`getUserCartsSummary`），会污染日志与线上排障信噪比。

- 建议（待实现）：
  - 对 `order_type` 做统一枚举与验证（至少 takeout/dine_in/reservation），并对堂食/预订补齐 `table_id/reservation_id` 的条件必填校验。
  - 统一定制选项的“空值语义”：将空 map 归一化为 nil，或在 SQL 比较时对 `customizations` 统一 `COALESCE(customizations,'{}'::jsonb)` 以实现预期去重。
  - 配送费计算需要可解释输出：当不可算/暂停配送时，建议返回明确的状态字段（例如 `delivery_available=false`、原因码），避免默认为 0。
  - 清理 debug 输出，统一用结构化日志（并避免打印经纬度等敏感信息的原始值）。

**结论记录（配送费·链路核对：配置/高峰/促销/计算）**
- 端到端链路：
  - 区域运费配置：`POST /delivery-fee/regions/{region_id}/config`（operator）→ `delivery_fee_configs`（唯一 `region_id`）
    - 查询：`GET /delivery-fee/config/{region_id}` → `GetDeliveryFeeConfigByRegion`。
    - 更新：`PATCH /delivery-fee/regions/{region_id}/config`（operator）→ `UpdateDeliveryFeeConfig`。
  - 高峰时段：`POST/GET /operator/regions/{region_id}/peak-hours`、`DELETE /operator/peak-hours/{id}` → `peak_hour_configs`。
  - 商户满返：`POST/GET/DELETE /delivery-fee/merchants/{merchant_id}/promotions...` → `merchant_delivery_promotions`。
  - 运费计算：
    - 外部 API：`POST /delivery-fee/calculate`：要求 `delivery_fee_configs` 存在且 `is_active=true`，否则 404/400。
    - 内部调用：`calculateDeliveryFeeInternal`：配置缺失时降级到默认基础运费；天气/高峰系数尽力获取（失败不报错）。

- 严重/重要问题（记录）：
  - 路由契约与绑定不一致（高风险）：
    - `createDeliveryFeeConfig` 的路由包含 `{region_id}`，但实现未 `ShouldBindUri`，完全依赖 body 的 `region_id`；若中间件按 URI 校验权限，这里存在“URI 与 body 不一致”的歧义与潜在越权/误写风险。
    - `createDeliveryPromotion` 的路由写 `{id}`，但实现用 `uri:"merchant_id"` 绑定；这会导致绑定失败或拿不到商户ID（接口不可用/行为不确定）。
  - 峰值配置缺少“时段不重叠”校验：迁移注释声明“同一区域时段不允许重叠，代码层面校验”，但当前创建流程只做时间格式解析，未见重叠检测；会导致系数叠加规则不可预测。
  - `days_of_week` 语义不一致：DB 注释写 1-7（周一到周日），但 API 校验为 0-6 且计算使用 `time.Weekday()`（0=周日）；会造成配置端与执行端理解冲突。
  - 天气系数计算在缓存/DB 两条路径语义不一致：缓存路径使用 `Coefficient*WarningCoefficient`，DB 路径使用 `final_coefficient`（注释里是 `max(天气,预警)`）；暂停配送原因也被硬编码为固定文本，未使用 `suspend_reason`。
  - 促销折扣应用点分散：`calculateDeliveryFeeWithConfig` 返回 `PromotionDiscount` 但不在 `FinalFee` 中扣减（注释说由上层抵扣）。这要求调用方全部正确处理，否则会出现“展示/计价不一致”。

- 建议（待实现）：
  - 统一路由参数来源：对所有涉及 `{region_id}/{merchant_id}` 的接口，强制 `BindUri` 并校验与 body 一致（或干脆 body 不再重复传）。
  - 给 peak hour 增加不重叠约束：至少在写入前查重叠区间（含跨日），必要时用排他约束/标准化存储（将跨日拆成两段）。
  - 明确 `days_of_week` 规范并统一到文档/校验/执行三处一致。
  - 统一天气系数口径：缓存与 DB 的计算公式一致，并把 `suspend_reason` 透传给调用方。
  - 促销折扣推荐“单一事实来源”：要么 `FinalFee` 就是最终需支付运费，要么明确 `PromotionDiscount` 必须由调用方扣减，并在所有调用点加一致性测试。

**结论记录（配送·链路核对：推荐/抢单/状态机/轨迹）**
- 端到端链路：
  - 推荐可接订单：`GET /v1/delivery/recommend` → `ListDeliveryPoolNearbyByRegion`（按骑手区域过滤）→ 推荐算法（`algorithm`）→ 可选调用自建 LBS 获取真实距离。
  - 抢单：`POST /v1/delivery/grab/:order_id` → 检查 `delivery_pool` 存在/过期/区域/高值单资格 → `GrabOrderTx`（锁 rider 行、分配 delivery、从池移除、冻结押金、写押金流水）→ 再尝试更新订单状态为 `courier_accepted` + 写订单状态日志（非事务内）。
  - 配送状态机（骑手侧）：
    - `start-pickup`：assigned → picking（`UpdateDeliveryToPickupTx`）
    - `confirm-pickup`：picking → picked（`UpdateDeliveryToPickedTx`）并同步订单状态 picked
    - `start-delivery`：picked → delivering（`UpdateDeliveryToDeliveringTx`）并同步订单状态 delivering
    - `confirm-delivery`：delivering → delivered（`CompleteDeliveryTx`：解冻押金、写流水、更新统计与高值单积分）并同步订单状态 rider_delivered
  - 顾客侧查询：
    - `GET /v1/delivery/order/:order_id` 仅订单所有者可查配送单
    - `GET /v1/delivery/:delivery_id/track` / `rider-location` 仅订单所有者或该配送骑手可查轨迹/最新位置

- 严重/重要问题（记录）：
  - 抢单并发与幂等：`GrabOrderTx` 依赖 `AssignDelivery WHERE rider_id IS NULL` 实现“先到先得”，但失败会以 500 返回给客户端（实际上更像“已被接走”的 404/409 语义）。此外事务内未强校验 `delivery_pool` 行仍存在（`RemoveFromDeliveryPool` 删除 0 行不报错），存在“通过竞态窗口接走已被移除/取消的池订单”的一致性风险。
  - 订单状态更新不原子：抢单后 `UpdateOrderToCourierAccepted` 在事务外执行；若该步失败会对客户端返回 500，但 delivery 已分配/押金已冻结/订单池已移除，容易形成“用户重试导致多次副作用”的风险。建议把“delivery 状态 + order 状态 + 关键日志（至少一条）”尽量收敛到同一事务或用幂等门闩保护外部可见行为。
  - 状态机约束不一致：
    - `UpdateDeliveryToPickup` 的 SQL 缺少 `AND status='assigned'` 条件（仅校验 rider_id），目前靠 handler 先查再判断；在并发/重试下更容易出现重复推进且难以解释。
    - `confirmDelivery` handler 未做“仅 delivering 可送达”的显式前置校验，完全依赖 Tx 内 SQL 条件；一旦不满足会以 500 反馈（应为 400/409）。
  - “delivered vs completed” 未闭环：deliveries 表与 SQL 提供了 `completed` 状态与 `UpdateDeliveryToCompleted`，但当前链路只推进到 `delivered`；缺少“顾客确认收餐/平台结算完成/异常结案”等把 delivery 推到 completed 的闭环接口与对应订单状态流转。
  - expires_at 语义自相矛盾：DB 注释与查询说明“外卖订单始终可见直到被接单或取消，expires_at 不再用于可见性过滤”，但 `grabOrder` 仍用 `expires_at` 做硬过期拒绝；同时推荐/附近列表 SQL 不过滤 expires，导致“推荐列表展示可接但抢单会提示已过期”的体验与契约冲突。
  - 轨迹/围栏事件的幂等返回语义：`delivery_location_events` 以 `(delivery_id, event_type)` 唯一且 `ON CONFLICT DO NOTHING RETURNING ...`，若重复上报会返回 0 行（sqlc 多半表现为 `ErrNoRows`）。若上层未将其视为幂等成功而当成 500，会产生“正常重试被当错误”的风险（需在对应上报接口中显式处理）。

- 建议（待实现）：
  - 抢单冲突显式化：将“被抢走/不在池/已取消/已过期”与“内部错误”区分返回码（建议 409/404/400），并在 Tx 内对池行做 `SELECT ... FOR UPDATE` 或 `DELETE ... RETURNING`，确保“只有仍在池中的订单才能被接走”。
  - 状态机以 DB 为准：把关键状态迁移条件前移到 SQL `WHERE`（包括 picking 需要 assigned），并统一把“状态不允许”的场景映射为 400/409；不要用 500 表达业务拒绝。
  - 补齐 completed 闭环：明确完成条件（顾客确认/超时自动确认/平台结算），新增接口与任务把 delivery 从 delivered 推到 completed，并同步订单状态与日志。
  - 统一 expires_at 口径：若要保留“硬过期”，则推荐/附近列表 SQL 需要过滤；若不再用于可见性，则 grabOrder 不应拒绝或应采用更明确的取消/失效标记字段。
  - 通知与日志的幂等门闩：对抢单/状态推进产生的通知与状态日志，建议以“是否首次推进该状态”为条件触发，避免重试导致重复通知。

### 3.6 配送与骑手
- [x] 骑手：`api/rider.go`
- [x] 骑手申请：`api/rider_application.go`
- [x] 骑手位置事件：`api/rider_location_events.go`

**结论记录（骑手·链路核对：入驻/押金/上线/位置/积分/异常）**
- 端到端链路：
  - 入驻：`POST /v1/rider/apply`（`api/rider.go`）直接 `CreateRider` 生成 riders 记录（status 初始为 pending，后续管理员 approve/reject）。
  - 押金充值：`POST /v1/rider/deposit` 创建 `payment_orders(business_type=rider_deposit,status=pending)` 并微信 JSAPI 下单；支付回调 `POST /v1/webhooks/wechat-pay/notify` 将 `payment_orders` 置为 `paid` 并入队 `TaskPaymentSuccess`；worker 侧 `ProcessTaskPaymentSuccess` → `handleRiderDepositPaid` 执行 `UpdateRiderDeposit` + `CreateRiderDeposit(type=deposit)`。
  - 押金提现：`POST /v1/rider/withdraw` 先 `WithdrawDepositTx`（`FOR UPDATE` 锁 rider、校验可用余额、扣减 deposit_amount、写 `rider_deposits(type=withdraw)`），再调用微信提现；失败走 `RollbackWithdrawTx` 写 `withdraw_rollback` 流水恢复余额。
  - 上下线：`/v1/rider/online|offline|status` 以“无进行中配送 + 可用押金阈值 + rider.status=active”为主要门槛。
  - 位置上报：`POST /v1/rider/location` 批量写 `rider_locations` + 更新 riders 最新经纬度，并在有活跃配送时触发围栏事件处理（见下条“骑手位置事件”）。
  - 高值单资格：`GET /v1/rider/score`/`history` 读取 `rider_profiles.premium_score` 与变更日志。
  - 延时/异常上报：`/rider/orders/{id}/delay|exception` 目前实现以写 `order_status_logs` + 异步通知为主（且 Swagger 路由前缀与其他接口不一致，需与路由装配核对）。

- 严重/重要问题（记录）：
  - **P0：骑手押金充值入账幂等性缺口（高风险资金类）**：`payment_orders.status` 在回调后长期停留在 `paid`；worker 的 `handleRiderDepositPaid` 仅检查“是否 paid”，没有任何“已处理门闩/唯一约束/关联键”。任务重复投递或重试会重复加余额、重复写 `rider_deposits(type=deposit)`。
  - **充值入账缺少对账主键**：`rider_deposits` 没有 `payment_order_id/out_trade_no` 级别的强关联字段与唯一约束，后续对账、排障、追溯都会困难；也难以从 DB 层强制幂等。
  - worker 入账未加锁且非事务：`handleRiderDepositPaid` 直接读 rider 后计算新余额再更新，缺少 `FOR UPDATE`/Tx，存在并发写覆盖风险（与提现/冻结等并发场景下更明显）。
  - 提现失败回滚使用请求 ctx：回滚发生在 handler 内且沿用原请求 ctx；若客户端断连/超时导致 ctx cancelled，回滚可能失败，留下“余额已扣但未到账”的资金一致性风险。
  - 入驻链路存在“双轨”且语义冲突：同一项目里同时存在 `POST /v1/rider/apply`（直接建 riders）与“rider_applications 草稿/材料/审核”链路（见下一节），会造成重复入口、状态机不一致、运维/客服无法解释。

- 建议（待实现）：
  - 充值入账做强幂等：引入“处理门闩”（例如 `payment_orders.processed_at/processed_status` 或单独 `payment_process_logs(payment_order_id unique)`），并在入账时以 Tx + `FOR UPDATE` 保证一次性扣改。
  - 给 `rider_deposits` 增加对账关联：至少增加 `related_payment_order_id`（或复用 `related_order_id` 的语义但更推荐独立字段），并建立唯一约束防止重复入账。
  - 提现回滚使用独立 context：回滚建议使用 `context.Background()` 或带超时的新 ctx，避免被请求取消影响补偿动作。
  - 合并/收敛入驻入口：选择“rider_applications 申请表”为唯一入口，`/v1/rider/apply` 走废弃或改为管理员/内部接口，确保状态机只有一个来源。

**结论记录（骑手申请·链路核对：草稿→材料→提交→自动审核→创建骑手）**
- 端到端链路：
  - 草稿：`GET /v1/rider/application` → `GetRiderApplicationByUserID` 不存在则 `CreateRiderApplication(status=draft)`。
  - 更新基础信息：`PUT /v1/rider/application/basic`（仅 draft）→ `UpdateRiderApplicationBasicInfo WHERE status='draft'`。
  - 上传身份证并 OCR：`POST /v1/rider/application/idcard/ocr`（仅 draft）→ 图片内容安全检测 → 保存本地 → 调用微信 OCR → `UpdateRiderApplicationIDCard WHERE status='draft'`（写 URL/OCR JSON，并可自动填 real_name）。
  - 上传健康证：`POST /v1/rider/application/healthcert`（仅 draft）→ 内容安全检测 → 保存 → 通用印刷体 OCR + 规则解析 → `UpdateRiderApplicationHealthCert WHERE status='draft'`。
  - 提交：`POST /v1/rider/application/submit`（仅 draft）→ `SubmitRiderApplication(draft→submitted)` → 自动 `ApproveRiderApplication` 或 `RejectRiderApplication`（两者都要求 `WHERE status='submitted'`）→ 通过后尝试从申请创建 `riders` 记录。
  - 重置：`POST /v1/rider/application/reset`（仅 rejected）→ `ResetRiderApplicationToDraft WHERE status='rejected'`。

- 严重/重要问题（记录）：
  - **P0：审核通过与创建 rider 未形成原子闭环**：自动通过路径先把申请置为 `approved`，再在事务外创建 `riders`；若 `CreateRider` 因唯一约束/DB 错误失败，当前只打日志不回滚申请状态，会出现“申请已通过但没有骑手档案”的卡死状态。
  - migration 已引入 `riders.application_id` 与唯一索引，但当前创建 rider 并未填充该字段（实现仅日志里说“通过 user_id 关联，简化处理”），属于“数据库设计意图与实现脱节”。
  - 状态并发下错误码不友好：handler 先读 `app.Status` 再调用 `Submit... WHERE status='draft'`；若并发提交导致 SQL 返回 0 行，容易被当成 500（应为 400/409 语义）。
  - 自动审核策略过于“硬拒绝”：OCR/规则解析天然存在误识别；当前不满足即直接 `rejected`，会放大误杀与客服成本。更合理的是“不通过则转人工审核/补材料”，而不是直接拒绝。
  - 与 `POST /v1/rider/apply` 的重复入口（见上一节）会导致产品/运营流程难以解释：到底以申请表为准还是以 riders 表为准。

- 建议（待实现）：
  - 把“Approve + CreateRider + 回填 application_id（以及必要的 rider 初始状态）”收敛到同一 Tx，保证要么全成功、要么全失败；并为“已存在 rider”提供幂等行为（例如直接回填 application_id 或返回 409）。
  - 对自动审核做降级：不满足条件时优先置为 `submitted` 等待人工审核，或增加 `needs_manual_review` 之类状态，避免直接 `rejected`。
  - 用测试覆盖资金/状态机关键点：目前 DB 层对提现/回滚有单测，但 rider_deposit 支付成功入账（worker）缺少幂等与并发测试，属于高优先级补齐项。

**结论记录（骑手位置上报/围栏事件·链路核对：上报→轨迹→围栏→自动推进）**
- 端到端链路：
  - 位置上报入口实际在 `POST /v1/rider/location`（实现位于 `api/rider.go`），批量写入 `rider_locations`（`BatchCreateRiderLocations` CopyFrom）并更新 riders 的 `current_longitude/current_latitude/location_updated_at`。
  - 若骑手存在活跃配送单，会在上报后触发 `processDeliveryLocationEvents`（`api/rider_location_events.go`）：
    - 按 delivery 状态选择围栏目标（商家取餐点/顾客收货点）
    - 满足精度阈值与半径阈值后写入 `delivery_location_events(arrive_*)`
    - 若满足驻留规则（窗口内样本数+最短驻留时间），写入 `delivery_location_events(dwell_*)`
    - 可选自动推进：dwell_pickup → 自动开始取餐/确认取餐；dwell_dropoff → 自动确认送达（均受 config 开关控制）

- 严重/重要问题（记录）：
  - “模块归属/路由可发现性”不一致：审查项是 `api/rider_location_events.go`，但对外 API 路由与参数校验在 `api/rider.go`；容易造成“改了围栏逻辑但忘了上报入口”或 Swagger/维护人员理解偏差。建议在任务文档与代码结构上明确：该文件是 geofence 处理器（无路由）。
  - 活跃配送单选择策略可能误关联：上报时若骑手有多个活跃配送，当前取 `ListRiderActiveDeliveries` 的第一个作为 activeDelivery；会导致轨迹/围栏事件绑定到非预期 delivery。
  - 驻留判定依赖客户端 `recorded_at`：虽然限制了“未来≤5分钟/历史≤1小时”，但攻击者仍可通过构造时间序列满足驻留时长与样本数，从而触发自动取餐/自动送达；这属于“GPS/时间戳可伪造”导致的状态机被动推进风险。
  - 自动推进副作用很大：围栏一旦触发，可能推进 delivery 与 order 状态并触发通知/解冻押金结算（自动送达走 `CompleteDeliveryTx`）。需要把该功能定位为高风险可控开关，并补齐更强的条件（例如最小驻留时长、最小样本间隔、速度阈值、与手动操作互斥等）。
  - 围栏事件的幂等返回语义需要全链路一致：DB 层使用 `(delivery_id, event_type)` 唯一 + `ON CONFLICT DO NOTHING RETURNING`，重复触发会返回 0 行；当前 `createDeliveryLocationEvent` 已将 `ErrNoRows` 视为“未创建但非错误”。若未来新增“事件上报 API”，需要同样按幂等成功处理，否则重试会被误判 500。

- 建议（待实现）：
  - 明确边界与文档：在 Swagger/代码注释中标注“位置上报接口在 rider.go；rider_location_events.go 仅为围栏处理器”，避免维护漂移。
  - 活跃配送单选择与绑定规则定稿：如果允许多单并行，上报必须显式携带 `delivery_id` 且服务端校验归属；如果不允许多单并行，应在业务层强约束并将选择策略写死（且可解释）。
  - 加固驻留与自动推进：把 `recorded_at` 与服务器接收时间结合（或引入签名/设备可信度），并增加样本间隔/速度/轨迹合理性校验，降低伪造触发。

### 3.7 营销与增长
- [x] 优惠券：`api/voucher.go`
- [x] 折扣：`api/discount.go`
- [x] 收藏：`api/favorite.go`
- [x] 推荐：`api/recommendation.go`

**结论记录（营销与增长·第一轮：路由→鉴权→handler→SQL→测试）**

#### 3.7.1 路由与鉴权边界（`api/server.go`）
- 优惠券：
  - 商户侧（创建/管理）：`/v1/merchants/:id/vouchers/*`（`authGroup`，需登录）
  - 用户侧（领取/查询）：`/v1/vouchers/*`（`authGroup`，需登录）
- 折扣：`/v1/merchants/:id/discounts/*`（`authGroup`，需登录）
- 收藏：`/v1/favorites/*`（`authGroup`，需登录）
- 推荐：`/v1/behaviors/track`、`/v1/recommendations/*`（`authGroup`，需登录）
- 推荐配置（注释声称“区域运营商权限”）：`/v1/regions/:id/recommendation-config`（`authGroup`，需登录）

#### 3.7.2 优惠券（Voucher）端到端链路取证
- 领取链路：`POST /v1/vouchers/:voucher_id/claim`（`api/voucher.go`） → `store.ClaimVoucherTx`（`db/sqlc/tx_voucher.go`）
  - Tx 内先 `GetVoucherForUpdate`（`FOR UPDATE` 锁券模板）→ 校验 `is_active/valid_from/valid_until` → `CheckUserVoucherExists` 防重复领取 → `IncrementVoucherClaimedQuantity`（`claimed_quantity < total_quantity` 条件更新，天然并发门闩）→ `CreateUserVoucher(expires_at=valid_until)`。
- 使用/核销链路：订单创建 `POST /v1/orders`（`api/order.go`）校验 `GetUserVoucher`（归属/状态/有效期/merchant_id/min_order_amount/allowed_order_types）后，将 `user_voucher_id + voucher_amount` 传入 `CreateOrderTx`（`db/sqlc/tx_create_order.go`）
  - Tx 内 `GetUserVoucherForUpdate`（锁用户券）→ 再校验 `status/expiry` → **立即 `MarkUserVoucherAsUsed(status='unused' 条件更新)` 并 `IncrementVoucherUsedQuantity`**。

#### 3.7.3 折扣（Discount）端到端链路取证
- 规则 CRUD：`api/discount.go` → `db/query/discount.sql`
  - create/update/delete 走 `getMerchantIDByUser` + 校验 `rule.merchant_id`；但 list/get 存在权限与归属校验缺口（见风险）。
- 下单应用：`api/order.go` 通过 `ListActiveDiscountRules` 选“折扣金额最大”的规则；未见对 `can_stack_with_voucher/can_stack_with_membership` 的应用约束。

#### 3.7.4 收藏（Favorite）端到端链路取证
- 收藏写入：`api/favorite.go` → `db/query/favorite.sql`
  - `AddFavoriteMerchant/AddFavoriteDish` 使用 `INSERT ... ON CONFLICT ... DO NOTHING RETURNING *`，重复收藏会返回 0 行（见风险）。
- 收藏读取：`ListFavoriteMerchants/ListFavoriteDishes` 通过 join 返回商户/菜品概览；计数接口单独查询。

#### 3.7.5 推荐（Recommendation）端到端链路取证
- 行为埋点：`POST /v1/behaviors/track` → `TrackBehavior` 写入 `user_behaviors`。
- 推荐生成：`/v1/recommendations/dishes|combos|merchants` 使用 `algorithm.PersonalizedRecommender` 计算 ID 列表 → 批量拉取富字段（含商户信息/标签/销量/可选距离与运费估算）→ `SaveRecommendations` 落库（5 分钟过期）。
- 推荐配置：`GetRecommendationConfig/UpsertRecommendationConfig`（`db/query/recommendation.sql`）在 `api/recommendation.go` 暴露给 `/v1/regions/:id/recommendation-config`。

#### 3.7 风险分级与建议

**P0（必须修）**
- 推荐配置接口权限未落地：`getRecommendationConfig/updateRecommendationConfig` 注释宣称“非该区域运营商 403”，但实现仅要求登录，未做“运营商身份/region 归属”校验；任意登录用户可读取/篡改任意 region 的推荐参数（影响推荐结果、可能被恶意引流/投毒）。
- 优惠券在“订单创建（pending）”阶段即核销：`CreateOrderTx` 会在创建订单时直接 `MarkUserVoucherAsUsed + IncrementVoucherUsedQuantity`；若后续支付失败/用户放弃/订单取消，当前未看到回滚/恢复用户券的链路，可能造成用户券不可逆损失与口径失真。

**P1（应尽快修）**
- ErrNoRows 类型不一致：voucher/discount/favorite 多处用 `sql.ErrNoRows` 判定 not found，但 store/sqlc 基于 pgx（通常为 `pgx.ErrNoRows`）；测试 stub 也大量使用 `sql.ErrNoRows`，存在“测试通过但线上 404→500 漂移”的风险。
- 折扣规则 list/get 权限与归属校验不足：
  - `GET /v1/merchants/:id/discounts` 未校验调用方为商户且 merchant_id 匹配，可读取任意商户“所有状态”规则（含未生效/下架）。
  - `GET /v1/merchants/:id/discounts/:id` handler 未校验规则的 `merchant_id` 与 path merchant_id 一致，存在跨商户读取风险。
- 收藏接口幂等性缺口：重复收藏触发 `ON CONFLICT DO NOTHING RETURNING` 0 行时，当前 handler 会走 500（应视为幂等成功或返回 200/204）。
- 订单创建错误映射疑似失效：`createOrder` 中使用 `errors.Is(err, fmt.Errorf("voucher already used"))` 进行分支判断（动态构造 error），实际可能永远不命中，导致错误码/提示漂移。

**P2（优化项）**
- 订单金额预览接口 `GET /v1/orders/calculate` 支持 `voucher_code` 直接按券模板抵扣（未要求领取、未校验 `is_active`），但下单接口实际使用 `user_voucher_id`；存在“预览与下单可用性不一致/券码可枚举”的体验与安全治理问题。

### 3.8 评价与申诉
- [x] 评价：`api/review.go`
- [x] 评价上传：`api/review_upload.go`
- [x] 申诉：`api/appeal.go`

#### 3.8.1 路由与鉴权边界
- 路由装配：`api/server.go`
  - 评价（需登录，`authGroup`）：
    - `POST /v1/reviews/images/upload`（上传评价图片）
    - `POST /v1/reviews`（创建评价）
    - `GET /v1/reviews/:id`（查询评价）
    - `GET /v1/reviews/me`（我的评价列表）
    - `GET /v1/reviews/merchants/:id`（顾客视角：商户可见评价列表）
    - `GET /v1/reviews/merchants/:id/all`（商户视角：包含隐藏评价）
    - `POST /v1/reviews/:id/reply`（商户回复评价）
  - 删除评价（需 operator，`CasbinRoleMiddleware(RoleOperator)` + `LoadOperatorMiddleware()`）：
    - `DELETE /v1/reviews/:id`（运营商删除违规评价，仅限管辖区域）
  - 申诉（均需登录，`authGroup`；operator 路由额外挂 Casbin/operator 中间件）：
    - 商户：`GET /v1/merchant/claims`、`GET /v1/merchant/claims/:id`、`POST/GET /v1/merchant/appeals`、`GET /v1/merchant/appeals/:id`
    - 骑手：`GET /v1/rider/claims`、`GET /v1/rider/claims/:id`、`POST/GET /v1/rider/appeals`、`GET /v1/rider/appeals/:id`
    - 运营商：`GET /v1/operator/appeals`、`GET /v1/operator/appeals/:id`、`POST /v1/operator/appeals/:id/review`
  - 另有“信用分申诉（仅记录/提示）”：`POST /v1/trust-score/appeals`（`SubmitAppeal`）

#### 3.8.2 评价（Review）端到端链路取证
- 数据层：`db/query/review.sql`
  - `CreateReview/GetReview/GetReviewByOrderID`
  - `ListReviewsByMerchant/ListAllReviewsByMerchant/ListReviewsByUser` + `Count*`
  - `UpdateMerchantReply/DeleteReview`

1) 上传评价图片：`POST /v1/reviews/images/upload`（`api/review_upload.go`）
- 关键链路：登录用户 → 读取 multipart file → 微信图片安全检测（`wechatClient.ImgSecCheck`）→ 通过才落盘到 `uploads/reviews/{user_id}/...` → 返回可访问 URL。
- 测试：`api/review_upload_test.go`
  - 覆盖 OK / 风险图片（不落盘）/ 微信侧错误（不落盘）/ 未授权。

2) 创建评价：`POST /v1/reviews`（`api/review.go`）
- 关键校验（先审后存）：
  - 订单归属与状态：必须属于当前用户且为 completed。
  - 幂等：`GetReviewByOrderID`，已有评价则 409。
  - 可见性：读取 `user_profile.trust_score`，低信用用户评价默认 `is_visible=false`。
  - 内容安全：要求 `user.wechat_openid` 存在，并对文本走微信 `MsgSecCheck`。
  - 图片归属：要求 images 只能引用 `uploads/reviews/{user_id}/...` 且通过归属校验。
- DB：`CreateReview`。
- 测试：`api/review_test.go`
  - 覆盖 OK / 低信用隐藏 / 订单不存在 / 不归属 / 未完成 / 已评价 / 未授权 / 入参校验。

3) 查询/列表：`GET /v1/reviews/:id`、`GET /v1/reviews/me`、`GET /v1/reviews/merchants/:id`、`GET /v1/reviews/merchants/:id/all`
- `GET /v1/reviews/merchants/:id`：仅返回 `is_visible=true` 的评价（顾客视角）。
- `GET /v1/reviews/merchants/:id/all`：要求调用方具备 `merchant_owner` 角色，且 `user_role.related_entity_id == merchant_id`，返回包含隐藏评价。
- 测试：`api/review_test.go` 覆盖 `listMerchantReviews` happy path 与参数/未授权。

4) 商户回复：`POST /v1/reviews/:id/reply`
- 权限：要求调用方具备 `merchant_owner` 且关联商户与评价 merchant_id 一致。
- 内容安全：同样要求商户用户具备 `wechat_openid`，并在写库前走微信文本安全检测（先审后存）。
- DB：`UpdateMerchantReply` 写 `merchant_reply + replied_at`。
- 测试：`api/review_test.go` 覆盖 OK / review 不存在 / 非 merchant_owner / 商户不匹配。

5) 运营商删除评价：`DELETE /v1/reviews/:id`
- 权限：operator 角色 + 区域归属校验（`checkOperatorManagesRegion`）。
- DB：`GetReview` → `GetMerchant(review.merchant_id)` → `CheckOperatorManagesRegion(operator_id, merchant.region_id)` → `DeleteReview`。
- 测试：`api/review_test.go` 覆盖 OK / 非 operator / 不管辖区域 / review 不存在。

#### 3.8.3 申诉（Appeal）端到端链路取证
- 数据层：`db/query/appeal.sql`
  - 申诉：`CreateAppeal/GetAppeal/GetAppealByClaim/CheckAppealExists`
  - 商户/骑手视角：`List*Appeals*`、`Get*AppealDetail`；索赔侧：`List*Claims*`、`Get*ClaimDetail*`
  - 运营商视角：`ListOperatorAppeals/GetOperatorAppealDetail/ReviewAppeal/GetAppealForPostProcess`

1) 商户/骑手创建申诉：`POST /v1/merchant/appeals`、`POST /v1/rider/appeals`
- 资格校验：索赔必须为 `approved/auto-approved`（`GetClaimForAppeal`）。
- 归属校验：merchant 侧校验 `claimInfo.merchant_id == merchant.id`；rider 侧校验 `claimInfo.rider_id == rider.id`。
- 幂等门闩：`CheckAppealExists(claim_id)`，已有则 409。
- DB：`CreateAppeal`（写入 region_id 以便运营商按区域处理）。
- 测试：`api/appeal_test.go` 覆盖商户/骑手创建 OK、非商户/非骑手、claim 不存在/不归属、已存在申诉等分支。

2) 运营商审核：`POST /v1/operator/appeals/:id/review`（`api/appeal.go`）
- 前置：Casbin operator 角色 + `GetOperatorByUser` 加载 operator。
- 区域校验：`GetAppeal` 后检查 `appeal.region_id == operator.region_id`。
- 状态机：仅允许 pending → approved/rejected（SQL `ReviewAppeal` 也带 `WHERE status='pending'`）。
- approved 要求补偿金额：`compensation_amount` 必填且 >0。
- 后处理：审核成功后尝试查询 `GetAppealForPostProcess`，并分发异步任务 `DistributeTaskProcessAppealResult`（失败不阻塞响应）。
- 测试：`api/appeal_test.go` 覆盖 approve/reject OK、申诉不存在、不在区域、已审核、批准缺少补偿金额等分支，并校验 taskDistributor 被调用。

3) “信用分申诉（SubmitAppeal）”：`POST /v1/trust-score/appeals`
- 行为：仅返回 `status=noted` 的提示型响应，不落库、不进入运营审核链路。
- 测试：`api/trust_score_test.go` 的 `TestSubmitAppealAPI` 覆盖 OK/参数不合法/未授权。

#### 3.8 风险分级与建议

**P1（应尽快修）**
- ErrNoRows 类型不一致再次出现：评价相关 handler 对 `GetUser/GetReview` 的 not found 判断在 `sql.ErrNoRows` 与 `pgx.ErrNoRows` 间不统一，且单测中也混用（例如 deleteReview 测试 stub 使用 `sql.ErrNoRows`）。存在“测试通过但线上 404→500 漂移”的契约风险。
- 申诉幂等门闩粒度可能过粗：`CheckAppealExists` 仅按 `claim_id` 判断，不区分 `appellant_type`；这会把“同一索赔由不同主体（merchant/rider）分别申诉”的可能性直接封死。若产品语义期望“各自可申诉”，需要调整为 `(claim_id, appellant_type)` 维度。
- 审核后处理可靠性：`reviewAppeal` 审核成功后对 `GetAppealForPostProcess/DistributeTaskProcessAppealResult` 的失败不阻塞响应且无强制重试门闩；会导致 appeals 表已进入 approved/rejected，但信用分更新/通知等后处理可能丢失，形成对账与客诉隐患。

**P2（优化项）**
- 运营商审核并发竞态的错误映射：SQL 已通过 `WHERE status='pending'` 做并发门闩，但 handler 仍可能在竞态下返回 500（而非“已审核/不可重复审核”类 400/409）；建议统一错误码映射策略，减少客户端重试风暴。


### 3.9 搜索与扫码

- [x] 搜索：`api/search.go`
- [x] 扫码：`api/scan.go`

#### 3.9.1 路由与鉴权边界（public 暴露面）
- 路由装配：`api/server.go`
  - 搜索：`v1.Group("/search")` 下挂载（**无需认证**）：
    - `GET /v1/search/dishes`（`searchDishes`）
    - `GET /v1/search/merchants`（`searchMerchants`）
    - `GET /v1/search/combos`（`searchCombos`）
    - `GET /v1/search/rooms`（`searchRooms`）
  - 扫码点餐：`GET /v1/scan/table`（`scanTable`，**无需认证**）
- 中间件链路：search/scan 受全局 `RateLimiter`、全局超时（30s）与 `ResponseEnvelopeMiddleware` 影响；但未挂载 `SensitiveAPIMiddleware`（相对更容易被批量爬取/压测）。

#### 3.9.2 搜索（Search）端到端链路取证
- 菜品搜索：`GET /v1/search/dishes`（`api/search.go`）
  - 入参：`page_id/page_size` 必填；`keyword` 可选（不传即“全量枚举”）；`merchant_id` 可选；`user_latitude/user_longitude` 可选；`tag_id` 可选。
  - DB：
    - 指定商户：`SearchDishesByName/CountSearchDishesByName`（`db/query/dish.sql`）
    - 全局搜索：`SearchDishesGlobal/CountSearchDishesGlobal`（`db/query/dish.sql`，JOIN merchants + `earth_distance` + 可选 `tag_id` EXISTS）
  - 额外链路：若传入用户经纬度，会调用 `server.getRouteDistancesByMerchant` 批量请求 `mapClient.GetDistanceMatrix`，并对每条结果调用 `calculateDeliveryFeeInternal`（外部依赖与成本点）。
- 商户搜索：`GET /v1/search/merchants`（`api/search.go`）
  - DB：`SearchMerchants/CountSearchMerchants`（`db/query/merchant.sql`）
    - 过滤：SQL 仅过滤 `m.status='active' AND m.deleted_at IS NULL`；keyword 为空时 `ILIKE '%%'`，等价于分页枚举全量 active 商户。
  - 额外链路：若传入用户经纬度且 `mapClient != nil`，会再调用 `mapClient.GetDistanceMatrix` 批量计算路网距离，并用 `calculateDeliveryFeeInternal` 估算运费。
- 套餐搜索：`GET /v1/search/combos`（`api/search.go`）
  - DB：`SearchCombosGlobal/CountSearchCombosGlobal`（`db/query/combo.sql`，keyword 为空时允许枚举；同时支持按商户名/套餐名搜索）
  - 额外链路：同样会在传入经纬度时走路网距离与运费估算。
- 包间搜索：`GET /v1/search/rooms`（`api/search.go`）
  - 入参：`reservation_date/reservation_time` 必填；其余为可选过滤项。
  - DB：
    - 无 tag：`SearchRoomsWithImage + CountSearchRooms`（`db/query/table.sql`）
    - 有 tag：`SearchRoomsByMerchantTag`（`db/query/table.sql`，当前 handler 直接以返回列表长度作为 total）
  - 过滤：SQL 仅返回 `tables.table_type='room' AND tables.status='available' AND merchants.status='active'`，并排除指定日期/时段已被预订（`pending/paid/confirmed`）。

#### 3.9.3 扫码点餐（Scan Table）端到端链路取证
- 扫码入口：`GET /v1/scan/table?merchant_id=&table_no=`（`api/scan.go`）
  - 商户校验：`GetMerchant(merchant_id)`（store）→ handler 要求 `merchant.Status == 'approved'`（否则 503）。
  - 桌台校验：`GetTableByMerchantAndNo(merchant_id, table_no)`（`db/query/table.sql`）→ handler 仅拒绝 `status=='disabled'`（否则继续返回菜单）。
  - 菜单数据：
    - 分类：`ListDishCategories(merchant_id)`（`db/query/dish.sql`）
    - 菜品：`ListDishesForMenu(merchant_id)`（`db/query/dish.sql`，仅上架 `is_online=true`）
    - 套餐：`ListOnlineCombosByMerchant(merchant_id)`（`db/query/combo.sql`）
    - 活动：`ListActiveDeliveryPromotionsByMerchant`（`db/query/delivery_promotion.sql`）+ `ListActiveDiscountRules`（`db/query/discount.sql`）；两者查询失败会被吞掉（仍返回 200，但 promotions 为空）。

#### 3.9.4 测试覆盖核对
- 搜索：`api/search_test.go` 覆盖 `searchDishes/searchMerchants/searchRooms` 的参数校验与 500 分支；**未见 `searchCombos` 的单测覆盖**。
- 扫码：`api/scan_test.go` 覆盖 `scanTable` 的 OK/NotFound/商户未批准/桌台停用/参数校验/内部错误，以及“活动查询失败被忽略仍 OK”的行为。

#### 3.9 风险分级与建议

**P1（应尽快修）**
- public 搜索/扫码的“可枚举 + PII 暴露”风险：
  - `GET /v1/search/merchants` 为 public，返回 `phone/address`，且 `keyword` 可不传（等价于分页枚举全量商户）。
  - `GET /v1/scan/table` 为 public，返回 merchant phone/address + 菜单/活动 + table 信息。
  - 以上接口缺少更严格的“敏感 API 限流/防爬”策略，叠加其他 public/弱鉴权链路时会放大联动风险（见 3.4/3.5 中对 dining session 与 billing group 的风险记录）。
- 地图路网距离调用可被滥用：`searchDishes/searchMerchants/searchCombos` 在传入用户经纬度时会调用 `mapClient.GetDistanceMatrix`；若对外开放且缺少更严限流/缓存，存在外部调用成本与 DoS 风险。
- 商户状态枚举/口径不一致：search SQL 过滤 `merchants.status='active'`，而 `scanTable` handler 强约束 `merchant.Status=='approved'`；可能导致“搜索可见但扫码不可用/状态解释混乱”，建议统一状态机枚举与对外口径。

**P2（优化项）**
- `ListDishCategories` 未过滤 `dish_categories.deleted_at`（若存在软删语义，可能导致扫码菜单返回已删除分类）。
- `SearchDishesGlobal/CountSearchDishesGlobal` 存在重复条件（`d.is_online = true` 重复两次），建议清理以提升可维护性。
- swagger/注释与校验不一致：如 `searchMerchants` 注释宣称 keyword 必填，但 `binding:"omitempty"` 允许缺省。

### 3.10 区域与公共资源
- [x] 区域：`api/region.go`
- [x] 公共 URL：`api/public_url.go`

#### 3.10.1 路由与鉴权边界（`api/server.go`）
- 区域（Region）后备接口（**无需认证**）：
  - `GET /v1/regions/available`（`listAvailableRegions`）
  - `GET /v1/regions/:id/check`（`checkRegionAvailability`）
  - `GET /v1/regions/:id`（`getRegion`）
  - `GET /v1/regions`（`listRegions`）
  - `GET /v1/regions/:id/children`（`listRegionChildren`）
  - `GET /v1/regions/search`（`searchRegions`）
- 说明：`api/region.go` 文件头也明确“仅作为灾备/回退能力”。

#### 3.10.2 区域（Region）端到端链路取证
- 数据层：`db/query/region.sql`
  - 查询类：`GetRegion/ListRegions/ListRegionChildren/SearchRegionsByName/ListAvailableRegions`。
  - 关键实现点：`ListAvailableRegions` 使用 `NOT EXISTS (SELECT 1 FROM operators o WHERE o.region_id=r.id)` 过滤“未被运营商占用”的区域。

- Handler 核对：`api/region.go`
  - `getRegion`：path `id` → `store.GetRegion` → 404 使用 `sql.ErrNoRows` 判定。
  - `listRegions`：分页 + 可选 `level/parent_id` → `store.ListRegions`（`pgtype.Int2/Int8` 可选入参）。
  - `listRegionChildren`：path `id` → `store.ListRegionChildren(parent_id)`。
  - `searchRegions`：query `q` 必填 → `store.SearchRegionsByName`，限制最多 100 条。
  - `listAvailableRegions`：复用 `listRegionsRequest`（分页 + 可选 `parent_id/level`）→ `store.ListAvailableRegions` → 响应 `{regions,total,page_id,page_size}`（`total` 为当前页条数）。
  - `checkRegionAvailability`：`GetRegion` 后用 `store.GetOperatorByRegion(region_id)` 判断占用；若存在运营商，会在响应 `reason` 里拼接运营商名称。

- 测试覆盖：`api/region_test.go`
  - 覆盖 `get/list/children/search/available/check` 的 OK/BadRequest/NotFound/InternalError 分支；mock 的 NotFound 多使用 `sql.ErrNoRows`。

#### 3.10.3 公共 URL helpers（`api/public_url.go`）
- `externalBaseURL(ctx)`：基于 `X-Forwarded-Proto/X-Forwarded-Host`（回退到 `Request.Host`）生成对外 baseURL；非 localhost 强制 https。
  - 被 `api/upload_signed.go` 用于拼装下载用的签名 URL。
- `normalizeUploadPath(p)`：把非 http(s) 的输入归一化为 `uploads/...` 相对路径。
  - 被 `api/ecommerce_applyment.go` 的 `uploadImageToWechat` 用于把“本地 uploads 路径”转为 `os.Open` 的文件路径。

#### 3.10 风险分级与建议

**P1（应尽快修）**
- ErrNoRows 类型不一致再次出现：`api/region.go` 使用 `errors.Is(err, sql.ErrNoRows)` 判定 404；而 store/sqlc 基于 pgx 时常见为 `pgx.ErrNoRows`。测试也普遍 stub 为 `sql.ErrNoRows`，存在“测试通过但线上 404→500 漂移”的契约风险。

**P2（优化项）**
- Swagger/注释与路由真实鉴权边界漂移：`/v1/regions/available` 与 `/v1/regions/:id/check` 的注释带 `@Security BearerAuth`，但 `api/server.go` 实际装配为 public（无需认证）；容易误导调用方与权限矩阵。
- “运营商占用判断”可能与多区域模型不一致：`ListAvailableRegions`/`checkRegionAvailability` 基于 `operators.region_id` 与 `GetOperatorByRegion`，而项目同时存在 `operator_regions` 多区域关系表（`db/query/operator_region.sql`）。若真实语义以 `operator_regions` 为准，这两个后备接口可能给出错误可用性结论。
- `listAvailableRegions` 响应的 `total` 为当前页条数而非全量计数；若被前端用于分页展示，会造成“总数口径”误解。

### 3.11 上传与资源访问

- [x] 上传签名：`api/upload_signed.go`
- [x] 上传 URL：`api/upload_url.go`
- [x] uploads 目录路由/访问策略：`/uploads/*filepath`（见 `api/server.go` + `api/upload_signed.go`）

#### 3.11 取证要点（端到端链路）

1) 路由与访问策略（`api/server.go` + `api/upload_signed.go`）
- 资源下载入口：`GET /uploads/*filepath`（**不走认证**，全局路由）。
- 签名获取入口：`POST /v1/uploads/sign`（在 `authGroup` 下，需要登录）。
- 下载 handler `getSignedUpload` 的策略：
  - 若 `isPubliclyAccessibleUploadPath(normalized)==true`，直接 `ctx.File(...)` 返回（无需签名），Cache-Control: `public, max-age=300`。
  - 否则必须带 `expires, uid, sig`，校验 HMAC（uid+expires+path），Cache-Control: `private, max-age=60`。
- public 目录判定：
  - `uploads/public/` 一律公共可直出。
  - `uploads/merchants/{id}/(logo|storefront|environment)/...` 公共可直出。
  - `uploads/reviews/...` 公共可直出。

2) 签名算法与绑定维度（`api/upload_signed.go`）
- HMAC-SHA256，签名内容：`uid|expires|normalizedPath`，输出 base64url。
- 签名 key：优先 `UPLOAD_URL_SIGNING_KEY`；否则回退到 `TokenSymmetricKey`。
- TTL：`UPLOAD_URL_TTL`；默认 10 分钟。
- 重要：签名 URL 是“持有即访问”的预签名 URL，下载端不校验当前访问者身份，只校验签名参数。

3) 路径规范化/存储 URL（`api/upload_url.go`）
- `normalizeImageURLForStorage` 会从完整 URL 中截取 `/uploads/` 之后并去掉 query（用于把“带签名的临时 URL”回写为持久化相对路径）。
- `normalizeUploadURLForClient` 将存储的 `uploads/...` 转为客户端可直接访问的 `/uploads/...`（或透传外链）。

4) 上传落盘策略（`util/upload.go`）
- 本地文件系统落盘到 `uploads/`：
  - 商户 logo → `uploads/public/merchants/{user_id}/logo/...`（公共直出）
  - 商户其他图片（含证照/门头/环境等）→ `uploads/merchants/{user_id}/{category}/...`
  - 评价图片 → `uploads/reviews/{user_id}/...`（公共直出）
  - OCR 专用上传 `UploadMerchantImageForOCR` 额外做 magic number 校验与更小的 2MB 限制；普通上传主要靠扩展名白名单。

#### 3.11 风险分级与建议

**P0（必须修）**
- 评价图片匿名直出：`uploads/reviews/` 被视为公共目录，任何人可匿名下载；若评价上传被滥用为“上传隐私图片/证件照”，会造成不可控的数据外泄面。建议将评价图片改为“默认私有 + 需要签名/鉴权”，或至少分目录（public review vs private evidence）。

**P1（应尽快修）**
- 证照访问策略与“敏感图片”目标不一致：签名接口文档强调“证照/身份证/健康证等敏感图片需签名访问”，但 `isUploadPathOwnedByUser` 对 `business_license/food_permit` 直接返回 true（任意登录用户可签名获取）。若业务上确实希望公开展示，应明确：是否允许匿名/是否仅在商户详情页展示、是否做脱敏/加水印；若不希望公开，应将签名权限收敛到商户 owner/manager 或基于 merchant_id 的访问控制。
- 上传类型校验不一致：`UploadMerchantImage/UploadReviewImage/...` 仅做扩展名白名单（且使用 `strings.Contains` 判断），而 OCR 上传才做 magic number 校验；存在“伪造扩展名上传非图片内容”的攻击面与可维护性不一致。建议统一使用 magic number 校验，并用严格白名单比较（而非 contains）。
- 生成签名 URL 的 baseURL 依赖 `X-Forwarded-Host/Proto`（`externalBaseURL`）：若边界网关未剥离/校验这些头，可能出现 Host 注入，导致客户端拿到被污染域名的 URL（虽不影响签名校验，但存在钓鱼/误导风险）。

**P2（优化项）**
- 预签名 URL 参数暴露 uid：URL 中显式携带 `uid`，会暴露用户 ID 维度；可考虑只在签名 msg 中包含 uid，但 URL 参数不回显 uid（或用不可逆 token 替代）。
- Cache-Control 策略偏粗：公共素材 `max-age=300`、私有 `max-age=60`；可按目录与业务场景细化（例如 public 长缓存、私有禁缓存），并考虑 ETag/Last-Modified。

### 3.12 通知、WebSocket、微信相关
- [x] 通知：`api/notification.go`、`api/notification_helper.go`
- [x] WebSocket 指标：`api/ws_metrics.go`（以及 `websocket/*` 实现）
- [x] 微信：`api/wechat.go`、`api/payment_callback.go`（webhooks）

#### 3.12 取证要点（端到端核对）

1) 路由与鉴权边界（`api/server.go`）
- 通知 API：挂在 `authGroup` 下（需登录）
  - `GET /v1/notifications`（列表）
  - `GET /v1/notifications/unread/count`
  - `POST /v1/notifications/:id/read`、`POST /v1/notifications/read-all`
  - `DELETE /v1/notifications/:id`
  - 偏好：`GET/PUT /v1/notifications/preferences`
- WebSocket：同样挂在 `authGroup` 下（需登录）
  - `GET /v1/ws`（骑手/商户）
  - `GET /v1/platform/ws`（平台运营/管理）
- 微信回调（webhooks）：挂在 `v1.Group("/webhooks")` 下 **无需登录**，使用“验签 + 解密”作为入口安全边界
  - `POST /v1/webhooks/wechat-pay/notify`（支付成功）
  - `POST /v1/webhooks/wechat-pay/refund-notify`（退款结果）
  - `POST /v1/webhooks/wechat-ecommerce/notify`（合单支付）
  - `POST /v1/webhooks/wechat-ecommerce/refund-notify`（平台退款结果）
  - `POST /v1/webhooks/wechat-ecommerce/profit-sharing-notify`（分账结果）
  - `POST /v1/webhooks/wechat-ecommerce/applyment-notify`（二级商户进件状态）

2) 通知 CRUD 与偏好（`api/notification.go`）
- 核心链路：token → `authPayload.UserID` → store 层以 `user_id` 过滤（例如 `ListUserNotifications/CountUnreadNotifications/MarkNotificationAsRead/DeleteNotification/UpsertNotificationPreferences`）。
- 通知“创建 + 推送”由 helper 完成：`CreateNotificationWithPreferences`（`api/notification_helper.go`）读取偏好/免打扰 → 创建通知 → 若在线则通过 `wsHub` 推送并标记 `is_pushed`。

3) WebSocket 连接准入与可靠投递（`api/notification.go` + `websocket/*`）
- Upgrade：gorilla/websocket；当前 `CheckOrigin` **恒为 true**（注释提示生产应校验 Origin）。
- 准入（`/v1/ws`）：通过 `ListUserRoles(user_id)` 判定 rider/merchant；rider 额外要求 `IsOnline==true` 才允许连接。
- 回放：支持 `last_sequence` 触发 `ReplayToClient(..., limit=200)`。
- 平台 WS（`/v1/platform/ws`）：白名单角色（`admin/operator/platform_admin/platform_operator/finance`），entityID 以 `user_id` 为键。
- Hub（`websocket/hub.go`）：维护 riders/merchants/platforms 三类连接；支持 ack/message store TTL、离线队列（内存/Redis）、重试队列、跨进程 Redis Pub/Sub。

4) 跨进程推送链路一致性（worker ↔ API server）
- API server 启动时（`api/server.go`）若配置 Redis，会启动 `websocket.PubSubManager` 并 `PSubscribe`：
  - `notification:rider:*`、`notification:merchant:*`、`notification:platform:alerts`
- 订阅端期望 payload 结构为 `NotificationPushMessage{entity_type, entity_id, message{type,data,timestamp}}`（`websocket/pubsub.go`）。
- worker 侧存在“直接往 channel publish 原始 wsMessage JSON”的实现（例如 `worker/task_send_notification.go`、`worker/task_process_payment.go` 的配送池推送），与订阅端期望结构**不一致**：订阅端会 unmarshal 失败并丢弃消息（仅打日志）。

5) 微信回调：验签/解密/幂等/异步（`api/payment_callback.go`）
- 安全门闩：读取 body → 取 `Wechatpay-*` 头 → `VerifyNotificationSignature(...)` 验签 → 解密 resource。
- 幂等：以 `notification.ID` 做去重（`CheckNotificationExists` + `CreateWechatNotification` 记录通知 ID）。
- 支付成功：解密后查 `payment_orders(out_trade_no)` → 校验订单状态/金额 → 更新 `payment_orders` 为 paid → 入队 `TaskProcessPaymentSuccess`（失败会向平台 WS 告警）。
- 退款结果：解密后入队 `TaskProcessRefundResult`。
- 分账/合单支付/进件状态：均验签 + 解密 + 幂等记录 + 更新本地状态/入队后续任务。

#### 3.12 风险分级与建议

**P1（应尽快修）**
- WebSocket `CheckOrigin` 恒 true：若存在 H5/浏览器场景，会暴露“跨站 WebSocket 连接”攻击面（配合 token 泄露/前端存储策略会放大）。建议按环境启用 Origin 白名单校验，至少校验 `Origin` 与允许域名集合一致。
- worker→Redis→API server 的推送 payload 协议不一致：PubSub 订阅端期望 `NotificationPushMessage`，而 worker publish 的是原始 `{type,data,timestamp}`；在启用 Pub/Sub 的部署形态下，会导致实时通知/配送池推送**大量丢失**（只剩离线拉取或单进程直推）。建议统一为一个 push envelope（优先复用 `websocket.PublishNotificationPush`）。
- 回调 handler 读取 body 使用 `io.ReadAll` 且未见 size 上限：恶意大 body 可能导致内存压力/DoS。建议对 webhooks 路由使用 `http.MaxBytesReader`（或 Gin 中间件）限制请求体大小。
- 日志敏感信息泄露：支付回调在解析失败时记录 `body`；进件状态回调在解密后 parse 失败时记录 `decrypted`。建议默认不落完整 body/解密明文（只记录 request_id/notification_id 及摘要字段）。

**P2（优化项）**
- 验签未检查 timestamp 漂移/重放窗口（wechat SDK 内部未做也会受影响）：虽然 `notification.ID` 去重可抵御“同一通知重放”，但仍建议对 timestamp 做合理窗口校验并打点，降低 DoS 与噪声。
- 回调幂等检查为“先查再写”：在并发重复通知下可能出现竞态（两次都查不到）；虽然支付单/分账单状态机多数有二次门闩，但建议用 DB 唯一约束/UPSERT 把幂等做成强一致。

**建议（按优先级）**
- 定稿并统一“Redis Pub/Sub 推送协议”：worker 统一产出 `NotificationPushMessage`；平台告警与业务通知统一 message 结构。
- 加固 WebSocket 安全边界：生产启用 Origin 校验；限制 query token 入口或至少强制 TLS + 脱敏日志；补充连接级速率限制。
- 回调入口做资源保护：限制 body 大小、限制处理时长、对异常/验签失败做指标与告警；日志只打最小可定位字段。

### 3.13 中间件与权限框架（实现与测试）

- [x] API Server/路由组：`api/server.go`
- [x] 通用 middleware：`api/middleware.go`
- [x] 安全 middleware：`api/middleware_security.go`
- [x] 限流 middleware：`api/middleware_ratelimit.go`
- [x] tracing / prometheus：`api/middleware_tracing.go`、`api/middleware_prometheus.go`
- [x] RBAC：`api/rbac_middleware.go`、`api/role_access.go`
- [x] Casbin：`api/casbin_enforcer.go`
- [x] validator/util：`api/validator.go`、`api/util.go`、`api/customization_validator.go`

#### 3.13 取证要点（端到端核对）

1) 路由装配与中间件链（`api/server.go`）
- 全局 middleware 顺序：CORS → 安全头 →（生产）HSTS → Request-ID/Tracing → 请求日志 → Prometheus → 全局限流 → 全局 Timeout(30s)。
- `v1.Use(ResponseEnvelopeMiddleware())` 默认包裹 `{code,message,data}`，客户端可用 `X-Response-Envelope: 0` 显式关闭；同时对 `/v1/webhooks/*` 与 websocket upgrade 自动跳过。
- `authGroup := v1.Group("")` 只挂 `authMiddleware`（认证），**未全局启用 CasbinMiddleware**；Casbin 仅在少数 operator/admin 路由组通过 `CasbinRoleMiddleware` 局部使用。

2) 认证/授权链路
- `authMiddleware` 支持 WebSocket：当 upgrade 时允许 `?token=`（query token）作为 access token。
- 权限体系存在多来源：
  - “关系型/实体加载”中间件（如 `LoadOperatorMiddleware`、`MerchantStaffMiddleware`）
  - 角色枚举 RBAC（`RoleMiddleware(...)`）
  - Casbin 路径/方法授权（`casbin/model.conf` + `casbin/policy.csv`）
  - `/v1/role-access` 手写元数据矩阵（`api/role_access.go`）

3) 可观测性/安全/限流
- RequestLogging 记录 `path` + **RawQuery** + `user_agent` + `client_ip` + `user_id`（若已认证）。
- Prometheus 指标：对 404 使用实际 path（非路由模板），可能产生更高 label cardinality。
- RateLimiter：全局 visitors 有定期清理；但 `SensitiveAPIMiddleware` 采用独立 map 且**没有清理机制**。
- CORS allow headers 已包含 `X-Response-Envelope`，确保浏览器侧可显式 opt-out。

#### 3.13 风险分级与建议

**P0（必须修）**
- Casbin **fail-open**：`globalCasbinEnforcer == nil` 时 `CasbinMiddleware/CasbinRoleMiddleware` 直接跳过权限检查并放行。若生产 Casbin 初始化失败（`NewServer` 里只是 warn 并继续运行），依赖 CasbinRoleMiddleware 的路由组可能退化为“仅认证不授权”。示例：平台统计组使用 `CasbinRoleMiddleware(RoleAdmin)`，在 enforcer 为 nil 时会直接放行到 handler。

**P1（应尽快修）**
- 日志泄露风险：请求日志记录 RawQuery；而 WebSocket 认证支持 `?token=`，且认证失败会 `log.Warn().Str("url", ctx.Request.URL.String())` 输出完整 URL（含 token）。这会把 access token 暴露到日志系统。
- 权限矩阵一致性风险：当前存在“路由装配 + RBAC + Casbin policy + role-access 元数据”多份真相来源；且 Casbin 并未全局启用，`policy.csv` 可能无法反映真实可访问面，容易给前端/运营/安全审计造成误判。

**P2（优化项）**
- ResponseEnvelope opt-in 与 CORS allow headers 不一致，跨域场景下前端无法稳定启用统一响应封装。
- `SensitiveAPIMiddleware` 的 map 无清理机制，存在长期运行的内存增长点。
- Prometheus 404 使用实际 path 作为 label，可能导致高基数与监控成本上升。

**建议（按优先级）**
- Casbin：将初始化失败视为强依赖（启动失败/ready=fail），或至少在 middleware 中 fail-closed（返回 503/500 并打点告警），避免静默放行。
- 日志：默认不记录 RawQuery 或对 `token` 等敏感参数做脱敏/白名单；WebSocket auth 失败日志避免打印完整 URL。
- 权限：明确单一事实来源（建议从路由装配生成 policy/metadata 或统一由 Casbin 承载），并增加“路由权限快照”与 CI 检查防漂移。
- 测试：补充最小化测试覆盖（Casbin enforcer 为 nil 时应拒绝访问；关键路由组授权/越权用例；ResponseEnvelope header 行为）。

---

## 4. 单接口“完整链路”核对清单（模板，不计入本次模块完成度）

> 注意：本节是“逐接口复核模板”，用于未来对单个接口做更细粒度核对/复盘。
> - **不纳入本次按模块维度的勾选完成度**（本次完成度以第 3 节模块勾选为准）。
> - 每组接口里建议至少对关键写链路/资金链路/状态机链路做全量审查；读接口可抽样。

### 4.1 接口信息
- 路由与方法：
- 访问方与权限：
- 入参（body/query/path）：
- 出参：
- 关键字段含义与约束：

### 4.2 Handler 层
- [ ] 输入校验完整（必填/范围/枚举/格式）
- [ ] 认证/授权正确（角色、资源归属校验）
- [ ] 幂等策略明确（是否需要 idempotency key / 去重）
- [ ] 超时/取消传递（context 使用一致）

### 4.3 业务逻辑
- [ ] 状态机约束正确（订单/支付/配送/审核等状态流转）
- [ ] 重要业务不变量清晰（库存不为负、资金守恒等）
- [ ] 并发条件下正确（乐观锁/悲观锁/唯一约束）

### 4.4 数据库链路
- [ ] 事务边界正确（跨表写入、先后顺序、回滚策略）
- [ ] 查询/更新有索引支撑（热点表、分页、过滤条件）
- [ ] 错误处理正确（not found、unique violation、外键等）

### 4.5 外部依赖
- [ ] 调用外部服务有超时/重试/熔断策略
- [ ] 回调验签与安全校验（支付/微信）

### 4.6 返回与可观测性
- [ ] 错误码/错误信息一致且可定位
- [ ] 日志无敏感信息泄露
- [ ] 指标/Tracing 覆盖关键链路

### 4.7 测试
- [ ] 单测覆盖关键分支
- [ ] 针对边界/异常/越权有测试

**问题记录**
- 严重（必须修）：
- 重要（应修）：
- 建议（可优化）：

---

## 5. 审查推进日志（可选）

- 日期：2026-01-16
  - 完成：3.2 门店/位置（`api/location.go`）取证落盘并勾选；产出总体报告草案 `docs/backend_review_report.md`。
  - 完成：3.13 中间件与权限框架取证落盘并勾选（中间件链、RBAC/Casbin、role-access、响应封装、限流/日志/指标）。
  - 完成：3.12 通知/WebSocket/微信相关取证落盘并勾选（通知 CRUD/偏好、WS 准入与回放、Hub/队列/PubSub、支付/退款/分账/进件回调验签与幂等）。
  - 发现：location 接口实际在认证组；`matchRegionID` 依赖 `Adcode→regions.code` 精确匹配但 OSM 侧 `Adcode=postcode` 可能长期失效；跨模块仍存在资金幂等、状态机漂移、ErrNoRows 类型不一致、权限边界与日志敏感信息等高优先级风险。
  - 发现（新增）：Casbin enforcer nil 时 fail-open；请求日志会记录 RawQuery 且 WebSocket 允许 query token，存在 access token 落日志的风险；权限矩阵多来源易漂移。
  - 发现（新增）：worker publish 的 Redis 推送 payload 与 API server PubSub 订阅协议不一致，可能导致实时推送丢失；webhooks 回调未限制 body 大小且存在 body/decrypted 落日志点；WS upgrader `CheckOrigin` 恒 true。
  - 下一步：本轮模块级审查已完成；进入“按 P0/P1 优先级整改 + 按第 4 节逐接口清单做验收/回归测试补齐”的阶段（详见 `docs/backend_review_report.md` 的整改与验收顺序建议）。
