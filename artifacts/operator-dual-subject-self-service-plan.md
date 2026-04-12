# 运营商双主体自助入驻改造方案

## 1. 目标

本方案用于把运营商入驻从当前的“默认个人、营业执照可选、后端靠资料推断主体”改造成“前台显式选择主体、后端显式建模、审核与开户分流、分账接收方稳定落库”的完整闭环。

目标范围：

1. 前台同时支持个人运营商和企业运营商自助入驻。
2. 用户必须在系统内明确选择主体类型，不能再通过营业执照号存在性推断。
3. 个人运营商和企业运营商从申请草稿、提交审核、审核通过、微信开户、分账接收方、退款处理，全链路彻底区分。
4. 个人运营商走 openid 分账，不走微信二级商户进件。
5. 企业运营商走微信二级商户进件，分账到子商户号。

## 2. 当前问题

当前链路已经在开户与分账阶段做了部分分流，但前后端模型并不一致。

### 2.1 前台问题

当前小程序运营商入驻页存在以下问题：

1. 没有主体选择。
2. 文案仍是“营业执照可选”。
3. 基础信息接口不承载主体类型。
4. 第一步校验不区分个人与企业。
5. 用户无法清楚知道自己是在申请个人运营商还是企业运营商。

### 2.2 后端问题

当前后端申请链路存在以下问题：

1. 运营商主体类型没有显式字段。
2. submit 校验仍以通用弱校验为主，只要求姓名、联系人、手机号、身份证正反面。
3. 开户链路用 business_license_number 是否存在来推断企业运营商。
4. 分账链路也在用同一套推断逻辑。
5. 审核后台缺少“主体类型”这一等价于业务真相的字段。

### 2.3 结果问题

这会导致三个实际风险：

1. 前台表达的业务边界与后端运行边界不一致。
2. 企业运营商自助入驻不是完整闭环，只是“可选上传营业执照”的半支持状态。
3. 后续只要资料写回或 OCR 结果异常，就可能影响主体识别和分账路径判断。

## 3. 目标业务模型

运营商主体类型改为显式枚举，并作为整个链路的单一事实来源。

### 3.1 主体类型定义

新增 operator_subject_type：

1. PERSONAL
2. ENTERPRISE

语义：

1. PERSONAL：自然人运营商，审核通过后可经营，可分账到 openid，不进入微信二级商户开户。
2. ENTERPRISE：企业运营商，审核通过后必须补齐微信开户资料并完成进件，之后分账到子商户号。

### 3.2 全链路规则

1. 草稿创建时就必须写入 operator_subject_type。
2. 所有前端页面都按 operator_subject_type 驱动显示字段、提示文案、提交校验、进度说明。
3. 所有后端校验都按 operator_subject_type 分支执行。
4. 微信开户 eligibility 只由 operator_subject_type 决定，不再由营业执照号推断。
5. 分账接收方解析只认 operator_subject_type 与 receiver profile，不认零散资料字段。

## 4. 数据模型改造

## 4.1 operator_applications

建议在运营商申请主表新增以下字段：

1. subject_type text not null
2. subject_display_name text null
3. onboarding_version text not null default 'v2'
4. readiness_state text not null default 'draft'
5. readiness_snapshot jsonb not null default '{}'

字段说明：

1. subject_type：PERSONAL 或 ENTERPRISE。
2. subject_display_name：给前台直接展示的主体文案，例如“个人运营商”“企业运营商”。
3. onboarding_version：区分旧草稿和新方案草稿，便于迁移。
4. readiness_state：draft、incomplete、ready、submitted。
5. readiness_snapshot：按主体计算出的缺失项、已完成项、阻断原因快照。

## 4.2 企业专属资料字段

如果现有 operator_applications 只保存通用字段，建议把企业主体专属字段显式补齐到申请表或单独 profile 表。首期为了降低改造面，可以先放在 operator_applications。

建议新增字段：

1. company_name text
2. unified_social_credit_code text
3. legal_person_name text
4. legal_person_id_number text
5. business_license_media_asset_id bigint
6. business_license_number text
7. business_license_valid_until text
8. settlement_account_type text
9. settlement_account_name text
10. settlement_bank_name text
11. settlement_bank_code bigint
12. settlement_bank_branch_name text
13. settlement_bank_branch_code text
14. settlement_bank_account_encrypted text
15. applyment_contact_phone text
16. applyment_contact_email text
17. merchant_shortname text

说明：

1. 个人运营商不要求企业字段全部有值。
2. 企业运营商提交审核前只要求入驻审核必要字段；提交微信开户前再要求开户必要字段全部 ready。
3. settlement_* 与 applyment_* 字段可以继续复用当前 bindbank 能力，但应从“提交时瞬时参数”升级为“稳定资料档案”。

## 4.3 分账接收方档案

建议新增 receiver_profiles 作为统一接收方视图。

关键字段：

1. owner_type
2. owner_id
3. subject_type
4. receiver_type
5. account
6. receiver_name
7. relation_type
8. status
9. last_error
10. last_synced_at

运营商取值：

