# LocalLife 商户后台设计规范

基于套餐管理页面 (combos-page-client.tsx) 提取的设计规范。

## 1. 布局结构

### 页面壳 (Page Shell)

```tsx
<PageShell>
    <PageHeader
        title="页面标题"
        description="页面描述"
        actions={<Button>操作按钮</Button>}
    />
    <PageContent>
        {/* 页面内容 */}
    </PageContent>
</PageShell>;
```

### 左右分栏布局 (Master-Detail)

```tsx
<div className="flex h-[calc(100vh-12rem)] gap-6">
    {/* 左侧列表面板 */}
    <div className="w-1/3 min-w-[320px] flex flex-col bg-white rounded-xl border shadow-sm">
        {/* 面板头部 */}
        <div className="p-4 border-b space-y-4">
            {/* 标题和操作 */}
        </div>
        {/* 滚动列表 */}
        <ScrollArea className="flex-1 p-2">
            {/* 列表内容 */}
        </ScrollArea>
    </div>

    {/* 右侧编辑器面板 */}
    <div className="flex-1 bg-white rounded-xl border shadow-sm flex flex-col">
        {/* 编辑器内容 */}
    </div>
</div>;
```

## 2. 卡片和面板

### 标准面板容器

```tsx
className = "bg-white rounded-xl border shadow-sm";
```

### 面板头部

```tsx
className = "p-4 border-b space-y-4";
// 或带flex布局
className = "flex items-center justify-between p-4 border-b";
```

### 筛选栏容器

```tsx
className =
    "flex flex-col md:flex-row gap-4 items-center justify-between bg-card p-4 rounded-xl border border-muted/50 shadow-sm";
```

## 3. 表单区块

### 区块标题 (带左侧强调线)

```tsx
<h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
    区块标题
</h3>;
```

### 区块容器

```tsx
<section className="space-y-4">
    <h3 className="text-sm font-semibold text-slate-900 border-l-4 border-primary pl-3">
        基本信息
    </h3>
    <div className="grid gap-4">
        {/* 表单字段 */}
    </div>
</section>;
```

### 编辑器内容区

```tsx
<div className="p-6 space-y-8 max-w-3xl mx-auto">
    {/* 多个表单区块 */}
</div>;
```

## 4. 列表项

### 可选择的列表项

```tsx
<div
    className={cn(
        "p-4 rounded-lg border transition-all cursor-pointer hover:border-primary/50 hover:bg-slate-50",
        isSelected
            ? "border-primary bg-primary/5 ring-1 ring-primary"
            : "border-slate-100",
    )}
>
    {/* 列表项内容 */}
</div>;
```

### 列表项间距

```tsx
className = "space-y-2";
```

## 5. 搜索框

### 带图标的搜索框

```tsx
<div className="relative">
    <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
    <Input
        placeholder="搜索..."
        className="pl-9 bg-slate-50 border-slate-200"
    />
</div>;
```

## 6. 空状态

### 居中空状态

```tsx
<div className="flex-1 flex flex-col items-center justify-center text-muted-foreground">
    <div className="w-16 h-16 bg-slate-100 rounded-full flex items-center justify-center mb-4">
        <IconComponent className="w-8 h-8 text-slate-400" />
    </div>
    <p>提示文字</p>
</div>;
```

### 列表空状态

```tsx
<div className="text-center py-10 text-muted-foreground text-sm">
    暂无数据
</div>;
```

## 7. 状态标签 (Badge)

### 在售/上架状态

```tsx
<Badge
    variant={isOnline ? "default" : "secondary"}
    className="text-xs h-5 px-1.5"
>
    {isOnline ? "在售" : "下架"}
</Badge>;
```

### 状态颜色映射

- 成功/可用: `bg-emerald-500` / `text-emerald-600`
- 警告/待处理: `bg-amber-500` / `text-amber-600`
- 错误/停用: `bg-rose-500` / `text-rose-500`
- 主要: `bg-primary` / `text-primary`
- 次要: `bg-slate-500` / `text-muted-foreground`

## 8. 价格显示

### 主价格

```tsx
<span className="text-lg font-bold text-primary">
    <span className="text-xs font-normal">¥</span>
    {formatAmount(price)}
</span>;
```

### 原价/优惠信息

```tsx
<div className="text-xs text-emerald-600 font-medium">
    比原价节省 {discountRate}%
</div>;
```

## 9. 表单控件

### 表单标签

```tsx
<Label>
    字段名称 <span className="text-destructive">*</span>
</Label>;
```

### 价格输入框

```tsx
<div className="relative">
    <span className="absolute left-3 top-1/2 -translate-y-1/2 text-muted-foreground">
        ¥
    </span>
    <Input type="number" className="pl-7" placeholder="0.00" />
</div>;
```

### 开关+标签

```tsx
<div className="flex items-center gap-2 h-10">
    <Switch id="switch-id" checked={value} onCheckedChange={onChange} />
    <Label htmlFor="switch-id" className="cursor-pointer font-normal">
        {value ? "启用状态" : "禁用状态"}
    </Label>
</div>;
```

## 10. 标签选择器

### 可选标签

```tsx
<Badge
    variant={isSelected ? "default" : "outline"}
    className={cn(
        "cursor-pointer px-3 py-1.5 transition-all text-sm font-normal",
        isSelected
            ? "border-primary"
            : "text-muted-foreground hover:bg-slate-100",
    )}
>
    {tag.name}
    {isSelected && <CheckCircle2 className="ml-1.5 h-3.5 w-3.5" />}
</Badge>;
```

