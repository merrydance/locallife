---
title: 异步通知
slug: bct-1f9qm3q0l3bdo
source_url: https://doc.mandao.com/docs/bct/bct-1f9qm3q0l3bdo
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 7f96535598bb76815a8632f499334cbc875563cc9fc34541604eb3db0e80c3bf
doc_version: 1704434112
---

# 异步通知
**注意事项：**

- 由于网络抖动等异常因素以及商户侧未按照约定返回OK，宝付支付系统会多次请求商户侧通知地址，商户系统需做订单的幂等性处理，避免重复处理导致损失。
- 商户系统接收到异步通知后，为保证资金安全建议做验签处理，并校验返回的订单金额与商户侧订单金额一致性，防止数据篡改泄漏等造成资损。
- 初次使用请仔细核对，信息是否有误，出现错误请及时联系宝付技术人员。

文档更新时间: 2024-01-17 02:14   作者：超级管理员
