# LocalLife 现代设计系统指南 (Design System Guide)

这份文档旨在为 LocalLife 小程序建立统一、现代化且高度用户友好的 UI
设计规范。所有界面开发应严格遵循本指南，以确保用户体验的一致性和高品质视觉呈现。

本文件只负责小程序基础设计系统，不再承担交互任务流、API 消费契约、评审清单或执行型 instruction 的职责。

交互与任务流规则请查看 `.github/standards/weapp/INTERACTION_STANDARDS.md`。

API 消费契约请查看 `.github/standards/weapp/API_INTERACTION_CONTRACT.md`。

提示与错误反馈总规则请查看 `.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`。

---

## 0. Scope

本文件用于定义：

- 视觉基础原则
- token 与安全区基线
- 页面壳与基础布局模式
- 基础组件视觉模式
- 文本可读性、触控热区、骨架屏等基础体验规范

本文件不负责：

- 服务层到页面层的贯通检查
- 支付、上传、OCR、轮询等异步任务流语义
- 提交前命令校验清单
- prompts 和 instructions 的任务路由规则

---

## 1. 设计原则 (Principles)

- **组件优先复用 (Component First)**：能直接满足需求时，优先使用 TDesign 原生组件；不要为了局部偏好重做或重写近似组件。
- **对比度优先 (Contrast
  First)**：确保内容清晰可见。背景与文字应有足够的对比度，避免顺色。
  - _Bad_: 浅红背景 + 深红文字 (对比度低)
  - _Good_: 纯白背景 + 深红文字 (描边风格) 或 深红背景 + 白色文字 (实心风格)
- **统一韵律 (Consistent Rhythm)**：遵循 8rpx (4px)
  的间距网格系统，建立视觉秩序。
- **层级分明 (Clear Hierarchy)**：通过字号、字重和颜色区分信息重要性。
- **亲和力 (Friendly)**：使用柔和的圆角和适度的阴影，营造现代且亲切的氛围。

### 1.1 TDesign 资料来源与 MCP 使用

- 小程序基础组件选型默认以 TDesign Miniprogram 为第一来源；只有在现有组件确实无法满足需求时，才考虑本地补充组件。
- 需要确认组件是否存在、该用哪个组件、支持哪些属性或事件、升级后是否有变更时，优先使用官方 TDesign MCP 或官方组件文档核实，而不是凭记忆复制旧页面写法。
- 组件选型优先查组件列表；具体接法优先查组件文档；升级或版本对齐优先查变更日志；只有在排查平台兼容或结构限制时，才查看 DOM 结构。
- 使用 MCP 或官方文档前，先对齐仓库当前依赖版本；如果最新文档与本仓库实际安装版本行为不一致，应以 `weapp/package.json` 中的当前版本能力为准。
- MCP 和官方文档用于确认组件能力与升级影响，不用于为视觉漂移背书；不能因为查到了某个内部结构或可选 variant，就把它当作覆盖内部类名、默认使用 outline 风格、或绕开现有 token 体系的理由。

---

## 2. 样式变量索引 (CSS Tokens)

所有样式变量均已在 `app.wxss` 中定义，请在页面开发中直接引用，严禁硬编码数值。

### 🎨 颜色系统 (Colors)

| 语义         | 变量名                    | 色值 (Light)      | 用途                     |
| :----------- | :------------------------ | :---------------- | :----------------------- |
| **品牌主色** | `--td-brand-color`        | `#FF6B58` (Coral) | 按钮、重要图标、价格     |
| **极淡主色** | `--td-brand-color-light`  | `#FFF5F3`         | **Tag 背景**、弱强调背景 |
| **成功色**   | `--td-success-color`      | `#00897B`         | 状态：完成、通过         |
| **警告色**   | `--td-warning-color`      | `#FFC107`         | 状态：待处理、优惠券     |
| **错误色**   | `--td-error-color`        | `#E53935`         | 状态：异常、满减         |
| **页面背景** | `--td-bg-color-page`      | `#FAFAFA`         | 整体页面底色             |
| **卡片背景** | `--td-bg-color-container` | `#FFFFFF`         | 卡片容器背景             |

### 📝 文字系统 (Typography)

