# 直连支付组文档样例

本文件基于微信支付普通商户直连支付官方文档摘录，保留 contracts、errorcodes 与 alignment audit 所需的接口、字段、枚举和语义约束。

来源页面：

- JSAPI下单：https://pay.weixin.qq.com/doc/v3/merchant/4012791897.md
- 微信支付订单号查询订单：https://pay.weixin.qq.com/doc/v3/merchant/4012791899.md
- 商户订单号查询订单：https://pay.weixin.qq.com/doc/v3/merchant/4012791900.md
- 关闭订单：https://pay.weixin.qq.com/doc/v3/merchant/4012791901.md
- 支付结果通知：https://pay.weixin.qq.com/doc/v3/merchant/4012791902.md
- 申请退款：https://pay.weixin.qq.com/doc/v3/merchant/4012791903.md
- 查询单笔退款：https://pay.weixin.qq.com/doc/v3/merchant/4012791904.md
- 发起异常退款：https://pay.weixin.qq.com/doc/v3/merchant/4012791905.md
- 退款结果通知：https://pay.weixin.qq.com/doc/v3/merchant/4012791906.md

## 直连支付-JSAPI下单

### 接口说明

请求方式：【POST】
`/v3/pay/transactions/jsapi`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| appid | string(32) | true | 公众账号ID |
| mchid | string(32) | true | 商户号 |
| description | string(127) | true | 商品描述 |
| out_trade_no | string(32) | true | 商户订单号 |
| time_expire | string(64) | false | 支付结束时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| attach | string(128) | false | 商户数据包 |
| notify_url | string(255) | true | 商户回调地址 |
| goods_tag | string(32) | false | 订单优惠标记 |
| support_fapiao | boolean | false | 电子发票入口开放标识，枚举值：true false |
| amount | object | true | 订单金额 |
| amount.total | integer | true | 总金额，单位为分，整型，必须大于0 |
| amount.currency | string(16) | false | 货币类型，固定传：CNY |
| payer | object | true | 支付者信息 |
| payer.openid | string(128) | true | 用户标识 |
| detail | object | false | 商品详情 |
| detail.cost_price | integer | false | 订单原价，单位为分 |
| detail.invoice_id | string(32) | false | 商品小票ID |
| detail.goods_detail | array[object] | false | 单品列表 |
| detail.goods_detail.merchant_goods_id | string(32) | true | 商户侧商品编码 |
| detail.goods_detail.wechatpay_goods_id | string(32) | false | 微信支付商品编码 |
| detail.goods_detail.goods_name | string(256) | false | 商品名称 |
| detail.goods_detail.quantity | integer | true | 商品数量 |
| detail.goods_detail.unit_price | integer | true | 商品单价，单位为分 |
| scene_info | object | false | 场景信息 |
| scene_info.payer_client_ip | string(45) | true | 用户终端IP |
| scene_info.device_id | string(32) | false | 商户端设备号 |
| scene_info.store_info | object | false | 商户门店信息 |
| scene_info.store_info.id | string(32) | true | 门店编号 |
| scene_info.store_info.name | string(256) | false | 门店名称 |
| scene_info.store_info.area_code | string(32) | false | 地区编码 |
| scene_info.store_info.address | string(512) | false | 详细地址 |
| settle_info | object | false | 结算信息 |
| settle_info.profit_sharing | boolean | false | 分账标识，枚举值：true false |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| prepay_id | string(64) | true | 预支付交易会话标识 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 无效请求 | 请根据接口返回的详细信息检查 |
| 400 | APPID_MCHID_NOT_MATCH | AppID和mch_id不匹配 | 请确认 AppID 和 mch_id 是否匹配 |
| 400 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 401 | SIGN_ERROR | 签名错误 | 请检查签名参数和方法是否都符合签名算法要求 |
| 403 | NO_AUTH | 商户无权限 | 请商户前往商户平台申请此接口相关权限 |
| 403 | OUT_TRADE_NO_USED | 商户订单号重复 | 请核实商户订单号是否重复提交 |
| 429 | FREQUENCY_LIMITED | 频率超限 | 请求频率超限，请降低请求接口频率 |
| 500 | SYSTEM_ERROR | 系统错误 | 系统异常，请用相同参数重新调用 |