1. 个人运营商：receiver_type=PERSONAL_OPENID，account=user.wechat_openid。
2. 企业运营商：receiver_type=MERCHANT_ID，account=operator.sub_mch_id。

收益：

1. worker 不再依赖申请表资料猜测接收方类型。
2. 接收方注册、重试、排障有统一落点。
3. 退款回退规则可以按 receiver_type 明确分叉。

## 5. 前端方案

## 5.1 小程序入驻页改造

目标页面是当前的运营商注册页，需要把“统一表单”改成“主体驱动表单”。

### Step 1 选择主体

第一步改为先选主体，再选区域。

页面结构：

1. 主体选择卡片：个人运营商 / 企业运营商。
2. 主体差异提示：
   个人运营商：个人身份审核，通过后按微信 openid 分账。
   企业运营商：企业资料审核，通过后需完成微信支付开户并按子商户分账。
3. 区域选择。
4. 基础联系人信息。

交互要求：

1. 选择主体后，后续步骤标题、文案、必填项立即切换。
2. 已有草稿时，如果 subject_type 已写入，则不允许在前台直接切换；必须重置草稿后重选，避免混合脏数据。

### Step 2 资料上传

个人运营商资料：

1. 运营商名称或默认展示名称
2. 联系人姓名
3. 联系人手机号
4. 身份证正面
5. 身份证背面

企业运营商资料：

1. 企业名称
2. 统一社会信用代码
3. 联系人姓名
4. 联系人手机号
5. 营业执照
6. 法人身份证正面
7. 法人身份证背面

规则：

1. 企业主体下，营业执照必须为必填，UI 上不能再出现“可选”。
2. 个人主体下，不显示企业营业执照上传区。
3. OCR 写回后按主体分别展示识别结果和错误提示。

### Step 3 提交确认

确认页需显式展示：

1. 当前主体类型。
2. 当前申请审核通过后的下一步动作。
3. 企业主体会在审核通过后进入微信支付开户流程。
4. 个人主体不会进入微信开户，而是绑定微信 openid 分账。

### Step 4 结果页

结果页文案分主体展示：

1. PERSONAL：等待人工审核，通过后即可按个人运营商身份经营。
2. ENTERPRISE：等待人工审核，通过后还需完成微信支付开户。

## 5.2 小程序接口改造

建议改造以下接口语义：

1. POST /v1/operator/application
   创建草稿时允许传 subject_type。
2. PUT /v1/operator/application/basic
   承载 subject_type 对应的基础字段。
3. GET /v1/operator/application
   返回 subject_type、subject_display_name、readiness_state、readiness_snapshot。
4. POST /v1/operator/application/submit
   按 subject_type 做不同提交校验。

建议返回结构新增：

1. subject_type
2. subject_display_name
3. missing_fields
4. next_action
5. onboarding_version

## 5.3 Web 管理与开户页改造

### 审核后台

平台审核页必须显式展示：

1. 主体类型。
2. 该主体的审核必填项完成情况。
3. 审核通过后的后续动作。

### 运营商开户页

当前 Web 开户页已经支持 can_submit 和 block_reason，但还需要进一步与显式主体对齐：

1. 直接返回 subject_type。
2. PERSONAL 时展示“无需开户，按 openid 分账”，不展示任何企业开户表单。
3. ENTERPRISE 时展示 readiness 明细，缺少哪项开户资料就阻断哪项。
4. 开户表单提交后把银行资料写回 operator applyment profile，而不是只作为一次性请求体。

## 6. 后端方案

## 6.1 申请层

### 新增显式主体校验

后端要新增统一主体校验入口：

1. validateOperatorApplicationSubjectType
2. validateOperatorApplicationDraftBySubject
3. buildOperatorApplicationReadiness

职责：

1. 拒绝未知主体类型。
2. 生成当前主体缺失项快照。
3. 返回给前端稳定的 missing_fields 和 block_reason。
4. 在 submit 前做主体对应的必填校验。

### 提交审核规则

个人运营商提交审核所需：

1. subject_type=PERSONAL
2. 区域已选
3. 联系人姓名与手机号齐全
4. 身份证正反面齐全
5. 个人姓名可来自 name 或 OCR 写回结果

企业运营商提交审核所需：

1. subject_type=ENTERPRISE
2. 区域已选
3. 企业名称齐全
4. 联系人姓名与手机号齐全
5. 营业执照齐全
6. 法人身份证正反面齐全
7. 营业执照号或 OCR 识别结果可用

注意：

1. 审核通过不等于微信开户完成。
2. 企业运营商可以先审核通过，再进入开户资料补齐与微信进件。

## 6.2 微信开户层

建议把“审核通过”和“微信开户”拆成两个明确阶段。

### 阶段 A 运营商入驻审核

1. PERSONAL 和 ENTERPRISE 都走人工审核。
2. 审核通过后创建 operator 账号。
3. operator.subject_type 同步落库。

### 阶段 B 企业微信开户

1. 仅 ENTERPRISE 可进入。
2. PERSONAL 请求开户接口直接返回 block_reason，而不是再做资料推断。
3. 开户 readiness 需要独立检查：
   联系邮箱、开户手机号、商户简称、对公账户、开户行信息、结算账号等。
