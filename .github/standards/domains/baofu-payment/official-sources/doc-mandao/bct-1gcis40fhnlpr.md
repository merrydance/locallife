---
title: 原单业务信息查询
slug: bct-1gcis40fhnlpr
source_url: https://doc.mandao.com/docs/bct/bct-1gcis40fhnlpr
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 17ba07b7bac2f1c22e4ce9ff59d17ac13e124c38861cf8e8bf8fe09ed7b54d79
doc_version: 1743557519
---

# 原单业务信息查询
#### 交易URL

●测试环境地址：[https://vgw.baofoo.com/cutpayment/protocol/backTransRequest](https://vgw.baofoo.com/cutpayment/protocol/backTransRequest) ●正式环境地址：[https://public.baofoo.com/cutpayment/protocol/backTransRequest](https://public.baofoo.com/cutpayment/protocol/backTransRequest)

#### 商户请求

| 字段名称 | 变量名 | 字段类型 | 是否必填 | 描述 |
| --- | --- | --- | --- | --- |
| 报文发送时间 | send_time | ISODateTime | M | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 报文流水号 | msg_id | Max32Text | M | |
| 报文编号/版本号 | version | Max7Text | M | 4.0.0.0 |
| 商户号 | member_id | Max11Numeric | M | 宝付提供给商户的唯一编号 |
| 终端号 | terminal_id | Max11Numeric | M | |
| 交易类型 | txn_type | code | M | 固定值：179 |
| 原订单交易日期 | orig_trade_date | ISODateTime | M | 格式：yyyy-MM-dd HH:mm:ss如2017-12-19 20:19:19 |
| 原支付订单商户订单号 | orig_trans_id | Max50Text | M | 商户提交的标识支付的唯一原订单号 |
| 商户保留域1 | req_reserved1 | Max255Text | O | |
| 商户保留域2 | req_reserved2 | Max255Text | O | |
| 系统保留域1 | additional_info1 | Max255Text | O | |
| 系统保留域2 | additional_info2 | Max255Text | O | |
| 签名域 | signature | Max512Text | M | |

#### 返回报文

| 字段名 | 变量名 | 字段类型 | 必填 | 描述 |
| --- | --- | --- | --- | --- |
| 报文发送时间 | send_time | ISODateTime | M | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 应答报文流水号 | msg_id | Max32Text | M | 宝付支付分配的代理商终端号 |
| 报文编号/版本号 | version | Max7Text | M | 4.0.0.0 |
| 应答码 | resp_code | Max16Text | M | 具体参见附录5：商户接口应答码 |
| 交易类型 | Max11Text | Max11Text | R | |
| 商户号 | member_id | Max11Numeric | R | 宝付支付分配的商户号 |
| 终端号 | terminal_id | Max11Numeric | R | 宝付支付分配的终端号 |
| 业务返回码 | biz_resp_code | | M | 具体参见附录1：业务应答码 |
| 业务返回说明 | biz_resp_msg | | M | |
| 原支付订单商户订单号 | orig_trans_id | Max50Text | R | 原支付订单商户订单号 |
| 原支付单总金额 | orig_order_amt | Max12Numeric | O | 单位：分 |
| 剩余分账金额 | shareable_amt | Max12Numeric | O | 单位：分 |
| 退款成功金额 | refund_succ_amt | Max12Numeric | O | 单位：分 |
| 退款处理中的金额 | refund_proc_amt | Max12Numeric | O | 单位：分 |
| 商户保留域1 | req_reserved1 | Max255Text | O | |
| 商户保留域2 | req_reserved2 | Max255Text | O | |
| 系统保留域1 | additional_info1 | Max255Text | O | |
| 系统保留域2 | additional_info2 | Max255Text | O | |
| 签名域 | signature | Max512Text | M | |

文档更新时间: 2025-04-02 01:31 作者：ian
