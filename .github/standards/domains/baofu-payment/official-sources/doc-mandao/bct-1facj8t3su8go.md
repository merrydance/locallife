---
title: 接口说明
slug: bct-1facj8t3su8go
source_url: https://doc.mandao.com/docs/bct/bct-1facj8t3su8go
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 63713ac18c4fab9444edc6f6b8ed528122321b046e403f584e7b9f3b8f48a6a2
doc_version: 1770188635
---

# 接口说明
# 2 业务方案说明

## 2.1 应用场景

转账支付产品由宝付开发，提供给合作商户的一种安全的支付产品。  
用户购买商品或服务时，向宝付分配给商户的专属账户打款，宝付进行自动匹配加值的模式。  
注：该接口为纯后台模式

## 2.2 业务流程

1、商户开通转账支付/转账支付分账产品，自动创建分配商户转账支付专属账户；  
2、用户在商户平台进行正常的支付请求，商户向用户展示收款专属账户信息；

> `账号：开通商户号后由宝付分配`  
> `户名：宝付支付（上海）有限公司`  
> `银行：支付机构备付金集中存管账户`  
> `开户城市：上海`  
> `开户网点：宝付网络-备付金账户`  
> `行号：991290000793`

3、商户组织交易报文，将用户支付要素发送给宝付；  
4、用户通过网银或手机APP向商户的收款专属账户进行支付付款款项；  
5、宝付调用银行得到支付结果；  
6、宝付异步返回支付结果给商户；

## 2.3 业务接口

### 2.3.1 转账支付类交易

转账支付类交易是指商户根据持卡人请求指令，将持卡人信息发送至宝付，持卡人通过网银或手机APP从银行卡账户中支付付费款项的业务，该模式属于后台支付模式。

交易URL  
测试环境地址：<https://vgw.baofoo.com/cutpayment/api/backTransRequest>  
正式环境地址：<https://public.baofoo.com/cutpayment/api/backTransRequest>

#### 2.3.1.1 请求报文

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 版本号 | version | M | 4.0.0.0 |
| 02 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 03 | 终端号 | terminal_id | M |  |
| 04 | 交易类型 | txn_type | M | 取值0431 |
| 05 | 交易子类 | txn_sub_type | M | 97 |
| 06 | 加密数据类型 | data_type | M | json |
| 07 | 加密数据 | data_content | M | 具体参数如下加密数据。注意：加密之前,先将组装的数据（请参照数据模版组装）进行Base64编码转化，然后再进行证书加密。 |

**加密数据**

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 02 | 终端号 | terminal_id | M |  |
| 03 | 交易子类 | txn_sub_type | M | 97 |
| 04 | 商户订单号 | trans_id | M | 商户发起的支付订单号 |
| 05 | 商户流水号 | trans_serial_no | M | 8-20 位字母和数字，每次请求都不可重复 |
| 06 | 接入类型 | biz_type | C | 其他：不填写和默认0000,表示为储蓄卡支付。 |
| 07 | 支付金额 | order_money | M | 单位：分 |
| 08 | 收单金额 | acquiring_money | M | 单位：分。 |
| 09 | 收款方账户号 | payee_acct_code | M | 收款方账户号 |
| 10 | 付款方账户号 | payer_acct_code | M | 付款方账户号 |
| 11 | 付款方开户名 | payer_user_name | M | 付款方开户名 |
| 12 | 回调地址 | call_back_url | O | 如果传入，则异步通知使用该url；如果未传入，则不做异步通知 |
| 14 | 请求方保留域 | req_reserved | O |  |
| 15 | 手续费承担方 | fee_member_id | O |  |

**加密数据模板**

- JSON

      {
      "terminal_id":123456,
      "member_id":123564,
      "txn_sub_type:88,
      "trans_id":"123456",
      "trans_serial_no":"1234567890",
      "additional_info":"附加字段",
      "req_reserved":"请求方保留域",
      "order_money":"1000000",
      "payee_acct_code":"收款方账户号",
      "payer_acct_code ":"付款方账户号",
      "payer_user_name ":"付款方开户名",
      "bank_serial_no":"银行流水",
      "call_back_url ":"支付回调地址",
      "share_info":"100000363,10;100000364,90 "
      "share_notify_url":"分账回调地址",
      } 

