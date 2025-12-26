# LocalLife 响应式设计工程标准 (v3.0)

本文档定义了 LocalLife 小程序的全角色适配方案，将响应式状态细化为 4 种，并确立 `match-media` 为结构适配的核心手段。

## 1. 四大响应式断点 (The 4 Breakpoints)

| 状态名称 | 触发阈值 (px) | 适用场景 | 布局策略 |
| :--- | :--- | :--- | :--- |
| **Mobile (手机)** | < 750 | 标准智能手机 | 单列布局，底部 TabBar |
| **Tablet (平板)** | 750 ~ 1279 | iPad, 折叠屏, 窗口化小程序 | 多列网格 (2-3列)，浮动/卡片式布局 |
| **PC-Window (桌面窗口)** | 1280 ~ 1599 | PC 微信非全屏, 小尺寸笔记本 | 双栏布局，常驻侧边栏 (收起态) |
| **PC-Full (桌面全屏)** | >= 1600 | 高清显示器全屏 | **最大宽度约束 (1440px/1600px)**，多栏看板，高级统计边栏 |

## 2. 工程化适配标准：方案 A (match-media)

为了解决“同一个页面为了适应不同屏幕多了很多组件”的复杂度，我们采用 **“物理隔离、逻辑对齐”** 的原则。

### 2.1 结构化隔离 (WXML)
优先使用 `match-media` 进行大块布局的切换，而不是在一个组件内通过复杂的 CSS 控制显示隐藏。

```xml
<!-- 手机端结构 -->
<match-media max-width="749">
  <mobile-dashboard data="{{stats}}" />
</match-media>

<!-- 平板与桌面窗口结构 -->
<match-media min-width="750" max-width="1599">
  <tablet-dashboard data="{{stats}}" />
</match-media>

<!-- 桌面全屏结构 -->
<match-media min-width="1600">
  <view class="full-screen-wrapper">
    <pc-dashboard-layout data="{{stats}}" />
  </view>
</match-media>
```

### 2.2 样式对齐 (WXSS)
在每个组件内部，通过全局样式变量对齐字体、间距。

| 元素 | Mobile | Tablet | PC-Window | PC-Full |
| :--- | :--- | :--- | :--- | :--- |
| **容器边距** | 32rpx | 40rpx | 48rpx | 64rpx(限制max-width) |
| **标题字号** | 32rpx | 34rpx | 36rpx | 40rpx |
| **正文字号** | 28rpx | 28rpx | 30rpx | 30rpx |
| **网格间距** | 16rpx | 24rpx | 32rpx | 40rpx |

## 3. PC-Full 特殊处理：最大宽度约束
在 PC 全屏下，为了防止内容横向拉伸过长导致阅读困难，页面主容器必须应用 `max-width`。

```css
/* responsive.wxss 提供 */
.pc-full-constraint {
  max-width: 1440px;
  margin: 0 auto;
  box-shadow: 0 0 40rpx rgba(0,0,0,0.05); /* 侧边阴影增加悬浮感 */
}
```

## 4. 实施清单

1. **Behavior 注入**: 所有页面必须引入 `responsiveBehavior` 以感知高度 and 设备类型。
2. ** match-media 优先**: 凡是涉及到“行变列”、“单栏变多栏”的变动，必须使用 `match-media`。
3. **字体阶梯化**: 必须使用 CSS 变量，严禁在 WXSS 中写死单一像素值。
