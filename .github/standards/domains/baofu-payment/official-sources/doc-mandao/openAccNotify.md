---
title: 开户结果通知
slug: openAccNotify
source_url: https://doc.mandao.com/docs/bct/openAccNotify
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 041108d7495da1d8c451701278c419702f3fc2a8ebc5cd32bfd6a11e09737874
doc_version: 1776823944
---

# 开户结果通知
------------------------------------------------------------------------

#### 接口说明:

1.商户接收到通知后务必在接收通知后返回大写OK  
2.宝付系统在未确认商户接收通知成功后将会通过重发机制通知商户（重发次数10次，请做好重复通知的处理逻辑，避免进行多次确认）通知发给商户。  
3.通知接口格式:<http://www.baidu.com/result?member_id=1&terminal_id=2&data_type=JSON&data_content=密文>  
**异步通知规则**  
1、地址通且有返回OK则后续不再通知  
2、地址通但未返回OK则通知4次，间隔时间：0，15S，15S，30S，单位：秒  
3、地址不通则通知10次，间隔时间：0，2，5，10，30，60，90，180，240，820 单位：分钟

#### 通知参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| member_id | String | 10 | M | 商户号 |
| terminal_id | String | 10 | M | 终端号 |
| data_type | String | 5 | M | JSON,data_content的明文组装格式 |
| data_content | String | 128 | M | 加密数据，需使用宝付公钥解密后再做base64解码 |

#### 开户通知参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| member_id | String | 10 | M | 商户号 |
| terminal_id | String | 10 | M | 终端号 |
| memberType | int | 1 | M | 类型:类型:1-个人,2-企业,3-个体工商户 |
| state | String | 4 | M | 状态 1 成功 0 失败 -1 异常 2开户处理中 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| transSerialNo | String | 200 | M | 请求流水号 |
| loginNo | String | 128 | M | 登录号 |
| customerName | String | 64 | M | 商户名称 |
| contractNo | String | 64 | M | 商户客户号 |
| noticeType | String | 32 | M |  |

##### 开户成功示例

    {
        "contractNo": "CP690000000000001468",
        "customerName": "张宝",
        "errorCode": "",
        "errorMsg": "",
        "loginNo": "person002",
        "memberId": "100030218",
        "memberType": "1",
        "state": "1",
        "terminalId": "200005478",
        "transSerialNo": "TSN314778753119603185643720"
    }

##### 开户失败示例

    {
        "contractNo": "",
        "customerName": "张宝",
        "errorCode": "ID_CARD_CHECK_FAILED",
        "errorMsg": "身份证号码不合法",
        "loginNo": "person002",
        "memberId": "100030218",
        "memberType": "1",
        "state": "0",
        "terminalId": "200005478",
        "transSerialNo": "TSN818378770194066009819111"
    }

文档更新时间: 2026-04-22 02:12   作者：超级管理员
