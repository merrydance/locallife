# CARD-06 修复堂食扫码入口死链

状态：进行中（代码完成，待人工回归）

优先级：P0

所属阶段：Phase 1

## 问题目标

修复堂食扫码入口中跳往不存在页面的路径，确保商户信息查看路径真实可达。

## 影响范围

- [weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts](weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts)
- [weapp/miniprogram/app.json](weapp/miniprogram/app.json)

## 任务内容

- [x] 确认商户信息页的真实落点。
- [x] 修复或替换错误路由。
- [x] 若没有现成页面，则先移除死链入口或改到现有公开详情页。

## 完成定义

- [ ] 堂食扫码页所有用户可见入口都能正常跳转。
- [ ] 不再触发 Page Not Found。

## 验证要求

- [ ] 人工验证扫码页中的商户信息入口。
- [x] 核对 app.json 中的注册路径。
- [x] 编辑器诊断与定向 lint 校验通过。

## 完成记录

- [x] 修复完成
- [x] 路由核对完成
- [ ] 回归完成

补充说明：

- 扫码页中的商户详情入口已从不存在的 `/pages/merchant/detail/detail` 改为现有公开详情页 `/pages/takeout/restaurant-detail/index`。
- 已核对 [weapp/miniprogram/app.json](weapp/miniprogram/app.json) 中存在目标分包注册；仍需补一次扫码页人工点击回归。