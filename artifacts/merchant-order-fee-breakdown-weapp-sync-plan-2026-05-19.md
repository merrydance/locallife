# 小程序商户订单金额明细同步任务文档

**日期**：2026-05-19  
**范围**：`weapp/` 小程序商户侧订单列表与订单详情。  
**风险等级**：G3。原因：展示用户实付、商户实收、平台服务费、支付通道费等资金/支付相邻金额真值；错误展示会影响商户接单、拒单、退款认知和财务预期。  
**任务域 owner**：商户订单页面组 `weapp/miniprogram/pages/merchant/orders/*`；金额 view model owner 放在 `weapp/miniprogram/utils/merchant-order-detail-view.ts`；后端合同入口放在 `weapp/miniprogram/api/order-management.ts`。  
**视觉范围**：非顾客侧商户工具页面，遵循 `.github/standards/weapp/NON_CONSUMER_DESIGN_SYSTEM.md`。不引入顾客侧品牌语言，不做解释性大卡片。

---

## 背景与后端合同

后端已经完成 `locallife/` 商户订单支付可见性与费用清单修复，参考：

- `artifacts/merchant-order-payment-visibility-fee-breakdown-backend-fix-plan-2026-05-18.md`
- `locallife/api/order_response.go`
- `locallife/api/order.go`
- `locallife/worker/task_process_payment.go`

后端商户订单列表、详情、新订单通知现在提供同名 `fee_breakdown`，金额单位均为分。小程序必须以该字段作为商户订单金额展示真值，不能再前端自算实付或用旧字段拼凑费用明细。

目标字段：

```ts
export interface MerchantOrderFeeBreakdown {
  food_amount: number
  merchant_discount_amount: number
  voucher_discount_amount: number
  food_payable_amount: number
  delivery_fee_amount: number
  delivery_fee_discount_amount: number
  delivery_payable_amount: number
  customer_payable_amount: number
  platform_service_fee_amount: number
  payment_channel_fee_amount: number
  merchant_receivable_amount: number
}
```

字段口径：

- `food_amount`：商品餐费原价。
- `merchant_discount_amount`：商户优惠金额。
- `voucher_discount_amount`：用户券/平台券金额。
- `food_payable_amount`：餐费应付。
- `delivery_fee_amount`：配送费原价。
- `delivery_fee_discount_amount`：配送费优惠。
- `delivery_payable_amount`：用户实际承担配送费。
- `customer_payable_amount`：用户订单实付。
- `platform_service_fee_amount`：商户可见平台服务费合计，不拆平台/运营商。
- `payment_channel_fee_amount`：商户承担的支付通道费。
- `merchant_receivable_amount`：商户实收。

不可变规则：

1. 商户订单列表/详情展示金额必须优先来自 `fee_breakdown`。
2. 不允许在小程序端用 `subtotal + delivery_fee - delivery_fee_discount - discount_amount` 自算实付金额。
3. 不允许展示或新增 `provider_payment_fee`、`provider_payment_fee_rate_bps`、`platform_commission`、`operator_commission`、bps、30bps 等内部/拆分字段。
4. `fee_breakdown` 缺失时，页面不得编造 0 或旧字段兜底；订单详情应显示“订单费用明细暂不可用，请稍后重试”，列表应显示“金额同步中”并保留订单处理能力。
5. 动作接口 `accept/reject/ready/complete` 当前后端响应未补 `fee_breakdown`，小程序动作成功后必须回读当前订单列表，而不是用动作响应覆盖本地金额真值。

---

## 当前落后点

- `weapp/miniprogram/api/order-management.ts` 的 `OrderResponse` 没有 `fee_breakdown`。
- `OrderManagementAdapter.calculateActualAmount()` 仍前端自算旧金额，且漏掉 `voucher_discount_amount`。
- `weapp/miniprogram/pages/merchant/orders/detail/index.wxml` 只显示商品小计、配送费、实付金额。
- `weapp/miniprogram/pages/merchant/orders/list/index.wxml` 只显示 `total_amount`。
- `weapp/miniprogram/pages/merchant/orders/list/index.ts` 动作成功后用动作响应直接覆盖列表行，可能抹掉已经加载的 `fee_breakdown`。
- `weapp/miniprogram/api/merchant-finance.ts` 仍是旧财务口径；本任务记录为后续独立任务，不在本次执行范围内。

---

## 本次目标

完成后：

1. 小程序商户订单 API 类型显式包含 `fee_breakdown`。
2. 商户订单详情展示完整金额明细：
   - 餐费原价
   - 商户优惠
   - 平台/券优惠
   - 餐费应付
   - 配送费
   - 配送优惠
   - 配送应付
   - 用户实付
   - 平台服务费
   - 支付通道费
   - 商户实收
