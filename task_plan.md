# 小程序端体验一致性整改计划（二次审查）

## 目标

针对本次从用户视角复查 `weapp/` 发现的问题，制定一份可直接进入实施的修复计划。整改重点是让功能入口真实可用、文案语义贴近用户任务、同类页面交互保持一致，并避免历史模板或技术字段继续污染用户体验。

## 范围

- 目标目录：`weapp/`
- 任务类型：修复任务计划，当前先不实施业务代码
- 适用角色：消费者预订链路、商家打印机管理、运营商区域配置、平台工作台、商家/平台非消费者管理页
- 主要依据：
  - `.github/instructions/weapp-mini-program.instructions.md`
  - `.github/prompts/weapp-review.prompt.md`
  - `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`
  - `.github/standards/weapp/REVIEW_CHECKLIST.md`
  - `.github/standards/frontend/FRONTEND_ARCHITECTURE_BASELINE.md`
- 执行约束：后续实施时每完成一个任务单独验证、单独提交，提交前确认工作区只包含该任务相关改动。

## 非目标

- 不新增后端未支持的业务能力、字段或状态。
- 不重做小程序整体信息架构、视觉系统或多端响应式方案。
- 不把技术排障信息彻底删除；只调整为不干扰日常用户任务的呈现方式。
- 不把死模板里的旧入口改造成新页面。
- 不顺手重构未被本次审查命中的页面。

## 优先级说明

- P1：当前用户可见、会明显影响理解、信任或操作效率的问题。
- P2：体验一致性和维护风险问题，当前不一定阻断用户，但会持续制造漂移。
- P3：清理类任务，主要降低后续误用和审查噪声。

## 修复任务

### P1-1 预订确认页：改成用户任务语言，移除内部流程说明 `[complete]`

**用户问题**

确认预订页现在出现“当前步骤 / 本步将完成 / 下一步”“本页只负责提交预订”等内部流程语言。用户在提交前需要确认的是订什么、什么时候到店、是否要付定金、提交后去哪里，而不是理解页面职责。

**涉及文件**

- 修改：`weapp/miniprogram/pages/reservation/confirm/index.wxml`
- 可能修改：`weapp/miniprogram/pages/reservation/confirm/index.wxss`
- 核对：`weapp/miniprogram/pages/reservation/confirm/index.ts`

**实施边界**

- 删除或改造 `task-summary-card`，不再出现“当前步骤”“本步将完成”“页面承接”等内部流程文案。
- 保留真实状态：定金模式继续说明提交后拉起微信支付；非定金模式继续说明提交后进入点菜。
- 支付提示改为用户结果导向，例如“提交后将进入点菜页，菜品金额以点菜页为准”。
- 底部主按钮保持一个明确主动作，不新增多按钮决策。
- 只改文案与必要布局，不改预订提交、支付、跳转逻辑。

**不做**

- 不改变 `paymentMode` 判断。
- 不改变创建预订、微信支付、结果页跳转逻辑。
- 不新增后端接口或本地假状态。

**验收标准**

- 页面不再出现“当前步骤”“本步将完成”“本页只负责”“结果页承接”等内部过程文案。
- 用户能在首屏理解：包间信息、日期时间、人数、联系人、是否需要支付、提交后去哪里。
- 定金和非定金两种模式都有清晰、自然的提示文案。

**验证**

- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run check:wxml-handlers`
- 手工检查确认预订页定金/非定金两种展示。

**建议提交**

- `fix: clarify reservation confirmation copy`

### P1-2 预订词汇：统一“预订”，消除一级入口和标题漂移 `[complete]`

**用户问题**

同一预订业务在 TabBar、页面标题、分组标题中混用“预定”和“预订”。一级入口和确认页标题不一致，会让产品显得不稳定，也不符合移动应用长期培养出的稳定主词预期。

**涉及文件**

- 修改：`weapp/miniprogram/app.json`
- 修改：`weapp/miniprogram/pages/reservation/index.json`
- 修改：`weapp/miniprogram/pages/reservation/create/index.json`
- 修改：`weapp/miniprogram/pages/reservation/confirm/index.json`
- 修改：`weapp/miniprogram/pages/reservation/confirm/index.wxml`
- 核对：`weapp/miniprogram/pages/reservation/**/*.wxml`

**实施边界**

- 用户可见标题、Tab 文案、分组标题统一为“预订”。
- 注释中的“预定”可顺手改为“预订”，但不扩大到非预订业务。
- 路由名、字段名、接口名、类型名不因为中文文案统一而重命名。

**不做**

- 不调整 TabBar 图标、页面路径或分享路径。
- 不改变任何业务状态枚举。

**验收标准**

- `app.json` 的预订 Tab 文案为“预订”。
- 预订确认页标题为“确认预订”。
- 预订页面用户可见中文不再混用“预定”。

**验证**

- 在仓库根目录运行：`rg -n "预定" weapp/miniprogram/app.json weapp/miniprogram/pages/reservation`
- 确认剩余命中如存在，只能是不可避免的非用户可见历史注释，并说明原因。
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`

