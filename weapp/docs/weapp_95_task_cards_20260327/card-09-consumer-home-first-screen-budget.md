# CARD-09 消费首页首屏预算与数据装配优化

状态：进行中（代码完成，待人工回归）

优先级：P1

所属阶段：Phase 2

## 问题目标

控制消费首页首屏请求数，减少 per-item 请求扇出和弱网抖动。

## 影响范围

- [weapp/miniprogram/pages/takeout/index.ts](weapp/miniprogram/pages/takeout/index.ts)
- 相关商户、菜品和搜索 API 文件

## 任务内容

- [x] 梳理首屏请求预算和现状请求数。
- [x] 减少每个商户的同步补充请求数量。
- [x] 能批量化的字段优先批量化，不能批量化的改为渐进水合。
- [x] 评估主列表接入已有虚拟列表组件的必要性。

## 完成定义

- [ ] 首屏请求数明显下降。
- [ ] 首屏内容先稳定展示，再逐步补全细节。

## 验证要求

- [ ] 对比改造前后的首屏请求数。
- [ ] 人工验证弱网下首屏稳定性。

## 完成记录

- [x] 请求预算完成
- [x] 数据装配重构完成
- [ ] 弱网验证完成

补充说明：

- 外卖首页已去掉首次加载前的额外“是否有商户”探测请求，改为直接复用首页搜索结果回填 `hasServiceProviders`。
- 商户卡片水合已从“每页返回后立即对全部商户并发拉 3 个补充请求”改为“首屏优先批次 + 后台渐进批次”，优先加载首屏菜品，再延迟补详情和老顾客标识。
- 变更后已通过编辑器诊断、定向 ESLint 校验；`weapp` 全量 lint 和 compile 仍被仓库既有问题阻塞，其中 compile 当前卡在 [weapp/miniprogram/pages/register/operator/index.ts](weapp/miniprogram/pages/register/operator/index.ts)。