## 直连支付-微信支付订单号查询订单

### 接口说明

请求方式：【GET】
`/v3/pay/transactions/id/{transaction_id}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| transaction_id | string(32) | true | 微信支付订单号 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| mchid | string(32) | true | 商户号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| appid | string(32) | true | 公众账号ID |
| mchid | string(32) | true | 商户号 |
| out_trade_no | string(32) | true | 商户订单号 |
| transaction_id | string(32) | true | 微信支付订单号 |
| trade_type | string(16) | true | 交易类型，枚举值：JSAPI NATIVE APP MICROPAY MWEB FACEPAY |
| trade_state | string(32) | true | 交易状态，枚举值：SUCCESS REFUND NOTPAY CLOSED REVOKED USERPAYING PAYERROR |
| trade_state_desc | string(256) | true | 交易状态描述 |
| bank_type | string(32) | false | 银行类型 |
| attach | string(128) | false | 商户数据包 |
| success_time | string(64) | false | 支付完成时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| payer | object | false | 支付者信息 |
| payer.openid | string(128) | false | 用户标识 |
| amount | object | false | 订单金额 |
| amount.total | integer | false | 总金额，单位为分 |
| amount.payer_total | integer | false | 用户支付金额，单位为分 |
| amount.currency | string(16) | false | 固定返回：CNY |
| amount.payer_currency | string(16) | false | 固定返回：CNY |
| scene_info | object | false | 场景信息 |
| scene_info.device_id | string(32) | true | 商户端设备号 |
| promotion_detail | array[object] | false | 优惠功能 |
| promotion_detail.coupon_id | string(32) | true | 券ID |
| promotion_detail.name | string(64) | true | 优惠名称 |
| promotion_detail.scope | string(32) | false | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string(32) | false | 优惠类型，枚举值：CASH NOCASH |
| promotion_detail.amount | integer | true | 优惠券面额，单位为分 |
| promotion_detail.stock_id | string(32) | false | 活动ID |
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
| 400 | INVALID_REQUEST | 无效请求 | 请根据接口返回的详细信息检查 |
| 400 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 401 | SIGN_ERROR | 签名错误 | 请检查签名参数和方法是否都符合签名算法要求 |
| 403 | RULE_LIMIT | 业务规则限制 | 因业务规则限制请求频率，请查看接口返回的详细信息 |
| 403 | TRADE_ERROR | 交易错误 | 因业务原因交易失败，请查看接口返回的详细信息 |
| 404 | ORDER_NOT_EXIST | 订单不存在 | 请检查传入的微信支付订单号是否正确 |
| 429 | FREQUENCY_LIMITED | 频率超限 | 请降低请求接口频率 |
| 500 | SYSTEM_ERROR | 系统错误 | 系统异常，请用相同参数重新调用 |

## 直连支付-商户订单号查询订单

### 接口说明

请求方式：【GET】
`/v3/pay/transactions/out-trade-no/{out_trade_no}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| out_trade_no | string(32) | true | 商户订单号 |

