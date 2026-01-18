# 讨论议题记录（2026-01-18）

> 目标：记录讨论问题、关键发现与最终结论。

## 1. 集团多门店管理
### 可用方案（去重版）
**目标**：在保持现有单店入驻与经营能力的前提下，新增集团/品牌治理与跨店管理闭环。

#### 1) 现状与差距
- 当前仅有单店模型，缺少集团/品牌实体与跨店权限治理。
- 多店人员关联仅靠 merchant_staff 与 merchant_bosses，无法承载集团级治理与报表。

#### 2) 新增实体与字段（统一口径）
- merchant_group_applications（集团入驻申请）：
  - id, applicant_user_id, group_name, contact_phone, license_number, license_image_url, address, region_id, status(draft/submitted/approved/rejected), reject_reason, reviewed_by, reviewed_at, application_data, created_at, updated_at
- merchant_groups（集团）：
  - id, name, owner_user_id, status, contact_phone, license_number, license_image_url, address, region_id, application_data, created_at, updated_at
- merchant_brands（品牌，可选）：
  - id, group_id, name, logo_url, description, status, created_at, updated_at
- merchant_group_members（集团成员）：
  - id, group_id, user_id, role(owner/admin/finance/ops), status, joined_at, invited_by
- merchant_group_join_requests（门店加入申请）：
  - id, group_id, merchant_id, applicant_user_id, status(pending/approved/rejected/cancelled), reason, reviewed_by, reviewed_at, created_at
- group_policies（集团策略）：
  - group_id, pricing_mode(central/store), menu_mode(central/store), inventory_mode(central/store), promotion_mode(central/store)
- group_menu_templates / brand_menu_templates（可选）：
  - id, group_id/brand_id, payload(json), version, status, created_at, updated_at
- merchants 增加字段：
  - group_id（可选）、brand_id（可选）

