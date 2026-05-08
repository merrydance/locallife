---
title: 账户绑定查询接口
slug: bct-1gahm7u2hj1s1
source_url: https://doc.mandao.com/docs/bct/bct-1gahm7u2hj1s1
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: e7eff5ac92bee4c6047667ff95aef1b70ee3d7edb886e9983177f06befbcb49b
doc_version: 1741947311
---

# 账户绑定查询接口
接口用于查询宝付账簿二级户之间绑定从属关系。

------------------------------------------------------------------------

#### 接口说明

报文编号：T-1001-013-22

当header的 sysRespCode 为S_0000时，body的retCode如下：  
1.retCode=1 说明接口调用成功。具体业务是否成功。看具体的参数字段。  
2.retCode=0 说明接口调用失败。异常或者参数校验失败。  
3.retCode=2 说明接口调用处理中。需要调用查询接口查询状态。

当header的 sysRespCode 为非S_0000时，系统异常或者校验失败。和具体业务无关联。

#### 请求参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| version | String | 5 | M | 版本号4.1.0 |
| contractNo | String | [1,32] | M | 平台编号 |
| requestNo | String | [1,32] | M | 请求流水号 |

#### 返回参数

| 参数名称 | 类型 | 长度 | 出现要求 | 参数备注 |
| --- | --- | --- | --- | --- |
| retCode | int | 4 | M | 返回码 1 成功 0 失败 2 处理中 |
| errorCode | String | 20 | C | 错误码 |
| errorMsg | String | 40 | C | 错误原因 |
| back1 | String | 64 | O | 备用字段 |
| back2 | String | 64 | O | 备用字段 |
| back3 | String | 100 | O | 备用字段 |
| list | String |  | O | 绑定关系集合 json数组 |
| list.contractNo | String |  | O | 平台编号 |
| list.upperContractNo | String |  | O | 上级平台编号 |
| list.state | String |  | O | 状态 OPEN(“开启”),PENDING_OPEN(“待开启”),CLOSED(“关闭”); |

文档更新时间: 2025-09-16 09:09   作者：超级管理员
