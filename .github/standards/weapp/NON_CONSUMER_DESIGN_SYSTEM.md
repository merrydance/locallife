# LocalLife 非顾客侧设计系统指南 (Non-Consumer Design System Guide)

本文件定义 LocalLife 小程序中商户、运营、平台、骑手等非顾客侧页面的默认视觉与布局标准。

它服务的是工具型、任务型、管理型页面，不服务顾客侧品牌表达。非顾客侧页面默认采用极大克制、极简、稳定的 TDesign Miniprogram 视觉语言；具体组件边界、允许扩展方式和例外范围统一看 3.1，不在开头重复铺开。

默认页面交付硬基线仍以 `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md` 为准。

顾客侧页面的视觉系统与品牌表达，请查看 `.github/standards/weapp/DESIGN_SYSTEM.md`。

## 0. Scope

本文件用于定义：

- 非顾客侧页面的视觉目标与页面气质
- page shell、左右 gutter、内容区内边距和安全区的默认承接方式
- TDesign-first 组件选型与允许的定制边界
- 非顾客侧按钮、卡片、弹层和区块层级的默认表达

本文件不负责：

- 后端契约、状态恢复、请求预算、弱网或高风险路径语义
- API 交互、支付、轮询、重试和登录恢复规则
- prompts、instructions 或 review 输出格式

## 1. Default Positioning

- 非顾客侧页面默认追求任务清晰、信息克制、状态可信、长期一致，而不是品牌氛围、装饰层次或营销表达。
- 页面中的“设计感”主要来自稳定的 page shell、明确的层级、统一的间距、安全区、对齐和 TDesign 组件一致性，而不是大量自定义块面、品牌色、阴影和装饰元素。
- 除页面结构必需的容器、分区、摘要、错误承接和安全区补白外，默认不要在 TDesign 组件外再包多层视觉壳。
- 若某个非顾客侧页面需要偏离默认 TDesign 视觉语言，必须先能说明业务原因、页面范围和官方支持的实现方式；不能因为“想更好看”就引入局部自创风格。
- 避免把说明堆成首屏大段文案；必要说明应贴近状态、字段或动作，用短句表达，而不是单独做解释性主区块。

## 2. Role-Specific Read Order

当任务涉及非顾客侧页面时，按以下顺序读取：

1. `.github/standards/weapp/PAGE_DELIVERY_BASELINE.md`
2. `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`
3. `.github/standards/weapp/NON_CONSUMER_PAGE_EXECUTION_CHECKLIST.md`（需要快速落地或收口样式时优先通读）
4. `.github/standards/weapp/REVIEW_CHECKLIST.md`（仅 review 或复审时）
5. `weapp/miniprogram/app.wxss`（仅在需要确认 page shell 与共享类当前实现时）

## 3. Core Principles

### 3.1 TDesign First

- 组件选型先查 TDesign MCP 与官方文档；若最新文档与仓库当前依赖版本不一致，以 `weapp/package.json` 中的实际安装版本能力为准。
- 用户可见控件默认优先使用 TDesign Miniprogram。
- 组件存在且能承接任务时，不得为了局部审美改回原生控件或手写近似组件。
- 只允许使用 TDesign MCP 文档明确支持的 theme、size、shape、layout、slot、class、style 和属性扩展方式。
- 若退回原生或本地重写控件，必须能说明 TDesign 不满足的具体点，且例外范围已经收敛到最小。
- 禁止覆盖 TDesign 内部类名、依赖内部节点结构、改写伪元素、替换内部状态色或重写默认交互反馈。
- 如平台能力确实要求回退原生用户可见控件，必须在控件上方就近写明 `<!-- weapp-gate allow-native-control: reason -->`；如必须保留极小范围的 TDesign 内部类兼容处理，必须在选择器上方写明 `/* weapp-gate allow-tdesign-internal-class: reason */`，且理由应直接说明 TDesign 当前缺口或平台限制。

### 3.2 Minimal Containers

- 非顾客侧页面默认只保留形成页面所必需的外层结构：page shell、内容区、必要分区、状态区、底部动作区和安全区承接。
- 不要为了“显得完整”给每个小块都单独包卡片、说明块、装饰头图或额外背景层。
- 若内容已经能直接依靠 TDesign 组件和少量布局容器表达，就不要再额外制造本地视觉组件。
- 非顾客侧单任务页、设置页、表单页、详情编辑页默认禁止在顶部放置解释性大卡片、导读大卡片、任务边界说明卡片或“这里只负责……”式说明块；首屏应直接进入任务本身。
- 需要说明同步状态、编辑边界、失败影响或局部规则时，应优先用贴近位置的短句、状态标签、字段 note 或页内状态承接；图标或动画只能辅助，不替代必要说明。

### 3.3 Page Shell Before Components

- 页面在放置 TDesign 组件前，先建立统一的 page shell、导航下留白、左右 gutter、底部安全区和内容区内边距。
- page shell 负责页面外部节奏；内容区容器负责内部内容 padding；不要把这些间距职责分散到每个卡片、每个区块或每个 TDesign 组件上。
- 只要共享的 page shell 或共享容器已经正确承接这些职责，就不要求每页重复手写同一套 padding。

