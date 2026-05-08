---
title: 接口须知
slug: notice
source_url: https://doc.mandao.com/docs/bct/notice
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 6c23fb491958b5740924324315977bf79b60c5f11de37267521fc7371052e758
doc_version: 1746715045
---

# 接口须知
**系统技术文档由于功能升级要求会随着版本对请求参数、响应参数和异步通知增加必要的字段，请各接入商户做好系统兼容。**

### 概念

宝财通是指商户在宝付入网，商户下游企业或个体客户（ B端或 C端）在宝付开通的虚拟账户，如集团业务、分账业务等，对会员入网流程进行简化，实现在支付业务中的记账或清结算等功能。

一级商户：特约商户，在宝付开通商户号；  
二级商户：由一级商户通过接口进行开户，仅用作分账交易承载和清结算等；  
注： 二级商户按实际业务， 可支持企业、个体和个人。

### 测试对接信息

[宝财通2.0测试信息](https://sp.baofoo.com/supprecive/download/demo/98d7d41c4537c491d813dab113588aa7)

DEMO仅供参考，实现方法商户可按自已的开发方案实施。

### DEMO下载

| 语言 | 说明 | 日期 | 操作 |
| --- | --- | --- | --- |
| JAVA | 适用于Java语言、JDK版本1.7及以上的开发环境 | 2025/04/22 | [点击下载](https://sp.baofoo.com/supprecive/download/demo/edc1eee02c20003659ecfa69090a87b6) |
| PHP | 适用于PHP5.5及以上的开发环境 | 2023/12/06 | [点击下载](https://sp.baofoo.com/supprecive/download/demo/c01190e6c2690b15be60f418ff473245) |
| NET | 本项目加密方法依赖 BouncyCastle VS2015版可通过NuGet安装 BouncyCastle.Cryptography包。由于依赖包过大，DEMO已删除依赖包，运行时请自行通过NuGet安装/还原 | 2024/02/20 | [点击下载](https://sp.baofoo.com/supprecive/download/demo/157d75212e43933c52ba6d14858da1bd) |

目前需将宝户通商户迁移至宝财通账户的商户，可参考迁移方案  
宝户通迁移方案[`点击下载`](https://sp.baofoo.com/support-admin/sys/file/get/new/b8dfc248-a147-4811-95af-05562039265e "宝户通迁移方案下载")  
宝户通迁移方案需要联系商户经理申请

### 流程说明

![](https://doc.mandao.com/uploads/bct/images/m_93124aea250208088b8b1f17e5ba1cbc_r.jpg "null")

文档更新时间: 2025-05-14 06:48   作者：超级管理员
