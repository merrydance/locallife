---
title: 支付结果查询
slug: bct-1f9qhoo5vj18a
source_url: https://doc.mandao.com/docs/bct/bct-1f9qhoo5vj18a
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 52ea851bc2b60a79c82e02d5dce40cdc03bcc469f020ac90919626f3cf3cf71a
doc_version: 1704423449
---

# 支付结果查询
当系统返回异常或其他原因导致订单状态不明确时，可通过该接口查询订单状态。 当交易请求超时，不建议立即发起查询，建议延后再查询。

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
| 3. | 报文编号/版本号 | version | M | Max7Text | 4.0.0.0 |
| 4. | 终端号 | terminal_id | M | Max11Numeric | |
| 5. | 交易类型 | txn_type | M | code | 固定值：07(见附录：交易类型枚举) |
| 6. | 商户号 | member_id | M | Max11Numeric | 宝付提供给商户的唯一编号 |
| 7. | 商户原始订单号 | orig_trans_id | M | Max50Text | 商户提交的标识支付的唯一原订单号 |
| 8. | 交易日期 | orig_trade_date | M | ISODateTime | 格式：yyyy-MM-dd HH:mm:ss如2017-12-19 20:19:19 |
| 9. | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 10. | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 11. | 系统保留域1 | additional_info1 | O | Max255Text | |
| 12. | 系统保留域2 | additional_info2 | O | Max255Text | |
| 13. | 签名域 | signature | M | Max512Text | |

#### 返回报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1. | 报文发送日期时间 | send_time | M | ISODateTime | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 2. | 应答报文流水号 | msg_id | M | Max32Text | |
| 3. | 报文编号/版本号 | version | M | Max7Text | 4.0.0.0 |
| 4. | 应答码 | resp_code | M | Max16Text | 具体参见附录5：商户接口应答码 |
| 5. | 终端号 | terminal_id | R | Max11Numeric | |
| 6. | 交易类型 | txn_type | R | Max11Text | |
| 7. | 商户号 | member_id | R | Max11Numeric | 宝付提供给商户的唯一编号 |
| 8. | 业务返回码 | biz_resp_code | M | | 具体参见附录1：业务应答码 |
| 9. | 业务返回说明 | biz_resp_msg | M | | |
| 10. | 成功金额 | succ_amt | C | Max12Numeric | 单位：分,例：1元则100 |
| 11. | 成功时间 | succ_time | C | ISODateTime | 支付成功时间 |
| 12. | 宝付订单号 | order_id | C | Max32Numeric | |
| 13. | 商户订单号 | trans_id | R | Max50Text | |
| 14. | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 15. | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 16. | 系统保留域1 | additional_info1 | O | Max255Text | |
| 17. | 系统保留域2 | additional_info2 | O | Max255Text | |
| 18. | 签名域 | signature | M | Max512Text | |

文档更新时间: 2024-06-05 05:22 作者：超级管理员
