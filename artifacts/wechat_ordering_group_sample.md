# 支付下单组文档样例

本文件基于微信支付服务商普通支付与合单支付官方文档摘录，保留 contracts/errorcodes 审计所需的接口、字段、枚举与错误码结构。

## 普通支付-小程序下单

### 接口说明

请求方式：【POST】
`/v3/pay/partner/transactions/jsapi`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_appid | string(32) | true | 服务商 APPID |
| sp_mchid | string(32) | true | 服务商商户号 |
| sub_appid | string(32) | false | 子商户 APPID |
| sub_mchid | string(32) | true | 子商户号 |
| description | string(127) | true | 商品描述 |
| out_trade_no | string(32) | true | 商户订单号 |
| time_expire | string(64) | false | 支付结束时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| attach | string(128) | false | 附加数据 |
| notify_url | string(255) | true | 通知地址 |
| goods_tag | string(32) | false | 订单优惠标记 |
| settle_info | object | false | 结算信息 |
| settle_info.profit_sharing | boolean | false | 是否指定分账，枚举值：true：是 false：否 |
| settle_info.subsidy_amount | integer | false | 补差金额，单位为分 |
| support_fapiao | boolean | false | 电子发票入口开放标识，true：是 false：否 |
| amount | object | true | 订单金额 |
| amount.total | integer | true | 总金额，单位为分 |
| amount.currency | string(16) | false | 货币类型，CNY：人民币 |
| payer | object | true | 支付者信息 |
| payer.sp_openid | string(128) | false | 用户服务标识 |
| payer.sub_openid | string(128) | false | 用户子标识 |
| detail | object | false | 商品详情 |
| detail.cost_price | integer | false | 订单原价，单位为分 |
| detail.invoice_id | string(32) | false | 商品小票 ID |
| detail.goods_detail | array | false | 单品列表 |
| detail.goods_detail.merchant_goods_id | string(32) | true | 商户侧商品编码 |
| detail.goods_detail.wechatpay_goods_id | string(32) | false | 微信支付商品编码 |
| detail.goods_detail.goods_name | string(256) | false | 商品名称 |
| detail.goods_detail.quantity | integer | true | 商品数量 |
| detail.goods_detail.unit_price | integer | true | 商品单价，单位为分 |
| scene_info | object | false | 场景信息 |
| scene_info.payer_client_ip | string(45) | true | 用户终端 IP |
| scene_info.device_id | string(32) | false | 商户端设备号 |
| scene_info.store_info | object | false | 商户门店信息 |
| scene_info.store_info.id | string(32) | true | 门店编号 |
| scene_info.store_info.name | string(256) | false | 门店名称 |
| scene_info.store_info.area_code | string(32) | false | 地区编码 |
| scene_info.store_info.address | string(512) | false | 详细地址 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| prepay_id | string(64) | true | 预支付交易会话标识 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 400 | APPID_MCHID_NOT_MATCH | AppID 和 mch_id 不匹配 | 请确认 AppID 和 mch_id 是否匹配 |
| 400 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 400 | ORDER_CLOSED | 订单已关闭 | 当前订单已关闭，请重新下单 |
| 403 | ACCOUNT_ERROR | 账号异常 | 用户账号异常，无需更多操作 |
| 403 | NO_AUTH | 商户无权限 | 请商户前往申请此接口相关权限 |
| 403 | OUT_TRADE_NO_USED | 商户订单号重复 | 请核实商户订单号是否重复提交 |
| 403 | RULE_LIMIT | 业务规则限制 | 因业务规则限制请求频率，请查看接口返回的详细信息 |
| 403 | TRADE_ERROR | 交易错误 | 因业务原因交易失败，请查看接口返回的详细信息 |
| 404 | ORDER_NOT_EXIST | 订单不存在 | 请检查订单是否发起过交易 |
| 429 | FREQUENCY_LIMITED | 频率超限 | 请降低请求接口频率 |
| 500 | BANK_ERROR | 银行系统异常 | 银行系统异常，请用相同参数重新调用 |
| 500 | INVALID_TRANSACTIONID | 订单号非法 | 请检查微信支付订单号是否正确 |
| 500 | OPENID_MISMATCH | OpenID 和 AppID 不匹配 | 请确认 OpenID 和 AppID 是否匹配 |
| 500 | SYSTEM_ERROR | 系统错误 | 系统异常，请用相同参数重新调用 |

