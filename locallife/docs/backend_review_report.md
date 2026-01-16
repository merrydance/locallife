# 后端代码审查总体报告

- 日期：2026-01-16
- 范围：`locallife/` 后端 Go 服务（Gin API、sqlc Store/Tx、worker、外部集成 wechat/maps 等）
- 约束：仅审查与记录，不直接修改业务代码
- 事实来源：详细取证与单点结论以 `docs/backend_review_tasks.md` 为准

---

## 1. 结论摘要

本轮审查已完成“总体架构 + 横切能力 + 交易链路/骑手链路 + 商户侧核心链路（入驻/申请/OCR/进件/财务/统计/位置）”的端到端核对。

总体风险画像：
- **P0（必须修）**集中在“资金入账/幂等门闩缺失”“状态机与 DB 约束漂移”“可导致数据外带的图片读取链路”“关键写链路缺少事务/唯一约束兜底”。
- **P1（应尽快修）**集中在“ErrNoRows 类型不一致导致线上 404/409 → 500 漂移”“权限边界过宽导致财务/PII 暴露”“日期口径少算一天”“日志泄露敏感信息”。
- **P2（优化项）**集中在“public/需登录的契约漂移”“BI 字段语义错配”“测试覆盖缺口与可维护性问题”。

---

## 2. 审查进度（截至 2026-01-16）

已审查并在任务清单中勾选：
- 1.x 总体架构审查
- 2.x 横切能力审查
- 3.12 通知/WebSocket/微信相关（通知 CRUD、WS 准入与可靠投递、webhooks 验签/解密/幂等/异步）
- 3.13 中间件与权限框架（路由装配、中间件链、RBAC/Casbin、role-access、一致性与测试）
- 3.1 用户与认证（token/user/address/membership）
- 3.2 商户侧（merchant/merchant_application/ecommerce_applyment/merchant_stats/merchant_finance/location）
- 3.11 上传与资源访问（签名 URL、/uploads 访问策略、本地落盘与公共目录）
- 3.5 交易链路（购物车/订单/支付下单/回调/配送费/配送）
- 3.6 配送与骑手（rider/rider_application/rider_location_events）
- 3.3 平台侧/运营后台（boss/platform_stats/operator_application/operator_stats/operator_merchant_rider/billing_group）
- 3.4 商品与资源管理（菜品/菜品上传/套餐/库存/标签/桌台/桌台上传/预订/就餐会话/后厨 + 扫码/搜索入口的风险联动核对）
- 3.7 营销与增长（优惠券/折扣/收藏/推荐 + 订单用券核销链路与推荐配置接口）
- 3.8 评价与申诉（评价创建/上传/可见性、商户回复与运营商删除；商户/骑手申诉与运营商审核；信用分申诉入口）
- 3.9 搜索与扫码（search/scan 的 public 暴露面 + handler/SQL/test 端到端取证）
- 3.10 区域与公共资源（regions 后备接口 + 可用性检查；公共 URL 生成与 uploads 链路引用）

未审查（示例，详见任务清单）：更多异步任务、风控/合规模块与边缘功能模块。

---

## 3. P0 风险（必须修）

> 下面仅列“跨模块影响大、可直接造成资金/数据/状态不可恢复”的问题；每条细节与证据链请以 `docs/backend_review_tasks.md` 对应模块段落为准。

1) 资金入账幂等性缺口（骑手押金充值）
- 现象：支付回调后 worker 对“已处理”缺少门闩/唯一约束，重复投递/重试可能**重复加余额**。
- 影响：直接资金损失、对账困难、后续补偿成本极高。

2) 状态机与数据库约束漂移（旧商户申请链路 vs 迁移后的 status 约束）
- 现象：`merchant_applications` 迁移后仅允许 `draft/submitted/approved/rejected`，但旧接口仍使用 `pending` 等状态。
- 影响：线上出现“查不出来/无法审核/重复提交”与不可解释的流程卡死。

3) 商户申请重置/再提交可能降级商户状态
- 现象：reset/approve 逻辑可能把已 `active` 的商户写回 `pending/approved`。
- 影响：商户可用性、结算与权限链路被破坏，属于高影响可用性事故风险。

4) 高风险图片读取/上传链路（LFI/路径穿越 + SSRF + DoS）
- 现象：进件图片上传既支持本地文件读，也支持 URL 拉取；缺少足够的路径约束、超时与流式限流。
- 影响：可能导致任意文件外带、内网探测、内存/连接耗尽。

5) uploads 目录存在匿名直出面（评价图片）
- 现象：`GET /uploads/*filepath` 对 `uploads/reviews/` 直接公共直出（无需签名/认证）。
- 影响：若评价上传被滥用为隐私图片/证件照上传，会形成不可控的数据外泄面。

