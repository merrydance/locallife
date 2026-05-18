---
title: 请求入口
slug: bct-1f9o5s1lqlean
source_url: https://doc.mandao.com/docs/bct/bct-1f9o5s1lqlean
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 5905ac532fdf019735ffcaba8cd13f35c64b8c58e2f54c648fc2c59958087b7f
doc_version: 1713943009
---

# 请求入口
### 接口地址

- 测试请求地址：
 [https://mch-juhe.baofoo.com/mch-service/api](https://mch-juhe.baofoo.com/mch-service/api)
- 生产请求地址：
 [https://juhe.baofoo.com/mch-service/api](https://juhe.baofoo.com/mch-service/api)

### 请求参数格式约定如下：

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 接口名称 | method | 是 | S(64) | merchant_report | 对应接口的名称，参考各接口说明 |
| 字符集 | charset | 是 | S(16) | UTF-8 | 字符集编码，固定值UTF-8 |
| 接口版本号 | version | 是 | S(8) | 1.0 | 对应接口的版本号，固定值1.0 |
| 格式化类型 | format | 是 | S(8) | json | 各个接口业务参数的格式化类型，固定值json |
| 时间戳 | timestamp | 是 | T | 20210315155012 | 时间戳与宝付支付系统时间误差不超过10分钟，格式为yyyyMMddHHmmss，如：2021年3月15日15点50分12秒表示为：20210315155012 |
| 加密签名类型 | signType | 是 | E | SM2 | 商户生成签名和加密字符串使用的算法类型，见附录【签名类型】 |
| 签名证书序列号 | signSn | 是 | S(10) | 1 | 发送方公钥证书序列号，用于接收方验签证书选择 |
| 加密证书序列号 | ncrptnSn | 是 | S(10) | 1 | 接收方公钥证书序列号，用于接收方解密证书选择 |
| 数字信封 | dgtlEnvlp | 否 | S | | 使用接收方公钥加密的对称密钥，并做16进制转码，是否需要传值，详见各个业务接口说明 |
| 签名串 | signStr | 是 | S | | 使用发送方私钥签名的非对称密钥，并做16进制转码 |
| 业务参数 | bizContent | 是 | S | | 业务数据报文，JSON格式，具体见各[业务接口](https://doc.mandao.com/docs/bct/bct-1f9o61rnb442i)定义 |

### 返回参数格式约定如下:

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 返回状态码 | returnCode | 是 | S(16) | SUCCESS | 成功：SUCCESS 失败：FAIL 通信标识，非交易标识 |
| 返回信息 | returnMsg | 是 | S(128) | OK | 当returnCode返回FAIL时返回错误信息，如：验签失败 解密失败 商户号不存在 参数格式验证错误等 检查报文重新发起请求 |

当returnCode返回SUCCESS时，以下字段有值：

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 字符集 | charset | 是 | S(16) | UTF-8 | 字符集编码，固定值UTF-8 |
| 接口版本号 | version | 是 | S(8) | 1.0 | 对应接口的版本号，固定值1.0 |
| 格式化类型 | format | 是 | S(8) | json | 各个接口业务参数的格式化类型，固定值json |
| 加密签名类型 | signType | 是 | E | SM2 | 与商户原请求签名加密类型一致，如：商户请求采用国密类型，则对应返回也采用国密类型 |
| 签名证书序列号 | signSn | 是 | S(10) | 1 | 发送方公钥证书序列号，用于接收方验签证书选择，固定值1 |
| 加密证书序列号 | ncrptnSn | 是 | S(10) | 1 | 接收方公钥证书序列号，用于接收方解密证书选择，固定值1 |
| 数字信封 | dgtlEnvlp | 否 | S | | 使用接收方公钥加密的对称密钥，并做16进制转码，返回是否有值，详见各个业务接口返回参数说明 |
| 签名串 | signStr | 是 | S | | 使用发送方私钥签名的非对称密钥，并做16进制转码 |
| 返回参数 | dataContent | 是 | S | | 业务数据报文，JSON格式，具体见各业务接口返回参数说明 |

文档更新时间: 2024-04-24 07:16 作者：超级管理员
