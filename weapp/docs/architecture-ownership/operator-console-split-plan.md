# operator-console split plan

## 当前观察

- 当前文件：weapp/miniprogram/services/operator-console.ts
- 当前直接 importer：0
- 当前状态：已退化为受保护占位文件，用于阻止旧超级 service 导入路径复活

本计划原本针对一个混合了 3 类 owner 的聚合文件：

- 区域选择共享 owner
- 分析页 owner
- 运营中心 dashboard owner

当前这 3 类 owner 已经拆出，`operator-console.ts` 不再承接业务逻辑。

## 为什么现在就该拆

当前文件同时依赖这些后端能力组：

- operator-basic-management
- operator-analytics
- operator-merchant-management
- operator-rider-management

风险不在于行数，而在于下面三类漂移已经具备土壤：

1. 新增运营能力最容易继续顺手挂到这个文件里。
2. dashboard 和 analytics 的 view model、错误承接、恢复策略以后会继续互相污染。
3. 一个页面组为了省事改共享聚合时，另一个页面组会被被动带上变更半径。

## 拆分目标

目标不是把一个超级 service 拆成更多小碎片，而是把 owner 收回到正确页面组。

### 目标 owner 边界

- 区域选择共享：单独 owner
- analytics 页面组：单独 owner
- dashboard 页面组：单独 owner

### 目标文件形态

建议收敛为：

- weapp/miniprogram/services/operator-regions.ts
- weapp/miniprogram/services/operator-analytics-dashboard.ts
- weapp/miniprogram/services/operator-workbench.ts

命名原则：

- 以任务域或页面组命名。
- 不再用 `operator-console` 这种容易继续吸收能力的总括名。

## 拆分方案

## 第一步：先抽共享区域选择 owner

当前状态：已完成。

从 operator-console.ts 中抽出：

- `ConsolePickerOption`
- `ConsoleRegionOption`
- `ConsoleRegionPickerState`
- `buildRegionPickerState`
- `mapRegions`
- `loadOperatorRegions`

落点：weapp/miniprogram/services/operator-regions.ts

原因：

- 这是当前两个页面唯一明确共享、且边界稳定的能力。
- 它不应该继续和 dashboard 聚合、analytics 聚合绑在一个文件里。

当前进度：

- `operator-regions.ts` 已创建。
- operator dashboard 与 analytics 页面已改为直接依赖该共享 owner。

## 第二步：收回 analytics owner

当前状态：已完成。

从 operator-console.ts 中抽出：

- `OperatorAnalyticsMetric`
- `OperatorAnalyticsRegionSummary`
- `OperatorMerchantRankingView`
- `OperatorRiderRankingView`
- `OperatorAnalyticsPageData`
- `loadOperatorAnalyticsPageData`

落点：weapp/miniprogram/services/operator-analytics-dashboard.ts

原因：

- analytics 页当前只需要区域切换、分析指标、区域摘要、排行视图。
- 它不应被 dashboard 的待审汇总、财务概览、中心卡片聚合拖在一起。

## 第三步：收回 dashboard owner

当前状态：已完成。

从 operator-console.ts 中抽出：

- `OperatorPendingSummary`
- `OperatorPendingApprovalItem`
- `OperatorCenterStats`
- `OperatorCenterFinance`
- `OperatorCenterRiderRankingItem`
- `OperatorCenterPageData`
- `loadOperatorCenterPageData`

落点：weapp/miniprogram/services/operator-workbench.ts

原因：

- dashboard 页有自己的主任务：运营中心概览、待审入口、财务摘要、排行摘要。
- 这些数据与 analytics 页虽然都属于 operator，但不是同一个页面组任务。

## 第四步：去掉 operator-console 入口

当前状态：已完成。

当两个页面都完成迁移后：

- operator dashboard 改为只依赖 `operator-regions.ts` 和 `operator-workbench.ts`
- operator analytics 改为只依赖 `operator-regions.ts` 和 `operator-analytics-dashboard.ts`
- `operator-console.ts` 已退化为受保护占位文件

收口标准：

- 不再有页面直接导入 `services/operator-console.ts`
- super-service gate 对 `operator-console` 的 allowlist 已收缩到 0

## 不建议的错误拆法

- 不要把纯格式化 helper 再抽成新的 `operator-console-shared.ts`
- 不要把 merchant ranking、rider ranking、finance、pending 各拆成过细 service，最后让页面重新手工拼装
- 不要为了复用把 analytics 和 dashboard 再收回一个新的 `operator-home.ts`

## 推荐实施顺序

1. 新建 `operator-regions.ts`，先迁移两个页面的 region picker 依赖。
2. 新建 `operator-analytics-dashboard.ts`，迁移 analytics 页面。
3. 新建 `operator-workbench.ts`，迁移 dashboard 页面。
4. 删除 `operator-console.ts` 的页面导入。
5. 收紧 super-service gate allowlist。

## 当前剩余风险

- 任务域切分与 importer 迁移已完成，但新 owner 文件未来仍需防止重新走向聚合膨胀。
- `operator-console.ts` 作为占位文件应在合适时机进一步评估是否保留，或由更明确的规则层替代其“旧路径保护”职责。