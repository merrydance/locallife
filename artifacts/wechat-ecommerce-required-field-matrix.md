# 微信电商进件与分账必填矩阵

> 已废弃说明：本文件包含企业运营商进件字段矩阵。当前业务已取消运营商进件，相关内容仅保留作历史评估记录，不得作为当前产品或代码实现依据。

本文件基于当前已确认的微信支付原文整理，仅覆盖当前业务范围：

1. 餐饮商户主体 4 个体工商户
2. 餐饮商户主体 2 企业
3. 企业运营商进件
4. 个人运营商、骑手走 PERSONAL_OPENID 分账

## 1. 先给结论

可以。基于你提供的微信支付原文，已经可以清晰判断当前范围内两种主体进件的必填项、条件必填项、二选一必填项，以及 openid 分账的关键约束。

同时也明确了一个重要限制：

1. 企业运营商即使不开线下店铺，sales_scene_info 仍然是必填。
2. sales_scene_info 中 store_name 必填。
3. store_url 和 store_qr_code 二选一必填。

因此，企业运营商不能以“没有店铺”作为不传经营场景的理由。产品上必须为企业运营商提供一个真实可访问的经营场景承载物，最适合的是 store_url，而不是虚构二维码。

## 2. 主体 4 与主体 2 的统一必填框架

对主体 4 个体工商户 和主体 2 企业，以下字段在标准进件路径中都应视为必填：

1. out_request_no
2. organization_type
3. business_license_info
4. account_info
5. contact_info
6. sales_scene_info
7. merchant_shortname

此外：

1. 敏感字段必须加密。
2. Wechatpay-Serial 请求头必须带微信支付公钥 ID 或平台证书序列号。
3. 图片必须先走图片上传接口换取 MediaID。

## 3. 主体 4 个体工商户进件矩阵

### 3.1 直接必填

1. out_request_no
2. organization_type=4
3. business_license_info.business_license_copy
4. business_license_info.business_license_number
5. business_license_info.merchant_name
6. business_license_info.legal_person
7. account_info.bank_account_type
8. account_info.account_bank
9. account_info.account_name
10. account_info.account_number
11. contact_info.contact_type
12. contact_info.contact_name
13. contact_info.mobile_phone
14. sales_scene_info.store_name
15. merchant_shortname

### 3.2 标准身份证路径下的必填

如果不主动切到其他证件类型，微信默认按身份证路径处理，因此应视为必填：

1. id_card_info.id_card_copy
2. id_card_info.id_card_national
3. id_card_info.id_card_name
4. id_card_info.id_card_number
5. id_card_info.id_card_valid_time_begin
6. id_card_info.id_card_valid_time

### 3.3 建议按生产硬闸门处理为必填

虽然微信文档对以下字段写的是“建议填写”或条件较弱，但生产上建议直接作为 readiness 必填：

1. business_license_info.company_address
2. business_license_info.business_time

原因：

1. 文档明确说明企业/个体户不填时系统会尝试查国家工商信息。
2. 如果查不到，会被审核驳回。
3. 既然我们能从现有商户资料中拿到，就不应把审核风险交给微信兜底。

### 3.4 条件必填

1. 如果 id_doc_type 明确选择非身份证，则 id_doc_info 整体必填，id_card_info 不再使用。
2. 如果 contact_info.contact_type=66 经办人，则以下字段必填：
contact_info.contact_id_doc_type
contact_info.contact_id_card_number
contact_info.contact_id_doc_copy
contact_info.contact_id_doc_period_begin
contact_info.contact_id_doc_period_end
3. 如果经办人证件类型需要反面照，则 contact_info.contact_id_doc_copy_back 必填。
4. 如果开户银行查询结果显示需要支行，则 account_info.bank_branch_id 和 account_info.bank_name 二选一必填。
5. 如果 settlement_info.qualification_type 选择了需要特殊资质的行业，则 qualifications 必填。
6. 如果微信审核阶段额外要求补充材料，则 business_addition_pics 需要补传。

### 3.5 二选一必填

1. sales_scene_info.store_url 和 sales_scene_info.store_qr_code 二选一必填。
2. 当开户银行需要支行信息时，account_info.bank_branch_id 和 account_info.bank_name 二选一必填。

### 3.6 账户限制

主体 4 个体工商户可用：

1. 74 对公账户
2. 75 对私账户

生产建议：

