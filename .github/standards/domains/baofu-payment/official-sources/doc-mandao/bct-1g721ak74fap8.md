---
title: 月终余额查询接口
slug: bct-1g721ak74fap8
source_url: https://doc.mandao.com/docs/bct/bct-1g721ak74fap8
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: d8ddf602985225ef61b668b240e8bdf12c8310a9dc30a755d07f734667c00a51
doc_version: 1737362562
---

# 月终余额查询接口
#### 月终余额查询

------------------------------------------------------------------------

#### 接口说明

报文编号：T-1001-013-12

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.1.0 |
| contractNo | String | 32 | M | 客户号 |
| date | String | 6 | M | 日期格式：YYYYMM |
| reqNo | String | 32 | M | 请求流水号 |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| contractNo | String | 32 | M | 客户号 |
| customerName | String | 32 | M | 客户姓名 |
| customerType | String | - | M | 客户类型：1个体、2个人、3企业 |
| beforeBalance | String | 16 | M | 月初余额，单位：元 |
| inBalance | String | 16 | M | 入金金额，单位：元 |
| inDetails | List | - | M | 入金明细：BusinessSummaryAmount |
| outBalance | String | 16 | M | 出金金额，单位：元 |
| outDetails | List | - | M | 出金明细：BusinessSummaryAmount |
| afterBalance | String | 16 | M | 期末金额，单位：元 |

BusinessSummaryAmount

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| businessType | String | 200 | M | 业务类型 / SHARE-分账 / OFFSET_SHARE-差额分账 / REFUND-退款 / TRANSFER-转账 / WITHDRAW-提现 / CLEAR-资金清算 / OTHER-其他 / WITHDRAW_CANCEL-提现退回 |
| businessTypeName | String | 200 | M | 业务类型名称 |
| amount | String | 16 | M | 业务汇总金额，单位：元 |

文档更新时间: 2025-09-16 09:09   作者：ian