### Query 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| mchid | string(32) | true | 商户号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| appid | string(32) | true | 公众账号ID |
| mchid | string(32) | true | 商户号 |
| out_trade_no | string(32) | true | 商户订单号 |
| transaction_id | string(32) | false | 微信支付订单号 |
| trade_type | string(16) | false | 交易类型，枚举值：JSAPI NATIVE APP MICROPAY MWEB FACEPAY |
| trade_state | string(32) | true | 交易状态，枚举值：SUCCESS REFUND NOTPAY CLOSED REVOKED USERPAYING PAYERROR |
| trade_state_desc | string(256) | true | 交易状态描述 |
| bank_type | string(32) | false | 银行类型 |
| attach | string(128) | false | 商户数据包 |
| success_time | string(64) | false | 支付完成时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| payer | object | false | 支付者信息 |
| payer.openid | string(128) | false | 用户标识 |
| amount | object | false | 订单金额 |
| amount.total | integer | false | 总金额，单位为分 |
| amount.payer_total | integer | false | 用户支付金额，单位为分 |
| amount.currency | string(16) | false | 固定返回：CNY |
| amount.payer_currency | string(16) | false | 固定返回：CNY |
| scene_info | object | false | 场景信息 |
| scene_info.device_id | string(32) | true | 商户端设备号 |
| promotion_detail | array[object] | false | 优惠功能 |
| promotion_detail.coupon_id | string(32) | true | 券ID |
| promotion_detail.name | string(64) | true | 优惠名称 |
| promotion_detail.scope | string(32) | false | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string(32) | false | 优惠类型，枚举值：CASH NOCASH |
| promotion_detail.amount | integer | true | 优惠券面额，单位为分 |
| promotion_detail.stock_id | string(32) | false | 活动ID |
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
| 400 | INVALID_REQUEST | 无效请求 | 请根据接口返回的详细信息检查 |
| 400 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 401 | SIGN_ERROR | 签名错误 | 请检查签名参数和方法是否都符合签名算法要求 |
| 403 | RULE_LIMIT | 业务规则限制 | 因业务规则限制请求频率，请查看接口返回的详细信息 |
| 403 | TRADE_ERROR | 交易错误 | 因业务原因交易失败，请查看接口返回的详细信息 |
| 404 | ORDER_NOT_EXIST | 订单不存在 | 请检查商户订单号是否下单成功 |
| 429 | FREQUENCY_LIMITED | 频率超限 | 请降低请求接口频率 |
| 500 | SYSTEM_ERROR | 系统错误 | 系统异常，请用相同参数重新调用 |

## 直连支付-关闭订单

### 接口说明

请求方式：【POST】
`/v3/pay/transactions/out-trade-no/{out_trade_no}/close`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| out_trade_no | string(32) | true | 商户订单号 |

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| mchid | string(32) | true | 商户号 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 无效请求 | 请根据接口返回的详细信息检查 |
| 400 | MCH_NOT_EXISTS | 商户号不存在 | 请检查商户号是否正确 |
| 401 | SIGN_ERROR | 签名错误 | 请检查签名参数和方法是否都符合签名算法要求 |
| 403 | RULE_LIMIT | 业务规则限制 | 因业务规则限制请求频率，请查看接口返回的详细信息 |
| 403 | TRADE_ERROR | 交易错误 | 因业务原因交易失败，请查看接口返回的详细信息 |
| 429 | FREQUENCY_LIMITED | 频率超限 | 请降低请求接口频率 |
| 500 | SYSTEM_ERROR | 系统错误 | 系统异常，请用相同参数重新调用 |

## 直连支付-支付结果通知

### 接口说明