3. 商户订单列表显示轻量摘要：用户实付 + 商户实收。
4. `pending` 不再作为商户订单可见筛选状态进入请求。
5. 商户订单动作成功后刷新列表，以 GET 商户订单接口重新获取带 `fee_breakdown` 的真值。
6. 增加静态契约检查脚本，防止回归到旧字段自算或漏展示关键字段。

非目标：

- 不修改 `locallife/` 后端。
- 不修改 Flutter `merchant_app/`。
- 不重构商户财务页；财务旧口径另起任务。
- 不重做订单列表/详情整体视觉系统。
- 不从 WebSocket snapshot 直接渲染金额，仍以刷新后的商户订单 GET 响应为真值。

---

## 涉及文件

创建：

- `weapp/scripts/check-merchant-order-fee-breakdown.js`  
  静态合同检查，确认小程序接入 `fee_breakdown`，并阻止旧金额自算和内部字段泄露。

修改：

- `weapp/package.json`  
  增加 `check:merchant-order-fee-breakdown`，并纳入 `quality:check`。保留已有 `check:request-id` 未提交改动。
- `weapp/miniprogram/api/order-management.ts`  
  新增 `MerchantOrderFeeBreakdown`，在 `OrderResponse` 加 `fee_breakdown?: MerchantOrderFeeBreakdown`，废弃旧自算方法或改为只读后端真值。
- `weapp/miniprogram/utils/merchant-order-detail-view.ts`  
  新增金额 view model 类型和构建函数。
- `weapp/miniprogram/pages/merchant/orders/detail/index.ts`  
  将 `fee_breakdown` 转为详情页金额行。
- `weapp/miniprogram/pages/merchant/orders/detail/index.wxml`  
  展示完整金额明细。
- `weapp/miniprogram/pages/merchant/orders/detail/index.wxss`  
  补充金额明细样式。
- `weapp/miniprogram/pages/merchant/orders/list/index.ts`  
  列表行增加实付/实收文案；过滤 `pending`；动作后刷新列表。
- `weapp/miniprogram/pages/merchant/orders/list/index.wxml`  
  轻量展示用户实付与商户实收。
- `weapp/miniprogram/pages/merchant/orders/list/index.wxss`  
  补充列表金额摘要样式。

---

## 执行步骤

### 任务 1：用静态契约检查锁住回归

- [ ] 创建 `weapp/scripts/check-merchant-order-fee-breakdown.js`。
- [ ] 脚本必须检查：
  - `order-management.ts` 定义 `MerchantOrderFeeBreakdown`。
  - `OrderResponse` 包含 `fee_breakdown?: MerchantOrderFeeBreakdown`。
  - 类型文件包含 11 个后端字段。
  - `merchant-order-detail-view.ts` 暴露金额明细 view model builder。
  - 详情 WXML 展示 `用户实付`、`平台服务费`、`支付通道费`、`商户实收`。
  - 列表 WXML 展示 `实收`。
  - `order-management.ts` 不再包含旧公式 `subtotal + delivery_fee - delivery_fee_discount - discount_amount`。
  - weapp 商户订单相关代码不出现禁止字段：`provider_payment_fee`、`provider_payment_fee_rate_bps`、`platform_commission`、`operator_commission`。
- [ ] 运行 `cd weapp && node scripts/check-merchant-order-fee-breakdown.js`，预期先失败。
- [ ] 在 `weapp/package.json` 增加：

```json
"check:merchant-order-fee-breakdown": "node scripts/check-merchant-order-fee-breakdown.js"
```

并把 `quality:check` 更新为先运行该检查。

### 任务 2：同步商户订单 API 类型和 adapter

- [ ] 在 `weapp/miniprogram/api/order-management.ts` 新增 `MerchantOrderFeeBreakdown`。
- [ ] 在 `OrderResponse` 增加 `fee_breakdown?: MerchantOrderFeeBreakdown`。保持可选，因为动作接口当前返回旧 `orderResponse`，但列表/详情 GET 必须有。
- [ ] 将 `OrderManagementAdapter.calculateActualAmount()` 改为：

```ts
static getCustomerPayableAmount(order: OrderResponse): number {
  return order.fee_breakdown?.customer_payable_amount ?? order.total_amount
}
```

- [ ] 增加：

```ts
static getMerchantReceivableAmount(order: OrderResponse): number | null {
  return typeof order.fee_breakdown?.merchant_receivable_amount === 'number'
    ? order.fee_breakdown.merchant_receivable_amount
    : null
}
```

- [ ] 保留旧 `calculateActualAmount` 时必须标记 deprecated，并改为调用 `getCustomerPayableAmount()`，不得再自算。
- [ ] `getOrderList` 的 `status` 入参类型排除 `pending`：

```ts
export type MerchantVisibleOrderStatus = Exclude<OrderResponse['status'], 'pending'>
```

### 任务 3：构建金额 view model

- [ ] 在 `weapp/miniprogram/utils/merchant-order-detail-view.ts` 新增：