#### 2.3.1.2 应答报文（宝付返回报文）

| 序号 | 参数含义 | 参数名称 | 必填 | 参数备注 |
| --- | --- | --- | --- | --- |
| 01 | 应答码 | resp_code | M | 应答码为0000时，表示订单预提交成功 |
| 02 | 应答信息 | resp_msg | M |  |
| 03 | 版本号 | version | R |  |
| 04 | 商户号 | member_id | R | 宝付提供给商户的唯一编号 |
| 05 | 终端号 | terminal_id | R |  |
| 16 | 接入类型 | biz_type | R |  |
| 07 | 数据类型 | data_type | R | xml/json |
| 08 | 交易类型 | txn_type | R | 取值0431 |
| 09 | 交易子类 | txn_sub_type | R | 97 |
| 10 | 商户流水号 | trans_serial_no | R | 8-20 位字母和数字，每次请求都不可重复 |
| 11 | 商户订单号 | trans_id | R | 商户订单号 |
| 12 | 附加字段 | additional_info | O |  |
| 13 | 预留字段 | req_reserved | R |  |

支付响应结果都是宝付通过非对称加密公私钥方式发送给商户。商户接收到宝付的响应报文，将接收到的data_content内容，用宝付发给商户且后缀为（\*.cer）的公钥证书进行解密。

**返回报文实例**  
4d34ab4f6f7f0cf3bec59a8028bb253ae6e6408c3ccb60d440a900d7e1cc627765cb8d9e0ec8b90caab5501d4fc54fcca54e7932e1254ff2f54aebd476f1f663f76fe4e855767621012fd69fb67130ac66362dc110219352b8b72b913cb354c50adf94b4b57728b1ec604288e0abab32987b72ba2f056c021d15326caac1d23d164f068f8208a471f7e939e0cc8aa757db805e109ab79ec547e5adbebfef338755bafe27bacd43329b0e9aa9660b63503eaf91f1185a868e54a850bc73531fee1f8f2d760f2bec40bcbf5cacd6723e8d2bae01dd89247020de6efc2a104c70ac5a448cd43bd18ab32c6b97516158454a98941864245eb837a82a4df039373e3b400f5e297ffc4504a205ba50a52d9f8824699dc21c91bf0d3da3026baf4c8863fb6c297ddbe84191e0928361ef70adacb6f886ba60eb4c77cd1925a144ecf993c7f27d00f2fe8402b26323f5341f4cc4febe09b99975cae0740a5f3c3f44a23b6efed9a8dff1ec8501252763a000945950d18ed71dc698b1fb23cf2456ef63f73c3fb46578d838de0477dc5535b6e282a3c7c29c50d2a30688c1d8fb64720a4badaf2818888ff428d42207de75fa8f10296c0827291845a54dfd29ef58ca57e839d4d6327b5287ba3f72eed76b7728d51bf1d9ab207d0d5e2b2cc732ae29cd4baef6165359fd18b986723f484c5ee87be99dc393a147e7642c855644076522f45625305828dcb150cbf006b945551207b3fad9e34d9adc7e6d7f9fe87ee8c4a2e671609a75670f66f2c64175e72d1fe0f417c60996950c3aa616c0e275c5fa5217edf37b16c1df3d4f80dce6c2b5c86f13ff2507e643b30e9d77d2dded554ac8f6532e80641dac9528b0e21d3c7ffafcbdb270218789f19bc9f9c7e3a642fa332191bf8d0c73873b20720e896d91359ae55b791fdab61b5e4a3cae4b5457925053078560f2bff13bd9ce7d53fd195700a7c10b4bd91e8ebf7edce21d591be6896f35b90304bfa343a38bf6cdde3e0f1f1c87306f36a2cb787ddb4e7b8fa7488977b85df7afde96c13278ee2661df3de8663d48a526d12e6b11b517e1e75b1f2d5a1d72c30a09eccb516cff4bbb14a52a26cdf6b88cee178c65a9fb86b049f2fc7d37cafe3c98bf54d737162df60c60460c478754c30c3d622a471df4f56d5d8e03f04d3528f8e112dcb55550fa4cccbdde1de3718362b0194be9b47901651e974acd21bfc6ec4b5577ee86bdfdc3f68927e5155f3717697690e9d30499be645d

