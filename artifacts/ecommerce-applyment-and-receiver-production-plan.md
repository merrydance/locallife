# 微信进件与分账接收方生产实施方案

> 已废弃说明：本方案中包含企业运营商微信进件与开户设计。当前业务已取消运营商进件，运营商不再进入 applyment 链路；本文件仅保留作历史方案存档，不得作为当前实现依据。

## 1. 最终业务范围

本方案以当前确认后的最终范围为准，不做历史兼容。

### 1.1 需要微信进件的主体

1. 餐饮商户主体 4：个体工商户
2. 餐饮商户主体 2：企业
3. 企业运营商

### 1.2 不走微信进件的主体

1. 个人运营商：分账直接到 openid
2. 骑手：分账直接到 openid

### 1.3 明确不支持的主体

1. 2401 小微商户
2. 2500 个人卖家
3. 3 事业单位
4. 2502 政府机关
5. 1708 社会组织
6. 金融机构

## 2. 目标架构

当前系统把“绑卡”和“微信进件”耦合在一起。生产方案需要拆成两条独立链路：

1. 微信进件链路
适用于主体 4、2 和企业运营商。先完成资料齐套校验，再提交微信进件，成功后获得子商户号并作为后续分账接收方。

2. openid 接收方链路
适用于个人运营商和骑手。不创建微信二级商户，不提交微信进件，不维护微信开户状态，只维护分账所需的 openid 接收方档案。

### 2.1 核心设计原则

1. 主体类型必须显式选择，不能再通过字符串推断。
2. 资料齐全后才允许微信进件，禁止提交时临时碰运气。
3. 分账层只识别接收方类型，不识别业务角色。
4. 商户经营场景二维码由后端自动生成，前端不重复上传。
5. 前端页面以 readiness 结果驱动，不再以银行表单作为进件入口。

## 3. 现状与问题

### 3.1 当前商户/运营商进件入口问题

当前提交入口位于以下文件：

1. locallife/api/ecommerce_applyment.go
2. web/src/components/merchant/finance-account-page-client.tsx
3. web/src/app/operator/applyment/page.tsx
4. weapp/miniprogram/pages/merchant/settings/applyment/index.wxml
5. weapp/miniprogram/pages/operator/applyment/index.wxml

存在的问题：

1. 入口语义是“填银行账户”，但实际触发了微信进件提交。
2. 主体类型由后端字符串推断，不够可靠。
3. 运营商个人与企业没有彻底分流。
4. Web 银行表单采集了 contact_phone 和 contact_email，但后端 bindbank 请求结构未完整承接。
5. 微信客户端模型支持的字段多于业务实际组装字段，容易误判为已完整支持。

### 3.2 当前可复用能力

现有实现中可直接复用的能力：

1. 商户营业执照、身份证、食品经营许可证、门头照、环境照、定位和地址资料
2. 商户二维码自动生成链路
3. 敏感字段加密链路
4. 微信进件异步状态查询与恢复链路
5. 微信分账、添加接收方、openid 使用能力

### 3.3 当前必须重做的部分

1. 进件主体建模
2. readiness 校验层
3. 运营商个人与企业分流
4. 分账接收方统一模型
5. 前端页面与文案

## 4. 新的数据模型

### 4.1 新增统一分账接收方档案

建议新增 receiver_profiles 表，作为微信分账接收方的统一抽象。

建议字段：

1. id
2. owner_type
3. owner_id
4. receiver_type
5. openid
6. sub_mchid
7. relation_type
8. receiver_name
9. receiver_name_encrypted
10. status
11. ready_state
12. last_sync_at
13. last_error
14. created_at
15. updated_at

字段语义：

1. owner_type：merchant、operator、rider
2. receiver_type：personal_openid、merchant_sub_mchid、operator_sub_mchid
3. status：draft、active、disabled、error
4. ready_state：ready、not_ready

用途：

1. 个人运营商和骑手使用 openid 类型接收方
2. 已完成微信进件的商户和企业运营商使用 sub_mchid 类型接收方
3. 分账逻辑只查 receiver_profiles，不再临时推断

### 4.2 商户进件资料档案

建议新增 merchant_applyment_profiles 表，保存商户微信进件所需的正式资料视图，而不是在提交时从多个来源临时拼装。

建议字段：

1. merchant_id
2. organization_type
3. contact_phone
4. contact_email
5. merchant_shortname
6. account_type
7. account_bank
8. account_bank_code
9. bank_alias
10. bank_alias_code
11. bank_address_code
12. bank_branch_id
13. bank_name
14. account_number_encrypted
15. account_name
16. readiness_state
17. readiness_snapshot
18. last_validated_at
19. created_at
20. updated_at

说明：