| 层级        | 变量名                | 大小 (rpx) | 场景示例                   |
| :---------- | :-------------------- | :--------- | :------------------------- |
| **Display** | `--font-size-display` | `48`       | 巨型金额、Banner标题       |
| **Title**   | `--font-size-title`   | `36`       | 导航栏标题、一级页面标题   |
| **Heading** | `--font-size-heading` | `32`       | **卡片标题**、重要模块标题 |
| **Base**    | `--font-size-base`    | `28`       | **正文**、列表项文字       |
| **Small**   | `--font-size-sm`      | `26`       | 辅助信息、次要说明         |
| **Caption** | `--font-size-xs`      | `24`       | 标签文字、底部提示         |
| **Mini**    | `--font-size-xxs`     | `20`       | 角标、超小标签             |

### 📐 间距系统 (Spacing)

基于 8rpx 网格。

| 变量名        | 值 (rpx) | 场景示例                     |
| :------------ | :------- | :--------------------------- |
| `--spacer-xs` | `8`      | 图标与文字间距               |
| `--spacer-sm` | `16`     | 标签之间间距、小模块间距     |
| `--spacer-md` | `24`     | **标准卡片内边距**、卡片间距 |
| `--spacer-lg` | `32`     | 区块与区块间距               |

### 🛟 安全区系统 (Safe Area)

以下变量已经在 `app.wxss` 中定义，应优先复用，避免每个页面重复手写：

| 变量名                     | 值                                   | 用途                     |
| :------------------------- | :----------------------------------- | :----------------------- |
| `--safe-area-bottom`       | `env(safe-area-inset-bottom)`        | 页面底部安全区基础值     |
| `--popup-bottom-padding-sm`| `calc(24rpx + var(--safe-area-bottom))` | 小型弹层底部留白      |
| `--popup-bottom-padding-md`| `calc(32rpx + var(--safe-area-bottom))` | 标准弹层底部留白      |
| `--popup-bottom-padding-lg`| `calc(40rpx + var(--safe-area-bottom))` | 重操作弹层底部留白    |

规则：

- 页面内容区、固定底栏、底部弹层必须显式处理安全区。
- 已有 token 能表达的场景，不要重复手写 `env(safe-area-inset-bottom)`。
- 只有在“需要为固定栏高度预留页面滚动空间”时，才允许在页面容器上写 `calc(120rpx + env(...))` 这类表达。

### ⭕ 圆角系统 (Radius)

| 变量名           | 值 (rpx) | 场景示例                   |
| :--------------- | :------- | :------------------------- |
| `--radius-sm`    | `8`      | 小按钮、内嵌图片           |
| `--radius-md`    | `12`     | **标准卡片圆角**、菜品图片 |
| `--radius-lg`    | `16`     | 大弹窗、大卡片             |
| `--radius-round` | `999`    | 胶囊按钮、头像             |

---

## 3. 组件设计规范 (Component Guide)

### 3.1 标签 (Tag)

为了解决对比度问题，请严格遵循以下 usage：

总规则：

- 小程序页面中的标签默认禁止使用任何 outline 风格，包括 `variant="outline"`、`variant="light-outline"` 等描边写法。
- 标签要么承担明确状态语义，要么承担轻量属性归类；两类场景都应优先使用 TDesign 已有的 `dark` 或默认实底能力，不要用描边标签制造“看起来更轻”的假层级。
- 若确实因为平台兼容或现有组件能力缺失而必须偏离，必须在任务说明或评审结论中明确写出原因；不能把个人视觉偏好当例外。

- **强强调/状态 (Status/Strong)**
  - **样式**: 深色背景 + 白色文字
  - **代码**: `<t-tag variant="dark" theme="primary/danger">`
  - **场景**: "预制菜"、"推荐"、"休息中"、"已完成"

- **一般属性/信息 (Attribute/Info)**
  - **样式**: 深色背景 + 白色文字（与强调场景统一）
  - **代码**: `<t-tag variant="dark" theme="primary">`
  - **场景**: "微辣"、"免配送费"、"川菜"
  - **注意**: 项目中统一使用 `variant="dark"`，禁止使用 `light-outline` 或 `light`，保持全局视觉一致性。

### 3.2 卡片 (Card)

所有内容应尽可能封装在卡片中。

