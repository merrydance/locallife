---
title: 云闪付无感支付签约
slug: bct-1fao053hgjkir
source_url: https://doc.mandao.com/docs/bct/bct-1fao053hgjkir
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 66777659dcf6e9ea037e7ac488750ce24532b1593571fa6ad0f6da18693972bd
doc_version: 1705457082
---

# 云闪付无感支付签约
# 接口说明

| 接口名称 | sign_contract |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 是 |

# 应用场景

## **1.云闪付微信小程序签约**

签约交易是指商户在微信小程序上跳转云闪付微信小程序进行签约，不涉及支付。支付请使用“[统一下单交易创建](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037643a5cd5)”接口。

### 云闪付小程序签约流程

![null](https://doc.mandao.com/uploads/pay-doc/images/m_921ebb279b9b62beddee22ebbceb3c68_r.png)

**微信小程序跳转方法请参考微信官方说明。**

> [https://developers.weixin.qq.com/miniprogram/dev/api/navigate/wx.navigateToMiniProgram.html](https://developers.weixin.qq.com/miniprogram/dev/api/navigate/wx.navigateToMiniProgram.html)

## **2.云闪付APP签约**

签约流程参考云闪付微信小程序签约。

# 注意事项

签约结果可通过异步通知或查询结果获取。

# 接口参数

> - 请求参数：

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 交易商户号 | merId | 是 | S(16) | 123456987 | 宝付支付分配的商户号 |
| 交易终端号 | terId | 是 | S(16) | 123456789 | 宝付支付分配的终端号 |
| 商户订单号 | outTradeNo | 是 | S(32) | 1234323JKHDFE1243252 | 商户系统内部服务订单号，要求32个字符内，且在同一个商户号下唯一 |
| 交易时间 | txnTime | 是 | T | 20210315155012 | 订单交易时间 |
| 订单有效时间 | timeExpire | 否 | I | 30 | 订单支付的有效时间，单位：分钟，不传此参数则宝付支付默认有效时间30分钟 |
| 签约模板ID | planId | 是 | S(32) | 1234323JKHDFE1243252 | 签约模板ID，与接入产品对应，入网时生成 |
| 禁止贷记卡支付 | forbidCredit | 是 | S(1) | 0 | 1：禁止0：不禁止不传默认为0 |
| 交易发起场景 | invokeScene | 是 | E | 03 | 详见附录【[交易发起场景](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f038a3vq0cv9)】 |
| 请求渠道参数 | reqExtend | 是 | C | | 请求渠道参数，字段详细说明参考下文 / 请求渠道参数：reqExtend。 |
| 业务扩展参数 | bizExtend | 否 | C | | 预留字段 |
| 商户回调地址 | notifyUrl | 是 | S(256) | [https://www.example.com/return_url](https://www.example.com/return_url) | 商户接收签约成功回调通知的地址 |

## 请求渠道参数：reqExtend

> - 签约公共参数：
> - 公共参数适用于APP签约、小程序签约、H5签约

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 订单描述 | orderDesc | 否 | S(32) | 描述订单信息 | 描述订单信息，显示在银联支付控件或客户端支付界面中 |
| 二级商户代码 | subMerId | 否 | S(15) | SUB464887 | 商户类型为平台类商户接入时必须上送 |
| 二级商户全称 | subMerName | 否 | S(40) | 白云科技有限公司 | 商户类型为平台类商户接入时必须上送 |
| 二级商户简称 | subMerAbbr | 否 | S(16) | 白云科技 | 商户类型为平台类商户接入时必须上送 |
| 订单详细信息（支持单品） | acqAddnData | 否 | C | {“goodsInfo”:[{“id”:”1234567890”,”name”:”商品1”,”price”:”500”,”quantity”:”1”},{“id”:”1234567891”,”name”:”商品2”,”price”:”1000”,”quantity”:”2”,”category”:”类目1”,”addnInfo”: “商品图片[http://www.test.com/xxx.jpg"}]}](http://www.test.com/xxx.jpg"}]}) | 详见：订单详细信息：acqAddnData / JSON格式 |
| 风险信息域 | riskRateInfo | 否 | C | {“riskRateInfo”:”000”,”shippingCountryCode”:”商品1”,”shippingProvinceCode”:”86”,”shippingCityCode”:”021”,”shippingDistrictCode”:”021”,”shippingStreet”:”陆家嘴”,”commodityName”:”商品名称”} | 详见：风控信息域：riskRateInfo / JSON数组格式 |

> - 小程序签约场景需上送

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 商户小程序id | merWxMpAppId | 是 | S(32) | 79797897888 | 签约场景为小程序时必填，用于签约完成/失败/取消后跳转 / 小程序ID必须为在银联入网时报备的ID，否则跳转至云闪付微信小程序页面会报错 |
| 商户小程序path | merWxMpPath | 是 | S(512) | [http://test.com](http://test.com) | 签约场景为小程序时必填，用于签约完成/失败/取消后跳转 |

> - H5签约场景需上送

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| APP应用scheme | scheme | 否 | S(128) | | 外部APP通过WAP打开调起支付控件时上送 |
| APP应用包名 | packageName | 否 | S(128) | | 外部APP通过WAP打开调起支付控件时上送 |
| 前台通知地址 | frontUrl | 是 | S(256) | [https://www.example.com/return_url](https://www.example.com/return_url) | 前台返回商户结果时使用，前台类交易需上送 |
| 失败交易前台跳转地址 | frontFailUrl | 否 | S(256) | [https://www.example.com/return_url](https://www.example.com/return_url) | 前台交易若商户上送此字段，则在交易失败时，页面跳转至商户该URL（不带交易信息，仅跳转） |
| 持卡人IP | customerIp | 否 | S(40) | 127.0.0.1 | 前台交易，有IP防钓鱼要求的商户上送 |

## 订单详细信息：acqAddnData

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 商品信息 | goodsInfo | 否 | C | {“goodsInfo”:[{“id”:”1234567890”,”name”:”商品1”,”price”:”500”,”quantity”:”1”},{“id”:”1234567891”,”name”:”商品2”,”price”:”1000”,”quantity”:”2”,”category”:”类目1”,”addnInfo”: “商品图片[http://www.test.com/xxx.jpg"}]}](http://www.test.com/xxx.jpg"}]}) | 详见：商品信息：goodsInfo / JSON数组格式 |

## 商品信息：goodsInfo

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 商品编号 | id | 否 | S(32) | 7897989 | 商品编号 |
| 商品名称 | name | 否 | S(256) | 商品1 | 商品名称 |
| 商品单价 | price | 否 | I | 500 | 以分为单位 |
| 商品数量 | quantity | 否 | I | 2 | 商品数量 |
| 商品类目 | category | 否 | S(24) | 类目1 | 商品类目 |
| 附加信息 | addnInfo | 否 | S(100) | 商品图片[http://www.test.com/xxx.jpg](http://www.test.com/xxx.jpg) | 内容自定义 |

## 风控信息域：riskRateInfo

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 商品风险类别标识 | shippingFlag | 否 | S(3) | 111 | 111：虚拟高风险类（无物流、非实名登记、易变现如：游戏点卡、游戏装备、手机充值、礼品卡、虚拟账户充值） / 110：虚拟低风险类（无物流、非实名登记、不易变现如：电影票、信息咨询）100：虚拟实名类（无物流、实名登记、不易变现如：航空售票、酒店预订、旅游产品、学费、行政费用（税费、车船使用费）、汽车、房产） / 001：实物高风险类（有物流、易变现如：数码家电、黄金、珠宝首饰等） / 000：实物低风险类（有物流、不易变现如：服饰、食品、日用品等） |
| 收货地址-国家 | shippingCountryCode | 否 | S(3) | | 国家代码 |
| 收货地址-省 | shippingProvinceCode | 否 | S(6) | | 省份代码 |
| 收货地址-市 | shippingCityCode | 否 | S(6) | | 市区代码 |
| 收货地址-地区 | shippingDistrictCode | 否 | S(6) | | 地区代码 |
| 收货地址-详细 | shippingStreet | 否 | S(256) | | 详情地址 |
| 商品种类 | commodityCategory | 否 | S(4) | | 区分商品类别 |
| 商品名称 | commodityName | 否 | S(256) | | 商品名称 |
| 商品URL | commodityUrl | 否 | S(1024) | [http://test.com](http://test.com) | 商品URL |
| 商品单价 | commodityUnitPrice | 否 | I | 20 | 商户单价 |
| 商品数量 | commodityQty | 否 | I | 10 | 商品数量 |
| 收货/订单手机号 | shippingMobile | 否 | S(20) | 18895263214 | 收货人手机号 |
| 订单地址最后修改时间 | addressModifyTim | 否 | T | 20230315155012 | |
| 用户注册时间 | userRegisterTime | 否 | T | 20230315155012 | |
| 收货（订单）姓名的最后修改时间 | orderNameModifyTime | 否 | T | 20230315155012 | |
| 账户ID | userId | 否 | S(128) | user123648999 | |
| 收货/订单姓名 | orderName | 否 | S(32) | 欧阳 | |
| 优质用户标识码 | userFlag | 否 | E | 0 | 0普通用户，1-优质用户 |
| 订单手机号最后修改时间 | mobileModifyTime | 否 | T | 20230315155012 | |
| 风险级别 | riskLevel | 否 | E | 0 | 基于绑定关系的支付交易时使用：0：无风险业务1：有风险业务 |
| 商户端用户ID | merUserId | 否 | S(64) | user12364477 | |
| 商户端用户注册时间 | merUserRegDt | 否 | T | 20230315 | |
| 商户端用户注册邮箱 | merUserEmail | 否 | S(256) | [test_email@qq.com](mailto:test_email@qq.com) | |
| 硬盘序列号 | diskSep | 否 | S(64) | 7979879899879 | 1.持卡人支付时的存储设备的硬盘序列号 / 2.终端硬件序列号 |
| IMEI | imei | 否 | S(64) | 798789798797 | 持卡人支付时手机设备的IMEI |
| MAC地址 | macAddr | 否 | S(17) | 00-02-0F-ED-01-03 | 持卡人支付时使用设备的MAC地址 |
| LBS信息 | lbs | 否 | S(32) | +37.12/-121.23 | 1.空中发卡时的位置信息，经纬度，格式为纬度/经度，+表示北纬、东经，-表示南纬、西经。举例：+37.12/-121.23或者+37/-121 / 2.sourceIP、lbs、fullDeviceNumber这三要素建议至少上送一个 |
| 设备通讯号码 | deviceNumber | 否 | S(32) | | 1.终端拨号号码 / 2.单个手机号,可能包含前缀（发起交易的手机号码，不是接收验证码的手机号） |
| 设备类型 | deviceType | 否 | E | 1 | 设备类型：1.Phone / 2. Pad / 3. iWatch / 4.PC |
| 卡片信息录入方式 | captureMethod | 否 | E | 1 | 卡号录入方式，例如：1.camera：表示摄像头捕捉得到卡号 / 2.manual：用户手输入卡号 / 3.nfc：nfc方式读取卡号 / 4.unknow：未知的获取卡号方式。经手工修改卡号后，均应填写为manual，表示手工输入。 |
| 设备sim卡数量 | simCardCount | 否 | I | 100 | |
| 设备名称 | deviceName | 否 | S(128) | POS机 | |
| 设备标识 | deviceID | 否 | S(64) | | 移动终端设备的唯一标识 |
| 银行预留手机号 | mobile | 否 | S(20) | 1859638795 | 银行卡预留手机号码仅1个，不包括+86等信息 |
| 应用提供方账户ID | accountIdHash | 否 | S(64) | | 用来标识用户在智能设备上登录账号ID信息的哈希值，与用户登录账号ID是一一对应关系，为登录账号ID的替换值。 |
| 设备SIM卡号码 | fullDeviceNumber | 否 | S(32) | | 1.持卡人用来做设备卡加载时所使用设备的号码，多个号码用逗号隔开。 / 2.sourceIP、lbs、fullDeviceNumber这三要素建议至少上送一个 |
| IP | sourceIP | 否 | S(64) | 192.168.1.1 | 1.必送（IP、设备GPS位置、设备SIM卡号码，这三要素至少上送一个） / 2.sourceIP、lbs、fullDeviceNumber这三要素建议至少上送一个 |
| 设备使用语言 | deviceLanguage | 否 | S(3) | | 移动支付设备所设定的使用语言，语言代码取值遵从ISO639-3标准。 |
| 账户关键信息修改时间 | accountEmailLife | 否 | I | 12 | 设备用户重要信息修改时间，最近一次修改email距今X个月，X的数值范围：0-24，表示0-24个月，大于24个月赋值24。 |
| 持卡人姓名 | cardHolderName | 否 | S(256) | 三李 | 持卡人姓名，名在前，姓在后。 |
| 持卡人账单地址 | billingAddress | 否 | S(256) | | 用户账单地址信息。 |
| 持卡人邮编 | billingZip | 否 | S(6) | | 用户账单邮编信息。 |
| 总体风险评级 | riskScore | 否 | I | 5 | 风险评级, 1-5分，5分最高。 |
| 风险评级版本号 | riskStandardVersion | 否 | S(8) | | 设备厂商给出加载流程风险建议时所基于的风险判断原则对应的版本。 |
| 设备评级 | deviceScore | 否 | I | 3 | 设备厂商给设备的评分，1-5分，5分可信度越高。 |
| 账户评级 | accountScore | 否 | I | 9 | 设备厂商给用户账户的评分，取值从0到9。 |
| 设备SIM卡号码评级 | phoneNumberScore | 否 | I | 3 | 设备号码评分，加载流程对应手机号信任评级级别，取值从1到5。 |
| 评级原因码 | riskReasonCode | 否 | S(100) | 否 | |
| 绑卡渠道 | applyChannel | 否 | E | 01 | 01:银行自有渠道 / 02:非银行渠道 |
| 第三方风险总体评分 | thirdPartyRiskScore | 否 | I | 1025 | 第三方提供的细化风险总体评分，最多可支持5位数字 |
| 第三方设备标识 | thirdPartyDeviceId | 否 | S(100) | | 第三方提供的设备标识 |
| 第三方建议 | thirdPartyAdvise | 否 | S(5) | | 第三方针对当前交易风险情况提供的处置建议 |
| 设备型号 | deviceMode | 否 | S(256) | | 设备型号 |
| 安全载体发行方 | safeCarrIss | 否 | S(16) | 第三方 | 安全载体发行方 |
| 设备指纹ID | seId | 否 | S(64) | | |
| CSN | csn | 否 | S(64) | | 移动支付部交易专用，由移动支付部为SD卡，SIM卡，读卡器统一分配的唯一ID号，用于识别卡片载体 |

> - 返回参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 交易商户号 | merId | 是 | S(16) | 123456987 | 宝付支付分配的商户号 |
| 交易终端号 | terId | 是 | S(16) | 123456789 | 宝付支付分配的终端号 |
| 商户订单号 | outTradeNo | 是 | S(32) | 1234323JKHDFE1243252 | 调用接口传入的商户订单号 |
| 宝付交易号 | tradeNo | 否 | S(32) | 230904111102220002521469 | 与商户订单号对应的宝付侧唯一交易号 |
| 订单状态 | orderState | 是 | E | SUCCESS | 详见：[订单状态](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f037v64h9dd5)-云闪付签约状态 |
| 渠道返回参数 | respExtend | 是 | C | | 字段详细说明参考下文 / 请求渠道参数：reqExtend。 |
| 业务扩展参数 | bizExtend | 否 | C | | 预留字段 |
| 业务结果 | resultCode | 是 | S(16) | SUCCESS | 业务处理结果 |
| 错误代码 | errCode | 否 | S(32) | SYSTEM_BUSY | 当业务结果FAIL时，返回错误代码 |
| 错误描述 | errMsg | 否 | S(128) | 系统繁忙，请稍后再试 | 当业务结果为FAIL时，返回错误描述 |

## 渠道返回参数：respExtend

> - 公共参数：
> - 公共参数适用于APP签约、小程序签约、H5签约

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 银联受理订单号 | tn | 否 | S(64) | 0000300001201908301055157220022 | 商户调用支付控件时使用 |

> - 小程序签约

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 云闪付小程序id | cqpMpAppId | 预支付交易会话标识 | 否 | S(64) | 用于商户跳转进行签约 |
| 云闪付小程序path | cqpMpPath | 微信调用数据 | 否 | S(1028) | 用于商户跳转进行签约 |

文档更新时间: 2024-01-17 02:04 作者：超级管理员