## 4. Page Shell And Spacing

非顾客侧页面默认优先复用共享 page shell：

- `page-shell`
- `page-shell--with-nav`
- `page-shell--bottom-safe`
- `page-shell--page-gutter`

当前实现定义见 `weapp/miniprogram/app.wxss`。
优先参考本文件给出的 page shell 结构、共享 shell 类和下方推荐骨架；仓库中的既有页面只能作为“当前怎么接 page shell”的实现样本，不能反向覆盖本文件对说明卡、文本动作按钮和局部视觉壳的硬规则。

默认要求：

- 导航下留白由 `page-shell--with-nav` 或等效共享能力统一承接。
- 页面左右外部边距默认由 `page-shell--page-gutter` 或等效共享容器统一承接。
- 内容区内部 padding 由 `content-section`、`list-section` 或等效内容容器统一承接。
- 底部安全区由 `page-shell--bottom-safe` 或等效共享能力统一承接。
- 不要一边用了 page shell，一边又在首张卡片、首个 section 或局部组件上叠第二套导航下间距或左右 gutter。
- 若页面属于管理页、编辑页、列表运维页，优先参考本文件的 `page-content` + 状态分支骨架和共享 page shell 组合，而不是复制某个历史页面里的 note-card、文本动作按钮或局部包装层。

推荐结构：

```xml
<view
  class="page-container page-shell page-shell--with-nav page-shell--bottom-safe page-shell--page-gutter"
  style="--page-shell-nav-height: {{navBarHeight}}px; --page-shell-bottom-offset: 0rpx;"
>
  <view wx:if="{{initialLoading}}" class="state-block">
    <t-loading theme="circular" size="40rpx" text="加载中..." vertical />
  </view>

  <view wx:elif="{{initialError}}" class="state-block">
    <t-empty icon="error-circle" description="加载失败，请重试" />
    <t-button theme="primary" bind:tap="onRetry">重新加载</t-button>
  </view>

  <block wx:else>
    <view class="page-content">
      <!-- TDesign-first 页面内容 -->
    </view>
  </block>
</view>
```

适用边界：

- 这是按菜品管理页面组抽象出的非顾客侧默认外层结构，不是要求每页逐字照抄。
- 若页面已经通过共享布局组件或等效容器承接了相同职责，只要节奏一致即可。
- `content-section`、`list-section` 仍然可以用于更简单的内容型页面或 `page-content` 内部的具体区块，但不应替代成熟管理页的整体骨架参考。

## 5. Visual And Token Rules

- 默认使用 TDesign 的基础页面背景、容器背景、文字层级、圆角、分割线和状态色。
- 非顾客侧页面不默认复用顾客侧品牌主色、装饰性氛围色、营销块面或顾客侧专属视觉 token。
- 如果颜色和视觉层级仅靠 TDesign 默认 token 即可表达清楚，就不要再新增本地色板。
- 允许使用共享 spacing、radius、安全区和 page shell 相关 token；这些属于跨角色可复用的基础布局 token。

## 6. Components

### 6.1 Buttons

- 主操作默认使用 TDesign 原生按钮层级；是否使用 `large`、`round`、`block` 由当前页面结构和动作位置决定，不做全局硬性规定。
- 区块级新增、删除、增减、排序、局部管理动作优先使用 TDesign 原生轻量按钮接法；icon button、icon-led button、轻量文本按钮都可以，由当前语义密度和误触风险决定。
- 嵌套组件相邻icon按钮可以通过内层按钮小于外层按钮加以视觉区分
- 不要机械地把局部动作都做成同一种按钮模板；若纯文字按钮已经能清楚表达语义且不会制造误触，不必为了统一外观强行改成图标按钮。

进一步落地为四类默认模式：

- 页级主动作：编辑页、设置页、创建页的保存/提交动作，默认放在 `page-content` 末尾的内容流动作区，例如 `form-actions`。除非动作必须在长滚动过程中持续可见，否则不要再包自定义 fixed `footer-bar`、阴影壳或额外安全区壳。
- 状态重试动作：`state-block`、嵌入式空态、首屏失败等场景下的“重新加载”“重新校验”，默认直接使用 TDesign 原生按钮，不再额外指定 `size="small"`、`shape="round"` 作为常规样式。
- 区块头局部动作：`section-top-action`、`field-action-row` 一类区域里的“新增标签”“管理分类”“新增规格组”，默认使用带图标的 `size="small"` 按钮，并保留短文案；不要把这类页级局部动作默认做成纯圆形加号按钮。
- 行级工具动作：重复编辑器、局部列表项尾部、紧凑型 note slot 内的增删改按钮，才优先使用 `shape="circle"`、`size="small"` 或 `size="extra-small"` 的图标按钮，用来表达“局部工具”而不是“区块级动作”。

默认禁忌：

