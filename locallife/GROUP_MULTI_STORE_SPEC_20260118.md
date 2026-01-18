# 集团多门店管理落地执行文档（2026-01-18）

> 目标：给研发/产品/测试直接落地执行的规格，覆盖数据模型、接口、权限、流程、状态机、验收标准。

## 1. 范围与前置约束
- 前置：用户必须为微信用户，小程序静默登录/注册。
- 员工加入仅支持二维码扫码绑定 openid；账号/密码不可加入。
- 新加入员工必须分配角色才有权限；未分配角色无权限。
- 门店入驻流程沿用现有；集团入驻流程新增但不需要进件。

## 2. 数据模型（DDL 设计）
### 2.1 merchant_group_applications（集团入驻申请）
- id (PK)
- applicant_user_id (FK users.id)
- group_name
- contact_phone
- license_number
- license_image_url
- address
- region_id (FK regions.id)
- status (draft/submitted/approved/rejected)
- reject_reason (nullable)
- reviewed_by (FK users.id, nullable)
- reviewed_at (nullable)
- application_data (jsonb)
- created_at, updated_at

### 2.2 merchant_groups（集团）
- id (PK)
- name
- owner_user_id (FK users.id)
- status (active/disabled)
- contact_phone
- license_number
- license_image_url
- address
- region_id (FK regions.id)
- application_data (jsonb)
- created_at, updated_at

### 2.3 merchant_brands（品牌，可选）
- id (PK)
- group_id (FK merchant_groups.id)
- name
- logo_url
- description
- status (active/disabled)
- created_at, updated_at

### 2.4 merchant_group_members（集团成员）
- id (PK)
- group_id (FK merchant_groups.id)
- user_id (FK users.id)
- role (owner/admin/finance/ops)
- status (active/disabled)
- joined_at
- invited_by (FK users.id, nullable)

### 2.5 merchant_group_join_requests（门店加入申请）
- id (PK)
- group_id (FK merchant_groups.id)
- merchant_id (FK merchants.id)
- applicant_user_id (FK users.id)
- status (pending/approved/rejected/cancelled)
- reason (nullable)
- reviewed_by (FK users.id, nullable)
- reviewed_at (nullable)
- created_at

### 2.6 group_policies（集团策略）
- group_id (PK, FK merchant_groups.id)
- pricing_mode (central/store)
- menu_mode (central/store)
- inventory_mode (central/store)
- promotion_mode (central/store)

### 2.7 group_menu_templates / brand_menu_templates（可选）
- id (PK)
- group_id/brand_id (FK)
- payload (json)
- version
- status (active/archived)
- created_at, updated_at

### 2.8 merchants（现有表新增字段）
- group_id (nullable, FK merchant_groups.id)
- brand_id (nullable, FK merchant_brands.id)

## 3. 权限与鉴权
- 集团级角色：owner/admin/finance/ops（跨店资源权限）。
- 门店级角色：沿用 merchant_staff，仅限 merchant_id。
- Casbin 规则新增 group 维度：
  - /v1/groups/:id/* 仅 group_member 可访问。
  - /v1/groups/:id/merchants 仅 group_member 可访问。
- 每个请求必须校验 group_id/merchant_id 归属，防越权。

## 4. 业务流程与状态机
### 4.1 集团入驻流程
1) GET /v1/groups/applications/me 获取/创建草稿。
2) PUT /v1/groups/applications/basic 更新基础信息（含 region_id）。
3) POST /v1/groups/applications/license/ocr（可选，复用证照OCR链路）。
4) POST /v1/groups/applications/submit 提交。
5) POST /v1/groups/applications/:id/review 审核通过/拒绝。
- 通过：创建 merchant_groups，状态 active；无需进件。
- 拒绝：记录 reject_reason，可重置为 draft。

### 4.2 门店入驻流程（现有）
- 沿用当前门店入驻流程与审核规则。
- approved 后需进件完成转 active 才可营业。

### 4.3 门店加入集团流程
1) 门店搜索集团：GET /v1/groups?keyword=xxx
2) 门店申请加入：POST /v1/groups/:id/join-requests
3) 门店撤回申请（审核前）：POST /v1/groups/:id/join-requests/:request_id/cancel
4) 集团审核：POST /v1/groups/:id/join-requests/:request_id/approve|reject
5) 审核通过：写入 merchants.group_id/brand_id，记录审计日志。

### 4.4 员工入职扫码流程（复用现有实现）
1) 门店生成邀请码：POST /v1/merchant/staff/invite-code
2) 小程序扫码并提交邀请码：POST /v1/bind-merchant
3) 生成 merchant_staff（role=pending），等待分配角色后生效。

## 5. 接口清单（需实现）
### 5.1 集团入驻
- POST /v1/groups/applications
- GET /v1/groups/applications/me
- PUT /v1/groups/applications/basic
- POST /v1/groups/applications/license/ocr
- POST /v1/groups/applications/submit
- POST /v1/groups/applications/:id/review

### 5.2 集团/品牌
- POST /v1/groups
- GET /v1/groups/:id
- PATCH /v1/groups/:id
- GET /v1/groups/:id/merchants
- POST /v1/groups/:id/brands
- GET /v1/brands/:id

### 5.3 门店加入集团
- POST /v1/groups/:id/join-requests
- GET /v1/groups/:id/join-requests
- POST /v1/groups/:id/join-requests/:request_id/approve
- POST /v1/groups/:id/join-requests/:request_id/reject

### 5.4 邀请/扫码绑定（复用现有）
- POST /v1/merchant/staff/invite-code
- POST /v1/bind-merchant

### 5.5 策略/模板
- PUT /v1/groups/:id/policies
- POST /v1/groups/:id/menu-templates
- POST /v1/brands/:id/menu-templates

## 6. 治理与一致性要求
- 申请/审核/撤销/退出必须记录审计日志。
- 权限变更后需刷新缓存与通知。
- 模板下发需支持版本回滚。
- 报表口径与结算口径统一（退款/优惠/分账/服务费）。

## 7. 验收标准
- 单店可独立运营且无集团依赖。
- 集团入驻后可创建品牌与查看门店列表。
- 门店加入集团后，集团可管理其菜单/促销（按策略）。
- 未分配角色的员工无权限。
- 审计日志完整可追溯。

## 8. 需废弃与删除
- Boss 扫码认领店铺流程与相关接口/逻辑直接删除（/v1/claim-boss、/v1/merchant/boss-bind-code、/v1/boss/merchants、/v1/merchant/bosses）。
