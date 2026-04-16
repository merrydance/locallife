# 交易退款组文档样例

本文件基于微信支付服务商交易退款组官方文档摘录，保留 contracts、errorcodes 与 alignment audit 所需的接口、字段、枚举和语义约束。

来源页面：

- 申请退款：https://pay.weixin.qq.com/doc/v3/partner/4012476892.md
- 查询单笔退款（按微信支付退款单号）：https://pay.weixin.qq.com/doc/v3/partner/4012476908.md
- 查询单笔退款（按商户退款单号）：https://pay.weixin.qq.com/doc/v3/partner/4012476911.md
- 退款结果通知：https://pay.weixin.qq.com/doc/v3/partner/4012124635.md
- 查询垫付回补通知：https://pay.weixin.qq.com/doc/v3/partner/4012476916.md
- 垫付退款回补：https://pay.weixin.qq.com/doc/v3/partner/4012476927.md
- 发起异常退款：https://pay.weixin.qq.com/doc/v3/partner/4015181616.md

## 交易退款-申请退款

### 接口说明

请求方式：【POST】
`/v3/ecommerce/refunds/apply`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |
| sp_appid | string(32) | true | 电商平台 APPID |
| sub_appid | string(32) | false | 二级商户 APPID |
| transaction_id | string(32) | false | 微信订单号 |
| out_trade_no | string(32) | false | 商户订单号 |
| out_refund_no | string(64) | true | 商户退款单号 |
| reason | string(80) | false | 退款原因 |
| amount | object | true | 订单金额信息 |
| amount.refund | integer | true | 退款金额，单位为分 |
| amount.from | array[object] | false | 退款出资账户及金额 |
| amount.from.account | string(32) | true | 出资账户类型，枚举值：AVAILABLE UNAVAILABLE |
| amount.from.amount | integer | true | 出资金额，单位为分 |
| amount.total | integer | true | 原订单金额，单位为分 |
| amount.currency | string(16) | false | 退款币种，目前只支持：CNY |
| notify_url | string(256) | false | 外网可访问的退款结果回调地址，不能携带参数 |
| refund_account | string(32) | false | 退款出资商户，枚举值：REFUND_SOURCE_PARTNER_ADVANCE REFUND_SOURCE_SUB_MERCHANT |
| funds_account | string(32) | false | 资金账户，枚举值：AVAILABLE |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信支付退款订单号 |
| out_refund_no | string(64) | true | 商户退款单号 |
| create_time | string(64) | true | 退款创建时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| amount | object | true | 订单退款金额信息 |
| amount.refund | integer | true | 退款金额，单位为分 |
| amount.from | array[object] | false | 退款出资账户及金额 |
| amount.from.account | string(32) | true | 出资账户类型，枚举值：AVAILABLE UNAVAILABLE |
| amount.from.amount | integer | true | 出资金额，单位为分 |
| amount.payer_refund | integer | true | 用户退款金额，单位为分 |
| amount.discount_refund | integer | false | 优惠退款金额，单位为分 |
| amount.currency | string(16) | false | 货币类型，目前只支持：CNY |
| amount.advance | integer | false | 垫付金额，单位为分 |
| promotion_detail | array[object] | false | 优惠退款详情 |
| promotion_detail.promotion_id | string(32) | true | 券 ID |
| promotion_detail.scope | string(32) | true | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string(32) | true | 优惠类型，枚举值：COUPON DISCOUNT |
| promotion_detail.amount | integer | true | 优惠券面额，单位为分 |
| promotion_detail.refund_amount | integer | true | 优惠退款金额，单位为分 |
| refund_account | string(32) | false | 退款出资商户，枚举值：REFUND_SOURCE_PARTNER_ADVANCE REFUND_SOURCE_SUB_MERCHANT |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求参数符合参数格式，但不符合业务规则 | 请根据具体错误提示调整请求 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 403 | NO_AUTH | 没有退款权限 | 请检查是否有退该订单的权限 |
| 403 | NOT_ENOUGH | 余额不足 | 根据提示进行充值后重试 |
| 403 | USER_ACCOUNT_ABNORMAL | 用户账户异常或已注销 | 请改用其他方式处理退款 |
| 404 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 404 | RESOURCE_NOT_EXISTS | 订单不存在 | 请检查订单号是否正确且订单已支付 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请查单确认或降低频率后重试 |
| 500 | SYSTEM_ERROR | 接口返回错误 | 请不要更换退款单号，使用相同参数重试 |