```css
.card {
  background: var(--td-bg-color-container);
  border-radius: var(--radius-md);
  padding: var(--spacer-md);
  margin-bottom: var(--spacer-md);
  box-shadow: var(--shadow-sm); /* TDesign内置阴影变量 */
}
```

### 3.3 按钮 (Button)

组件约束：优先直接使用 TDesign 原生按钮能力；允许做主题色、间距、安全区和页面布局适配，不允许无必要地修改 TDesign 内部状态行为、图标语义或默认交互反馈。

总规则：

- 小程序页面按钮默认禁止使用 outline 风格，包括 `variant="outline"` 的主次按钮组合。
- 次操作统一优先使用 `theme="default"`；危险次操作使用 `theme="danger"`；只有在标准中已定义的特定组件模式另有说明时，才允许出现不同组合。
- 不允许为了“看起来更轻”“不想太重”而把提交、保存、编辑、关闭、取消等标准动作降级成 outline 按钮。
- 不允许通过覆盖 `.t-button` 内部类名、状态类、伪元素或组件结构去手工重画一套按钮视觉；按钮风格收口应优先通过 TDesign 原生 theme、size、shape、block 能力完成。

- **页面主操作**: 统一使用 `t-button` + `theme="primary"` + `size="large"` + `shape="round"` + `block`
- **页面次操作**: 统一使用 `theme="default"`，尺寸与主操作保持一致，不允许主次按钮高度不一致。
- **底部弹层主操作**: 不允许再使用 `size="medium"` 或视觉尺寸偏小的按钮，统一使用大按钮规范。
- **圆角**: 底部主操作、双按钮操作统一使用 `shape="round"`。页面内的小型次要操作可使用 `shape="round"`，不要混入风格不一致的直角按钮。
- **点击面积**: 底部主操作必须满足明显可点、可盲点，不允许出现“视觉上像辅助按钮，实际上承担主流程提交”的情况。

#### 页面底部主按钮

适用场景：表单提交、保存设置、进入下一步、发起核心操作。

标准：

- 单按钮时使用全宽 `block` 大按钮。
- 双按钮时使用等宽双列布局，左右按钮高度一致。
- 主操作放右侧，返回或取消放左侧。
- 提交中使用按钮自身 `loading` 和 `disabled` 状态，不额外叠加第二套加载提示。

推荐示例：

```xml
<t-button block theme="primary" size="large" shape="round">保存</t-button>

<view class="btn-row">
  <t-button block theme="default" size="large" shape="round">取消</t-button>
  <t-button block theme="primary" size="large" shape="round">确认</t-button>
</view>
```

#### 底部弹层动作按钮

规则：

- 弹层底部动作区延续页面底部主按钮规范，不允许因为放进弹层就缩成小按钮。
- “取消 / 确认”“上一步 / 下一步”“关闭 / 保存”都应使用统一的大按钮样式，默认采用左右双列等宽布局。
- 双按钮场景下，两个按钮都必须使用 `block` 或等效方式撑满各自半屏宽度；只写 `size="large"` 但仍按文案宽度渲染，视为不符合规范。
- 弹层中若存在表单滚动区域，动作按钮区必须独立于滚动内容固定在弹层底部；不要把底部动作按钮塞进滚动内容尾部再依赖 padding “碰运气”露出来。
- 弹层底部双按钮默认使用 `size="large"`。如有特殊场景降为 `medium`，也必须保持左右等宽，不允许退回内容宽小按钮。

#### 列表新增按钮

适用场景：规则管理、优惠管理、券管理、桌台管理等列表型页面。

规则：

- 有持续“新增”动作的列表页，统一优先使用右下角悬浮新增按钮。
- 空状态和非空状态的新增入口形态应保持一致，不要出现“空状态是居中按钮，非空状态变成悬浮按钮”的双规范。
- 若空状态需要强化说明，可以保留说明文案，但主要新增入口仍应与列表态一致。

推荐实现：

```xml
<t-fab icon="add" bind:click="onCreate" />
```

### 3.4 页面布局骨架 (Page Shell)

过去规范对“页面该怎么搭”定义不足，导致各页面在顶部间距、容器结构、滚动方式上各自实现。后续统一采用以下骨架。

