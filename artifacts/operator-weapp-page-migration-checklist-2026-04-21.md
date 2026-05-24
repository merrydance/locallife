# 运营商小程序页面改造清单（基于当前后端真实开放能力）

## 目的

这份文档把当前运营商小程序页面与后端真实开放能力做一一对表，方便后续围绕具体页面、具体接口、具体漂移点继续讨论。

本次判断遵循两个原则：

1. 以后端真实路由注册为准，核心依据是 `locallife/api/server.go`。
2. 不把前端 service 假设、旧注释、Swagger 漂移、历史残留路径当成真实能力。

## 总体结论

当前运营商小程序页面可以分成三类：

## 已确认产品决策

这几条是本轮讨论里已经明确敲定的边界，后续默认按这个方向推进：

### 1. 入驻页选区走通用 region 查询链路

- 入驻页的省、市、区县选择，不走运营商名下区域接口。
- 应直接调用后端 region 表对应的通用查询接口，按省、市、区县逐级选择。
- 这个场景是“选择申请目标区域”，不是“查询运营商已管理区域”。

### 2. 运营商区域列表按单页处理

- 运营商名下区域通常不会很多。
- 运营商区域页按单页展示即可。
- 前端不需要继续围绕复杂分页、无限加载来设计交互。

### 3. 提现能力整体取消

- 新流程已经调整为直接分账到运营商零钱。
- 因此不再存在运营商提现能力。
- 财务页后续只保留查看能力，不再保留提现产品心智。

### 一类：可以直接保留并继续改体验

- 运营商注册入口页 `pages/register/operator/index`
- 运营分析页 `pages/operator/analytics/index`
- 运营规则页 `pages/operator/rules/index`
- 区域选择页 `pages/operator/region/index`
- 区域配置摘要页 `pages/operator/region/config`
- 代取费页 `pages/operator/delivery-fee/index`
- 峰时时段页 `pages/operator/timeslot/index`
- 食安案件列表页 `pages/operator/safety/report/index`
- 食安案件详情页 `pages/operator/safety/detail/index`
- 区域扩展申请页 `pages/operator/region-expansion/index`

### 二类：主页面可保留，但要先删掉漂移功能

- 运营中心首页 `pages/operator/dashboard/index`
- 商户列表页 `pages/operator/merchants/index`
- 商户详情页 `pages/operator/merchants/detail/index`
- 骑手列表页 `pages/operator/riders/index`
- 骑手详情页 `pages/operator/riders/detail/index`
- 财务页 `pages/operator/finance/withdraw/index`

### 三类：建议直接下线，或者等待后端补口后重做

- 申诉列表页 `pages/operator/appeal/list/index`
- 申诉详情页 `pages/operator/appeal/detail/index`

## 后端真实能力范围

以下是当前已经确认真实开放给运营商的能力域。

### 申请与扩区

- `GET /v1/operator/application`
- `POST /v1/operator/application`
- `PATCH /v1/operator/application/basic`
- `PATCH /v1/operator/application/region`
- `POST /v1/operator/application/submit`
- `POST /v1/operator/application/reset`
- `POST /v1/operator/region-expansion`
- `GET /v1/operator/region-expansion`

### 区域与统计

- `GET /v1/operator/regions`
- `GET /v1/operator/regions/:region_id/stats`
- `GET /v1/operator/stats/realtime`
- `GET /v1/operator/trend/daily`
- `GET /v1/operator/merchants/ranking`
- `GET /v1/operator/riders/ranking`

### 商户与骑手管理

- 商户：`GET /v1/operator/merchants`
- 商户：`GET /v1/operator/merchants/summary`
- 商户：`GET /v1/operator/merchants/:id`
- 商户：`GET /v1/operator/merchants/:id/stats`
- 商户：`GET /v1/operator/merchants/:id/capabilities`
- 商户：`PATCH /v1/operator/merchants/:id/capabilities`
- 骑手：`GET /v1/operator/riders`
- 骑手：`GET /v1/operator/riders/summary`
- 骑手：`GET /v1/operator/riders/:id`
- 骑手：`GET /v1/operator/riders/:id/stats`

明确不存在的能力：

- 不存在 `POST /v1/operator/merchants/:id/suspend`
- 不存在 `POST /v1/operator/merchants/:id/resume`
- 不存在 `POST /v1/operator/riders/:id/suspend`
- 不存在 `POST /v1/operator/riders/:id/resume`

### 经营规则与代取费

- `GET /v1/operator/rules`
- `PATCH /v1/operator/rules/:key`
- `GET /v1/delivery-fee/regions/:region_id/config`
- `POST /v1/delivery-fee/regions/:region_id/config`
- `PATCH /v1/delivery-fee/regions/:region_id/config`
- `GET /v1/operator/regions/:region_id/peak-hours`
- `POST /v1/operator/regions/:region_id/peak-hours`
- `DELETE /v1/operator/peak-hours/:id`