1. 商户证照、食品证、门头照、环境照、定位等仍复用既有申请表，不复制原始媒体。
2. 该档案负责保存微信进件正式依赖的补充字段和就绪态快照。

### 4.3 企业运营商进件资料档案

建议新增 operator_applyment_profiles 表，仅服务企业运营商进件。

建议字段：

1. operator_id
2. organization_type
3. contact_phone
4. contact_email
5. merchant_shortname
6. account_type
7. account_bank
8. account_bank_code
9. bank_alias
10. bank_alias_code
11. bank_address_code
12. bank_branch_id
13. bank_name
14. account_number_encrypted
15. account_name
16. readiness_state
17. readiness_snapshot
18. last_validated_at
19. created_at
20. updated_at

说明：

1. 个人运营商不进入该表。
2. 企业运营商的营业执照、法人身份证等资料必须在上游申请链路中补齐，不能再沿用当前过弱的必填模型。

### 4.4 ecommerce_applyments 表的建议调整

现有 ecommerce_applyments 已保存基础进件快照，但建议增加以下字段：

1. profile_type
2. profile_id
3. receiver_profile_id
4. readiness_snapshot
5. sales_scene_type
6. request_scope_version

目的：

1. 明确这次进件是来自商户还是企业运营商资料档案
2. 保存提交当时的规则版本与 readiness 结果
3. 为审计和后续排障提供稳定上下文

## 5. 主体规则矩阵

### 5.1 餐饮商户主体 4 个体工商户

必须满足：

1. 已选择 organization_type=4
2. 有营业执照图片与 OCR
3. 有营业执照号
4. 营业执照在有效期内
5. 有经营者身份证正反面
6. 身份证有效期合法
7. 有食品经营许可证
8. 食品经营许可证在有效期内
9. 有门头照
10. 有环境照
11. 有商户名称
12. 有经营地址
13. 有地图定位
14. 有联系手机号
15. 有联系邮箱
16. 有商户简称
17. 有结算账户资料
18. 可成功生成店铺二维码

账户规则：

1. 默认允许 ACCOUNT_TYPE_PRIVATE
2. 如业务要求允许对公账户，作为后续扩展而不是首期默认能力

经营场景：

1. store_name = 商户名称
2. store_qr_code = 后端自动生成并上传微信后的 MediaID

### 5.2 餐饮商户主体 2 企业

必须满足：

1. 已选择 organization_type=2
2. 有营业执照图片与 OCR
3. 有营业执照号
4. 营业执照在有效期内
5. 有法人身份证正反面
6. 身份证有效期合法
7. 有食品经营许可证
8. 食品经营许可证在有效期内
9. 有门头照
10. 有环境照
11. 有商户名称
12. 有经营地址
13. 有地图定位
14. 有联系手机号
15. 有联系邮箱
16. 有商户简称
17. 有结算账户资料
18. 可成功生成店铺二维码

账户规则：

1. 默认只允许 ACCOUNT_TYPE_BUSINESS
2. 如果未来要放开个人账户，必须另开规则并由产品、法务和财务确认

经营场景：

1. store_name = 商户名称
2. store_qr_code = 后端自动生成并上传微信后的 MediaID

### 5.3 企业运营商

必须满足：

1. 运营商主体明确为企业
2. 有营业执照图片与 OCR
3. 有营业执照号
4. 营业执照在有效期内
5. 有法人身份证正反面
6. 身份证有效期合法
7. 有联系人手机号
8. 有联系邮箱
9. 有主体名称
10. 有商户简称
11. 有结算账户资料
12. 有外部可访问经营场景 URL

账户规则：

1. 默认只允许 ACCOUNT_TYPE_BUSINESS

经营场景：

1. store_name = 企业运营商名称
2. store_url = 外部域名下的可访问页面 URL

### 5.4 个人运营商

不走微信进件。

必须满足：

1. 有可用 openid
2. 已创建或可创建 PERSONAL_OPENID 类型接收方
3. 已通过分账接收方 ready 校验

不需要：

1. 微信子商户号
2. 微信进件状态机
3. 银行账户进件资料

### 5.5 骑手

不走微信进件。

必须满足：

1. 有可用 openid
2. 已创建或可创建 PERSONAL_OPENID 类型接收方

## 6. 字段来源矩阵

### 6.1 商户进件字段来源

后端复用字段：

1. merchant_name：来自 merchant_application 或 merchant 主表
2. business_license_number：来自 merchant_application
3. business_license_copy：来自 merchant_application 媒体资源
4. legal_person_name：来自 merchant_application
5. legal_person_id_number：来自 merchant_application
6. id_card_front_copy：来自 merchant_application 媒体资源
7. id_card_back_copy：来自 merchant_application 媒体资源
8. business_address：来自 merchant_application
9. food_permit：来自 merchant_application
10. storefront_images：来自 merchant_application
11. environment_images：来自 merchant_application
12. 经纬度：来自 merchant_application