5) 默认地址一致性非原子
- 现象：清空默认与设置默认分两次写，缺少事务/唯一约束兜底。
- 影响：出现“多默认/无默认”，进而影响履约、运费计算与区域匹配。

6) 骑手申请通过但未创建 rider 的原子性缺口
- 现象：申请先置 `approved`，再事务外创建 riders；创建失败会留下“申请已通过但无档案”。
- 影响：业务流程卡死，需要人工介入修复。

7) 会员充值 attach 拼 JSON 导致“支付成功但不入账”
- 现象：无匹配规则时 attach 生成 `<nil>`，worker 侧 `json.Unmarshal` 失败并 `SkipRetry`。
- 影响：支付成功但余额不到账，属于资金链路事故。

8) 权限框架 fail-open（Casbin 初始化失败/未就绪时放行）
- 现象：Casbin 中间件在 `globalCasbinEnforcer == nil` 时跳过权限检查并放行；而服务启动期 Casbin 初始化失败仅 warn 并继续运行。
- 影响：依赖 Casbin 的路由组可能退化为“仅认证不授权”，存在越权访问与数据暴露风险。

9) Billing Group 越权读取/加入（仅登录即可枚举）
- 现象：`/v1/billing-groups`、`/v1/billing-groups/:id/orders`、`/v1/billing-groups/:id/join` 当前仅要求登录，缺少“账单组成员/用餐会话参与者”的前置校验；而 SQL 层已存在 `GetActiveBillingGroupMember` 可用于校验。
- 影响：任意已登录用户在知道/猜测 `dining_session_id` 或 `billing_group_id` 的情况下，可能读取账单组与订单信息（金额/状态等）或加入不属于自己的账单组，属于直接的数据暴露与权限绕过面。

10) 用餐会话 open 缺少“桌台口令/会话参与者”校验，可能被远程加入并联动 Billing Group
- 现象：`/v1/dining-sessions/open` 仅要求登录；当桌台存在进行中的 session 时，调用方可在缺少“物理在场证明/桌台 secret”的情况下被加入默认 billing group。
- 影响：与 Billing Group 越权面叠加，形成“知道 table_id（或通过 public 扫码接口推导）即可远程入组/枚举账单与订单”的风险链路。

11) 库存扣减在“无记录”分支返回 unlimited（与 SQL 语义冲突）
- 现象：SQL 语义为 `total_quantity=-1` 才代表无限库存；但库存 handler 在 ErrNoRows 分支把“无记录”解释成“无限库存(Available=true, stock=-1)”。
- 影响：可能导致超卖/错卖，进而引发资金与履约事故，并掩盖库存配置缺失。

12) 推荐配置接口缺少“区域运营商”权限校验（可被任意登录用户篡改）
- 现象：`/v1/regions/:id/recommendation-config` 注释宣称“非该区域运营商 403”，但实现仅要求登录，未做运营商身份与 region 归属校验。
- 影响：可被恶意修改推荐参数，导致推荐结果被投毒/引流，形成区域级别的业务与风控事故。

13) 优惠券在“订单创建（pending）”阶段即核销，缺少失败/取消回滚
- 现象：`CreateOrderTx` 在创建订单时直接 `MarkUserVoucherAsUsed + IncrementVoucherUsedQuantity`；未见与“支付失败/订单取消/过期关闭”联动的恢复逻辑。
- 影响：用户券可能被不可逆消耗且统计口径失真，属于资金相关高影响体验/对账风险。

---

## 4. P1 风险（应尽快修）

1) ErrNoRows 类型不统一导致错误码漂移
- 现象：handler/测试在 `sql.ErrNoRows` vs `pgx.ErrNoRows` 上不一致，单测可能“测得过”，线上却 404→500。
- 影响：接口契约不稳定、客户端误判重试、可观测性变差。

16) 申诉幂等门闩粒度与语义可能不匹配
- 现象：`CheckAppealExists` 仅按 `claim_id` 判断是否已申诉，不区分 `appellant_type`；若同一索赔同时关联 merchant/rider，两侧“各自申诉”的语义会被互斥。
- 影响：真实业务诉求无法表达、客诉处理困难；也会导致接口返回 409 的原因难以解释。

17) 运营商审核后处理不可靠（已审核但异步链路可能丢）
- 现象：`reviewAppeal` 在写库后通过异步任务处理“信用分更新/通知等”，但任务分发失败不会阻塞响应，且缺少可观测/重试门闩。
- 影响：appeals 状态已变更但后处理未生效，形成对账与合规审计隐患。

