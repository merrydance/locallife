---
title: 云闪付授权
slug: bct-1fao034siup3b
source_url: https://doc.mandao.com/docs/bct/bct-1fao034siup3b
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: f9912f1b68d4fd7e7996f12674661ca3a8995f6abd3f55fbe1f100e6d0b25f6e
doc_version: 1705457007
---

# 云闪付授权
# 接口说明

# 应用场景

在进行云闪付QUICK_PASS_NATIVE_JS支付前需先调用该授权接口获取用户标识，再调用云闪付获取用户标识接口quick_pass_auth获取user_id，最后再进行支付，本接口为同步应答模式。

# 注意事项

> - 该接口是由商户直接发起请求到银联，银联重定向到商户的页面，通过UR回调形式应答页面。
> - 请求银联地址：
> [https://qr.95516.com/qrcGtwWeb-web/api/userAuth](https://qr.95516.com/qrcGtwWeb-web/api/userAuth)
> - 请求方式：
> [https://qr.95516.com/qrcGtwWeb-web/api/userAuth?version=1.0.0&redirectUrl=https%3a%2f%2fpay.icbc.com%2furlToGetUserId](https://qr.95516.com/qrcGtwWeb-web/api/userAuth?version=1.0.0&redirectUrl=https%3a%2f%2fpay.icbc.com%2furlToGetUserId)

# 接口参数

> - 请求：

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 版本号 | version | 是 | S(16) | 1.0.0 | |
| 重定向地址 | redirectUrl | 是 | S(256) | [https://www.aaa.com](https://www.aaa.com) | 需进行URLEncode |

> - 返回

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 授权码 | userAuthCode | 否 | S(16) | 123456 | 获取APP用户信息的临时授权码，建议5分钟有效，并且只能访问一次 |
| 应答码 | respCode | 是 | S(10) | 00 | 00 – 成功, 34 – 不支持获取用户信息, 其它– 失败 |

文档更新时间: 2024-01-17 02:03 作者：超级管理员