前端补录字段：

1. organization_type
2. contact_email
3. merchant_shortname
4. account_type
5. account_bank
6. account_bank_code
7. bank_alias
8. bank_alias_code
9. bank_address_code
10. bank_branch_id
11. bank_name
12. account_number
13. account_name

自动生成字段：

1. store_qr_code
2. readiness_snapshot
3. out_request_no

### 6.2 企业运营商进件字段来源

后端复用字段：

1. operator_name：来自企业运营商申请资料
2. business_license_number：来自企业运营商申请资料
3. business_license_copy：来自企业运营商媒体资源
4. legal_person_name：来自企业运营商申请资料
5. legal_person_id_number：来自企业运营商申请资料
6. id_card_front_copy：来自企业运营商媒体资源
7. id_card_back_copy：来自企业运营商媒体资源

前端补录字段：

1. contact_email
2. merchant_shortname
3. account_type
4. account_bank
5. account_bank_code
6. bank_alias
7. bank_alias_code
8. bank_address_code
9. bank_branch_id
10. bank_name
11. account_number
12. account_name

自动生成或系统依赖字段：

1. store_url
2. readiness_snapshot
3. out_request_no

### 6.3 个人运营商与骑手接收方字段来源

后端复用字段：

1. user_id
2. wechat_openid
3. full_name

前端或产品流程补充字段：

1. 是否完成微信授权绑定
2. 是否同意作为分账接收方

自动生成字段：

1. receiver_type = PERSONAL_OPENID
2. relation_type
3. receiver_profile status

## 7. 接口重构方案

### 7.1 商户进件接口

新增接口：

1. GET /v1/merchant/applyment/readiness
2. PUT /v1/merchant/applyment/profile
3. POST /v1/merchant/applyment/submit

职责：

1. readiness：返回缺失项、阻断原因、已复用字段摘要、是否可提交
2. profile：保存补录字段，例如 organization_type、contact_email、merchant_shortname 和银行资料
3. submit：只在 readiness=true 时允许调用，并内部再次校验

下线或降级：

1. 现有 bindbank 入口改为 profile 写接口，不再直接提交微信进件

### 7.2 企业运营商进件接口

新增接口：

1. GET /v1/operator/applyment/readiness
2. PUT /v1/operator/applyment/profile
3. POST /v1/operator/applyment/submit

限制：

1. 仅企业运营商可用
2. 个人运营商直接返回 unsupported_for_personal_operator

### 7.3 openid 接收方接口

新增统一接收方接口：

1. GET /v1/receivers/me
2. POST /v1/receivers/bind-openid
3. POST /v1/receivers/activate

职责：

1. 查询个人运营商或骑手的 openid 接收方状态
2. 确保 openid 已绑定且可用于分账
3. 如需要，调用微信添加分账接收方接口完成激活

## 8. 服务层重构方案

### 8.1 新增 ApplymentReadinessService

职责：

1. 根据主体类型输出 readiness 结果
2. 统一执行字段存在性校验
3. 统一执行有效期校验
4. 统一执行账户类型校验
5. 统一执行经营场景依赖校验

输出结构建议：

1. organization_type
2. is_ready
3. missing_fields
4. blocking_reasons
5. auto_filled_fields
6. recommended_actions
7. sales_scene_type

### 8.2 新增 ApplymentPayloadBuilder

按主体拆成 3 套 builder：

1. MerchantIndividualApplymentBuilder
2. MerchantEnterpriseApplymentBuilder
3. OperatorEnterpriseApplymentBuilder

要求：

1. builder 只接受已经通过 readiness 的 profile
2. builder 不允许在内部再尝试猜主体
3. builder 产出的微信请求字段必须可单测

### 8.3 新增 ReceiverProfileService

职责：

1. 统一管理 openid 与 sub_mchid 两类接收方
2. 封装 AddProfitSharingReceiver 调用
3. 为分账任务提供 receiver 解析能力
4. 对接收方状态进行懒同步或定期同步

## 9. 前端改造方案

### 9.1 商户端

当前问题：

1. 页面文案仍是“尚未提交进件申请，请填写银行结算账户信息”
2. 容易误导用户以为只填银行卡即可

目标页面结构：

1. 资料检查结果区
2. 已复用资料展示区
3. 需补录资料区
4. 银行账户区
5. 提交按钮区

交互规则：

1. readiness 未通过时禁用提交按钮
2. 逐项显示缺失项
3. 不再让用户重复上传已有证照和二维码

### 9.2 运营商端

必须先分流：

1. 个人运营商不显示微信开户页面
2. 企业运营商显示 readiness 页面和进件提交流程

