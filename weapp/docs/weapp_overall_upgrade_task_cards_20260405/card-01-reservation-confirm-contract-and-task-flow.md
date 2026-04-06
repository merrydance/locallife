# CARD-01 预订确认页任务边界与支付真实化

状态：代码修复完成，待人工回归

优先级：P0

所属批次：Batch 1

## 问题目标

让预订确认页只展示真实会影响流程的选择，并把“确认预订”“支付”“后续点餐”三件事的任务边界拆清楚。

## 影响范围

- [weapp/miniprogram/pages/reservation/confirm/index.ts](weapp/miniprogram/pages/reservation/confirm/index.ts)
- [weapp/miniprogram/pages/reservation/confirm/index.wxml](weapp/miniprogram/pages/reservation/confirm/index.wxml)
- 相关预订 service/API 封装

## 已知问题

- 页面展示了支付方式，但关键提交逻辑没有真正消费这部分选择。
- 用户难以判断当前动作是在锁桌、付款，还是仅进入下一步。
- 页面任务边界模糊，属于高风险主链路上的假控件问题。

## 任务内容

- [x] 核对预订确认页当前真实后端 contract，确认哪些输入会真正影响后续流程。
- [x] 清除不被真实流程消费的支付方式或流程控件，或者补齐其真实 contract 落点。
- [x] 把页面主任务收口成一个明确动作，不让多个流程含义混在同一个主按钮里。
- [x] 调整确认页的文案、说明区和状态区，让用户清楚知道“这一步完成什么、下一步还需要什么”。
- [x] 检查提交中、失败、取消、回退后的页内状态，避免再次进入时落回伪完成状态。

## 完成定义

- [x] 确认页不再展示无效支付选项或伪任务分支。
- [x] 页面主操作的语义唯一且明确。
- [x] 用户能清楚分辨“提交预订”和“支付结果承接”是两个不同阶段。

## 验证要求

- [x] 核对后端 handler、DTO 或 service contract，确认页面展示字段和动作语义真实有效。
- [ ] 人工验证正常确认、取消返回、提交失败、重新进入四类场景。
- [x] review 时使用 `.github/standards/weapp/REVIEW_CHECKLIST.md`，重点检查视觉、交互与契约闭环。

## 完成记录

- [x] contract 核对完成
- [x] 页面任务边界重构完成
- [x] 2026-04-06 补做支付结果语义修复，确认确认页不会再把明确失败错误承接为未知结果
- [ ] 提交与失败分支回归完成
- [x] review 完成

PR 链接、验证结果和残余风险见 [weapp/docs/weapp_overall_upgrade_task_cards_20260405/README.md](weapp/docs/weapp_overall_upgrade_task_cards_20260405/README.md)。