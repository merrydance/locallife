# 商户侧页面缺口表与实施清单

日期：2026-03-26

配套文档：
- `weapp/docs/merchant/MERCHANT_BACKEND_ALIGNMENT_MATRIX_2026-03-26.md`

## 文档目标

本文件回答两个问题：

1. 当前小程序商户侧页面，哪些能力已经覆盖，哪些只是部分覆盖，哪些完全缺失。
2. 如果按一次性大改实施，应该如何拆阶段、拆页面、拆文件和拆验收。

## 当前页面清单

当前商户页面目录：

- `merchant/dashboard`
- `merchant/orders/list`
- `merchant/orders/detail`
- `merchant/dishes`
- `merchant/dishes/edit`
- `merchant/dishes/categories`
- `merchant/combos`
- `merchant/combos/edit`
- `merchant/inventory`
- `merchant/merchant-categories`
- `merchant/delivery-promotions`
- `merchant/reservations`
- `merchant/tables`
- `merchant/printers`
- `merchant/claims`
- `merchant/claims/detail`
- `merchant/appeals`
- `merchant/finance`
- `merchant/profile-images`
- `merchant/config`

## 缺失页面清单

以下能力后端已开放，但当前商户页目录下没有对应页面：

1. `merchant/kitchen`
2. `merchant/complaints/index`
3. `merchant/complaints/detail`
4. `merchant/staff/index`
5. `merchant/stats/index`
6. `merchant/settings/business-hours`
7. `merchant/settings/membership`
8. `merchant/settings/profile`
9. `merchant/settings/application`
10. `merchant/settings/applyment`
11. `merchant/settings/display-config`

## 页面覆盖状态总表

### 评级说明

| 评级 | 含义 |
| --- | --- |
| A | 页面结构存在，核心动作已打通，只需重构设计和状态体系 |
| B | 页面存在，但只覆盖后端能力的一部分，需要补动作或补接口 |
| C | 页面存在，但本质仍是半成品，需要按能力域重做 |
| D | 后端已有能力但页面完全缺失 |

### 当前页面评估

| 页面 | 当前评级 | 主要问题 | 动作 |
| --- | --- | --- | --- |
| `merchant/dashboard` | B | 有工作台雏形，但统计、异常、分发入口不足 | 重构 |
| `merchant/orders/list` | B | 已承接主链路，但拒单/高优先级/履约细态不完整 | 重构 |
| `merchant/orders/detail` | B | 已承接详情，但动作闭环和履约追踪还不够完整 | 重构 |
| `merchant/dishes` | B | 已有列表，但批量、规格、标签经营化不足 | 重构 |
| `merchant/dishes/edit` | B | 表单存在，但定制项/规格/标签不完整 | 补齐 |
| `merchant/dishes/categories` | A | 结构基本合理 | 收口 |
| `merchant/combos` | B | 已有基础 CRUD，但组合关系表达不够完整 | 重构 |
| `merchant/combos/edit` | B | 套餐与菜品关联动作不完整 | 补齐 |
| `merchant/inventory` | B | 有基础页，但统计与经营信号不足 | 重构 |
| `merchant/merchant-categories` | B | 配置与设置域边界不清 | 并入设置重构 |
| `merchant/delivery-promotions` | A | 已有基础能力 | 收口 |
| `merchant/reservations` | B | 页面存在，但今日预订、代客创建、确认/爽约不完整 | 重构 |
| `merchant/tables` | B | 基础存在，但标签、图片、二维码链路不完整 | 重构 |
| `merchant/printers` | B | 列表与配置存在，但 display config 未系统承接 | 重构 |
| `merchant/claims` | B | 列表存在，但行为摘要与判责依据承接不足 | 重构 |
| `merchant/claims/detail` | B | 详情存在，但 recovery 和 decision 反馈需要收口 | 重构 |
| `merchant/appeals` | B | 有基础列表，但详情与结果态不足 | 重构 |
| `merchant/finance` | C | 只覆盖账户/提现/进件，缺少完整财务域 | 整页重做 |
| `merchant/profile-images` | B | 能力孤立，没有纳入统一设置域 | 并入设置重构 |
| `merchant/config` | C | 当前只是入口列表，不足以承载后端设置能力 | 整页重做 |
| `merchant/kitchen` | D | 后端有、API 有、页面无 | 新增 |
| `merchant/complaints/index` | D | 后端有、页面无 | 新增 |
| `merchant/complaints/detail` | D | 后端有、页面无 | 新增 |
| `merchant/staff/index` | D | 后端有、页面无 | 新增 |
| `merchant/stats/index` | D | 后端有、页面无 | 新增 |