## 普通支付-微信支付订单号查询订单

### 接口说明

请求方式：【GET】
`/v3/pay/partner/transactions/id/{transaction_id}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| transaction_id | string(32) | true | 微信支付订单号 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_mchid | string(32) | true | 服务商商户号 |
| sub_mchid | string(32) | true | 子商户号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_appid | string(32) | true | 服务商 APPID |
| sp_mchid | string(32) | true | 服务商商户号 |
| sub_appid | string(32) | false | 子商户 APPID |
| sub_mchid | string(32) | true | 子商户号 |
| out_trade_no | string(32) | true | 商户订单号 |
| transaction_id | string(32) | true | 微信支付订单号 |
| trade_type | string(16) | true | 交易类型，枚举值：JSAPI NATIVE APP MICROPAY MWEB FACEPAY |
| trade_state | string(32) | true | 交易状态，枚举值：SUCCESS REFUND NOTPAY CLOSED REVOKED USERPAYING PAYERROR |
| trade_state_desc | string(256) | true | 交易状态描述 |
| bank_type | string(32) | false | 银行类型 |
| attach | string(128) | false | 商户数据包 |
| success_time | string(64) | false | 支付完成时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| payer | object | false | 支付者信息 |
| payer.sp_openid | string(128) | false | 用户服务商标识 |
| payer.sub_openid | string(128) | false | 用户子商户标识 |
| amount | object | false | 订单金额 |
| amount.total | integer | false | 总金额，单位为分 |
| amount.payer_total | integer | false | 用户支付金额，单位为分 |
| amount.currency | string(16) | false | 固定返回：CNY |
| amount.payer_currency | string(16) | false | 固定返回：CNY |
| scene_info | object | false | 场景信息 |
| scene_info.device_id | string(32) | true | 商户端设备号 |
| promotion_detail | array[object] | false | 优惠功能 |
| promotion_detail.coupon_id | string(32) | true | 券 ID |
| promotion_detail.name | string(64) | true | 优惠名称 |
| promotion_detail.scope | string(32) | false | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string(32) | false | 优惠类型，枚举值：CASH NOCASH |
| promotion_detail.amount | integer | true | 优惠券面额，单位为分 |
| promotion_detail.stock_id | string(32) | false | 活动 ID |
| promotion_detail.wechatpay_contribute | integer | false | 微信出资，单位为分 |
| promotion_detail.merchant_contribute | integer | false | 商户出资，单位为分 |
| promotion_detail.other_contribute | integer | false | 其他出资，单位为分 |
| promotion_detail.currency | string(16) | false | 固定返回：CNY |
| promotion_detail.goods_detail | array[object] | false | 单品列表 |
| promotion_detail.goods_detail.goods_id | string(32) | true | 商品编码 |
| promotion_detail.goods_detail.quantity | integer | true | 商品数量 |
| promotion_detail.goods_detail.unit_price | integer | true | 商品单价，单位为分 |
| promotion_detail.goods_detail.discount_amount | integer | true | 商品优惠金额，单位为分 |
| promotion_detail.goods_detail.goods_remark | string(128) | false | 商品备注 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 400 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 403 | RULE_LIMIT | 业务规则限制 | 因业务规则限制请求频率，请查看接口返回的详细信息 |
| 403 | TRADE_ERROR | 交易错误 | 因业务原因交易失败，请查看接口返回的详细信息 |
| 404 | ORDER_NOT_EXIST | 订单不存在 | 请检查传入的微信支付订单号是否正确 |
| 429 | FREQUENCY_LIMITED | 频率超限 | 请降低请求接口频率 |
| 500 | SYSTEM_ERROR | 系统错误 | 系统异常，请用相同参数重新调用 |

## 普通支付-商户订单号查询订单

### 接口说明

请求方式：【GET】
`/v3/pay/partner/transactions/out-trade-no/{out_trade_no}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| out_trade_no | string(32) | true | 商户订单号 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_mchid | string(32) | true | 服务商商户号 |
| sub_mchid | string(32) | true | 子商户号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_appid | string(32) | true | 服务商 APPID |
| sp_mchid | string(32) | true | 服务商商户号 |
| sub_appid | string(32) | false | 子商户 APPID |
| sub_mchid | string(32) | true | 子商户号 |
| out_trade_no | string(32) | true | 商户订单号 |
| transaction_id | string(32) | false | 微信支付订单号 |
| trade_type | string(16) | false | 交易类型，枚举值：JSAPI NATIVE APP MICROPAY MWEB FACEPAY |
| trade_state | string(32) | true | 交易状态，枚举值：SUCCESS REFUND NOTPAY CLOSED REVOKED USERPAYING PAYERROR |
| trade_state_desc | string(256) | true | 交易状态描述 |
| bank_type | string(32) | false | 银行类型 |
| attach | string(128) | false | 商户数据包 |
| success_time | string(64) | false | 支付完成时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| payer | object | false | 支付者信息 |
| payer.sp_openid | string(128) | false | 用户服务商标识 |
| payer.sub_openid | string(128) | false | 用户子商户标识 |
| amount | object | false | 订单金额 |
| amount.total | integer | false | 总金额，单位为分 |
| amount.payer_total | integer | false | 用户支付金额，单位为分 |
| amount.currency | string(16) | false | 固定返回：CNY |
| amount.payer_currency | string(16) | false | 固定返回：CNY |
| scene_info | object | false | 场景信息 |
| scene_info.device_id | string(32) | true | 商户端设备号 |
| promotion_detail | array[object] | false | 优惠功能 |
| promotion_detail.coupon_id | string(32) | true | 券 ID |
| promotion_detail.name | string(64) | true | 优惠名称 |
| promotion_detail.scope | string(32) | false | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string(32) | false | 优惠类型，枚举值：CASH NOCASH |
| promotion_detail.amount | integer | true | 优惠券面额，单位为分 |
| promotion_detail.stock_id | string(32) | false | 活动 ID |
| promotion_detail.wechatpay_contribute | integer | false | 微信出资，单位为分 |
| promotion_detail.merchant_contribute | integer | false | 商户出资，单位为分 |
| promotion_detail.other_contribute | integer | false | 其他出资，单位为分 |
| promotion_detail.currency | string(16) | false | 固定返回：CNY |
| promotion_detail.goods_detail | array[object] | false | 单品列表 |
| promotion_detail.goods_detail.goods_id | string(32) | true | 商品编码 |
| promotion_detail.goods_detail.quantity | integer | true | 商品数量 |
| promotion_detail.goods_detail.unit_price | integer | true | 商品单价，单位为分 |
| promotion_detail.goods_detail.discount_amount | integer | true | 商品优惠金额，单位为分 |
| promotion_detail.goods_detail.goods_remark | string(128) | false | 商品备注 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 400 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 403 | RULE_LIMIT | 业务规则限制 | 因业务规则限制请求频率，请查看接口返回的详细信息 |
| 403 | TRADE_ERROR | 交易错误 | 因业务原因交易失败，请查看接口返回的详细信息 |
| 404 | ORDER_NOT_EXIST | 订单不存在 | 请检查商户订单号是否下单成功 |
| 429 | FREQUENCY_LIMITED | 频率超限 | 请降低请求接口频率 |
| 500 | SYSTEM_ERROR | 系统错误 | 系统异常，请用相同参数重新调用 |

