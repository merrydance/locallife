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
- 讨论要点：
- 待补充证据/接口：
- 风险/影响：
- 结论：

## 3. 骑手接单资格算法
- 讨论要点：
- 待补充证据/接口：
- 风险/影响：
- 结论：

## 4. 千人千面推荐算法
- 讨论要点：
- 待补充证据/接口：
- 风险/影响：
- 结论：