请求方式：【POST】
`/v1/webhooks/wechat-pay/notify`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| appid | string(32) | true | 公众账号ID |
| mchid | string(32) | true | 商户号 |
| out_trade_no | string(32) | true | 商户订单号 |
| transaction_id | string(32) | true | 微信支付订单号 |
| trade_type | string(16) | true | 交易类型，枚举值：JSAPI NATIVE APP MICROPAY MWEB FACEPAY |
| trade_state | string(32) | true | 交易状态，枚举值：SUCCESS REFUND NOTPAY CLOSED REVOKED USERPAYING PAYERROR |
| trade_state_desc | string(256) | true | 交易状态描述 |
| bank_type | string(32) | true | 银行类型 |
| attach | string(128) | false | 商户数据包 |
| success_time | string(64) | true | 支付完成时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| payer | object | true | 支付者信息 |
| payer.openid | string(128) | true | 用户标识 |
| amount | object | true | 订单金额 |
| amount.total | integer | true | 总金额，单位为分 |
| amount.payer_total | integer | true | 用户支付金额，单位为分 |
| amount.currency | string(16) | true | 固定返回：CNY |
| amount.payer_currency | string(16) | true | 固定返回：CNY |
| scene_info | object | false | 场景信息 |
| scene_info.device_id | string(32) | true | 商户端设备号 |
| promotion_detail | array[object] | false | 优惠功能 |
| promotion_detail.coupon_id | string(32) | true | 券ID |
| promotion_detail.name | string(32) | false | 优惠名称 |
| promotion_detail.scope | string(32) | false | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string(32) | false | 优惠类型，枚举值：CASH NOCASH |
| promotion_detail.amount | integer | true | 优惠券面额，单位为分 |
| promotion_detail.stock_id | string(32) | false | 活动ID |
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

## 直连支付-申请退款

### 接口说明

请求方式：【POST】
`/v3/refund/domestic/refunds`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| transaction_id | string(32) | false | 微信支付订单号，transaction_id 和 out_trade_no 必须二选一 |
| out_trade_no | string(32) | false | 商户订单号，transaction_id 和 out_trade_no 必须二选一 |
| out_refund_no | string(64) | true | 商户退款单号 |
| reason | string(80) | false | 退款原因 |
| notify_url | string(256) | false | 退款结果回调url |
| funds_account | string | false | 退款资金来源，枚举值：AVAILABLE UNSETTLED |
| amount | object | true | 金额信息 |
| amount.refund | integer | true | 退款金额，单位为分 |
| amount.total | integer | true | 原订单金额，单位为分 |
| amount.currency | string(16) | true | 退款币种，固定传：CNY |
| amount.from | array[object] | false | 退款出资账户及金额 |
| amount.from.account | string | true | 出资账户类型，枚举值：AVAILABLE UNAVAILABLE |
| amount.from.amount | integer | true | 出资金额，单位为分 |
| goods_detail | array[object] | false | 退款商品 |
| goods_detail.merchant_goods_id | string(32) | true | 商户侧商品编码 |
| goods_detail.wechatpay_goods_id | string(32) | false | 微信侧商品编码 |
| goods_detail.goods_name | string(256) | false | 商品名称 |
| goods_detail.unit_price | integer | true | 商品单价，单位为分 |
| goods_detail.refund_amount | integer | true | 商品退款金额，单位为分 |
| goods_detail.refund_quantity | integer | true | 商品退货数量 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信支付退款单号 |
| out_refund_no | string(64) | true | 商户退款单号 |
| transaction_id | string(32) | true | 微信支付订单号 |
| out_trade_no | string(32) | true | 商户订单号 |
| channel | string | true | 退款渠道，枚举值：ORIGINAL BALANCE OTHER_BALANCE OTHER_BANKCARD |
| user_received_account | string(64) | true | 退款入账账户 |
| success_time | string(64) | false | 退款成功时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| create_time | string(64) | true | 退款创建时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| status | string | true | 退款状态，枚举值：SUCCESS CLOSED PROCESSING ABNORMAL |
| funds_account | string | true | 资金账户，枚举值：UNSETTLED AVAILABLE UNAVAILABLE OPERATION BASIC ECNY_BASIC |
| amount | object | true | 金额信息 |
| amount.total | integer | true | 订单金额，单位为分 |
| amount.refund | integer | true | 退款金额，单位为分 |
| amount.from | array[object] | false | 退款出资账户及金额 |
| amount.from.account | string | true | 出资账户类型，枚举值：AVAILABLE UNAVAILABLE |
| amount.from.amount | integer | true | 出资金额，单位为分 |
| amount.payer_total | integer | true | 用户实际支付金额，单位为分 |
| amount.payer_refund | integer | true | 用户退款金额，单位为分 |
| amount.settlement_refund | integer | true | 应结退款金额，单位为分 |
| amount.settlement_total | integer | true | 应结订单金额，单位为分 |
| amount.discount_refund | integer | true | 优惠退款金额，单位为分 |
| amount.currency | string(16) | true | 固定返回：CNY |
| amount.refund_fee | integer | false | 手续费退款金额，单位为分 |
| promotion_detail | array[object] | false | 优惠退款详情 |
| promotion_detail.promotion_id | string(32) | true | 券ID |
| promotion_detail.scope | string | true | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string | true | 优惠类型，枚举值：CASH NOCASH |
| promotion_detail.amount | integer | true | 代金券面额，单位为分 |
| promotion_detail.refund_amount | integer | true | 优惠退款金额，单位为分 |
| promotion_detail.goods_detail | array[object] | false | 退款商品 |
| promotion_detail.goods_detail.merchant_goods_id | string(32) | true | 商户侧商品编码 |
| promotion_detail.goods_detail.wechatpay_goods_id | string(32) | false | 微信侧商品编码 |
| promotion_detail.goods_detail.goods_name | string(256) | false | 商品名称 |
| promotion_detail.goods_detail.unit_price | integer | true | 商品单价，单位为分 |
| promotion_detail.goods_detail.refund_amount | integer | true | 商品退款金额，单位为分 |
| promotion_detail.goods_detail.refund_quantity | integer | true | 商品退货数量 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求参数符合参数格式，但不符合业务规则 | 请根据具体错误提示处理 |
| 401 | SIGN_ERROR | 签名错误 | 请检查签名参数和方法是否都符合签名算法要求 |
| 403 | NOT_ENOUGH | 余额不足 | 商户账户余额不足 |
| 403 | USER_ACCOUNT_ABNORMAL | 退款请求失败 | 商户可自行处理退款 |
| 404 | MCH_NOT_EXISTS | MCHID不存在 | 请检查商户号是否正确 |
| 404 | RESOURCE_NOT_EXISTS | 订单号不存在 | 请检查订单号是否正确且是否已支付 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 该笔退款为受理中，请调用查单接口确认或降低频率原单重试 |
| 500 | SYSTEM_ERROR | 系统超时 | 请不要更换商户退款单号，请使用相同参数再次调用API |

