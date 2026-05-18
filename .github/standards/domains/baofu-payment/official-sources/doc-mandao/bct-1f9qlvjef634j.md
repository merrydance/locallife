---
title: 统一下单交易创建
slug: bct-1f9qlvjef634j
source_url: https://doc.mandao.com/docs/bct/bct-1f9qlvjef634j
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 8cff5e06e1750a4b029a92882c46f207df3191b24a949300242c2ac6eb7a5d6e
doc_version: 1734950003
---

# 统一下单交易创建
# 接口说明

| 接口名称 | unified_order |
| --- | --- |
| 是否幂等 | 是 |
| 接口模式 | 直连 |
| 异步通知 | 是 |

# 应用场景

除付款码支付（被扫）场景外，商户系统需先调用该接口在宝付系统生成预支付交易单，返回成功结果之后商户侧按不同场景使用Native、JSAPI、APP等方式调起支付。

# 注意事项

- 下单成功后，请依照传入的支付方式，获取接口返回的渠道返回参数扩展字段，从扩展字段中获得不同场景调起支付的业务参数。
- 下单指定产品类型为SHARING且订单类型订单类型为7,支付成功后，商户可通过确认分账接口进行订单分账。
- 请求参数支付方式属性扩展字段需要传入的支付属性，请依照传入的支付方式进行传入，详见附录：【[支付属性](https://doc.mandao.com/docs/bct/bct-1f9qrefjkin2b "支付属性")】。
- 超过支付订单有效时间还未支付成功订单，宝付支付系统会发起关单，并将关单结果通过请求商户侧服务端通知地址告知，或商户发起支付订单查询结果。

# 接口参数

## 请求参数：

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 交易商户号 | merId | 是 | S(16) |  | 宝付支付分配的商户号 |
| 交易终端号 | terId | 是 | S(16) |  | 宝付支付分配的终端号 |
| 交易商户订单号 | outTradeNo | 是 | S(32) | 20210315155012 | 商户系统内部订单号，同一个商户号下唯一 |
| 用户实际付款金额 | txnAmt | 是 | I | 100 | 交易金额，单位：分，如：1元则传入100 |
| 交易时间 | txnTime | 是 | T | 20210315155012 | 订单交易时间 |
| 订单总金额 | totalAmt | 是 | I | 100 | 如包含营销信息，则订单总金额=用户实际付款金额+营销总金额，反之订单总金额=用户实际付款金额 |
| 订单有效时间 | timeExpire | 否 | I | 72460 | 订单支付的有效时间，单位：分钟，不传此参数则宝付支付默认有效时间30分钟，允许最大时效7天 |
| 产品类型 | prodType | 是 | E | SHARING | 详见附录：产品类型 |
| 订单类型 | orderType | 是 | S | 7 | 宝财通2.0模式必传。传值:7 |
| 支付方式 | payCode | 是 | E | WECHAT_JSAPI | 详见附录：【[支付方式](https://doc.mandao.com/docs/bct/bct-1f9qrdro3gtv1)】 |
| 支付方式属性 | payExtend | 是 | C | 微信公众号为例：{“sub_openid”:”1231231231”,”sub_appid”:”1231231123”,”body”:”特价手机”} | 根据传入的支付方式选择相应的支付属性。 / 详见附录：【[支付属性](https://doc.mandao.com/docs/bct/bct-1f9qrefjkin2b)】 |
| 聚合交易商户号 | subMchId | 否 | S(64) |  | 微信/支付宝必传，在微信/支付宝报备的二级商户号 |
| 服务端通知地址 | notifyUrl | 否 | S(128) | [https://www.example.com/return_url](https://www.example.com/return_url) | 付款成功后请求商户侧服务端地址 |
| 页面端跳转地址 | pageUrl | 否 | S(128) | [https://www.example.com/caallback_url](https://www.example.com/caallback_url) | 支付完成后跳转的地址：必须是https协议 |
| 禁止贷记卡支付 | forbidCredit | 否 | S(1) | 0 | 1：禁止0：不禁止不传默认为0 |
| 附加字段 | attach | 否 | S(128) |  | 预留字段 |
| 请求方保留域 | reqReserved | 否 | S(128) |  | 预留字段 |
| [营销信息](#mktInfo) | mktInfo | 否 | S | {“mktAmt”:100,”mktMerId”:”100000”} | JSON格式，目前仅支持交易商户承担营销金额 / [无营销金额时该字段不上送] / 详见：[营销信息：mktInfo] |
| [风控信息](#riskInfo) | riskInfo | 否 | S | {“clientIp”:”XXX”,”locationPoint”:”XXX”} | 微信/支付宝必传，JSON格式 |

## 营销信息：mktInfo

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 商户号 | mktMerId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 营销金额 | mktAmt | 是 | I | 100 | 营销金额，单位：分，如：1元则传入100 |

### 风控信息：riskInfo

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 用户ip地址 | clientIp | 是 | S(64) | 100000 | 付款用户ip地址 |
| 交易商户终端经纬度 | locationPoint | 否 | S(128) | 100,100 | 包含经度和纬度，英文逗号分隔 |

## 返回参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 商户订单号 | outTradeNo | 是 | S(64) | 20210315155012 | 商户系统内部订单号，同一个商户号下唯一 |
| 订单状态 | txnState | 否 | E | WAIT_PAYING | 订单状态，详见附录 |
| 宝付交易号 | tradeNo | 否 | S(32) | 12312312312 | 与商户订单号对应的宝付侧唯一交易号 |
| 请求渠道订单号 | reqChlNo | 否 | S(64) |  | 宝付请求渠道订单号 |
| 支付方式 | payCode | 是 | E |  | 原样返回 |
| 渠道返回参数 | chlRetParam | 否 | C |  | 根据不同的支付方式返回相应的业务参数，作为商户侧唤起支付，详见附录：【[统一下单渠道返回参数](https://doc.mandao.com/docs/bct/bct-1f9qrf159lni6)】 |
| 业务结果 | resultCode | 是 | S(16) | SUCCESS | 业务处理结果 |
| 错误代码 | errCode | 否 | S(32) |  | 当业务结果FAIL时，返回错误代码 |
| 错误描述 | errMsg | 否 | S(128) |  | 当业务结果为FAIL时，返回错误描述 |

文档更新时间: 2024-12-23 10:33   作者：超级管理员
