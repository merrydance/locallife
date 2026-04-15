# 微信支付收付通下单组官方契约基线

## 1. 范围

本文件只覆盖平台收付通下单组，不覆盖直连支付。

当前纳入统一本地契约的官方接口：

- 普通支付-小程序下单：POST /v3/pay/partner/transactions/jsapi
- 普通支付-查询订单：GET /v3/pay/partner/transactions/id/{transaction_id}
- 普通支付-查询订单：GET /v3/pay/partner/transactions/out-trade-no/{out_trade_no}
- 普通支付-关闭订单：POST /v3/pay/partner/transactions/out-trade-no/{out_trade_no}/close
- 普通支付-小程序调起支付：wx.requestPayment
- 合单支付-小程序下单：POST /v3/combine-transactions/jsapi
- 合单支付-查询订单：GET /v3/combine-transactions/out-trade-no/{combine_out_trade_no}
- 合单支付-关闭订单：POST /v3/combine-transactions/out-trade-no/{combine_out_trade_no}/close

当前仓库主实现锚点：

- locallife/wechat/ecommerce.go
- locallife/logic/payment_order_service.go
- locallife/logic/combined_payment_service.go
- locallife/api/payment_order.go

## 2. 不可降级规则

- 必填、条件必填、字段类型、字段长度、枚举、状态、错误码必须逐接口核对，不允许凭旧实现猜测。
- 未接入字段必须在本地契约中明确标注“未接入”，不能因为代码暂未使用就从契约基线消失。
- 所有微信错误必须落结构化日志，至少带 request_id、操作名、主业务标识和微信错误码。
- 前端不得直接消费微信原始错误；必须返回稳定的业务语义。
- 对查询、关单等高风险读写路径，遇到上游未知态或漂移态，不得乐观报成功。

## 3. 官方文档

- 普通支付-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012088031.md
- 普通支付-小程序下单：https://pay.weixin.qq.com/doc/v3/partner/4012714911.md
- 普通支付-按微信订单号查询：https://pay.weixin.qq.com/doc/v3/partner/4012760565.md
- 普通支付-按商户订单号查询：https://pay.weixin.qq.com/doc/v3/partner/4013080235.md
- 普通支付-关闭订单：https://pay.weixin.qq.com/doc/v3/partner/4012760574.md
- 普通支付-小程序调起支付：https://pay.weixin.qq.com/doc/v3/partner/4012090181.md
- 合单支付-开发指引：https://pay.weixin.qq.com/doc/v3/partner/4012089542.md
- 合单支付-小程序下单：https://pay.weixin.qq.com/doc/v3/partner/4012760633.md
- 合单支付-小程序调起支付：https://pay.weixin.qq.com/doc/v3/partner/4012091236.md
- 合单支付-查询订单：https://pay.weixin.qq.com/doc/v3/partner/4012761049.md
- 合单支付-关闭订单：https://pay.weixin.qq.com/doc/v3/partner/4012761093.md

## 4. 普通支付-小程序下单

### 4.1 请求字段

