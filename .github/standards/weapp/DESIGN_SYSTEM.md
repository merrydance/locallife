# LocalLife 现代设计系统指南 (Design System Guide)

这份文档旨在为 LocalLife 小程序建立统一、现代化且高度用户友好的 UI
设计规范。所有界面开发应严格遵循本指南，以确保用户体验的一致性和高品质视觉呈现。

本规范已根据当前小程序内已验证的页面实现同步更新，尤其补齐以下过去缺失或过时的部分：

- 页面布局骨架
- 组件嵌套层级
- 顶部导航与首屏内容间距
- 页面底部与弹层底部安全区
- 底部操作按钮样式与尺寸
- 列表页新增入口形态

---

## 0. 规范优先级与同步来源

### 0.1 规范优先级

- 已验证并稳定运行的页面实现，是设计规范的直接来源，不允许长期出现“页面已经有成熟做法，但规范仍停留在旧写法”的状态。
- 当规范与线上成熟页面冲突时，应优先参考已验证页面，并在同一任务中回写规范。
- 不允许继续以旧规范为依据，重复产出已经被页面实践证明不可用的按钮、边距和弹层样式。

### 0.2 当前应优先参考的页面实现

- 用户中心页面：顶部内容与自定义导航的间距、滚动容器、页面底部安全区。
- 餐厅入驻页面：固定底部操作栏、大按钮、双按钮布局。
- 配送优惠管理：底部弹层内容区与底部安全距离。
- `app.wxss`：全局 spacing、radius、z-index、安全区 token。

### 0.3 当前文件的职责

- 定义“小程序页面应该怎么搭”。
- 定义“哪些样式必须抽为公共规则或公共组件”。
- 给出可以直接复用的页面结构和样式基线，而不是只给抽象原则。

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

- **页面主操作**: 统一使用 `t-button` + `theme="primary"` + `size="large"` + `shape="round"` + `block`
- **页面次操作**: 统一使用 `theme="default"` 或 `theme="primary" variant="outline"`，尺寸与主操作保持一致，不允许主次按钮高度不一致。
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
- “取消 / 确认”“上一步 / 下一步”“关闭 / 保存”都应使用统一的大按钮样式。
- 弹层中若存在表单滚动区域，动作按钮区要与滚动内容一起保留足够底部安全距离。

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
      <view class="form-actions">
        <t-button theme="default" size="large" shape="round">取消</t-button>
        <t-button theme="primary" size="large" shape="round">保存</t-button>
      </view>
    </scroll-view>
  </view>