## 直连支付-查询单笔退款

### 接口说明

请求方式：【GET】
`/v3/refund/domestic/refunds/{out_refund_no}`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| out_refund_no | string(64) | true | 商户退款单号 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信支付退款单号 |
| out_refund_no | string(64) | true | 商户退款单号 |
| transaction_id | string(32) | true | 微信支付订单号 |
| out_trade_no | string(32) | true | 商户订单号 |
| channel | string | true | 退款渠道，枚举值：ORIGINAL BALANCE OTHER_BALANCE OTHER_BANKCARD |
| user_received_account | string(64) | true | 退款入账账户 |
| success_time | string(64) | false | 退款成功时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| create_time | string(64) | true | 退款创建时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| status | string | true | 退款状态，枚举值：SUCCESS CLOSED PROCESSING ABNORMAL |
| funds_account | string | true | 资金账户，枚举值：UNSETTLED AVAILABLE UNAVAILABLE OPERATION BASIC ECNY_BASIC |
| amount | object | true | 金额信息 |
| amount.total | integer | true | 订单金额，单位为分 |
| amount.refund | integer | true | 退款金额，单位为分 |
| amount.from | array[object] | false | 退款出资账户及金额 |
| amount.from.account | string | true | 出资账户类型，枚举值：AVAILABLE UNAVAILABLE |
| amount.from.amount | integer | true | 出资金额，单位为分 |
| amount.payer_total | integer | true | 用户实际支付金额，单位为分 |
| amount.payer_refund | integer | true | 用户退款金额，单位为分 |
| amount.settlement_refund | integer | true | 应结退款金额，单位为分 |
| amount.settlement_total | integer | true | 应结订单金额，单位为分 |
| amount.discount_refund | integer | true | 优惠退款金额，单位为分 |
| amount.currency | string(16) | true | 固定返回：CNY |
| amount.refund_fee | integer | false | 手续费退款金额，单位为分 |
| promotion_detail | array[object] | false | 优惠退款详情 |
| promotion_detail.promotion_id | string(32) | true | 券ID |
| promotion_detail.scope | string | true | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string | true | 优惠类型，枚举值：CASH NOCASH |
| promotion_detail.amount | integer | true | 代金券面额，单位为分 |
| promotion_detail.refund_amount | integer | true | 优惠退款金额，单位为分 |
| promotion_detail.goods_detail | array[object] | false | 退款商品 |
| promotion_detail.goods_detail.merchant_goods_id | string(32) | true | 商户侧商品编码 |
| promotion_detail.goods_detail.wechatpay_goods_id | string(32) | false | 微信侧商品编码 |
| promotion_detail.goods_detail.goods_name | string(256) | false | 商品名称 |
| promotion_detail.goods_detail.unit_price | integer | true | 商品单价，单位为分 |
| promotion_detail.goods_detail.refund_amount | integer | true | 商品退款金额，单位为分 |
| promotion_detail.goods_detail.refund_quantity | integer | true | 商品退货数量 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | HTTP请求不符合微信支付 APIv3 接口规则 | 请参阅接口规则 |
| 401 | SIGN_ERROR | 签名错误 | 请检查签名参数和方法是否都符合签名算法要求 |
| 404 | MCH_NOT_EXISTS | MCHID不存在 | 请检查商户号是否正确 |
| 404 | RESOURCE_NOT_EXISTS | 退款单不存在 | 请检查退款单号是否有误 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 该笔退款为受理中，请调用查单接口确认或降低频率原单重试 |
| 500 | SYSTEM_ERROR | 系统超时等 | 请不要更换商户退款单号，请使用相同参数再次调用API |