个人运营商页面目标：

1. 展示 openid 接收方绑定状态
2. 允许重新拉起微信授权或校验绑定状态

企业运营商页面目标：

1. 展示企业进件资料检查结果
2. 展示银行资料补录与提交入口

### 9.3 骑手端

不新增微信进件页。

仅在骑手收益或结算相关页面展示：

1. 当前 openid 接收方状态
2. 如果缺失 openid，提示先完成微信绑定

## 10. 分账链路改造

### 10.1 当前问题

当前分账链路依赖已有商户/运营商体系，但接收方模型不统一，后续会在个人运营商和骑手场景下变得混乱。

### 10.2 目标方案

分账前统一解析 receiver profile：

1. 商户：receiver_type=merchant_sub_mchid
2. 企业运营商：receiver_type=operator_sub_mchid
3. 个人运营商：receiver_type=personal_openid
4. 骑手：receiver_type=personal_openid

### 10.3 分账处理流程

1. 根据业务角色拿到 owner_type 与 owner_id
2. 查询 active 且 ready 的 receiver profile
3. 若 receiver_type 为 sub_mchid，则按子商户接收方发起或确保添加接收方
4. 若 receiver_type 为 personal_openid，则按 PERSONAL_OPENID 接收方发起或确保添加接收方
5. 分账订单记录中落 receiver_profile_id 与 receiver_type，供对账和排障使用

## 11. 迁移与实施顺序

### 第一阶段：模型和接口边界

1. 新增 receiver_profiles 表
2. 新增 merchant_applyment_profiles 表
3. 新增 operator_applyment_profiles 表
4. 扩展 ecommerce_applyments 表
5. 下线直接 bindbank 提交语义，改为 profile 保存与 readiness

### 第二阶段：商户链路

1. 新增商户 organization_type 显式选择
2. 实现商户 readiness
3. 实现商户主体 4 builder
4. 实现商户主体 2 builder
5. 改造商户前端页

### 第三阶段：运营商链路

1. 区分个人运营商与企业运营商
2. 个人运营商切到 openid 接收方模式
3. 企业运营商补齐企业资料模型
4. 实现企业运营商 readiness 与 builder
5. 改造运营商前端页

### 第四阶段：骑手链路

1. 新增骑手 receiver profile
2. 将骑手收益链路统一到 openid 接收方

### 第五阶段：分账统一化

1. 分账任务统一解析 receiver profile
2. 统一接收方注册与缓存策略
3. 对账与回退链路补 receiver_profile_id

## 12. 测试方案

### 12.1 单元测试

1. readiness 规则测试
2. organization_type 显式映射测试
3. 商户 4 builder 测试
4. 商户 2 builder 测试
5. 企业运营商 builder 测试
6. receiver profile 解析测试
7. openid 接收方添加请求测试

### 12.2 集成测试

1. 商户主体 4 从 readiness 到 submit 的完整流程
2. 商户主体 2 从 readiness 到 submit 的完整流程
3. 企业运营商从 readiness 到 submit 的完整流程
4. 个人运营商 openid 接收方绑定与分账流程
5. 骑手 openid 接收方绑定与分账流程

### 12.3 回归测试

1. 商户二维码自动生成仍可工作
2. 微信进件状态恢复与异步跟踪不回退
3. 分账回调、查询、回退链路不受 receiver_profiles 引入影响

### 12.4 安全测试

1. 身份证号、手机号、银行卡号仍全部加密存储或掩码输出
2. openid 不落不必要日志
3. readiness 错误信息不泄露敏感内容

## 13. 验收口径

上线前必须满足：

1. 商户只允许主体 4 和 2 进入微信进件
2. 个人运营商无法进入微信进件页或提交接口
3. 企业运营商可独立完成微信进件
4. 骑手没有微信进件入口
5. 商户和企业运营商只有 readiness=true 才能提交微信进件
6. 商户二维码仍由后端自动生成
7. 分账系统能根据 receiver profile 正确分流到 sub_mchid 或 openid
8. 所有新增路径有单元测试与集成测试覆盖

## 14. 仍需对照微信原文逐条确认的点

当前范围已足够开始系统设计和代码实施，但以下项目在正式编码前仍应逐条对照微信支付原文：

1. 企业运营商使用 store_url 时的经营场景字段必填项
2. 商户主体 2 和 4 在联系邮箱、联系人类型、账户类型方面的精确必填规则
3. 企业运营商是否存在额外资质、补充材料或结算规则要求
4. PERSONAL_OPENID 接收方的关系类型、姓名字段和添加接收方前置条件
5. 若订单分账同时涉及商户和运营商，接收方注册顺序与幂等要求

在上述 5 项没有逐条与微信原文核对前，不建议直接编写最终 payload builder。