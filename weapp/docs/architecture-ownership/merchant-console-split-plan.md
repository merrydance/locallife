# merchant-console split plan

## 当前观察

- 当前文件：weapp/miniprogram/services/merchant-console.ts
- 当前直接 importer：0
- 当前状态：已退化为受保护占位文件，用于阻止旧超级 service 导入路径复活

本计划原本针对一个承接 4 类 owner 的聚合文件：

- 商户工作台基础资料
- 营业状态切换
- 工作台概览统计
- 工单与投诉摘要
- 入驻状态视图
- App 绑定码生成

当前这 4 类 owner 已经拆出，`merchant-console.ts` 不再承接业务逻辑。

## 为什么现在就该做拆分计划

merchant-console 当前风险不在于已经过度扩散，而在于它最容易在后续迭代里继续吸收：

1. dashboard 新卡片继续顺手加一个 fetch 函数。
2. 设置页、设备页、财务页后续若复用工作台能力，继续直接导入该文件。
3. profile、open status、applyment、bind code 这几类 owner 本来不同，却被一个文件兜底承接。

所以它适合现在先做“防膨胀拆分计划”，而不是等它真的变成 merchant 超级 service 再回头清理。

## 拆分目标

目标不是机械拆函数，而是按 dashboard 主任务和可独立 owner 的能力边界收口。

### 目标 owner 边界

- dashboard 工作台聚合 owner
- 商户营业状态 owner
- 入驻与开店状态 owner
- App 绑定 owner

### 目标文件形态

建议收敛为：

- weapp/miniprogram/services/merchant-dashboard.ts
- weapp/miniprogram/services/merchant-open-status.ts
- weapp/miniprogram/services/merchant-applyment-console.ts
- weapp/miniprogram/services/merchant-app-bind.ts

如果第二阶段觉得过细，可至少先做到：

- merchant-dashboard.ts
- merchant-storefront-status.ts
- merchant-app-bind.ts

## 拆分方案

## 第一步：收回 dashboard 聚合 owner

当前状态：已完成。

从 merchant-console.ts 中抽出：

- `fetchMerchantConsoleOverview`
- `fetchMerchantConsoleOrderSummary`
- `fetchMerchantConsoleComplaintSummary`

落点：weapp/miniprogram/services/merchant-dashboard.ts

原因：

- 这些能力共同服务 merchant dashboard 的首页概览任务。
- 它们不应再和 profile、开关状态、绑定码混在一起。

当前进度：

- `merchant-dashboard.ts` 已创建。
- merchant dashboard 页面已改为直接依赖该 owner。

## 第二步：收回 storefront status owner

当前状态：已完成。

从 merchant-console.ts 中抽出：

- `fetchMerchantConsoleProfile`
- `fetchMerchantConsoleOpenStatus`
- `updateMerchantConsoleOpenStatus`

落点：weapp/miniprogram/services/merchant-open-status.ts

原因：

- 这三者共同服务“商户当前是否可营业、以什么状态呈现”的同一任务域。
- 它们会天然影响 dashboard，但不等于必须由 dashboard 聚合文件拥有。

当前进度：

- `merchant-open-status.ts` 已创建。
- merchant dashboard 页面与视图类型已改为直接依赖该 owner。

## 第三步：收回 applyment owner

当前状态：已完成。

从 merchant-console.ts 中抽出：

- `fetchMerchantApplymentStatusView`

落点：weapp/miniprogram/services/merchant-applyment-console.ts

原因：

- 入驻状态视图来自独立后端真值，属于独立业务链，不应长期埋在 dashboard 聚合里。

当前进度：

- `merchant-applyment-console.ts` 已创建。
- merchant dashboard 页面已改为直接依赖该 owner。

## 第四步：收回 app bind owner

当前状态：已完成。

从 merchant-console.ts 中抽出：

- `createMerchantAppBindCode`

落点：weapp/miniprogram/services/merchant-app-bind.ts

原因：

- 这是独立动作 owner，将来极可能被设置页或设备管理页复用。
- 若继续挂在 merchant-console.ts，后续跨页复用时最容易把这个文件升级成真正的超级 service。

当前进度：

- `merchant-app-bind.ts` 已创建。
- merchant dashboard 页面已改为直接依赖该 owner。

## 第五步：去掉 merchant-console 入口

当前状态：已完成。

当 dashboard 页面完成迁移后：

- dashboard 直接依赖上述按任务域拆开的 service
- `merchant-console.ts` 已退化为受保护占位文件

收口标准：

- 不再有页面直接导入 `services/merchant-console.ts`
- super-service gate 对 `merchant-console` 的 allowlist 已收缩到 0

## 不建议的错误拆法

- 不要把每个 API 再拆成一个 service 文件，让 dashboard 页面自己重新手拼
- 不要因为当前只有一个 importer，就把所有 dashboard 相关未来能力继续塞进 merchant-console
- 不要把 app bind、applyment、营业状态继续伪装成“首页小功能”而不承认它们各自有 owner

## 推荐实施顺序

1. 新建 `merchant-dashboard.ts`，先收走 overview、order summary、complaint summary。
2. 新建 `merchant-open-status.ts`，迁移营业状态相关入口。
3. 新建 `merchant-applyment-console.ts` 与 `merchant-app-bind.ts`。
4. merchant dashboard 页面改走新 service。
5. 删除 `merchant-console.ts` 的页面导入。
6. 收紧 super-service gate allowlist。

## 当前剩余风险

- 任务域切分与 importer 迁移已完成，但新 owner 文件未来仍需防止重新走向聚合膨胀。
- `merchant-console.ts` 作为占位文件应在合适时机进一步评估是否保留，或由更明确的规则层替代其“旧路径保护”职责。