| 字段 | 类型 | 必填/条件必填 | 说明 | LocalLife 当前状态 |
| :-- | :-- | :-- | :-- | :-- |
| sp_appid | string(32) | 必填 | 服务商 AppID | 已接入，来自服务商配置 |
| sp_mchid | string(32) | 必填 | 服务商商户号 | 已接入，来自服务商配置 |
| sub_appid | string(32) | 条件必填 | 传 sub_openid 时必填，且需与 sub_mchid 绑定 | 官方支持，本项目未接入（单一 appid 约束） |
| sub_mchid | string(32) | 必填 | 二级商户号 | 已接入 |
| description | string(127) | 必填 | 商品描述 | 已接入 |
| out_trade_no | string(32) | 必填 | 商户订单号，同商户号下唯一 | 已接入 |
| time_expire | string(RFC3339) | 选填 | 支付结束时间 | 已接入 |
| attach | string(128) | 选填 | 附加数据，仅支付完成后在查询/通知中原样返回 | 已接入 |
| notify_url | string(255) | 必填 | 支付通知地址，不可带参数 | 已接入 |
| goods_tag | string(32) | 选填 | 订单优惠标记 | 已接入 |
| settle_info.profit_sharing | boolean | 选填 | 是否指定分账 | 已接入 |
| settle_info.subsidy_amount | integer | 条件生效 | profit_sharing=true 时才生效，单笔最高 5000 元 | 未接入 |
| support_fapiao | boolean | 选填 | 电子发票入口开放标识 | 已接入 |
| amount.total | integer | 必填 | 订单总金额，单位分 | 已接入 |
| amount.currency | string(16) | 选填 | 货币类型，境内默认 CNY | 已接入，默认 CNY |
| payer.sp_openid | string(128) | 二选一 | 用户在服务商 AppID 下的唯一标识 | 已接入 |
| payer.sub_openid | string(128) | 二选一 | 用户在子商户 AppID 下的唯一标识 | 官方支持，本项目未接入（单一 appid 约束） |
| detail | object | 选填 | 商品详情，压缩后总长度不超过 6144 字节 | 未接入 |
| detail.cost_price | integer | 选填 | 订单原价 | 未接入 |
| detail.invoice_id | string(32) | 选填 | 商品小票 ID | 未接入 |
| detail.goods_detail | array | 选填 | 单品列表，至少 1 条 | 未接入 |
| detail.goods_detail.merchant_goods_id | string(32) | 必填 | 商户侧商品编码 | 未接入 |
| detail.goods_detail.wechatpay_goods_id | string(32) | 选填 | 微信支付商品编码 | 未接入 |
| detail.goods_detail.goods_name | string(256) | 选填 | 商品名称 | 未接入 |
| detail.goods_detail.quantity | integer | 必填 | 商品数量 | 未接入 |
| detail.goods_detail.unit_price | integer | 必填 | 商品单价 | 未接入 |
| scene_info | object | 选填 | 场景信息 | 已接入 |
| scene_info.payer_client_ip | string(45) | 条件必填 | 传 scene_info 时必填 | 已接入，并在客户端侧强校验 |
| scene_info.device_id | string(32) | 选填 | 商户端设备号 | 已接入 |
| scene_info.store_info | object | 选填 | 门店信息 | 未接入 |
| scene_info.store_info.id | string(32) | 必填 | 门店编号 | 未接入 |
| scene_info.store_info.name | string(256) | 选填 | 门店名称 | 未接入 |
| scene_info.store_info.area_code | string(32) | 选填 | 地区编码 | 未接入 |
| scene_info.store_info.address | string(512) | 选填 | 门店地址 | 未接入 |

### 4.2 响应字段

| 字段 | 类型 | 必填 | 说明 | LocalLife 当前状态 |
| :-- | :-- | :-- | :-- | :-- |
| prepay_id | string(64) | 必填 | 预支付交易会话标识，有效期 2 小时 | 已接入 |

### 4.3 前端调起支付字段

| 字段 | 类型 | 必填 | 说明 |
| :-- | :-- | :-- | :-- |
| timeStamp | string(32) | 必填 | 秒级时间戳 |
| nonceStr | string(32) | 必填 | 随机字符串 |
| package | string(128) | 必填 | 固定格式 prepay_id=xxx |
| signType | string(32) | 必填 | 仅支持 RSA |
| paySign | string(512) | 必填 | 官方规则为以实际调起支付的小程序 appid 参与签名；本项目当前仅启用单一服务商 appid，因此统一使用唯一配置 appid |

### 4.4 普通支付-查询订单

#### 4.4.1 请求字段

| 字段 | 类型 | 必填 | 说明 | LocalLife 当前状态 |
| :-- | :-- | :-- | :-- | :-- |
| transaction_id | string(32) | 二选一 | 微信支付订单号，path 参数 | 已接入 |
| out_trade_no | string(32) | 二选一 | 商户订单号，path 参数 | 已接入 |
| sp_mchid | string(32) | 必填 | 服务商商户号，query 参数 | 已接入 |
| sub_mchid | string(32) | 必填 | 子商户号，query 参数 | 已接入 |

#### 4.4.2 响应字段

