# Operator Phase 0 基线台账

日期：2026-03-28

## 1. 用途

这份台账用于把运营侧从“聊天里的排查结论”变成“可持续执行的基线清单”。

使用原则：

1. 每个注册页面至少核对一次真实 API wrapper 和动作闭环。
2. 每个页面都要标记路由状态、合同状态、缺失功能和优先级。
3. 孤儿页面也必须入账，避免历史实现继续隐性承载真实能力。

## 2. 基线快照

1. operator 子包注册页面：18 个。
2. 孤儿页面：2 组，分别是 dashboard/dashboard 和 merchants/list/list。
3. 当前整体评分：64 / 100。
4. 当前已确认的共性重点：统计真值、分页合同、DTO 漂移、路由收口、运营工具能力缺口。

## 3. 全局已确认结论

1. dashboard/index 与 analytics/index 的统计口径没有统一，当前不满足运营看板可信度要求。
2. merchants/index 与 riders/index 只完成了最小列表展示，尚未完整暴露后端已支持的搜索、筛选、动作能力。
3. appeal/detail/index 仍停留在简化版详情，没有承接后端 timeline、证据、关联订单等完整信息。
4. delivery-fee/index 与 region/config、timeslot/index、rules/index 存在重复域建模，尚未收敛唯一职责。
5. safety/report/index 当前只做列表，operatorBasicManagementService 已支持 submitSafetyReport，但页面没有入口。
6. finance/withdraw/index 只做提现申请，operatorBasicManagementService 已支持佣金明细，但页面没有消费。
7. region-expansion/index 使用的是 listRegions 而非 listAvailableRegions，无法保证只申请可申请区域。
8. 已注册页面与孤儿页并存，且孤儿页承载了更完整的商户管理能力，说明信息架构已漂移。

## 4. 页面台账

状态说明：

1. 路由状态：注册 / 孤儿
2. 合同核查：未开始 / 部分完成 / 已完成
3. 优先级：P0 / P1 / P2

| 页面 | 路由状态 | 领域 | 优先级 | 合同核查 | 评分 | 关键问题 | 功能缺失 |
| --- | --- | --- | --- | --- | --- | --- | --- |
| dashboard/index | 注册 | 运营首页 | P0 | 部分完成 | 72 | 第一轮已修正统计口径与待办入口分流；仍缺统一待办聚合页和区域钻取 | 缺真正的全量待办工作台；缺区域统计钻取 |
| analytics/index | 注册 | 全局分析 | P0 | 部分完成 | 68 | 第一轮已修正近 7 天聚合；仍只有商户榜，分析粒度偏粗 | 缺骑手榜、区域统计、趋势聚合筛选 |
| merchants/index | 注册 | 商户列表 | P0 | 部分完成 | 66 | 第一轮已接待审筛选与 total 分页；线上页能力仍弱于孤儿页 | 缺搜索、显式筛选 UI、暂停、恢复、状态批量治理 |
| merchants/detail/index | 注册 | 商户详情 | P0 | 部分完成 | 56 | 通过 as unknown as 吞掉 DTO 漂移；字段与服务层契约不一致 | 缺商户动作面板、更多真实经营字段、详情回流 |
| riders/index | 注册 | 骑手列表 | P0 | 部分完成 | 63 | 第一轮已接待审筛选与 total 分页；仍直接 request 绕过服务层 | 缺搜索、筛选 UI、暂停、恢复、排行联动 |
| riders/detail/index | 注册 | 骑手详情 | P0 | 部分完成 | 50 | 页面字段与服务 DTO 严重漂移；直接 request 获取详情 | 缺暂停、恢复、证件字段统一、位置与绩效完整展示 |
| appeal/list/index | 注册 | 申诉列表 | P0 | 部分完成 | 55 | 只取第一页；未接 has_more；状态集合过窄 | 缺分页、更多状态筛选、统计摘要 |
| appeal/detail/index | 注册 | 申诉详情 | P0 | 部分完成 | 57 | 使用简化 AppealResponse；未消费 operator detail 全量字段 | 缺 timeline、evidence_files、related_order、完整状态流 |
| finance/withdraw/index | 注册 | 财务提现 | P1 | 部分完成 | 72 | 仅覆盖概览与提现；资金域过窄 | 缺佣金明细、提现记录、状态筛选 |
| applyment/index | 注册 | 微信开户 | P1 | 部分完成 | 76 | 状态映射仍较薄；与 operator application 状态机尚需统一 | 缺更细状态说明、开户结果回流与指引闭环 |
| region/index | 注册 | 区域选择 | P1 | 部分完成 | 63 | hasMore 通过 length 推断；存在添加区域假动作 | 缺权限矩阵、区域统计摘要、真实新增路径 |
| region/config | 注册 | 区域配送配置 | P1 | 部分完成 | 61 | 同页承载基础运费和峰时管理，职责偏重；字段校验较弱 | 缺区域摘要、操作保护、局部保存反馈细化 |
| timeslot/index | 注册 | 时段系数 | P2 | 部分完成 | 74 | 仅做新增与删除；缺编辑和更细校验 | 缺编辑现有时段、冲突检测、时段排序 |
| delivery-fee/index | 注册 | 配送费配置 | P0 | 部分完成 | 38 | 与真实 DTO 不一致；本地 mock unknown fields；默认 regionId=1；与 region/config 重复 | 应判定为下线或重构；当前不应继续作为真实入口 |
| rules/index | 注册 | 规则中心 | P1 | 部分完成 | 70 | 使用 raw request；数值编辑保护较弱；区域上下文单点传递 | 缺更细分类校验、变更审计、只读字段解释 |
| safety/report/index | 注册 | 食安事件列表 | P1 | 部分完成 | 62 | 只有列表，没有错误壳；后端 submit 能力未接入 | 缺提交事件入口、详情摘要、页内 error/retry 壳 |
| safety/detail/index | 注册 | 食安事件详情 | P1 | 部分完成 | 64 | 已处理态与待处理态未分离；错误壳较弱 | 缺只读模式、证据预览、恢复结果回流说明 |
| region-expansion/index | 注册 | 扩区申请 | P1 | 部分完成 | 59 | 未使用 available regions 合同；可申请区域筛选不可信 | 缺可申请区域过滤、申请前校验、分页与筛选 |
| dashboard/dashboard | 孤儿 | 旧运营工作台 | P0 | 部分完成 | 24 | 未注册；第一轮已清理孤儿商户页死链，但页面本身仍属历史实现 | 应迁移有效能力或删除 |
| merchants/list/list | 孤儿 | 旧商户管理页 | P0 | 部分完成 | 35 | 未注册；却承载搜索、筛选、暂停、恢复等真实能力 | 应迁回注册页或删除 |

