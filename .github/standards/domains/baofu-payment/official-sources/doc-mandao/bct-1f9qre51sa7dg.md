---
title: 订单状态
slug: bct-1f9qre51sa7dg
source_url: https://doc.mandao.com/docs/bct/bct-1f9qre51sa7dg
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: af2cf95176458e46d637e292a07094ef12f03bd70e1c4fe0ea2e7030350bc4c9
doc_version: 1711436986
---

# 订单状态
**支付返回状态**

| 枚举值 | 枚举类型 |
| --- | --- |
| SUCCESS | 交易成功，支付成功的订单再次发起支付依然返回支付成功，商户侧需做幂等处理 |
| CLOSED | 已关闭 通常存在3种情况会关闭订单 1：商户侧发起的订单关闭 2：超出订单有效期还未支付成功的订单，系统自动关闭 3：被风控的订单 已关闭的订单不能再次发起支付 |
| WAIT_PAYING | 下单成功，等待用户支付中 |
| PAY_ERROR | 支付失败，同一笔订单号在有效期内可再次发起支付 |
| REFUND | 支付订单已退款 |
| ABNORMAL | 支付异常，返回此状态的支付订单，请稍后发起查询。 |

**退款返回状态**

| 枚举值 | 枚举类型 |
| --- | --- |
| SUCCESS | 退款成功 |
| REFUND | 退款受理成功 |
| REFUND_ERROR | 退款失败 |
| ABNORMAL | 退款异常，返回此状态的退款订单，请稍后发起查询。 |

**分账返回状态**

| 枚举值 | 枚举类型 |
| --- | --- |
| SUCCESS | 分账成功 |
| PROCESSING | 分账处理中 |
| CANCELED | 取消分账 |
| ABNORMAL | 分账请求异常 |

**云闪付签约订单状态**

| 枚举值 | 枚举类型 |
| --- | --- |
| SUCCESS | 成功 |
| PROCESSING | 处理中 |
| CLOSED | 已关闭 |
| FAIL | 失败 |
| ABNORMAL | 返回此状态的订单，请稍后发起查询 |

文档更新时间: 2024-03-26 07:09 作者：超级管理员