### 财务、分账、补差、投诉

- `GET /v1/operators/me/finance/overview`
- `GET /v1/operators/me/commission`
- `GET /v1/operators/me/profit-sharing/configs`
- `GET /v1/operators/me/payment-orders/:id/profit-sharing/amounts`
- `POST /v1/operators/me/payment-orders/:id/profit-sharing/receivers/delete`
- `POST /v1/operators/me/payment-orders/:id/subsidies`
- `POST /v1/operators/me/payment-orders/:id/subsidies/return`
- `POST /v1/operators/me/payment-orders/:id/subsidies/cancel`
- `GET /v1/operators/me/complaints`
- `POST /v1/operators/me/complaints/:id/complete`

明确不存在的能力：

- 不存在 `GET /v1/operators/me`
- 不存在 `PATCH /v1/operators/me`
- 不存在 `GET /v1/operators/me/finance/account/balance`
- 不存在 `POST /v1/operators/me/finance/withdraw`

### 食安与追偿争议

- `GET /v1/operator/food-safety/cases`
- `GET /v1/operator/food-safety/cases/:id`
- `POST /v1/operator/food-safety/cases/:id/investigate`
- `POST /v1/operator/food-safety/cases/:id/resolve`
- `GET /v1/operator/recovery-disputes`
- `GET /v1/operator/recovery-disputes/summary`
- `GET /v1/operator/recovery-disputes/:id`
- `GET /v1/operator/recoveries/:id`

明确不存在的能力：

- 不存在 `GET /v1/operator/appeals`
- 不存在 `GET /v1/operator/appeals/:id`
- 不存在 `POST /v1/operator/appeals/:id/review`
- 不存在 `GET /v1/operator/appeals/summary`
- 不存在 `GET /v1/operator/claims/:claim_id/recovery`
- 不存在 `POST /v1/operator/claims/:claim_id/recovery/waive`

## 页面级改造清单

下面按页面给出当前调用、真实可用后端、主要漂移点和建议改法。

---

## A. 可直接保留的页面

### 1. 运营商注册入口页

- 页面：`weapp/miniprogram/pages/register/operator/index.ts`
- 当前定位：运营商入驻申请，不在 operator 子包里，但属于运营商链路。
- 当前主要调用：
  - `getOperatorApplication`
  - `getOrCreateOperatorApplication`
  - `updateOperatorBasic`
  - `resetOperatorApplication`
  - `submitOperatorApplication`
  - `ocrOperatorBusinessLicense`
  - `ocrOperatorIdCard`
- 真实后端：申请链路整体存在，可继续使用。
- 结论：保留。
- 改法建议：
  - 这页可以继续做交互和体验，不需要因为后端接口真实性而缩减功能。
  - 区域选择明确走通用 region 查询链路，按省、市、区县逐级选择。
  - 不要把运营商名下区域列表误当成入驻页选区数据源。

### 2. 运营分析页

- 页面：`weapp/miniprogram/pages/operator/analytics/index.ts`
- 当前主要调用：通过 `services/operator-console.ts` 聚合加载区域统计、实时统计、日趋势、商户排行、骑手排行。
- 真实后端：上述统计接口都存在。
- 主要漂移点：页面依赖的聚合 service 文件中混入了 operator appeals 相关封装，但分析页主数据本身不依赖这些死接口。
- 结论：保留。
- 改法建议：继续接真实统计域即可，不要把 appeals 摘要拼回分析页。

### 3. 运营规则页

- 页面：`weapp/miniprogram/pages/operator/rules/index.ts`
- 当前主要调用：`GET /v1/operator/rules`、`PATCH /v1/operator/rules/:key`
- 真实后端：存在。
- 结论：保留。
- 改法建议：直接围绕真实规则接口继续迭代。

### 4. 区域选择页

- 页面：`weapp/miniprogram/pages/operator/region/index.ts`
- 当前主要调用：`GET /v1/operator/regions`
- 真实后端：存在。
- 主要漂移点：前端部分逻辑预期响应里有 `has_more`，后端当前主要返回 `regions`、`total`、`page`、`limit`。
- 结论：保留。
- 改法建议：
  - 这页按单页列表处理即可。
  - 前端不需要继续强调分页和无限加载体验。
  - 如果底层仍保留分页兼容，也应退化成简单首屏加载，不再依赖 `has_more` 作为核心交互前提。

### 5. 区域配置摘要页

- 页面：`weapp/miniprogram/pages/operator/region/config.ts`
- 当前主要调用：区域配置摘要、跳转代取费页和峰时页。
- 真实后端：相关配置接口都存在。
- 结论：保留。
- 改法建议：作为“区域配置入口页”继续保留即可。

### 6. 代取费页

