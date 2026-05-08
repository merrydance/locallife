---
title: 云闪付无感支付
slug: bct-1fao0kpfs1nlf
source_url: https://doc.mandao.com/docs/bct/bct-1fao0kpfs1nlf
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 75a455161ee5c8fdd7c718c366badf01b2e79374678af20d3f91997d6ecae989
doc_version: 1705457623
---

# 云闪付无感支付
## 云闪无感支付

云闪付无感支付包含：

- 云闪付微信小程序签约支付
- 云闪付APP签约支付

### 1.云闪付无感支付签约流程

#### 签约交易（跳转云闪付微信小程序）

![云闪付无感支付微信小程序签约流程加步骤](https://doc.mandao.com/uploads/juhe_pay/images/m_3de3e69fac77e6419f8def8a786611b1_r.jpg)

- **商户与宝付接口交互说明**

| 交互序号 | 接口名称 | 说明 |
| --- | --- | --- |
| 2、5 | 3.9[云闪付无感支付签约](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037c2didc0q) | |
| 7 | 使用微信小程序的官方调用方法 | 详见无感支付签约中的调用连接 |
| 10 | 5.4[云闪付签约异步通知](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037ovukkv68) | |

#### 签约交易（跳转云闪付APP）

![云闪付无感支付APP签约流程加步骤](https://doc.mandao.com/uploads/juhe_pay/images/m_ce53ad489212bc856483de8f59928dc8_r.jpg)

- **商户与宝付接口交互说明**

| 交互序号 | 接口名称 | 说明 |
| --- | --- | --- |
| 2、5 | 3.9[云闪付无感支付签约](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037c2didc0q) | |
| 7 | 通过调用云闪付SDK唤起云闪付APP | 调用参考[云闪付APP支付开发包](https://sp.baofoo.com/support-admin/sys/file/get/new/3d86eec5-48a1-48bb-aaf7-714a35d3d411) / 调用方式详见云闪付APP支付开发包 / 注：接口文档以在线文档为准，开发包的接口文档仅参考 / 商户接入过云闪付APP支付可复用SDK |
| 9 | 5.4[云闪付签约异步通知](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037ovukkv68) | |

#### 签约订单查询

![签约订单查询](https://doc.mandao.com/uploads/juhe_pay/images/m_72d7a4494df1f7a74bab0909319feeb6_r.jpg)

- **商户与宝付接口交互说明**

| 交互序号 | 接口名称 | 说明 |
| --- | --- | --- |
| 1、2 | 4.4[签约订单](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037j1aujbop) | |

### 2. 云闪付无感支付（微信小程序）扣款流程

#### 扣款交易

![云闪付无感支付扣款商户交互流程](https://doc.mandao.com/uploads/juhe_pay/images/m_f9a196d6ceaaa434c3a7eff70f669823_r.jpg)

- **商户与报复接口交互说明**

| 交互序号 | 接口名称 | 说明 |
| --- | --- | --- |
| 2、5 | 3.1[统一下单交易创建](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037643a5cd5) | |
| 8 | 5.1[支付结果](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037n3bam87o) | |
| 9 | 5.2[分账结果](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037nks0j8rv) | 订单有分账情况下出现该通知 |

#### 退款交易

![云闪付app退款商户交互](https://doc.mandao.com/uploads/juhe_pay/images/m_cd61d289e2cd2b333e552481b38cbce8_r.jpg)

- **商户与报复接口交互说明**

| 交互序号 | 接口名称 | 说明 |
| --- | --- | --- |
| 2、3 | 3.7[申请退款](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037allmlijq) | |

#### 扣款订单查询

![云闪付app支付查询类交易商户交互流程](https://doc.mandao.com/uploads/juhe_pay/images/m_8ad5376398fe0c9cca56a96f2d474b68_r.jpg)

- **商户与报复接口交互说明**

| 交互序号 | 接口名称 | 说明 |
| --- | --- | --- |
| 2、3 | 4.1[支付订单](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037fqvh6j2u) | |
| 5、6 | 4.2[分账订单](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037h62bt8mb) | |
| 7、8 | 4.3[退款订单](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037i85v8eva) | |

文档更新时间: 2024-01-17 02:13 作者：超级管理员
