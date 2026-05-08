---
title: 绑定结果查询
slug: bct-1f9qhob58tblr
source_url: https://doc.mandao.com/docs/bct/bct-1f9qhob58tblr
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 0bb7b38e169325e030ae6c953d158a8e433505bccca2601d9cd17c351ff13988
doc_version: 1704421064
---

# 绑定结果查询
#### 交易URL

- 测试环境地址：
 [https://vgw.baofoo.com/cutpayment/protocol/backTransRequest](https://vgw.baofoo.com/cutpayment/protocol/backTransRequest)
- 正式环境地址：
 [https://public.baofoo.com/cutpayment/protocol/backTransRequest](https://public.baofoo.com/cutpayment/protocol/backTransRequest)

#### 请求报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1 | 报文发送日期时间 | send_time | M | ISODateTime | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 2 | 报文流水号 | msg_id | M | Max32Text | 商户流水号 |
| 3 | 报文编号/版本号 | version | M | Max7Text | 4.0.0.0 |
| 4 | 终端号 | terminal_id | M | Max11Numeric | |
| 5 | 交易类型 | txn_type | M | code | 固定值：03（见附录：交易类型枚举） |
| 6 | 商户号 | member_id | M | Max11Numeric | 宝付提供给商户的唯一编号 |
| 7 | 数字信封 | dgtl_envlp | C | Max512Text | 格式：01\|对称密钥，01代表AES,加密方式：Base64转码后使用宝付的公钥加密 |
| 8 | 用户ID | user_id | C | Max50Text | 用户在商户平台唯一ID |
| 9 | 银行卡号 | acc_no | C | Max20Text | 与user_id必须其中一个有值,加密方式：Base64转码后，使用数字信封指定的方式和密钥加密 |
| 10 | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 11 | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 12 | 系统保留域1 | additional_info1 | O | Max255Text | |
| 13 | 系统保留域2 | additional_info2 | O | Max255Text | |
| 14 | 签名域 | signature | M | Max512Text | |

#### 返回报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1 | 报文发送日期时间 | send_time | M | ISODateTime | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 2 | 应答报文流水号 | msg_id | M | Max32Text | |
| 3 | 报文编号/版本号 | version | M | Max7Text | 4.0.0.0 |
| 4 | 应答码 | resp_code | M | Max16Text | 具体参见附录5：商户接口应答码 |
| 5 | 终端号 | terminal_id | R | Max11Numeric | |
| 6 | 交易类型 | txn_type | R | Max11Text | |
| 7 | 商户号 | member_id | R | Max11Numeric | 宝付提供给商户的唯一编号 |
| 8 | 业务返回码 | biz_resp_code | M | | 具体参见附录1：业务应答码 |
| 9 | 业务返回说明 | biz_resp_msg | M | | |
| 10 | 数字信封 | dgtl_envlp | M | Max512Text | 格式：01\|对称密钥，01代表AES,加密方式：Base64转码后使用商户的公钥加密 |
| 11 | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 12 | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 13 | 系统保留域1 | additional_info1 | O | Max255Text | |
| 14 | 系统保留域2 | additional_info2 | O | Max255Text | |
| 15 | 签名域 | signature | M | Max512Text | |
| 16 | 协议列表 | protocols | M | Max1024Text | 格式：***签约协议号\|用户ID\|银行卡号\|银行编码\|银行名称; 签约协议号\|用户ID\|银行卡号\|银行编码\|银行名称***,加密方式：Base64转码后，使用数字信封指定的方式和密钥加密 |
| 17 | 签名域 | signature | M | Max512Text | |

文档更新时间: 2024-06-05 05:22 作者：超级管理员
