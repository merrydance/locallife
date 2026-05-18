---
title: 宝付凭证文件
slug: juhe_pay-1f03799ice2ps
source_url: https://doc.mandao.com/docs/bct/juhe_pay-1f03799ice2ps
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 7ca4691d2e2e7a6c7a0916adb7d4a5aff2415be45a9a10ed466d9426e5216c6c
doc_version: 1763516202
---

# 宝付凭证文件
# 1.文档说明

## 1.1文档目的

本文档的目的是为宝付代付凭证文件定义一个接口规范，以帮助商户技术人员快速接入宝付代付凭证文件接口，并快速掌握代付凭证文件接口中的相关功能，便于尽快的投入使用。

## 1.2阅读对象

- 商户开发人员、维护人员和管理人员
- 宝付相关的技术人员

## 1.3技术支持

在开发或使用宝付代付凭证文件接口时，如果您有任何技术上的疑问，请按如下方式寻求帮助，宝付技术支持人员会及时处理，给予您答复：

**技术支持热线：021-68819999-8005 技术支持Email：[support@baofoo.com](mailto:support@baofoo.com) 技术支持QQ：800066689**

## **DEMO下载**

**友情提示：**

DEMO仅供开发者参考，实际参数以对应产品接口文档为准。

| Demo版本 | 更新日期 | 下载链接 |
| --- | --- | --- |
| JAVA版 | 2023-06-30 | [点击下载](https://sp.baofoo.com/support-admin/sys/file/get/new/dec0c632-236d-4a7c-b0be-551e0fb82de5) |

# 2.接口须知

## 2.1术语定义

| 序号 | 符号缩写 | 符号性质 | 符号说明 |
| --- | --- | --- | --- |
| 1 | M | 强制域(Mandatory) | 必须填写的属性，否则会被认为格式错误 |
| 2 | C | 条件域(Conditional) | 某条件成立时必须填写的属性 |
| 3 | O | 选用域(Optional) | 选填属性 |
| 4 | R | 原样返回域(Returned) | 必须与先前报文中对应域的值相同的域 |

## 2.2流程示意图（仅供参考）

![null](https://docs.baofu.com/uploads/interface_document/images/m_efa5a9df53537e0b5ca657e1468e9348_r.png)

## 2.3 凭证样例

**聚合收单凭证** ![null](https://doc.mandao.com/uploads/bct/images/m_10dbc2588dd641b36dd0c2283c5199e3_r.png) **宝财通聚合分账凭证** ![null](https://doc.mandao.com/uploads/bct/images/m_06fc069db2f02bff3da1a3585d19bf0c_r.png) **宝财通内部转账凭证** ![null](https://doc.mandao.com/uploads/bct/images/m_01660c627e7f15355389291ea6b9be91_r.png) **宝财通转账分账凭证** ![null](https://doc.mandao.com/uploads/bct/images/m_dcc8bdd6ff1173ebf21579e9fbfe8725_r.png)

# 3.凭证文件生成/查询接口

## 3.1接口说明

1.服务编号：`T-1001-003-03` 2.接口用于生成凭证文件，可做查询接口（有效期内不叠加次数）。 3.仅支持下载一年内的交易凭证，请下载后自行留存。 4.如需要重新生成订单，请联系宝付技术支持重置。

**注：请求地址请结合[宝付交易统一入口文档](https://docs.baofu.com/docs/interface_document/unionGwAPI)查看**

## 3.2请求报文

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| memberTransId | String | 100 | M | 商户订单号 |
| noticeUrl | String | 100 | O | 通知地址(不传不回调) |
| fileType | String | 2（枚举值） | M | 文件类型 / 1:PNG |
| transferDate | String | 20 | M | 原交易请求日期yyyy-MM-dd |
| orderType | String | 2 | M | 订单类型： / 9:聚合收单凭证 / 10:宝财通聚合分账凭证 / 91:内部划拨 / 92:付款 / 12:宝财通网银支付分账凭证 / 13:宝财通转账支付分账凭证 |

## 3.3返回报文

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| state | String | 10 | M | 响应码 |
| message | String | 40 | M | 提示信息 |
| urlDownload | String | 20 | C | 下载地址(有且仅当在有效期15分钟之内) |
| effectiveTime | String | 50 | C | URL截止有效时间yyyy-MM-dd HH:mm:ss |
| voucherName | String | 100 | C | 文件名称（zip） |
| voucherSize | String | 4 | C | 凭证数量 |
| count | String | | C | 当前订单笔数（日/月） |
| voucherId | String | | C | 宝付凭证订单号 |
| feeAmount | String | | C | 费用 |

# 4.凭证通知

> 异步通知以GET和POST方式发送到商户上送的接收地址（noticeUrl），商户接收到异步回调之后，需要商户在接收通知的地址页面上输出大写OK 表示接收成功，告诉宝付已经成功接收并处理完毕，宝付系统在未得到商户接收通知成功的反馈时，将通过重发机制再次通知商户（重发次数 2~10 次），直到商户接收成功或达到最大重发次数为止。

## 4.1 通知参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| memberId | String | 20 | M | 商户号 |
| terminalId | String | 20 | M | 终端号 |
| verifyType | int | 4 | M | 加密类型，原传入参数 |
| data_content | String | 100 | M | 加密数据 |

**data_content数据**

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| state | String | 1 | M | 处理状态 1成功 2失败 |
| memberId | String | 20 | M | 商户号 |
| terminalId | String | 20 | M | 终端号 |
| productNumber | String | 4 | M | 生成次数 |
| memberTransId | String | 100 | M | 商户订单号 |
| downloadUrl | String | 20 | C | 下载地址(有且仅当在有效期60分钟之内) |
| voucherName | String | 100 | C | 文件名称（zip） |
| voucherSize | String | 4 | C | 凭证数量 |
| effectiveTime | String | 50 | C | URL截止有效时间yyyy-MM-dd HH:mm:ss |
| count | String | | C | 当前订单笔数（日/月） |
| voucherId | String | | C | 宝付凭证订单号 |
| feeAmount | String | | C | 费用 |

# 5.注意事项

> 初次使用请仔细核对，信息是否有误，出现错误请及时联系宝付技术人员。以上字段及长度仅供参考

## 附录1：响应码信息

| 响应码 | 状态 | 错误信息 |
| --- | --- | --- |
| 0000 | 成功 | 请求交易成功返回此编码后需要判断交易状态，成功失败以交易状态为准（中态） |
| 0001 | 失败 | 商户公共参数格式不正确 |
| 0701 | 失败 | 系统处理失败 |
| 0401 | 失败 | 不存在成功订单！ |
| 0702 | 失败 | 系统暂停受理 |
| 0705 | 失败 | 文件已生成，请收到通知后及时下载 |
| 0999 | 未知 | 主机系统繁忙 |
| 9999 | 失败 | 接口服务已停止使用,请联系宝付技术支持 |

## 附件2：宝付统一入口须知

[宝付交易统一入口接口文档](https://docs.baofu.com/docs/interface_document/unionGwAPI)

**附件**

- [bct分账示例.png](https://doc.mandao.com/uploads/bct/images/m_af127f41485ae208cee2eb9a3934d992_r.png)

文档更新时间: 2025-11-19 01:36 作者：超级管理员
