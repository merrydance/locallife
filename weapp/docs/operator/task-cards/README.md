# Operator 修复任务卡索引

日期：2026-03-28

本目录用于把运营侧全量审计结果拆成可领取、可并行、可勾选的任务卡。

## 使用方式

1. 每张卡尽量满足单人或单小组可在一轮迭代内完成。
2. 每张卡必须同时回填代码验证、人工回归和剩余风险。
3. 如果任务卡涉及多个页面，必须先完成合同矩阵，再落代码。

## 任务卡列表

### Phase 0：真值与路由基线

- [ ] card-01-operator-truth-and-route-closure.md

### Phase 1：主链路合同修复

- [ ] card-02-operator-merchant-rider-contract.md
- [ ] card-03-operator-appeal-and-safety.md

### Phase 2：区域与规则域收口

- [ ] card-04-operator-region-delivery-rule.md
	当前状态：待人工验证

### Phase 3：资金、开户、分析收口

- [ ] card-05-operator-finance-analytics-applyment.md
	当前状态：待人工验证

## 推荐执行顺序

1. 先做 card-01，把路由、孤儿页、统计真值和 DTO 真值锁死。
2. 再并行推进 card-02 和 card-03，把高频列表和审核主链打通。
3. 然后执行 card-04，解决区域、配送费、规则、扩区多页并行问题。
4. 最后执行 card-05，把财务、开户、分析收口到完整运营工具。

## 完成标准

1. 所有注册页面完成合同复核。
2. 孤儿页被迁移或删除。
3. 后端已提供的关键运营能力都有页面入口或明确下线说明。
4. 运营侧统一评分达到 95 分以上。