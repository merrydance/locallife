---
title: 确认分账
slug: bct-1f9qlvu1em0tb
source_url: https://doc.mandao.com/docs/bct/bct-1f9qlvu1em0tb
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 1282f6b06569a4f4ddac853e6c756b1fed4ab27b29b9470a4b187e3c60aa3e2c
doc_version: 1768269833
---

# 确认分账
# 接口说明

| 接口名称 | share_after_pay |
| --- | --- |
| 是否幂等 | 是 |
| 接口模式 | 直连 |
| 异步通知 | 是 |

# 应用场景

在支付接口中产品类型prodType传入SHARING且订单类型为7，当支付完成后可调用该接口进行后续分账业务。

# 注意事项

- 同一笔支付订单支持多次确认分账，多次分账需要传递相同的原支付单商户订单号和不同的分账订单号，多次分账总金额不能超过原支付订单总金额。
- 针对同一笔支付订单，确认分账接口不支持和申请退款接口同时发起。
- 确认分账接口同步返回结果仅表示业务受理结果，当resultCode返回SUCCESS，状态为PROCESSING表示分账受理成功，未返回分账状态时，请调用查询接口，以查询返回结果为准。
- 分账结果通过请求商户侧服务端通知地址告知，或商户发起分账查询分账结果。  
  注：订单支付成功后必须在365天内完成分账，过期无法分账，需退款

# 接口参数

## 请求参数：

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 交易商户号 | merId | 是 | S(16) |  |  |
| 交易终端号 | terId | 是 | S(16) |  |  |
| 原支付订单宝付订单号 | originTradeNo | 否 | S(32) | 12312312312 | 与商户订单号对应的宝付侧唯一订单号，推荐传入此值。originTradeNo和originOutTradeNo必须二选一上送 |
| 原支付订单商户订单号 | originOutTradeNo | 否 | S(64) | 20210315155012 | 商户系统内部订单号，同一个商户号下唯一。originTradeNo和originOutTradeNo必须二选一上送 |
| 交易时间 | txnTime | 是 | T | 20210315155012 | 发起分账交易时间 |
| 分账订单号 | outTradeNo | 是 | S(50) | 100000 | 商户分账订单号，查询分账订单 |
| 分账结果通知地址 | notifyUrl | 否 | S(128) | [http://www.example.com/notify](http://www.example.com/notify) | 宝付分账完成通知商户侧接收地址，不传入此值则不通知 |
| 分账信息 | sharingDetails | 是 | C | “sharingDetails”:[{“sharingAmt”:100,”sharingMerId”:”100000”},{“sharingAmt”:200,”sharingMerId”:”100001”}] | JSON数组 |
| -商户号 | sharingMerId | 是 | S(64) | 100000 | 宝付支付分配的商户号 |
| -分账金额 | sharingAmt | 是 | I | 100 | 分账金额，单位：分，如：1元则传入100 |

## 返回参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 |  |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 |  |
| 商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 业务结果 | resultCode | 是 | S(16) | SUCCESS | 业务处理结果 |
| 错误代码 | errCode | 否 | S(32) |  | 当业务结果FAIL时，返回错误代码 |
| 错误描述 | errMsg | 否 | S(128) |  | 当业务结果为FAIL时，返回错误描述 |
| 宝付订单号 | tradeNo | 否 | S(32) | 12312312312 | 与商户订单号对应的宝付侧唯一订单号 |
| 订单状态 | txnState | 是 | E | REFUND | 详见附录订单状态 |
| 完成时间 | finishTime | 否 | T | 20210315155012 | 订单状态为成功时才有值 |
| 分账成功金额 | succAmt |  | I |  | 单位：分 |
| 清算日期 | clearingDate | 否 | D |  |  |

文档更新时间: 2026-01-13 02:03   作者：超级管理员