#### 3) 权限模型
- 集团级角色：owner/admin/finance/ops（跨店范围内授权）。
- 门店级角色：继续使用 merchant_staff，仅限本店。
- 新员工未分配角色前无权限。
- 应用层权限集成：
  - Casbin 增加 group 维度的资源规则（如 /v1/groups/:id/*、/v1/groups/:id/stats/*）。
  - 请求鉴权时同时校验 group_id 或 merchant_id 归属（避免跨组越权）。

#### 4) 核心业务流程（闭环）
1. 集团入驻：提交集团资质 -> 审核通过 -> merchant_groups 可用（无需进件）。
2. 门店入驻：沿用现有门店流程 -> 商户状态 approved -> 进件完成后 active 才可营业。
3. 门店加入集团：门店搜索集团发起 join_request -> 集团审核通过 -> merchants.group_id 绑定生效。
4. 集团邀请门店/员工：生成邀请/二维码 -> 被邀请方扫码确认 openid -> 绑定 group_member 或 merchant_staff。
5. 模板下发与覆盖：集团/品牌模板发布 -> 门店按策略同步 -> 允许字段覆盖并记录版本。
6. 统计与结算：门店维度结算、集团维度汇总报表。

#### 5) 关键治理点
- 加入/退出流程治理与审计日志（申请/邀请/通过/拒绝/撤销/解除）。
- 权限与数据隔离（跨店资源范围、财务/促销可见性边界清晰）。
- 模板覆盖冲突与回滚机制（优先级、版本回滚、变更记录）。
- 结算与报表口径一致性（集团汇总 vs 门店实收，含退款/优惠/分账/服务费）。
- 运营侧通知与异步一致性（加入后权限生效、缓存刷新、异步任务最终一致）。

#### 6) 接口设计（建议）
- 集团入驻：
  - POST /v1/groups/applications
  - GET /v1/groups/applications/me
  - POST /v1/groups/applications/submit
  - POST /v1/groups/applications/:id/review
- 集团/品牌：
  - POST /v1/groups
  - GET /v1/groups/:id
  - GET /v1/groups/:id/merchants
  - PATCH /v1/groups/:id
  - POST /v1/groups/:id/brands
  - GET /v1/brands/:id
- 门店加入集团：
  - POST /v1/groups/:id/join-requests
  - GET /v1/groups/:id/join-requests
  - POST /v1/groups/:id/join-requests/:request_id/approve
  - POST /v1/groups/:id/join-requests/:request_id/reject
- 邀请/扫码绑定：
  - POST /v1/groups/:id/invitations
  - POST /v1/groups/:id/invitations/accept
- 成员与权限：
  - POST /v1/groups/:id/members
  - PATCH /v1/groups/:id/members/:member_id/role
  - DELETE /v1/groups/:id/members/:member_id
- 模板与策略：
  - PUT /v1/groups/:id/policies
  - POST /v1/groups/:id/menu-templates
  - POST /v1/brands/:id/menu-templates
- 报表：
  - GET /v1/groups/:id/stats/overview
  - GET /v1/groups/:id/stats/merchants

#### 7) 兼容与迁移
- 不做“单店集团”强制迁移：单店保持现状；仅当门店申请加入集团时才建立关联。
- merchant_staff 保留；merchant_bosses 可逐步下线；group_member 仅在集团场景启用。

## 2. 异常订单处理的算法
### 现有实现梳理（已落地）
**覆盖范围**：用户索赔、商户/骑手申诉、食安熔断、团伙欺诈检测、骑手延迟/异常上报。

#### 2.1 入口与链路
- 用户索赔：/v1/claims（自动评估与建单，见 [api/risk_management.go](api/risk_management.go)）
- 索赔人工审核：/v1/claims/{id}/review（见 [api/risk_management.go](api/risk_management.go)）
- 食安上报：/v1/food-safety/report（见 [api/risk_management.go](api/risk_management.go)）
- 欺诈检测（管理员手动触发）：/v1/fraud/detect（见 [api/risk_management.go](api/risk_management.go)）
- 商户/骑手索赔与申诉：/v1/merchant/claims*、/v1/rider/claims*、/v1/rider/appeals*（见 [api/appeal.go](api/appeal.go)）
- 骑手异常与延迟上报：/v1/rider/orders/{id}/delay、/v1/rider/orders/{id}/exception（见 [api/rider.go](api/rider.go)）

#### 2.2 索赔自动审核与赔付策略
实现位于 [algorithm/claim_auto_approval.go](algorithm/claim_auto_approval.go) 与 [algorithm/trust_score_types.go](algorithm/trust_score_types.go)：
- **总原则**：索赔“都赔”，区别只在于**是否秒赔**、**是否需要证据**、**赔付来源**（商户/骑手押金/平台垫付）。
- **类型策略**：
  - food-safety → 人工审核（manual）
  - timeout → 仅赔运费，来源骑手押金
  - damage → 全额，来源骑手押金
  - foreign-object → 全额，来源商户
- **行为回溯**（3个月窗口）：
  - 5单3索赔 或 索赔比例≥60% → 警告（warned）
  - 已警告后 → 需要证据（evidence-required）
  - 平台垫付次数≥2 → 拒绝服务（reject-service）
- **输出状态**：instant / auto / manual / evidence-required / platform-pay
- **证据要求**：若需证据且未提交，直接返回“需证据”提示。

#### 2.3 赔付落地与副作用
同样在 [algorithm/claim_auto_approval.go](algorithm/claim_auto_approval.go)：
- 建单时记录 `approval_type`、`trust_score_snapshot`、`lookback_result`。
- 若赔付来源为骑手押金，会触发扣款事务并向用户退款（DeductRiderDepositAndRefundTx）。
- 异步任务：
  - 异物索赔 → 触发商户异物历史检查
  - 餐损/超时索赔 → 触发骑手餐损历史检查
  入口见 [api/risk_management.go](api/risk_management.go)。

#### 2.4 行为回溯与可疑模式识别
实现位于 [algorithm/lookback_checker.go](algorithm/lookback_checker.go)：
- 依次回溯 30d → 90d → 1y，收集索赔/订单/商户/骑手关联。
- 可疑模式：时间集中（72h内多次）、同一商户/骑手集中（≥80%）、7天内高频（≥3次）。

#### 2.5 团伙欺诈检测
实现位于 [algorithm/fraud_detector.go](algorithm/fraud_detector.go)：
- 设备复用：同设备 ≥3用户 且 7天内 ≥3索赔。
- 地址聚类：同地址 ≥3用户 且 7天内 ≥3索赔。
- 协同索赔：1小时内 ≥3用户投诉同一商户 **且用户存在关联**（共享设备/地址/已确认团伙）。
- 若用户无关联但集中投诉同一商户 → 标记“商户可疑”，不判用户欺诈。
- 管理员可通过 /v1/fraud/detect 触发（见 [api/risk_management.go](api/risk_management.go)）。

#### 2.6 食安熔断
实现位于 [algorithm/food_safety_handler.go](algorithm/food_safety_handler.go)：
- 高信用用户 + 有证据 → 立即熔断24小时。
- 1小时内 ≥3 次举报 → 若非恶作剧，熔断48小时。
- 恶作剧判定：新用户首单、已确认团伙、共享地址等。
- 熔断后取消未来预订并发送通知。

#### 2.7 商户异物索赔追踪
实现位于 [algorithm/merchant_foreign_object_tracker.go](algorithm/merchant_foreign_object_tracker.go)：
- 7天内异物索赔 ≥3 次 → 通知商户整改（不直接停业）。

#### 2.8 骑手异常/延迟上报
实现位于 [api/rider.go](api/rider.go)：
- 延迟上报：只允许 picked_up/delivering 状态，写入订单状态日志并通知用户/商户。
- 异常上报：记录异常日志，按类型通知平台或商户（如联系不上顾客、商户未出餐）。

#### 2.9 申诉与人工审核
实现位于 [api/appeal.go](api/appeal.go)：
- 商户/骑手可查看索赔并提交申诉，运营商可审核。
- 索赔人工审核在 /v1/claims/{id}/review（见 [api/risk_management.go](api/risk_management.go)）。

#### 2.10 风险/影响
- 策略强调“先赔后惩”，对恶意用户以信用分与限制服务兜底（阈值见 [algorithm/trust_score_types.go](algorithm/trust_score_types.go)）。
- 骑手与商户的负向影响通过押金扣款与熔断机制执行，需配合通知与申诉流程。

#### 2.11 结论
现有实现已覆盖索赔自动化决策、风控检测、食安熔断、商户异物追踪与骑手异常上报，并形成“先赔付、后治理、可追溯”的闭环；后续可在此基础上补全治理指标与运营看板。

#### 2.12 信任分系统（与异常订单强相关）
**核心实现位置**：
- 信任分与阈值常量：[algorithm/trust_score_types.go](algorithm/trust_score_types.go)
- 计算与惩罚逻辑：[algorithm/trust_score_calculator.go](algorithm/trust_score_calculator.go)
- 风控确认后扣分：[algorithm/fraud_detector.go](algorithm/fraud_detector.go)
- API 入口与查询/审核：[api/risk_management.go](api/risk_management.go)

**机制与规则摘要**：
- **100分制为主**（只降不升）：
  - 初始100，警告阈值85，拒绝服务阈值70（见 [algorithm/trust_score_types.go](algorithm/trust_score_types.go)）。
- **扣分类型**：
  - 用户索赔相关：警告 -5、需证据 -10、平台垫付 -15、申诉失败 -10、团伙欺诈 -30（见同上）。
  - 商户：一周内3次异物 -15、食安事件 -25、超时/拒单等扣分（同上）。
  - 骑手：餐损/超时/取消等扣分（同上，注意部分为兼容旧体系）。
- **扣分落地**：
  - UpdateTrustScore 会记录变更、写入变更日志并触发阈值处理（见 [algorithm/trust_score_calculator.go](algorithm/trust_score_calculator.go)）。
  - 客户低于70 → 拉黑（禁止下单）；低于85 → 警告通知。
  - 商户低于70 → 停业/封禁并通知。
  - 骑手信用分已逐步弱化，改用“高值单资格积分”机制（逻辑在同文件中注明）。

**与异常订单联动点**：
- 索赔审核拒绝后可触发恶意索赔扣分（见 [api/risk_management.go](api/risk_management.go)）。
- 团伙欺诈被确认后：拉黑用户并扣分（见 [algorithm/fraud_detector.go](algorithm/fraud_detector.go)）。
- 索赔自动审核流程会记录信任分快照与行为警告（见 [algorithm/claim_auto_approval.go](algorithm/claim_auto_approval.go)）。

**恢复机制**：
- 商户/骑手恢复申请与信用分流程已下线。

## 3. 骑手接单资格算法
### 现有实现梳理（高值单资格积分）
**是否相关**：相关。接单资格与异常订单（超时/餐损）直接联动，异常会扣积分并影响高值单接单权限。

#### 3.1 入口与链路
- 骑手高值单资格积分查询：/v1/rider/score（见 [api/rider.go](api/rider.go)）
- 积分变更历史：/v1/rider/score/history（见 [api/rider.go](api/rider.go)）
- 订单抢单校验（高值单限制）：/v1/delivery/grab/:order_id（见 [api/delivery.go](api/delivery.go)）
- 运营商侧查看资格：运营商骑手详情接口（见 [api/operator_merchant_rider.go](api/operator_merchant_rider.go)）

#### 3.2 资格规则
定义于 [algorithm/trust_score_types.go](algorithm/trust_score_types.go)：
- **高值单阈值**：运费 ≥ 10 元（1000 分）判定为高值单。
- **资格判断**：高值单资格积分 `premium_score >= 0` 才能接高值单。
- **积分规则**：
  - 完成普通单 +1
  - 完成高值单 -3
  - 超时 -5
  - 餐损 -10

#### 3.3 接单校验逻辑
实现位于 [api/delivery.go](api/delivery.go)：
- 抢单时先判断是否高值单；若是，查询骑手 `premium_score`。
- `premium_score < 0` → 拒绝接单，提示需先完成普通单累积积分。

#### 3.4 与异常订单处理的联动
- 餐损/超时索赔在异常链路中发生（见 2.3、2.8），会触发积分扣减逻辑（规则定义在 [algorithm/trust_score_types.go](algorithm/trust_score_types.go)）。
- 因扣分导致 `premium_score < 0` 时，高值单接单资格立即失效。

#### 3.5 结论
骑手接单资格算法与异常订单处理强相关：异常会直接影响资格积分，从而限制高值单接单能力，建议与“异常订单处理算法”合并讨论与评估。

## 4. 千人千面推荐算法
- 讨论要点：
- 待补充证据/接口：
- 风险/影响：
- 结论：
