---
title: 签约订单查询
slug: bct-1fao07u6loaqf
source_url: https://doc.mandao.com/docs/bct/bct-1fao07u6loaqf
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: b9dd83ff99b1f52f6ff157c4dc203cc51a0b6f74502ca7080d79b3733fcb9cf9
doc_version: 1705457171
---

# 签约订单查询
# 接口说明

| 接口名称 | contract_order_query |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 否 |

商户通过订单查询接口主动查询签约、解约订单状态。

注意事项：

> - 宝付订单号、商户订单号至少选择一个传入，推荐传入宝付订单号，如果同时传入，系统默认使用宝付订单号。
> - 调用签约接口、解约接口，系统返回交易状态异常时，请调用查询接口确认订单状态。
> - 查询接口返回状态异常时，请稍后再次发起查询。

# 接口参数

> - 请求：

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 宝付订单号 | tradeNo | 否 | S(32) | 12312312312 | 与商户订单号对应的宝付侧唯一订单号，推荐传入此值 |
| 商户订单号 | outTradeNo | 否 | S(32) | 20210315155012 | 商户系统内部订单号，同一个商户号下唯一 |

> - 返回：

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 宝付订单号 | tradeNo | 否 | S(32) | 12312312312 | 与商户订单号对应的宝付侧唯一订单号，推荐传入此值 |
| 商户订单号 | outTradeNo | 否 | S(32) | 20210315155012 | 商户系统内部订单号，同一个商户号下唯一 |
| 订单状态 | orderState | 否 | E | SUCCESS | 详见附录订单状态 / [订单状态-云闪付签约订单状态](https://doc.mandao.com/docs/pay-doc/pay-doc-1e1p6k1c3vgpq) |
| 完成时间 | finishTime | 否 | T | 20210315155012 | 订单状态为成功时才有值 |
| 业务结果 | resultCode | 是 | S(16) | SUCCESS | SUCCESS：成功 FAIL：失败 |
| 错误代码 | errCode | 否 | S(32) | SUCCESS | 当业务结果FAIL时，返回错误代码 |
| 错误描述 | errMsg | 否 | S(128) | SUCCESS | 当业务结果FAIL时，返回错误代码 |
| 渠道返回参数 | chlRetParam | 否 | C | SUCCESS | 根据不同的支付方式返回相应的业务参数 / 详见：[异步通知渠道返回参数](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f038bk79pg68) |

文档更新时间: 2024-01-17 02:06 作者：超级管理员