## 直连支付-发起异常退款

### 接口说明

请求方式：【POST】
`/v3/refund/domestic/refunds/{refund_id}/apply-abnormal-refund`

### Path 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信支付退款单号 |

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| out_refund_no | string(64) | true | 商户退款单号 |
| type | string | true | 异常退款处理方式，枚举值：USER_BANK_CARD MERCHANT_BANK_CARD |
| bank_type | string(16) | false | 开户银行 |
| bank_account | string(1024) | false | 收款银行卡号 |
| real_name | string(1024) | false | 收款用户姓名 |

### 应答参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| refund_id | string(32) | true | 微信支付退款单号 |
| out_refund_no | string(64) | true | 商户退款单号 |
| transaction_id | string(32) | true | 微信支付订单号 |
| out_trade_no | string(32) | true | 商户订单号 |
| channel | string | true | 退款渠道，枚举值：ORIGINAL BALANCE OTHER_BALANCE OTHER_BANKCARD |
| user_received_account | string(64) | true | 退款入账账户 |
| success_time | string(64) | false | 退款成功时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| create_time | string(64) | true | 退款创建时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| status | string | true | 退款状态，枚举值：SUCCESS CLOSED PROCESSING ABNORMAL |
| funds_account | string | true | 资金账户，枚举值：UNSETTLED AVAILABLE UNAVAILABLE OPERATION BASIC ECNY_BASIC |
| amount | object | true | 金额信息 |
| amount.total | integer | true | 订单金额，单位为分 |
| amount.refund | integer | true | 退款金额，单位为分 |
| amount.from | array[object] | false | 退款出资账户及金额 |
| amount.from.account | string | true | 出资账户类型，枚举值：AVAILABLE UNAVAILABLE |
| amount.from.amount | integer | true | 出资金额，单位为分 |
| amount.payer_total | integer | true | 用户实际支付金额，单位为分 |
| amount.payer_refund | integer | true | 用户退款金额，单位为分 |
| amount.settlement_refund | integer | true | 应结退款金额，单位为分 |
| amount.settlement_total | integer | true | 应结订单金额，单位为分 |
| amount.discount_refund | integer | true | 优惠退款金额，单位为分 |
| amount.currency | string(16) | true | 固定返回：CNY |
| amount.refund_fee | integer | false | 手续费退款金额，单位为分 |
| promotion_detail | array[object] | false | 优惠退款详情 |
| promotion_detail.promotion_id | string(32) | true | 券ID |
| promotion_detail.scope | string | true | 优惠范围，枚举值：GLOBAL SINGLE |
| promotion_detail.type | string | true | 优惠类型，枚举值：CASH NOCASH |
| promotion_detail.amount | integer | true | 代金券面额，单位为分 |
| promotion_detail.refund_amount | integer | true | 优惠退款金额，单位为分 |
| promotion_detail.goods_detail | array[object] | false | 退款商品 |
| promotion_detail.goods_detail.merchant_goods_id | string(32) | true | 商户侧商品编码 |
| promotion_detail.goods_detail.wechatpay_goods_id | string(32) | false | 微信侧商品编码 |
| promotion_detail.goods_detail.goods_name | string(256) | false | 商品名称 |
| promotion_detail.goods_detail.unit_price | integer | true | 商品单价，单位为分 |
| promotion_detail.goods_detail.refund_amount | integer | true | 商品退款金额，单位为分 |
| promotion_detail.goods_detail.refund_quantity | integer | true | 商品退货数量 |