#### 类型 A：普通滚动页面

适用场景：用户中心、列表页、详情页、资料页。

标准结构：

```xml
<custom-navbar title="标题" showBack="{{true}}" bind:navheight="onNavHeight" />

<scroll-view
  scroll-y
  class="page-scroll"
  style="height: {{scrollViewHeight}}px; margin-top: {{navBarHeight}}px;"
>
  <view class="content-section">
    <!-- 页面内容 -->
  </view>
</scroll-view>
```

规则：

- 顶部首个内容块与自定义导航之间的留白，统一通过内容容器的顶部 padding 控制。
- 推荐基线为 `content-section` 使用 `padding-top: var(--spacer-sm)`。
- 不要在每个首屏卡片上单独叠加额外 `margin-top` 来“碰运气”调间距。

#### 类型 B：带固定底部操作栏的页面

适用场景：入驻、设置保存、编辑提交流程。

标准结构：

```xml
<custom-navbar title="标题" showBack="{{true}}" bind:navheight="onNavHeight" />

<view class="page-container" style="padding-top: {{navBarHeight}}px; padding-bottom: calc(140rpx + env(safe-area-inset-bottom));">
  <view class="page-content">
    <!-- 页面内容 -->
  </view>

  <view class="submit-bar">
    <t-button block theme="primary" size="large" shape="round">提交</t-button>
  </view>
</view>
```

规则：

- 页面内容区必须为固定底栏预留滚动空间，否则最后一个表单项和底部按钮会互相遮挡。
- 页面底栏高度、内边距、安全区处理必须统一，不允许页面各自定义一套底栏高度。

### 3.5 组件嵌套层级 (Component Nesting)

过去多个页面存在容器层级过深、卡片套卡片、区块职责不清的问题。统一按以下层级组织：

- 页面层：`page-container` / `page-scroll`
- 内容层：`content-section` / `list-section`
- 区块层：`card-section`
- 实体容器层：`card-body` / `form-card` / `promo-card`
- 组件层：`t-cell`、`t-input`、`t-image`、`t-tag`、`t-button`

约束：

- 不要无意义地出现三层以上视觉卡片嵌套。
- 列表项本身已经是卡片时，不要再在卡片内部套另一层白底卡片作为主要容器。
- 页面负责业务编排，通用上传、图片预览、底部动作条、状态卡片应优先复用组件，不要每页手写。

### 3.6 页面底部安全区规范

#### 普通页面内容区

推荐：

```css
.content-section {
  padding: var(--spacer-sm) var(--spacer-md) calc(var(--spacer-lg) + var(--safe-area-bottom));
}
```

适用：无固定底栏，但页面需要保证最后一个内容块不贴底。

#### 固定底栏页面

推荐：

```css
.submit-bar {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  padding: 24rpx;
  padding-bottom: calc(24rpx + env(safe-area-inset-bottom));
  background: #fff;
}
```

适用：设置页、编辑页、入驻页、需要持续显示主操作的页面。

#### 底部弹层

推荐：

```css
.form-scroll {
  padding: var(--spacer-lg);
  padding-bottom: var(--popup-bottom-padding-md);
}
```

规则：

- 弹层安全区统一以 `--popup-bottom-padding-md` 作为默认值。
- 弹层若承载更重的操作区或更长按钮，可使用 `--popup-bottom-padding-lg`。
- 不允许弹层底部按钮紧贴 Home Indicator 或被截断。

### 3.7 底部弹层标准结构 (Bottom Popup)

底部弹层过去最大的问题不是有没有弹层，而是内容区、动作区、安全区和按钮尺寸都没有统一。后续统一使用以下结构：

```xml
<t-popup visible="{{visible}}" placement="bottom">
  <view class="form-popup">
    <scroll-view scroll-y class="form-scroll">
      <view class="form-title">标题</view>
      <view class="form-body">
        <!-- 表单内容 -->
      </view>
    </scroll-view>
    <view class="form-actions">
      <t-button block theme="default" size="large" shape="round">取消</t-button>
      <t-button block theme="primary" size="large" shape="round">保存</t-button>
    </view>
  </view>
</t-popup>
```

规则：

