# 微信支付商户注销组官方契约基线

## 1. 目的

本文是 LocalLife 对“商户注销组”官方接口契约的单一汇总文件，仅覆盖以下 4 个官方接口：

- 注销预校验-商户注销资格校验
- 注销提现-提交注销提现申请
- 注销提现-商户申请单号查询申请单状态
- 注销提现-微信支付申请单号查询申请单状态

本文用途是把官方文档中的请求路径、请求/响应字段、必填要求、条件必填、类型、状态枚举、错误码统一落成一个仓库内可复核的单文件真相源，防止实现层、测试层、审查层和 prompt 层各自发明字段或状态。

本文不是本地业务裁剪文档，也不是实现说明。若官方文档变更，应先更新本文，再改代码。

## 2. 官方来源

- 产品介绍：https://pay.weixin.qq.com/doc/v3/partner/4018153750.md
- 注销预校验-商户注销资格校验：https://pay.weixin.qq.com/doc/v3/partner/4016420099.md
- 注销提现-提交注销提现申请：https://pay.weixin.qq.com/doc/v3/partner/4013892756.md
- 注销提现-商户申请单号查询申请单状态：https://pay.weixin.qq.com/doc/v3/partner/4013892759.md
- 注销提现-微信支付申请单号查询申请单状态：https://pay.weixin.qq.com/doc/v3/partner/4013892765.md

## 3. 统一执行规则

- 不允许只摘状态、不摘字段；也不允许只摘请求体、不摘响应体。
- 不允许只记录必填字段，而忽略条件必填字段。
- 不允许把多个接口的错误码混成一套模糊“通用错误”。必须按接口逐项对齐。
- 不允许把查询接口响应中的可选字段当成稳定必返字段。
- 不允许发明本文未出现的枚举值、状态值、字段名、条件规则。
- 代码、测试、review、prompt 若与本文冲突，以官方文档和本文为准，必须回改。

## 4. 能力组总览

### 4.1 能力链路

1. 先调用资格校验接口，确认是否允许发起注销提现。
2. 再调用提交接口发起申请。
3. 后续仅能通过两种查单接口跟踪状态：按商户申请单号、按微信支付申请单号。
4. 真正的远端状态真相由查询接口返回的 cancel_state 与 withdraw_state 共同表达。

### 4.2 统一枚举池

以下枚举值在商户注销组内跨接口复用，代码中不得偏离。

#### 注销状态 cancel_state

| 值 | 语义 |
| --- | --- |
| ACCEPTED | 已受理 |
| REVIEWING | 审核中 |
| REJECTED | 审批驳回 |
| WAITING_MERCHANT_CONFIRM | 等待商户确认 |
| REVOKED | 已撤销 |
| SYSTEM_PROCESSING | 系统处理中 |
| CANCELED | 已注销 |
| FUND_PROCESSING | 资金处理中 |
| FINISH | 注销完成 |

#### 提现标记 withdraw

| 值 | 语义 |
| --- | --- |
| NOT_APPLY_WITHDRAW | 仅注销，不申请提现 |
| APPLY_WITHDRAW | 申请提现 |

#### 提现状态 withdraw_state

| 值 | 语义 |
| --- | --- |
| WITHDRAW_PROCESSING | 提现处理中 |
| WITHDRAW_EXCEPTION | 提现异常 |
| WITHDRAW_SUCCEED | 提现成功 |

#### 出款子账户类型 out_account_type

| 值 | 语义 |
| --- | --- |
| BASIC_ACCOUNT | 基本户 |
| OPERATE_ACCOUNT | 运营账户 |
| MARGIN_ACCOUNT | 保证金户 |
| TRADE_FEE_ACCOUNT | 手续费账户 |

#### 付款状态 pay_state

| 值 | 语义 |
| --- | --- |
| PAY_PROCESSING | 处理中 |
| PAY_SUCCEED | 付款成功 |
| PAY_FAIL | 付款失败 |
| BANK_REFUNDED | 银行退票 |

## 5. 接口一：注销资格校验

### 5.1 官方基础信息

- 官方名称：注销预校验-商户注销资格校验
- 支持商户：普通服务商
- 请求方式：GET
- 路径：/v3/ecommerce/account/apply-cancel-withdraw/validate-cancel/{sub_mchid}

### 5.2 请求参数

#### Header

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| Authorization | string | 是 | 按 APIv3 签名规则生成 |
| Accept | string | 是 | 固定 application/json |

#### Path

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | 是 | 待检查的二级商户号 |

### 5.3 响应体