| 字段 | 类型 | 必填/条件必填 | 说明 | LocalLife 当前状态 |
| :-- | :-- | :-- | :-- | :-- |
| sp_appid | string(32) | 必填 | 服务商 AppID | 已接入 |
| sp_mchid | string(32) | 必填 | 服务商商户号 | 已接入 |
| sub_appid | string(32) | 选填 | 子商户 AppID | 已接入 |
| sub_mchid | string(32) | 必填 | 子商户号 | 已接入 |
| out_trade_no | string(32) | 必填 | 商户订单号 | 已接入 |
| transaction_id | string(32) | 条件必填 | 支付成功后返回 | 已接入 |
| trade_type | string(16) | 条件必填 | JSAPI/NATIVE/APP/MICROPAY/MWEB/FACEPAY | 已接入 |
| trade_state | string(32) | 必填 | SUCCESS/REFUND/NOTPAY/CLOSED/REVOKED/USERPAYING/PAYERROR | 已接入 |
| trade_state_desc | string(256) | 必填 | 交易状态描述 | 已接入 |
| bank_type | string(32) | 选填 | 银行类型 | 已接入 |
| attach | string(128) | 选填 | 商户数据包 | 已接入 |
| success_time | string(64) | 选填 | 支付完成时间 | 已接入 |
| payer.sp_openid | string(128) | 选填 | 用户服务商标识 | 已接入 |
| payer.sub_openid | string(128) | 选填 | 用户子商户标识 | 已接入 |
| amount.total | integer | 选填 | 总金额 | 已接入 |
| amount.payer_total | integer | 选填 | 用户支付金额 | 已接入 |
| amount.currency | string(16) | 选填 | 货币类型 | 已接入 |
| amount.payer_currency | string(16) | 选填 | 用户支付币种 | 已接入 |
| scene_info.device_id | string(32) | 选填 | 设备号 | 已接入 |
| promotion_detail | array | 选填 | 优惠功能明细 | 已接入查询结构 |
| promotion_detail.coupon_id | string(32) | 必填 | 券 ID | 已接入查询结构 |
| promotion_detail.name | string(64) | 条件必填 | 优惠名称 | 已接入查询结构 |
| promotion_detail.scope | string(32) | 选填 | GLOBAL/SINGLE | 已接入查询结构 |
| promotion_detail.type | string(32) | 选填 | CASH/NOCASH | 已接入查询结构 |
| promotion_detail.amount | integer | 必填 | 优惠券面额 | 已接入查询结构 |
| promotion_detail.stock_id | string(32) | 选填 | 活动 ID | 已接入查询结构 |
| promotion_detail.wechatpay_contribute | integer | 选填 | 微信出资 | 已接入查询结构 |
| promotion_detail.merchant_contribute | integer | 选填 | 商户出资 | 已接入查询结构 |
| promotion_detail.other_contribute | integer | 选填 | 其他出资 | 已接入查询结构 |
| promotion_detail.currency | string(16) | 选填 | 优惠币种 | 已接入查询结构 |
| promotion_detail.goods_detail | array | 选填 | 单品列表 | 已接入查询结构 |
| promotion_detail.goods_detail.goods_id | string(32) | 必填 | 商品编码 | 已接入查询结构 |
| promotion_detail.goods_detail.quantity | integer | 必填 | 商品数量 | 已接入查询结构 |
| promotion_detail.goods_detail.unit_price | integer | 必填 | 商品单价 | 已接入查询结构 |
| promotion_detail.goods_detail.discount_amount | integer | 必填 | 商品优惠金额 | 已接入查询结构 |
| promotion_detail.goods_detail.goods_remark | string(128) | 选填 | 商品备注 | 已接入查询结构 |

#### 4.4.3 交易状态枚举

| 状态值 | 官方语义 | LocalLife 处理要求 |
| :-- | :-- | :-- |
| SUCCESS | 支付成功 | 可视为成功 |
| REFUND | 转入退款 | 不得视为未支付 |
| NOTPAY | 未支付 | 可视为待支付 |
| CLOSED | 已关闭 | 可视为关闭 |
| REVOKED | 已撤销，仅付款码支付 | 不得映射为成功 |
| USERPAYING | 用户支付中，仅付款码支付 | 不得映射为成功 |
| PAYERROR | 支付失败，仅付款码支付 | 可视为失败 |

### 4.5 普通支付-关闭订单

#### 4.5.1 请求字段

