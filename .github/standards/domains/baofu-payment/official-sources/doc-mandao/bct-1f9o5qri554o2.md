---
title: 接口须知
slug: bct-1f9o5qri554o2
source_url: https://doc.mandao.com/docs/bct/bct-1f9o5qri554o2
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: f750b92dc9e02a2e536b9f146f0172ec5a00fadfac6f8e2ac0a63e0035b2e132
doc_version: 1704338127
---

# 接口须知
# 通讯模式

> 接口如无特殊要求均采用HTTP POST方式提交。

# 接口地址

- 测试请求地址：<https://mch-juhe.baofoo.com/mch-service/api>
- 生产请求地址：<https://juhe.baofoo.com/mch-service/api>

# 参数说明

| 参数类型 | 简称 | 参数说明 |
| --- | --- | --- |
| 整数 | I | 整数，十亿以内，简称是大写INT的首字母 |
| 日期 | D | 使用yyyyMMdd（如20210315）的格式 |
| 日期时间 | T | 使用yyyyMMddHHmmss（如20210315155012）的格式 |
| 字符串 | S | 任意合法的字符串，如S(16)，表示字符串长度不超过16位 |
| 枚举值 | E | 见具体参数描述 |
| 浮点数 | F | 不超过10亿，小数点后最多7位 |
| 复合类型 | C | 数组内部嵌套键值对 |
| 布尔类型 | B | 布尔类型：true/false |

# 签名和验签

- 所有请求和返回报文都包含签名参数，接收方务必检查签名的正确性，以保证业务数据合法安全。
- 签名和验签支持SM2和RSA两种方式，签名结果需要转换成16进制字符串。
- RSA签名使用标准签名算法”SHA256withRSA”，密钥长度2048位。
- RSA签名步骤：明文转json字符串-\>RSA-\>HEX(16)-\>密文(signStr)

# 幂等支持

- 本文档中部分接口支持幂等，当同一个商户订单号requestNo多次调用时，遵循如下：
- 同一个requestNo代表同一笔交易，requestNo需保证全局唯一
- 如之前已经返回成功，再次调用仍然会返回成功，不会重复处理交易
- requestNo只能包含字母、数字、下划线 \_

# 特殊说明

本文档中部分接口字段名前包含中划线（-），当出现中划线时，表示该字段为上一级字段的叶子字段，即代表父字段为一个集合字段（JSON数组或JSON格式）

文档更新时间: 2024-01-04 03:15   作者：超级管理员
