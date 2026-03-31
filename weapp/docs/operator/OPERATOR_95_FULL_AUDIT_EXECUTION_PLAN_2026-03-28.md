# Operator 95+ 全量审计与修复执行计划

日期：2026-03-28

> 历史快照说明
>
> 本文是 2026-03-28 审计阶段形成的运营侧执行计划快照，保留的是当时的风险判断、页面域划分和阶段性修复路径，不代表当前 operator 子包的现行注册页、页面职责、统计合同或最终交付状态。
>
> 涉及当前真值时，请优先以 weapp/miniprogram 的实际页面代码、当前注册配置、后端真实路由，以及 weapp/docs/operator 下的最新任务卡和后续收口文档为准。

## 1. 目标

本计划用于把微信小程序运营侧从“核心页面已具雏形，但统计口径、列表合同、详情契约、区域治理和运营工具完整性存在漂移”的状态，提升到稳定 95 分以上。

本轮目标只有两条：

1. 完全对齐后端真实接口和字段语义。
2. 把运营侧提升为列表可信、动作可审、状态一致、区域权限清晰的运营控制台。

本计划不是零散修页面，而是先锁定唯一真相，再按域推进。

## 2. 当前结论

基于 2026-03-28 的全量页面复核，运营侧当前已经不是“有没有页面”的问题，而是进入“页面是不是按后端真实合同运行”的阶段。

当前已经确认的高风险事实：

1. 运营首页和分析页存在统计口径漂移，累计值、本月值、单日趋势点被混用于近 7 天或近 30 天标签。
2. 商户、骑手、申诉三条主链路的分页、筛选、详情契约和动作闭环并未统一到后端返回的 total 或 has_more。
3. 仓库中存在已注册页面与未注册历史页面并行的情况，信息架构已经漂移，导致线上可用能力和代码中存在的能力不一致。
4. 多个详情页仍在绕过服务层或通过强制类型转换吞掉 DTO 漂移，后续后端字段一旦调整，页面可能静默错位。
5. 区域、配送费、规则、时段系数存在多页面重复承载同一业务域的问题，职责没有收口。
6. 后端已提供的部分运营能力尚未落到注册页面，包括骑手暂停与恢复、运营佣金明细、食安事件提交、申诉详情时间线与证据、区域统计钻取等。

结论：下一阶段不能继续按单页修补推进，必须先建立运营侧唯一合同矩阵，再按域推进收口。

## 3. 审查边界

### 3.1 注册页面范围

当前 operator 子包注册页面共 18 个，以 app.json 为准：

1. dashboard/index
2. analytics/index
3. merchants/index
4. merchants/detail/index
5. riders/index
6. riders/detail/index
7. rules/index
8. region/index
9. region/config
10. timeslot/index
11. delivery-fee/index
12. safety/report/index
13. safety/detail/index
14. applyment/index
15. finance/withdraw/index
16. appeal/list/index
17. appeal/detail/index
18. region-expansion/index

### 3.2 子孙页面与孤儿实现

除注册页面外，当前还存在两组需要纳入审计但未实际注册的历史实现：

1. pages/operator/dashboard/dashboard
2. pages/operator/merchants/list/list

这两组页面代表历史设计方向，不能继续忽略，否则后续会重复出现“线上页没有、仓库里有”的能力漂移。

### 3.3 后端核对范围

每个页面至少反查以下一项或多项：

1. 小程序 API wrapper
2. 接口 request / response DTO
3. 实际页面是否使用服务层而不是直接 request
4. 动作按钮是否触发真实后端行为
5. 后端已提供但页面未暴露的能力

## 4. 评分标准

每个页面统一按 100 分评分，低于 95 分不得视为完成。

### 4.1 评分维度

1. 后端合同一致性：25 分
2. 动作闭环完整性：20 分
3. 分页、筛选与统计真值：15 分
4. loading / success / empty / error / retry 完整性：10 分
5. 弱网与失败反馈：10 分
6. 信息架构与路由收口：10 分
7. 设计系统与可维护性：10 分

### 4.2 95 分达标标准

同时满足以下条件才算达标：

1. 页面字段、动作、状态机与后端真实合同一致。
2. 列表分页、筛选、总数和 has_more 以后端返回为准，不再由前端推断。
3. 所有关键按钮都调用真实 API，而不是只改本地状态或弹提示。
4. 页面具备 loading、success、empty、error、retry 的完整表达。
5. 统计标签与统计口径一致，没有累计值冒充周期值。
6. 不存在死链、重复入口、重复状态源、孤儿页面继续承载真实能力。

## 5. 全域问题分类

### 5.1 P0 合同真值问题

1. 仪表盘与分析页统计口径错误。
2. 商户、骑手、申诉详情页 DTO 漂移，部分页面绕过服务层。
3. appeal/list 未接分页合同。
4. delivery-fee/index 存在与真实 DTO 不一致的本地字段与默认 regionId 行为。
5. 已注册页面与未注册页面并行，导致真实能力漂移。

### 5.2 P1 主链路缺能力问题

1. 线上商户列表缺搜索、筛选、暂停、恢复能力。
2. 骑手列表和详情页未暴露暂停、恢复等后端已支持能力。
3. 申诉详情未展示后端已提供的 evidence_files、timeline、related_order 等字段。
4. 财务页未接佣金明细能力。
5. 食安事件页未接提交事件能力。
6. region-expansion 未使用 available regions 合同筛可申请区域。

### 5.3 P2 治理与体验问题

1. 区域、配送费、规则、时段系数域存在重复页面和职责重叠。
2. 多页仍通过 length === pageSize 推断 hasMore。
3. 局部页面未完全遵守设计系统 token 和 tag 规范。
4. 页面间区域上下文传递不统一。

## 6. 分阶段修复计划

### Phase 0：基线锁定

目标：锁定运营侧唯一真相，停止后续继续基于旧页面或旧 DTO 猜测实现。

二级任务：

1. 锁定 operator 子包真实注册页面与孤儿页面清单。
2. 为每个页面建立统一审计台账与评分卡。
3. 建立页面到 API 服务、DTO、动作、缺失能力的映射矩阵。
4. 明确哪些能力应保留、哪些历史页面应删除或迁移。

产出：

1. 运营侧全量审计台账。
2. 页面与接口映射矩阵。
3. 任务卡索引与阶段拆分。

退出标准：

1. 后续改造不再基于前端旧实现推断后端语义。

### Phase 1：真值与路由收口

目标：先把最容易误导运营判断的统计、分页、路由和 DTO 漂移问题止血。

二级任务：

1. 修正 dashboard/index 与 analytics/index 的统计窗口和标签语义。
2. 收口 merchants/index、riders/index、appeal/list/index 的分页合同。
3. 收口 merchants/detail/index、riders/detail/index、appeal/detail/index 的 DTO 真值来源。
4. 处理 dashboard/dashboard 与 merchants/list/list 的保留、迁移或删除方案。
5. 统一线上页和仓库中实际能力的唯一入口。

重点对象：

1. dashboard/index
2. analytics/index
3. merchants/index
4. riders/index
5. appeal/list/index
6. appeal/detail/index
7. dashboard/dashboard
8. merchants/list/list

退出标准：

1. 所有关键列表和统计页的数值、分页、入口不再漂移。

### Phase 2：列表、详情、动作主链补齐

目标：把运营最常用的商户、骑手、申诉三条链路补成可运营状态。

二级任务：

1. 商户列表补齐搜索、筛选、暂停、恢复或明确迁移旧页面能力。
2. 商户详情按真实 DTO 展示，去掉强制类型转换。
3. 骑手列表与详情统一接 operatorRiderManagementService，并补齐暂停、恢复等动作。
4. 申诉列表接分页与状态全集，补齐 compensated 或真实后端状态映射。
5. 申诉详情补齐 timeline、evidence_files、related_order、回流刷新。

退出标准：

1. 商户、骑手、申诉链路做到列表可信、详情完整、动作可回流。

### Phase 3：区域、规则、费率域收口

目标：把区域治理相关页面从“多页并行的半重复工具”收成统一的运营规则域。

二级任务：

1. 明确 region/index、region/config、delivery-fee/index、timeslot/index、rules/index 的唯一职责边界。
2. region/index 去掉假动作，按后端权限与真实区域状态展示。
3. region/config 统一承载基础配送配置与峰时配置，或明确拆分后的跳转关系。
4. delivery-fee/index 若保留则完全按后端 DTO 收口；若不保留则下线路由和入口。
5. rules/index 补字段校验、分类合同、区域上下文和操作保护。
6. region-expansion 改用 available regions 合同，避免申请不可申请区域。