| 字段 | 类型 | 必填 | 说明 | LocalLife 当前状态 |
| :-- | :-- | :-- | :-- | :-- |
| out_trade_no | string(32) | 必填 | 商户订单号，path 参数 | 已接入 |
| sp_mchid | string(32) | 必填 | 服务商商户号 | 已接入 |
| sub_mchid | string(32) | 必填 | 子商户号 | 已接入 |

#### 4.5.2 响应

- 官方为 204 No Content，无响应体。
- 本地不得伪造微信响应字段；只可返回本地关闭结果投影。

## 5. 合单支付-小程序下单

### 5.1 请求字段

| 字段 | 类型 | 必填/条件必填 | 说明 | LocalLife 当前状态 |
| :-- | :-- | :-- | :-- | :-- |
| combine_appid | string(32) | 必填 | 合单发起方 AppID | 已接入 |
| combine_mchid | string(32) | 必填 | 合单发起方商户号 | 已接入 |
| combine_out_trade_no | string(32) | 必填 | 合单总订单号 | 已接入 |
| combine_payer_info.openid | string(128) | 二选一 | 合单发起方 openid | 已接入 |
| combine_payer_info.sub_openid | string(128) | 二选一 | 子商户 openid；传该字段时必须有 sub_appid | 官方支持，本项目未接入（单一 appid 约束） |
| scene_info | object | 选填 | 场景信息 | 已接入 |
| scene_info.payer_client_ip | string(45) | 条件必填 | 传 scene_info 时必填 | 已接入，并在客户端侧强校验 |
| scene_info.device_id | string(16) | 选填 | 终端设备号 | 已接入 |
| sub_orders | array | 必填 | 商品单列表，最多 50 条 | 已接入，并做 50 条上限校验 |
| sub_orders.mchid | string(32) | 必填 | 商品单发起方商户号 | 已接入；为空时回退服务商商户号 |
| sub_orders.sub_mchid | string(32) | 选填 | 特约商户商户号 | 已接入 |
| sub_orders.sub_appid | string(32) | 条件必填 | 使用 sub_openid 时需传 | 官方支持，本项目未接入（单一 appid 约束） |
| sub_orders.out_trade_no | string(32) | 必填 | 商品单商户订单号 | 已接入 |
| sub_orders.amount.total_amount | integer | 必填 | 商品单金额，单位分 | 已接入 |
| sub_orders.amount.currency | string(8) | 必填 | 币种，境内为 CNY | 已接入，固定 CNY |
| sub_orders.attach | string(128) | 必填 | 附加数据，在查询和通知中原样返回 | 已接入，并在客户端侧强校验 |
| sub_orders.description | string(127) | 必填 | 商品描述 | 已接入，并在客户端侧强校验 |
| sub_orders.settle_info.profit_sharing | boolean | 选填 | 是否指定分账 | 已接入 |
| sub_orders.settle_info.subsidy_amount | integer | 条件生效 | profit_sharing=true 时才生效，最高 5000 元 | 未接入 |
| sub_orders.goods_tag | string(32) | 选填 | 订单优惠标记 | 已接入 |
| sub_orders.detail | string | 选填 | 商品详情文本 | 未接入 |
| time_start | string(RFC3339) | 选填 | 支付起始时间 | 已接入 |
| time_expire | string(RFC3339) | 选填 | 支付结束时间 | 已接入 |
| notify_url | string(255) | 选填 | 支付通知地址 | 已接入 |

### 5.2 响应字段

| 字段 | 类型 | 必填 | 说明 | LocalLife 当前状态 |
| :-- | :-- | :-- | :-- | :-- |
| prepay_id | string(64) | 必填 | 预支付交易会话标识，有效期 2 小时 | 已接入 |

## 6. 合单查询订单

### 6.1 请求字段

| 字段 | 类型 | 必填 | 说明 |
| :-- | :-- | :-- | :-- |
| combine_out_trade_no | string(32) | 必填 | 合单商户订单号，path 参数 |

### 6.2 响应字段

