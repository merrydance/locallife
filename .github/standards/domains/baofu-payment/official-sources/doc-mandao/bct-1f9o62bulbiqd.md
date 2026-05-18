---
title: 报备认证
slug: bct-1f9o62bulbiqd
source_url: https://doc.mandao.com/docs/bct/bct-1f9o62bulbiqd
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: e5e54e509b42db4820224f586016d7df484cb47872cd42a70871662cb66d126a
doc_version: 1777270832
---

# 报备认证
# 接口说明

| 接口名称 | merchant_report |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 否 |

# 应用场景

收单机构通过此接口报备终端信息，银联后续会对交易中上送的终端信息进行校验。

# 接口参数

## 请求参数：

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 交易商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 交易终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 报备类型 | reportType | 是 | E | WECHAT | 详见附录：《报备类型》 |
| 报备编号 | reportNo | 是 | S(64) | 20211220120030798 | 每次请求报备接口唯一编号 |
| 报备信息 | reportInfo | 是 | C |  | JSON格式，根据传入报备类型，查看相应的报备信息说明 / 详见：[报备信息参数：reportInfo] |
| 宝财通二级商户号 | bctMerId | 是 | S(64) | CM1234567890 | 宝财通二级用户客户号 |

### 报备信息参数：reportInfo

#### 微信参数:

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 商户名称 | merchant_name | 是 | S(50) | 商户名称 | 该名称是公司主体全称 |
| 商户简称 | merchant_shortname | 是 | S(20) | 商户简称 | 该名称是显示给消费者看的商户名称 |
| 客服电话 | service_phone | 是 | S(20) | 075586010000 | 方便银联在必要时能联系上商家 |
| 联系人 | contact | 否 | S(10) | 联系人 | 同上 |
| 联系电话 | contact_phone | 否 | S(11) | 13000000000 | 同上 |
| 联系邮箱 | contact_email | 否 | S(30) | [test@test.com](mailto:test@test.com) | 同上 |
| 渠道商商户号 | channel_id | 是 | S(32) | 10100000 | 收单机构为其渠道商申请,测试环境传：148717784 |
| 渠道商商户名称 | channel_name | 是 | S(50) | 渠道商商户名称 | 渠道商商户名称，测试环境传：宝财通收单商户有限公司 |
| 经营类目 | business | 是 | S(10) | 101 | [行业类目](https://doc.mandao.com/attach_files/bct/288)，请填写对应的ID,测试环境传:758-2 |
| 联系人微信账号类型 | contact_wechatid_type | 否 | S(32) | type_wechatid | 如传微信号，值为 type_wechatid |
| 联系人微信帐号 | contact_wechatid | 否 | S(32) | OPENID_01231232 | 微信号 |
| 申请服务 | service_codes | 是 | C | [“JSAPI”,”APPLET”] | 申请服务，可传送所有需要开通的服务，详见《微信服务类型》，JSON数组格式 |
| 地址信息 | address_info | 是 | C | - | 地址信息，JSON格式 / 详见:[地址信息：address_info] |
| 商户证件编号 | business_license | 是 | S(20) | 100000011234561 | 商户证件编号（企业或者个体工商户提供营业执照，事业单位提供事证号，小微商户提供身份证号） |
| 商户证件类型 | business_license_type | 是 | S(32) | NATIONAL_LEGAL | 商户证件类型，取值范围详见《微信证件类型》枚举 |
| 银行结算卡信息 | bankcard_info | 是 | C |  | 商户对应银行所开立的结算卡信息，JSON格式 / 详见：[银行卡信息：bankcard_info] |

##### 地址信息：address_info

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 城市编码 | city_code | 是 | S(6) | 510800 | 城市编码是与国家统计局一致，请查询 :[https://dmfw.mca.gov.cn/XzqhVersionPublish.html](https://dmfw.mca.gov.cn/XzqhVersionPublish.html) |
| 区县编码 | district_code | 是 | S(1) | 510812 | 区县编码是与国家统计局一致，请查询 : [https://dmfw.mca.gov.cn/XzqhVersionPublish.html](https://dmfw.mca.gov.cn/XzqhVersionPublish.html) |
| 省份编码 | province_code | 是 | S(10) | 510000 | 省份编码是与国家统计局一致，请查询 : [https://dmfw.mca.gov.cn/XzqhVersionPublish.html](https://dmfw.mca.gov.cn/XzqhVersionPublish.html) |
| 详细地址 | address | 是 | S(64) |  | 商户详细经营地址或人员所在地点 |
| 经度 | longitude | 否 | S(11) | - | 浮点型, 小数点后最多保留 6 位。如需要录入经纬度，请以高德坐标系为准，录入时请确保经纬度参数准确。高德经纬度查询 [http://lbs.amap.com/console/show/picker](http://lbs.amap.com/console/show/picker) |
| 纬度 | latitude | 否 | S(10) | - | 同上 |
| 地址类型 | type | 否 | S(32) |  | 地址类型，取值范围：BUSINESS_ADDRESS：经营地址（默认） |

##### 银行卡信息：bankcard_info

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 银行卡号 | card_no | 是 | S(48) | 6228480402637874213 | 银行卡号 |
| 银行卡持卡人姓名 | card_name | 是 | S(32) | 张三 | 银行卡持卡人姓名 |
| 银行开户行名称 | bank_branch_name | 否 | S(32) | 招商银行杭州高新支行 | 银行开户行名称 |

#### 支付宝参数:

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 商户名称 | name | 是 | S(128) | 商户名称 | 该名称是公司主体全称 |
| 商户简称 | alias_name | 是 | S(64) | 商户简称 | 商户简称 |
| 客服电话 | service_phone | 是 | S(64) | 075586010000 | 商户客服电话 |
| 商户经营类目 | category_id | 否 | S(32) | 2015050700000000 | 商户经营类目 |
| 标准商户类别码 | mcc | 是 | S(4) | 5976 | [类别码](https://doc.mandao.com/attach_files/bct/288),例如5976表示“专业销售-药品医疗-康复和身体辅助用品”,测试环境传:5411 |
| 支付宝pid | source | 是 | S(20) | 2088100020003000 | 支付宝pid。测试环境传：2088100020003000 |
| 支付宝pid名称 | source_name | 是 | S(64) | 支付宝pid对应的名称 | 支付宝pid对应的名称，测试环境传：宝财通收单商户有限公司 |
| 商户证件编号 | business_license | 否 | S(20) | 100000011234561 | 商户证件编号（企业或者个体工商户提供营业执照，事业单位提供事证号）注：business_license 与 contact_info.id_card_no 两字段至少上送一个。 |
| 商户证件类型 | business_license_type | 否 | S(32) | NATIONAL_LEGAL | 商户证件类型，与商户证件编号（business_license）同时出现，商户证件类型，取值范围详见《证件类型》枚举 |
| 商户联系人信息 | contact_info | 是 | C | - | 商户联系人信息，JSON格式 / 详见：[商户联系人信息：contact_info] |
| 地址信息 | address_info | 是 | C | - | 地址信息,JSON格式 / 详见：[地址信息：address_info] |
| 银行结算卡信息 | bankcard_info | 是 | C |  | 商户对应银行所开立的结算卡信息，JSON格式 / 详见：[银行卡信息：bankcard_info] |
| 支付二维码信息 | pay_code_info | 否 | C | - | 商户的支付二维码中信息，用于营销活动，JSON数组 |
| 支付宝账号 | logon_id | 否 | C | [“[user@domain.com](mailto:user@domain.com)”] | 商户的支付宝账号，JSON数组 |
| 备注信息 | memo | 否 | S(512) | - | 商户备注信息 |
| 申请服务 | service_codes | 否 | C | [“F2F”] | 申请服务，默认情况下开通F2F服务，可传送所有需要开通的服务。允许同时申请多个服务，各服务的准入验证相互独立，服务申请实时生效；详见《支付宝服务类型》,JSON数组 |
| 网站信息 | site_info | 否 | C | - | service_codes包含PC和APP时必填,PC必传 site_url，APP必传 site_name, JSON格式 / 详见：[网站信息：site_info] |
| 间连商户等级 | indirect_level | 是 | S(32) | - | 详见《间连等级》枚举 |

##### 商户联系人信息：contact_info

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 姓名 | name | 是 | S(128) | 张三 | 联系人名字 |
| 电话 | phone | 否 | S(20) | 0571-85022088 | 固定电话 |
| 手机号 | mobile | 否 | S(20) | 13888888888 | 手机号 |
| 邮箱 | email | 否 | S(128) | [test@test.com](mailto:test@test.com) | 邮箱 |
| 联系人业务标识 | tag | 是 | C | [“06”,”08”] | 表示商户联系人的职责。详《联系人业务标识枚举》，JSON数组 |
| 联系人类型 | type | 是 | S(20) | AGENT | 商户联系人业务标识枚举，表示商户联系人的职责。详《联系人类型》枚举 |
| 身份证号 | id_card_no | 否 | S(32) | 110000199001011234 | business_license 与 contact_info.id_card_no两字段至少上送一个 |

##### 地址信息：address_info

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 城市编码 | city_code | 是 | S(10) | - | 城市编码是与国家统计局一致，请查询： ::[https://www.mca.gov.cn/n156/n186/index.html](https://www.mca.gov.cn/n156/n186/index.html) |
| 区县编码 | district_code | 是 | S(10) | - | 区县编码是与国家统计局一致，请查询 : :[https://www.mca.gov.cn/n156/n186/index.html](https://www.mca.gov.cn/n156/n186/index.html) |
| 地址 | address | 是 | S(256) |  | 商户详细经营地址或人员所在地点 |
| 省份编码 | province_code | 是 | S(10) |  | 省份编码是与国家统计局一致，请查询 : :[https://www.mca.gov.cn/n156/n186/index.html](https://www.mca.gov.cn/n156/n186/index.html) |
| 经度 | longitude | 否 | S(11) | - | 浮点型, 小数点后最多保留 6 位。如需要录入经纬度，请以高德坐标系为准，录入时请确保经纬度参数准确。高德经纬度查询 [http://lbs.amap.com/console/show/picker](http://lbs.amap.com/console/show/picker) |
| 纬度 | latitude | 否 | S(10) | - | 同上 |
| 地址类型 | type | 否 | S(32) |  | 地址类型，取值范围：BUSINESS_ADDRESS：经营地址（默认） |

##### 银行卡信息：bankcard_info

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 银行卡号 | card_no | 是 | S(48) | 6228480402637874213 | 银行卡号 |
| 银行卡持卡人姓名 | card_name | 是 | S(128) | 张三 | 银行卡持卡人姓名 |
| 银行开户行名称 | bank_branch_name | 否 | S(512) | 招商银行杭州高新支行 | 银行开户行名称 |

##### 网站信息：site_info

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 站点类型 | site_type | 是 | S(32) | - | 详见《站点类型》枚举 |
| 站点地址 | site_url | 否 | S(256) | - | 站点地址 |
| 站点名称 | site_name | 否 | S(512) | - | 站点名称 |
| 账号 | account | 否 | S(128) | - | 账号 |
| 密码 | password | 否 | S(128) | - | 密码 |

## 返回参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 交易商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 交易终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 报备类型 | reportType | 是 | E | WECHAT | 详见附录：《报备类型》 |
| 报备编号 | reportNo | 是 | S(64) | 20211220120030798 | 每次请求报备接口唯一编号 |
| 报备信息 | reportInfo | 是 | C |  | JSON格式，根据传入报备类型，查看相应的报备信息说明 / 详见：[报备信息参数：reportInfo] |
| 宝财通二级商户号 | bctMerId | 是 | S(64) | CM1234567890 | 宝财通二级用户客户号 |
0

**附件**

- [经营类目&MCC.xlsx](https://doc.mandao.com/attach_files/bct/288 "经营类目&MCC.xlsx")

文档更新时间: 2026-04-27 06:20   作者：超级管理员
