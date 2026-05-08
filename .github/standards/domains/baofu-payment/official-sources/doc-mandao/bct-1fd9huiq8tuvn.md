---
title: 退款接口(协议/转账/网银支付)
slug: bct-1fd9huiq8tuvn
source_url: https://doc.mandao.com/docs/bct/bct-1fd9huiq8tuvn
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: a5f64cf70b51a23cc0467cf376677d07b8e6f5c840e0f39faa22c67575869829
doc_version: 1761711563
---

# 退款接口(协议/转账/网银支付)
# 1.文档说明

## 1.1文档目的

本文档的目的是为宝付商户退款API定义一个接口规范，以帮助商户技术人员快速接入宝付认证网关，并快速掌握其相关功能，便于尽快的投入使用。

## 1.2阅读对象

- 商户开发人员、维护人员和管理人员
- 宝付相关的技术人员

**DEMO参考**

| Demo版本 | 更新日期 | 下载链接 |
| --- | --- | --- |
| JAVA版 | 2023-06-29 | [点击下载](https://sp.baofoo.com/support-admin/sys/file/get/new/201d78c9-b0c9-407c-b3ea-5286a462086a) |
| PHP版 | 2023-06-29 | [点击下载](https://docs.baofu.com/attach_files/interface_document/74) |

## 1.3技术支持

在开发或使用退款API接口时，如果您有任何技术上的疑问，请按如下方式寻求帮助，宝付技术支持人员会及时处理，给予您答复：

- 技术支持热线：021-68819999
- 技术支持Email：support@baofoo.com
- 技术支持QQ：800066689

## 1.4术语与定义

### 1.4.1符号含义

| 序号 | 符号缩写 | 符号性质 | 符号说明 |
| --- | --- | --- | --- |
| 1 | M | 强制域(Mandatory) | 必须填写的属性，否则会被认为格式错误 |
| 2 | C | 条件域(Conditional) | 某条件成立时必须填写的属性 |
| 3 | O | 选用域(Optional) | 选填属性 |
| 4 | R | 原样返回域(Returned) | 必须与先前报文中对应域的值相同的域 |

### 1.4.2术语含义

- 商户号：宝付提供给商户的唯一编号，是商户在宝付的唯一标识；
- 终端号：商户在与宝付签订某项具体产品功能的合作协议自动分配的会员属性，将用于进行具体交易的必要参数。
- 原商户订单号：原先支付时商户请求提交的支付订单号。
- 退款商户订单号：商户退款时请求宝付提交的退款订单号，当天请求不可重复。
- 退款商户流水号：商户请求宝付退款时提交的流水号，每次请求均不可重复；
- 退款宝付业务流水号：宝付返回给商户的唯一退款订单号，是宝付记录一笔退款订单的唯一标识。

# 2.业务方案说明

## 2.1应用场景

退款API产品是宝付为满足商户在与宝付发生交易后需发起退款，以非页面手动点击，而是以纯后台的模式发起退款。

## 2.2业务流程

![](https://doc.mandao.com/uploads/refund_trade/images/m_18bbcd9720c7504d9493b7a1417fc168_r.png "null")

**流程说明：**  
1、持卡人在支付成功后，发起退款；  
2、商户向宝付发送退款请求；  
3、宝付进行信息审核，通知商户是否受理；  
4、退款结果无论成功或失败都会通知商户；  
5、商户可以向宝付发起退款查询；  
6、宝付返回查询结果详情。

# 3.业务接口说明

**测试信息：**  
参考demo

## 3.1退款请求接口

### 3.1.1交易URL

> 测试环境地址：<https://vgw.baofoo.com/cutpayment/api/backTransRequest>  
> 正式环境地址：<https://public.baofoo.com/cutpayment/api/backTransRequest>

### 3.1.2请求报文

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 版本号 | version | M | 4.0.0.0 |
| 02 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 03 | 终端号 | terminal_id | M |  |
| 04 | 交易类型 | txn_type | M | 331 |
| 05 | 交易子类 | txn_sub_type | M | 09 |
| 06 | 加密数据类型 | data_type | M | data_type=xml或json |
| 07 | 加密数据 | data_content | M | 具体参数如下加密数据 / 注意：加密之前,先将组装的数据（请参照数据模版组装）进行Base64编码转化，然后再进行证书加密。 |
| 08 | 风险控制参数 | risk_item | O | 风控预留字段 |

**加密数据**

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 终端号 | terminal_id | M |  |
| 02 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 03 | 交易子类 | txn_sub_type | M | 09 |
| 04 | 退款类型 | refund_type | M | 取值 / 1:网银支付 / 8:协议支付 / 18:转账支付 |
| 05 | 原商户订单号 | trans_id | M | 原商户发起的支付订单号 |
| 06 | 退款商户订单号 | refund_order_no | M | 退款时商户端生成的订单号 |
| 07 | 退款商户流水号 | trans_serial_no | M | 每次发起退款的流水不能重复 |
| 08 | 退款原因 | refund_reason | M |  |
| 09 | 退款金额 | refund_amt | M | 单位：分,例：1元则提交100 |
| 10 | 退款发起时间 | refund_time | M | 14 位定长。格式：年年年年月月日日时时分分秒秒 |
| 11 | 附加字段 | additional_info | O | 长度不超过 128 位 |
| 12 | 请求方保留域 | req_reserved | O |  |
| 13 | 服务器通知商户地址 | notice_url | O | 成功或者失败通知商户地址 |
| 15 | 营销组合支付已分账退款信息 | part_share_refund_info | O | 宝财通2.0退款需传字段，单位(分) / 格式:商户1,金额1;商户2,金额2… / 例如:CM100000363,10;100000364,90; |
| 16 | 营销组合支付分账退款是否垫资 | share_refund_advance_flag | O | 传1：是 / 不传或传0：否 / 如果垫资，part_share_refund_info字段传：垫资商户号，垫资金额； / 目前只支持主商户垫资。 / 注：垫资可能导致资金损失，请商户确认需要后谨慎选择 |

### 3.1.3应答报文

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 应答码 | resp_code | M | 具体参照错误码表 |
| 02 | 应答消息 | resp_msg | M | 具体参照错误码表 |
| 03 | 退款宝付业务流水号 | refund_business_no | M | 宝付返回给商户的唯一退款订单号 |
| 04 | 退款商户订单号 | refund_order_no | R |  |
| 05 | 退款金额 | refund_amt | R | 单位：分 |
| 06 | 终端号 | terminal_id | R |  |
| 07 | 商户号 | member_id | R |  |
| 08 | 数据类型 | data_type | R | xml/json |
| 09 | 交易类型 | txn_type | R |  |
| 10 | 交易子类 | txn_sub_type | R |  |
| 11 | 版本号 | version | O |  |
| 12 | 附加字段 | additional_info | O |  |
| 13 | 预留字段 | req_reserved | O |  |

### 3.1.4范例

#### 3.1.4.1请求范例

    https://tgw.baofoo.com/cutpayment/api/backTransRequest?version=4.0.0.1&member_id=123456&terminal_id=123456&txn_type=331&risk_item=risk&txn_sub_type=09&data_type=xml&data_content=2ac723cbe2ba7dab519525dec8208ab70c240d8bf8981ef5fdba238857612efe271dc212d4088a991ff5ce4cdf145a80c0cd24b43bd59ffd4c0f2a84b478196330f4fc76a0175feb932eb91c0ceef344099d1716a389fc5f503fece1bd69a6750715518402f116f72aff78da831816e29c2c4112af7a3aeeaec8821126a43ede694432aeb48b999e4369a0ea6f95e3f26eac6737976659c7d749b9adaea76483c4bb90efe11e30fd61fddd5fd3528aa01779bdbb953197652700cd82b6c51cd817e0374f1feee3befecb8773e90018ca79a4219e267c9e30e9efc96b61a45cfb980804eb5adbd4240cfee1cba25c7fd59162bb97359890fce2e21b3a060ad3174511e6db8adfd47cdaa6ad9a1a8928386a95d8d27d0f525a2124b66eba18b69735e8cb432a6be2af45304c8602b486adc6d2cd97bcd81d7939f130a41760ff92ae1f4b70960bcf9d5d4f5b57c65c72b81b7d420c7dc17181d3d62b298e4d2b74241cbe49a1067dc965f2fa333e723f71a0707e1bf9853073be0323dc5134eb2d688f3f949ea1e44344fa5b50a2e57e4a502f478ad67511e6db3503b9247269a91991b478b2b38fe72ad48bdef4144497b1a5072ecb5c2665763a6c36dbb0e6e2ac426bd1483911ec590da4464ce1ce0c7a9b846a589d38b65517cc6f09436b062d5cbd08d56507244956c1cf7236644fe890d1665a18742df9c93366fb8f98d06dd2ead44080fe94dabaebd1f46b99dac5da928ce89161c1304d7f4ecf5d3e8ead49cbd1373593a038c6f48df699a44b12aed6ac78f0b6b7a1287b730ae7a03053dae72061849fa8b472cb171c2740878b35bf5b4f9a1ecf910c5c41637fb5d98554e6b19aff8df4ed5be8c1519e59599298c70ddd1c9da5351f541a6f86cd4077a6788600f576f9bac39bee73f44274f2e07710809a0df0d71472d405d56db3d8488b66716de1c8cfdcb85577640bd53ce8895e1b29959931c8038e6159c496374ad9108187fe43bafd1c19796b978659427c7d7ad9124c14abffe7bd225b95a02c6cd0d2fd3536f9979a29e1421430085a81e3212edd67dd7975451dbd261f13524d382d12987b1a3f257c2615a6fc23c9ae850ebc09d106ff0ed2f3836ee19bd2cbe376b5f50a84f78a89d8b1182a18c8ddf10d266723fb82310293ecb656a2a9a2559453d702eed7949c190baae17d028260c1024a6c0ba2e335317e88b

**请求密文组装：**

- **xml格式：**

      <?xml version="1.0" encoding="UTF-8" ?>
      <data_content>
      <terminal_id>123456</terminal_id>
      <member_id>123564</member_id>
      <txn_sub_type>09</txn_sub_type>
      123456
      <refund_order_no>4509883</refund_order_no>
      <refund_reason>需要退款<refund_reason>
      <refund_amt>100</refund_amt>
      <refund_time>20160119111112</refund_time>
      <refund_type>2</refund_type>
      <notice_url>http://www.example.com/page_url</notice_url>
      1234567890
      <additional_info>附加字段</additional_info>
      <req_reserved>请求方保留域</req_reserved>
      </data_content>

- **json格式：**

      {
      "terminal_id":123456,
      "member_id":123564,
      "txn_sub_type:09,
      "trans_id":"123456",
      "refund_order_no":"14509883",
      "refund_reason":"需要退款",
      "refund_amt":"100",
      "refund_time":"20160119111112",
      "refund_type":"2",
      "notice_url":"http://www.example.com/page_url",
      "trans_serial_no":"1234567890",
      "additional_info":"附加字段",
      "req_reserved":"请求方保留域"
      }

  `备注：如果明文参数中data_type的值为xml，这里组装密文则使用xml格式，反之为json。将以上组装的字符串先进行base64进行加密，然后使用商户私钥进行证书加密，生成的密文则对应为data_content的值。`

#### 3.1.4.2应答范例

    6b269fdc40c6356713cf899cb0162ed03296299409d4745012078bd11b2f746045d77bd6b0ba637b73fa811c4300a05a87ce9881985a82995348ead4242ede9ba7952ed4a235e7288665ee9332c1c31f25c092059ed7155bc7bcbfd5069c8487832d376dceadd4201588b6fe1e3f91abe80c916e26ade7ee1cdfbc6e44748feb2c3987c3127f7200c4487464630e4e6ca51586010d05fbb8a200a0d5d988922238446ccce21f14b5c1fb56e4fac43f020fa3b3d9f991a2f4435a62ba61a9496acc15a3594fb6a0d63d96423aa3ddcc8ab75526c07ee4eaab08fe6a0f3ca10fd3494e700bda13dc5dfca099666f3eb8832537648e560580a3d6e8969d7dd9700b8118a994ce24e0d410f42f8c0aa02e6331ae5caf2afd4ac56fd2ca2491768d1409cdf7b81f50fa0f39bde32291dd02782195b17c3588298d6842a67924f62cdb96be4856c961fb067e509b94bd712f4a8a313b5578b1c0c441911d360b529c34fff9f7e0ff50d1e7e976ef2f7779765cadbe5cd2ba2396cecfa8ce85ef0ced8402b4d2d43e39237cccb1e83c58d8632b975214ea99dc8289e56365615e7e9c80fd665a5dd6ead4195e247a7fcd3e4282fa04a37eec2cd3f3d7382a7aaf40043117e78f8486042925899cfb5c5771f2b89a7b4d2a9baf1a3bb4179f7aa139c96e36df19746a167bbbe0663a92c3a91b856b02699464f23e2330c080915307f23d243f9db15683cd08156cc54e36fc4e5784a9384a888b240f097a0340e3d2046b21b21384ed446a4b6ca26df89c8bb746b21084d70841511aeedf0dfaad560f7f633196c7e1d5255e61866147d890e70156b6cf9796f80f64b47c239223cbf3e9c4262ec8dc5c41106218ac80ff110ab88b70c57281a384b4d2595e005ebcb70343e459dce32641c8fdeb00ee667fad61707f5263030fadabfd425f0ca8a652eec703ca5eeb0e37a9dbd8e1eff415fe72c20af520ab1c3bfc836e80de8713e66c2cec9ef89cb8b13e4acd4d5a7e7e358d10ba744694ab670ba483ee86ea10489cc905e9be8055ef287915e5f123

**应答密文解析：**

- **Xml格式：**

      <?xmlversion="1.0"encoding="UTF-8" ?>
      <result>
      <refund_amt></refund_amt>
      <refund_order_no>20160119123456</refund_order_no>
      <refund_business_no></refund_business_no>
      <version>4.0.0.1</version>
      <member_id>123456</member_id>
      <terminal_id>456789</terminal_id>
      <resp_code>0000</resp_code>
      <resp_msg>退款交易已受理</resp_msg>
      <req_reserved>预留字段</req_reserved>
      <additional_info></additional_info>
      <txn_sub_type>09</txn_sub_type>
      <txn_type>331</txn_type>
      <data_type>xml</data_type>
      </result>

- **Json格式：**

      {
      "data_type":"json",
      "member_id":"123456",
      "terminal_id":"456789",
      "refund_order_no":"20160119123456",
      "req_reserved":"预留字段",
      "resp_code":"0000",
      "resp_msg":"退款交易已受理",
      "txn_sub_type":"09",
      "txn_type":"331",
      "version":"4.0.0.1"
      }

  `备注：resp_code返回0000是表示此次受理交易已成功，并不代表退款成功。`

## 3.2退款状态查询

### 3.2.1交易URL

> 测试环境地址：<https://vgw.baofoo.com/cutpayment/api/backTransRequest>  
> 正式环境地址：<https://public.baofoo.com/cutpayment/api/backTransRequest>

### 3.2.2请求报文

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 版本号 | version | M | 4.0.0.0 |
| 02 | 商户号 | member_id | M |  |
| 03 | 终端号 | terminal_id | M |  |
| 04 | 交易类型 | txn_type | M | 331 |
| 05 | 交易子类 | txn_sub_type | M | 10 |
| 06 | 加密数据类型 | data_type | M | data_type =xml或json |
| 07 | 加密数据 | data_content | M | 具体参数如下加密数据 |

**加密数据**

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 交易子类 | txn_sub_type | M | 10 |
| 03 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 04 | 终端号 | terminal_id | M |  |
| 05 | 退款商户订单号 | refund_order_no | M | 退款时商户端生成的订单号 |
| 06 | 商户流水号 | trans_serial_no | M | 每次发起退款不能重复 |
| 07 | 附加字段 | additional_info | O | 长度不超过 128 位 |
| 08 | 请求方保留域 | req_reserved | O |  |

### 3.2.3应答报文

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 应答码 | resp_code | M | 具体参见附录：应答码 |
| 02 | 应答信息 | resp_msg | M | 填写具体的应答信息 |
| 03 | 退款商户订单号 | refund_order_no | R |  |
| 04 | 成功退款金额 | refund_amt | M | 交易成功后返回的金额,单位：分 |
| 05 | 商户号 | member_id | R |  |
| 06 | 终端号 | terminal_id | R |  |
| 07 | 交易类型 | txn_type | R |  |
| 08 | 交易子类 | txn_sub_type | R |  |
| 09 | 数据类型 | data_type | R |  |
| 10 | 版本号 | version | O |  |
| 11 | 附加字段 | additional_info O |  |  |
| 12 | 预留字段 | req_reserved O |  |  |

### 3.2.4范例

#### 3.2.4.1请求范例

    https://tgw.baofoo.com/cutpayment/api/backTransRequest?version=4.0.0.1&member_id=123456&terminal_id=123456&txn_type=331&txn_sub_type=10&data_type=xml&data_content=1f3f2b265e7fee210b6a04c78430d491956eae7858077f0c4ec90b34e5b94c7fb7bafcc8a58d55edb8880b97450843b544e33a3e6c31cf6461407925b05b81c49f2411574472531978600cdaabc73274f67065c0c6a5cc74d484f5552b5fd4039163fbbd65a904fc73b5891c6605d519b9e829bd2a36e21a911d86a9ba9e383b0a733c7ea357b8b36de7b50a1e9af45e8364a5dd4dde05416bf7ef768b609fa28eba198b0999bc06523799ad45f7693e4f22ae8392f1b6f55ae060531c80234787779fb5bbd3e26dbe7a2000776953c5c8affc4ec971d82c2679cf3bf11472346b74d271e5afec79f092af0599dfd5b7c6027276b211496e45871fba23af0d026e2c9aef9df41e210fbe90dea67f4ad606b7c4d8112d8c2a69a854c9d819c87b5c96d4d7ff92da4bbd540bbb994454374d4d963c8d900f8a462478bab8ff02daf3d6fef40f7602276a66e7c681e99264420fbafd57ad0bcdaecbc9473f3e6493f996d898cc288eed5e839e4e3a8ad77822ea57c7640a12337c51b3d42aecd8a5605b90623d94f8b5007f851504b42d32677c1d76a0b63a76c5689422f906363faafceeb1c216a2f95ee4e9fc5d638596df6e8378ec67b6598f3f663a5473f700fa6633ab5b852d1ffa323e648360713240b97b0ba68ff243167c995760f93e6677d68609c713e662be1b75e6c23bbb97209618116ce51bded2a113331a12b252011b35019c168e2e06f26409cc73a46ae467fa6b676ab8a11a1d1d29a68af8d55200cf6db46d259aa92e872728e410e50577c0f8aa028cd582c8adf564265e06299ff1a940e943fdf053e548840488f5dbc7804ea65330646e161e312c6e0dc9b5d0d78e9ba44e18966786568ea87bd7ef941647dd5d2efd60c616b98373b056

**请求密文组装：**

- **xml格式：**

      <?xml version="1.0" encoding="UTF-8" ?>
      <data_content>
      <terminal_id>123456</terminal_id>
      <member_id>123564</member_id>
      <txn_sub_type>10</txn_sub_type>
      <refund_order_no>4985612</refund_order_no>
      1234567890
      <additional_info>附加字段</additional_info>
      <req_reserved>请求方保留域</req_reserved>
      </data_content>

- **json格式：**

      {
      "terminal_id":123456,
      "member_id":123564,
      "txn_sub_type":10,
      "refund_order_no":"4985612",
      "trans_serial_no":"1234567890",
      "additional_info":"附件字段",
      "req_reserved":"请求方保留域"
      }

> **备注：如果明文参数中data_type的值为xml，这里组装密文则使用xml格式，反之为json。将以上组装的字符串先进行base64加密，然后使用商户私钥进行证书加密，生成的密文则对应为data_content的值。**

#### 3.2.4.2应答范例

    6b269fdc40c6356713cf899cb0162ed03296299409d4745012078bd11b2f746045d77bd6b0ba637b73fa811c4300a05a87ce9881985a82995348ead4242ede9ba7952ed4a235e7288665ee9332c1c31f25c092059ed7155bc7bcbfd5069c8487832d376dceadd4201588b6fe1e3f91abe80c916e26ade7ee1cdfbc6e44748feb2c3987c3127f7200c4487464630e4e6ca51586010d05fbb8a200a0d5d988922238446ccce21f14b5c1fb56e4fac43f020fa3b3d9f991a2f4435a62ba61a9496acc15a3594fb6a0d63d96423aa3ddcc8ab75526c07ee4eaab08fe6a0f3ca10fd3494e700bda13dc5dfca099666f3eb8832537648e560580a3d6e8969d7dd9700b8118a994ce24e0d410f42f8c0aa02e6331ae5caf2afd4ac56fd2ca2491768d1409cdf7b81f50fa0f39bde32291dd02782195b17c3588298d6842a67924f62cdb96be4856c961fb067e509b94bd712f4a8a313b5578b1c0c441911d360b529c34fff9f7e0ff50d1e7e976ef2f7779765cadbe5cd2ba2396cecfa8ce85ef0ced8402b4d2d43e39237cccb1e83c58d8632b975214ea99dc8289e56365615e7e9c80fd665a5dd6ead4195e247a7fcd3e4282fa04a37eec2cd3f3d7382a7aaf40043117e78f8486042925899cfb5c5771f2b89a7b4d2a9baf1a3bb4179f7aa139c96e36df19746a167bbbe0663a92c3a91b856b02699464f23e2330c080915307f23d243f9db15683cd08156cc54e36fc4e5784a9384a888b240f097a0340e3d2046b21b21384ed446a4b6ca26df89c8bb746b21084d70841511aeedf0dfaad560f7f633196c7e1d5255e61866147d890e70156b6cf9796f80f64b47c239223cbf3e9c4262ec8dc5c41106218ac80ff110ab88b70c57281a384b4d2595e005ebcb70343e459dce32641c8fdeb00ee667fad61707f5263030fadabfd425f0ca8a652eec703ca5eeb0e37a9dbd8e1eff415fe72c20af520ab1c3bfc836e80de8713e66c2cec9ef89cb8b13e4acd4d5a7e7e358d10ba744694ab670ba483ee86ea10489cc905e9be8055ef287915e5f123

**应答密文解析：**

- **Xml 格式：**

      <?xml version="1.0" encoding="UTF-8" ?>
      <result>
      <refund_amt></refund_amt>
      <refund_order_no>20160119123456</refund_order_no>
      <version>4.0.0.1</version>
      <member_id>123456</member_id>
      <resp_code>BF00128</resp_code>
      <resp_msg>该笔订单不存在</resp_msg>
      <terminal_id>456789</terminal_id>
      <req_reserved>预留字段</req_reserved>
      <additional_info>附加字段</additional_info>
      <txn_sub_type>10</txn_sub_type>
      <txn_type></txn_type>
      <data_type>xml</data_type>
      </result>

- **Json 格式：**

      {
      "additional_info":"附加字段",
      "data_type":"json",
      "member_id":"123456",
      "refund_order_no":"20160119123456",
      "req_reserved":"预留字段",
      "resp_code":"BF00128",
      "resp_msg":"该笔订单不存在",
      "terminal_id":"456789",
      "txn_sub_type":"10",
      "version":"4.0.0.1"
      }

> **备注：resp_code返回0000是表示退款成功。**

## 3.3通知商户退款结果

### 3.3.1异步通知报文

异步通知，商户接收到通知后，需在`notice_url`页面上输出 **`OK`**字符，表示接收成功\<除了 **`OK`** 无其他内容\>，若不返回则20分钟通知一次，一共通知5次结束。商户还可通过`退款状态查询接口`查询退款订单的状态。

**商户接受参数名为data_content,以下是报文体：**

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户号 | member_id | R |  |
| 02 | 退款商户订单号 | refund_order_no | R |  |
| 03 | 退款成功金额 | refund_amt | M |  |
| 04 | 退款宝付业务流水号 | refund_business_no | M |  |
| 05 | 返回码 | resp_code | M | 参照错误码 |
| 06 | 应答消息 | resp_msg | M | 应答消息 |

### 3.3.2异步通知范例

    6b269fdc40c6356713cf899cb0162ed03296299409d4745012078bd11b2f746045d77bd6b0ba637b73fa811c4300a05a87ce9881985a82995348ead4242ede9ba7952ed4a235e7288665ee9332c1c31f25c092059ed7155bc7bcbfd5069c8487832d376dceadd4201588b6fe1e3f91abe80c916e26ade7ee1cdfbc6e44748feb2c3987c3127f7200c4487464630e4e6ca51586010d05fbb8a200a0d5d988922238446ccce21f14b5c1fb56e4fac43f020fa3b3d9f991a2f4435a62ba61a9496acc15a3594fb6a0d63d96423aa3ddcc8ab75526c07ee4eaab08fe6a0f3ca10fd3494e700bda13dc5dfca099666f3eb8832537648e560580a3d6e8969d7dd9700b8118a994ce24e0d410f42f8c0aa02e6331ae5caf2afd4ac56fd2ca2491768d1409cdf7b81f50fa0f39bde32291dd02782195b17c3588298d6842a67924f62cdb96be4856c961fb067e509b94bd712f4a8a313b5578b1c0c441911d360b529c34fff9f7e0ff50d1e7e976ef2f7779765cadbe5cd2ba2396cecfa8ce85ef0ced8402b4d2d43e39237cccb1e83c58d8632b975214ea99dc8289e56365615e7e9c80fd665a5dd6ead4195e247a7fcd3e4282fa04a37eec2cd3f3d7382a7aaf40043117e78f8486042925899cfb5c5771f2b89a7b4d2a9baf1a3bb4179f7aa139c96e36df19746a167bbbe0663a92c3a91b856b02699464f23e2330c080915307f23d243f9db15683cd08156cc54e36fc4e5784a9384a888b240f097a0340e3d2046b21b21384ed446a4b6ca26df89c8bb746b21084d70841511aeedf0dfaad560f7f633196c7e1d5255e61866147d890e70156b6cf9796f80f64b47c239223cbf3e9c4262ec8dc5c41106218ac80ff110ab88b70c57281a384b4d2595e005ebcb70343e459dce32641c8fdeb00ee667fad61707f5263030fadabfd425f0ca8a652eec703ca5eeb0e37a9dbd8e1eff415fe72c20af520ab1c3bfc836e80de8713e66c2cec9ef89cb8b13e4acd4d5a7e7e358d10ba744694ab670ba483ee86ea10489cc905e9be8055ef287915e5f123

> **备注：退款成功或者失败均会通知商户。解密响应报文时，先使用RSA证书解密，再使用BASE64进行解密，具体报文格式跟退款请求时报文格式一致。**

**应答密文解析：**

- **xml格式：**

      <?xml version="1.0" encoding="UTF-8" ?>
      <result>
      <terminal_id>123456</terminal_id>
      <member_id>123564</member_id>
      <refund_amt>100</txn_sub_type>
      <refund_order_no>4985612</refund_order_no>
      <resp_code>0000</resp_code>
      <resp_msg>交易成功</resp_msg>
      <refund_business_no>20160119458974</refund_business_no>
      </result>

- **json格式：**

      {
      "terminal_id":123456,
      "member_id":123564,
      "refund_amt":100,
      "refund_order_no":"4985612",
      "resp_code":"0000",
      "additional_info":"附件字段",
      "resp_msg":"退款成功",
      "refund_business_no":"20160119458974"
      }

> **备注：resp_code返回0000是表示退款成功。**

# 附录：

### 1.应答码

**1)成功类**

| 错误码 | 含义 |
| --- | --- |
| 0000 | 交易成功，已退款成功金额为准（退款请求接口返回0000表示退款受理；退款状态查询接口返回0000表示退款成功） |

**2)交易处理中或未知，需要后续查询**

| 序号 | 符号缩写 | 符号性质 | 符号说明 |
| --- | --- | --- | --- |
| 1 | M | 强制域(Mandatory) | 必须填写的属性，否则会被认为格式错误 |
| 2 | C | 条件域(Conditional) | 某条件成立时必须填写的属性 |
| 3 | O | 选用域(Optional) | 选填属性 |
| 4 | R | 原样返回域(Returned) | 必须与先前报文中对应域的值相同的域 |
0

**3)交易失败**

