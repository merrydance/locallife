# LocalLife Web UI 统一规范（唯一入口）

> 适用范围：Merchant / Operator / Platform Web 端。
>
> 本文作为统一入口，替代“按页面口口相传”的做法；历史文档可保留，但以本文约束为准。

## 1. 统一目标

- 保证页面在布局、颜色、字号、交互密度上风格一致。
- 控制组件和样式自由度：优先复用，不做任意发挥。
- 让“字段对齐后端契约 + 文案生产化”成为上线前必检项。

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

## 6. 历史文档关系

- 细节参考：`src/styles/design-system.md`（商户侧历史实践）
- 通用约束：`DESIGN_GUARDRAILS.md`
- 本文定位：统一入口 + 可执行标准（优先级最高）

## 7. 落地建议（下一步）

- 先覆盖 Operator 页面的枚举中文化与分区统一。
- 再逐步覆盖 Merchant / Platform 页面的样式类收敛。
- 每次页面改造同步更新字段审查台账。
