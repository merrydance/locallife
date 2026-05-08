---
title: 分账状态查询
slug: bct-1f9qhp1tv3e22
source_url: https://doc.mandao.com/docs/bct/bct-1f9qhp1tv3e22
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 1181b7fbf65303e99394bcec35c86d0a375e494ca0659e81d7959edf2694cd59
doc_version: 1706079525
---

# 分账状态查询
当分账交易收单成功却没有收到分账成功通知时，可通过该接口查询分账订单状态。

#### 交易URL

- 测试环境地址：
 [https://vgw.baofoo.com/cutpayment/protocol/backTransRequest](https://vgw.baofoo.com/cutpayment/protocol/backTransRequest)
- 正式环境地址：
 [https://public.baofoo.com/cutpayment/protocol/backTransRequest](https://public.baofoo.com/cutpayment/protocol/backTransRequest)

#### 请求报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1. | 报文发送日期时间 | send_time | M | ISODateTime | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 2. | 报文流水号 | msg_id | M | Max32Text | 商户流水号 |
| 3. | 报文编号/版本号 | version | M | Max7Text | 4.0.0.4 |
| 4. | 终端号 | terminal_id | M | Max11Numeric | |
| 5. | 交易类型 | txn_type | M | code | 固定值：59(见附录：交易类型枚举) |
| 6. | 商户号 | member_id | M | Max11Numeric | 宝付提供给商户的唯一编号 |
| 7. | 商户原始订单号 | orig_trans_id | M | Max50Text | 商户提交的标识支付的唯一原订单号 |
| 8. | 交易日期 | orig_trade_date | M | ISODateTime | 格式：yyyy-MM-dd HH:mm:ss如2017-12-19 20:19:19 |
| 9. | 分账流水号 | share_msg_id | M | Max32Text | 确认分账时的商户流水号 |
| 10. | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 11. | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 12. | 系统保留域1 | additional_info1 | O | Max255Text | |
| 13. | 系统保留域2 | additional_info2 | O | Max255Text | |
| 14. | 签名域 | signature | M | Max512Text | |

#### 返回报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1. | 报文发送日期时间 | send_time | M | ISODateTime | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 2. | 应答报文流水号 | msg_id | M | Max32Text | |
| 3. | 报文编号/版本号 | version | M | Max7Text | 4.0.0.4 |
| 4. | 应答码 | resp_code | M | Max16Text | 具体参见附录5：商户接口应答码 |
| 5. | 终端号 | terminal_id | R | Max11Numeric | |
| 6. | 交易类型 | txn_type | R | Max11Text | |
| 7. | 商户号 | member_id | R | Max11Numeric | 宝付提供给商户的唯一编号 |
| 8. | 业务返回码 | biz_resp_code | M | | 具体参见附录1：业务应答码 |
| 9. | 业务返回说明 | biz_resp_msg | M | | |
| 10. | 订单状态 | order_state | C | Max2Numeric | 1：交易成功 / 2：交易处理中 / 0：未支付 / -1：交易失败 |
| 11. | 分账状态 | share_state | C | Max2Numeric | 订单状态为“1”时才会分账； / 1：分账成功 / 0：待分账 |
| 12. | 分账明细 | share_detail | C | 无限制 | 子商户分账明细， / 格式为：`商户号\|分账流水\|分账时间\|分账金额\|;商户号\|分账流水\|分账时\|分账金额\|;` / 分账时间格式为:yyyy-MM-dd HH:mm:ss / 分账金额单位为:分 |
| 13. | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 14. | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 15. | 系统保留域1 | additional_info1 | O | Max255Text | |
| 16. | 系统保留域2 | additional_info2 | O | Max255Text | |
| 17. | 签名域 | signature | M | Max512Text | |

> 注：该接口响应参数resp_code只会返回S（成功）和F（失败）。F代表接口参数错误或者分账订单不存在。S代表分账订单存在，具体的订单状态和分账状态需要判断order_state和share_state的返回值。

文档更新时间: 2024-06-05 05:22 作者：超级管理员
