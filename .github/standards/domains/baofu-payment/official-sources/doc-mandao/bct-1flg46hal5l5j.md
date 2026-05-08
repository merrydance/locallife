---
title: 确认支付类交易
slug: bct-1flg46hal5l5j
source_url: https://doc.mandao.com/docs/bct/bct-1flg46hal5l5j
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: b18af57af1a46b5b96d5b340e1d0d45cdb19a1345c0ea81118e27eb051d7feb4
doc_version: 1717581956
---

# 确认支付类交易
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
| 5. | 交易类型 | txn_type | M | code | 固定值:160（见附录：交易类型枚举） |
| 6. | 商户号 | member_id | M | Max11Numeric | 宝付提供给商户的唯一编号 |
| 7. | 数字信封 | dgtl_envlp | M | Max512Text | 格式：01\|对称密钥，01代表AES / 加密方式：Base64转码后使用宝付的公钥加密 |
| 8. | 预支付唯一码 | unique_code | M | Max255Text | 格式：预支付唯一码 |
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
| 8. | 宝付订单号 | order_id | C | Max32Numeric | |
| 9. | 商户订单号 | trans_id | R | Max50Text | |
| 10. | 业务返回码 | biz_resp_code | M | | 具体参见附录1：业务应答码 |
| 11. | 业务返回说明 | biz_resp_msg | M | | |
| 12. | 成功金额 | succ_amt | C | Max12Numeric | 单位：分,例：1元则提交100 |
| 13. | 成功时间 | succ_time | C | ISODateTime | 支付成功时间 |
| 14. | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 15. | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 16. | 系统保留域1 | additional_info1 | O | Max255Text | |
| 17. | 系统保留域2 | additional_info2 | O | Max255Text | |
| 18. | 签名域 | signature | M | Max512Text | |

文档更新时间: 2024-06-05 10:05 作者：超级管理员