**建议提交**

- `fix: unify reservation wording`

### P1-3 商家打印机页：隐藏日常任务里的技术字段，重排操作入口 `[complete]`

**用户问题**

打印机列表和状态弹层直接展示设备 ID、门店 ID、SN、raw 类型、raw 角色、“云端返回”“信息状态”等技术字段。商家日常使用时更需要知道“这台打印机能不能用、负责什么单、最近检查是否正常”，而不是阅读后端或厂商字段。

**涉及文件**

- 修改：`weapp/miniprogram/pages/merchant/printers/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/printers/index.wxss`
- 可能修改：`weapp/miniprogram/pages/merchant/printers/index.ts`

**实施边界**

- 列表主信息保留：打印机名称、启用状态、友好的类型标签、角色标签、外卖/堂食/预订打印开关、创建/更新时间。
- 列表中移除或降级展示 `设备 ID`、`门店 ID`、`SN`；如排障仍需要，放入“更多信息”或复制排障信息入口。
- 详情中使用 `printer_type_label`、`printer_role_label`，不再直接展示 `printer_type`、`printer_role` raw 值。
- 状态弹层保留用户可理解状态：在线状态、工作状态、设备角色、设备型号、查询时间。
- `provider_status`、`info_status` 等厂商返回信息默认不作为主状态行展示；需要保留时放入“高级信息”折叠区或“复制排障信息”。
- 行内操作从纯文字分隔改为图标或图标加短文案的小按钮，至少区分普通操作、测试操作、危险删除操作。

**不做**

- 不改变打印机列表加载、编辑、删除、测试打印、查询状态的接口。
- 不删除排障所需字段的数据来源。
- 不新增厂商状态解释规则，除非现有代码已有稳定映射。

**验收标准**

- 商家列表第一眼不再看到后端 ID 或厂商 raw 状态。
- “编辑 / 状态 / 测试 / 删除”仍全部可用，且删除为明显危险操作。
- 状态弹层主区域只展示商家能理解并行动的信息。
- `npm run gate:non-consumer-ui-patterns` 不再报告打印机页的纯文字本地操作。

**验证**

- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run check:wxml-handlers`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run gate:non-consumer-ui-patterns`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`
- 手工检查打印机列表、状态弹层、测试打印确认、删除确认。

**建议提交**

- `fix: simplify merchant printer management copy`

### P1-4 运营商区域配置：把 ID 错误改成可恢复用户文案 `[complete]`

**用户问题**

运营商区域配置、峰时时段、配送费配置在缺少区域名或区域参数时展示 `ID: xxx`、`缺少区域ID`。运营人员此时需要知道如何恢复，而不是看到内部字段名。

**涉及文件**

- 修改：`weapp/miniprogram/pages/operator/region/config.wxml`
- 修改：`weapp/miniprogram/pages/operator/timeslot/index.wxml`
- 修改：`weapp/miniprogram/pages/operator/delivery-fee/index.ts`
- 可能修改：`weapp/miniprogram/pages/operator/delivery-fee/index.wxml`
- 可能修改：`weapp/miniprogram/pages/operator/timeslot/index.ts`

**实施边界**

- 有 `regionId` 但没有 `regionName` 时，页面显示“未命名区域”或“当前区域”，不显示 `ID: xxx`。
- 没有有效区域参数时，错误文案改为“未选择区域，请返回区域列表重新选择”。
- 如果现有错误空态只有“重试”按钮，需确认是否适合增加“返回区域列表”；若增加，使用真实已注册路由 `/pages/operator/region/index`。
- 不改变区域配置、配送费、峰时时段保存接口。

**不做**

- 不重做运营商区域选择流程。
- 不把区域 ID 从请求参数中删除。
- 不新增区域名查询接口。

**验收标准**

- 用户可见文案不再出现 `ID:`、`区域ID`。
- 缺少区域参数时，页面给出可恢复方向，而不是只能重试同一个失败请求。
- 正常从区域列表进入时，区域名仍正确展示。

**验证**

- 在仓库根目录运行：`rg -n "ID:|区域ID|缺少区域ID" weapp/miniprogram/pages/operator/region weapp/miniprogram/pages/operator/timeslot weapp/miniprogram/pages/operator/delivery-fee`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`
- 手工检查：正常区域进入、缺少区域参数直接进入。