| 字段 | 类型 | 必返 | 说明 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | 是 | 被检查的二级商户号 |
| merchant_state | string | 是 | 商户号状态 |
| validate_result | string | 是 | 是否允许发起注销提现 |
| account_info | array[object] | 否 | 已开通资金账户的实时余额；发起清退前应再次调用校验接口确认 |
| block_reasons | array[object] | 否 | 不可发起注销的原因列表；同一商户可能返回多个原因 |

#### merchant_state 枚举

| 值 | 语义 |
| --- | --- |
| NORMAL | 正常 |
| HAS_BEEN_CANCELLED | 已注销 |

#### validate_result 枚举

| 值 | 语义 |
| --- | --- |
| ALLOW_CANCEL_WITHDRAW | 可发起注销提现申请 |
| NOT_ALLOW_CANCEL_WITHDRAW | 不可发起注销提现申请 |

#### account_info 字段

| 字段 | 类型 | 必返 | 说明 |
| --- | --- | --- | --- |
| out_account_type | string | 是 | 已开通的出款子账户类型 |
| amount | integer | 是 | 账户金额，单位分 |

#### block_reasons 字段

| 字段 | 类型 | 必返 | 说明 |
| --- | --- | --- | --- |
| type | string | 否 | 不可注销原因类型 |
| description | string(512) | 否 | 不可注销原因描述 |

#### block_reasons.type 枚举

| 值 | 语义 |
| --- | --- |
| CONSUMER_COMPLAINT_UNPROCESSED | 消费者投诉未处理 |
| HAS_BLOCKING_CONTROL | 存在不可注销管控 |
| FUNDS_PENDING_PROCESSING | 商户资金待处理 |
| OTHER_REASON | 其他原因 |

### 5.4 错误码

本接口官方文档未单列业务错误码，只有错误码总表。

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 |
| 400 | INVALID_REQUEST | HTTP 请求不符合 APIv3 规则 |
| 401 | SIGN_ERROR | 签名验证不通过 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 |

## 6. 接口二：提交注销提现申请

### 6.1 官方基础信息

- 官方名称：注销提现-提交注销提现申请
- 支持商户：平台商户
- 请求方式：POST
- 路径：/v3/ecommerce/account/apply-cancel-withdraw

### 6.2 请求参数

#### Header

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| Authorization | string | 是 | 按 APIv3 签名规则生成 |
| Accept | string | 是 | 固定 application/json |
| Content-Type | string | 是 | 固定 application/json |
| Wechatpay-Serial | string | 是 | 当请求体含敏感字段时必须提供；可用微信支付公钥 ID 或平台证书序列号 |

#### Body 顶层字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | 是 | 待注销的二级商户号 |
| out_request_no | string(32) | 是 | 商户自定义注销申请单号；要求在服务商维度唯一，且只允许字母数字 |
| withdraw | string | 否 | 是否同时申请提现 |
| payee_info | object | 否 | 收款账号信息；填写 withdraw 后进入提现路径时使用 |
| proof_medias | array[ProofMedia] | 否 | 付款申请材料 |
| additional_materials | array[string] | 否 | 其他补充材料，最多 10 张图片 media_id |
| remark | string(32) | 否 | 备注，展示在银行系统，用于平台与二级商户对账 |

#### withdraw 枚举

| 值 | 语义 |
| --- | --- |
| NOT_APPLY_WITHDRAW | 仅注销，不申请提现 |
| APPLY_WITHDRAW | 申请提现 |

#### payee_info 字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| account_type | string | 是 | 收款账户类型 |
| bank_account_info | object | 是 | 银行账户信息 |
| identity_info | object | 否 | 对私银行卡时必填的证件信息 |

#### payee_info.account_type 枚举

| 值 | 语义 |
| --- | --- |
| ACCOUNT_TYPE_CORPORATE | 对公银行账户 |
| ACCOUNT_TYPE_PERSONAL | 对私银行卡账户 |

#### payee_info.bank_account_info 字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| account_name | string(2048) | 是 | 开户名称；敏感字段，必须加密 |
| account_bank | string(128) | 是 | 开户银行名称 |
| bank_branch_id | string(128) | 否 | 开户银行联行号；是否需要填写取决于开户银行规则 |
| bank_branch_name | string(128) | 否 | 开户银行全称含支行；与联行号按官方规则二选一或按需填写 |
| account_number | string(2048) | 是 | 银行账号；敏感字段，必须加密 |

#### payee_info.identity_info 字段

官方规则：当收款账户为对私银行卡时，此对象必填，且以下三个字段都必须填写。

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| id_doc_type | string | 条件必填 | 对私银行卡开户人证件类型 |
| identification_name | string(2048) | 条件必填 | 证件姓名；敏感字段，必须加密 |
| identification_no | string(2048) | 条件必填 | 证件号码；敏感字段，必须加密 |

#### id_doc_type 枚举

| 值 | 语义 |
| --- | --- |
| IDENTIFICATION_TYPE_ID_CARD | 中国大陆居民身份证 |
| IDENTIFICATION_TYPE_OVERSEA_PASSPORT | 其他国家或地区居民护照 |
| IDENTIFICATION_TYPE_HONGKONG_PASSPORT | 中国香港居民来往内地通行证 |
| IDENTIFICATION_TYPE_MACAO_PASSPORT | 中国澳门居民来往内地通行证 |
| IDENTIFICATION_TYPE_TAIWAN_PASSPORT | 中国台湾居民来往大陆通行证 |
| IDENTIFICATION_TYPE_FOREIGN_RESIDENT | 外国人居留证 |
| IDENTIFICATION_TYPE_HONGKONG_MACAO_RESIDENT | 港澳居民证 |
| IDENTIFICATION_TYPE_TAIWAN_RESIDENT | 台湾居民证 |

#### proof_medias 字段

官方规则：以下主体场景要求上传付款申请材料，否则可不传。

- 企业主体且经营证照已注吊撤：必填
- 事业单位 / 政府机关 / 社会组织：必填
- 小微、个体工商户、经营证照存续企业：无需填写

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| proof_media_type | string | 是 | 材料类型 |
| proof_media | string(1024) | 是 | 图片上传接口预先生成的 media_id |

#### proof_media_type 枚举

| 值 | 语义 |
| --- | --- |
| WITHDRAWAL_APPLICATION | 付款申请书材料 |

### 6.3 响应体

| 字段 | 类型 | 必返 | 说明 |
| --- | --- | --- | --- |
| applyment_id | string(32) | 否 | 微信支付返回的注销提现申请单号，后续查单唯一标识 |
| out_request_no | string(32) | 否 | 商户注销申请单号 |

### 6.4 错误码

#### 公共错误码

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 |
| 400 | INVALID_REQUEST | HTTP 请求不符合 APIv3 规则 |
| 401 | SIGN_ERROR | 签名验证不通过 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 |

#### 业务错误码

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| 400 | BIZ_ERR_NEED_RETRY | 系统异常，请稍后重试 |
| 400 | ALREADY_EXISTS | out_request_no 已被占用 |

### 6.5 官方注意事项

- 仅适用于被微信支付管控且无法解除该管控的商户，例如营业执照已注销或吊销。
- 审核通过并经商户确认后，账户内所有余额会打款到指定银行卡；若存在多个账户，可能产生多笔入账。
- 审核通过后超过 30 天未由商户确认，单据会撤销。

## 7. 接口三：按商户申请单号查询申请单状态

### 7.1 官方基础信息

- 官方名称：注销提现-商户申请单号查询申请单状态
- 支持商户：平台商户
- 请求方式：GET
- 路径：/v3/ecommerce/account/apply-cancel-withdraw/out-request-no/{out_request_no}

### 7.2 请求参数

#### Header

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| Authorization | string | 是 | 按 APIv3 签名规则生成 |
| Accept | string | 是 | 固定 application/json |

#### Path

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| out_request_no | string(32) | 是 | 提交接口中的商户注销申请单号 |

### 7.3 响应体

| 字段 | 类型 | 必返 | 说明 |
| --- | --- | --- | --- |
| applyment_id | string(32) | 是 | 微信支付注销提现申请单号 |
| out_request_no | string(32) | 是 | 商户注销申请单号 |
| cancel_state | string | 是 | 注销提现申请单状态 |
| cancel_state_description | string(256) | 是 | 注销提现申请单状态文字描述 |
| withdraw | string | 否 | 提交接口中的提现标记 |
| withdraw_state | string | 否 | 提现状态；仅在进入资金处理中后返回 |
| withdraw_state_description | string(256) | 否 | 提现状态文字描述 |
| account_withdraw_result | array[object] | 否 | 针对具体账户的提现结果 |
| modify_time | string(32) | 否 | 最后更新时间，RFC3339 |
| sub_mchid | string(32) | 是 | 申请注销的二级商户号 |
| account_info | array[object] | 否 | 涉及提取资金的申请单才会返回；展示已开通账户信息 |
| confirm_cancel | object | 否 | 等待商户确认时返回，用于引导商户扫码确认 |

#### account_withdraw_result 字段

