---
title: 绑定授权目录
slug: bct-1f9o63qmkndkc
source_url: https://doc.mandao.com/docs/bct/bct-1f9o63qmkndkc
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: cd96b027c3ed41943e99845894d8630f5c722b8ea10315510dc47172d6ca76df
doc_version: 1712114522
---

# 绑定授权目录
# 接口说明

| 接口名称 | bind_sub_config |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 否 |

# 应用场景

服务商给特约子商户配置支付目录；每个商户最多配置5个支付目录。

# 特殊说明

当返回结果：resultCode的值为SUCCESS 即绑定成功。

# 接口参数

## 请求参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 交易商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 交易终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 商户识别码 | subMchId | 是 | S(30) |  | 微信/支付宝分配的商户识别码 |
| 授权类型 | authType | 是 | E | AUTH | 详见附录：《授权类型》 |
| 授权内容 | authContent | 是 | S(256) | [http://www.qq.com/wechat](http://www.qq.com/wechat) | 1 授权类型为AUTH填写支付路径，要求符合URI格式规范，每次添加一个支付目录，最多5个 / 2 授权类型为JSAPI需填写公众号appid / 3 授权类型为APPLET需填写小程序appid |
| 备注 | remark | 是 | S(128) | JSAPI授权目录 | 备注信息 |

## 返回参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 业务结果 | resultCode | 是 | S(16) | SUCCESS | 业务处理结果 |
| 错误代码 | errCode | 否 | S(32) |  | 当业务结果FAIL时，返回错误代码 |
| 错误描述 | errMsg | 否 | S(128) |  | 当业务结果为FAIL时，返回错误描述 |

文档更新时间: 2024-04-03 03:22   作者：超级管理员
