---
title: 商户违规处罚通知回调
slug: bct-1fkgcm5kmrb3f
source_url: https://doc.mandao.com/docs/bct/bct-1fkgcm5kmrb3f
captured_from: authenticated Baofoo doc.mandao BCT documentation
captured_date: 2026-05-08
source_capture_sha256: 6abc8b974315f3416aa9676078f209c4f84d611697b45269a557c2c7b42de5c9
doc_version: 1716448370
---

# 商户违规处罚通知回调
**注意事项：**

> - 由于网络抖动等异常因素以及商户侧未按照约定返回OK，宝付支付系统会多次请求商户侧通知地址，商户系统必须能够正确处理重复的通知。 推荐的做法是，当商户系统收到通知进行处理时，先检查对应业务数据的状态，并判断该通知是否已经处理。如果未处理，则再进行处理；如果已处理，则直接返回结果成功。在对业务数据进行状态检查和处理之前，要采用数据锁进行并发控制，以避免函数重入造成的数据混乱。
> - 初次使用请仔细核对，信息是否有误，出现错误请及时联系宝付技术人员。

# 接口说明

被平台风险处置时，推送商户违规处理记录和交易拦截记录时，商户会收到通知回调。商户接收到通知系统处理完成后，按约定返回OK。

# 通知参数

| 字段名 | 变量名 | 必填 | 类型 | 示例值 | 描述 |
| --- | --- | --- | --- | --- | --- |
| 代理商商户号 | agentMerId | 否 | S(16) | 100000 | 宝付支付分配的商户号 |
| 代理商终端号 | agentTerId | 否 | S(16) | 100000 | 宝付支付分配的终端号 |
| 商户号 | merId | 是 | S(16) | 100000 | 宝付支付分配的商户号 |
| 终端号 | terId | 是 | S(16) | 100000 | 宝付支付分配的终端号 |
| 二级商户号 | subMchId | 是 | S(32) | 1900009231 | 详见附录订单状态 |
| 商户公司名称 | companyName | 是 | S(64) | 财付通支付科技有限公司 | 商户公司名称 |
| 违约订单号 | recordId | 是 | S(128) | 200201820200101080076610000 | 对违约商户处理通知的唯一标识，可用于去重 |
| 处罚方案 | punishPlan | 是 | S(2048) | 关闭支付权限 | 微信支付对违约商户的具体处罚方案，可根据 / 具体的处罚方案指引商户登录商户平台/商家助 / 手小程序进行申诉/相关操作，使用时请留意该 / 值为处罚方法的文本内容，并非枚举值 |
| 处罚时间 | punishTime | 是 | S(64) | 20210315155012 | 微信支付对违约商户的处置时间， / 格式为：yyyyMMDDHHmmss |
| 处罚详细描述 | punishDescription | 是 | S(128) | 利用特殊行业违规经营，加重处罚 | 微信支付对违约商户处罚方案的详细描述信息， / 补充处罚方案的相关影响 |
| 风险类型 | riskType | 是 | S(2048) | ONE_YUAN_PURCHASES | 具体枚举值见附录 |
| 风险类型中文描述 | riskDescription | 是 | S(2048) | 涉嫌一元购 | 微信支付对违约商户定义的风险类型枚举值对 / 应的中文描述 |

# 通知应答参数

    OK

### 微信支付对违约商户定义的风险类型

| 枚举值 | 枚举描述 |
| --- | --- |
| ONE_YUAN_PURCHASES | 涉嫌一元购 |
| MULTI_LEVEL_DISTRIBUTION_REBATE | 涉嫌多级分销返利 |
| PROHIBITED_BUSINESS_CATEGORIES | 涉嫌我司未开放类目 |
| CASH_ADVANCE_VIA_CREDIT_CARD | 涉嫌信用卡套现 |
| INDUCING_USERS_TO_MAKE_PAYMENTS | 涉嫌诱导支付 |
| FRAUD | 涉嫌欺诈 |
| MALICIOUS_FAN_COUNT_BOOSTING | 涉嫌恶意吸粉 |
| CROSS_CATEGORY_ACTIVITIES | 涉嫌跨类目 |
| CROSS_CATEGORY_BUSINESS | 涉嫌跨类目经营 |
| GAMBLING | 涉嫌赌博 |
| LEWD_CONTENT | 涉嫌色情 |
| UNLICENSED_PAYMENT_AND_SETTLEMENT_BUSINESS | 涉嫌无证经营支付结算业务 |
| INVESTMENT | 涉嫌投资理财 |
| TRANSACTION_DISPUTE | 涉嫌交易纠纷 |
| CROSS_BORDER_USE_OF_DOMESTIC_PAYMENT_API | 涉嫌境内支付接口跨境使用 |
| OVERSEAS_ACTIVITIES_OUTSIDE_THE_BUSINESS_SCOPE_APPROVED_BY_REGULATORY_AUTHORITIES | 涉嫌境外超监管批复范围经营 |
| UNUSUAL_TRANSACTION | 涉嫌交易异常 |
| UNLICENSED_BUSINESS | 涉嫌无资质经营 |
| WEALTH_INVESTMENT | 涉嫌投资理财 |
| AFFILIATED_TO_A_VIOLATING_ENTITY | 涉嫌关联违规主体等异常风险 |
| INVOLVED_IN_A_JUDICIAL_CASE | 涉嫌司法案件 |
| INCORRECT_INFORMATION_SUBMITTED | 涉嫌资料异常 |
| APPEAL_SUCCESSFUL | 申诉成功 |
| REPORTED_BY_OTHERS | 涉嫌他人投诉举报 |
| VIOLATING_SMART_CATERING_ACTIVITIES | 涉嫌智慧餐饮活动违规 |
| MORE_THAN_ONE_MERCHANT_UNDER_A_SINGLE_MERCHANT_ID | 涉嫌同一商户号下挂多个商户 |
| CROSS_REGION_USE_OF_INTERNATIONAL_PAYMENT_API | 涉嫌境外支付接口跨区域 |
| UNUSUAL_REAL_TIME_TRANSACTION | 涉嫌实时交易异常 |
| UNACCEPTABLE_DOCUMENTS | 涉嫌资料不合格 |
| LARGE_AMOUNT_TRANSACTION | 涉嫌大额交易 |
| ALL_MERCHANTS_HAVE_CONFIRMED_THE_WILLINGNESS_TO_OPEN_AN_ACCOUNT | 无交易商户未确认开户意愿 |
| UNCONFIRMED_WILLINGNESS_TO_OPEN_AN_ACCOUNT | 未确认开户意愿 |
| INACTIVE_TRANSACTION | 交易停滞 |
| OTHER_UNUSUAL_ACTIVITIES | 涉嫌其它异常 |

文档更新时间: 2024-05-23 07:12   作者：超级管理员