**建议提交**

- `fix: clarify operator region fallback copy`

### P2-1 非消费者列表操作：统一图标化或图标加文案的小按钮 `[complete]`

**用户问题**

商家、平台管理页中多处列表行使用纯文字“编辑 / 删除 / 移除”等操作。移动端用户已经习惯通过图标、颜色和位置快速识别局部操作，纯文字分隔会降低扫读效率，危险操作也不够醒目。

**涉及文件**

- 修改：`weapp/miniprogram/pages/merchant/combos/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/delivery-promotions/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/discount-rules/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/dishes/categories/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/dishes/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/settings/recharge-rules/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/staff/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/tables/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/vouchers/index.wxml`
- 修改：`weapp/miniprogram/pages/platform/tables/tags/index.wxml`
- 可能修改：以上页面对应 `.wxss`

**实施边界**

- 对每个列表行本地操作改为图标按钮或图标加短文案按钮。
- 编辑类操作使用普通或 primary 语义；删除/移除类操作使用 danger 语义。
- 保留原有 handler、dataset、禁用态、loading 态。
- 页面内如果确实需要纯文字按钮，必须加 `weapp-gate allow-text-action: <具体原因>`，并说明为什么图标会降低清晰度。

**不做**

- 不改变列表数据结构、分页、搜索、删除确认逻辑。
- 不统一所有管理页视觉，只清理本次 gate 报出的本地操作问题。
- 不把主按钮替换成图标按钮；本任务只处理列表行或卡片内的局部操作。

**验收标准**

- 全量 `node scripts/check-non-consumer-ui-patterns.js` 不再报告上述页面的纯文字本地操作，除非有明确 allow 注释。
- 编辑、删除、移除等操作点击行为与整改前一致。
- 删除/移除等危险操作在视觉上与普通操作区分明确。

**验证**

- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" node scripts/check-non-consumer-ui-patterns.js`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run check:wxml-handlers`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`
- 手工抽查套餐、菜品、桌台、优惠券至少四个列表页的编辑和删除。

**建议提交**

- `fix: standardize non-consumer row actions`

### P2-2 非消费者说明卡片：收敛为标签、空态或行动旁提示 `[complete]`

**用户问题**

部分商家管理表单用本地说明卡片承载“暂无标签”“请选择适用类型”等提示。小屏管理页应尽量短、直接、任务优先；说明卡片过多会让页面像说明文档，降低效率。

**涉及文件**

- 修改：`weapp/miniprogram/pages/merchant/combos/edit/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/dishes/edit/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/settings/application/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/tables/edit/index.wxml`
- 修改：`weapp/miniprogram/pages/merchant/vouchers/edit/index.wxml`
- 可能修改：以上页面对应 `.wxss`

**实施边界**

- “暂无标签，可先新增标签”类提示改为对应控件附近的轻量空态、辅助文本或操作旁提示。
- 错误/警告类信息保留强可见性，但优先使用现有 TDesign notice/empty/alert 模式或页面标准状态，而不是自建说明卡片。
- 如果某块说明本身就是用户必须阅读的风险提示，可保留，但必须加 `weapp-gate allow-explanatory-card: <具体原因>`。
- 不改变表单字段和提交逻辑。

**不做**

- 不重写编辑页布局。
- 不新增标签管理能力。
- 不删除真实风险提示，只调整承载方式。

**验收标准**

- 全量 `node scripts/check-non-consumer-ui-patterns.js` 不再报告上述页面的 explanatory-card 问题，除非有明确 allow 注释。
- 空标签、无适用类型、刷新错误等状态仍有用户可理解的提示。
- 表单提交和校验行为不变。

**验证**

- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" node scripts/check-non-consumer-ui-patterns.js`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`
- 手工抽查套餐编辑、菜品编辑、桌台编辑、优惠券编辑。

**建议提交**

- `fix: reduce non-consumer explanatory cards`

### P3-1 平台工作台：删除未使用响应式模板，避免旧入口复活 `[complete]`

