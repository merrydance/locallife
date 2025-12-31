# 微信小程序 + TDesign 前端开发规范

本文档总结了本项目在微信小程序 + TDesign + Skyline 渲染器技术栈下的开发经验和最佳实践。

---

## 技术栈概览

| 技术 | 版本/说明 |
|------|----------|
| 微信小程序基础库 | 3.x |
| 渲染器 | Skyline (glass-easel) |
| UI 组件库 | TDesign Miniprogram |
| 语言 | TypeScript |

---

## 一、Skyline 渲染器适配

### 1.1 页面滚动

Skyline 渲染器**不支持原生页面滚动**，必须使用 `<scroll-view>` 组件。

```xml
<scroll-view 
  scroll-y 
  class="page-scroll" 
  style="height: {{scrollViewHeight}}px; margin-top: {{navBarHeight}}px;"
  enhanced="{{true}}"
  show-scrollbar="{{false}}"
>
  <!-- 页面内容 -->
</scroll-view>
```

### 1.2 高度计算

```typescript
onLoad() {
  const windowInfo = wx.getWindowInfo()
  const menuButton = wx.getMenuButtonBoundingClientRect()
  const statusBarHeight = windowInfo.statusBarHeight || 0
  const navBarContentHeight = menuButton.height + (menuButton.top - statusBarHeight) * 2
  const navBarHeight = statusBarHeight + navBarContentHeight
  
  // windowHeight 已扣除原生 tabBar，只需扣除自定义导航栏
  const scrollViewHeight = windowInfo.windowHeight - navBarHeight
  
  this.setData({ navBarHeight, scrollViewHeight })
}
```

> **关键点**：`windowInfo.windowHeight` 在有原生 tabBar 的页面中已自动扣除 tabBar 高度。

### 1.3 页面配置

所有使用 Skyline 的页面需要在 `index.json` 中配置：

```json
{
  "renderer": "skyline",
  "componentFramework": "glass-easel"
}
```

---

## 二、组件样式管理

### 2.1 样式隔离

微信小程序组件默认启用样式隔离 (`isolated`)，外部样式无法影响组件内部。

**解决方案**：在组件 `index.json` 中设置：

```json
{
  "component": true,
  "styleIsolation": "apply-shared"
}
```

### 2.2 CSS 变量

**问题**：Skyline 渲染器对 CSS 变量支持有限，变量可能不生效。

**解决方案**：在组件内直接使用显式值，在 `app.wxss` 定义变量仅作文档用途。

```css
/* 推荐：直接使用值 */
.card {
  padding: 20rpx;
  border-radius: 16rpx;
}

/* 避免：依赖 CSS 变量 */
.card {
  padding: var(--customer-card-padding);
}
```

### 2.3 TDesign 组件样式覆盖

TDesign 组件有自己的样式隔离，页面级 CSS 无法直接覆盖。

**解决方案**：用自定义包装器替代 TDesign 的 theme 属性：

```xml
<!-- 避免：使用 theme="card" -->
<t-cell-group theme="card">...</t-cell-group>

<!-- 推荐：自定义包装器 -->
<view class="card-body">
  <t-cell-group>...</t-cell-group>
</view>
```

```css
.card-body {
  background: #fff;
  border-radius: 16rpx;
  overflow: hidden;
  box-shadow: 0 4rpx 12rpx rgba(0, 0, 0, 0.06);
}
```

---

## 三、消费侧页面统一设计规范

### 3.1 设计 Token