4. 只有 readiness=ready 才允许调用微信进件接口。

建议新增接口：

1. GET /v1/operator/applyment/readiness
2. PUT /v1/operator/applyment/profile
3. POST /v1/operator/applyment/submit

语义：

1. readiness：返回企业开户所需缺失项。
2. profile：保存企业开户补充资料。
3. submit：真正提交微信进件。

这样可以把当前 bindbank 接口从“既采集又提交”拆成“采集资料”和“提交微信”。

## 6.3 分账层

worker 层禁止再通过申请资料推断主体，应改为以下顺序：

1. 读 operator.subject_type。
2. 读 receiver_profile。
3. 按 receiver_type 执行接收方注册、分账、回退。

规则：

1. PERSONAL：
   receiver_type=PERSONAL_OPENID
   account=user.wechat_openid
   不允许走企业回退语义
2. ENTERPRISE：
   receiver_type=MERCHANT_ID
   account=operator.sub_mch_id
   缺少 sub_mch_id 时直接失败并报警

## 6.4 状态机

建议把运营商状态拆成两层：

### 入驻申请状态

1. draft
2. submitted
3. approved
4. rejected

### 企业开户状态

1. not_required
2. pending_profile
3. ready_to_submit
4. submitted
5. auditing
6. to_be_signed
7. signing
8. finish
9. rejected
10. frozen

PERSONAL 默认固定为 not_required。

## 7. 迁移方案

## 7.1 数据迁移

老数据需要一次性回填 subject_type。

回填规则建议：

1. 已有运营商申请且存在有效营业执照号或营业执照资产的，先回填为 ENTERPRISE。
2. 其余回填为 PERSONAL。
3. 所有历史回填记录打 migration_source 标签，方便后续人工校正。

注意：

1. 这只是迁移兜底，不再作为运行期判断逻辑。
2. 对已通过审核但资料边界模糊的记录，应加一轮运营核对名单。

## 7.2 接口兼容

建议分两期上线：

### 第一期

1. 后端新增 subject_type 字段与返回结构。
2. 旧客户端未传 subject_type 时，后端临时按旧规则兜底并标记 onboarding_version=v1_legacy。
3. 新客户端开始按新协议工作。

### 第二期

1. 小程序全量后，关闭 legacy 推断入口。
2. submit 和 applyment 全部改为强制 subject_type。
3. worker 全面切换到 subject_type + receiver_profile。

## 8. 实施拆分

建议按以下顺序落地，避免一次改太多边界。

### Phase 1 显式主体建模

1. DB 新增 operator subject_type。
2. API 返回 subject_type。
3. 小程序入驻页新增主体选择。
4. submit 改为按主体校验。

交付结果：前台和后端对“个人/企业运营商”有同一份事实来源。

### Phase 2 企业入驻资料闭环

1. 补企业必填字段。
2. 小程序企业表单改成完整必填模型。
3. 审核后台展示主体与资料完整度。

交付结果：企业运营商真正具备自助提交审核能力。

### Phase 3 企业微信开户资料档案化

1. 拆 operator applyment readiness/profile/submit。
2. 银行资料稳定落库。
3. Web 开户页按 readiness 渲染。

交付结果：微信开户不再依赖一次性绑卡表单。

### Phase 4 分账接收方收口

1. receiver_profile 落地。
2. worker 改读 subject_type + receiver_profile。
3. 回退、重试、补偿按 receiver_type 分叉。

交付结果：个人和企业运营商在分账层彻底收口，不再混用推断逻辑。

## 9. 验证矩阵

必须覆盖以下测试：

### 申请链路

1. 个人运营商草稿创建成功。
2. 企业运营商草稿创建成功。
3. 个人主体缺身份证时不能提交。
4. 企业主体缺营业执照时不能提交。
5. 企业主体切回个人主体必须先重置草稿。

### 开户链路

1. 个人运营商访问开户状态返回 not_required。
2. 个人运营商调用开户提交接口被阻断。
3. 企业运营商缺开户资料时返回 pending_profile 与 missing_fields。
4. 企业运营商 readiness 完整后才能提交微信开户。

### 分账链路

1. 个人运营商走 PERSONAL_OPENID。
2. 企业运营商走 MERCHANT_ID。
3. 企业运营商缺 sub_mch_id 时显式失败并报警。
4. 个人运营商退款回退被正确阻断。

### 迁移链路

1. 老数据回填 subject_type 后，状态页能正确展示。
2. legacy 客户端仍可读取草稿。
3. 新客户端提交后不会再走营业执照推断主体。

## 10. 结论

这次改造的根本点不是“多加几个字段”，而是把运营商主体从“资料特征”升级成“业务主键”。

只有这样，以下三件事才能同时成立：

1. 前台真正支持个人运营商和企业运营商自助入驻。
2. 审核、开户、分账三条链路使用同一份主体真相。
3. 后端不再因为 OCR、资料缺失或历史兼容逻辑而误判主体。

首推实施顺序：先做 subject_type 显式建模和前台主体分流，再做企业资料闭环，最后收口微信开户与分账接收方。