| 字段 | 类型 | 必填/条件必填 | 说明 | LocalLife 当前状态 |
| :-- | :-- | :-- | :-- | :-- |
| combine_appid | string(32) | 必填 | 合单发起方 AppID | 已接入 |
| combine_mchid | string(32) | 必填 | 合单发起方商户号 | 已接入 |
| combine_out_trade_no | string(32) | 必填 | 合单总单号 | 已接入 |
| combine_payer_info.openid | string(128) | 选填 | 用户标识 | 已接入 |
| scene_info.device_id | string(32) | 选填 | 设备号 | 已接入 |
| sub_orders | array | 选填 | 商品单列表 | 已接入 |
| sub_orders.mchid | string(32) | 必填 | 商品单商户号 | 已接入 |
| sub_orders.sub_mchid | string(32) | 选填 | 特约商户商户号 | 已接入 |
| sub_orders.sub_appid | string(32) | 选填 | 子商户 AppID | 已接入 |
| sub_orders.sub_openid | string(128) | 选填 | 子商户 openid | 已接入 |
| sub_orders.out_trade_no | string(32) | 必填 | 商品单订单号 | 已接入 |
| sub_orders.transaction_id | string(32) | 选填 | 微信支付订单号 | 已接入 |
| sub_orders.trade_type | string | 选填 | 交易类型，JSAPI/NATIVE/APP/MWEB | 已接入 |
| sub_orders.trade_state | string | 必填 | SUCCESS/REFUND/NOTPAY/CLOSED/PAYERROR | 已接入 |
| sub_orders.bank_type | string(32) | 选填 | 付款银行 | 已接入 |
| sub_orders.attach | string(128) | 选填 | 附加数据 | 已接入 |
| sub_orders.success_time | string | 选填 | 支付完成时间 | 已接入 |
| sub_orders.amount.total_amount | integer | 必填 | 标价金额 | 已接入 |
| sub_orders.amount.payer_amount | integer | 必填 | 用户支付金额 | 已接入 |
| sub_orders.amount.currency | string(16) | 必填 | 标价币种 | 已接入 |
| sub_orders.amount.payer_currency | string(16) | 必填 | 用户支付币种 | 已接入 |
| sub_orders.amount.settlement_rate | integer | 选填 | 结算汇率 | 已接入 |
| sub_orders.promotion_detail | array | 选填 | 优惠明细 | 未接入存储，但查询结构已保留扩展位 |

### 6.3 交易状态枚举

| 状态值 | 官方语义 | LocalLife 聚合语义 |
| :-- | :-- | :-- |
| SUCCESS | 支付成功 | paid |
| REFUND | 转入退款 | refunded |
| NOTPAY | 未支付 | pending |
| CLOSED | 已关闭 | closed |
| PAYERROR | 支付失败 | failed |
| SUCCESS + NOTPAY 混合 | 官方无聚合值 | partial |
| 其他未知组合 | 官方未承诺 | mixed / unknown，禁止当作成功 |

## 7. 合单关闭订单

### 7.1 请求字段

| 字段 | 类型 | 必填/条件必填 | 说明 | LocalLife 当前状态 |
| :-- | :-- | :-- | :-- | :-- |
| combine_out_trade_no | string(32) | 必填 | 合单商户单号，path 参数 | 已接入 |
| combine_appid | string(32) | 必填 | 合单发起方 AppID | 已接入 |
| sub_orders | array | 必填 | 商品单列表，必须与下单时的子单集合完全一致 | 已接入 |
| sub_orders.mchid | string(32) | 必填 | 商品单发起方商户号 | 已接入；为空时回退服务商商户号 |
| sub_orders.out_trade_no | string(32) | 必填 | 商品单订单号 | 已接入，并在客户端侧强校验 |
| sub_orders.sub_mchid | string(32) | 选填 | 特约商户商户号 | 已接入 |
| sub_orders.sub_appid | string(32) | 选填 | 服务商模式下对应子商户 AppID | 已接入 |

### 7.2 响应

- 官方为 204 No Content，无响应体。
- 本地不得伪造微信响应字段；只可返回本地关闭结果投影。

## 8. 小程序调起支付

### 8.1 普通支付与合单支付统一字段

| 字段 | 类型 | 必填 | 说明 |
| :-- | :-- | :-- | :-- |
| timeStamp | string(32) | 必填 | 秒级时间戳 |
| nonceStr | string(32) | 必填 | 随机字符串 |
| package | string(128) | 必填 | prepay_id=xxx |
| signType | string(32) | 必填 | 仅支持 RSA |
| paySign | string(256/512) | 必填 | 按 appid、timeStamp、nonceStr、package 参与签名 |