- 页面：`weapp/miniprogram/pages/operator/delivery-fee/index.ts`
- 当前主要调用：
  - `GET /v1/delivery-fee/regions/:region_id/config`
  - `PATCH /v1/delivery-fee/regions/:region_id/config`
  - `POST /v1/delivery-fee/regions/:region_id/config`
- 真实后端：存在。
- 结论：保留。
- 改法建议：继续围绕真实接口做页面交互。

### 7. 峰时时段页

- 页面：`weapp/miniprogram/pages/operator/timeslot/index.ts`
- 当前主要调用：
  - `GET /v1/operator/regions/:region_id/peak-hours`
  - `POST /v1/operator/regions/:region_id/peak-hours`
  - `DELETE /v1/operator/peak-hours/:id`
- 真实后端：存在。
- 结论：保留。

### 8. 食安案件列表页

- 页面：`weapp/miniprogram/pages/operator/safety/report/index.ts`
- 当前主要调用：`GET /v1/operator/food-safety/cases`
- 真实后端：存在。
- 结论：保留。

### 9. 食安案件详情页

- 页面：`weapp/miniprogram/pages/operator/safety/detail/index.ts`
- 当前主要调用：
  - `GET /v1/operator/food-safety/cases/:id`
  - `POST /v1/operator/food-safety/cases/:id/investigate`
  - `POST /v1/operator/food-safety/cases/:id/resolve`
- 真实后端：存在。
- 结论：保留。

### 10. 区域扩展申请页

- 页面：`weapp/miniprogram/pages/operator/region-expansion/index.ts`
- 当前主要调用：
  - `POST /v1/operator/region-expansion`
  - `GET /v1/operator/region-expansion`
  - 通用 `GET /v1/regions`
  - 通用 `GET /v1/regions/available`
- 真实后端：存在。
- 结论：保留。

---

## B. 可保留，但必须先删漂移功能的页面

### 11. 运营中心首页

- 页面：`weapp/miniprogram/pages/operator/dashboard/index.ts`
- 当前主要调用：
  - 区域列表
  - 运营统计与概览
  - 申诉待办摘要
  - 异常告警流
- 真实后端可用部分：
  - `GET /v1/operator/regions`
  - `GET /v1/operator/regions/:region_id/stats`
  - `GET /v1/operators/me/finance/overview`
  - `GET /v1/operator/merchants/summary`
  - `GET /v1/operator/riders/summary`
- 主要漂移点：
  - 页面通过 `services/operator-console.ts` 使用不存在的 operator appeals 接口。
  - 页面直接连接 `/v1/platform/ws`。
  - 页面直接调用 `/v1/platform/alerts`。
  - 这两块属于平台能力，不是运营商当前真实开放能力。
- 结论：页面主体可保留，但必须拆掉“申诉待办”和“平台异常告警”两块。
- 改法建议：
  - 保留统计卡片、区域切换、商户摘要、骑手摘要、财务概览。
  - 删除 appeals 卡片。
  - 删除平台 alerts 和 platform websocket 接入。

### 12. 商户列表页

- 页面：`weapp/miniprogram/pages/operator/merchants/index.ts`
- 当前主要调用：
  - `GET /v1/operator/merchants`
  - `GET /v1/operator/merchants/summary`
  - `POST /v1/operator/merchants/:id/suspend`
  - `POST /v1/operator/merchants/:id/resume`
- 真实后端可用部分：列表和 summary 存在。
- 主要漂移点：暂停/恢复接口不存在。
- 结论：保留列表，但去掉暂停和恢复动作。
- 改法建议：
  - 删掉所有暂停/恢复按钮与批量动作。
  - 如果需要“状态调整”，只能改成展示型状态，不要伪装成可操作能力。

### 13. 商户详情页

- 页面：`weapp/miniprogram/pages/operator/merchants/detail/index.ts`
- 当前主要调用：
  - `GET /v1/operator/merchants/:id`
  - `GET /v1/operator/merchants/:id/stats`
  - `POST /v1/operator/merchants/:id/suspend`
  - `POST /v1/operator/merchants/:id/resume`
- 真实后端可用部分：详情和 stats 存在。
- 主要漂移点：暂停/恢复接口不存在。
- 结论：保留详情展示和统计，删掉暂停/恢复入口。

### 14. 骑手列表页

- 页面：`weapp/miniprogram/pages/operator/riders/index.ts`
- 当前主要调用：
  - `GET /v1/operator/riders`
  - `GET /v1/operator/riders/summary`
  - `POST /v1/operator/riders/:id/suspend`
  - `POST /v1/operator/riders/:id/resume`
- 真实后端可用部分：列表和 summary 存在。
- 主要漂移点：暂停/恢复接口不存在。
- 结论：保留列表，删掉暂停/恢复动作。

### 15. 骑手详情页