## 普通支付-关闭订单

### 接口说明

请求方式：【POST】
`/v3/pay/partner/transactions/out-trade-no/{out_trade_no}/close`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| out_trade_no | string(32) | true | 商户订单号 |

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_mchid | string(32) | true | 服务商商户号 |
| sub_mchid | string(32) | true | 子商户号 |

### 应答参数

无应答包体

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 400 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 403 | RULE_LIMIT | 业务规则限制 | 因业务规则限制请求频率，请查看接口返回的详细信息 |
| 403 | TRADE_ERROR | 交易错误 | 因业务原因交易失败，请查看接口返回的详细信息 |
| 429 | FREQUENCY_LIMITED | 频率超限 | 请降低请求接口频率 |
| 500 | SYSTEM_ERROR | 系统错误 | 系统异常，请用相同参数重新调用 |

## 合单支付-JSAPI 下单

### 接口说明

请求方式：【POST】
`/v3/combine-transactions/jsapi`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| combine_appid | string(32) | true | 合单商户 Appid |
| combine_mchid | string(32) | true | 合单商户号 |
| combine_out_trade_no | string(32) | true | 合单商户订单号 |
| scene_info | object | false | 场景信息 |
| scene_info.device_id | string(16) | false | 终端设备号 |
| scene_info.payer_client_ip | string(45) | true | 用户终端 IP |
| sub_orders | array[object] | true | 商品单列表 |
| sub_orders.mchid | string(32) | true | 商品单商户号 |
| sub_orders.attach | string(128) | true | 附加数据 |
| sub_orders.amount | object | true | 订单金额 |
| sub_orders.amount.total_amount | integer | true | 标价金额，单位为分 |
| sub_orders.amount.currency | string(8) | true | 标价币种，人民币：CNY |
| sub_orders.out_trade_no | string(32) | true | 商品单商户订单号 |
| sub_orders.sub_mchid | string(32) | false | 特约商户商户号 |
| sub_orders.description | string(127) | true | 商品描述 |
| sub_orders.settle_info | object | false | 结算信息 |
| sub_orders.settle_info.profit_sharing | boolean | false | 是否指定分账，枚举值：true：是 false：否 |
| sub_orders.settle_info.subsidy_amount | integer | false | 补差金额，单位为分 |
| sub_orders.sub_appid | string(32) | false | 子商户绑定的 Appid |
| sub_orders.goods_tag | string(32) | false | 订单优惠标记 |
| combine_payer_info | object | true | 支付者 |
| combine_payer_info.openid | string(128) | false | 用户标识 |
| combine_payer_info.sub_openid | string(128) | false | 用户子标识 |
| time_start | string | false | 支付起始时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| time_expire | string | false | 支付结束时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| notify_url | string(255) | false | 通知地址 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| prepay_id | string(64) | true | 预支付交易会话标识 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 202 | USERPAYING | 用户支付中，需要输入密码 | 等待 5 秒后查询订单 |
| 400 | APPID_MCHID_NOT_MATCH | AppID 和 mch_id 不匹配 | 请确认 AppID 和 mch_id 是否匹配 |
| 400 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 400 | ORDER_CLOSED | 订单已关闭 | 当前订单已关闭，请重新下单 |
| 403 | ACCOUNTERROR | 账号异常 | 用户账号异常，无需更多操作 |
| 403 | NOAUTH | 商户无权限 | 请商户前往申请此接口相关权限 |
| 403 | OUT_TRADE_NO_USED | 商户订单号重复 | 请核实商户订单号是否重复提交 |
| 403 | RULELIMIT | 业务规则限制 | 因业务规则限制请求频率，请查看接口返回的详细信息 |
| 403 | TRADE_ERROR | 交易错误 | 因业务原因交易失败，请查看接口返回的详细信息 |
| 404 | ORDERNOTEXIST | 订单不存在 | 请检查订单是否发起过交易 |
| 429 | FREQUENCY_LIMITED | 频率超限 | 请降低请求接口频率 |
| 500 | BANKERROR | 银行系统异常 | 银行系统异常，请用相同参数重新调用 |
| 500 | INVALID_TRANSACTIONID | 订单号非法 | 请检查微信支付订单号是否正确 |
| 500 | OPENID_MISMATCH | OpenID 和 AppID 不匹配 | 请确认 OpenID 和 AppID 是否匹配 |
| 500 | SYSTEMERROR | 系统错误 | 系统异常，请用相同参数重新调用 |