## 11. 操作按钮

### 编辑器头部按钮组

```tsx
<div className="flex items-center gap-4">
  <h2 className="text-lg font-semibold">编辑标题</h2>
  <div className="flex gap-2">
    <Button variant="outline" size="sm">
      <RefreshCw className="h-3 w-3 mr-1" />
      重置
    </Button>
    <Button variant="destructive" size="sm">
      <Trash2 className="h-3 w-3 mr-1" />
      删除
    </Button>
  </div>
</div>
<Button disabled={saving}>
  {saving && <RefreshCw className="mr-2 h-4 w-4 animate-spin" />}
  保存更改
</Button>
```

### 悬浮操作按钮

```tsx
<div className="flex gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
    <Button
        variant="ghost"
        size="icon"
        className="h-8 w-8 hover:bg-primary/10 hover:text-primary rounded-full"
    >
        <Edit className="h-4 w-4" />
    </Button>
    <Button
        variant="ghost"
        size="icon"
        className="h-8 w-8 hover:bg-destructive/10 hover:text-destructive rounded-full"
    >
        <Trash2 className="h-4 w-4" />
    </Button>
</div>;
```

## 12. 弹窗/模态框

### 全屏模态框

```tsx
<div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4 backdrop-blur-sm">
    <div className="bg-white rounded-xl shadow-xl w-full max-w-lg max-h-[80vh] flex flex-col">
        {/* 头部 */}
        <div className="p-4 border-b flex items-center justify-between">
            <h3 className="font-semibold text-lg">标题</h3>
            <Button variant="ghost" size="icon">
                <XCircle className="h-5 w-5" />
            </Button>
        </div>
        {/* 内容 */}
        <ScrollArea className="flex-1">...</ScrollArea>
        {/* 底部 */}
        <div className="p-4 border-t flex justify-end bg-slate-50 rounded-b-xl">
            <Button>确认</Button>
        </div>
    </div>
</div>;
```

## 13. 内容分隔线

### 带边框的内容区

```tsx
<div className="rounded-lg border bg-slate-50/50">
    <div className="divide-y">
        {/* 子项 */}
    </div>
</div>;
```

### 底部汇总栏

```tsx
<div className="p-3 bg-slate-100 text-right text-sm font-medium border-t">
    总计：¥{total}
</div>;
```

## 14. 颜色规范

### 语义色

| 用途 | 背景色        | 文字色        |
| ---- | ------------- | ------------- |
| 主要 | `primary`     | `primary`     |
| 成功 | `emerald-500` | `emerald-600` |
| 警告 | `amber-500`   | `amber-600`   |
| 错误 | `rose-500`    | `destructive` |
| 信息 | `blue-500`    | `blue-600`    |

### 中性色

| 用途 | 色值                                                        |
| ---- | ----------------------------------------------------------- |
| 背景 | `bg-white`, `bg-slate-50`, `bg-muted/50`                    |
| 边框 | `border-slate-100`, `border-slate-200`, `border-muted/50`   |
| 文字 | `text-slate-900`, `text-slate-700`, `text-muted-foreground` |
| 图标 | `text-slate-400`, `text-muted-foreground`                   |

## 15. 间距规范

| 用途         | 值                   |
| ------------ | -------------------- |
| 页面级间距   | `gap-6`, `space-y-6` |
| 区块内间距   | `space-y-4`, `gap-4` |
| 紧凑间距     | `space-y-2`, `gap-2` |
| 面板内边距   | `p-4`, `p-6`         |
| 列表项内边距 | `p-3`, `p-4`         |

## 16. 字体规范

| 用途       | 类名                             |
| ---------- | -------------------------------- |
| 页面标题   | `text-xl font-semibold`          |
| 面板标题   | `text-lg font-semibold`          |
| 区块标题   | `text-sm font-semibold`          |
| 列表项标题 | `font-medium text-slate-900`     |
| 正文       | `text-sm`                        |
| 辅助文字   | `text-xs text-muted-foreground`  |
| 价格大字   | `text-lg font-bold text-primary` |

## 17. 反馈与通知

### 消息通知 (Toast)

使用 `sonner` 库进行轻量级通知。禁止使用原生 `alert()`。

```tsx
import { toast } from "sonner";

// 成功逻辑
toast.success("操作成功");

// 错误提示
toast.error("操作失败，请重试");

// 普通信息
toast.info("已打印小票");
```

### 确认对话框 (ConfirmDialog)

对于不可逆或重要的操作（如删除、退出），必须使用 `ConfirmDialog`
组件。禁止使用原生 `confirm()`。

```tsx
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

// 状态控制
const [open, setOpen] = useState(false);

// JSX
<ConfirmDialog
    open={open}
    onOpenChange={setOpen}
    title="删除提醒"
    description="确定要删除此项目吗？该操作无法撤销。"
    confirmText="确认删除"
    variant="destructive" // 重要删除使用 destructive 变体
    onConfirm={handleDelete}
/>;
```

### 交互准则

1. **轻量反馈**：表单保存、状态切换等成功提示使用 `toast`。
2. **重要决策**：删除、重置、敏感设置等必须使用 `ConfirmDialog`。
3. **加载状态**：提交按钮必须处理 `loading`/`saving` 状态，并禁用重复点击。