| 序号 | 符号缩写 | 符号性质 | 符号说明 |
| --- | --- | --- | --- |
| 1 | M | 强制域(Mandatory) | 必须填写的属性，否则会被认为格式错误 |
| 2 | C | 条件域(Conditional) | 某条件成立时必须填写的属性 |
| 3 | O | 选用域(Optional) | 选填属性 |
| 4 | R | 原样返回域(Returned) | 必须与先前报文中对应域的值相同的域 |
1

### 2.交易子类

| 序号 | 符号缩写 | 符号性质 | 符号说明 |
| --- | --- | --- | --- |
| 1 | M | 强制域(Mandatory) | 必须填写的属性，否则会被认为格式错误 |
| 2 | C | 条件域(Conditional) | 某条件成立时必须填写的属性 |
| 3 | O | 选用域(Optional) | 选填属性 |
| 4 | R | 原样返回域(Returned) | 必须与先前报文中对应域的值相同的域 |
2

### 3．订单重复性校验

宝付为了确保交易能够准确的通知到商户，有可能会重复发送通知消息，为此宝付提醒商户，采取正确的防重复校验。  
大多数校验通知都采取先查询后更新的方式，这种方式存在一个很大的漏洞，当多个通 知请求在很短的一个时间内达到时，查询数据有可能是脏数据，导致订单重复更新，后续工 作重复执行。

对于此种情况，宝付结合自身校验情况，分享两个校验方式。  
1) 如果是单线程或者订单资源在一个共享区域，那么可以采取锁定资源的方式，每次 调用加锁，每次调用完毕解锁。  
2) 大部分数据都存在数据库，宝付绝大多数都是这种情况，利用数据库的特性来控制， 在我们调用数据库更新信息时，数据库会返回给我们更新条目数，我们利用这个特性，这样 操作，当我们更新订单时，把这个订单的原始状态作为条件进行更新，当我们短时间内操作 更新时，第二次更新必然不成功，因为条件不满足了，状态变了，那么数据库在第二次就会 返回更新数量为0，这样我们就发现问题了，这就是熟称的“乐观锁”。

文档更新时间: 2025-10-29 04:19   作者：超级管理员
