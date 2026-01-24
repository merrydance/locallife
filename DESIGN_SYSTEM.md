# LocalLife 现代设计系统指南 (Design System Guide)

这份文档旨在为 LocalLife 小程序建立统一、现代化且高度用户友好的 UI
设计规范。所有界面开发应严格遵循本指南，以确保用户体验的一致性和高品质视觉呈现。

---

## 1. 设计原则 (Principles)

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
  - **样式**: 淡色背景 + 深色文字 (配合 `.tag-outline` 样式或标准变量)
  - **代码**: `<t-tag variant="light-outline" theme="primary">`
  - **场景**: "微辣"、"免配送费"、"川菜"
  - **注意**: `variant="light-outline"`
    现在的背景色已通过全局变量调整为极淡色，确保了清晰度。

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

- **主操作**: `theme="primary" variant="base"` (实心)
- **次操作**: `theme="primary" variant="outline"` (描边)
- **圆角**: 统一使用 `shape="round"` (全圆角) 或 `shape="square"`
  (小圆角)，避免混用。建议列表操作用 `shape="circle"` (圆形图标) 或
  `shape="round"` (文字按钮)。

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
| `<view class="tag">` | **`<t-tag>`**            | `<t-tag variant="light-outline">`                      |
| `<input>`            | **`<t-input>`**          | `<t-input label="标题" placeholder="..." />`           |
| 弹窗/Dialog          | **`<t-dialog>`**         | 使用 `Dialog.confirm({...})` API 调用                  |
| 加载中               | **`<t-loading>`**        | `<t-loading theme="circular" />`                       |
| 图标                 | **`<t-icon>`**           | `<t-icon name="app" />`                                |
| 布局/间距            | **`<t-row>`, `<t-col>`** | 栅格布局，减少手写 flex                                |
| 分割线               | **`<t-divider>`**        | `<t-divider content="底线" />`                         |

---

## 5. 开发最佳实践

1. **避免硬编码**: 尽量不要在 wxss 中写死 `color: #FF6B58` 或
   `padding: 24rpx`，请使用 `color: var(--td-brand-color)` 和
   `padding: var(--customer-card-padding)`。
2. **暗黑模式兼容**: 使用 CSS 变量可以自动适配暗黑模式（如果未来开启）。
3. **保持简洁**: 减少不必要的装饰线条，利用**间距**（Spacing）来分隔内容。

---

_LocalLife Design System v1.0_
