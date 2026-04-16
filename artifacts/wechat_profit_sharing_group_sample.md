# 补差与分账组-分账文档样例

本文件基于微信支付服务商分账组官方文档摘录，保留 contracts、errorcodes 与 alignment audit 所需的接口、字段、枚举和语义约束。

来源页面：

- 请求分账：https://pay.weixin.qq.com/doc/v3/partner/4012691594.md
- 查询分账结果：https://pay.weixin.qq.com/doc/v3/partner/4012477734.md
- 请求分账回退：https://pay.weixin.qq.com/doc/v3/partner/4012477737.md
- 查询分账回退结果：https://pay.weixin.qq.com/doc/v3/partner/4012477740.md
- 解冻剩余资金：https://pay.weixin.qq.com/doc/v3/partner/4012477745.md
- 查询订单剩余分账金额：https://pay.weixin.qq.com/doc/v3/partner/4012477751.md
- 添加分账接收方：https://pay.weixin.qq.com/doc/v3/partner/4012477758.md
- 删除分账接收方：https://pay.weixin.qq.com/doc/v3/partner/4012477759.md
- 分账动账通知：https://pay.weixin.qq.com/doc/v3/partner/4012116672.md

## 分账-请求分账

### 接口说明

请求方式：【POST】
`/v3/ecommerce/profitsharing/orders`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| appid | string(32) | false | 公众账号 ID，分账接收方类型包含 PERSONAL_OPENID 时必填 |
| sub_mchid | string(32) | true | 电商平台二级商户号 |
| transaction_id | string(32) | true | 微信订单号 |
| out_order_no | string(64) | true | 商户分账单号 |
| receivers | array[object] | true | 分账接收方列表 |
| receivers.type | string(32) | true | 分账接收方类型，枚举值：MERCHANT_ID PERSONAL_OPENID |
| receivers.receiver_account | string(64) | true | 分账接收方账号 |
| receivers.amount | integer | true | 分账金额，单位为分 |
| receivers.description | string(80) | true | 分账描述 |
| receivers.receiver_name | string(10240) | false | 分账个人接收方姓名或商户全称 |
| finish | boolean | true | 是否分账完成，枚举值：true false |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 电商平台二级商户号 |
| transaction_id | string(32) | true | 微信订单号 |
| out_order_no | string(64) | true | 商户分账单号 |
| order_id | string(64) | true | 微信分账单号 |
| receivers | array[object] | true | 分账接收方列表 |
| receivers.amount | integer | true | 分账金额，单位为分 |
| receivers.description | string(80) | true | 分账描述 |
| receivers.result | string(32) | true | 分账结果，枚举值：PENDING SUCCESS CLOSED |
| receivers.finish_time | string | true | 分账完成时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| receivers.fail_reason | string(64) | false | 分账失败原因，枚举值：ACCOUNT_ABNORMAL NO_RELATION RECEIVER_HIGH_RISK RECEIVER_REAL_NAME_NOT_VERIFIED NO_AUTH RECEIVER_RECEIPT_LIMIT PAYER_ACCOUNT_ABNORMAL INVALID_REQUEST |
| receivers.type | string(32) | true | 接收方类型，枚举值：MERCHANT_ID PERSONAL_OPENID |
| receivers.receiver_account | string(64) | true | 接收方账号 |
| receivers.detail_id | string(64) | true | 分账明细单号 |
| receivers.abnormal_status | string | false | 异常处理状态，枚举值：ABNORMAL_PENDING ABNORMAL_FINISHED ABNORMAL_CLOSED |
| receivers.funds_abnormal_closed_reason | string | false | 异常处理关单原因，枚举值：CLOSED_REASON_TIMEOUT CLOSED_REASON_RESTRICT_TRANSFER |
| receivers.funds_abnormal_redirect_id | string | false | 微信支付在途异常资金付款单号 |
| receivers.funds_abnormal_receivers | array[object] | false | 平台接收异常资金商户号或运营主体列表 |
| receivers.funds_abnormal_receivers.mchid | string | false | 平台接收异常资金商户号 |
| status | string | false | 分账单状态，枚举值：PROCESSING FINISHED |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 403 | NO_AUTH | 商户无权限 | 请开通商户号分账权限 |
| 403 | RULE_LIMIT | 分账金额超出最大分账比例 | 确认分账金额 |
| 403 | NOT_ENOUGH | 分账金额不足 | 调整分账金额 |
| 429 | FREQUENCY_LIMITED | 对同笔订单分账频率过高 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 分账-查询分账结果

### 接口说明

