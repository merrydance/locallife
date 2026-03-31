# CARD-06 骑手孤岛页与旧配送管理清理

状态：已实现，待人工回归

优先级：P0

所属阶段：Phase 3

## 问题目标

清理 claims/index、delivery/manage/manage 这类注册但无可靠入口或整体过时的页面，停止旧实现继续污染 rider 域。

## 影响范围

- weapp/miniprogram/pages/rider/claims/**
- weapp/miniprogram/pages/rider/delivery/manage/**
- weapp/miniprogram/components/delivery-task-card/**
- weapp/miniprogram/app.json

## 任务内容

- [x] 为 claims/index、delivery/manage/manage 做保留、迁移、下线三选一决策。
- [x] 若保留 delivery/manage/manage，则必须修复经纬度入参、/v1 路径、状态枚举、事件绑定和不存在字段；若不保留，则清理注册和入口。
- [x] 清理 delivery-task-card 里的旧状态机 accepted/picking_up/picked_up 和未实现事件 onNavigate。
- [x] 清理无效备份跳转和死导航。

## 完成定义

- [x] rider 子包不再保留注册但不可用的旧页面。
- [x] 所有保留页面都有真实入口和真实职责。

## 验证要求

- [x] 检查 app.json 注册页与真实入口一致。
- [x] 检查删除或迁移后不存在死链接。
- [x] 执行最小相关质量检查。

## 完成记录

- [x] 决策完成
- [x] 页面清理完成
- [ ] 回归完成

## 本次实现说明

- claims/index 保留并收口为真实索赔与申诉入口，配合详情页承接追偿与申诉动作。
- 已删除的旧页面已从 rider 子包与接口契约中整体下线，不再作为 rider 域保留页面。
- delivery/manage/manage 已从 rider 子包注册中移除，并删除整页与其唯一依赖组件 delivery-task-card，避免继续保留过时状态机和无入口页面。
- 剩余工作集中在继续排查 rider 域的无效备份跳转与死导航，并完成真机回归。