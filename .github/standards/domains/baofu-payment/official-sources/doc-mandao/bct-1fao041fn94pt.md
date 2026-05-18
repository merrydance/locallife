---
title: 云闪付获取用户表示
slug: bct-1fao041fn94pt
source_url: https://doc.mandao.com/docs/bct/bct-1fao041fn94pt
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 2df897df5810309821cef4c9bed90d350facb5f96ea6f97e6cf0bd117b2163af
doc_version: 1705457029
---

# 云闪付获取用户表示
# 接口说明

| 接口名称 | quick_pass_auth |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 否 |

# 应用场景

在进行云闪付QUICK_PASS_NATIVE_JS支付前需先调用该接口获取用户标识，再进行支付。

# 接口参数

> - 请求：

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 交易商户号 | merId | 是 | S(16) | | |
| 交易终端号 | terId | 是 | S(16) | | |
| 授权码 | userAuthCode | 是 | S(16) | 123456 | 6.3接口返回的授权码 |
| 银联支付标识 | appUpIdentifier | 是 | S(256) | UnionPay/1.0 ICBCeLife | 识别HTTP请求User Agent 中包含银联支付标识 / 格式为：UnionPay/ / 注意APP标识仅支持字母和数字。 |

> - 返回

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 业务结果 | resultCode | 是 | S(16) | SUCCESS | 业务处理结果 |
| 错误代码 | errCode | 否 | S(32) | | 当业务结果FAIL时，返回错误代码 |
| 错误描述 | errMsg | 否 | S(128) | | 当业务结果为FAIL时，返回错误描述 |
| 用户标识 | userId | 是 | S(16) | 123456 | |

文档更新时间: 2024-01-17 02:03 作者：超级管理员
