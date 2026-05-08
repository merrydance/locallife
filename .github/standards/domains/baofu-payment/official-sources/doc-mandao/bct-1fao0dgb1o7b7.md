---
title: 云闪付签约异步通知渠道返回参数
slug: bct-1fao0dgb1o7b7
source_url: https://doc.mandao.com/docs/bct/bct-1fao0dgb1o7b7
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 97e13348a336343a8cbbdad40a6ef4867fb574a3d1d2d8bb74e3db13dba0c38d
doc_version: 1705457357
---

# 云闪付签约异步通知渠道返回参数
## 渠道异步通知参数

> - 公共参数：
> - 公共参数适用于APP签约、小程序签约、H5签约

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 标记化支付信息域 | tokenPayData | 否 | C |  | 签约成功后返回，详见：[标记化支付信息域] |

## 标记化支付信息域

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 支付标记 | token | 否 | S(32) | 7978979797979789 | 用于替代卡号的标记。如果是标记化支付，那么就上送该字段，而不需要上送卡号。 |
| 标记请求者ID | trId | 否 | S(32) | 7978979797979789 | 标记请求者ID。如果是标记化支付，需要上送 trId。 |
| 标记担保级别 | tokenLevel | 否 | I |  | 标记的担保级别 |
| 标记生效时间 | tokenBegin | 否 | T | 20230606163030 | 标记生效时间 |
| 标记失效时间 | tokenEnd | 否 | T | 20250606163030 | 标记失效时间 |
| 标记类型 | tokenType | 否 | S(2) | 01 | 标记类型在TR申请入网的时候配置，表明该标记的标记类型 |

文档更新时间: 2024-01-17 02:09   作者：超级管理员