## 5. 页面到接口映射速览

| 页面 | 当前主要接口 | 审计结论 |
| --- | --- | --- |
| dashboard/index | operator-basic-management, operator-analytics, operator-merchant-management, operator-rider-management | 统计与待办聚合逻辑需重做 |
| analytics/index | operator-analytics, operator-merchant-management | 需要补真正的周期聚合与区域分析 |
| merchants/index | operator-merchant-management | 接口只接了 getMerchantList，未落地动作能力 |
| merchants/detail/index | operator-merchant-management | DTO 使用与服务层声明不一致 |
| riders/index | request + /v1/operator/riders | 应统一改回 operatorRiderManagementService |
| riders/detail/index | request + operator-rider-management | 详情字段需按 OperatorRiderDetailResponse 重构 |
| appeal/list/index | appeals-customer-service | 需接分页合同并复核状态全集 |
| appeal/detail/index | appeals-customer-service, claimManagementService | 需改用更完整的 operator appeal detail 语义 |
| finance/withdraw/index | operator-basic-management, operator-finance | 需补佣金列表与提现记录 |
| applyment/index | operator-applyment | 需统一开户与入驻状态机 |
| region/index | operator-basic-management | 需去掉假动作并按真实区域权限展示 |
| region/config | delivery-fee | 可保留，但需和 delivery-fee/index 去重 |
| timeslot/index | delivery-fee | 基本可用，需补编辑能力 |
| delivery-fee/index | delivery-fee | 当前应视为重复实现高风险页 |
| rules/index | /v1/operator/rules raw request | 可保留，但需补校验与服务层封装 |
| safety/report/index | operator-basic-management | 需补 submitSafetyReport 对应入口 |
| safety/detail/index | operator-basic-management | 需补态区分、证据展示与错误壳 |
| region-expansion/index | operator-application | 需改用 listAvailableRegions |

## 6. 后端已提供但前端未完整落地的能力

1. 骑手暂停与恢复：operator-rider-management 已提供 suspendRider 和 resumeRider，注册页面未消费。
2. 运营佣金明细：operator-basic-management 已提供 getCommissionList，财务页未消费。
3. 食安事件提交：operator-basic-management 已提供 submitSafetyReport，列表页未提供入口。
4. 区域统计：operator-basic-management 和 operator-analytics 都提供 getRegionStats，但注册页面没有单独钻取或展示。
5. 申诉详情全量信息：operator-analytics 定义了 related_order、timeline、evidence_files，当前详情页未消费。
6. 可申请区域筛选：operator-application 已提供 listAvailableRegions，扩区页未消费。

## 7. 下一批执行顺序

1. dashboard/index 与 analytics/index：先修统计真值。
2. merchants/index、riders/index、appeal/list/index：统一分页、筛选、动作合同。
3. merchants/detail/index、riders/detail/index、appeal/detail/index：收口 DTO 与详情能力。
4. delivery-fee/index、region/config、rules/index、timeslot/index：统一区域规则域职责。
5. finance/withdraw/index、safety/report/index、region-expansion/index：补已存在后端能力的页面入口。