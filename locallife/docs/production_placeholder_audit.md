# 生产上线前：空实现/硬编码/Mock 清单（可勾选）

说明：以下为运行时路径中的 TODO/Mock/Demo/临时逻辑与配置，需逐项移除或替换为真实实现，确保前端数据可接收、可落库、可计算、可产出真实结果。

## 后端（Go）
- [ ] 微信电商证书加密未实现：[wechat/ecommerce.go](wechat/ecommerce.go#L671)
- [ ] 信用分通知改造 TODO：[worker/task_trust_score.go](worker/task_trust_score.go#L129)

## 小程序运行时逻辑（前端）
### Demo/Mock 模式与数据
- [ ] Demo 模式用户注入与跳过后端请求：[weapp/miniprogram/app.ts](weapp/miniprogram/app.ts#L287-L304)
- [ ] Mock 开关配置（需确保生产关闭且无兜底 mock）：[weapp/miniprogram/config/index.ts](weapp/miniprogram/config/index.ts#L90-L91)
- [ ] Demo 数据集（需移除或禁止生产引用）：[weapp/miniprogram/mock/demo-data.ts](weapp/miniprogram/mock/demo-data.ts#L2)

### 业务流程中的 Mock/TODO
- [ ] 订单评价上传使用 Mock URL： [weapp/miniprogram/pages/order/review/index.ts](weapp/miniprogram/pages/order/review/index.ts#L9-L32)
- [ ] 骑手押金支付/退款 API 未实现： [weapp/miniprogram/pages/rider/deposit/index.ts](weapp/miniprogram/pages/rider/deposit/index.ts#L52-L65)
- [ ] 骑手异常页使用 Mock 数据与上传占位： [weapp/miniprogram/pages/rider/exception/index.ts](weapp/miniprogram/pages/rider/exception/index.ts#L42-L86)
- [ ] 消息中心 Mock 数据兜底： [weapp/miniprogram/pages/message/center/index.ts](weapp/miniprogram/pages/message/center/index.ts#L36-L58)
- [ ] 商户数据图表使用 Mock 分布： [weapp/miniprogram/pages/merchant/analytics/enhanced/index.ts](weapp/miniprogram/pages/merchant/analytics/enhanced/index.ts#L65)
- [ ] 运营自动化 Mock 列表/开关： [weapp/miniprogram/pages/operator/automation/index.ts](weapp/miniprogram/pages/operator/automation/index.ts#L23-L57)
- [ ] 运营规则 Mock 列表： [weapp/miniprogram/pages/operator/rules/index.ts](weapp/miniprogram/pages/operator/rules/index.ts#L23-L51)
- [ ] 钱包页 Mock 数据： [weapp/miniprogram/pages/user_center/wallet/index.ts](weapp/miniprogram/pages/user_center/wallet/index.ts#L30-L64)
- [ ] 评价图片上传 TODO： [weapp/miniprogram/pages/user_center/reviews/create/index.ts](weapp/miniprogram/pages/user_center/reviews/create/index.ts#L103)
- [ ] 信用分页面固定 Mock 用户 ID 与兜底： [weapp/miniprogram/pages/credit/index.ts](weapp/miniprogram/pages/credit/index.ts#L23-L42)
- [ ] 运营注册提交 TODO： [weapp/miniprogram/pages/register/operator/index.ts](weapp/miniprogram/pages/register/operator/index.ts#L339)
- [ ] 预约列表 Demo 逻辑： [weapp/miniprogram/pages/reservation/list/index.ts](weapp/miniprogram/pages/reservation/list/index.ts#L101)
- [ ] 预约首页临时筛选状态： [weapp/miniprogram/pages/reservation/index.ts](weapp/miniprogram/pages/reservation/index.ts#L38)
- [ ] 外卖页 TODO 跳转： [weapp/miniprogram/pages/takeout/index.ts](weapp/miniprogram/pages/takeout/index.ts#L748)
- [ ] 地图组件使用 Mock 坐标： [weapp/miniprogram/components/map-view/index.ts](weapp/miniprogram/components/map-view/index.ts#L21)

### API 未接入（前端占位）
- [ ] 用户优惠券列表 API 未实现： [weapp/miniprogram/services/coupon.ts](weapp/miniprogram/services/coupon.ts#L59)
- [ ] 领取优惠券 API 未实现： [weapp/miniprogram/services/coupon.ts](weapp/miniprogram/services/coupon.ts#L69)

### 路由/页面缺失（前端 TODO）
- [ ] 订单评价页不存在： [weapp/miniprogram/utils/navigation.ts](weapp/miniprogram/utils/navigation.ts#L79)
- [ ] 用户地址编辑页不存在： [weapp/miniprogram/utils/navigation.ts](weapp/miniprogram/utils/navigation.ts#L102)
- [ ] 预约详情页不存在： [weapp/miniprogram/utils/navigation.ts](weapp/miniprogram/utils/navigation.ts#L137)

## 备注
- 本清单仅包含运行时路径（非测试代码）。
- 每完成一项请勾选，并在对应 PR/任务中附上实现与测试证据。