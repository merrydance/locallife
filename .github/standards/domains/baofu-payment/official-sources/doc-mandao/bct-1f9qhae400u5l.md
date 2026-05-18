---
title: 接口说明
slug: bct-1f9qhae400u5l
source_url: https://doc.mandao.com/docs/bct/bct-1f9qhae400u5l
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: d88d2691112aaa20ca9a229dc7990e04db61f153fcef025fa3a9d79c44fd625a
doc_version: 1743996903
---

# 接口说明
# 通讯模式

> 接口如无特殊要求均采用HTTP POST方式提交。

# 接口地址

> - 测试请求地址：<https://mch-juhe.baofoo.com/api>
> - 生产请求地址：<https://juhe.baofoo.com/api>

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

# 签名和验签

> - 所有请求和返回报文都包含签名参数，接收方务必检查签名的正确性，以保证业务数据合法安全。
> - 签名和验签支持国密（SM2）和RSA两种方式，签名结果需要转换成16进制字符串。
> - RSA签名使用标准签名算法”SHA256withRSA”，密钥长度2048位。
> - RSA签名步骤：明文转json字符串-\>RSA-\>HEX(16)-\>密文(signStr)

# 幂等支持

> 本文档中部分接口支持幂等，当同一个商户订单号outTradeNo多次调用时，遵循如下：
>
> - 同一个outTradeNo代表同一笔交易，outTradeNo需保证全局唯一
> - 如之前已经返回成功，再次调用仍然会返回成功，不会重复处理交易
> - outTradeNo只能包含字母、数字、下划线 \_

# 特殊说明

> 本文档中部分接口字段名前包含中划线（-），当出现中划线时，表示该字段为上一级字段的叶子字段，即代表父字段为一个集合字段（JSON数组或JSON格式）

### DEMO参考

宝财通对接：聚合支付DEMO仅供参考方法，测试环境使用的商户测试信息及证书需要使用[宝财通2.0测试信息](https://sp.baofoo.com/support-admin/sys/file/get/new/6a8d655a-efc5-4773-9b12-54d84f320044)

| Demo版本 | 下载链接 |
| --- | --- |
| JAVA版 | [点击下载](https://sp.baofoo.com/supprecive/download/demo/52ac6dca3db3cecada74007cf85542e5) |
| .NET版 | [点击下载](https://sp.baofoo.com/supprecive/download/demo/d7643586f50554bdf80340d88b36db77) |
| PHP版 | [点击下载](https://sp.baofoo.com/supprecive/download/demo/587e919c7dab6dfb444f2372a17f2b7d) |

文档更新时间: 2025-04-07 03:35   作者：超级管理员