## 普通支付-支付结果通知

### 接口说明

请求方式：【POST】
`/v1/webhooks/wechat-ecommerce/payment-notify`

### Body 参数

以下字段对应 resource.ciphertext 解密后的通知资源对象。

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| sp_appid | string(32) | true | 服务商 APPID |
| sp_mchid | string(32) | true | 服务商商户号 |
| sub_appid | string(32) | false | 子商户 APPID |
| sub_mchid | string(32) | true | 子商户号 |
| out_trade_no | string(32) | true | 商户订单号 |
| transaction_id | string(32) | true | 微信支付订单号 |
| trade_type | string(16) | true | 交易类型，枚举值：JSAPI NATIVE APP MICROPAY MWEB FACEPAY |
| trade_state | string(32) | true | 交易状态，枚举值：SUCCESS REFUND NOTPAY CLOSED REVOKED USERPAYING PAYERROR |
| trade_state_desc | string(256) | true | 交易状态描述 |
| bank_type | string(32) | true | 银行类型 |
| attach | string(128) | false | 商户数据包 |
| success_time | string(64) | true | 支付完成时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| payer | object | true | 支付者信息 |
| payer.sp_openid | string(128) | false | 用户服务商标识 |
| payer.sub_openid | string(128) | false | 用户子商户标识 |
| amount | object | true | 订单金额 |
| amount.total | integer | true | 总金额，单位为分 |
| amount.payer_total | integer | true | 用户支付金额，单位为分 |
| amount.currency | string(16) | true | 货币类型，固定返回：CNY |
| amount.payer_currency | string(16) | true | 用户支付币种，固定返回：CNY |
| scene_info | object | false | 场景信息 |
| scene_info.device_id | string(32) | false | 商户端设备号 |
| promotion_detail | array[object] | false | 优惠功能 |
| promotion_detail.coupon_id | string(32) | true | 券 ID |
| promotion_detail.name | string(64) | false | 优惠名称 |
| promotion_detail.scope | string(32) | false | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string(32) | false | 优惠类型，枚举值：CASH NOCASH |
| promotion_detail.amount | integer | true | 当前订单中享受的优惠券金额，单位为分 |
| promotion_detail.stock_id | string(32) | false | 活动 ID |
| promotion_detail.wechatpay_contribute | integer | false | 微信出资，单位为分 |
| promotion_detail.merchant_contribute | integer | false | 商户出资，单位为分 |
| promotion_detail.other_contribute | integer | false | 其他出资，单位为分 |
| promotion_detail.currency | string(16) | false | 固定返回：CNY |
| promotion_detail.goods_detail | array[object] | false | 单品列表 |
| promotion_detail.goods_detail.goods_id | string(32) | true | 商品编码 |
| promotion_detail.goods_detail.quantity | integer | true | 商品数量 |
| promotion_detail.goods_detail.unit_price | integer | true | 商品单价，单位为分 |
| promotion_detail.goods_detail.discount_amount | integer | true | 商品优惠金额，单位为分 |
| promotion_detail.goods_detail.goods_remark | string(128) | false | 商品备注 |