## 交易退款-查询单笔退款（按微信支付退款单号）

### 接口说明

请求方式：【GET】
`/v3/ecommerce/refunds/id/{refund_id}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信退款单号 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信支付退款订单号 |
| out_refund_no | string(64) | true | 商户退款单号 |
| transaction_id | string(32) | true | 微信支付交易订单号 |
| out_trade_no | string(32) | true | 商户原交易订单号 |
| channel | string(16) | false | 退款渠道，枚举值：ORIGINAL BALANCE OTHER_BALANCE OTHER_BANKCARD |
| user_received_account | string(64) | false | 退款入账账户 |
| success_time | string(64) | false | 退款成功时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| create_time | string(64) | true | 退款创建时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| status | string(16) | true | 退款状态，枚举值：SUCCESS CLOSED PROCESSING ABNORMAL |
| amount | object | true | 退款金额信息 |
| amount.refund | integer | true | 退款金额，单位为分 |
| amount.from | array[object] | false | 退款出资账户及金额 |
| amount.from.account | string(32) | true | 出资账户类型，枚举值：AVAILABLE UNAVAILABLE |
| amount.from.amount | integer | true | 出资金额，单位为分 |
| amount.payer_refund | integer | true | 用户退款金额，单位为分 |
| amount.discount_refund | integer | false | 优惠退款金额，单位为分 |
| amount.currency | string(16) | false | 货币类型，目前只支持：CNY |
| amount.advance | integer | false | 垫付金额，单位为分 |
| promotion_detail | array[object] | false | 优惠退款信息 |
| promotion_detail.promotion_id | string(32) | true | 券 ID |
| promotion_detail.scope | string(32) | true | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string(32) | true | 优惠类型，枚举值：COUPON DISCOUNT |
| promotion_detail.amount | integer | true | 优惠券面额，单位为分 |
| promotion_detail.refund_amount | integer | true | 优惠退款金额，单位为分 |
| refund_account | string(32) | false | 退款出资商户，枚举值：REFUND_SOURCE_PARTNER_ADVANCE REFUND_SOURCE_SUB_MERCHANT |
| funds_account | string(32) | false | 资金账户，枚举值：UNSETTLED AVAILABLE UNAVAILABLE OPERATION BASIC ECNY_BASIC |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求参数符合参数格式，但不符合业务规则 | 请根据具体错误提示调整请求 |
| 401 | SIGN_ERROR | 参数签名结果不正确 | 请检查签名参数和签名方法 |
| 403 | NO_AUTH | 没有退款权限 | 请检查是否有退该订单的权限 |
| 403 | REQUEST_BLOCKED | 请求受阻 | 请根据错误提示调整请求后重试 |
| 404 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 404 | RESOURCE_NOT_EXISTS | 订单不存在 | 请检查订单号是否正确且订单已支付 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 接口返回错误 | 请不要更换退款单号，使用相同参数重试 |

## 交易退款-查询单笔退款（按商户退款单号）

### 接口说明

请求方式：【GET】
`/v3/ecommerce/refunds/out-refund-no/{out_refund_no}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| out_refund_no | string(64) | true | 商户退款单号 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信支付退款订单号 |
| out_refund_no | string(64) | true | 商户退款单号 |
| transaction_id | string(32) | true | 微信支付交易订单号 |
| out_trade_no | string(32) | true | 商户原交易订单号 |
| channel | string(16) | false | 退款渠道，枚举值：ORIGINAL BALANCE OTHER_BALANCE OTHER_BANKCARD |
| user_received_account | string(64) | false | 退款入账账户 |
| success_time | string(64) | false | 退款成功时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| create_time | string(64) | true | 退款创建时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| status | string(16) | true | 退款状态，枚举值：SUCCESS CLOSED PROCESSING ABNORMAL |
| amount | object | true | 退款金额信息 |
| amount.refund | integer | true | 退款金额，单位为分 |
| amount.from | array[object] | false | 退款出资账户及金额 |
| amount.from.account | string(32) | true | 出资账户类型，枚举值：AVAILABLE UNAVAILABLE |
| amount.from.amount | integer | true | 出资金额，单位为分 |
| amount.payer_refund | integer | true | 用户退款金额，单位为分 |
| amount.discount_refund | integer | false | 优惠退款金额，单位为分 |
| amount.currency | string(16) | false | 货币类型，目前只支持：CNY |
| amount.advance | integer | false | 垫付金额，单位为分 |
| promotion_detail | array[object] | false | 优惠退款信息 |
| promotion_detail.promotion_id | string(32) | true | 券 ID |
| promotion_detail.scope | string(32) | true | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string(32) | true | 优惠类型，枚举值：COUPON DISCOUNT |
| promotion_detail.amount | integer | true | 优惠券面额，单位为分 |
| promotion_detail.refund_amount | integer | true | 优惠退款金额，单位为分 |
| refund_account | string(32) | false | 退款出资商户，枚举值：REFUND_SOURCE_PARTNER_ADVANCE REFUND_SOURCE_SUB_MERCHANT |
| funds_account | string(32) | false | 资金账户，枚举值：UNSETTLED AVAILABLE UNAVAILABLE OPERATION BASIC ECNY_BASIC |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求参数符合参数格式，但不符合业务规则 | 请根据具体错误提示调整请求 |
| 401 | SIGN_ERROR | 参数签名结果不正确 | 请检查签名参数和签名方法 |
| 403 | NO_AUTH | 没有退款权限 | 请检查是否有退该订单的权限 |
| 403 | REQUEST_BLOCKED | 请求受阻 | 请根据错误提示调整请求后重试 |
| 404 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 404 | RESOURCE_NOT_EXISTS | 订单不存在 | 请检查订单号是否正确且订单已支付 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 接口返回错误 | 请不要更换退款单号，使用相同参数重试 |