请求方式：【GET】
`/v3/ecommerce/profitsharing/orders`

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 电商平台二级商户号 |
| transaction_id | string(32) | true | 微信订单号 |
| out_order_no | string(64) | true | 商户分账单号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 电商平台二级商户号 |
| transaction_id | string(32) | true | 微信订单号 |
| out_order_no | string(64) | true | 商户分账单号 |
| order_id | string(64) | true | 微信分账单号 |
| status | string(32) | true | 分账单状态，枚举值：PROCESSING FINISHED |
| receivers | array[object] | true | 分账接收方列表 |
| receivers.amount | integer | true | 分账金额，单位为分 |
| receivers.description | string(80) | true | 分账描述 |
| receivers.result | string(32) | true | 分账结果，枚举值：PENDING SUCCESS CLOSED |
| receivers.finish_time | string | true | 分账完成时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| receivers.fail_reason | string(64) | false | 分账失败原因，枚举值：ACCOUNT_ABNORMAL NO_RELATION RECEIVER_HIGH_RISK RECEIVER_REAL_NAME_NOT_VERIFIED NO_AUTH RECEIVER_RECEIPT_LIMIT PAYER_ACCOUNT_ABNORMAL INVALID_REQUEST |
| receivers.type | string(32) | true | 接收方类型，枚举值：MERCHANT_ID PERSONAL_OPENID |
| receivers.receiver_account | string(64) | true | 接收方账号 |
| receivers.detail_id | string(64) | true | 分账明细单号 |
| receivers.abnormal_status | string | false | 异常处理状态，枚举值：ABNORMAL_PENDING ABNORMAL_FINISHED ABNORMAL_CLOSED |
| receivers.funds_abnormal_closed_reason | string | false | 异常处理关单原因，枚举值：CLOSED_REASON_TIMEOUT CLOSED_REASON_RESTRICT_TRANSFER |
| receivers.funds_abnormal_redirect_id | string | false | 微信支付在途异常资金付款单号 |
| receivers.funds_abnormal_receivers | array[object] | false | 平台接收异常资金商户号或运营主体列表 |
| receivers.funds_abnormal_receivers.mchid | string | false | 平台接收异常资金商户号 |
| finish_amount | integer | false | 分账完结金额，单位为分 |
| finish_description | string(80) | false | 分账完结描述 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 404 | RESOURCE_NOT_EXISTS | 记录不存在 | 请检查请求的单号是否正确 |
| 429 | FREQUENCY_LIMITED | 商户发起分账查询的频率过高 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 分账-请求分账回退

### 接口说明

请求方式：【POST】
`/v3/ecommerce/profitsharing/returnorders`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 电商平台二级商户号 |
| order_id | string(64) | false | 微信分账单号，和商户分账单号二选一填写 |
| out_order_no | string(64) | false | 商户分账单号，和微信分账单号二选一填写 |
| out_return_no | string(64) | true | 商户回退单号 |
| return_mchid | string(32) | true | 回退商户号 |
| amount | integer | true | 回退金额，单位为分 |
| description | string(80) | true | 回退描述 |
| transaction_id | string(32) | false | 微信订单号，大于 6 个月的订单必填 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 电商平台二级商户号 |
| order_id | string(64) | true | 微信分账单号 |
| out_order_no | string(64) | true | 商户分账单号 |
| out_return_no | string(64) | true | 商户回退单号 |
| return_mchid | string(32) | true | 回退商户号 |
| amount | integer | true | 回退金额，单位为分 |
| return_no | string(64) | true | 微信回退单号 |
| result | string(32) | true | 回退结果，枚举值：PROCESSING SUCCESS FAILED |
| fail_reason | string(64) | false | 失败原因，枚举值：ACCOUNT_ABNORMAL TIME_OUT_CLOSED PAYER_ACCOUNT_ABNORMAL |
| finish_time | string | true | 完成时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| transaction_id | string(32) | true | 微信订单号 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 403 | NO_AUTH | 回退方未开通分账回退功能 | 请先让回退方开通分账回退功能 |
| 403 | NOT_ENOUGH | 剩余可回退的金额不足 | 调整回退金额 |
| 429 | FREQUENCY_LIMITED | 商户发起分账回退的频率过高 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 分账-查询分账回退结果

### 接口说明