### 错误码

| 状态码 | 错误码 | 描述 | 解决方案 |
| --- | --- | --- | --- |
| 400 | PARAM_ERROR | 参数错误 | 请根据错误提示正确传入参数 |
| 400 | INVALID_REQUEST | 请求参数符合参数格式，但不符合业务规则 | 请根据具体错误提示处理 |
| 401 | SIGN_ERROR | 签名错误 | 请检查签名参数和方法是否都符合签名算法要求 |
| 404 | RESOURCE_NOT_EXISTS | 退款单不存在 | 请检查退款单号是否有误 |
| 429 | FREQUENCY_LIMITED | 频率限制 | 该笔退款为受理中，请调用查单接口确认或降低频率原单重试 |
| 500 | SYSTEM_ERROR | 系统超时等 | 请不要更换商户退款单号，请使用相同参数再次调用API |

## 直连支付-退款结果通知

### 接口说明

请求方式：【POST】
`/v1/webhooks/wechat-pay/refund-notify`

### Body 参数

| 参数名 | 类型 | 必填 | 描述 |
| --- | --- | --- | --- |
| mchid | string(32) | true | 商户号 |
| out_trade_no | string(32) | true | 商户订单号 |
| transaction_id | string(32) | true | 微信支付订单号 |
| out_refund_no | string(64) | true | 商户退款单号 |
| refund_id | string(32) | true | 微信支付退款单号 |
| refund_status | string(32) | true | 退款状态，枚举值：SUCCESS CLOSED PROCESSING ABNORMAL |
| success_time | string(64) | false | 退款成功时间，遵循 RFC3339 格式：yyyy-MM-DDTHH:mm:ss+TIMEZONE |
| user_received_account | string(64) | true | 退款入账账户 |
| amount | object | true | 金额信息 |
| amount.total | integer | true | 原订单金额，单位为分 |
| amount.refund | integer | true | 退款金额，单位为分 |
| amount.payer_total | integer | true | 用户实际支付金额，单位为分 |
| amount.payer_refund | integer | true | 用户退款金额，单位为分 |