- `form-popup` 负责弹层容器和顶部圆角。
- `form-scroll` 只负责滚动内容本身，不负责承载底部动作按钮。
- `form-actions` 负责统一按钮布局和安全区承接，不要散落在内容末尾临时写两个按钮。

### 3.8 标题与副标题层级

配置类页面当前常见问题是标题和副标题几乎同色，导致重点不清。统一如下：

- 页面主标题：使用 `--td-text-color-primary`，字号不低于 `Heading`。
- 页面副标题、说明文案：使用 `--td-text-color-secondary`。
- 区块眉题、辅助标签：使用 `--td-text-color-placeholder` 或更弱层级。
- 状态必须同时使用文案和颜色，不允许仅靠一个灰色块或状态色表达。

---

## 4. 基础组件选型原则

- 优先使用 TDesign Miniprogram 组件，不为局部视觉偏好重写已有能力。
- 允许做主题色、间距、安全区和页面组合层适配，不允许无必要地改写组件内部状态语义。
- 不允许为了局部页面审美去覆盖 TDesign 内部类名、节点结构、伪元素、交互动效或状态色；默认只允许在 token、外层布局、容器间距、安全区和组合方式上做适配。
- 只有在确认存在平台限制、组件缺陷或缺失能力时，才允许做最小范围的 TDesign 样式覆盖，并且必须优先说明“为什么 token 和外层组合无法解决”。
- 输入、图片、按钮、标签、加载、弹窗、图标等通用元素应优先复用成熟组件，而不是在页面内重新手写。

## 5. 可读性与触控热区

### 5.1 文本可读性

- 正文默认使用 `Base` 或更高等级字号，避免将长段正文降到过小字号。
- 副文案应弱于主标题，但不能弱到需要依赖高亮色才能阅读。
- 重要状态信息必须同时通过文案和视觉层级表达，不依赖单一颜色区分。
- 页面首屏说明文案应优先短句、直接、单一结论，不要在标题、副标题、提示条、卡片说明中重复解释同一件事。
- 若说明文案不能帮助用户理解当前状态或决定下一步动作，应删掉而不是为了“显得完整”保留。

### 5.2 信息密度

- 首屏优先呈现当前任务最关键的信息，不要在第一屏堆叠多个同级卡片竞争注意力。
- 同一区块内尽量保持一种主阅读方向，减少横纵交错造成的扫描负担。
- 长列表卡片优先保证标题、关键状态、主操作可快速扫读，次要信息后置。

### 5.3 触控热区

- 主操作、列表项主点击区、图标按钮应保证足够触控热区，默认不低于约 `88rpx` 的可点击区域。
- 不允许用视觉上很小的文字链或紧贴边缘的小图标承担核心操作。
- 底部主操作必须支持单手稳定点击，不应出现“看起来像辅助按钮，实际承担主流程提交”的弱化设计。

### 5.4 输入与键盘遮挡

- 输入框、选择器、上传入口在键盘弹出后仍应保持当前操作上下文可见。
- 固定底部操作栏页面必须避免键盘遮挡当前输入项或主操作按钮。
- 长表单中的当前编辑区域应优先保持在可视区，不应要求用户频繁手动滚动找回焦点。

## 6. 动效与骨架屏

### 6.1 骨架屏基线

- 禁止用全屏转菊花替代页面骨架。
- 列表、卡片、详情模块的骨架形状应尽量贴近真实布局，避免加载完成后明显跳变。
- 骨架可以有轻微流动感，但不应使用喧宾夺主的强动画。

### 6.2 动效节奏

- 动效应服务于状态切换和空间理解，不为装饰而装饰。
- 页面切换、弹层展开、局部反馈的节奏要克制，避免在高频任务中制造阻滞感。
- 对效率型页面，应优先减少不必要的过渡，让状态确认更直接。

## 7. 相关权威文档

- 交互任务流与恢复策略：`.github/standards/weapp/INTERACTION_STANDARDS.md`
- API 消费契约与异步结果承接：`.github/standards/weapp/API_INTERACTION_CONTRACT.md`
- 提示、成功反馈、错误映射：`.github/standards/frontend/USER_FEEDBACK_STANDARDS.md`
- 小程序运行时提示细则：`weapp/docs/miniprogram-prompt-system.md`

---

_LocalLife Design System v1.3_