**解密后报文**

- JSON

      {
      "version ":" 4.0.0.0",
      "member_id":"123456",
      "terminal_id":"456789",
      "resp_code":"0000",
      "resp_msg":"交易成功",
      "data_type":"json",
      "biz_type ":"biz_type ",
      " resp_msg ":"交易成功",
      "req_reserved":"预留字段",
      "txn_sub_type":"88",
      "txn_type":"0431",
      "trans_serial_no":"3451592355",
      "trans_id":"1984854162"
      }

  请求响应结果的数据类型根据请求参数字段“data_type”来决定，输出内容包括

### 2.3.2 转账支付订单查询接口

交易URL  
测试环境地址：<https://vgw.baofoo.com/cutpayment/api/backTransRequest>  
正式环境地址：<https://public.baofoo.com/cutpayment/api/backTransRequest>

#### 2.3.2.1 请求报文

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 版本号 | version | M | 4.0.0.0 |
| 02 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 03 | 终端号 | terminal_id | M |  |
| 04 | 交易类型 | txn_type | M | 取值0431 |
| 05 | 交易子类 | txn_sub_type | M | 40 |
| 06 | 加密数据类型 | data_type | M | data_type =xml或json |
| 07 | 加密数据 | data_content | M | 具体参数如下加密数据。 |

**加密数据**

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 交易子类 | txn_sub_type | M | 40 |
| 02 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 03 | 终端号 | terminal_id | M |  |
| 04 | 商户订单号 | trans_id | M | 商户发起的支付订单号 |
| 05 | 商户流水号 | trans_serial_no | M | 8-20 位字母和数字，每次请求都不可重复 |
| 06 | 接入类型 | biz_type | C | 其他：不填写和默认0000,表示为储蓄卡支付。 |
| 07 | 请求方保留域 | req_reserved | O |  |
| 08 | 附加字段 | additional_info | O |  |

#### 2.3.2.2 应答报文

| 序号 | 参数含义 | 参数名称 | 必填 | 参数备注 |
| --- | --- | --- | --- | --- |
| 01 | 应答码 | resp_code | M | 0000表示交易成功，仅代表查询订单状态成功，不代表订单支付成功，具体订单什么状态需要看state的值，具体参考下面state各状态代表的含义 |
| 02 | 应答信息 | resp_msg | M |  |
| 03 | 商户订单号 | trans_id | R | 商户订单号 |
| 04 | 终端号 | terminal_id | R |  |
| 05 | 交易类型 | txn_type | R | 取值0431 |
| 06 | 交易子类 | txn_sub_type | R | 40 |
| 07 | 数据类型 | data_type | R | xml/json |
| 08 | 版本号 | version | O |  |
| 09 | 附加字段 | additional_info | O |  |
| 10 | 预留字段 | req_reserved | O |  |
| 11 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 12 | 商户流水号 | trans_serial_no | R | 8-20 位字母和数字，每次请求都不可重复 |
| 13 | 接入类型 | biz_type | R |  |
| 14 | 订单状态 | state | M | 订单的状态：0：待确认（预下单时的状态），1：成功，2：失败，3：待处理，4：已取消 |
| 15 | 成功时间 | succ_time | O | 仅state=1时即订单已经加值成功的时候才有成功时间,其余状态为空值 |

### 2.3.3 转账支付异步通知

#### 2.3.2.1 功能说明