2) 权限边界偏宽（财务/统计/PII）
- 现象：多个敏感接口仅依赖 `GetMerchantByOwner`（owner + active staff），未额外收敛到 owner/manager。
- 影响：普通员工可访问财务数据、顾客手机号等 PII，存在合规与内控风险。

3) 上传内容校验不一致（仅扩展名白名单 vs magic number）
- 现象：多数上传仅按扩展名白名单校验；OCR 上传才校验 magic number。
- 影响：存在“伪造扩展名上传非图片内容”的攻击面与策略不一致，且公共目录直出会放大影响。

4) 证照访问策略与“敏感图片”目标不一致
- 现象：签名接口文档将证照列为敏感，但实现允许任意登录用户对 `business_license/food_permit` 获取签名 URL。
- 影响：若业务并非刻意公开，会造成证照信息扩散与合规压力；若刻意公开，需要明确展示面与最小化泄露。

3) 统计日期口径少算一天
- 现象：`end_date` 若按日期解析为 00:00:00 且 SQL 用 `<= end_date`，会漏掉当日绝大多数订单。
- 影响：BI/对账偏差，且与财务口径不一致。

4) OCR/请求日志可能泄露敏感信息
- 现象：worker/handler 记录了 OCR 文本片段、请求 payload（含电话/地址/证件信息片段）。
- 影响：日志侧泄露敏感信息，扩大数据面。

5) 地图编码体系可能不匹配导致 region 精确匹配长期失效
- 现象：OSM reverse 将 `Adcode` 绑定为 `postcode`，可能无法命中 `regions.code`，长期走“最近邻兜底”。
- 影响：边界区域误匹配风险 + 额外外部调用成本 + 难以解释的区域归属。

6) access token 可能落日志（WebSocket query token + RawQuery 记录）
- 现象：请求日志记录 RawQuery；WebSocket 认证允许 `?token=`，且认证失败日志会打印完整 URL。
- 影响：令牌泄露到日志系统后会扩大攻击面（重放/横向移动）。

7) 权限矩阵多来源导致漂移
- 现象：存在“路由装配 + RBAC + Casbin policy + role-access 元数据”多份来源，且 Casbin 并未全局启用。
- 影响：前端/审计误判真实可访问面，回归测试与权限治理成本上升。

8) WebSocket `CheckOrigin` 恒 true（浏览器/H5 场景存在 CSWSH 面）
- 现象：WS upgrader 未做 Origin 限制；若叠加 token 泄露/弱前端存储策略，会放大“跨站 WebSocket 连接”攻击面。
- 影响：实时消息链路被滥用、账号侧骚扰/信息泄露风险上升。

9) worker→Redis→API server 推送协议不一致导致实时推送丢失
- 现象：Pub/Sub 订阅端期望 `NotificationPushMessage`，但 worker publish 的是原始 `{type,data,timestamp}` JSON；订阅端 unmarshal 失败会丢弃。
- 影响：通知/配送池等实时推送在多进程部署下不可靠（业务退化、履约延迟、排障困难）。

10) webhooks 未限制 body 大小且存在明文落日志点
- 现象：回调 handler 用 `io.ReadAll` 读取 body 且未见上限；解析失败时记录 `body`，进件回调在 parse 失败时记录 `decrypted`。
- 影响：DoS（内存压力）+ 日志侧敏感信息暴露（合规与内控风险）。

11) 标签创建接口未实现管理员权限（注释与实现不一致）
- 现象：`POST /v1/tags` 注释称管理员，但实现仅验证登录即可创建 active tag。
- 影响：任意登录用户可污染标签体系（类型/排序/命名），影响搜索/推荐/运营治理与数据质量。

12) public 搜索/扫码返回面偏大（枚举/爬取/联动风险）
- 现象：
	- `GET /v1/scan/table` 为 public，响应包含 merchant phone/address、完整菜单/套餐/活动信息与 table 相关数据。
	- `GET /v1/search/merchants` 为 public 且 keyword 可缺省，返回 merchant phone/address，等价于“分页枚举全量商户”。
- 影响：可被批量爬取；并与“open dining session + billing group”链路叠加时，远程枚举与加入风险更高。

13) public 搜索接口会触发外部地图调用（成本/DoS 风险）
- 现象：`searchDishes/searchMerchants/searchCombos` 在传入用户经纬度时会调用 `mapClient.GetDistanceMatrix`（路网距离），并进一步触发运费估算。
- 影响：若缺少更严格的限流/缓存，容易成为外部依赖成本放大点与 DoS 面。

