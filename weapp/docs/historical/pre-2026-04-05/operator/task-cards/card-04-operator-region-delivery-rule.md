# CARD-04 运营区域、配送费与规则域收口

状态：待人工验证

优先级：P1

所属阶段：Phase 2

## 问题目标

把 region、delivery-fee、timeslot、rules、region-expansion 五类页面收成统一规则域，避免重复入口和重复真值来源。

## 影响范围

1. weapp/miniprogram/pages/operator/region/index
2. weapp/miniprogram/pages/operator/region/config
3. weapp/miniprogram/pages/operator/delivery-fee/index
4. weapp/miniprogram/pages/operator/timeslot/index
5. weapp/miniprogram/pages/operator/rules/index
6. weapp/miniprogram/pages/operator/region-expansion/index
7. weapp/miniprogram/api/delivery-fee.ts
8. weapp/miniprogram/api/operator-application.ts

## 任务内容

- [x] 明确 region/config、delivery-fee/index、timeslot/index 的唯一职责边界。
- [x] 判定 delivery-fee/index 保留还是下线；若保留，必须完全对齐真实 DTO，不再使用本地 mock 字段和默认 regionId=1。
- [x] region/index 去掉添加区域假动作，统一使用真实路径。
- [x] region/index、region-expansion/index 接后端真实分页与可申请区域合同。
- [x] region-expansion/index 已改用后端可申请区域合同。
- [x] rules/index 补输入校验、编辑保护和统一服务层封装。
- [x] timeslot/index 补编辑现有时段、冲突校验或给出后端不支持说明。

## 完成定义

- [x] 区域与规则域没有重复入口和重复真值来源。
- [x] 扩区申请只展示真正可申请区域。
- [x] 配送费、时段系数、规则配置的页面边界清晰。

## 验证要求

- [x] 相关页面编辑器诊断通过。
- [x] 运行 npm run quality:check。
- [ ] 人工验证区域选择、规则修改、峰时新增删除、扩区申请主链。

## 完成记录

- [x] 区域域收口完成
- [x] 配送费页 DTO 收口完成
- [x] 扩区可申请区域合同收口完成
- [x] 规则域收口完成

## 本轮进展

1. region/index 已去掉本地“添加区域”假动作，统一跳转至真实扩区申请页。
2. region/index 已改为使用后端 has_more 作为分页真值，不再通过 pageSize 猜测还有下一页。
3. rules/index 已补充输入校验、未改动拦截和保存态保护；统一服务层封装仍待后续收口。
4. region/config 已改为“区域配置中心”摘要页，只承载基础配送费与峰时时段的分流，不再重复编辑同一份规则。
5. timeslot/index 已补时段冲突校验，并在页面内明确“当前后端仅支持新增和删除，修改需删除后重建”的边界说明。
6. rules/index 已新增正式 operator-rules 服务层封装，页面不再直接 request /v1/operator/rules。

## 主链验收结论

1. 代码侧主链已闭合：region、region-expansion、delivery-fee、timeslot、rules 五条链均已接入真实后端合同或明确后端不支持边界。
2. 当前剩余事项仅包括人工回归：区域选择、规则修改、峰时新增删除、扩区申请主链。
3. 通过人工回归后，本卡即可转入 Phase 5 统一评分复核。