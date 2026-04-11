# LocalLife Web UI 统一规范（唯一入口）

> 适用范围：Merchant / Operator / Platform Web 端。
>
> 本文作为统一入口，替代“按页面口口相传”的做法；历史文档可保留，但以本文约束为准。

## 1. 统一目标

- 保证页面在布局、颜色、字号、交互密度上风格一致。
- 控制组件和样式自由度：优先复用，不做任意发挥。
- 让“字段对齐后端契约 + 文案生产化”成为上线前必检项。
- 让提示、错误和成功反馈遵守统一前端约束，不在 Web 端重新发明一套提示逻辑。

## 2. 允许使用的基础组件（白名单）

优先使用 shadcn/ui 组件：

- 容器与结构：`Card` `PageShell` `PageHeader` `PageContent` `Separator`
- 表单与筛选：`Input` `Select` `Textarea` `Switch` `Label`
- 数据展示：`Table` `Badge` `Tabs` `Pagination`（已有实现时）
- 操作与反馈：`Button` `AlertDialog` `Dialog` `Toast/Alert`（已有实现时）

约束：

- 同语义控件必须一致：分类切换统一 `Tabs`，状态筛选统一 `Select`，列表数据统一 `Table`。
- 无必要不新增视觉组件；若新增，需先沉淀到 `components/ui`。
- 对于固定集合/枚举值（如状态、星期、时间段），遵循“能用选择组件不用输入组件”：优先 `Select`，避免 `Input` 自由输入导致脏数据与认知负担。

## 3. Tailwind 样式约束（受限集合）

### 3.1 布局

- 页面主间距：`space-y-4` / `space-y-6`
- 栅格：`grid gap-4 md:grid-cols-2 xl:grid-cols-4`
- 行内筛选：`flex flex-col gap-3 md:flex-row md:items-center`

### 3.2 表面层级

- 基础容器：`rounded-lg border bg-background`
- 弱化容器：`bg-muted/40 border`
- 禁止新增硬编码阴影和颜色（如 `shadow-[...]`、`text-[#xxxxxx]`）

### 3.3 文本

- 正文：`text-sm`
- 次级信息：`text-muted-foreground`
- 指标数值：`text-2xl font-semibold`（仅用于关键 KPI）

### 3.4 状态与反馈

- 错误提示：`border-destructive/30 bg-destructive/5 text-destructive`
- 成功提示：使用语义色（例如现有成功提示块）并保持全站一致样式

### 3.5 Tabs

- 必须使用统一 Tabs 样式（见 `src/components/ui/tabs.tsx`）
- 选中态必须显著区别于未选中态（背景 + 字色 + 阴影）

## 4. 页面结构约束

- 固定骨架：`PageHeader + PageContent`。
- 单一业务流最多两层卡片：
  - 筛选/分类层
  - 数据/操作层
- 禁止把同一流程硬拆成 3 张以上并列卡片。

## 5. 字段与文案约束

### 5.1 字段

上线前逐页检查：

1. 页面字段必须来自后端契约（interface + API response）。
2. 不向业务用户暴露研发字段（如内部 key/调试状态）。
3. `editable=false` 字段禁止编辑并给出业务原因。

### 5.2 文案

- 仅使用业务语义，不使用研发语气：
  - 禁止：`与小程序一致`、`debug`、`proxy`、`fallback`
- 枚举值需要业务可读映射：
  - 如 `pending/approved/rejected` 应映射为中文状态

### 5.3 用户反馈

- 同一动作只能有一个主提示，不同时叠加 Toast、Alert、页内状态和跳转结果页来描述同一结果。
- 不直接渲染后端原始错误、英文错误、网关错误或技术字段名，先转换成业务文案。
- 如果页面刷新、状态标签、金额、列表行、详情区已经明确表达结果，不再额外追加 success Toast。
- 首屏加载失败、列表刷新失败、关键数据缺失必须优先使用页内 Alert 或空态承接，不允许只给一个 Toast。
- 需要确认、解释原因、给下一步指引的场景优先使用 `AlertDialog` 或 `Dialog`，不使用 Toast 代替。

参考标准：`.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`

## 6. 历史文档关系

- 细节参考：`design-system.md`（商户侧历史实践）
- 通用约束：`DESIGN_GUARDRAILS.md`
- 本文定位：统一入口 + 可执行标准（优先级最高）

## 7. 落地建议（下一步）

- 先覆盖 Operator 页面的枚举中文化与分区统一。
- 再逐步覆盖 Merchant / Platform 页面的样式类收敛。
- 每次页面改造同步更新字段审查台账。

## 8. 页面变更完整性检查

每次 Web 端改动都要检查是否完成了从数据到界面的整条链路：

1. 类型与契约：新增字段或状态是否已进入 type/interface、API 调用、页面状态与渲染分支。
2. 结构与模式：页面是否仍保持 `PageHeader + PageContent`，筛选/分类/数据三者关系是否清晰。
3. 状态完备：是否覆盖 loading、empty、error、disabled、submitting 等用户可感知状态。
4. 文案与映射：枚举、状态、错误提示是否已转为业务可读文案，而不是直接暴露后端值。
5. 组件复用：是否复用了 `src/components/ui/` 现有模式，而不是在页面里生造一套交互。
6. 反馈闭环：成功和失败反馈是否与页面结构一致，是否存在重复提示或 Toast-only 错误处理。

## 9. 禁止出现的退化模式

- 新字段只接进页面局部组件，未同步到筛选、表格列、详情区或提交逻辑中应出现的位置。
- 新状态只改了接口类型，没有补页面显示映射、Badge、筛选项或禁用态逻辑。
- 用硬编码颜色、字号、阴影解决单页问题，导致视觉系统漂移。
- 页面 copy 出现“与小程序一致”“fallback”“debug”等研发措辞。
- 相同语义的控件在不同页面使用完全不同的交互形式且无明确理由。

## 10. 上线前最低验收门槛

- 说明本次改动影响的路由、组件和后端字段范围。
- 至少运行最小相关校验命令，例如 `npm run lint` 或更小范围验证，并说明执行结果。
- 若页面依赖新增字段或状态，必须人工检查真实 loading、empty、error 分支，不接受只看 happy path。
- 若页面涉及保存、提交、审批、启停、删除、刷新、支付、复制等动作，必须额外检查是否符合统一提示规则。