1. 默认支持 75 对私。
2. 如业务需要，也可支持 74 对公。
3. 使用 75 时，account_name 必须与经营者身份证姓名一致。
4. 使用 74 时，account_name 必须与营业执照 merchant_name 一致。

### 3.7 超级管理员规则

主体 4 可选：

1. 65 经营者/法定代表人
2. 66 经办人

生产建议：

1. 第一阶段只支持 65。
2. 这样可以避免经办人证件、授权资料、实名签约校验的额外复杂度。

## 4. 主体 2 企业进件矩阵

### 4.1 直接必填

1. out_request_no
2. organization_type=2
3. business_license_info.business_license_copy
4. business_license_info.business_license_number
5. business_license_info.merchant_name
6. business_license_info.legal_person
7. account_info.bank_account_type
8. account_info.account_bank
9. account_info.account_name
10. account_info.account_number
11. contact_info.contact_type
12. contact_info.contact_name
13. contact_info.mobile_phone
14. sales_scene_info.store_name
15. merchant_shortname

### 4.2 标准身份证路径下的必填

如果默认采用法定代表人身份证路径，应视为必填：

1. id_card_info.id_card_copy
2. id_card_info.id_card_national
3. id_card_info.id_card_name
4. id_card_info.id_card_number
5. id_card_info.id_card_valid_time_begin
6. id_card_info.id_card_valid_time

### 4.3 企业场景下建议提升为硬必填

1. business_license_info.company_address
2. business_license_info.business_time

理由同主体 4，一旦依赖工商自动补齐，失败会被审核驳回。

### 4.4 企业特有条件必填

1. 如果法定代表人不是最终受益人，则 ubo_info_list 必填。
2. 如果法定代表人只是最终受益人之一，则 ubo_info_list 需要填写其他最终受益人。
3. 如果 id_doc_type 选择非身份证，则 id_doc_info 必填。
4. 如果 contact_info.contact_type=66 经办人，则经办人证件字段按条件必填。
5. 如果开户银行需要支行，则 bank_branch_id 和 bank_name 二选一必填。
6. 如果选择了特殊行业，则 qualifications 必填。

### 4.5 二选一必填

1. sales_scene_info.store_url 和 sales_scene_info.store_qr_code 二选一必填。
2. account_info.bank_branch_id 和 account_info.bank_name 在需要支行时二选一必填。

### 4.6 账户限制

主体 2 企业根据文档只允许：

1. 74 对公账户

因此生产上必须硬限制：

1. 企业主体不允许提交 75 对私账户。
2. account_name 必须与营业执照 merchant_name 一致。

### 4.7 超级管理员规则

主体 2 可选：

1. 65 法定代表人
2. 66 经办人

生产建议：

1. 第一阶段优先支持 65。
2. 第二阶段再评估 66 经办人。

原因：

1. 66 会引入经办人证件、授权校验、签约实名一致性等额外复杂度。
2. 首期上线不需要把这条高风险分支一起放进来。

## 5. 企业运营商进件矩阵

企业运营商虽然不开店铺，但如果仍然要作为二级商户进件，就必须满足微信接口的经营场景要求。

### 5.1 可以明确判定的必填

如果企业运营商走主体 2 企业：

1. organization_type=2
2. business_license_info 核心字段必填
3. 法定代表人身份证路径核心字段必填
4. account_info 核心字段必填
5. contact_info 核心字段必填
6. sales_scene_info 必填
7. merchant_shortname 必填

### 5.2 关键结论

企业运营商“不需要店铺”不等于“不需要经营场景”。

因为微信原文写得非常明确：

1. sales_scene_info 是 true 必填。
2. sales_scene_info.store_name 是 true 必填。
3. sales_scene_info.store_url 和 sales_scene_info.store_qr_code 二选一必填。

所以企业运营商首期正确方案应为：

1. 使用 store_url，不使用 store_qr_code。
2. store_url 指向该企业运营商真实可访问的业务主页，例如区域运营招商页、区域服务页、品牌官网业务页。
3. store_name 使用企业运营主体名称或对外业务名称。

不建议：

1. 伪造一个无实际业务承载的空页面。
2. 为无店铺运营商硬塞一个店铺二维码。

### 5.3 企业运营商首期建议收口

1. 只支持 contact_type=65 法定代表人。
2. 只支持身份证路径，不支持其他证件类型。
3. 只支持对公账户。
4. 只支持 sales_scene_info.store_url。
5. 不支持 store_qr_code。
6. 不支持经办人。
7. 不支持其他证件类型。

这样可以把企业运营商首期做成一条稳定、可审计的窄路径。