## 分阶段实施清单

## Phase 0: 合同与基建准备

### 交付物

1. 后端能力对齐矩阵
2. 页面覆盖缺口表
3. 页面级 capability checklist 模板
4. 统一 merchant 状态与格式化层
5. 统一 merchant 页面壳

### 需要新增或重构的前端基础文件

| 文件建议 | 作用 |
| --- | --- |
| `weapp/miniprogram/components/merchant-page-shell/*` | 统一 loading/empty/error/skeleton 壳层 |
| `weapp/miniprogram/components/merchant-stat-card/*` | 首页/统计/财务复用统计卡 |
| `weapp/miniprogram/components/merchant-action-panel/*` | 统一主动作区域 |
| `weapp/miniprogram/components/merchant-status-banner/*` | 统一状态横幅 |
| `weapp/miniprogram/utils/merchant-formatters.ts` | 金额、时间、状态、角色、标签格式化 |
| `weapp/miniprogram/utils/merchant-permissions.ts` | 角色显隐与动作权限 |
| `weapp/miniprogram/utils/merchant-page-state.ts` | 列表页和详情页状态机 |

### Phase 0 验收

1. 新组件和工具层不依赖具体业务页面。
2. 状态文案不再散落在各页面文件中。
3. 至少有一个列表页和一个详情页切到统一页面壳。

## Phase 1: API 层合同化重构

### 必改文件

| 文件 | 动作 |
| --- | --- |
| `weapp/miniprogram/api/order-management.ts` | 收口商户订单与 KDS status/type 映射 |
| `weapp/miniprogram/api/merchant-finance.ts` | 扩展到完整 finance 域 |
| `weapp/miniprogram/api/merchant-stats.ts` | 扩展到完整 stats 域 |
| `weapp/miniprogram/api/table-device-management.ts` | 补 display config、二维码、图片、标签链路 |

### 需要新增的 API 文件

| 新文件建议 | 能力域 |
| --- | --- |
| `weapp/miniprogram/api/merchant-complaints.ts` | 投诉列表、详情、回复、完成 |
| `weapp/miniprogram/api/merchant-staff.ts` | 员工列表、邀请码、增删改角色 |
| `weapp/miniprogram/api/merchant-settings.ts` | 商户资料、营业状态、营业时间、会员设置 |
| `weapp/miniprogram/api/merchant-application.ts` | 申请草稿、基础信息、图片、提交、重置 |
| `weapp/miniprogram/api/merchant-risk.ts` | 用户风险摘要、判责辅助信息 |

### Phase 1 验收

1. 页面不再直接拼 URL。
2. 每个 API 文件按 capability 分域，不堆在单一超大文件中。
3. 同一能力域的 DTO 不再在多个页面重复定义。

## Phase 2: 核心交易履约域

### 页面范围

1. `merchant/dashboard`
2. `merchant/orders/list`
3. `merchant/orders/detail`
4. `merchant/kitchen` 新增
5. `merchant/reservations`
6. `merchant/tables`
7. `merchant/printers`

### 目标

1. 首页成为经营与任务分发中心。
2. 订单列表成为任务页，而不是单纯列表页。
3. 订单详情成为行动页，而不是只读信息页。
4. 后厨页正式承接 `kitchen` 能力。
5. 桌台和预订形成闭环。
6. 打印机与展示配置并入统一店务域。

### 页面级检查项