| 属性 | 值 | 说明 |
|------|-----|------|
| 卡片内边距 | 20rpx | 卡片内部 padding |
| 卡片间距 | 12rpx | margin-bottom |
| 元素间隙 | 20rpx | flex gap |
| 图片尺寸 | 180rpx × 180rpx | 卡片缩略图 |
| 卡片圆角 | 16rpx | border-radius |
| 图片圆角 | 12rpx | 图片 border-radius |
| 阴影 | 0 4rpx 12rpx rgba(0,0,0,0.06) | box-shadow |
| 页面内边距 | 12rpx 20rpx 24rpx | content-section padding |
| 标题字号 | 28rpx | 卡片主标题 |
| 副标题字号 | 24rpx | 次要文字 |
| 辅助字号 | 22rpx | 最小文字 |
| 价格颜色 | #FF6B58 | 品牌橙红色 |
| 评分颜色 | #FFC107 | 金色 |
| 成功色 | #00897B | 青色 |

### 3.2 页面结构模板

```xml
<custom-navbar title="页面标题" />

<scroll-view 
  scroll-y 
  style="height: {{scrollViewHeight}}px; margin-top: {{navBarHeight}}px;"
>
  <view class="content-section">
    <!-- 卡片列表 -->
    <card-component wx:for="{{items}}" wx:key="id" />
    
    <!-- 加载状态 -->
    <view wx:if="{{!hasMore}}" class="no-more">没有更多了</view>
  </view>
</scroll-view>
```

### 3.3 统一卡片样式

```css
.card {
  display: flex;
  gap: 20rpx;
  padding: 20rpx;
  background: #fff;
  border-radius: 16rpx;
  margin-bottom: 12rpx;
  box-shadow: 0 4rpx 12rpx rgba(0, 0, 0, 0.06);
  transition: all 0.2s ease;
}

.card:active {
  transform: scale(0.98);
  box-shadow: 0 2rpx 8rpx rgba(0, 0, 0, 0.04);
}

.card-image {
  width: 180rpx;
  height: 180rpx;
  border-radius: 12rpx;
  flex-shrink: 0;
}

.card-info {
  flex: 1;
  display: flex;
  flex-direction: column;
  gap: 8rpx;
  min-width: 0;
}

.card-title {
  font-size: 28rpx;
  font-weight: 500;
  color: #333;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.card-price {
  font-size: 28rpx;
  font-weight: bold;
  color: #FF6B58;
}
```

---

## 四、避坑指南

### 4.1 容器嵌套

❌ **避免**：多层容器嵌套导致 padding/gap 叠加

```xml
<view class="listings">
  <view class="list-container">
    <card />
  </view>
</view>
```

✅ **推荐**：单层容器

```xml
<view class="content-section">
  <card />
</view>
```

### 4.2 间距管理

❌ **避免**：容器 gap 与卡片 margin 同时使用

```css
.container { gap: 24rpx; }
.card { margin-bottom: 12rpx; }
```

✅ **推荐**：只在卡片上设置 margin-bottom

```css
.container { /* no gap */ }
.card { margin-bottom: 12rpx; }
```

### 4.3 rpx vs px

- **消费侧**：使用 rpx 实现响应式
- **商户侧 PC SaaS**：使用 px + max-width 限制

---

## 五、文件结构

```
miniprogram/
├── styles/
│   ├── common.wxss       # 通用工具类
│   └── customer.wxss     # 消费侧样式规范
├── components/
│   ├── dish-card/        # 菜品卡片
│   ├── restaurant-card/  # 餐厅卡片
│   └── room-card/        # 包间卡片
└── pages/
    ├── takeout/          # 外卖首页
    ├── reservation/      # 预订首页
    └── user_center/      # 用户中心
```

---

## 六、检查清单

新增消费侧页面时确认：

- [ ] 页面 JSON 配置了 `"renderer": "skyline"`
- [ ] 使用 `<scroll-view>` 作为滚动容器
- [ ] 正确计算 `scrollViewHeight`
- [ ] 使用 `content-section` 作为内容区域
- [ ] 卡片组件使用统一设计值
- [ ] 组件 JSON 配置了 `"styleIsolation": "apply-shared"`
- [ ] 避免使用 CSS 变量（Skyline 兼容性问题）
- [ ] 不使用 TDesign theme="card"，改用自定义 card-body 包装
