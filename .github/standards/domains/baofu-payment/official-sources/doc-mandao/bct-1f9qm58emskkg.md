---
title: 分账结果通知
slug: bct-1f9qm58emskkg
source_url: https://doc.mandao.com/docs/bct/bct-1f9qm58emskkg
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: e3763ce4fc9393f6d0feec49cbf6ec4a8d8d90aa328675c1dc6fc4f6854f82fc
doc_version: 1704426010
---

# 分账结果通知
# 接口说明

- 分账完成后，宝付支付系统会把结果通过请求原支付单传入的服务端通知地址，通知到商户侧。商户接收到通知系统处理完成后，按约定返回OK。

# 通知参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 宝付交易号 | tradeNo | 否 | S(32) | 12312312312 | 与商户订单号对应的宝付侧唯一订单号 |
| 商户订单号 | outTradeNo | 否 | S(32) | 20210315155012 | 商户系统内部订单号，同一个商户号下唯一 |
| 订单状态 | txnState | 是 | E | REFUND | 详见附录订单状态 |
| 完成时间 | finishTime | 否 | T | 20210315155012 | 订单状态为成功时才有值格式为yyyyMMddHHmmss，如：2021年3月15日15点50分12秒表示为：20210315155012 |
| 成功金额 | succAmt | 否 | I | 100 | 单位：分，订单状态为成功时才有值 |
| 业务结果 | resultCode | 是 | S(16) | SUCCESS | SUCCESS：成功 |
| 清算日期 | clearingDate | 否 | D | 20210501 |  |

文档更新时间: 2024-01-05 03:40   作者：超级管理员