## 交易退款-退款结果通知

### 接口说明

请求方式：【POST】
`/v1/webhooks/wechat-ecommerce/refund-notify`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_mchid | string(32) | true | 电商平台商户号 |
| sub_mchid | string(32) | true | 二级商户号 |
| out_trade_no | string(32) | true | 商户订单号 |
| transaction_id | string(32) | true | 微信订单号 |
| out_refund_no | string(64) | true | 商户退款单号 |
| refund_id | string(32) | true | 微信退款单号 |
| refund_status | string(32) | true | 退款状态，枚举值：SUCCESS CLOSED ABNORMAL |
| success_time | string(64) | false | 退款成功时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| user_received_account | string(64) | true | 退款入账账户 |
| amount | object | true | 金额信息 |
| amount.total | integer | true | 订单总金额，单位为分 |
| amount.refund | integer | true | 退款金额，单位为分 |
| amount.payer_total | integer | true | 用户实际支付金额，单位为分 |
| amount.payer_refund | integer | true | 用户退款金额，单位为分 |
| refund_account | string(32) | false | 退款出资商户，枚举值：REFUND_SOURCE_PARTNER_ADVANCE REFUND_SOURCE_SUB_MERCHANT |

## 交易退款-查询垫付回补通知

### 接口说明

请求方式：【GET】
`/v3/ecommerce/refunds/{refund_id}/return-advance`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信退款单号，必须是垫付退款的微信退款单 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信退款单号 |
| advance_return_id | string(32) | true | 微信回补单号 |
| return_amount | integer | true | 垫付回补金额，单位为分 |
| payer_mchid | string(32) | true | 出款方商户号 |
| payer_account | string(32) | true | 出款方账户，枚举值：BASIC OPERATION |
| payee_mchid | string(32) | true | 入账方商户号 |
| payee_account | string(32) | true | 入账方账户，枚举值：BASIC OPERATION |
| result | string(32) | true | 垫付回补结果，枚举值：SUCCESS FAILED PROCESSING |
| success_time | string(64) | false | 垫付回补完成时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 参数签名结果不正确 | 请检查签名参数和签名方法 |
| 404 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 404 | RESOURCE_NOT_EXISTS | 垫付回补结果查询失败 | 请检查退款单号是否有误 |
| 500 | SYSTEM_ERROR | 接口返回错误 | 请不要更换退款单号，使用相同参数再次调用 API |

