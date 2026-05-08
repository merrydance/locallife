---
title: 确认分账类交易
slug: bct-1f9qho10m3inj
source_url: https://doc.mandao.com/docs/bct/bct-1f9qho10m3inj
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 655aed27fd3037fed3850e550b2249e6b46f6552dac1496e089231c7d110bd37
doc_version: 1709778327
---

# 确认分账类交易
此确认分账类交易只支持调用支付接口的成功交易进行分账。

注：订单支付成功后必须在365天内完成分账，过期无法分账，需退款

------------------------------------------------------------------------

#### 交易URL

- 测试环境地址：<https://vgw.baofoo.com/cutpayment/protocol/backTransRequest>
- 正式环境地址：<https://public.baofoo.com/cutpayment/protocol/backTransRequest>

#### 请求报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1. | 报文发送日期时间 | send_time | M | ISODateTime | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 2. | 报文流水号 | msg_id | M | Max32Text | 商户流水号 |
| 3. | 报文编号/版本号 | version | M | Max7Text | 4.0.0.4 |
| 4. | 终端号 | terminal_id | M | Max11Numeric |  |
| 5. | 交易类型 | txn_type | M | code | 固定值：144(见附录：交易类型枚举) |
| 6. | 商户号 | member_id | M | Max11Numeric | 宝付提供给商户的唯一编号 |
| 7. | 商户原始订单号 | orig_trans_id | M | Max50Text | 商户提交的标识支付的唯一原订单号 |
| 8. | 分账信息 | share_info | M | 不限制 | 单位(分);格式:商户1,金额1;商户2,金额2… / 例如:CM690000000000000348,10;CP690000000000011438,90; |
| 9. | 分账结果通知地址 | share_notify_url | O | 不限制 | 分账成功之后通知地址 |
| 10. | 商户保留域1 | req_reserved1 | O | Max255Text |  |
| 11. | 商户保留域2 | req_reserved2 | O | Max255Text |  |
| 12. | 系统保留域1 | additional_info1 | O | Max255Text |  |
| 13. | 系统保留域2 | additional_info2 | O | Max255Text |  |
| 14. | 签名域 | signature | M | Max512Text |  |

#### 返回报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1 | 报文发送日期时间 | send_time | M | ISODateTime | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 2 | 应答报文流水号 | msg_id | M | Max32Text |  |
| 3 | 报文编号/版本号 | version | M | Max7Text | 4.0.0.4 |
| 4 | 应答码 | resp_code | M | Max16Text | 具体参见附录5：商户接口应答码 |
| 5 | 终端号 | terminal_id | R | Max11Numeric |  |
| 6 | 交易类型 | txn_type | R | Max11Text |  |
| 7 | 商户号 | member_id | R | Max11Numeric | 宝付提供给商户的唯一编号 |
| 8 | 业务返回码 | biz_resp_code | M |  | 具体参见附录1：业务应答码 |
| 9 | 业务返回说明 | biz_resp_msg | M |  |  |
| 10 | 商户保留域1 | req_reserved1 | O | Max255Text |  |
| 11 | 商户保留域2 | req_reserved2 | O | Max255Text |  |
| 12 | 系统保留域1 | additional_info1 | O | Max255Text |  |
| 13 | 系统保留域2 | additional_info2 | O | Max255Text |  |
| 14 | 签名域 | signature | M | Max512Text |  |
| 15 | 签名域 | signature | M | Max512Text |  |

文档更新时间: 2024-06-05 05:22   作者：超级管理员