14) 折扣规则 list/get 权限与归属校验不足（登录即可跨商户读取“所有状态”规则）
- 现象：`GET /v1/merchants/:id/discounts`、`GET /v1/merchants/:id/discounts/:id` 未完整收敛到商户权限与 merchant_id 一致性校验。
- 影响：可导致竞争信息泄露与运营规则暴露，且形成“路径参数看似限制但实际未限制”的误导。

15) 收藏接口重复写入的幂等语义缺口（ON CONFLICT DO NOTHING → 500）
- 现象：重复收藏会触发 `ON CONFLICT DO NOTHING RETURNING` 返回 0 行，handler 侧可能误判为内部错误。
- 影响：客户端重试/并发点击会放大 500，影响体验并污染监控。

---

## 5. P2 风险（优化项）

- public/需登录契约漂移：命名为 public 的接口实际挂在 authGroup 下，易导致权限矩阵与文档误导。
- regions 后备接口契约漂移：Swagger 标注 `BearerAuth` 但路由实际为 public，且占用判断基于 `operators.region_id`（与 `operator_regions` 多区域模型可能不一致）。
- BI 字段语义错配：统计接口存在 `order_count` 填 `dish_count` 等易误解问题。
- 测试覆盖缺口：location 未见直接测试；多处只测 happy path，异常/攻击面（SSRF/LFI/竞态/幂等）覆盖不足。

---

## 6. 跨模块共性问题（可作为专项整治）

1) “双轨实现”与状态机漂移
- 旧/新链路并存（如商户申请、骑手入驻）导致数据源不一致、状态枚举不一致、测试锁死旧行为。

2) 幂等门闩不足
- 支付成功后的 worker、抢单/状态推进等处需要更系统的“幂等门闩 + DB 唯一约束”组合拳。

3) 依赖边界与降级策略不统一
- 部分依赖在启动期允许缺失并运行，但运行期会产生 500；需要明确哪些是强依赖并在健康检查中体现。

4) 错误映射与契约一致性
- 业务拒绝（409/400）与内部错误（500）混用会放大客户端重试风暴，并掩盖真实异常。

5) 敏感信息治理（PII/证件/银行卡）
- 既包含接口返回，也包含日志侧输出；需要系统性脱敏与最小权限。

---

## 7. 优先级建议（行动清单）

### 7.1 立即处理（P0）
- 给所有“支付成功入账/资金变更”引入幂等门闩：以 `payment_order_id` 或 `out_trade_no` 为唯一键，在 DB 层做唯一约束并用 Tx + `FOR UPDATE` 实现一次性处理。
- 收敛商户/骑手申请双轨：确定唯一对外链路与唯一状态机枚举，删/停用另一条或明确仅内部。
- 关掉/强约束进件图片的 URL 拉取与本地读取：只接受上传系统生成的 key；若保留 URL 拉取，必须白名单域名 + 禁内网 IP + 超时 + 流式大小限制。
- 默认地址写入改为事务性并加部分唯一索引兜底，避免并发产生多默认。
- 会员充值 attach 必须使用 `json.Marshal` 生成，避免 `<nil>`。
- 商户申请 reset/approve 不得降级 `active` 商户状态（需要显式状态保护）。

### 7.2 尽快处理（P1）
- 统一 NotFound 判定：以 `db.ErrRecordNotFound`（或 `pgx.ErrNoRows`）作为唯一来源，同步修正测试 stub。
- 收敛敏感接口权限：财务/顾客手机号等默认仅 owner/manager 可见；若必须给员工可见，至少脱敏与审计。
- 统一日期范围口径：用半开区间 `[start, end+1day)` 或把 end_date 归一化到当日结束，并在财务/统计保持一致。
- 建立日志脱敏规范：OCR/证件/手机号/地址等字段统一脱敏或不落日志。

---

## 8. 下一步建议（整改与验收顺序）

本轮模块级审查已完成；下一步建议从“继续审查”切换为“按优先级整改 + 用第 4 节逐接口清单做验收”。

按“资金/越权/数据外泄”优先：
1) P0 专项整改：资金入账幂等（回调/worker）、Billing Group + dining session 越权链路、上传/图片高风险链路（本地读/URL 拉取/公共直出）。
2) P1 专项整改：统一 ErrNoRows/NotFound 映射、收敛权限矩阵单一事实来源、日志脱敏与请求体大小限制、BI 日期口径统一。
3) 验收与回归：对上述整改涉及的关键接口，按 `docs/backend_review_tasks.md` 第 4 节“完整链路核对清单”逐条验收，并补齐边界/越权/幂等等回归测试。

---

## 9. 附：如何使用本报告

- 本文件用于“跨模块归并与决策优先级”。
- 具体接口/SQL/函数级证据链、复现路径与建议细节，继续沉淀在 `docs/backend_review_tasks.md` 对应章节中。