请求方式：【GET】
`/v3/ecommerce/profitsharing/returnorders`

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 电商平台二级商户号 |
| out_return_no | string(64) | true | 商户回退单号 |
| order_id | string(64) | false | 微信分账单号，和商户分账单号二选一填写 |
| out_order_no | string(64) | false | 商户分账单号，和微信分账单号二选一填写 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 电商平台二级商户号 |
| order_id | string(64) | true | 微信分账单号 |
| out_order_no | string(64) | true | 商户分账单号 |
| out_return_no | string(64) | true | 商户回退单号 |
| return_no | string(64) | true | 微信回退单号 |
| return_mchid | string(32) | true | 回退商户号 |
| amount | integer | true | 回退金额，单位为分 |
| result | string(32) | true | 回退结果，枚举值：PROCESSING SUCCESS FAILED |
| fail_reason | string(64) | false | 失败原因，枚举值：ACCOUNT_ABNORMAL TIME_OUT_CLOSED PAYER_ACCOUNT_ABNORMAL INVALID_REQUEST |
| finish_time | string | true | 完成时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| transaction_id | string(32) | true | 微信订单号 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 404 | RESOURCE_NOT_EXISTS | 记录不存在 | 请检查请求的单号是否正确 |
| 429 | FREQUENCY_LIMITED | 商户发起分账回退查询的频率过高 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 分账-解冻剩余资金

### 接口说明

请求方式：【POST】
`/v3/ecommerce/profitsharing/finish-order`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |
| transaction_id | string(32) | true | 微信订单号 |
| out_order_no | string(64) | true | 商户分账单号 |
| description | string(80) | true | 解冻原因描述 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 二级商户号 |
| transaction_id | string(32) | true | 微信订单号 |
| out_order_no | string(64) | true | 商户分账单号 |
| order_id | string(64) | true | 微信分账单号 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 403 | NO_AUTH | 商户无权限 | 请开通商户号分账权限 |
| 403 | NOT_ENOUGH | 分账金额为 0 | 分账已完成，无需再请求解冻剩余资金 |
| 429 | FREQUENCY_LIMITED | 对同笔订单分账频率过高 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 分账-查询订单剩余分账金额

### 接口说明

请求方式：【GET】
`/v3/ecommerce/profitsharing/orders/{transaction_id}/amounts`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| transaction_id | string(32) | true | 微信订单号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| transaction_id | string(32) | true | 微信订单号 |
| unsplit_amount | integer | true | 订单剩余待分金额，单位为分 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 404 | RESOURCE_NOT_EXISTS | 记录不存在 | 请检查请求的单号是否正确 |
| 429 | FREQUENCY_LIMITED | 商户发起查询的频率过高 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 分账-添加分账接收方

### 接口说明

请求方式：【POST】
`/v3/ecommerce/profitsharing/receivers/add`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| appid | string(32) | false | 公众账号 ID，分账接收方类型包含 PERSONAL_OPENID 时必填 |
| type | string(32) | true | 接收方类型，枚举值：MERCHANT_ID PERSONAL_OPENID |
| account | string(64) | true | 接收方账号 |
| name | string(256) | false | 接收方名称 |
| relation_type | string(32) | true | 与分账方的关系类型，枚举值：SUPPLIER DISTRIBUTOR SERVICE_PROVIDER PLATFORM OTHERS |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| type | string(32) | true | 接收方类型，枚举值：MERCHANT_ID PERSONAL_OPENID |
| account | string(64) | true | 接收方账号 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 403 | NO_AUTH | 商户无权限 | 请开通商户号分账权限 |
| 429 | FREQUENCY_LIMITED | 添加接收方频率过高 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 分账-删除分账接收方

### 接口说明

请求方式：【POST】
`/v3/ecommerce/profitsharing/receivers/delete`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| appid | string(32) | false | 公众账号 ID，分账接收方类型包含 PERSONAL_OPENID 时必填 |
| type | string(32) | true | 接收方类型，枚举值：MERCHANT_ID PERSONAL_OPENID |
| account | string(64) | true | 接收方账号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| type | string(32) | true | 接收方类型，枚举值：MERCHANT_ID PERSONAL_OPENID |
| account | string(64) | true | 接收方账号 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 403 | NO_AUTH | 商户无权限 | 请开通商户号分账权限 |
| 429 | FREQUENCY_LIMITED | 删除接收方频率过高 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 分账-分账动账通知

### 接口说明

请求方式：【POST】
`/v1/webhooks/wechat-ecommerce/profit-sharing-notify`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_mchid | string(32) | true | 服务商模式分账发起商户 |
| sub_mchid | string(32) | true | 服务商模式分账出资商户 |
| transaction_id | string(32) | true | 微信支付订单号 |
| order_id | string(64) | true | 微信分账或回退单号 |
| out_order_no | string(64) | true | 分账方系统内部的分账或回退单号 |
| receiver | object | true | 分账接收方对象 |
| receiver.type | string(32) | true | 分账接收方类型，枚举值：MERCHANT_ID PERSONAL_OPENID |
| receiver.account | string(64) | true | 分账接收方账号 |
| receiver.amount | int | true | 分账动账金额，单位为分 |
| receiver.description | string(80) | true | 分账或回退描述 |
| success_time | string(32) | true | 成功时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |