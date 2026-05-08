---
title: 报备信息修改
slug: bct-1f9o62opbejct
source_url: https://doc.mandao.com/docs/bct/bct-1f9o62opbejct
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: a5060b9e65c114c8145978100e4d5ed3fe096484941d2babd80881ec10850f29
doc_version: 1737085567
---

# 报备信息修改
# 接口说明

| 接口名称 | merchant_report_modify |
| --- | --- |
| 是否幂等 | 否 |
| 接口模式 | 直连 |
| 异步通知 | 否 |

# 应用场景

收单机构通过此接口更新终端信息，银联后续会对交易中上送的终端信息进行校验。

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
| 商户识别码 | subMchId | 是 | S(30) |  | 微信/支付宝分配的商户识别码 |
| 报备信息 | reportInfo | 是 | C |  | JSON格式，根据传入报备类型，查看相应的报备信息说明 / 详见：[报备信息参数：reportInfo] |

### 微信reportInfo参数:

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 渠道商商户号 | channel_id | 是 | S(32) | 10100000 | 收单机构为其渠道商申请 |
| 渠道商商户名称 | channel_name | 是 | S(50) | 收单服务商名称 | 收单服务商名称 |
| 商户名称 | merchant_name | 否 | S(50) | 商户名称 | 该名称是公司主体全称，修改商户简称时不需要填写 |
| 商户简称 | merchant_shortname | 是 | S(20) | 商户简称 | 该名称是显示给消费者看的商户名称 |
| 客服电话 | service_phone | 是 | S(20) | 075586010000 | 方便银联在必要时能联系上商家 |
| 申请服务 | service_codes | 否 | C | [“JSAPI”,”APPLET”] | 申请服务，可传送所有需要开通的服务，详见《微信服务类型》 |
| 地址信息 | address_info | 否 | C | - | 地址信息，JSON格式 / 详见：[地址信息：address_info] |
| 商户证件编号 | business_license | 否 | S(20) | 100000011234561 | 商户证件编号（企业或者个体工商户提供营业执照，事业单位提供事证号，小微商户提供身份证号） |
| 商户证件类型 | business_license_type | 否 | S(32) | NATIONAL_LEGAL | 商户证件类型，取值范围详见《微信证件类型》枚举 |
| 银行结算卡信息 | bankcard_info | 否 | C |  | 商户对应银行所开立的结算卡信息，JSON格式 / 详见：[银行结算卡信息：bankcard_info] |
| 商户状态 | merchant_state | 否 | S(2) | 00 | 详见《商户状态》枚举 |
| 交易控制位 | pay_ctrl | 否 | S(2) | 01 | 详见《交易控制位》枚举 |

