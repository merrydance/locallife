# Weapp 非用户侧页面执行清单

本清单用于商户、骑手、运营、平台等非用户侧页面的日常实现、改造和样式收口。

先读：

- `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`
- `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`

如果只是想快速判断一个页面是否符合当前标准，按下面顺序过一遍即可。

## 1. 页面骨架

- 是否使用统一 page shell：`page-shell`、`page-shell--with-nav`、`page-shell--bottom-safe`、`page-shell--page-gutter`。
- 导航下留白、左右 gutter、底部 safe area 是否由 page shell 承接，而不是每页自己叠一套。
- `page-content` 是否承担页面内部节奏：区块 gap、底部 padding、动作区与正文间距。
- 是否避免了顶部解释性大卡片；说明是否下沉到 section caption、字段 note、notice bar 或局部状态。

## 2. 分区结构

- 一个页面是否只有一个明确主任务。
- section 是否只在真正需要分组时才使用卡片或容器。
- 是否避免卡片套卡片、说明块套说明块、局部视觉壳堆叠。
- 区块标题、区块副标题、区块状态提示是否都服务当前任务，而不是“解释页面边界”。

## 3. 按钮分层

- 页级主动作：保存、提交、创建，默认放在内容流末尾的 `form-actions`，不要默认做 fixed `footer-bar`。
- 状态重试动作：重新加载、重新校验、刷新结果，默认直接用 TDesign 原生按钮，不额外指定 `small`、`round`、`large`、`block`。
- 区块头局部动作：新增标签、管理分类、新增规格组，默认用带图标的 `size="small"` 按钮并保留短文案。
- 行级工具动作：只在高密度局部编辑器里使用圆形图标按钮，例如增删规格项、行尾 remove。
- 是否避免把“新增/管理”这类区块级动作收成只有图标的圆形按钮。

## 4. 输入表单

- 长 label 是否已经收短，避免在 TDesign 输入行内换行。
- 单位是否优先放进 `suffix`，而不是塞进 label 文案。
- 数值字段是否在页面内保持一致：同一类金额、比例、数量输入统一右对齐；普通文本输入保持默认。
- 是否没有额外的 `t-class-input`、`custom-style="--td-input-*"` 一类自定义把 TDesign 输入框改出第二套观感。
- 若必须自定义输入框，只能为了局部布局，不得改变非用户侧页面的默认视觉语言。

## 5. 状态承接

- 是否区分首屏加载、首屏失败、局部刷新失败、已有数据下的刷新失败。
- 首屏失败是否页内承接，不依赖 Toast-only。
- 已有可信数据时，刷新失败是否保留已有数据，并用 notice/inline 状态说明。
- 异步结果是否提供回查、刷新或后续状态承接，而不是停留在一次性提示。

## 6. 弹层与局部编辑

- 少字段辅助创建优先 `t-dialog`，长内容或需要滚动时才使用 `t-popup`。
- 弹层动作区是否独立于滚动内容，并由安全区承接。
- 两层嵌套编辑结构里，组级动作和行级动作是否有明显层级差，不要同权重。

## 7. 反模式检查

- 是否还保留自定义 fixed `footer-bar`、`save-wrap`、额外阴影底栏。
- 是否还依赖 `page-shell-bottom-offset` 仅仅为了给自定义底栏让位。
- 是否把普通重试、返回、刷新按钮做成 round/large/block 的“视觉模板”。
- 是否存在输入框一部分左对齐、一部分右对齐，但不是出于字段类型语义，而是历史自定义样式造成。
- 是否存在说明文案过长，直接塞进 input label、cell title 或按钮文案里导致换行。

## 8. 提交前验证

- 先跑 `npm run quality:check`。
- 如果改了多个页面或共享样式，不只看编译结果，还要看 gate 是否通过。
- 交付说明里要说清楚：改了哪类规则、保留了哪些例外、哪些高风险动作没有顺手改动。

## 9. 当前推荐样本

- 页级主动作内容流：`weapp/miniprogram/pages/merchant/settings/profile/index.wxml`
- 编辑页内容流动作区：`weapp/miniprogram/pages/merchant/vouchers/edit/index.wxml`
- 区块头局部动作：`weapp/miniprogram/pages/merchant/dishes/edit/index.wxml`
- 嵌套编辑器局部工具按钮：`weapp/miniprogram/components/dish-customization-editor/index.wxml`