将结果异步通知给商户

#### 2.3.2.2 通知报文

member_id= 100000178&terminal_id=1000938&data_content=09f27e40024994307c854b5bff54fdbb79e7ed9900e6013e58a816e31ef39088f42417a5d87d05e01508b93bea5be2af8fa0562b8259b07eff3fb61fee70d69a5550561731b6bf1319a98091180490d6fd783af72d20e7bf53e3b924f6455f93d7234fff06fef004f05e6795903e21535a1b19fa75473fc99a8b8ff2aadc146f85bf2bb168ccfa724a77e24f4aea12ed0634b4810feed3c1c799bdbd03b98c8378ca5fa68efd1bccd54d8f9a7ad80912f8e5c29ff74f0b8bef247c4e4319f8778366f06f3b547f9586bf229e3b15bd38523999bb58f9c11fd68df33da5ad5d99f6a528c69b178248804da25b082ab08f9c504a4db70139f1a540614fa21cebb3

**解密后报文**

- Xml格式

      <result>
      <resp_code>0000</resp_code>
      <resp_msg>成功</resp_msg>
      <memberTransId>test0000000000a01</memberTransId>
      <state>1</state>
      <orderMoney>1.01</orderMoney>
      <succTime>20170808143943</succTime>
      </result>

- Json格式

      {
      "resp_code":"0000"
      "resp_msg":"成功"
      "memberTransId":"test0000000000a01",
      "orderMoney":"1.01",
      "state":"1",
      "succTime":"20170808143943"
      }

**加密数据**

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 应答码 | resp_code | M |  |
| 02 | 应答信息 | resp_msg | M |  |
| 03 | 商户订单号 | memberTransId | M |  |
| 03 | 订单金额 | orderMoney | M |  |
| 14 | 状态 | state | M | 1:通过 2:失败 |
| 15 | 成功时间 | succTime | M | yyyyMMddHHmmss |

注：  
宝付将掉单或超时的订单取出，补发支付结果（结果有成功和失败，不是仅仅发支付成功的订单，请商户务必要根据应答码判断支付结果是成功还是失败，再进行正确的处理），以GET和POST方式发送到商户配置的接收地址，商户接收到支付结果，并且进行相应处理之后，需要商户接收通知的地址在页面上输出 OK 表示接收成功\<除了 OK 无任何其他内容\>，告诉宝付已经成功接收并处理完毕，宝付系统在未得到商户接收通知成功的反馈时，将通过重发机制再次通知商户（重发次数 2~10 次，请以第一次收到的支付成功的消息为准，避免进行多次充值或支付），直到商户接收成功或达到最大重发次数为止。

### 2.3.4 确认分账接口

