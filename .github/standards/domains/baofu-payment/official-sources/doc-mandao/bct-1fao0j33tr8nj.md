---
title: 微信/支付宝流程
slug: bct-1fao0j33tr8nj
source_url: https://doc.mandao.com/docs/bct/bct-1fao0j33tr8nj
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 4eb1e42417f3acc3d0c4e680fccdae881802bc7d18cba21f849a1049ae36d13c
doc_version: 1736134072
---

# 微信/支付宝流程
## 聚合支付场景说明

- 1、扫码场景:商户通过在页面上展示二维码，用户通过使用微信/支付码扫码进行支付的场景。

      1.1、微信：使用公众号来实现，参看文档中公众号扫码支付流程。
      1.2、支付宝：支付宝支持主扫模式，接口中使用ALIPAY_NATIVE支付方式，返回支会链接，商户再将链接转成二维码向用户展示。

- 2、APP/H5场景：通过外部APP/H5等方式（非\[微信/支付宝\]体系），唤起支付场景。

      2.1、使用该场景进行外部唤起微信/支付宝支付的，依托于小程序（微信/支付宝）来实现。参看文档中的小程序流程图。
      2.2、微信APP可使用原生方法进行唤起支付，下单时接口需要上送WECHAT_APP支付方式。通过返参数调用微信原生SDK。

- 3、商户自有小程序/公众号在\[微信/支付宝\]体系内支付场景。

      3.1、该方式在接口中下单时上送对应的小程序/公众号的支付方式，通过返回的参数唤起[微信/支付宝]收银台进行支付。

以上场景的的支付都需要报备。

## 支付宝/微信流程相关

- 小程序  
  ![](https://doc.mandao.com/uploads/juhe_pay/images/m_94bd497df0eaecab69e53d17a394847b_r.png "null")

- 公众号  
  ![](https://doc.mandao.com/uploads/juhe_pay/images/m_b300d16cc7bb4a2f9ff0fb24c625f338_r.png "null")

文档更新时间: 2025-01-06 03:27   作者：超级管理员
