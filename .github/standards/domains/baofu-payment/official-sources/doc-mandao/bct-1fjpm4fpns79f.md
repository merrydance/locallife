---
title: 错误码
slug: bct-1fjpm4fpns79f
source_url: https://doc.mandao.com/docs/bct/bct-1fjpm4fpns79f
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 9894c4029422fc8f3403d1871c7cdcb586b7441abd3ffa323769100a4fd7ac42
doc_version: 1716200129
---

# 错误码
## 错误码汇总

> **当[header](https://doc.mandao.com/docs/bct/unionGw)的 sysRespCode 为S_0000时retCode**

| retCode | 含义 | 描述 |
| --- | --- | --- |
| 0 | 失败 | 接口调用失败，异常或者参数校验失败。 |
| 1 | 成功 | 接口调用成功，具体业务是否成功。看具体的参数字段。 |
| 2 | 处理中 | 接口调用处理中，需要调用查询接口查询状态。 |

> **开户接口业务参数**

| 参数名 | 描述 |
| --- | --- |
| retCode | 1 受理成功 0受理失败 |
| errorCode | 校验失败错误码 |
| errorMsg | 校验失败描述 |

> **errorCode 校验失败错误码-失败描述如下：**

| errorCode | 描述 |
| --- | --- |
| BF0001 | 请求参数非法 |
| BF0005 | 系统异常，请稍后再试 |
| BF00214 | 请求商户非法 |
| BF00062 | 开户只支持单笔 |

> **result中状态及错误码**

| 参数名 | 描述 |
| --- | --- |
| state | 状态 1 成功 0 失败 -1 异常 2开户处理中 |
| errorCode | 业务异常错误码 |
| errorMsg | 业务异常错误码描述 |

> **errorCode业务异常错误码**

| errorCode | errorMsg |
| --- | --- |
| BF00077 | 商户状态不正确 |
| BF00058 | 未开通相关产品 |
| BF00059 | 未开通相关功能 |
| BF00105 | 持卡人姓名与商户名称不同 |
| BF00110 | 上传客户文件不能为空 |
| BF00108 | 文件信息错误 |
| BF00107 | 文件缺失 |
| BF00060 | 该子商户已开户，请勿重复提交 |
| BF00063 | 您输入的银行卡号有误，请重新输入 |
| BF00111 | 绑定卡只能是借记卡 |
| BF0013 | 商户订单号已存在，请勿重复提交 |
| BF0002 | 数据库操作失败 |
| BF00106 | 持卡人姓名与经营者名称不同 |
| BF00217 | 企业有违法信息 |
| BF00061 | 企业法人四要素验证失败 |
| BF00218 | 企业信息比对比率低 |

> **异步通知开户结果错误码(包含上面的错误码)**

| errorCode | errorMsg |
| --- | --- |
| PARAMETER_VALID_NOT_PASS | 参数校验不通过 |
| PARAMETER_VALID | 请求参数有误 |
| ID_CARD_CHECK_FAILED | 身份证号码不合法 |
| EXISTED_LOGIN_NO | 登录号已存在 |
| REPEATED_REQUEST | 重复请求 |
| SYSTEM_INNER_ERROR | 服务忙，请稍后再试 |

文档更新时间: 2024-05-20 10:15   作者：超级管理员