## 6. 不需要提交的字段

对于当前业务范围，可以明确不做的字段：

1. finance_institution=true 相关全部字段
2. finance_institution_info
3. id_holder_type=SUPER 相关分支
4. authorize_letter_copy
5. owner
6. 2401、2500、3、2502、1708 相关主体分支
7. 个人卖家 business_addition_desc 强制说明分支

## 7. 生产硬闸门建议

虽然微信原文中有些字段是“建议填写”或“选填”，但为了避免审核驳回，当前业务范围建议这样收口：

### 7.1 主体 4 个体工商户

生产硬必填：

1. business_license_info.company_address
2. business_license_info.business_time
3. sales_scene_info.store_qr_code
4. contact_info.contact_type=65
5. account_info.bank_account_type 允许 74 或 75

说明：

1. 对餐饮商户，二维码由后端自动生成，最稳定。
2. 不建议个体商户首期走经办人。

### 7.2 主体 2 企业

生产硬必填：

1. business_license_info.company_address
2. business_license_info.business_time
3. sales_scene_info.store_qr_code
4. contact_info.contact_type=65
5. account_info.bank_account_type 只能 74
6. 如法定代表人非最终受益人，则 ubo_info_list 必填

### 7.3 企业运营商

生产硬必填：

1. business_license_info.company_address
2. business_license_info.business_time
3. sales_scene_info.store_url
4. contact_info.contact_type=65
5. account_info.bank_account_type=74
6. 外部域名配置可用

## 8. 分账接口矩阵

基于你贴出的分账原文，当前业务范围也已经可以明确判断。

### 8.1 统一必填

1. sub_mchid
2. transaction_id
3. out_order_no
4. receivers
5. finish

### 8.2 PERSONAL_OPENID 关键规则

当 receivers 中出现 PERSONAL_OPENID 时：

1. appid 必填
2. receiver_account 必须是个人 OpenID
3. receiver_name 选传
4. 如果传 receiver_name，则会校验实名匹配，不匹配会拒绝分账

这正好适用于：

1. 个人运营商
2. 骑手

### 8.3 MERCHANT_ID 关键规则

当 receivers.type=MERCHANT_ID 时：

1. receiver_account 必须是商户号
2. receiver_name 必传
3. receiver_name 需要加密

这正好适用于：

1. 已完成微信进件的餐饮商户
2. 已完成微信进件的企业运营商

## 9. 当前最终可执行判断

基于你已经提供的原文，现在可以直接下这些结论：

1. 主体 4 个体工商户的必填、条件必填、二选一必填已经足够清晰，可直接设计后端 readiness 规则。
2. 主体 2 企业的必填、条件必填、二选一必填也已足够清晰，可直接设计后端 readiness 规则。
3. 企业运营商如果要进件，必须准备 sales_scene_info，不存在“因为不开店所以不用传”的空间。
4. 个人运营商和骑手走 PERSONAL_OPENID 分账完全成立，但要记住 receivers 中只要有 PERSONAL_OPENID，appid 就必须传。
5. 商户与企业运营商如果走 MERCHANT_ID 分账，receiver_name 必传且要加密。

## 10. 建议的实现收口

首期生产版建议只支持以下窄路径：

1. 商户主体 4：身份证路径、contact_type=65、店铺二维码路径
2. 商户主体 2：身份证路径、contact_type=65、店铺二维码路径、必要时补 ubo_info_list
3. 企业运营商主体 2：身份证路径、contact_type=65、store_url 路径、对公账户路径
4. 个人运营商：PERSONAL_OPENID 分账
5. 骑手：PERSONAL_OPENID 分账

不建议首期支持：

1. 经办人 contact_type=66
2. 非身份证证件路径
3. 企业运营商二维码路径
4. 金融机构分支
5. 特殊主体分支

## 11. 仍需在实现时写死的规则

为了防止实现跑偏，代码里应直接写死以下规则：

1. 商户 organization_type 只允许 4 和 2
2. 商户主体 2 只能使用 bank_account_type=74
3. 商户主体 4 允许 74 或 75
4. 商户 contact_type 首期只允许 65
5. 企业运营商 organization_type 只允许 2
6. 企业运营商 sales_scene_info 只允许 store_url，不允许 store_qr_code
7. 个人运营商和骑手不允许进入微信进件提交接口
8. PERSONAL_OPENID 分账时 appid 必传
9. MERCHANT_ID 分账时 receiver_name 必传且加密