---
name: "微信小程序页面重构蓝图"
description: "Use when drafting a Mini Program page refactor request that must start with diagnosis, scoring, and a production-ready blueprint before coding. Trigger phrases: refactor Mini Program page, page diagnosis, setData optimization plan, weak-network UX blueprint, page architecture cleanup. 适用于发起微信小程序页面重构任务，先诊断，再给出高分方案与实施要求。"
---
为指定的小程序页面或组件生成一份可直接执行的重构任务说明。先审视现状，再输出高分方案，不要跳过诊断阶段。

适用边界：
- 这是 prompt，不是专属 agent 模式
- 用于生成高质量任务说明或直接发起实现请求
- 若只需要只读诊断，请改用微信小程序只读审计官

输出目标：
- 先给出现状诊断书，再给重构蓝图，最后给实施要求
- 明确指出痛点、风险等级、量化分数，以及为什么这些问题会影响真实用户体验或维护成本
- 要求最终实现遵循 `.github/standards/weapp/DESIGN_SYSTEM.md`、`.github/standards/weapp/INTERACTION_STANDARDS.md`、`.github/standards/weapp/PERFORMANCE_PRELOAD_STANDARDS.md` 和仓库现有模式
- 要求任何被改动的接口使用都先对齐真实后端契约，不得臆造后端并不存在的字段、状态、类型或能力
- 要求改动贯通 service、page state、event handlers、view feedback，而不是只改 WXML 或 WXSS
- 评分必须使用固定维度，不要自由发挥评分标准

请按以下结构生成：

## 任务背景
- 目标页面或组件路径：<必填>
- 用户角色与核心任务：<必填>
- 当前问题或想提升的体验：<必填>
- 相关 service/API/组件：<选填>
- 参考页面或交互：<选填>

## 用户任务路径
- 用 3 到 7 步描述真实用户完成任务的路径
- 标出最容易失败、迷失、重复操作或等待过长的节点

## 诊断书
- 按条列出主要痛点
- 每条包含：问题描述、风险等级、量化分数、运行时或维护影响
- 评分维度固定为：信息架构、状态设计、交互效率、异常恢复、性能负担、组件边界

## 高分方案
- 按模块说明重构方向
- 覆盖状态设计、交互反馈、性能优化、首屏请求预算、预加载策略、组件边界、样式令牌使用、弱网与异常容错

## 非目标
- 明确说明本轮不改什么，避免重构范围失控

## 实施要求
- 直接要求输出生产可用代码
- 明确需要补齐 loading、success、empty、error 四态
- 明确检查 setData 粒度、重复点击保护、懒加载或预加载策略是否合理
- 明确运行最小相关校验命令并报告结果

附加要求：
- 如果页面较大，要求先拆出可复用组件边界
- 如果涉及支付、登录、授权、订阅消息或上传，要求把状态恢复与错误提示写清楚
- 如果没有明显问题，也要输出残余风险和可提升项