</t-popup>
```

规则：

- `form-popup` 负责弹层容器和顶部圆角。
- `form-scroll` 负责滚动和底部安全距离。
- `form-actions` 负责统一按钮布局，不要散落在内容末尾临时写两个按钮。

### 3.8 标题与副标题层级

配置类页面当前常见问题是标题和副标题几乎同色，导致重点不清。统一如下：

- 页面主标题：使用 `--td-text-color-primary`，字号不低于 `Heading`。
- 页面副标题、说明文案：使用 `--td-text-color-secondary`。
- 区块眉题、辅助标签：使用 `--td-text-color-placeholder` 或更弱层级。
- 状态必须同时使用文案和颜色，不允许仅靠一个灰色块或状态色表达。

---

## 4. 组件库使用指南 (Component Library)

**原则：优先使用 TDesign Miniprogram 组件，严禁重复造轮子。**

所有通用 UI 元素必须优先使用 `tdesign-miniprogram` 提供的组件。仅在 TDesign
组件无法满足特定业务需求时，才考虑封装自定义组件。

- **组件路径**:
  `/home/sam/locallife/weapp/node_modules/tdesign-miniprogram/miniprogram_dist`
- **文档参考**: 请查阅 TDesign Miniprogram 官方文档或 component-docs。

### ✅ 推荐替代方案

| 手写元素             | **TDesign 组件**         | 推荐配置示例                                           |
| :------------------- | :----------------------- | :----------------------------------------------------- |
| `<view class="btn">` | **`<t-button>`**         | `<t-button theme="primary" shape="round">`             |
| `<image>`            | **`<t-image>`**          | `<t-image mode="aspectFill" lazy />` (自带加载/失败态) |
| `<view class="tag">` | **`<t-tag>`**            | `<t-tag variant="dark" theme="primary">`               |
| `<input>`            | **`<t-input>`**          | `<t-input label="标题" placeholder="..." />`           |
| 弹窗/Dialog          | **`<t-dialog>`**         | 使用 `Dialog.confirm({...})` API 调用                  |
| 加载中               | **`<t-loading>`**        | `<t-loading theme="circular" />`                       |
| 图标                 | **`<t-icon>`**           | `<t-icon name="app" />`                                |
| 布局/间距            | **`<t-row>`, `<t-col>`** | 栅格布局，减少手写 flex                                |
| 分割线               | **`<t-divider>`**        | `<t-divider content="底线" />`                         |

---

## 5. 开发最佳实践

1. **避免硬编码**：颜色、间距、圆角、安全区优先使用 `app.wxss` 中已有 token。
2. **布局先统一再开发业务**：页面动手前先选定页面骨架类型，而不是边写边补边距。
3. **底部主操作不降级**：页面底栏和底部弹层里的主操作统一使用大按钮，不因空间紧张改成小按钮。
4. **安全区统一处理**：普通页面、固定底栏、底部弹层分别使用对应安全区方案，不要混写。
5. **优先复用成熟实现**：上传预览、底部按钮区、列表新增入口、状态卡片优先复用已有组件或结构，不要每页重写。
6. **暗黑模式兼容**：优先使用 CSS 变量，以便后续统一适配主题。
7. **保持简洁**：减少装饰性线条，更多依赖间距、层级和留白组织信息。

---

## 6. 交互架构规范 (Interaction Architecture)

为了提升感知性能并确保应用在弱网环境下的鲁棒性，LocalLife 采用 **App Shell
(应用外壳) 架构**。

### 6.1 应用外壳原则 (App Shell Architecture)

- **实体先行**: 页面最外层的容器、导航栏 (Navbar)、Tab
  切换栏、以及核心模块的“卡片底座”应不依赖后端数据直接显示。
- **视觉连续性**:
  数据加载不应导致页面整体结构的剧烈跳变。禁止在数据接口返回前使用 `wx:if`
  销毁整个页面内容。
- **数据解耦**: UI
  与数据应为“容器与填充”的关系。即使没有数据，页面的“骨格”也应清晰可辨。

### 6.2 骨架屏与预期管理 (Skeleton Screens)

- **告别转菊花**: 禁止使用全屏 Loading 蒙层或旋转的“菊花图”。
- **骨架占位**: 在动态列表（如订单列表、菜品列表）数据返回前，使用
  `skeleton-card` 占位。
  - **形状对齐**: 骨架屏的形状、高度应与渲染后的真实卡片尽量一致，防止布局闪烁
    (Layout Shift)。
  - **呼吸动画**: 骨架屏应包含微弱的流动感动画 (`shimmer`
    效果)，告知用户系统正在运行。

### 6.3 状态完备性 (Robustness & States)

每个数据驱动的组件必须完整定义以下四种状态：

1. **加载中 (Loading)**: 显示骨架屏。
2. **正常 (Success)**: 渲染真实业务数据。
3. **空数据 (Empty)**: 显示明确的 `t-empty`
   占位，并提供“返回”或“重试”按钮，禁止白屏。
4. **异常 (Error)**: 捕获报错（如
   404/500），显示友好的错误提示，而不是任由页面塌陷。

### 6.4 专业工具效率原则 (Rider-Specific Efficiency)

针对骑手端等效率型应用：

- **减法原则**:
  移除所有对效率无助的“过度设计”（如聊天图标、复杂的加密掩码显示）。
- **秒级触达**:
  关键操作（如联系商家/用户）必须以最醒目的图标形式直接呈现在第一层级。
- **大操纵杆布局**:
  核心状态切换按钮应采用底部悬浮、全宽大按钮设计，适应单手盲操。

---

## 7. 页面与服务贯通检查 (Change Completeness)

每次小程序改动都必须检查“服务层 → 页面状态 → 事件处理 → 视图反馈”是否完整贯通：

1. **服务入参与返回**: 新字段、新动作是否进入 `api/` 或 `services/` 调用层。
2. **页面状态**: `data`、计算属性、局部 loading/submitting 状态是否同步扩展。
3. **事件处理**: 点击、切换、提交、刷新等交互是否真正触发对应逻辑，而非停留在占位分支。
4. **视图反馈**: 成功、失败、空数据、弱网重试提示是否都能被用户感知。
5. **样式归属**: 业务样式是否停留在页面局部，而不是泄漏到全局 `app.wxss` 或共享组件。

## 8. 共享组件与页面边界 (Component Boundaries)

- 通用展示、通用交互优先落在 `miniprogram/components/`，避免页面重复拷贝结构。
- 共享组件保持业务中性，不写页面专属文案、接口调用或业务状态枚举。
- 页面负责组装业务流程，组件负责可复用视图片段与明确事件输出。
- 新增组件前先检查 TDesign 是否已有等价能力，避免重复造轮子。

## 9. 禁止出现的退化模式 (Anti-Patterns)

- 只补了页面 WXML/WXSS，未补 service、事件或状态，导致按钮可点但行为空转。
- 数据未返回时整页 `wx:if` 消失，只剩白屏或全屏 loading。
- 将页面业务颜色、尺寸、边距直接硬编码，而不是使用 `app.wxss` 中的 token。
- 顶部首屏内容与自定义导航的间距靠页面内某个卡片的 `margin-top` 临时补齐，而不是通过统一内容容器控制。
- 底部弹层继续使用小按钮、细按钮或不带安全区的按钮区，导致主操作弱化或显示不全。
- 固定底部按钮栏没有给页面内容预留滚动空间，导致最后一个表单项被遮挡。
- 同一个“新增”动作在空状态、非空状态、不同页面使用完全不同的入口形态。
- 已有成熟上传预览组件或图片展示能力，页面仍重新手写一套且不支持预览。
- 为调试保留临时文案、占位按钮、短路返回、mock 数据或未删除的日志分支。
- 共享组件中混入商家页、骑手页、运营页特定业务样式或术语。

## 10. 提交前最低验收门槛 (Quality Gate)

- 明确说明改动影响的是 page、component、service 还是它们的组合。
- 至少运行最小相关校验命令，例如 `npm run lint`、`npm run compile` 或 `npm run quality:check` 中的合适子集，并说明执行结果。
- 对数据驱动页面，至少人工确认 loading、success、empty、error 四态中的受影响分支。

---

_LocalLife Design System v1.2_