### 应答参数

无应答包体

## 合单支付-合单支付通知

### 接口说明

请求方式：【POST】
`/v1/webhooks/wechat-ecommerce/combine-notify`

### Body 参数

以下字段对应 resource.ciphertext 解密后的通知资源对象。

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| combine_appid | string(32) | true | 合单发起方 AppID |
| combine_mchid | string(32) | true | 合单发起方商户号 |
| combine_out_trade_no | string(32) | true | 合单支付总订单号 |
| scene_info | object | false | 支付场景信息描述 |
| scene_info.device_id | string(16) | false | 终端设备号 |
| sub_orders | array[object] | true | 商品单列表 |
| sub_orders.mchid | string(32) | true | 商品单商户号 |
| sub_orders.trade_type | string(16) | true | 交易类型，枚举值：NATIVE JSAPI APP MWEB |
| sub_orders.trade_state | string(32) | true | 交易状态，枚举值：SUCCESS REFUND NOTPAY CLOSED PAYERROR |
| sub_orders.bank_type | string(32) | true | 银行类型 |
| sub_orders.attach | string(128) | false | 附加数据 |
| sub_orders.success_time | string(32) | true | 支付完成时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss.sss+TIMEZONE |
| sub_orders.transaction_id | string(32) | true | 微信支付订单号 |
| sub_orders.out_trade_no | string(32) | true | 商品单订单号 |
| sub_orders.sub_mchid | string(32) | false | 特约商户商户号 |
| sub_orders.sub_appid | string(32) | false | 子商户绑定的 Appid |
| sub_orders.amount | object | true | 订单金额信息 |
| sub_orders.amount.total_amount | integer | true | 商品单金额，单位为分 |
| sub_orders.amount.currency | string(16) | true | 标价币种，人民币：CNY |
| sub_orders.amount.payer_amount | integer | true | 用户支付金额，单位为分 |
| sub_orders.amount.payer_currency | string(8) | true | 用户实际支付币种，固定返回：CNY |
| sub_orders.amount.settlement_rate | integer | false | 结算汇率 |
| sub_orders.promotion_detail | array[object] | false | 优惠功能 |
| sub_orders.promotion_detail.coupon_id | string(32) | true | 券 ID |
| sub_orders.promotion_detail.name | string(64) | false | 优惠名称 |
| sub_orders.promotion_detail.scope | string(32) | false | 优惠范围，枚举值：GLOBAL SINGLE |
| sub_orders.promotion_detail.type | string(32) | false | 优惠类型，枚举值：CASH NOCASH |
| sub_orders.promotion_detail.amount | integer | true | 优惠券面额，单位为分 |
| sub_orders.promotion_detail.stock_id | string(32) | false | 活动 ID |
| sub_orders.promotion_detail.wechatpay_contribute | integer | false | 微信出资，单位为分 |
| sub_orders.promotion_detail.merchant_contribute | integer | false | 商户出资，单位为分 |
| sub_orders.promotion_detail.other_contribute | integer | false | 其他出资，单位为分 |
| sub_orders.promotion_detail.currency | string(16) | false | 固定返回：CNY |
| sub_orders.promotion_detail.goods_detail | array[object] | false | 单品列表 |
| sub_orders.promotion_detail.goods_detail.goods_id | string(32) | true | 商品编码 |
| sub_orders.promotion_detail.goods_detail.quantity | integer | true | 商品数量 |
| sub_orders.promotion_detail.goods_detail.unit_price | integer | true | 商品单价，单位为分 |
| sub_orders.promotion_detail.goods_detail.discount_amount | integer | true | 商品优惠金额，单位为分 |
| sub_orders.promotion_detail.goods_detail.goods_remark | string(128) | false | 商品备注 |
| combine_payer_info | object | true | 支付者信息 |
| combine_payer_info.openid | string(128) | true | 用户标识 |

