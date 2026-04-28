# 运营端小程序分层收敛任务卡

## 目标

把运营端小程序相关页面继续收敛到“页面 -> 领域服务 -> API 封装”的结构，避免页面直接按后端接口平铺实现。

## 范围边界

- 只处理运营端小程序页面与其直接依赖的 service/api。
- 本轮优先收口通知中心，再盘点并分批迁移旧页面。
- 不在本轮改 UI 视觉结构，不顺手重写无关页面。

## 任务卡

- [x] A. 收口通知中心服务层
  - 页面不再直接调用通知 API。
  - 列表、详情、单条已读、全部已读统一由通知领域服务暴露。
  - 保持现有交互和行为不变。
  - 验证：`npm run compile`、`npm run quality:check` 通过。

- [x] B. 盘点运营页分层现状
  - 输出 operator 页面分层清单。
  - 标明已合规、部分合规、未合规页面。
  - 验证：基于 `pages/operator/**/*.ts` import 盘点，并抽样复核代表页实现。

### B. 盘点结果

#### 已合规

- `pages/operator/dashboard/index`
- `pages/operator/analytics/index`
- `pages/operator/dispatch-hall/index`
- `pages/operator/notifications/index`
- `pages/operator/notifications/detail/index`

判定口径：页面不再直接依赖 API 层，数据编排和视图适配已下沉到领域 service。

#### 部分合规

- `pages/operator/region/index`
- `pages/operator/rules/index`
- `pages/operator/delivery-fee/index`
- `pages/operator/timeslot/index`
- `pages/operator/region/config`
- `pages/operator/finance/withdraw/index`
- `pages/operator/safety/report/index`

判定口径：页面仍然直接依赖 API，但大多是单域查询或单域读写，迁移成本相对可控。

#### 未合规

- `pages/operator/region-expansion/index`
- `pages/operator/riders/index`
- `pages/operator/riders/detail/index`
- `pages/operator/merchants/index`
- `pages/operator/merchants/detail/index`
- `pages/operator/safety/detail/index`

判定口径：页面直接依赖 API，且包含明显的业务编排、多接口串联、页面内适配或提交流程。

#### 迁移优先级建议

- 第一批：`riders` 域，边界清楚，列表和详情页都已形成稳定接口。
- 第二批：`merchants` 域，结构与 riders 相近，但数据面更宽。
- 第三批：`region-expansion` 与 `safety-detail`，因提交流程更重，单独收口。

- [x] C. 迁移骑手页到服务层
  - 骑手列表页、详情页不再直接依赖 API。
  - 页面只保留状态、交互、导航。
  - 验证：`npm run compile`、`npm run quality:check` 通过。

- [x] D. 迁移次级旧页
  - 在 merchants 和 rules 中择一推进。
  - 不并批扩散。
  - 本轮选择：`merchants`
  - 验证：`npm run compile`、`npm run quality:check` 通过。

- [x] E. 复核运营端分层结果
  - 复查是否仍有明显页面直调 API 热点。
  - 明确剩余遗留点，不虚报完成度。
  - 验证：复扫 `pages/operator/**/*.ts` 的 `/api/` 与 `/services/` 引用面。

- [x] F. 迁移配置域页面到服务层
  - 收口 `region/index`、`rules/index`、`delivery-fee/index`、`timeslot/index`、`region/config`。
  - 区域选择复用 `operator-regions`；规则与区域配置分别建立 service owner。
  - 验证：`npm run compile`、`npm run quality:check` 通过。

- [x] G. 收口剩余页面到服务层
  - 收口 `safety/report`、`safety/detail`、`region-expansion/index`、`finance/withdraw/index`。
  - 新增 `operator-safety`、`operator-region-expansion`、`operator-finance` 三个 service owner。
  - 验证：`npm run compile`、`npm run quality:check` 通过。

- [x] H. 复核 service 内部重复逻辑
  - 盘点 operator service 内部重复适配逻辑，重点检查金额格式化、时间显示、分页返回形态和命名边界。
  - 命名边界维持现状，不额外抽取跨域 super helper。
  - 仅把重复的“分转元/带符号金额”收敛到既有 `utils/util.ts`，避免在多个 service 内重复实现。
  - 时间显示保留 service 内局部语义，不为了去重强行共用一套格式规则。
  - 验证：`npm run compile` 通过。

### E. 最终复核结果

#### 当前已走 service 的页面

- `pages/operator/dashboard/index`
- `pages/operator/analytics/index`
- `pages/operator/dispatch-hall/index`
- `pages/operator/notifications/index`
- `pages/operator/notifications/detail/index`
- `pages/operator/riders/index`
- `pages/operator/riders/detail/index`
- `pages/operator/merchants/index`
- `pages/operator/merchants/detail/index`
- `pages/operator/region/index`
- `pages/operator/rules/index`
- `pages/operator/delivery-fee/index`
- `pages/operator/timeslot/index`
- `pages/operator/region/config`
- `pages/operator/region-expansion/index`
- `pages/operator/finance/withdraw/index`
- `pages/operator/safety/report/index`
- `pages/operator/safety/detail/index`

#### 当前仍直连 API 的页面

- 无

#### 结论

- 本轮已把通知、骑手、商户、配置域、食安、扩区、财务页面全部收口到 service 层。
- `pages/operator/**/*.ts` 已不存在页面直连 API 的实现面。
- 当前运营端页面层已统一收敛到“页面 -> 领域 service -> API 封装”的结构。
- service 层命名边界当前基本稳定，后续重点不再是页面分层，而是只在确有收益时继续做低风险局部去重。
- 本轮 service 审计仅收敛了重复金额格式化，没有把不同页面语义的时间展示强行抽成统一 helper，避免新的耦合和展示漂移。

## 审计要求

- 每完成一项即在本文件勾选。
- 每项任务都记录验证方式。
- 若中途发现边界扩大，先回写任务卡再继续。

## 当前状态

- 当前状态：本轮任务已完成
- 后续建议：如果继续推进，重点转到 service 内部复用治理和跨域复合页是否需要进一步归并，但应坚持“只收敛高重复、低语义分歧”的局部原则，不再把页面直连 API 当作当前主要问题。