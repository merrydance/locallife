---
title: 报备信息查询
slug: bct-1f9o63b6ufii5
source_url: https://doc.mandao.com/docs/bct/bct-1f9o63b6ufii5
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: f6a37102491580d108426533c48817e3eae19c28beccb56425823cc9e1f67e62
doc_version: 1704346299
---

# 报备信息查询
# 接口说明

| 接口名称 | merchant_report_query |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 否 |

# 应用场景

提供给商户报备后的商户查询，返回商户资料信息。

# 接口参数

## 请求参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 交易商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 交易终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 报备类型 | reportType | 是 | E | WECHAT | 详见附录：《报备类型》 |
| 报备编号 | reportNo | 是 | S(64) | 20211220120030798 | 每次请求报备接口唯一编号 |

## 返回参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 报备类型 | reportType | 是 | E | WECHAT | 详见附录：《报备类型》 |
| 报备编号 | reportNo | 是 | S(64) | 20211220120030798 | 每次请求报备接口唯一编号 |
| 报备状态 | reportState | 否 | S(16) | SUCCESS | 详见《报备状态》 |
| 渠道参数 | channelRetParam | 否 | C |  | 查询报备接口渠道返回参数，JSON格式 / 详见：[渠道参数：channelRetParam] |
| 业务结果 | resultCode | 是 | S(16) | SUCCESS | 业务处理结果 |
| 错误代码 | errCode | 否 | S(32) |  | 当业务结果FAIL时，返回错误代码 |
| 错误描述 | errMsg | 否 | S(128) |  | 当业务结果为FAIL时，返回错误描述 |

### 渠道参数：channelRetParam

#### 微信返回：

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 商户识别码 | sub_mch_id | 否 | S(30) | - | 微信分配的商户识别码 resultCode=SUCCESS时有值 |

#### 支付宝返回：

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 商户识别码 | sub_mch_id | 否 | S(30) | - | 微信分配的商户识别码 resultCode=SUCCESS时有值 |
| 间连等级 | indirect_level | 否 | S(16) | M1 | 间连商户等级，详见《间连等级枚举》 |

文档更新时间: 2024-01-04 05:31   作者：超级管理员