- 页面：`weapp/miniprogram/pages/operator/riders/detail/index.ts`
- 当前主要调用：
  - `GET /v1/operator/riders/:id`
  - `GET /v1/operator/riders/:id/stats`
  - `POST /v1/operator/riders/:id/suspend`
  - `POST /v1/operator/riders/:id/resume`
- 真实后端可用部分：详情和 stats 存在。
- 主要漂移点：暂停/恢复接口不存在。
- 结论：保留详情与统计，删掉暂停/恢复入口。

### 16. 财务页

- 页面：`weapp/miniprogram/pages/operator/finance/withdraw/index.ts`
- 当前主要调用：
  - `GET /v1/operators/me/finance/overview`
  - `GET /v1/operators/me/commission`
  - `GET /v1/operators/me/finance/account/balance`
  - `POST /v1/operators/me/finance/withdraw`
- 真实后端可用部分：overview 和 commission 存在。
- 主要漂移点：余额接口和提现接口都不存在。
- 结论：提现能力整体取消，这页只保留成只读财务页。
- 改法建议：
  - 不再保留“提现中”“申请提现”“可提现余额”等提现闭环语义。
  - 隐藏提现表单与提现提交按钮。
  - 隐藏为提现闭环服务的余额展示。
  - 页面定位改成“收入概览 + 佣金明细”。

---

## C. 建议下线或等待后端补口的页面

### 17. 申诉列表页

- 页面：`weapp/miniprogram/pages/operator/appeal/list/index.ts`
- 当前主要调用：
  - `GET /v1/operator/appeals`
  - `GET /v1/operator/appeals/summary`
- 真实后端：不存在这组接口。
- 结论：建议直接下线。
- 原因：这页的主数据源不存在，不是简单字段漂移，而是整条能力链不存在。

### 18. 申诉详情页

- 页面：`weapp/miniprogram/pages/operator/appeal/detail/index.ts`
- 当前主要调用：
  - `GET /v1/operator/appeals/:id`
  - `POST /v1/operator/appeals/:id/review`
  - `GET /v1/operator/claims/:claim_id/recovery`
  - `POST /v1/operator/claims/:claim_id/recovery/waive`
- 真实后端：上述路径都不存在。
- 后端当前真实能力：
  - 运营商可读追偿争议：`/v1/operator/recovery-disputes*`
  - 运营商可读追偿单：`GET /v1/operator/recoveries/:id`
- 结论：建议直接下线。
- 原因：
  - 申诉审核主链不存在。
  - 页面里附带的追偿单查询也接错了资源路径语义。
  - 这页不适合做表层修补。

---

## 当前最关键的漂移点

如果只看风险最高、最容易误导排期的点，优先关注下面这些：

### 1. Dashboard 混入平台能力

- 当前运营中心首页用了平台告警接口 `/v1/platform/alerts`
- 当前运营中心首页用了平台 websocket `/v1/platform/ws`
- 这两块不应继续作为运营商小程序的正式能力

### 2. Appeal 页面不是“字段没对上”，而是整条接口链不存在

- operator appeals 整组接口不存在
- recovery 相关前端路径也写错了资源模型
- 这不是联调问题，是产品/后端能力边界已经变了

### 3. 商户与骑手当前只有读管理，没有运营商手工停复入口

- 页面上的 suspend/resume 不是暂时失效，而是后端没有开放
- 如果产品一定要保留这类动作，需要后端重新定义能力，而不是前端继续保留按钮等接口恢复

### 4. 财务页当前不支持提现闭环

- overview 与 commission 能看
- 新流程已经确认不再保留提现能力
- 余额与提现能力也没有继续补口的必要
- 这意味着当前只做“财务查看页”，不能再保留“提现操作页”心智

## 推荐改造顺序

### 第一批：直接推进

- 注册入口页
- 分析页
- 规则页
- 区域配置相关页面
- 代取费页
- 峰时页
- 食安页
- 区域扩展页

### 第二批：先做降级再保留

- Dashboard 去掉 appeals 和平台 alerts
- 商户页去掉 suspend/resume
- 骑手页去掉 suspend/resume
- 财务页改成只读财务页，彻底移除提现心智

### 第三批：直接下线

- Appeal 列表页
- Appeal 详情页

## 适合继续讨论的切入方式

后面如果要继续细化，建议直接按下面方式点题：

- “展开讲 dashboard 应该删哪些块、保留哪些块”
- “把商户页的改造任务拆成 UI、service、接口映射三部分”
- “把财务页改成只读页的字段方案列出来”
- “按页面给我出一版逐页实施顺序”

## 备注

- 当前文档是页面改造视角，不是完整接口文档。
- 判断基于当前仓库代码，特别是 `locallife/api/server.go` 的真实注册结果。
- 如果后端随后补了新路由，这份清单需要同步更新。