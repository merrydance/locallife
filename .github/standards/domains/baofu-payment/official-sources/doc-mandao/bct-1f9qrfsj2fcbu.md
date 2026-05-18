---
title: 错误码
slug: bct-1f9qrfsj2fcbu
source_url: https://doc.mandao.com/docs/bct/bct-1f9qrfsj2fcbu
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 059d5c1f668f4cf66ef6a058fd4f3c7e227f42d3ba75a6d60f3a93d9c4d18750
doc_version: 1704431616
---

# 错误码
| 错误码 | 描述 | 解决方案 |
| --- | --- | --- |
| INVALID_PARAMETER | 参数无效 | 请检查提交的参数是否有误 |
| SYSTEM_BUSY | 系统繁忙，请稍后再试 | 请稍后再次发起 |
| ORDER_CREATED_FAIL | 订单创建失败 | 请检查提交的参数，稍后再次发起 |
| UNOPENED_PRODUCT | 商户未开通此产品 | 请联系宝付 |
| TRADE_AMT_EXCEEDS_LIMIT | 交易金额超过单笔支付限额 | 请联系宝付 |
| AGENT_RELATION_NOT_EXISTS | 代理商关系不存在 | 请检查提交的参数是否有误 |
| RISK_REFUSED | 风控拒绝 | 请联系宝付 |
| MERCHANT_NOT_EXIST | 商户号不存在 | 请检查商户号是否正确 |
| TERMINAL_NOT_EXIST | 终端号不存在 | 请检查终端号是否正确 |
| MERID_TERID_NOT_MATCH | 商户号和终端号不匹配 | 请确认商户号和终端号是否匹配 |
| VERIFY_ERROR | 验签错误 | 请检查签名参数和方法是否符合规范 |
| DGTL_DEC_ERROR | 数字信封解密失败 | 请检查签名参数和方法是否符合规范 |
| ORDER_EXIST | 订单已存在 | 请核实商户订单号是否重复提交 |
| ORDER_NOT_EXIST | 原订单不存在 | 请检查订单是否发起过交易 |
| SHARE_INFO_NOT_CORRECT | 分账信息校验不通过 | 请检查提交的参数是否有误 |
| SHARE_DEPLOY_NOT_EXIST | 分账配置不存在 | 请联系宝付 |
| DEPLOY_NOT_CORRECT | 分账配置不正确 | 请检查提交的参数是否有误 |
| FEE_MER_ID_ERROR | 扣费商户号传入错误 | 请检查提交的参数是否有误 |
| PAY_CODE_ERROR | 支付方式传入错误 | 请检查提交的参数是否有误 |
| TRADE_UNCONFIRMED | 交易结果未知，请稍后查询 | 请稍后查询 |
| ORDER_NOT_SUPPORT_REFUNDS | 订单未支付成功，无法退款 | 请核实支付订单是否支付成功 |
| REFUND_AMT_EXCEEDS | 退款金额超限 | 请检查提交的参数是否有误 |
| NOT_SUPPORT_PAY_CODE | 不支持的支付方式 | 请检查提交的参数是否有误 |
| NOT_SUPPORT_CONCURRENT | 不支持的并发操作 | 请分批次执行 |
| CHANNEL_RETURN_ERR | 渠道返回错误 | 请联系宝付 |
| MERCHANT_NOT_REPORT | 商户未在渠道报备 | 请先报备之后再做交易 |

文档更新时间: 2024-01-05 05:13   作者：超级管理员