#### 地址信息：address_info

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 城市编码 | city_code | 是 | S(20) | - | 城市编码是与国家统计局一致，请查询 :[http://www.stats.gov.cn/sj/tjbz/tjyqhdmhcxhfdm/2022/index.html](http://www.stats.gov.cn/sj/tjbz/tjyqhdmhcxhfdm/2022/index.html) |
| 区县编码 | district_code | 是 | S(20) | - | 区县编码是与国家统计局一致，请查询 : [http://www.stats.gov.cn/sj/tjbz/tjyqhdmhcxhfdm/2022/index.html](http://www.stats.gov.cn/sj/tjbz/tjyqhdmhcxhfdm/2022/index.html) |
| 地址 | address | 是 | S(512) |  | 商户详细经营地址或人员所在地点 |
| 省份编码 | province_code | 是 | S(20) |  | 省份编码是与国家统计局一致，请查询 : [http://www.stats.gov.cn/sj/tjbz/tjyqhdmhcxhfdm/2022/index.html](http://www.stats.gov.cn/sj/tjbz/tjyqhdmhcxhfdm/2022/index.html) |
| 经度 | longitude | 否 | S(11) | - | 浮点型, 小数点后最多保留 6 位。如需要录入经纬度，请以高德坐标系为准，录入时请确保经纬度参数准确。高德经纬度查询 [http://lbs.amap.com/cons](http://lbs.amap.com/cons) ole/show/picker |
| 纬度 | latitude | 否 | S(10) | - | 同上 |
| 地址类型 | type | 否 | S(32) |  | 地址类型，取值范围：BUSINESS_ADDRESS：经营地址（默认） |

#### 银行结算卡信息：bankcard_info

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 银行卡号 | card_no | 是 | S(48) | 6228480402637874213 | 银行卡号 |
| 银行卡持卡人姓名 | card_name | 是 | S(128) | 张三 | 银行卡持卡人姓名 |
| 银行开户行名称 | bank_branch_name | 否 | S(512) | 招商银行杭州高新支行 | 银行开户行名称 |

### 支付宝reportInfo参数:

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 商户名称 | name | 是 | S(128) | 商户名称 | 该名称是公司主体全称 |
| 商户简称 | alias_name | 是 | S(64) | 商户简称 | 商户简称 |
| 客服电话 | service_phone | 是 | S(64) | 075586010000 | 商户客服电话 |
| 支付宝pid | source | 是 | S(20) | 2088100020003000 | 支付宝pid |
| 支付宝pid名称 | source_name | 是 | S(64) | 支付宝pid对应的名称 | 支付宝pid对应的名称 |
| 商户证件编号 | business_license | 否 | S(20) | 100000011234561 | 商户证件编号（企业或者个体工商户提供营业执照，事业单位提供事证号）注：business_license 与 contact_info.id_card_no 两字段至少上送一个。 |
| 商户证件类型 | business_license_type | 否 | S(32) | NATIONAL_LEGAL | 商户证件类型，与商户证件编号（business_license）同时出现，商户证件类型，取值范围详见《支付宝证件类型》枚举 |
| 商户联系人信息 | contact_info | 否 | C | - | 商户联系人信息，JSON格式 / 详见：[商户联系人信息：contact_info] |
| 地址信息 | address_info | 否 | C | - | 地址信息，JSON格式 / 详见：[地址信息：address_info] |
| 银行结算卡信息 | bankcard_info | 否 | C |  | 商户对应银行所开立的结算卡信息，JSON格式 / 详见：[银行结算卡信息：bankcard_info] |
| 支付二维码信息 | pay_code_info | 否 | C | - | 商户的支付二维码中信息，用于营销 |
| 活动,JSON数组 |  |  |  |  |  |
| 支付宝账号 | logon_id | 否 | C | [user@domain.com](mailto:user@domain.com) | 商户的支付宝账号,JSON格式数组 |
| 商户经营类目 | category_id | 否 | S(32) | 2015050700000000 | 商户经营类目 |
| 备注信息 | memo | 否 | S(512) | - | 商户备注信息 |
| 标准商户类别码 | mcc | 是 | S(4) | 5976 | 例如5976表示“专业销售-药品医疗-康复和身体辅助用品” |
| 申请服务 | service_codes | 否 | C | [“F2F”] | 申请服务，详见《支付宝服务类型》,JSON数组 |
| 网站信息 | site_info | 否 | C | - | service_codes 包含 PC 和 APP 时必填,PC 必传 site_url，APP必传 site_name，JSON格式 / 详见：[网站信息：site_info] |
| 间连商户等级 | indirect_level | 否 | S(32) | - | 详见《间连等级》枚举 |
| 商户状态 | merchant_state | 否 | S(2) | 00 | 注销状态的商户，详见《商户状态》枚举 |
| 交易控制位 | pay_ctrl | 否 | S(2) | 01 | 详见《交易控制位》枚举 |

#### 商户联系人信息：contact_info

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 姓名 | name | 是 | S(128) | 张三 | 联系人名字 |
| 电话 | phone | 否 | S(20) | 0571-85022088 | 固定电话 |
| 手机号 | mobile | 否 | S(20) | 13888888888 | 手机号 |
| 邮箱 | email | 否 | S(128) | [test@test.com](mailto:test@test.com) | 邮箱 |
| 联系人业务标识 | tag | 是 | C | [“06”,”08”] | 表示商户联系人的职责。详《联系人业务标识枚举》，JSON数组 |
| 联系人类型 | type | 是 | S(20) | LEGAL_PERSON | 详见《联系人类型》 |
| 身份证号 | id_card_no | 否 | S(32) | 110000199001011234 | business_license与 contact_info.id_card_no 两字段至少上送一个 |

#### 地址信息：address_info

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 城市编码 | city_code | 是 | S(10) | - | 城市编码是与国家统计局一致，请查询 :[http://www.stats.gov.cn/tjsj/tjbz/tjyqhdmhcxhfdm/](http://www.stats.gov.cn/tjsj/tjbz/tjyqhdmhcxhfdm/) |
| 区县编码 | district_code | 是 | S(10) | - | 区县编码是与国家统计局一致，请查询 : [http://www.stats.gov.cn/tjsj/tjbz/tjyqhdmhcxhfdm/](http://www.stats.gov.cn/tjsj/tjbz/tjyqhdmhcxhfdm/) |
| 地址 | address | 是 | S(256) |  | 商户详细经营地址或人员所在地点 |
| 省份编码 | province_code | 是 | S(10) |  | 省份编码是与国家统计局一致，请查询 : [http://www.stats.gov.cn/tjsj/tjbz/tjyqhdmhcxhfdm/](http://www.stats.gov.cn/tjsj/tjbz/tjyqhdmhcxhfdm/) |
| 经度 | longitude | 否 | S(11) | - | 浮点型, 小数点后最多保留 6 位。如需要录入经纬度，请以高德坐标系为准，录入时请确保经纬度参数准确。高德经纬度查询 [http://lbs.amap.com/cons](http://lbs.amap.com/cons) ole/show/picker |
| 纬度 | latitude | 否 | S(10) | - | 同上 |
| 地址类型 | type | 否 | S(32) |  | 地址类型，取值范围：BUSINESS_ADDRESS：经营地址（默认） |

#### 银行结算卡信息：bankcard_info

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 银行卡号 | card_no | 是 | S(48) | 6228480402637874213 | 银行卡号 |
| 银行卡持卡人姓名 | card_name | 是 | S(128) | 张三 | 银行卡持卡人姓名 |
| 银行开户行名称 | bank_branch_name | 否 | S(512) | 招商银行杭州高新支行 | 银行开户行名称 |

#### 网站信息：site_info

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 站点类型 | site_type | 是 | S(32) | - | 站点类型 |
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
| 商户识别码 | subMchId | 是 | S(30) |  | 微信/支付宝分配的商户识别码 |
| 报备信息 | reportInfo | 是 | C |  | JSON格式，根据传入报备类型，查看相应的报备信息说明 / 详见：[报备信息参数：reportInfo] |
0

文档更新时间: 2025-01-17 03:46   作者：超级管理员
