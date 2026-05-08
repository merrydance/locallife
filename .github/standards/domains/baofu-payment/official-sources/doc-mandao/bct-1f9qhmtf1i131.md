---
title: 确认绑卡
slug: bct-1f9qhmtf1i131
source_url: https://doc.mandao.com/docs/bct/bct-1f9qhmtf1i131
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: d1aba99432cd754570b315882ed636750cdc6321ee09b05fb7b8069361dbbf96
doc_version: 1704421009
---

# 确认绑卡
绑定银行卡是指经过持卡人授权将个人银行卡和商户建立绑定关系，支付时不再需要输入银行卡信息。商户需先进行预绑卡，宝付（或银行）会发送短信验证码（可根据银行要求来配置是否发送）给持卡人，商户再使用确认绑卡接口，将短信验证码回传给宝付完成绑卡。

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
| 5 | 交易类型 | txn_type | M | code | 固定值：12(见附录：交易类型枚举) |
| 6 | 商户号 | member_id | M | Max11Numeric | 宝付提供给商户的唯一编号 |
| 7 | 数字信封 | dgtl_envlp | M | Max512Text | 格式：01\|对称密钥，01代表AES;加密方式：Base64转码后使用宝付的公钥加密 |
| 8 | 预签约唯一码 | unique_code | M | Max126Text | 格式：***预签约唯一码\|短信验证码***;加密方式：Base64转码后，使用数字信封指定的方式和密钥加密 |
| 9 | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 10 | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 11 | 系统保留域1 | additional_info1 | O | Max255Text | |
| 12 | 系统保留域2 | additional_info2 | O | Max255Text | |
| 13 | 签名域 | signature | M | Max255Text | |

#### 返回报文

| 序号 | 域名 | 变量名 | 必填 | 字段类型 | 备注 |
| --- | --- | --- | --- | --- | --- |
| 1 | 报文发送日期时间 | send_time | M | ISODateTime | 发送方发出本报文时的机器日期时间，如 2017-12-19 20:19:19 |
| 2 | 应答报文流水号 | msg_id | M | Max32Text | |
| 3 | 报文编号/版本号 | version | R | Max7Text | 4.0.0.0 |
| 4 | 应答码 | resp_code | M | Max11Numeric | 具体参见附录：商户接口应答码 |
| 5 | 终端号 | terminal_id | R | Max11Numeric | |
| 6 | 交易类型 | txn_type | R | code | |
| 7 | 商户号 | member_id | R | Max11Numeric | 宝付提供给商户的唯一编号 |
| 8 | 业务返回码 | biz_resp_code | M | Max50Text | 具体参见附录：业务应答码 |
| 9 | 业务返回说明 | biz_resp_msg | M | Max50Text | |
| 10 | 签约协议号 | protocol_no | C | Max126Text | 只有成功时该字段才有值;加密方式：Base64转码后，使用数字信封指定的方式和密钥加密 |
| 11 | 银行编码 | bank_code | C | Max10Text | 只有在绑卡成功后该字段才有值 |
| 12 | 银行名称 | bank_name | C | Max10Text | 只有在绑卡成功后该字段才有值 |
| 13 | 数字信封 | dgtl_envlp | M | Max512Text | 格式：01\|对称密钥，01代表AES;加密方式：Base64转码后使用商户的公钥加密 |
| 14 | 商户保留域1 | req_reserved1 | O | Max255Text | |
| 15 | 商户保留域2 | req_reserved2 | O | Max255Text | |
| 16 | 系统保留域1 | additional_info1 | O | Max255Text | |
| 17 | 系统保留域2 | additional_info2 | O | Max255Text | |
| 18 | 签名域 | signature | M | Max512Text | |

文档更新时间: 2024-06-05 05:22 作者：超级管理员
