---
title: 支付结果通知
slug: bct-1f9qm4ujg50cv
source_url: https://doc.mandao.com/docs/bct/bct-1f9qm4ujg50cv
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: ba25349a0d8c5bde79b1dc86dc139e7c0cb9ac1c178f6d8b651aa80160cf6c0f
doc_version: 1727079209
---

# 支付结果通知
# 接口说明

- 支付完成后或订单关闭后，宝付支付系统会把结果通过请求原支付单传入的服务端通知地址，通知到商户侧。商户接收到通知系统处理完成后，按约定返回OK。
- 注意：商户侧收到支付结果通知并且验签通过后，请判断订单状态为SUCCESS或CLOSED进行后续业务处理。注意：商户侧收到支付结果通知并且验签通过后，请判断订单状态为SUCCESS或CLOSED进行后续业务处理。
- 聚合通知频率，最多通知14次  
  0秒/10秒/10秒/10秒/10秒/15秒/1分钟/3分钟/10分钟/30分钟/60分钟/3小时/6小时/6小时

# 通知参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 宝付订单号 | tradeNo | 否 | S(32) | 12312312312 | 与商户订单号对应的宝付侧唯一订单号 |
| 商户订单号 | outTradeNo | 否 | S(32) | 20210315155012 | 商户系统内部订单号，同一个商户号下唯一 |
| 订单状态 | txnState | 否 | E | REFUND | 详见附录订单状态 |
| 完成时间 | finishTime | 否 | T | 20210315155012 | 订单状态为成功时才有值 |
| 成功金额 | succAmt | 否 | I | 100 | 单位：分，订单状态为成功时才有值 |
| 支付手续费 | feeAmt | 否 | I | 100 | 单位：分，订单状态为成功时才有值 |
| 分期手续费 | instFeeAmt | 否 | I | 100 | 单位：分，商户使用分期产品支付时，订单状态为成功时有值 |
| 业务结果 | resultCode | 否 | S(16) | SUCCESS | SUCCESS：成功 FAIL：失败 ，注：关单场景不返回 |
| 错误代码 | errCode | 否 | S(32) | SUCCESS | 当业务结果FAIL时，返回错误代码 |
| 错误描述 | errMsg | 否 | S(128) | SUCCESS | 当业务结果FAIL时，返回错误代码 |
| 请求渠道订单号 | reqChlNo | 否 | S | SUCCESS | 支付成功时返回 |
| 支付方式 | payCode | 是 |  |  |  |
| 渠道返回参数 | chlRetParam | 否 | C |  | 根据不同的支付方式返回相应的业务参数详见：渠道返回参数 |
| 清算日期 | clearingDate | 否 | D | 20210501 |  |

文档更新时间: 2024-09-23 08:13   作者：超级管理员
