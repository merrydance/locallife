# Weapp 95 分提升任务卡索引

日期：2026-03-27

本目录基于 [weapp/docs/WEAPP_95_SCORE_IMPROVEMENT_PLAN_2026-03-27.md](weapp/docs/WEAPP_95_SCORE_IMPROVEMENT_PLAN_2026-03-27.md) 继续拆分，目标是把五端提升计划落成可直接领取、排期、评审和验收的任务卡。

## 使用方式

- 每张任务卡尽量满足“单人或单小组可在一轮迭代内交付”。
- 任务卡完成后，直接回填状态、PR 链接、验证结果和残余风险。
- Phase 1 的 P0 止血项完成前，不应宣称 weapp 已进入 95 分提升阶段。
- 每张任务卡都必须输出对应的自动化校验和人工回归结果。

## 任务卡列表

### Phase 0：跨端治理基线

- [ ] [CARD-01 路由与页面注册总排查](weapp/docs/weapp_95_task_cards_20260327/card-01-route-and-page-registry-audit.md)
- [ ] [CARD-02 页面动作到 API 闭环清单](weapp/docs/weapp_95_task_cards_20260327/card-02-button-api-closure-matrix.md)
- [ ] [CARD-03 列表筛选与分页合同治理](weapp/docs/weapp_95_task_cards_20260327/card-03-pagination-and-filter-contract.md)
- [ ] [CARD-04 实时订阅生命周期模板收口](weapp/docs/weapp_95_task_cards_20260327/card-04-realtime-lifecycle-template.md)

### Phase 1：P0 止血项

- [ ] [CARD-05 修复骑手首页冷启动实时链路](weapp/docs/weapp_95_task_cards_20260327/card-05-rider-dashboard-cold-start-realtime.md)
- [ ] [CARD-06 修复堂食扫码入口死链](weapp/docs/weapp_95_task_cards_20260327/card-06-dinein-scan-route-repair.md)
- [ ] [CARD-07 商户营业状态切换真实化](weapp/docs/weapp_95_task_cards_20260327/card-07-merchant-business-status-contract.md)
- [ ] [CARD-08 修复商户预订订单过滤与分页错位](weapp/docs/weapp_95_task_cards_20260327/card-08-merchant-reservation-order-contract.md)

### Phase 2：高频页性能与状态完备性

- [ ] [CARD-09 消费首页首屏预算与数据装配优化](weapp/docs/weapp_95_task_cards_20260327/card-09-consumer-home-first-screen-budget.md)
- [ ] [CARD-10 消费侧搜索与结果状态完备性](weapp/docs/weapp_95_task_cards_20260327/card-10-consumer-search-and-result-states.md)

### Phase 3：五端能力补齐

- [ ] [CARD-11 商户侧能力缺口补齐批次](weapp/docs/weapp_95_task_cards_20260327/card-11-merchant-capability-gap-batch.md)
- [ ] [CARD-12 运营商侧一致性补齐批次](weapp/docs/weapp_95_task_cards_20260327/card-12-operator-consistency-batch.md)
- [ ] [CARD-13 平台侧控制面板提升批次](weapp/docs/weapp_95_task_cards_20260327/card-13-platform-control-surface-batch.md)
- [ ] [CARD-14 骑手侧钱包与异常链路提升批次](weapp/docs/weapp_95_task_cards_20260327/card-14-rider-wallet-and-exception-batch.md)

### Phase 4：统一收口

- [ ] [CARD-15 五端 95 分验收与回归收口](weapp/docs/weapp_95_task_cards_20260327/card-15-five-end-95-validation.md)

## 推荐执行顺序

1. 先完成 CARD-01 到 CARD-04，建立统一治理基线。
2. 立即推进 CARD-05 到 CARD-08，把直接影响业务正确性的缺陷止血。
3. 再执行 CARD-09 和 CARD-10，解决消费首页与搜索页的高频体验问题。
4. 然后并行执行 CARD-11 到 CARD-14，分别补齐商户、运营商、平台、骑手能力。
5. 最后执行 CARD-15，统一做五端评分和弱网回归。

## 执行拆分

- [ ] [Phase 0/1 开发顺序图与并行依赖图](weapp/docs/weapp_95_task_cards_20260327/phase-0-1-delivery-map.md)

## 完成标准

- [ ] Phase 0 完成
- [ ] Phase 1 完成
- [ ] Phase 2 完成
- [ ] Phase 3 完成
- [ ] Phase 4 完成
- [ ] 五端统一评分达到 95 分以上