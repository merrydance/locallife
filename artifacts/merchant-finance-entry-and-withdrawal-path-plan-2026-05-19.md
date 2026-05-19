# 商户小程序财务入口收口与提现路径澄清任务文档

**日期**：2026-05-19  
**范围**：`weapp/` 商户侧 dashboard、财务页、结算账户页、提现迁移页与相关用户文案  
**风险等级**：G2。原因：涉及商户财务入口、账户命名、页面跳转和提现迁移提示，属于支付相邻的高敏感展示调整，但不改变真实资金流、提现接口或后端状态机。  
**任务域 owner**：商户财务入口页面组 `weapp/miniprogram/pages/merchant/dashboard/**` + `weapp/miniprogram/pages/merchant/finance/**`；共享入口文案建议收敛到 `weapp/miniprogram/utils/merchant-finance-entry-view.ts`（新建）或既有 `merchant-dashboard-view.ts` / `user-facing.ts` 的最小扩展。  
**视觉范围**：非顾客侧商户工具页面，遵循 `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`。不引入解释性大卡片，不借用顾客侧品牌语言。  
**后端/契约来源**：本任务不新增后端接口；提现能力仍以微信支付商户平台/商家助手为外部处理路径。相关事实来源是现有小程序路由、`merchant-settlement-account.ts` 的账户状态语义、`merchant-applyment.ts` 与 `user-facing.ts` 的外部引导文案。

---

## 1. 现状诊断

当前用户之所以觉得“不友好”，不是因为缺字段，而是因为同一类资金能力被切成了多个并列入口：

- `weapp/miniprogram/utils/merchant-dashboard-view.ts` 的财务分组里同时有 `结算账户` 和 `宝付结算账户`。
- `weapp/miniprogram/pages/merchant/finance/index.ts` 的财务页又单独展示 `宝付结算账户`。
- `weapp/miniprogram/pages/merchant/config/index.ts` 也再次展示 `宝付结算账户`。
- `weapp/miniprogram/pages/merchant/dashboard/index.ts` 在营业恢复前置校验里还会去查宝付结算账户状态，并在必要时引导去相应页面。
- `weapp/miniprogram/pages/merchant/finance/withdrawals/*`、`cancel-withdraw/detail/index.wxml` 目前都只是“提现已迁移”的占位页，不存在真正的小程序内提现提交流程。
- `weapp/miniprogram/utils/user-facing.ts` 和 `weapp/miniprogram/api/merchant-applyment.ts` 已经有外部处理提现的文案，但分散，未形成统一口径。

结论先写在这里，免得后面跑偏：

1. 小程序里**没有可用的提现入口**。
2. 现有提现相关页面只是迁移说明，不应再假装可以发起提现。
3. `结算账户` 与 `宝付结算账户` 不应作为两个同等权重的一级入口同时出现。

---

## 2. 目标状态

完成后，用户看到的资金入口应该是这样的：

1. dashboard 只给一个清晰的财务入口，不再并列展示两个账户概念。
2. 财务页承接订单流水、结算记录和账户状态，不把“宝付”当成主入口标题反复出现。
3. 结算账户仍可查看和维护，但它是财务域里的一个子任务，不是 dashboard 上第二个同权入口。
4. 提现只保留迁移说明，明确告诉用户去微信支付商户平台/商家助手处理，不再留假动作。
5. 所有相关页面的文案口径一致，避免一个页面说“结算账户”，另一个页面说“宝付结算账户”，再另一个页面说“提现已迁移”却还给内链按钮造成误解。

---

## 3. 方案选择

### 方案 A  推荐

dashboard 只保留一个财务入口；财务页保留 `订单流水`、`结算记录`、`结算账户` 三个清晰入口；提现页只做迁移说明，不提供任何可发起提现的动作。

优点：

- 入口层级清楚。
- 既解决 dashboard 的“两个账户并列”问题，也不会把提现假装成小程序功能。
- 变更面小，最容易分批提交。

### 方案 B

保留现有层级，只把 `宝付结算账户` 改名成更中性的 `结算账户` / `收款账户`。

优点：

- 改动更少。

缺点：

- 入口数量没变，用户还是会在多个页面里反复看到同类账户项。

### 方案 C

只改提现迁移文案，其他入口保持原样。

优点：

- 代码最少。

缺点：

- 这次的核心困惑完全不解决，不推荐。

**推荐方案：A。**

---

## 4. 执行拆分

### 任务 1：建立统一的财务入口视图模型

**Files：**

- 新增：`weapp/miniprogram/utils/merchant-finance-entry-view.ts`
- 修改：`weapp/miniprogram/utils/merchant-dashboard-view.ts`
- 修改：`weapp/miniprogram/pages/merchant/dashboard/index.ts`
- 修改：`weapp/miniprogram/pages/merchant/finance/index.ts`
- 修改：`weapp/miniprogram/pages/merchant/finance/index.wxml`

**目标：**

- 把 dashboard 和财务页共用的资金入口文案、路由和迁移提示收口到一个地方。
- dashboard 不再显式展示 `宝付结算账户` 这个第一眼就让人困惑的并列入口。
- 财务页保留清晰的三段式入口：订单流水、结算记录、结算账户。

**执行要点：**

