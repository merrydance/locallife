---
title: 云闪付APP线上收银台直通模式说明
slug: bct-1fao0eosv44i1
source_url: https://doc.mandao.com/docs/bct/bct-1fao0eosv44i1
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: ae51acad19a8f8e6d0e9b2b29e23130e7f60ece868db614fb1fe8ef242aa8057
doc_version: 1705457407
---

# 云闪付APP线上收银台直通模式说明
银联线上收银台直通模式接入指南及列表

# 银联线上收银台直通模式说明

## 一、 Android ( 3.5.11 及以上版本支持)

*upmp_android/UPPayAssistEx.jar*中定义了检测用户安装银联支付 APP 列表的 接口， 接口定义如下:

> void getDirectApps(Context context, String mode, String merchantInfo, IUnionCallbackcallback)

### 1. 接口说明

该接口用于检测用户手机上装有哪些可用于直通模式的银联支付 APP，当用户手机上安装有 可用于直通模式的 APP，该接口会异步返回这些可用 APP 的英文简称列表。

### 2. 接口参数

| 名称 | 类型 | 是否可选 | 说明 |
| --- | --- | --- | --- |
| context | Context | 必填 | |
| mode | String | 选填 | 银联后台环境标识， 00：生产环境， 01： PM 环境，默认值为 00 |
| merchantInfo | String | 选填 | 商户号（银联商户号） |
| callback | IUnionCallback | 必填 | 回调数据 |

IUnionCallback 定义如下：

> public interface IUnionCallback { void onResult(Bundle result); void onError(String code，String msg); }

### 3. result 中参数说明

| 参数 | 字段 | 类型 | 说明 |
| --- | --- | --- | --- |
| 直通可用 app列表 | directApps | List | 直通可用 app 英文简称列表;云闪付名称：CQP / 支持银行详情请查询[云闪付APP线上收银台支持银行列表](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f0mshdn84ibg) |

### 4. code, msg 说明

| 参数 | 字段 | 类型 | 说明 |
| --- | --- | --- | --- |
| 错误码 | code | String | 参数错误 : 01 / 网络错误 : 02 / 其它 : 03 / |
| 错误信息 | msg | String 参数错误 : parameter error / 网络错误 : network error / 其它 : unknown error / | |

### 5. 域名

请确保添加以下域名：

> cn :[https://acpstatic.cup.com.cn](https://acpstatic.cup.com.cn) com :[https://acpstatic.95516.com](https://acpstatic.95516.com)

## 二、 IOS ( 3.4.9 及以上版本支持)

接口定义如下:

> - (void)getDirectApps:(NSString
> *)mode withMerchantInfo:(NSString*
> )merchantInfo
> succBlock:(UPPaymentDirectAppSucc)succBlock
> failBlock:(UPPaymentDirectAppFail)failBlock;

### 1. 接口说明

该接口用于检测用户手机上装有哪些可用于直通模式的银联支付 APP，当用户手机上安装有 可用于直通模式的 APP，该接口会异步返回这些可用 APP 的英文简称列表。

### 2. 接口参数

| 名称 | 类型 | 是否可选 | 说明 |
| --- | --- | --- | --- |
| mode | NSString | 选填 | 银联后台环境标识， 00：生产环境， 01：PM 环境，默认值为 00 |
| merchantInfo | NSString | 选填 | 商户号（银联商户号） |
| succBlock | NSBlock | 必填 | 成功回调 |
| failBlock | NSBlock | 必填 | 失败回调 |

### 3. 成功回调

> typedef void (^UPPaymentDirectAppSucc)(NSArray* directApps);

| 参数 | 字段 | 类型 | 说明 |
| --- | --- | --- | --- |
| 直通可用app | directApps | NSArray | 直通可用 app 英文简称列表 / 云闪付名称：CQP; / 支持银行详情请查询[云闪付APP线上收银台直通模式说明](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f0msipetdcep) |

### 4. 失败回调

> typedef void (^UPPaymentDirectAppFail)(NSString* code,NSString* msg);

| 参数 | 字段 | 类型 | 说明 |
| --- | --- | --- | --- |
| 错误码 | code | NSString | 参数错误 : 01 / 网络错误 : 02 / 其它 : 03 |
| 错误信息 | msg | NSString | 参数错误 : parameter error / 网络错误 : network error / 其它 : unknown error |

### 5. 域名

请确保添加以下域名：

> cn :[https://acpstatic.cup.com.cn](https://acpstatic.cup.com.cn) com :[https://acpstatic.95516.com](https://acpstatic.95516.com)

## 三、后台接口

直接跳转云闪付，需要在接口文档中的字段中:

| 子域名 | 标识 | 子域格式 | 说明 |
| --- | --- | --- | --- |
| 统一收银台直通模式的银行标识 | ebankEnAbbr | ANS..40 | 该值为统一收银台直通模式业务下，直通银行的英文简称。如果 商 户选择直接拉起特定银行 APP，必须上送此要素。云闪付必须上送；CQP / 支持银行详情请查询 / 支持银行详情请查询[云闪付APP线上收银台直通模式说明](https://doc.mandao.com/docs/juhe_pay/juhe_pay-1f0msipetdcep) |
| 个人网关类别 | ebankType | N2 | 01:APP+H5 / 02:APP / 03:H5 / 在银联线上收银台直通模式业务下， 商户可以选择银行 APP 调用的方式(取值只能为 01、02 或者 03); 如果商户未上送取值，默认取值 01。 商户上送此字段时，必须上送“统一收银台直通模式的银行标识”字段，否则将拒绝交易。 |

# 云闪付APP线上收银台直通模式支持银行列表

| 序号 | APP中文名称 | 银行简码（直通模式用） |
| --- | --- | --- |
| 1 | 银联云闪付 | CQP |
| 2 | 中国工商银行 | ICBC |
| 3 | 工银e生活 | ICBCC |
| 4 | 中国农业银行 | ABC |
| 5 | 中国银行缤纷生活 | BOCC |
| 6 | 中国建设银行 | CCB |
| 7 | 交通银行 | BoCom |
| 8 | 买单吧 | BoComC |
| 9 | 中信银行手机银行 | CNCB |
| 10 | 动卡空间 | CNCBC |
| 11 | 光大银行手机银行 | CEB |
| 12 | 招商银行 | CMB |
| 13 | 掌上生活 | CMBLIFE |
| 14 | 浦发银行 | SPDB |
| 15 | 浦大喜奔 | SPDBC |
| 16 | 民生银行手机银行 | CMBC |
| 17 | 全民生活 | CMBCC |
| 18 | 平安口袋银行 | PAB |
| 19 | 兴业银行手机银行 | CIB |
| 20 | 兴业生活（好兴动） | CIBC |
| 21 | 发现精彩 | GDBC |
| 22 | 华夏手机银行 | HXB |
| 23 | 京彩生活 | BCCB |
| 24 | 掌上京彩 | BCCBC |
| 25 | 上海银行手机银行 | BOS |
| 26 | 上银美好生活 | SHBANK |
| 27 | 华彩生活 | HXBC |
| 28 | 浙商银行 | CZBANK |
| 29 | 兰州银行 | LZBANK |
| 30 | 宁波银行 | BNB |
| 31 | 大连农商银行 | DLRCB |

文档更新时间: 2024-01-17 02:10 作者：超级管理员
