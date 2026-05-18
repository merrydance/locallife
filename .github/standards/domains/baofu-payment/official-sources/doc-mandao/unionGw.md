---
title: 账户请求入口
slug: unionGw
source_url: https://doc.mandao.com/docs/bct/unionGw
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 03a89b040748cac0059b4127d1b683082f13ad51c583266bdf9266dec2c80c85
doc_version: 1708676773
---

# 账户请求入口
# 1.文档说明

## 1.1文档目的

本文档是交易集中入口的说明文档。本文档主要介绍入口参数公共参数部分。具体业务部分的参数，参见具体的接口文档说明。  
`接口入口参数=公共参数+业务服务参数`

## 1.2阅读对象

- 接口对接开发人员、维护人员和管理人员

## 1.3技术支持

在开发或使用接口文档时，如果您有任何技术上的疑问，请按如下方式寻求帮助，宝付技术支持人员会及时处理，给予您答复：  
**技术支持热线：021-68819999  
技术支持Email：support@baofu.com  
技术支持QQ：800066689**

## 1.4术语与定义

### 1.4.1符号含义

| 序号 | 符号缩写 | 符号性质 | 符号说明 |
| --- | --- | --- | --- |
| 1 | M | 强制域(Mandatory) | 必须填写的属性，否则会被认为格式错误 |
| 2 | C | 条件域(Conditional) | 某条件成立时必须填写的属性 |
| 3 | O | 选用域(Optional) | 选填属性 |
| 4 | R | 原样返回域(Returned) | 必须与先前报文中对应域的值相同的域 |

### 1.4.2术语含义

- 商户号：宝付提供给商户的唯一编号，是商户在宝付的唯一标识；
- 终端号：商户在与宝付签订某项具体产品功能的合作协议自动分配的会员属性，将用于进行具体交易的必要参数。

# 2.统一入口说明

## 2.1 请求地址

- 测试环境地址：<https://vgw.baofoo.com/union-gw/api/%7B报文编号%7D/transReq.do>
- 正式环境地址：<https://public.baofu.com/union-gw/api/%7B报文编号%7D/transReq.do>

`提交方式：POST`

### 2.1.1 URL参数

**大致格式如下：**

    https://vgw.baofoo.com/union-gw/api/{报文编号}/transReq.do?memberId=%memberId&terminalId=%terminalId&verifyType=verifyType&content=%content&veryfyString=%veryfyString

## 2.2 请求报文

### 请求参数

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户号 | memberId | M | 宝付提供给商户的唯一编号 |
| 02 | 终端号 | terminalId | M | 终端号 |
| 03 | 签名类型 | verifyType | M | 类型值：1、2；参考verifyType传值说明 |
| 04 | 加密密文 | content | M | 字段按content说明组装后按verifyType类型加密，组装示例：{“body”: {“amount”: 1.00,”cardName”: “zhangsan”,”cardNo”: “123”}, / “header”: {“memberId”: “001”,”serviceTp”: “T-1001-001-01”,”terminalId”: “002”,”verifyType”: “1”}} |
| 05 | 验签密文 | veryfyString | C | 按verifyType类型加密 |

**verifyType传值说明：**

| verifyType值 | content加密方式 | verifyString加密方式 |
| --- | --- | --- |
| 1 | 1.先明文base64编码 / 2.然后RSA私钥加密生成16进制字符串 | 空 |
| 2 | 1.先明文base64编码 / 2.然后3des加密(key联系支持)生成16进制字符串 | 1.content基础上先sha-1签名，生成16进制小写字符串 / 2.然后RSA私钥签名生成16进制字符串 |

**content说明**  
1.格式：JSON  
2.组成：由header和body两部分组成  
3.明文示例:

    {
        "body": {
            "amount": 1.00,
            "cardName": "zhangsan",
            "cardNo": "123"
        },
        "header": {
            "memberId": "001",
            "serviceTp": "T-1001-001-01",
            "terminalId": "002",
            "verifyType": "1"
        }
    }

### header部分

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户号 | memberId | M | 宝付提供给商户的唯一编号 |
| 02 | 终端号 | terminalId | M | 终端号 |
| 03 | 报文编号 | serviceTp | M | 见附录；报文编号应与请求地址中一致 |
| 04 | 加密方式 | verifyType | M | 参考2.1.1 |

### body部分

参考具体业务接口的请求报文参数  
[账户API列表](https://doc.mandao.com/docs/bct/APIList)

## 2.3 同步响应报文

响应内容格式依赖于加密方式verifyType，具体如下

| verifyType值 | 返回内容 |
| --- | --- |
| 1 | base64和RSA私钥加密之后的内容 |
| 2 | content=数据内容&verifyString=签名内容 |

**verifyType=1时密文解密后的数据示例**

    {
        "body": {
            "accountType": "BASE_ACCOUNT",
            "balance": 33275.94,
            "retCode": 1
        },
        "header": {
            "memberId": "100024469",
            "serviceTp": "T-1001-006-03",
            "sysRespCode": "S_0000",
            "sysRespDesc": "",
            "terminalId": "200000994"
        }
    }

### header部分

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户号 | memberId | R | 宝付提供给商户的唯一编号 |
| 02 | 终端号 | terminalId | R | 终端号 |
| 03 | 报文编号 | serviceTp | R | 报文编号 |
| 04 | 系统返回码 | sysRespCode | M | 见附录 |
| 05 | 系统返回信息 | sysRespDesc | M |  |

### body部分

参考具体业务接口的返回报文参数  
[账户API列表](https://doc.mandao.com/docs/bct/APIList)

# 附录：

### 1.系统响应吗

> **注意：**
>
> - 此响应码仅代表系统请求的一个到达情况，不作为业务处理的结果标识。
> - 具体业务处理响应，需要根据body中具体的相关状态标识判断。

| 错误码 | 含义 | 请求状态 |
| --- | --- | --- |
| S_0000 | 请求正常 | 正常(请查看body中的相关信息) |
| S_E_0001 | 明文参数格式不正确,%s | 失败 |
| S_E_0002 | 明文参数解析失败,%s | 失败 |
| S_E_0003 | 商户信息不存在或状态不正常 | 失败 |
| S_E_0004 | 商户与终端号不匹配 | 失败 |
| S_E_0005 | ip未绑定，请联系宝付 | 失败 |
| S_E_0006 | 密文解密失败 | 失败 |
| S_E_0007 | 密文参数解析失败 | 失败 |
| S_E_0008 | 头参数格式不正确,%s | 失败 |
| S_E_0009 | 头参数和文明参数不一致,%s | 失败 |
| S_E_0010 | 接口服务报文不支持 | 失败 |
| S_E_9001 | 请求受理失败 | 失败 |
| S_E_9002 | 请求受理结果未知 | 未知 |

### 2.报文编号

> **报文规则**  
> 采用“T-XXXX-YY-NN”定义报文编号  
> T位固定位  
> XXXX 用来区分不同业务的报文  
> YY 为预留位  
> NN 为报文版本号信息

| 取值 | 交易子类 |
| --- | --- |
| T-1001-013-01 | 宝财通开户 |
| T-1001-013-02 | 修改绑定卡信息 |
| T-1001-013-03 | 开户信息查询 |
| T-1001-013-08 | 绑卡查询 |
| T-1001-013-06 | 余额查询 |
| T-1001-013-11 | 收支明细查询 |
| T-1001-013-14 | 账户提现 |
| T-1001-013-15 | 账户提现查询 |
| T-1001-013-13 | 账户间转账 |
| T-1001-013-10 | 账户间转账查询 |

文档更新时间: 2025-05-14 06:48   作者：超级管理员
