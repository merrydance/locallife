# Rider Task Cards

日期：2026-03-28

> 历史快照说明
>
> 本文是 2026-03-28 阶段的 rider 任务卡索引快照，保留的是当时的任务拆分和执行顺序，不代表当前 rider 子包所有任务边界、阶段命名或优先级仍与文中完全一致。
>
> 涉及当前真值时，请优先以实际代码、当前注册配置、后端真实路由，以及任务卡正文和后续收口文档为准。

## 1. 目标

把骑手侧的大问题拆成可以逐张勾选、逐张回归的小卡，避免继续以“整包大改”的方式推进。

## 2. 执行顺序

### Phase 1：主链路止血

1. card-01-rider-dashboard-and-history-contract.md
2. card-02-rider-task-detail-and-status-closure.md

### Phase 2：资金与异常域重建

3. card-04-rider-exception-claim-appeal-split.md
4. card-05-rider-wallet-withdraw-closure.md

### Phase 3：孤岛页治理

5. card-06-rider-orphan-pages-and-legacy-manage-cleanup.md

### Phase 4：统一验证收口

6. card-07-rider-runtime-validation-and-weak-network-regression.md

## 3. 使用规则

1. 每张卡只处理一个明确主题，不跨域吞并。
2. 未完成回归前，不得把卡标记为完成。
3. 对注册页的删除、迁移或下线决策必须在卡内写清楚，不得口头约定。