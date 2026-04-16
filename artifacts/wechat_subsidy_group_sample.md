# 补差与分账组-补差文档样例

本文件基于微信支付服务商补差组官方文档摘录，保留 contracts、errorcodes 与 alignment audit 所需的接口、字段、枚举和语义约束。

来源页面：

- 请求补差：https://pay.weixin.qq.com/doc/v3/partner/4012477631.md
- 请求补差回退：https://pay.weixin.qq.com/doc/v3/partner/4012477636.md
- 取消补差：https://pay.weixin.qq.com/doc/v3/partner/4012477639.md

## 补差-请求补差

### 接口说明

请求方式：【POST】
`/v3/ecommerce/subsidies/create`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 电商平台二级商户号 |
| transaction_id | string(64) | true | 微信订单号 |
| amount | integer | true | 补差金额，单位为分 |
| description | string(80) | true | 补差描述 |
| out_subsidy_no | string | false | 商户补差单号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string | true | 电商平台二级商户号 |
| transaction_id | string | true | 微信订单号 |
| subsidy_id | string(64) | true | 微信补差单号 |
| description | string | true | 补差描述 |
| amount | integer | false | 补差金额，单位为分 |
| result | string | true | 补差单结果，枚举值：SUCCESS FAIL REFUND |
| success_time | string | true | 补差完成时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| out_subsidy_no | string | false | 商户补差单号 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 补差-请求补差回退

### 接口说明

请求方式：【POST】
`/v3/ecommerce/subsidies/return`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 电商平台二级商户号 |
| out_order_no | string(64) | true | 商户补差回退单号 |
| transaction_id | string(64) | true | 微信订单号 |
| refund_id | string(64) | false | 微信退款单号 |
| amount | integer | true | 补差回退金额，单位为分 |
| description | string(80) | true | 补差回退描述 |
| subsidy_id | string(64) | false | 微信补差单号 |
| from | array[object] | false | 回退出资账户及金额 |
| from.account | string | false | 出资账户类型，枚举值：AVAILABLE UNAVAILABLE |
| from.amount | integer | false | 出资金额，单位为分 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string | true | 电商平台二级商户号 |
| transaction_id | string | true | 微信订单号 |
| subsidy_refund_id | string | true | 微信补差回退单号 |
| refund_id | string | false | 微信退款单号 |
| out_order_no | string | true | 商户补差回退单号 |
| amount | integer | true | 补差回退金额，单位为分 |
| description | string | true | 补差回退描述 |
| result | string | true | 补差回退结果，枚举值：SUCCESS FAIL |
| success_time | string | true | 补差回退完成时间，遵循 RFC3339 标准格式，格式为yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| subsidy_id | string(64) | false | 微信补差单号 |
| from | array[object] | false | 回退出资账户及金额 |
| from.account | string | false | 出资账户类型，枚举值：AVAILABLE UNAVAILABLE |
| from.amount | integer | false | 出资金额，单位为分 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |

## 补差-取消补差

### 接口说明

请求方式：【POST】
`/v3/ecommerce/subsidies/cancel`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string(32) | true | 电商平台二级商户号 |
| transaction_id | string(64) | true | 微信订单号 |
| description | string(80) | true | 取消补差描述 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sub_mchid | string | true | 电商平台二级商户号 |
| transaction_id | string | true | 微信订单号 |
| result | string | true | 取消补差结果，枚举值：SUCCESS FAIL |
| description | string | true | 取消补差描述 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请检查签名参数和签名方法 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 请降低频率后重试 |
| 500 | SYSTEM_ERROR | 系统异常，请稍后重试 | 请稍后重试 |