## 交易退款-垫付退款回补

### 接口说明

请求方式：【POST】
`/v3/ecommerce/refunds/{refund_id}/return-advance`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信退款单号，必须是垫付退款的微信退款单 |

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信退款单号 |
| advance_return_id | string(32) | true | 微信回补单号 |
| return_amount | integer | true | 垫付回补金额，单位为分 |
| payer_mchid | string(32) | true | 出款方商户号 |
| payer_account | string(32) | true | 出款方账户，枚举值：BASIC OPERATION |
| payee_mchid | string(32) | true | 入账方商户号 |
| payee_account | string(32) | true | 入账方账户，枚举值：BASIC OPERATION |
| result | string(32) | true | 垫付回补结果，枚举值：SUCCESS FAILED PROCESSING |
| success_time | string(64) | false | 垫付回补完成时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求参数符合参数格式，但不符合业务规则 | 请根据具体错误提示调整请求 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 403 | NO_AUTH | 没有垫付回补权限 | 请检查是否有垫付回补这笔退款单的权限 |
| 403 | NOT_ENOUGH | 余额不足 | 请确保出款方账户余额充足后重试 |
| 403 | REQUEST_BLOCKED | 请求受阻 | 请根据错误提示调整请求后重试 |
| 404 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 404 | RESOURCE_NOT_EXISTS | 退款单不存在 | 请检查退款单号是否有误以及订单状态是否正确 |
| 500 | SYSTEM_ERROR | 接口返回错误 | 请不要更换退款单号，使用相同参数再次调用 API |

## 交易退款-发起异常退款

### 接口说明

请求方式：【POST】
`/v3/ecommerce/refunds/{refund_id}/apply-abnormal-refund`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信退款单号 |

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |
| out_refund_no | string(64) | true | 商户退款单号 |
| type | string | true | 异常退款处理方式，枚举值：USER_BANK_CARD MERCHANT_BANK_CARD |
| bank_type | string(16) | false | 开户银行 |
| bank_account | string(1024) | false | 收款银行卡号 |
| real_name | string(1024) | false | 收款用户姓名 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信支付退款订单号 |
| out_refund_no | string(64) | true | 商户退款单号 |
| transaction_id | string(32) | true | 微信支付交易订单号 |
| out_trade_no | string(32) | true | 商户原交易订单号 |
| channel | string(16) | false | 退款渠道，枚举值：ORIGINAL BALANCE OTHER_BALANCE OTHER_BANKCARD |
| user_received_account | string(64) | false | 退款入账账户 |
| success_time | string(64) | false | 退款成功时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| create_time | string(64) | true | 退款创建时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| status | string(16) | true | 退款状态，枚举值：SUCCESS CLOSED PROCESSING ABNORMAL |
| amount | object | true | 退款金额信息 |
| amount.refund | integer | true | 退款金额，单位为分 |
| amount.from | array[object] | false | 退款出资账户及金额 |
| amount.from.account | string(32) | true | 出资账户类型，枚举值：AVAILABLE UNAVAILABLE |
| amount.from.amount | integer | true | 出资金额，单位为分 |
| amount.payer_refund | integer | true | 用户退款金额，单位为分 |
| amount.discount_refund | integer | false | 优惠退款金额，单位为分 |
| amount.currency | string(16) | false | 货币类型，目前只支持：CNY |
| amount.advance | integer | false | 垫付金额，单位为分 |
| promotion_detail | array[object] | false | 优惠退款信息 |
| promotion_detail.promotion_id | string(32) | true | 券 ID |
| promotion_detail.scope | string(32) | true | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string(32) | true | 优惠类型，枚举值：COUPON DISCOUNT |
| promotion_detail.amount | integer | true | 优惠券面额，单位为分 |
| promotion_detail.refund_amount | integer | true | 优惠退款金额，单位为分 |
| refund_account | string(32) | false | 退款出资商户，枚举值：REFUND_SOURCE_PARTNER_ADVANCE REFUND_SOURCE_SUB_MERCHANT |
| funds_account | string(32) | false | 资金账户，枚举值：UNSETTLED AVAILABLE UNAVAILABLE OPERATION BASIC ECNY_BASIC |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |