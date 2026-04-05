# Rider 95+ 全量审计与修复执行计划

日期：2026-03-28

> 历史快照说明
>
> 本文是 2026-03-28 审计阶段形成的执行计划快照，保留的是当时的问题判断、页面分层和修复路径，不代表当前 rider 子包的现行注册页、页面职责或最终交付状态。
>
> 涉及当前真值时，请优先以 weapp/miniprogram/app.json、实际页面代码、后端真实路由，以及 weapp/docs/historical/pre-2026-04-05/rider/task-cards 下的最新任务卡和后续收口文档为准。

## 1. 目标

本计划用于把微信小程序骑手侧从“主入口能看到部分数据，但深链、异常、资金和孤岛页都存在真实接口漂移”的状态，提升到稳定 95 分以上。

本轮目标只有两条：

1. 完全对齐后端真实接口和真实权限语义。
2. 把骑手侧收敛成主链稳定、状态完备、职责清晰、无历史孤岛页污染的交付面。

## 2. 当前结论

基于 2026-03-28 的全量页面复核，骑手侧当前已经不是“有没有页面”的问题，而是进入“页面是否真的按后端真值运行”的阶段。

当前已经确认的高风险事实：

1. 任务详情页使用了仅订单 owner 可查看的接口，导致骑手主链在详情层直接断开。
2. 历史任务页对 /v1/delivery/history 使用了错误的 query contract，翻页失真。
3. 孤岛页存在整页 DTO 错位与职责漂移，当前不具备真实可用性。
4. 异常页、索赔页、申诉接口和追偿支付已经发生页面职责漂移，用户看到的是“异常上报”，真实走的是 claim/appeal 流程。
5. 仓库内同时存在 dashboard 主实现和 delivery/manage 旧实现两套骑手面，且后者仍是注册页面，说明信息架构已经漂移。
6. 资金页只接了充值和账单，后端已有提现能力但前端未承接。

结论：不能继续按单页修补推进，必须先锁定骑手侧唯一真相，再按主链、领域、孤岛治理三层推进。

## 3. 审查边界

### 3.1 注册页面范围

当前 rider 子包注册页面共 6 个，以 app.json 为准：

1. dashboard/index
2. claims/index
3. claims/detail/index
4. deposit/index
5. task-detail/index
6. tasks/index

### 3.2 主链与孤岛划分

当前应按真实使用强度分层：

1. 主链：dashboard/index、task-detail/index、tasks/index。
2. 次链：claims/index、claims/detail/index、deposit/index。
3. 孤岛页：无。

### 3.3 后端核对范围

每个页面至少反查以下一项或多项：

1. 小程序 API wrapper 是否真实映射后端路由。
2. request / response 字段是否按真实 JSON 消费。
3. 按钮和事件是否真正触发后端动作。
4. 页面 loading、success、empty、error、retry 是否完整。
5. 后端已提供但页面未落地的能力是否需要补进主链。

## 4. 评分标准

每个页面统一按 100 分评分，低于 95 分不得视为完成。

### 4.1 评分维度

1. 后端合同一致性：30 分
2. 动作闭环完整性：20 分
3. 页面职责清晰度：15 分
4. loading / success / empty / error / retry 完整性：10 分
5. 弱网与失败反馈：10 分
6. 路由与入口收口：10 分
7. 设计系统与可维护性：5 分

### 4.2 95 分达标标准

同时满足以下条件才算达标：

1. 页面字段、动作、状态机与后端真实合同一致。
2. 主链不再通过错误权限接口、错误分页参数或错误状态枚举运行。
3. claim、appeal、recovery 各自页面职责清晰，不再混用，已下线的异常报备伪需求不再占据 rider 页面职责。
4. 所有关键页面具备 loading、success、empty、error、retry 五态表达。
5. 已注册页面和真实入口一致，不再保留高风险孤岛实现。

## 5. 全域问题分类

### 5.1 P0 主链真值问题

1. task-detail/index 调错 owner-only 接口。
2. tasks/index 翻页合同错误。
4. claims/index 曾混入 appeal 与 recovery 动作，需继续以列表页职责维持收口。

### 5.2 P1 领域能力缺口

1. deposit/index 未接提现能力。
2. claims/index 与 claims/detail/index 需要继续维持列表、详情、追偿、申诉分层后的真实闭环。
3. dashboard/index 缺稳定历史与申诉入口收口。

### 5.3 P2 治理与体验问题

