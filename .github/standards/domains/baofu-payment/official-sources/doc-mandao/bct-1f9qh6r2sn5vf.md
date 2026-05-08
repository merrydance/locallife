---
title: 接口说明
slug: bct-1f9qh6r2sn5vf
source_url: https://doc.mandao.com/docs/bct/bct-1f9qh6r2sn5vf
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 6d411480daae4649b033b6ee19c1b2410d65456ceaa85189455ecb486a8cc2b1
doc_version: 1704420503
---

# 接口说明
# 1 文档说明

## 1.1 文档目的

本文档是为宝付协议支付API产品定义一个接口规范，以帮助商户技术人员快速接入宝付协议支付API网关，并快速掌握其相关功能，便于尽快的投入使用。

## 1.2 阅读对象

- 商户开发人员、维护人员和管理人员
- 宝付协议支付API产品相关的技术人员

## 1.3 技术支持

在开发或使用宝付协议支付API接口时，如果您有任何技术上的疑问，请按如下方式寻求帮助，宝付技术支持人员会及时处理，给予您答复：  
**技术支持热线：021-68819999-8005**  
**技术支持Email：support@baofoo.com**  
**技术支持QQ：800066689**

## 1.4 术语与定义

### 1.4.1 符号含义

| 序号 | 符号缩写 | 符号性质 | 符号说明 |
| --- | --- | --- | --- |
| 1 | M | 强制域(Mandatory) | 必须填写的属性，否则会被认为格式错误 |
| 2 | C | 条件域(Conditional) | 某条件成立时必须填写的属性 |
| 3 | O | 选用域(Optional) | 选填属性 |
| 4 | R | 原样返回域(Returned) | 必须与先前报文中对应域的值相同的域 |

***数据类型***

> 类型语法：\[Max\]\[Min\]\[Size\]\[Type\]  
> Max:可选描述符，如果出现，则说明业务要素的长度最大为Size  
> Min:可选描述符，如果出现，则说明业务要素的长度最小为Size  
> Size：强制描述符，指定业务要素UTF-8编码前的最大字符数。  
> Type：强制描述符，指定业务要素的类型属性。Type主要属性如下

| 序号 | 字段类型Type | 符号说明 |
| --- | --- | --- |
| 1 | code | 编码枚举型数据，具体枚举类型见附录：枚举类型 |
| 2 | Text | 字符串 |
| 3 | Numeric | 数字 |
| 4 | ISODateTime | 日期时间，格式为 yyyy-MM-dd HH:mm:ss ,如:2017-12-20 21:54:21 |

***字符集及编码***

> 报文采用Unicode字符集，UTF-8编码方式。

***保留字***

> 报文内容中“\|”、“%”、 “#”、 “^”、 “-”等为局部保留字，在相关以此类字符作为分隔符的复合字段中不应出现。

### 1.4.2 术语含义

> - 商户号：宝付提供给商户的唯一编号，是商户在宝付的唯一标识；
> - 终端号：商户在与宝付签订某项具体产品功能的合作协议自动分配的会员属性，将用于进行具体交易的必要参数。
> - 商户流水号：商户请求宝付时提交的流水号，每次请求均不可重复；

## 1.5 通讯模式

> 采用HTTPS方式进行通讯。

## 1.6 报文说明

### 1.6.1 请求报文

***请求报文格式***

> 格式：key1=value1&key2=value2&key3=value3…  
> 例如：send_time=2018-01-24 13:25:33&msg_id=456795112&version=4.0.0.0&terminal_id=100000949&txn_type=03&member_id=100000749&dgtl_envlp=5a9c3ac419735d249e319727c89cfc0ce4a80d6a954980eaf3ea934316a56a121c758b0d13bf3302b877a8dd68619db72b2bd588ccdc9eb7fdb455705be1909df96540009146d7d81c96c0b90578f9344bd3fc00ded94d27c0c8040a83c02114b7a3a4698f830b7d0db60f230a5c3a4b38e7104088f2ee0139a4e765a9d79255&user_id=123&signature=7ca60bdea1f253b1a09588f7e4f0d455d984eaad0a446e61044c1527ea19fbdd70d690cc627327955b7a01a58acbc11cad6a26f8086c1bf23126da36832be59c46bc20e942bcae7614fcd9ba4dc7eec4c5e17024fb04fe5e63f2d137a3517a1e0c7bdea6d4ae33dbab7d20543e474a4bd790f7ba42cacaef45730623482a70ac

  
***签名算法***

> 将除签名字段之外的不为空的字段按key-value的形式构建TreeMap\<String, String\>对象，按key1=value1&key2=value2…模式将TreeMap对象转换为字符串，UTF-8编码格式下进行SHA-1计算后转换为16进制字节数组，用商户RSA私钥签名后转16进制。

  
***数字信封***

> 生成AES密钥，按照如下拼装：  
> 格式：`01|对称密钥`，01代表AES算法  
> 加密方式：Base64转码后使用宝付的公钥RSA加密

  
***敏感字段加密***

> 加密方式：Base64转码后，使用数字信封指定的方式和密钥加密

### 1.6.2 返回报文

***返回报文格式***

> 格式：key1=value1&key2=value2&key3=value3…  
> 例如：  
> biz_resp_code=0000&biz_resp_msg=交易成功&dgtl_envlp=74652829c07a71983c0da582321818aec41364528626e0f90eac1c633755b9dab84593695f5a101401052e9c64d457a881e442206330215de2281d2a3ea15d79e6732e296fdc36c6e0c76d17376cf6b9fc978b50bc747a9536d93226a69aba587f9fa5227a9b2cb915d1b822753f4a86a9fa1d81bf4d106723d927cf0f6365fb&member_id=100000749&msg_id=4a3f0b1862b94b6f853c1d28f9913f82&protocols=f222d7fe76b7c8ea7e22f3ee315e579a4263d697b12de605c287018e15cd530358dd8f638e4211b09e4e250d6b352304e0b454332aa0efda6977d435cf911dbc3943615ae31752e9a87c6e4b69dfc9e3af6be7a9a6e3f6a92a63e65b59936beb&resp_code=S&send_time=2018-01-25 09:53:01&signature=8ab74c7869632dc395cc945adcc388e6afceb759e4d406c3bb6e0e8002ec422f1615f2a43966d7337dcc57963f18877a959fe9f67b082da2cd95217ba003cc81f07962d665f576509ebc1a38f7ddf2a423775a794b262b7ffc4af615da3ba6bd05d0672c004d7cf80be3ed236f268078bb5c700d4b0a6ae9a0e58f2c782bd6ef&terminal_id=100000949&txn_type=03&version=4.0.0.0

  
***验签算法***

> 将宝付返回的除签名字段之外的不为空的字段按key-value的形式构建TreeMap\<String, String\>对象，按key1=value1&key2=value2…模式将TreeMap对象转换为字符串，UTF-8编码格式下进行SHA-1计算后转换为16进制字节数组。将宝付返回的签名字段转16进制字节数组，用宝付RSA公钥验签。

  
***数字信封解密***

> 解密方式：后使用商户的私钥解密后Base64解码  
> 解密后格式：`01|对称密钥`，01代表AES算法

  
***敏感字段解密***

> 解密方式：使用数字信封指定的方式和密钥解密后Base64解码

文档更新时间: 2024-01-05 02:08   作者：超级管理员