### 应答参数

无应答包体

## 合单支付-查询订单

### 接口说明

请求方式：【GET】
`/v3/combine-transactions/out-trade-no/{combine_out_trade_no}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| combine_out_trade_no | string(32) | true | 合单商户订单号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| combine_appid | string(32) | true | 合单商户 Appid |
| combine_mchid | string(32) | true | 合单商户号 |
| combine_payer_info | object | false | 合单支付者 |
| combine_payer_info.openid | string(128) | false | 用户标识 |
| sub_orders | array[object] | false | 商品单信息 |
| sub_orders.mchid | string(32) | true | 商品单商户号 |
| sub_orders.trade_type | string(16) | false | 交易类型，枚举值：NATIVE JSAPI APP MWEB |
| sub_orders.trade_state | string(32) | true | 交易状态，枚举值：SUCCESS REFUND NOTPAY CLOSED PAYERROR |
| sub_orders.bank_type | string(32) | false | 付款银行 |
| sub_orders.attach | string(128) | false | 附加数据 |
| sub_orders.success_time | string(32) | false | 支付完成时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| sub_orders.amount | object | false | 订单金额信息 |
| sub_orders.amount.total_amount | integer | true | 标价金额，单位为分 |
| sub_orders.amount.payer_amount | integer | true | 用户支付金额，单位为分 |
| sub_orders.amount.currency | string(16) | true | 货币类型，固定返回：CNY |
| sub_orders.amount.payer_currency | string(16) | true | 用户支付币种，固定返回：CNY |
| sub_orders.amount.settlement_rate | integer | false | 结算汇率 |
| sub_orders.transaction_id | string(32) | false | 微信支付订单号 |
| sub_orders.out_trade_no | string(32) | true | 商品单订单号 |
| sub_orders.sub_mchid | string(32) | false | 特约商户商户号 |
| sub_orders.sub_appid | string(32) | false | 子商户绑定的 Appid |
| sub_orders.sub_openid | string(128) | false | sub_appid 对应的 openid |
| sub_orders.promotion_detail | array[object] | false | 优惠功能 |
| sub_orders.promotion_detail.coupon_id | string(32) | true | 券 ID |
| sub_orders.promotion_detail.name | string(64) | false | 优惠名称 |
| sub_orders.promotion_detail.scope | string(32) | false | 优惠范围，枚举值：GLOBAL SINGLE |
| sub_orders.promotion_detail.type | string(32) | false | 优惠类型，枚举值：CASH NOCASH |
| sub_orders.promotion_detail.amount | integer | true | 优惠券面额，单位为分 |
| sub_orders.promotion_detail.stock_id | string(32) | false | 活动 ID |
| sub_orders.promotion_detail.wechatpay_contribute | integer | false | 微信出资，单位为分 |
| sub_orders.promotion_detail.merchant_contribute | integer | false | 商户出资，单位为分 |
| sub_orders.promotion_detail.other_contribute | integer | false | 其他出资，单位为分 |
| sub_orders.promotion_detail.currency | string(16) | false | 固定返回：CNY |
| sub_orders.promotion_detail.goods_detail | array[object] | false | 单品列表 |
| sub_orders.promotion_detail.goods_detail.goods_id | string(32) | true | 商品编码 |
| sub_orders.promotion_detail.goods_detail.quantity | integer | true | 商品数量 |
| sub_orders.promotion_detail.goods_detail.unit_price | integer | true | 商品单价，单位为分 |
| sub_orders.promotion_detail.goods_detail.discount_amount | integer | true | 商品优惠金额，单位为分 |
| sub_orders.promotion_detail.goods_detail.goods_remark | string(128) | false | 商品备注 |
| scene_info | object | false | 支付场景描述 |
| scene_info.device_id | string(32) | false | 商户端设备号 |
| combine_out_trade_no | string(32) | true | 合单商户订单号 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 202 | USERPAYING | 用户支付中，需要输入密码 | 等待 5 秒后查询订单 |
| 400 | APPID_MCHID_NOT_MATCH | AppID 和 mch_id 不匹配 | 请确认 AppID 和 mch_id 是否匹配 |
| 400 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 400 | ORDER_CLOSED | 订单已关闭 | 当前订单已关闭，请重新下单 |
| 403 | ACCOUNTERROR | 账号异常 | 用户账号异常，无需更多操作 |
| 403 | NOAUTH | 商户无权限 | 请商户前往申请此接口相关权限 |
| 403 | OUT_TRADE_NO_USED | 商户订单号重复 | 请核实商户订单号是否重复提交 |
| 403 | RULELIMIT | 业务规则限制 | 因业务规则限制请求频率，请查看接口返回的详细信息 |
| 403 | TRADE_ERROR | 交易错误 | 因业务原因交易失败，请查看接口返回的详细信息 |
| 404 | ORDERNOTEXIST | 订单不存在 | 请检查订单是否发起过交易 |
| 429 | FREQUENCY_LIMITED | 频率超限 | 请降低请求接口频率 |
| 500 | BANKERROR | 银行系统异常 | 银行系统异常，请用相同参数重新调用 |
| 500 | INVALID_TRANSACTIONID | 订单号非法 | 请检查微信支付订单号是否正确 |
| 500 | OPENID_MISMATCH | OpenID 和 AppID 不匹配 | 请确认 OpenID 和 AppID 是否匹配 |
| 500 | SYSTEMERROR | 系统错误 | 系统异常，请用相同参数重新调用 |

## 合单支付-关闭订单

### 接口说明

请求方式：【POST】
`/v3/combine-transactions/out-trade-no/{combine_out_trade_no}/close`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| combine_out_trade_no | string(32) | true | 合单商户单号 |

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| combine_appid | string(32) | true | 合单 Appid |
| sub_orders | array[object] | true | 商品单信息 |
| sub_orders.mchid | string(32) | true | 商品单商户号 |
| sub_orders.out_trade_no | string(32) | true | 商品单订单号 |
| sub_orders.sub_mchid | string(32) | false | 特约商户商户号 |
| sub_orders.sub_appid | string(32) | false | 服务商模式下，sub_mchid 对应的 sub_appid |

### 应答参数

无应答包体

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP 请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 验证不通过 | 请参阅签名常见问题 |
| 202 | USERPAYING | 用户支付中，需要输入密码 | 等待 5 秒后查询订单 |
| 400 | APPID_MCHID_NOT_MATCH | AppID 和 mch_id 不匹配 | 请确认 AppID 和 mch_id 是否匹配 |
| 400 | INVALID_REQUEST | 输入的请求非法，请求关单的子单信息与下单时的子单信息不一致 | 请检查关单传入的子单信息 |
| 400 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 400 | ORDER_CLOSED | 订单已关闭 | 当前订单已关闭，请重新下单 |
| 403 | ACCOUNTERROR | 账号异常 | 用户账号异常，无需更多操作 |
| 403 | NOAUTH | 商户无权限 | 请商户前往申请此接口相关权限 |
| 403 | NOTENOUGH | 余额不足 | 用户账号余额不足 |
| 403 | OUT_TRADE_NO_USED | 商户订单号重复 | 请核实商户订单号是否重复提交 |
| 403 | RULELIMIT | 业务规则限制 | 因业务规则限制请求频率，请查看接口返回的详细信息 |
| 403 | TRADE_ERROR | 交易错误 | 因业务原因交易失败，请查看接口返回的详细信息 |
| 404 | ORDERNOTEXIST | 订单不存在 | 请检查订单是否发起过交易 |
| 429 | FREQUENCY_LIMITED | 频率超限 | 请降低请求接口频率 |
| 500 | BANKERROR | 银行系统异常 | 银行系统异常，请用相同参数重新调用 |
| 500 | INVALID_TRANSACTIONID | 订单号非法 | 请检查微信支付订单号是否正确 |
| 500 | OPENID_MISMATCH | OpenID 和 AppID 不匹配 | 请确认 OpenID 和 AppID 是否匹配 |
| 500 | SYSTEMERROR | 系统错误 | 系统异常，请用相同参数重新调用 |