1. 新建一个最小共享视图模型文件，集中定义：
   - `财务`
   - `订单流水`
   - `结算记录`
   - `结算账户`
   - `提现功能已迁移`
   - `提现请在微信支付商户平台处理`
2. dashboard 只保留一个财务主入口，不再把 `宝付结算账户` 作为并列一级入口直接放在首屏。
3. 财务页保留结算账户入口，但标题使用用户能理解的 `结算账户`，不要在首屏标题里继续露出 `宝付`。
4. 进入结算账户的具体页面路径可以继续沿用现有路由，技术命名不等于用户命名。

**验收标准：**

- dashboard 首屏不再同时出现 `结算账户` 和 `宝付结算账户` 两个并列入口。
- 财务页入口命名清楚，用户能判断每个入口是看流水、看结算，还是看账户状态。
- 不需要看代码的人也能从页面文案判断，财务页不是提现入口。

---

### 任务 2：统一提现迁移页的用户口径

**Files：**

- 修改：`weapp/miniprogram/pages/merchant/finance/withdrawals/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/finance/withdrawals/create/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/finance/withdrawals/detail/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/finance/cancel-withdraw/detail/index.wxml`
- 视需要修改：`weapp/miniprogram/utils/user-facing.ts`
- 视需要修改：`weapp/miniprogram/api/merchant-applyment.ts`

**目标：**

- 让所有提现迁移页都只表达一件事：提现已经迁移到微信支付商户平台 / 商家助手。
- 不在迁移页里制造“我还能继续在小程序里提一次”的错觉。

**执行要点：**

1. 迁移页标题统一为 `提现功能已迁移` 或 `提现详情已迁移` 这一类短句。
2. 迁移页主体统一使用同一句说明：
   - `提现请在微信支付商户平台处理`
   - 或更完整的 `请前往微信支付商户平台/商家助手处理提现账户和提现申请`
3. 保留的次级入口只用于回到财务页、查看订单流水、查看结算记录，不要再出现伪提现按钮。
4. 如果页面里还有 `宝付结算账户` 这种技术味很重的外链标题，改成 `结算账户`。
5. `cancel-withdraw/detail` 仍然只做迁移说明，不引导用户以为这里还有可发起的注销提现动作。

**验收标准：**

- 任意提现相关页面都能一眼看出“提现不在小程序里办”。
- 页面内不存在 `发起提现`、`继续提现`、`提交提现` 这类误导动作。
- 次级入口只承担导航，不承担业务提交。

---

### 任务 3：对齐 merchant workbench 里的同名入口

**Files：**

- 修改：`weapp/miniprogram/pages/merchant/config/index.ts`
- 视需要修改：`weapp/miniprogram/pages/merchant/settings/applyment/index.ts`
- 视需要修改：`weapp/miniprogram/pages/merchant/settings/applyment/settlement-account/index.wxml`
- 视需要修改：`weapp/miniprogram/api/merchant-settlement-account.ts`

**目标：**

- 避免同一个账户概念在不同入口里反复换名字。
- 如果同一个路由在配置页、申请页、财务页都能进，用户看到的名称要尽量一致。

**执行要点：**

1. `config` 页里的 `宝付结算账户` 如果保留，至少要和财务页统一为 `结算账户`。
2. `applyment` 相关页面继续使用 `结算账户`，不要突然切成供应商名。
3. `merchant-settlement-account.ts` 里的状态语义只保留为技术侧真值，不把它包装成一个新的提现入口。
4. 不新增任何会把用户带回“小程序内提现”的按钮或页面。

**验收标准：**

- 用户在不同入口看到的是同一套中文命名，而不是一堆 provider 名称。
- 账户状态页和财务页的语义一致，不互相打架。

---

### 任务 4：验证、分批提交、推送

**Files：**

- 本任务涉及上面的所有改动文件

**执行顺序：**

1. 先完成任务 1，确认 dashboard / 财务页入口收口。
2. 再完成任务 2，统一提现迁移页。
3. 再补任务 3 的入口命名对齐。
4. 每个批次结束后都跑一次 `weapp` 校验。
5. 每个批次单独 commit，避免把入口收口和文案清理混成一坨。

**验证命令：**

```bash
cd weapp
npm run quality:check
git diff --check
```

**提交建议：**

- 批次 1：财务入口收口
- 批次 2：提现迁移页口径统一
- 批次 3：入口文案和配置页收口

**验收标准：**

- `npm run quality:check` 通过。
- `git diff --check` 通过。
- 用户从 dashboard 进入财务页时，第一眼不会再看到两个账户并列。

---

## 5. 明确非目标

1. 不改后端。
2. 不新增小程序内提现功能。
3. 不改真实结算账户状态机。
4. 不把 provider 名称扩展成用户可见主标题。
5. 不新增解释性大卡片，不在 dashboard 首屏堆“说明页”。

---

## 6. 残余风险

1. 微信支付商户平台 / 商家助手的真实提现操作仍然在外部完成，小程序只能明确告知路径，不能替代。
2. 如果后续产品希望把提现重新收回小程序，需要单独的后端契约与 workflow 设计，不应在本任务里偷偷加入口。
3. 旧的深链接或历史入口如果仍然能打开迁移页，用户看到的也只能是迁移说明，这是预期行为。