**用户问题**

当前平台工作台主页面没有引用旧模板，但 `templates/mobile-content.wxml`、`templates/tablet-content.wxml`、`templates/pc-content-full.wxml` 仍保留旧 handler、桌面大屏文案和不完整操作项。它们不是当前可见缺陷，但会在后续维护时误导开发者，甚至被重新引用后暴露死入口。

**涉及文件**

- 删除或修改：`weapp/miniprogram/pages/platform/dashboard/templates/mobile-content.wxml`
- 删除或修改：`weapp/miniprogram/pages/platform/dashboard/templates/tablet-content.wxml`
- 删除或修改：`weapp/miniprogram/pages/platform/dashboard/templates/pc-content-full.wxml`
- 可能删除：`weapp/miniprogram/pages/platform/dashboard/templates/stats-grid.wxml`
- 核对：`weapp/miniprogram/pages/platform/dashboard/dashboard.wxml`
- 核对：`weapp/miniprogram/pages/platform/dashboard/dashboard.ts`

**实施边界**

- 先用 `rg` 确认这些模板没有被 `dashboard.wxml`、页面 JSON、构建脚本或其他 WXML 引用。
- 如确认未引用，删除死模板和仅被死模板引用的局部模板。
- 如发现仍有真实引用，则不得直接删除；改为补齐对应 handler、路由和用户文案，或移除不可用入口。
- 当前可见 `dashboard.wxml` 的管理入口不在本任务内调整，除非引用关系证明它依赖旧模板。

**不做**

- 不新增平台商户申请或骑手申请旧页面。
- 不把平台工作台重新设计为 PC/Tablet 多模板结构。
- 不修改当前真实可见 dashboard 的数据加载。

**验收标准**

- `platform/dashboard/templates` 下不存在未引用的旧响应式 WXML，或每个保留模板都有清晰引用和完整 handler。
- 搜索不到 `onDateRangeChange`、`onTabChange` 等已不存在 handler 的模板绑定。
- 搜索不到“全屏模式”“Tablet 采用”等非小程序任务语言。

**验证**

- 在仓库根目录运行：`rg -n "mobile-content|tablet-content|pc-content-full|stats-grid|onDateRangeChange|onTabChange|全屏模式|Tablet" weapp/miniprogram/pages/platform/dashboard`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run compile`
- 在 `weapp/` 运行：`PATH="$HOME/.local/bin:$PATH" npm run quality:check`

**建议提交**

- `chore: remove stale platform dashboard templates`

## 推荐实施顺序

1. 执行 P1-1 和 P1-2，先修复消费者核心预订链路的文案语义。
2. 执行 P1-3，处理商家打印机这一处技术字段最集中的当前可见页面。
3. 执行 P1-4，修复运营商区域配置的异常文案和恢复路径。
4. 执行 P2-1 和 P2-2，清空非消费者 UI 规则失败项。
5. 执行 P3-1，删除平台工作台死模板，降低后续误用风险。

## 总体验收口径

- 用户可见文案优先描述任务结果和下一步动作，不描述页面职责、后端字段或厂商返回。
- 同一业务主词在一级入口、页面标题、按钮、分组标题中保持一致。
- 管理页列表操作符合小屏移动端预期：可扫读、可区分、危险操作醒目。
- 非消费者 UI 全量规则检查通过，或只剩有明确业务理由的 allow 注释。
- 当前可见页面不存在未实现入口；未引用旧模板不继续留在活跃目录中误导维护。

## 总体验证命令

从 `weapp/` 目录运行：

```bash
PATH="$HOME/.local/bin:$PATH" npm run check:wxml-handlers
PATH="$HOME/.local/bin:$PATH" node scripts/check-non-consumer-ui-patterns.js
PATH="$HOME/.local/bin:$PATH" npm run compile
PATH="$HOME/.local/bin:$PATH" npm run quality:check
```

手工验证至少覆盖：

- 消费者预订列表进入包间详情，再进入确认预订页。
- 预订确认页定金模式和非定金模式。
- 商家打印机列表、状态弹层、测试打印、删除确认。
- 运营商区域配置、配送费配置、峰时时段；包含正常入口和缺少区域参数入口。
- 套餐、菜品、桌台、优惠券等非消费者列表页的编辑和删除。

## 执行状态

- 当前状态：计划内修复已全部实施、验证、提交并推送。
- 下一步：如需继续提升体验，可进入真机/微信开发者工具走查与截图复审。