同协议支付-[确认分账类接口](https://doc.mandao.com/docs/bct/bct-1f9qho10m3inj)

### 2.3.5 分账接口查询接口

同协议支付-[分账状态查询](https://doc.mandao.com/docs/bct/bct-1f9qhp1tv3e22)

### 2.3.6 转账支付订单取消接口

#### 2.3.6.1 交易URL

测试环境地址：<https://vgw.baofoo.com/cutpayment/api/backTransRequest>  
正式环境地址：<https://public.baofoo.com/cutpayment/api/backTransRequest>

#### 2.3.6.2 请求报文

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 版本号 | version | M | 4.0.0.0 |
| 02 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 03 | 终端号 | terminal_id | M |  |
| 04 | 交易类型 | txn_type | M | 取值0431 |
| 05 | 交易子类 | txn_sub_type | M | 89 |
| 06 | 加密数据类型 | data_type | M | data_type =xml或json |
| 07 | 加密数据 | data_content | M | 具体参数如下加密数据。 |

**加密数据**

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 交易子类 | txn_sub_type | M | 89 |
| 02 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 03 | 终端号 | terminal_id | M |  |
| 04 | 商户订单号 | trans_id | M | 商户发起的支付订单号 |
| 05 | 商户流水号 | trans_serial_no | M | 8-20 位字母和数字，每次请求都不可重复 |
| 06 | 接入类型 | biz_type | C | 其他：不填写和默认0000,表示为储蓄卡支付 |
| 07 | 请求方保留域 | req_reserved | O |  |
| 08 | 附加字段 | additional_info | O | 长度不超过 128 位 |

#### 2.3.6.3 应答报文

| 序号 | 参数含义 | 参数名称 | 必填 | 参数备注 |
| --- | --- | --- | --- | --- |
| 01 | 应答码 | resp_code | M | 0000表示交易成功，仅代表查询订单状态成功，不代表订单支付成功，具体订单什么状态需要看state的值，具体参考下面state各状态代表的含义 |
| 02 | 应答信息 | resp_msg | M |  |
| 03 | 商户订单号 | trans_id | R | 商户订单号 |
| 04 | 终端号 | terminal_id | R |  |
| 05 | 交易类型 | txn_type | R | 取值0431 |
| 06 | 交易子类 | txn_sub_type | R | 89 |
| 07 | 数据类型 | data_type | R | xml/json |
| 08 | 版本号 | version | O |  |
| 09 | 附加字段 | additional_info | O |  |
| 10 | 预留字段 | req_reserved | O |  |
| 11 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 12 | 商户流水号 | trans_serial_no | R | 8-20 位字母和数字，每次请求都不可重复 |
| 13 | 接入类型 | biz_type | R | 默认0000 |
| 14 | 订单状态 | state | R | 1:取消成功 2:取消失败 |

### 2.3.8 异步通知流水信息

#### 2.3.8.1 功能说明

> 异步通知银行到账流水信息，异步通知以POST方式发送到商户配置的接收地址，商户接收到支付结果，并且进行相应处理之后，需要商户接收通知的地址在页面上输出 OK 表示接收成功\<除了 OK 无任何其他内容\>，告诉宝付已经成功接收并处理完毕，宝付系统在未得到商户接收通知成功的反馈时，将通过重发机制再次通知商户（重发次数 2~10 次，请以第一次收到的支付成功的消息为准，避免进行多次充值或支付），直到商户接收成功或达到最大重发次数为止。

#### 2.3.8.2 应答报文（宝付返回报文）

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 02 | 终端号 | terminal_id | M |  |
| 03 | 交易子类 | txn_sub_type | M | 97 |
| 04 | 商户订单号 | trans_id | M | 商户发起的支付订单号 |
| 05 | 商户流水号 | trans_serial_no | M | 8-20 位字母和数字，每次请求都不可重复 |
| 06 | 接入类型 | biz_type | C | 其他：不填写和默认0000,表示为储蓄卡支付。 |
| 07 | 支付金额 | order_money | M | 单位：分 |
| 08 | 收单金额 | acquiring_money | M | 单位：分。 |
| 09 | 收款方账户号 | payee_acct_code | M | 收款方账户号 |
| 10 | 付款方账户号 | payer_acct_code | M | 付款方账户号 |
| 11 | 付款方开户名 | payer_user_name | M | 付款方开户名 |
| 12 | 回调地址 | call_back_url | O | 如果传入，则异步通知使用该url；如果未传入，则不做异步通知 |
| 14 | 请求方保留域 | req_reserved | O |  |
| 15 | 手续费承担方 | fee_member_id | O |  |
0

**加密数据**

请求响应结果的数据类型根据请求参数字段“data_type”来决定，输出内容包括

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 02 | 终端号 | terminal_id | M |  |
| 03 | 交易子类 | txn_sub_type | M | 97 |
| 04 | 商户订单号 | trans_id | M | 商户发起的支付订单号 |
| 05 | 商户流水号 | trans_serial_no | M | 8-20 位字母和数字，每次请求都不可重复 |
| 06 | 接入类型 | biz_type | C | 其他：不填写和默认0000,表示为储蓄卡支付。 |
| 07 | 支付金额 | order_money | M | 单位：分 |
| 08 | 收单金额 | acquiring_money | M | 单位：分。 |
| 09 | 收款方账户号 | payee_acct_code | M | 收款方账户号 |
| 10 | 付款方账户号 | payer_acct_code | M | 付款方账户号 |
| 11 | 付款方开户名 | payer_user_name | M | 付款方开户名 |
| 12 | 回调地址 | call_back_url | O | 如果传入，则异步通知使用该url；如果未传入，则不做异步通知 |
| 14 | 请求方保留域 | req_reserved | O |  |
| 15 | 手续费承担方 | fee_member_id | O |  |
1

# 附录

## 1 应答码

- 交易成功类

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 02 | 终端号 | terminal_id | M |  |
| 03 | 交易子类 | txn_sub_type | M | 97 |
| 04 | 商户订单号 | trans_id | M | 商户发起的支付订单号 |
| 05 | 商户流水号 | trans_serial_no | M | 8-20 位字母和数字，每次请求都不可重复 |
| 06 | 接入类型 | biz_type | C | 其他：不填写和默认0000,表示为储蓄卡支付。 |
| 07 | 支付金额 | order_money | M | 单位：分 |
| 08 | 收单金额 | acquiring_money | M | 单位：分。 |
| 09 | 收款方账户号 | payee_acct_code | M | 收款方账户号 |
| 10 | 付款方账户号 | payer_acct_code | M | 付款方账户号 |
| 11 | 付款方开户名 | payer_user_name | M | 付款方开户名 |
| 12 | 回调地址 | call_back_url | O | 如果传入，则异步通知使用该url；如果未传入，则不做异步通知 |
| 14 | 请求方保留域 | req_reserved | O |  |
| 15 | 手续费承担方 | fee_member_id | O |  |
2

- 交易结果暂未知，需查询类

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 02 | 终端号 | terminal_id | M |  |
| 03 | 交易子类 | txn_sub_type | M | 97 |
| 04 | 商户订单号 | trans_id | M | 商户发起的支付订单号 |
| 05 | 商户流水号 | trans_serial_no | M | 8-20 位字母和数字，每次请求都不可重复 |
| 06 | 接入类型 | biz_type | C | 其他：不填写和默认0000,表示为储蓄卡支付。 |
| 07 | 支付金额 | order_money | M | 单位：分 |
| 08 | 收单金额 | acquiring_money | M | 单位：分。 |
| 09 | 收款方账户号 | payee_acct_code | M | 收款方账户号 |
| 10 | 付款方账户号 | payer_acct_code | M | 付款方账户号 |
| 11 | 付款方开户名 | payer_user_name | M | 付款方开户名 |
| 12 | 回调地址 | call_back_url | O | 如果传入，则异步通知使用该url；如果未传入，则不做异步通知 |
| 14 | 请求方保留域 | req_reserved | O |  |
| 15 | 手续费承担方 | fee_member_id | O |  |
3

- 交易失败，无需查询类

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 02 | 终端号 | terminal_id | M |  |
| 03 | 交易子类 | txn_sub_type | M | 97 |
| 04 | 商户订单号 | trans_id | M | 商户发起的支付订单号 |
| 05 | 商户流水号 | trans_serial_no | M | 8-20 位字母和数字，每次请求都不可重复 |
| 06 | 接入类型 | biz_type | C | 其他：不填写和默认0000,表示为储蓄卡支付。 |
| 07 | 支付金额 | order_money | M | 单位：分 |
| 08 | 收单金额 | acquiring_money | M | 单位：分。 |
| 09 | 收款方账户号 | payee_acct_code | M | 收款方账户号 |
| 10 | 付款方账户号 | payer_acct_code | M | 付款方账户号 |
| 11 | 付款方开户名 | payer_user_name | M | 付款方开户名 |
| 12 | 回调地址 | call_back_url | O | 如果传入，则异步通知使用该url；如果未传入，则不做异步通知 |
| 14 | 请求方保留域 | req_reserved | O |  |
| 15 | 手续费承担方 | fee_member_id | O |  |
4

## 2 银行编码

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 02 | 终端号 | terminal_id | M |  |
| 03 | 交易子类 | txn_sub_type | M | 97 |
| 04 | 商户订单号 | trans_id | M | 商户发起的支付订单号 |
| 05 | 商户流水号 | trans_serial_no | M | 8-20 位字母和数字，每次请求都不可重复 |
| 06 | 接入类型 | biz_type | C | 其他：不填写和默认0000,表示为储蓄卡支付。 |
| 07 | 支付金额 | order_money | M | 单位：分 |
| 08 | 收单金额 | acquiring_money | M | 单位：分。 |
| 09 | 收款方账户号 | payee_acct_code | M | 收款方账户号 |
| 10 | 付款方账户号 | payer_acct_code | M | 付款方账户号 |
| 11 | 付款方开户名 | payer_user_name | M | 付款方开户名 |
| 12 | 回调地址 | call_back_url | O | 如果传入，则异步通知使用该url；如果未传入，则不做异步通知 |
| 14 | 请求方保留域 | req_reserved | O |  |
| 15 | 手续费承担方 | fee_member_id | O |  |
5

## 3 交易子类

| 序号 | 域名 | 变量名 | 必填 | 备注 |
| --- | --- | --- | --- | --- |
| 01 | 商户号 | member_id | M | 宝付提供给商户的唯一编号 |
| 02 | 终端号 | terminal_id | M |  |
| 03 | 交易子类 | txn_sub_type | M | 97 |
| 04 | 商户订单号 | trans_id | M | 商户发起的支付订单号 |
| 05 | 商户流水号 | trans_serial_no | M | 8-20 位字母和数字，每次请求都不可重复 |
| 06 | 接入类型 | biz_type | C | 其他：不填写和默认0000,表示为储蓄卡支付。 |
| 07 | 支付金额 | order_money | M | 单位：分 |
| 08 | 收单金额 | acquiring_money | M | 单位：分。 |
| 09 | 收款方账户号 | payee_acct_code | M | 收款方账户号 |
| 10 | 付款方账户号 | payer_acct_code | M | 付款方账户号 |
| 11 | 付款方开户名 | payer_user_name | M | 付款方开户名 |
| 12 | 回调地址 | call_back_url | O | 如果传入，则异步通知使用该url；如果未传入，则不做异步通知 |
| 14 | 请求方保留域 | req_reserved | O |  |
| 15 | 手续费承担方 | fee_member_id | O |  |
6

## 4 匹配码规则

规则：2位的商户编码 + 36进制编码（当前时间与基准时间差36进制转大写）  
注：1.商户编码由宝付生成并提供，请联系宝付技术支持

    2. 36进制编码规则及demo请联系宝付技术支持

## 5 订单重复性校验

宝付为了确保交易能够准确的通知到商户，有可能会重复发送通知消息，为此宝付提醒商户，采取正确的防重复校验。  
大多数校验通知都采取先查询后更新的方式，这种方式存在一个很大的漏洞，当多个通 知请求在很短的一个时间内达到时，查询数据有可能是脏数据，导致订单重复更新，后续工 作重复执行。

对于此种情况，宝付结合自身校验情况，分享两个校验方式。  
1) 如果是单线程或者订单资源在一个共享区域，那么可以采取锁定资源的方式，每次 调用加锁，每次调用完毕解锁。  
2) 大部分数据都存在数据库，宝付绝大多数都是这种情况，利用数据库的特性来控制， 在我们调用数据库更新信息时，数据库会返回给我们更新条目数，我们利用这个特性，这样 操作，当我们更新订单时，把这个订单的原始状态作为条件进行更新，当我们短时间内操作 更新时，第二次更新必然不成功，因为条件不满足了，状态变了，那么数据库在第二次就会 返回更新数量为0，这样我们就发现问题了，这就是熟称的“乐观锁”。

文档更新时间: 2026-02-04 07:03   作者：超级管理员