- 不要把页级主动作做成固定底栏视觉壳，再额外依赖 `page-shell-bottom-offset` 为它让位。
- 不要把“新增/管理”这类区块头动作收缩成只有图标的圆形按钮，除非该区域已经存在非常强的上下文，不会牺牲可扫读性。
- 不要给普通重试、返回、刷新按钮附带无业务意义的 `round`、`large`、`block` 视觉约定。

当前推荐样式对应关系：

- `新增标签`、`管理分类`、`新增规格组` 这类区块头动作，参考 `weapp/miniprogram/pages/merchant/dishes/edit/index.wxml`。
- `保存店铺资料`、`保存代金券` 这类页级主动作，参考 `weapp/miniprogram/pages/merchant/settings/profile/index.wxml` 与 `weapp/miniprogram/pages/merchant/vouchers/edit/index.wxml`。
- 嵌套编辑器里的行级圆形工具按钮，继续参考 `weapp/miniprogram/components/dish-customization-editor/index.wxml` 与 `weapp/miniprogram/components/dish-customization-options/index.wxml`。

嵌套重复编辑器示例：

- 当页面存在“规格组 -> 规格项”这类两层重复编辑结构时，外层组级动作应放在组头右侧，内层行级动作应放在每行尾部的局部工具区，不要把两层按钮都做成同一视觉重量。
- 组级和行级动作可以通过尺寸、位置、主题或文案密度拉开层级，但不预设固定的 size / shape / theme 组合。
- 两层都涉及草稿态增删时，可以使用图标语义，也可以保留短文本动作；是否进入危险表达由真实风险决定，不做全局样式指定。
- 颜色分工只要求服务语义与层级，不预设固定的 danger / light / default 搭配模板。
- 行级增减按钮允许放入一个轻量局部操作容器中统一承接，但该容器只负责行尾动作聚合，不应升级成新的卡片或结构性视觉壳。

参考结构之一：

```xml
<view class="spec-group-card__header">
  <t-input size="small" />
  <t-button size="small" shape="circle" theme="danger" icon="remove" />
</view>

<view class="option-editor__row">
  <t-input size="small" />
  <t-input size="small" />
  <view class="option-editor__actions">
    <t-button size="extra-small" shape="circle" theme="light" icon="add" />
    <t-button size="extra-small" shape="circle" theme="default" icon="remove" />
  </view>
</view>
```

当前仓库参考实现：`weapp/miniprogram/components/dish-customization-editor/index.wxml` 与 `weapp/miniprogram/components/dish-customization-options/index.wxml`。

### 6.2 Cards And Sections

- 卡片只用于真正需要分组、隔离状态、承接摘要或危险信息的区块。
- 不要把每一段说明、每一排筛选、每一个轻量表单组都做成独立白卡。
- 优先用 page shell + 内容容器 + TDesign 组件本身建立秩序，再决定是否需要卡片。
- 顶部 hero 式说明卡片不属于非顾客侧默认结构；只有当说明本身就是当前任务主体时才允许例外。
- 从 fixed footer-bar 改回内容流动作区时，page shell 仍然只负责外层 safe area；页面自身还需要补足内容区底部节奏，例如 `page-content` 的底部 padding 和动作区与上一块内容之间的显式间距。

### 6.3 Popup And Dialog

- 少字段辅助创建、短确认流程优先使用 `t-dialog`。
- 只有当内容较长、需要滚动、需要持续底部动作区时，才使用底部 `t-popup`。
- 使用底部弹层时，动作区必须独立于滚动内容，并由统一安全区承接；按钮尺寸、圆角和是否等宽跟随当前场景与 TDesign 原生能力，不做全局锁定。

## 7. Review Questions

当评估非顾客侧页面是否符合本文件时，至少回答：

1. 页面是否仍然以 TDesign 标准实现为主，而不是靠本地视觉壳堆出复杂感。
2. page shell、左右 gutter、内容区内边距和底部安全区是否由统一外层结构承接，而不是分散在局部组件里。
3. 是否只有形成页面结构所必需的补充容器，没有无意义的卡片套娃和装饰包装。
4. 是否避免了顾客侧品牌色、营销氛围和装饰性 token 向非顾客侧页面渗透。
5. 所有非 TDesign 的用户可见控件是否都能说明 TDesign 不满足的具体原因以及例外范围。

## 8. Current Implementation References

- `weapp/miniprogram/app.wxss`
- `weapp/miniprogram/pages/merchant/dishes/index.wxml`
- `weapp/miniprogram/pages/merchant/dishes/edit/index.wxml`
- `weapp/miniprogram/pages/merchant/dishes/categories/index.wxml`
- `weapp/miniprogram/pages/operator/riders/index.wxml`
- `weapp/miniprogram/pages/platform/operators/index.wxml`

这些实现最多只用于回答“当前仓库里 page shell 和共享容器怎么接”；若现有页面仍保留说明卡、文本动作按钮或额外视觉壳等历史写法，一律以本文件的硬规则为准，不得把历史页面当成例外许可。