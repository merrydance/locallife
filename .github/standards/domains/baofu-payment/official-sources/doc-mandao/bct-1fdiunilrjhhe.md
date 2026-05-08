---
title: 支付结果通知
slug: bct-1fdiunilrjhhe
source_url: https://doc.mandao.com/docs/bct/bct-1fdiunilrjhhe
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 1f3e0d7b5776d7ee73d138d9abe8410a882c828f3c32f656b777d32ea5cf270d
doc_version: 1708657459
---

# 支付结果通知
### 3.1.2 支付返回报文

支付返回报文是指宝付在用户交易结束后，会将支付结果以页面返回和交易通知两种方式同时分别请求商户的页面返回地址和交易通知地址。  
支付结果异步通知页面通知和服务通知都是宝付通过非对称加密公私钥方式发送给商户。商户接收到宝付的通知，将接收到的data_content、terminal_id内容，用宝付发给商户的公钥证书（*.cer）进行解密，然后Base64解码。  
\**注：  
发送至商户页面返回地址的支付返回报文仅用于在页面上以一种友好的方式通知用户支付结果，代表支付成功或受理成功，并不代表交易成功，请切记不可使用该地址的服务进行交易结果处理。\*\*

**返回报文实例**  
data_content=194c4adff5c75c6dc8cca14ce6a3cd43563bea2a1dfbf9306536da0c2ba794282768d2e50ae0c144cefea720b721bb9222c1625bf52d34526ed8e0095c96f1f8746663a169b8ba0ebc2dae331c03015706c3c5e5b0e145b5c5a345c9f5cd882d5c8fe030d63331b3b5f755df927290927ea0ad7888cb36234630bf7052fa09cc5400a87dda910b89fa16895d21dc93d0bd54869cdd3382d032331a4c5af73ae4d8af44d2e494c871b0d38ee842f243190ce1e09ad239151ed366cea1c949bbc5f270d6d8f4ae0f32aaa2af21673cc2d4a6944e9cf1104ad4c6f6e3c2a2bd08a9b4d945370e2eaa6550c18a8187da9488271d95ae043ea423db281809194d92e5545afa5ea24c4d6a20bd20498ecead594c792338375ed4bb13209f7ca3fd821d2ca61bde987640b334a6565b7115ddeae145d185becd84f6c7c0035207dfaf3bc10339aa46e2c3319bb4b3725aac098b3091ebc1202e89ba56030e704fe219ffa4701191a3d46907af2724c362e18919c9ca851083715f74e35df512c43a97d65f39a91cb0c32b2533a2d41788b3ff75b5e20365e7983da0e5bdd60a11a3998f5041c8825f3e4c0f3a0b9a680fd4b12a3aaab8a46dbaaae5e6394c5838d3bca3286703e7bd95d1a091ce55ddaa22de7a4cfd7d18b65be064c5494f3dbf9b4c23f7fbb01e127b59274c075595a24f1c2a90c1f51a31237b8b05b5bdec4a012ac43dc1147e6afc715bb27ff26b664c5d81e0f84f487d4f9d54130a996d0fcabf14430bba99375a3ac43a0d58f6843dbb0c70398bd8c95653b11e4277ddda484e8b102c24a4c69428f22a7733e816b1ebb40c1c59ade8736250aafb2888184f622ad347719e96acfb4716f80fac4db5ebe726fead1b851aaafe8a1d31b0a23820c2&terminal_id=100001012

**解密后的报文**

- json格式：

      {"additional_info":"","fact_money":"2","member_id":"100018992","reserved":"","result":"1","result_desc":"01","succ_time":"20170428134119","terminal_id":"100001512","trans_id":"20170428134051154"}

- xml格式：

      <?xml version="1.0" encoding="UTF-8" ?>
        <result>
        <result_desc>01</result_desc>
        <fact_money>2</fact_money>
        <reserved/>
        <member_id>100018992</member_id>
        <terminal_id>100001512</terminal_id>
        20170428134152677
        <additional_info>test</additional_info>
        <succ_time>20170428134206</succ_time>
        <result>1</result>
      </result>

请求响应结果的数据类型根据请求参数字段“interfaceVersion”来决定，输出内容包括:

| 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- |
| member_id | 商户 ID | 保留 | 等同于商户提交的商户号 |
| terminal_id | 终端 ID | 保留 | 等同于商户提交的终端号 |
| trans_id | 订单 ID | 保留 | 等同于商户提交的订单 宝付不支持商户提交重复订单号，对重复订单号将 直接提示订单已经提交 |
| result | 支付结果 | 文档提供 | 代表与宝付的交易是否成功 / 1：成功 0：失败 可查看《附录：支付结果》 |
| result_desc | 订单结果 | 文档提供 | 代表该订单处理的结果成功与否 跟交易结果一起判断该笔订单的最终状态 查看附录：《附录：支付结果描述》 |
| fact_money | 成功金额 | 数值 | 单位：分 银行订单时验证成功金额和订单金额的一致 卡类订单时验证成功金额不可超过订单金额 |
| additional_info | 订单附加信息 | 保留 | 等同于商户提交的附加字段 |
| succ_time | 交易成功时间 | 字符串 | 交易成功，订单完成的时间 / 格式：年年年年月月日日时时分分秒秒 |

### 3.1.3 商户接收通知后通知页面内容

商户提交订单支付请求后，用户通过正常的支付流程支付成功或者支付失败后，支付接 口将支付结果通知给商户。  
支付结果通知由宝付系统确保一定成功发送给商户。成功发送是指由宝付发出结果通知 到商户后，能够成功获取商户的返回确认（商户接收通知的ReturnUrl在页面上输出OK表 示接收成功\<除了OK无任何其他内容\>），宝付系统在未确认商户接收通知成功后将会通过 重发机制通知商户（重发次数2~10次，请以第一次收到的支付成功的消息为准，避免进行 多次充值），同时会定时将那些已支付成功但商户未收到通知的交易取出，再次将支付成功 通知发给商户，直到商户接收成功或达到最大重发次数为止。

| 返回值 | 参数说明 |
| --- | --- |
| OK | 成功接收到宝付支付结果时返回OK / 注：宝付将接受返回的字符自动去除前空格，为兼容文件格式差异， 宝付会匹配前5个字符，包含OK的，则认为商户接受OK，否则按照规则补发通知。 |

文档更新时间: 2025-06-11 10:33   作者：超级管理员
