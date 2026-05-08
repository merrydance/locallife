---
title: 云闪付APP支付（线上收银台）
slug: bct-1fao0k4r3fjga
source_url: https://doc.mandao.com/docs/bct/bct-1fao0k4r3fjga
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 1b42446dace54e881bae59e30157ec61f8cef0e25734cb4d208143416e236040
doc_version: 1745317424
---

# 云闪付APP支付（线上收银台）
## 云闪付测试信息

- [云闪付测试卡信息](https://sp.baofoo.com/support-admin/sys/file/get/new/5f3c147d-f666-4181-9a48-84ca4940cd9f)
- [测试APK（云闪付测试APP_Android—暂不可用)](https://sp.baofoo.com/support-admin/sys/file/get/new/729d34b7-cb87-4a20-94a8-6afc858a6d47)
- [云闪付APP支付开发包下载](https://sp.baofoo.com/support-admin/sys/file/get/new/5e9b1170-74a2-4810-ae81-2a3dba7f3ae4)
- [云闪付APP支付HarmonyOS NEXT开发包](https://sp.baofoo.com/support-admin/sys/file/get/new/97308bad-5836-4d1a-93be-58a35e47d584)

## 云闪付APP支付(银联线上收银台)

### 1.云闪付APP支付流程

#### 消费交易

![云闪付APP支付消费交易流程](https://doc.mandao.com/uploads/juhe_pay/images/m_3b64a76f50d17f0628f24a32b57c899d_r.jpg)

- **商户与宝付接口交互说明**

| 交互序号 | 接口名称 | 说明 |
| --- | --- | --- |
| 2、5 | 3.1[统一下单交易创建](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037643a5cd5) | |
| 6 | 调用参考[云闪付APP支付开发包](https://sp.baofoo.com/support-admin/sys/file/get/new/3d86eec5-48a1-48bb-aaf7-714a35d3d411) | 调用方式详见云闪付APP支付开发包 / 注：接口文档以在线文档为准，开发包的接口文档仅参考 |
| 10 | 5.1[支付结果](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037n3bam87o) | |
| 11 | 5.2[分账结果](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037nks0j8rv) | 订单有分账情况下出现该通知 |

#### 退款交易

![云闪付app退款商户交互](https://doc.mandao.com/uploads/juhe_pay/images/m_cd61d289e2cd2b333e552481b38cbce8_r.jpg)

- **商户与宝付接口交互说明**

| 交互序号 | 接口名称 | 说明 |
| --- | --- | --- |
| 2、3 | 3.7[申请退款](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037allmlijq) | |

#### 查询交易

![云闪付app支付查询类交易商户交互流程](https://doc.mandao.com/uploads/juhe_pay/images/m_8ad5376398fe0c9cca56a96f2d474b68_r.jpg)

- **商户与宝付接口交互说明**

| 交互序号 | 接口名称 | 说明 |
| --- | --- | --- |
| 2、3 | 4.1[支付订单](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037fqvh6j2u) | |
| 5、6 | 4.2[分账订单](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037h62bt8mb) | |
| 7、8 | 4.3[退款订单](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037i85v8eva) | |

文档更新时间: 2025-04-22 10:23 作者：超级管理员