#### `merchant/dashboard`

必须新增或补齐：

1. 经营概览卡
2. 待处理任务入口
3. 异常提醒入口
4. KDS 入口
5. 财务/投诉/申诉/员工/设置快捷入口

#### `merchant/orders/list`

必须补齐：

1. 拒单入口
2. 更完整的订单状态细分
3. 风险单/超时单视觉信号
4. 局部骨架与失败重试
5. 主动作与次动作优先级收口

#### `merchant/orders/detail`

必须补齐：

1. 拒单动作
2. 履约轨迹细态
3. 下一步动作建议
4. 外卖联系与追踪动作
5. 失败回滚和刷新机制

#### `merchant/kitchen`

必须提供：

1. 新单/制作中/待取餐三栏或三段结构
2. 开始制作动作
3. 标记出餐动作
4. 超时风险高亮
5. 实时刷新与失败态

#### `merchant/reservations`

必须补齐：

1. 今日预订入口
2. 代客创建
3. 确认、完成、爽约动作
4. 菜品摘要与桌台联动
5. 状态筛选与统计视图

#### `merchant/tables`

必须补齐：

1. 桌台标签编辑
2. 桌台图片管理
3. 主图设置
4. 二维码查看/更新
5. 桌台状态与当前预订联动展示

#### `merchant/printers`

必须补齐：

1. display config
2. 打印类型开关
3. 测试打印反馈
4. 异常设备状态显示

## Phase 3: 菜单经营域

### 页面范围

1. `merchant/dishes`
2. `merchant/dishes/edit`
3. `merchant/dishes/categories`
4. `merchant/combos`
5. `merchant/combos/edit`
6. `merchant/inventory`
7. `merchant/delivery-promotions`
8. `merchant/merchant-categories`

### 目标

1. 让菜单页从后台维护页升级为经营控制台。
2. 让库存、类目、套餐、活动成为统一经营域，而不是互相孤立的管理页。

### 必做能力补齐

1. 菜品批量上下架
2. 菜品规格/定制项配置
3. 热卖/推荐标签
4. 套餐与菜品关联编辑
5. 库存统计信号
6. 活动配置与营销支出认知闭环

## Phase 4: 资金、风控、客服域

### 页面范围

1. `merchant/finance`
2. `merchant/claims`
3. `merchant/claims/detail`
4. `merchant/appeals`
5. `merchant/complaints/index` 新增
6. `merchant/complaints/detail` 新增
7. `merchant/stats/index` 新增

### 目标

1. 财务页从提现页升级为完整财务域。
2. 索赔/申诉页从记录页升级为判责与追偿处理页。
3. 投诉页补齐合规处理链路。
4. 统计页补齐经营分析链路。

### `merchant/finance` 必做项

1. 财务概览
2. 财务订单明细
3. 服务费明细
4. 营销支出
5. 日报
6. 结算记录
7. 结算时间线
8. 账户/提现/进件收口到同一 IA 中

### `merchant/claims` 与 `merchant/claims/detail` 必做项

1. 判责依据展示
2. 用户行为摘要展示
3. 追偿信息展示
4. 追偿支付动作
5. 与申诉动作串联

### `merchant/appeals` 必做项

1. 申诉列表按状态分段
2. 申诉详情与结果态
3. 与 claim detail 互链

### `merchant/complaints/*` 必做项

1. 投诉列表
2. 投诉详情
3. 商户回复
4. 完成投诉处理
5. 超时风险提示

### `merchant/stats/index` 必做项

1. 概览
2. 热销菜
3. 小时趋势
4. 复购率
5. 类目销售
6. 客户分析入口

## Phase 5: 设置与人员域

### 页面范围

1. `merchant/config`
2. `merchant/profile-images`
3. `merchant/staff/index` 新增
4. `merchant/settings/profile` 新增
5. `merchant/settings/business-hours` 新增
6. `merchant/settings/membership` 新增
7. `merchant/settings/application` 新增
8. `merchant/settings/applyment` 新增
9. `merchant/settings/display-config` 新增或并入 printers

