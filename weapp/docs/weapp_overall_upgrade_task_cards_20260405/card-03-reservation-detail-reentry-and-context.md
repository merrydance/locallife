# CARD-03 预订详情与返回重入恢复链路收口

状态：代码修复完成，待人工回归

优先级：P1

所属批次：Batch 1

## 问题目标

把预订详情页变成整个预订链中的稳定上下文页，让用户从确认、支付、返回列表、前后台切换后都能回到可信状态。

## 影响范围

- [weapp/miniprogram/pages/reservation/detail/index.ts](weapp/miniprogram/pages/reservation/detail/index.ts)
- [weapp/miniprogram/pages/reservation/detail/index.wxml](weapp/miniprogram/pages/reservation/detail/index.wxml)
- [weapp/miniprogram/pages/user_center/reservations/index.ts](weapp/miniprogram/pages/user_center/reservations/index.ts)
- [weapp/miniprogram/pages/user_center/reservations/index.wxml](weapp/miniprogram/pages/user_center/reservations/index.wxml)
- 相关预订详情查询、状态刷新和导航回流逻辑

## 已知问题

- 当前预订链中，详情页没有充分承担“当前真实状态锚点”的角色。
- 确认页、结果页、回前台和返回列表之间的上下文回流还不够清晰。
- 详情页若不能成为可信真值页，整个预订链会继续依赖一次性页面文案而不是状态本身。

## 任务内容

- [x] 核对预订详情页应承接的真实状态、字段和后续动作。
- [x] 让详情页成为预订链中的主状态承接页，而不是只做一次性信息展示。
- [x] 设计从确认页、支付结果页、返回列表和前后台切换后回到详情页时的刷新与恢复策略。
- [x] 检查详情页中的 CTA、状态文案、空态和错误态，确保都服务当前主任务。
- [x] 避免在详情页继续出现和支付结果页重复解释同一结果的双重提示。

## 完成定义

- [x] 详情页能稳定表达当前预订真实状态和下一步动作。
- [x] 返回页面、重入页面、回前台后，用户能回到可信上下文。
- [x] 详情页与确认页、结果页职责边界清楚，没有互相抢结果承接。

## 验证要求

- [ ] 人工验证从列表进入详情、从确认回详情、从结果页回详情、前后台切换后回详情等路径。
- [ ] 验证详情页的错误态、刷新态和已有数据保留策略。
- [x] review 时使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`，重点检查状态恢复、主任务清晰度和反馈去重。

## 完成记录

- [x] 详情 contract 与状态机梳理完成
- [x] 上下文回流策略重构完成
- [x] 2026-04-06 补做列表、详情与普通订单详情页的支付承接修复，避免共享支付流程返回未知态时误跳成功页
- [ ] 重入与恢复回归完成
- [x] review 完成

PR 链接、验证结果和残余风险见 [weapp/docs/weapp_overall_upgrade_task_cards_20260405/README.md](weapp/docs/weapp_overall_upgrade_task_cards_20260405/README.md)。