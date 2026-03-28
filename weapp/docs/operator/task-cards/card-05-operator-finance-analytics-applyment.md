# CARD-05 运营资金、分析与开户能力收口

状态：待人工验证

优先级：P1

所属阶段：Phase 3

## 问题目标

把财务、分析和开户从“可用单点页面”收成完整的运营工具能力。

## 影响范围

1. weapp/miniprogram/pages/operator/finance/withdraw/index
2. weapp/miniprogram/pages/operator/analytics/index
3. weapp/miniprogram/pages/operator/dashboard/index
4. weapp/miniprogram/pages/operator/applyment/index
5. weapp/miniprogram/api/operator-basic-management.ts
6. weapp/miniprogram/api/operator-analytics.ts
7. weapp/miniprogram/api/operator-applyment.ts

## 任务内容

- [x] finance/withdraw/index 补 getCommissionList 对应的佣金明细和状态表达。
- [x] analytics/index 补区域统计、周期聚合和必要的第二维排行，不再停留在极简摘要。
- [x] dashboard/index 收口待办入口和财务摘要与分析页的关系。
- [x] applyment/index 统一开户状态映射、结果提示和签约引导。
- [x] 明确财务、分析、开户三页之间的入口关系与信息架构。

## 完成定义

- [x] 财务页不仅能提现，还能查看运营收入明细。
- [x] 分析页的核心指标、排行和区域统计都按真实周期口径展示。
- [x] 开户页状态文案与后台状态机一致。

## 验证要求

- [x] finance、analytics、dashboard、applyment 相关页面编辑器诊断通过。
- [x] 运行 npm run quality:check。
- [ ] 人工回归提现、分析切换、待办入口、开户状态流转。

## 完成记录

- [x] 财务能力完成
- [x] 分析能力完成
- [x] 开户能力完成

## 本轮进展

1. finance/withdraw/index 已补“近期佣金明细”区块，接入 getCommissionList，并补 loading、empty、error 三态表达。
2. applyment/index 已统一开户状态映射，覆盖 active、bindbank_submitted、submitted、auditing、to_be_signed、signing、finish、rejected、rejected_sign 等状态，并补签约阶段引导。
3. analytics/index 已补区域选择、周期切换、区域统计和商户/骑手双排行，不再停留在极简摘要。
4. dashboard/index 已补“查看分析”“佣金明细”“开户状态”等直达入口，首页与分析、财务、开户三页的信息架构关系已明确。

## 主链验收结论

1. 代码侧主链已闭合：finance、analytics、dashboard、applyment 四页的数据合同、入口关系和状态表达均已完成收口。
2. 当前剩余事项仅包括人工回归：提现、分析切换、待办入口、开户状态流转。
3. 通过人工回归后，本卡即可转入 Phase 5 统一评分复核。