### 目标

1. 配置中心变成设置导航页。
2. 资料、营业、会员、图资、进件、员工彻底解耦。

### `merchant/staff/index` 必做项

1. 员工列表
2. 邀请码生成
3. 角色变更
4. 删除员工
5. owner 专属动作和 manager 可做动作区分

## 文件级实施清单

### app 与路由

| 文件 | 动作 |
| --- | --- |
| `weapp/miniprogram/app.json` | 补齐新增 merchant 页面注册与导航顺序 |
| `weapp/miniprogram/pages/user_center/index.ts` | 调整商户域入口映射 |
| `weapp/miniprogram/pages/user/bind-merchant/index.ts` | 与 staff 邀请链路打通 |

### 页面公共层

| 文件 | 动作 |
| --- | --- |
| `weapp/miniprogram/components/custom-navbar/*` | 统一商户页头部体验 |
| `weapp/miniprogram/components/list-skeleton/*` | 统一列表骨架 |
| `weapp/miniprogram/components/dish-skeleton/*` | 统一菜品骨架 |
| `weapp/miniprogram/utils/responsive.ts` | 统一商户页安全区数据获取 |

### 页面迁移顺序建议

1. 先改 `dashboard` + `orders/list` + `orders/detail`
2. 再补 `kitchen`
3. 再改 `tables` + `reservations` + `printers`
4. 再改 `dishes` + `combos` + `inventory`
5. 再改 `finance` + `claims` + `appeals`
6. 再补 `complaints` + `stats` + `staff`
7. 最后重做 `config` 和所有 settings 二级页

## 每个页面的强制验收清单

每个页面都必须完成以下 12 项检查：

1. 页面对应的后端接口是否全部列出。
2. 页面角色边界是否与后端 middleware 一致。
3. 页面是否覆盖 loading。
4. 页面是否覆盖 success。
5. 页面是否覆盖 empty。
6. 页面是否覆盖 error。
7. 页面主动作是否有 submitting 态。
8. 页面失败后是否有 retry 或明确回退。
9. 页面状态文案是否来自统一映射层。
10. 页面是否存在后端已支持但无入口的能力遗漏。
11. 页面是否存在前端自定义但后端不支持的伪动作。
12. 页面是否使用统一卡片、留白、状态横幅和按钮优先级。

## 一次性大改的风险控制

### 风险

1. 页面数量多，容易只重画 UI 不补齐动作。
2. API 扩展量大，容易出现页面和合同层重复定义 DTO。
3. 新旧页面并存期间，容易出现导航入口混乱。
4. 角色权限处理不一致时，最容易在上线后暴露 403 问题。

### 控制措施

1. 所有页面先做 capability checklist，再开始写 UI。
2. 所有页面先接统一 API 层，再接页面状态。
3. 所有新增页面先在 `app.json` 中以临时隐藏入口方式挂载，再统一切流。
4. 每个阶段结束后做一次“后端路由对前端落点”的反查。

## 最终交付标准

只有同时满足以下条件，商户侧改造才算完成：

1. 后端商户能力全部有前端落点。
2. 现有 merchant 页面没有残留半成品页。
3. 新增页面全部纳入统一设计和状态体系。
4. 配置中心不再承载过多杂糅能力。
5. 财务、投诉、员工、KDS 这四类当前缺口能力全部补齐。
6. 运行 `npm run lint`、`npm run compile`、`npm run quality:check` 通过。

## 下一步直接可执行任务

1. 先建共享 merchant 页面壳和状态映射层。
2. 同时扩展 API 层，优先补 `merchant-finance.ts`、`merchant-stats.ts`、`merchant-complaints.ts`、`merchant-staff.ts`。
3. 以 `dashboard`、`orders/list`、`orders/detail` 作为第一批迁移页面。
4. 紧接着新增 `merchant/kitchen`，避免 KDS 能力继续悬空。
5. 然后切入 `finance` 和 `config`，解决当前最影响整体上限的两页。