```ts
export interface MerchantOrderFeeBreakdownRow {
  key: string
  label: string
  value: number
  value_text: string
  tone: 'default' | 'discount' | 'total' | 'income' | 'fee'
  visible: boolean
}

export interface MerchantOrderFeeBreakdownView {
  available: boolean
  unavailable_text: string
  customer_payable_text: string
  merchant_receivable_text: string
  summary_rows: MerchantOrderFeeBreakdownRow[]
  settlement_rows: MerchantOrderFeeBreakdownRow[]
}
```

- [ ] 新增 `buildMerchantOrderFeeBreakdownView(order: OrderResponse)`：
  - `fee_breakdown` 存在时按后端字段生成 rows。
  - 折扣/优惠行金额为负数展示，例如 `-¥3.00`。
  - 0 元优惠行默认不展示，避免详情页过密。
  - `用户实付`、`平台服务费`、`支付通道费`、`商户实收`必须展示，即使金额为 0。
  - 缺失时 `available=false`，`unavailable_text='订单费用明细暂不可用，请稍后重试'`。

### 任务 4：详情页展示完整金额明细

- [ ] `MerchantOrderDetailView` 增加 `fee_breakdown_view: MerchantOrderFeeBreakdownView`。
- [ ] `formatDetail()` 调用 `buildMerchantOrderFeeBreakdownView(order)`。
- [ ] WXML 中把原 `商品小计 / 配送费 / 实付金额` 替换为：
  - `fee_breakdown_view.available` 为真时展示 `summary_rows` 和 `settlement_rows`。
  - 为假时展示就近短提示，不做解释大卡片。
- [ ] 样式保持非顾客侧克制工具风格；只补布局、行高、颜色，不新增营销感卡片。

### 任务 5：列表页轻量金额摘要和动作后回读

- [ ] `MerchantOrderListItem` 增加：

```ts
customer_payable_text: string
merchant_receivable_text: string
fee_breakdown_available: boolean
```

- [ ] `formatOrder()` 使用 adapter/view model 生成：
  - `customer_payable_text`：优先 `fee_breakdown.customer_payable_amount`，缺失时用 `total_amount`。
  - `merchant_receivable_text`：有 `merchant_receivable_amount` 时显示金额，缺失时显示 `金额同步中`。
  - `fee_breakdown_available`：是否有后端金额明细。
- [ ] WXML 将 `合计 ¥...` 改成：
  - `实付 {{item.customer_payable_text}}`
  - `实收 {{item.merchant_receivable_text}}`
- [ ] `OrderStatusFilter` 排除 `pending`，并新增 `normalizeMerchantOrderStatusFilter()`，URL 传入 `pending` 时回落到 `paid`。
- [ ] `performAction()` 成功后不再用动作响应直接 `syncOrderAfterAction(updatedOrder)` 作为最终本地真值；改为：
  - 先更新操作中态。
  - 动作成功后调用 `loadOrders(true, { showLoading: false, preserveCurrent: true })`。
  - 如果回读失败，保留旧数据并显示 `refreshErrorMessage`。

### 任务 6：验证

- [ ] 运行静态检查：

```bash
cd weapp
node scripts/check-merchant-order-fee-breakdown.js
```

预期：PASS。

- [ ] 运行 TypeScript 编译：

```bash
cd weapp
npm run compile
```

预期：无 TypeScript 错误。

- [ ] 运行小程序 lint：

```bash
cd weapp
npm run lint:all
```

预期：无新增 ESLint 错误。

- [ ] 若时间允许，运行：

```bash
cd weapp
npm run quality:check
```

预期：所有门禁通过。若失败，区分本次新增问题与仓库既有问题。

---

## 验收清单

- [ ] `OrderResponse` 可读 `fee_breakdown`。
- [ ] 订单详情页不再前端自算实付金额。
- [ ] 订单详情页可见 `用户实付`、`平台服务费`、`支付通道费`、`商户实收`。
- [ ] 订单列表页可见 `实付` 与 `实收`。
- [ ] `fee_breakdown` 缺失时不显示 0 兜底，不制造假金额。
- [ ] `pending` 不会从小程序商户订单页请求出去。
- [ ] 动作成功后通过列表 GET 回读金额真值。
- [ ] 静态契约检查能防止旧公式和内部字段回归。
- [ ] `npm run compile` 通过。
- [ ] 交付说明写明未处理的商户财务页旧口径风险。

---

## 后续独立任务

商户财务接口和页面仍使用旧字段，例如：

- `weapp/miniprogram/api/merchant-finance.ts`
- `weapp/miniprogram/pages/merchant/finance/*`

后端商户财务新口径为：

- `total_merchant_receivable_amount`
- `total_platform_service_fee_amount`
- `total_payment_channel_fee_amount`
- `merchant_receivable_amount`
- `platform_service_fee_amount`
- `payment_channel_fee_amount`

该部分应另起 `weapp` 商户财务金额口径同步任务，避免本次订单处理页面范围过大。
