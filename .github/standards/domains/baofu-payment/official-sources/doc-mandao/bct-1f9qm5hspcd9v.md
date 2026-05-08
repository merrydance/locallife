---
title: 退款结果通知
slug: bct-1f9qm5hspcd9v
source_url: https://doc.mandao.com/docs/bct/bct-1f9qm5hspcd9v
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 14cd0e4c9928938eee2b3fb5a941ae627827b8c11b53220575e37189b259b1bd
doc_version: 1704434368
---

# 退款结果通知
# 接口说明

- 退款完成后，宝付支付系统会把结果通过请求原支付单传入的服务端通知地址，通知到商户侧。商户接收到通知系统处理完成后，按约定返回OK。

# 通知参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 宝付退款交易号 | tradeNo | 是 | S(32) | 12312312312 | 与商户退款订单号对应的宝付侧唯一退款订单号， |
| 退款订单号 | outTradeNo | 是 | S(50) | 20210315155012 | 商户系统内部退款订单号，同一个商户号下唯一 |
| 订单状态 | refundState | 否 | E | REFUND | 详见附录订单状态 |
| 完成时间 | finishTime | 否 | T | 20210315155012 | 订单状态为成功时才有值 |
| 成功金额 | succAmt | 否 | I | 100 | 单位：分，订单状态为成功时才有值 |
| 业务结果 | resultCode | 是 | S(16) | SUCCESS | SUCCESS：成功 |
| 交易时间 | txnTime | 是 | T | 20210315155012 | 订单交易时间 |
| 错误代码 | errCode | 否 | S(32) | SUCCESS | 当业务结果FAIL时，返回错误代码 |
| 错误描述 | errMsg | 否 | S(128) | SUCCESS | 当业务结果FAIL时，返回错误代码 |

文档更新时间: 2024-01-05 05:59   作者：超级管理员