1. 主链多个页面只有 toast，没有页内 error/retry 壳。
2. dashboard/index 存在死代码导航和展示字段偏差。
3. 组件、service 和页面三层并行保存旧状态枚举，后续维护风险高。

## 6. 分阶段修复计划

### Phase 0：基线锁定

目标：锁定 rider 子包唯一真相，停止继续基于旧 service 或旧页面猜测后端语义。

二级任务：

1. 固化注册页面、入口状态、真实接口矩阵和评分卡。
2. 明确主链页面、次链页面和孤岛页边界。
3. 明确哪些页面保留重构，哪些页面迁移后删除。

产出：

1. rider Phase 0 基线台账。
2. rider 执行计划。
3. task cards 索引。

退出标准：

1. 后续改造不再依赖旧注释或错误 DTO 自证。

### Phase 1：主链路止血

目标：先把骑手主入口、任务详情和历史任务修到真实可跑。

二级任务：

1. 重做 task-detail/index 的详情数据源，不能再调用 owner-only 接口。
2. dashboard/index 修复主链跳转、展示字段、页内错误态和历史入口。
3. tasks/index 改回真实 page/limit 合同，接上分页和失败壳。
4. 收口所有配送状态流转后的回流和刷新策略。

退出标准：

1. 骑手从 dashboard 进入任务详情和历史任务不再断链。

### Phase 2：资金与索赔域重建

目标：把 deposit、claims 从“半成品页面”变成职责明确的骑手工具面。

二级任务：

1. deposit/index 补提现入口、失败回查和资金状态。
2. claims/index 保持 bucket 列表职责，不再混入申诉表单或伪异常历史。
3. claims/detail/index 承接责任判定、追偿支付、申诉提交与结果回看。

退出标准：

1. 资金与索赔相关页面各自有唯一职责页面。

### Phase 3：孤岛页治理与历史实现收口

目标：处理旧 service、死导航和历史文档等污染面，避免继续误导开发和测试。

二级任务：

1. claims/index 与 claims/detail/index 的入口和跳转边界保持稳定。
2. 清理错误的 service、旧状态枚举、未绑定交互和死导航。
3. 收口所有已下线页面的注册状态与残留文档描述。

退出标准：

1. rider 子包不再存在注册但不可用的高风险旧页面。

### Phase 4：统一验证与 95 分收口

目标：完成真实接口回归、弱网回归和最终评分。

二级任务：

1. 对 dashboard、task-detail、tasks、deposit、claims、claims/detail 做五态验证。
2. 对抢单、取餐、送达、充值、提现、追偿支付、申诉提交做主链验证。
3. 检查 rider 子包的所有注册页是否都有明确入口或明确下线决策。
4. 清理残留 dead route、旧 service、备份逻辑依赖。

退出标准：

1. rider 子包统一评分达到 95 分以上。
2. 不再存在已知 P0 / P1 合同漂移问题。

## 7. 任务卡拆分

详细任务卡见：

1. weapp/docs/historical/pre-2026-04-05/rider/task-cards/README.md
2. weapp/docs/historical/pre-2026-04-05/rider/task-cards/card-01-rider-dashboard-and-history-contract.md
3. weapp/docs/historical/pre-2026-04-05/rider/task-cards/card-02-rider-task-detail-and-status-closure.md
4. weapp/docs/historical/pre-2026-04-05/rider/task-cards/card-04-rider-exception-claim-appeal-split.md
5. weapp/docs/historical/pre-2026-04-05/rider/task-cards/card-05-rider-wallet-withdraw-closure.md
6. weapp/docs/historical/pre-2026-04-05/rider/task-cards/card-06-rider-orphan-pages-and-legacy-manage-cleanup.md
7. weapp/docs/historical/pre-2026-04-05/rider/task-cards/card-07-rider-runtime-validation-and-weak-network-regression.md

## 8. 验证基线

本轮审计阶段已完成：

1. rider 子包 6 个注册页面静态复核。
2. 对应前端 service 与后端真实路由、handler、关键 logic 的逐层核对。
3. 对关键 TypeScript 页面做编辑器诊断检查，未发现编译级错误。

本轮未完成：

1. 未做真机或开发者工具交互回归。
2. 未重新执行 npm run quality:check，因为本轮产物是文档与任务拆分，不是代码变更。

后续每张任务卡完成后，仍需补：

1. 最小相关页面交互验证。
2. 小程序质量检查命令。
3. 主链弱网和失败重试验证。