### 8.2 前端回调语义

- requestPayment:ok 仅表示调起成功返回，不是最终支付真值。
- requestPayment:fail cancel 表示用户取消支付。
- 其他 fail detail message 只能作为前端提示，最终支付状态以后端查询和支付通知为准。

## 9. 关键错误码与本地语义

### 9.1 创建下单

| 错误码 | HTTP | 官方说明 | LocalLife 对前端语义 |
| :-- | :-- | :-- | :-- |
| ORDER_CLOSED | 400 | 订单已关闭 | payment order has expired or been closed, please recreate the payment |
| OUT_TRADE_NO_USED | 403 | 商户订单号重复 | payment order is already being processed, please retry |
| ACCOUNTERROR / ACCOUNT_ERROR | 403 | 账号异常 | current wechat account cannot complete payment, please switch account and retry |
| TRADE_ERROR | 403 | 交易错误 | wechat could not create the payment order, please retry or use another payment method |
| RULELIMIT / RULE_LIMIT / FREQUENCY_LIMITED / RATELIMIT_EXCEEDED | 403/429 | 频率或规则限制 | payment service is temporarily unavailable, please retry later |
| SYSTEMERROR / SYSTEM_ERROR / BANKERROR / BANK_ERROR | 500 | 系统或银行异常 | payment service is temporarily unavailable, please retry later |
| PARAM_ERROR / INVALID_REQUEST | 400 | 参数错误 / 无效请求 | payment service configuration is temporarily unavailable, please retry later |
| APPID_MCHID_NOT_MATCH / OPENID_MISMATCH / MCH_NOT_EXISTS / SIGN_ERROR / NOAUTH / NO_AUTH | 400/401/403/500 | 配置、签名、权限问题 | payment service configuration is temporarily unavailable, please retry later |

### 9.2 查询订单

| 错误码 | HTTP | 官方说明 | LocalLife 对前端语义 |
| :-- | :-- | :-- | :-- |
| ORDERNOTEXIST / ORDER_NOT_EXIST | 404 | 订单不存在 | payment status is still being synchronized, please retry later |
| RULELIMIT / FREQUENCY_LIMITED / SYSTEMERROR / BANKERROR | 429/500 | 频控或系统异常 | payment status query is temporarily unavailable, please retry later |
| PARAM_ERROR / INVALID_REQUEST / SIGN_ERROR / NOAUTH | 400/401/403 | 参数、签名、权限错误 | payment status query configuration is temporarily unavailable, please retry later |

### 9.3 关闭订单

| 错误码 | HTTP | 官方说明 | LocalLife 对前端语义 |
| :-- | :-- | :-- | :-- |
| ORDER_CLOSED | 400 | 订单已关闭 | payment order is already closed |
| USERPAYING | 202 | 用户支付中 | payment is being processed, please retry after confirming the latest status |
| ORDERNOTEXIST / ORDER_NOT_EXIST | 404 | 订单不存在 | payment close status is still being synchronized, please retry later |
| INVALID_REQUEST | 400 | 合单子单信息与下单不一致 | payment close configuration is temporarily unavailable, please retry later |
| RULELIMIT / FREQUENCY_LIMITED / SYSTEMERROR / BANKERROR | 429/500 | 频控或系统异常 | payment close is temporarily unavailable, please retry later |

## 10. 当前实现约束

- 本仓库当前主路径已统一使用平台收付通普通支付或合单支付；直连支付不属于本文件范围。
- 本项目当前只启用单一 appid 的服务商 openid 支付路径；sub_openid 和 sub_appid 属于官方可选能力，但不在当前项目支付主链路内，调用应直接失败而不是静默降级。
- 普通支付和合单支付当前只接入当前业务所需字段子集；未接入字段不能默默丢弃，扩展前必须先更新本文件。
- 收付通客户端必须在失败时记录 request_id、wechat_operation、业务主键和微信错误明细。
- 逻辑层必须把微信错误映射为稳定的业务语义，禁止把微信原始报错直接返回前端。