退出标准：

1. 区域、规则、配送费、时段配置没有重复入口和重复真值来源。

### Phase 4：资金、开户、食安能力补齐

目标：把财务、开户与食安从“能看局部页面”提升到“具备完整运营工具能力”。

二级任务：

1. finance/withdraw/index 补运营佣金明细、区分概览与提现动作状态。
2. applyment/index 继续对齐开户状态机，明确 active、bindbank_submitted、finish 等状态文案。
3. safety/report/index 补提交事件入口或明确只读定位。
4. safety/detail/index 补错误壳、已处理态只读、恢复商户动作的权限和回流。

退出标准：

1. 资金、开户、食安链路具备完整的查看、提交、处理、回流能力。

### Phase 5：统一验证与95分收口

目标：完成运营侧统一评分、弱网回归和残余风险封板。

二级任务：

1. 按页面重新评分。
2. 对核心链路进行 loading、success、empty、error、retry 五态验证。
3. 对统计页、审批页、规则页、提现页做弱网和返回回流验证。
4. 清理调试日志、历史孤儿页和未使用实现。

退出标准：

1. operator 子包统一评分达到 95 分以上。
2. 不再存在已知 P0 / P1 合同漂移问题。

## 7. 任务卡拆分

详细任务卡见：

1. weapp/docs/operator/task-cards/README.md
2. weapp/docs/operator/task-cards/card-01-operator-truth-and-route-closure.md
3. weapp/docs/operator/task-cards/card-02-operator-merchant-rider-contract.md
4. weapp/docs/operator/task-cards/card-03-operator-appeal-and-safety.md
5. weapp/docs/operator/task-cards/card-04-operator-region-delivery-rule.md
6. weapp/docs/operator/task-cards/card-05-operator-finance-analytics-applyment.md

## 8. 验证基线

本轮审计阶段已确认：

1. 运营侧注册页面和孤儿页面已完成静态复核。
2. 关键页面抽样无编辑器诊断错误。
3. weapp 已通过 npm run quality:check。
4. card-01 第一轮代码已落地：dashboard/index 与 analytics/index 的统计真值已改为周期聚合，待办总入口已按待审类型分流，dashboard/dashboard 中的孤儿商户页死链已清理。
5. card-02 主链第一轮代码已落地：merchants/index 与 riders/index 已收口到正式服务层和真实分页合同，商户/骑手详情页已去掉旧 DTO 强转或直连 request，并补齐暂停、恢复动作回流。
6. card-03 第一轮代码已落地：appeal/list 已接真实分页与 compensated 状态，appeal/detail 已补齐关联订单、证据材料与时间线，safety/report 已补提交入口，safety/detail 已补错误壳、已处理只读和恢复回流。
7. card-04 第二轮已推进：region/index 已去掉假动作并改用真实扩区路径，region/index 与 region-expansion/index 已统一使用后端分页合同，rules/index 已补输入校验、未改动拦截与保存态保护。
8. card-05 第一轮已推进：finance/withdraw/index 已补近期佣金明细与状态表达，applyment/index 已统一开户状态映射和签约阶段引导。
9. weapp 当前已再次通过 npm run quality:check。
10. card-04 第三轮已推进：region/config 已改为配置中心摘要页并分流到 delivery-fee 与 timeslot，timeslot/index 已补冲突校验并明确后端仅支持新增、删除的边界。
11. card-05 第二轮已推进：analytics/index 已补区域选择、周期切换、区域统计和商户/骑手双排行，dashboard/index 已补分析、财务、开户的直达入口，首页信息架构完成第二轮收口。
12. card-04 第四轮已推进：rules/index 已接入正式 operator-rules 服务层，规则域不再直接 request，规则页输入校验、编辑保护与服务层封装已形成完整闭环。
13. card-04 第一轮代码已落地：region-expansion 已改用 available regions 合同筛真实可申请区域，delivery-fee 已去掉默认 regionId=1 与本地伪字段，仅保留后端真实 DTO 字段。
14. card-04 与 card-05 已完成代码侧主链验收，剩余事项压缩为人工页面回归与 Phase 5 统一评分复核。

后续每张任务卡完成后，仍需补：

1. 定向页面回归。
2. 最小相关 lint / compile 结果。
3. loading、empty、error、retry 四态或五态人工确认。