| 字段 | 类型 | 必返 | 说明 |
| --- | --- | --- | --- |
| out_account_type | string | 是 | 出款子账户类型 |
| pay_state | string | 是 | 付款状态 |
| state_description | string(256) | 是 | 付款状态描述 |

#### account_info 字段

| 字段 | 类型 | 必返 | 说明 |
| --- | --- | --- | --- |
| out_account_type | string | 是 | 出款子账户类型 |
| amount | integer | 是 | 账户金额，单位分 |

#### confirm_cancel 字段

| 字段 | 类型 | 必返 | 说明 |
| --- | --- | --- | --- |
| confirm_cancel_url | string(1024) | 否 | 当 cancel_state=WAITING_MERCHANT_CONFIRM 时返回；建议转成二维码引导商户扫码确认 |

### 7.4 错误码

官方文档仅给出公共错误码，未单列业务错误码。

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 |
| 400 | INVALID_REQUEST | HTTP 请求不符合 APIv3 规则 |
| 401 | SIGN_ERROR | 签名验证不通过 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 |

## 8. 接口四：按微信支付申请单号查询申请单状态

### 8.1 官方基础信息

- 官方名称：注销提现-微信支付申请单号查询申请单状态
- 支持商户：平台商户
- 请求方式：GET
- 路径：/v3/ecommerce/account/apply-cancel-withdraw/applyment-id/{applyment_id}

### 8.2 请求参数

#### Header

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| Authorization | string | 是 | 按 APIv3 签名规则生成 |
| Accept | string | 是 | 固定 application/json |

#### Path

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| applyment_id | string(32) | 是 | 微信支付注销提现申请单号 |

### 8.3 响应体

该接口响应体、字段可选性、枚举和值域与“按商户申请单号查询申请单状态”一致，唯一路径参数不同。

| 字段 | 类型 | 必返 | 说明 |
| --- | --- | --- | --- |
| applyment_id | string(32) | 是 | 微信支付注销提现申请单号 |
| out_request_no | string(32) | 是 | 商户注销申请单号 |
| cancel_state | string | 是 | 注销提现申请单状态 |
| cancel_state_description | string(256) | 是 | 注销提现申请单状态文字描述 |
| withdraw | string | 否 | 提交接口中的提现标记 |
| withdraw_state | string | 否 | 提现状态；仅在进入资金处理中后返回 |
| withdraw_state_description | string(256) | 否 | 提现状态文字描述 |
| account_withdraw_result | array[object] | 否 | 针对具体账户的提现结果 |
| modify_time | string(32) | 否 | 最后更新时间，RFC3339 |
| sub_mchid | string(32) | 是 | 申请注销的二级商户号 |
| account_info | array[object] | 否 | 涉及提取资金的申请单才会返回；展示已开通账户信息 |
| confirm_cancel | object | 否 | 等待商户确认时返回，用于引导商户扫码确认 |

### 8.4 错误码

官方文档仅给出公共错误码，未单列业务错误码。

| HTTP 状态码 | 错误码 | 说明 |
| --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 |
| 400 | INVALID_REQUEST | HTTP 请求不符合 APIv3 规则 |
| 401 | SIGN_ERROR | 签名验证不通过 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 |

## 9. 对实现层的强约束

- 提交接口里，只有 out_request_no 有业务唯一性错误码 ALREADY_EXISTS；查询接口没有该业务错误码。
- WAITING_MERCHANT_CONFIRM 不是终态；查单接口此时可能返回 confirm_cancel.confirm_cancel_url。
- 提现链路不是单一状态；必须同时保留 cancel_state、withdraw、withdraw_state、account_withdraw_result。
- account_info 与 account_withdraw_result 不是同一语义：前者是账户余额/账户信息，后者是实际提现结果。
- 预校验返回 NOT_ALLOW_CANCEL_WITHDRAW 时，block_reasons 才是解释性真相源，不能自行发明本地拒绝原因码体系替代。
- 条件必填必须按官方规则执行，尤其是对私银行卡下的 identity_info 三字段。
- 提交接口中的敏感字段必须按官方要求加密，且请求头必须带 Wechatpay-Serial。

## 10. 当前核对结论

- 这 4 个接口必须作为一个能力组统一处理，不能拆成“提交一套、查询一套、校验一套”分别随意实现。
- 任何声称“已对齐官方文档”的代码改动，至少要同时对齐本文中的请求参数、响应参数、状态枚举和错误码。
- 若后续还要补传播矩阵、review checklist 或实现回查，本文应作为第一真相源，而不是直接从已有 Go 结构体反推。