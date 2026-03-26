# 微信收付通投诉处理 & 补差接口 前端开发规格

> 适用范围：Web（运营商后台，Next.js）+ WeApp（商户端，微信小程序）
> 对应后端 commit：`7345cd0`（feat(payment): 实现微信收付通投诉处理、退款对账和补差接口）
> 日期：2026-03-22

---

## 一、背景与角色说明

### 微信收付通平台角色对应

本项目使用微信 **收付通（电商收付通）** 模式，涉及三个角色：

| 微信角色 | 本项目对应 | 说明 |
|---------|-----------|------|
| 服务商 / 平台商户 | **运营商（Operator）** | 持有平台 mchid，调用微信 API 的签约主体 |
| 二级商户 | **商户（Merchant）** | 有 sub_mchid，资金结算给该账户 |
| 消费者 / 付款用户 | **用户（User）** | 发起投诉的一方 |

### 各功能的角色归属（根据微信文档 API 端点）

| 功能模块 | 微信 API 端点 | 调用方（微信定义） | 涉及端 |
|---------|-------------|-----------------|--------|
| **用户投诉处理** | `/v3/merchant-service/complaints-v2` | **服务商**代管，**二级商户**需回应 | Web（运营商）+ WeApp（商户） |
| **收付通退款账单对账** | `/v3/bill/sub-merchant-fundflow-bill` | **服务商** 专属 | Web（运营商） |
| **补差** | `/v3/ecommerce/subsidies/` | **服务商** 专属，资金打入二级商户 | Web（运营商）仅此端 |

**关键结论：**
- **投诉**：消费者投诉的是二级商户，**二级商户（商户端）** 需在 48h 内回应，服务商（运营商）监督并可代为完结。两端均需实现。
- **补差**：服务商专属 API，商户不在调用链上。**WeApp 商户端暂不实现补差展示**（后端未开放商户侧补差查询接口）。
- **退款对账**：服务商专属，仅运营商后台。

---

本次后端新增三项收付通平台能力：

| 功能模块 | 必要性 | 影响端 |
|---------|--------|--------|
| 用户投诉处理 | 合规必要（微信要求 48h 内回应） | Web（运营商）+ WeApp（商户） |
| 收付通退款账单对账 | 对账盲区修复 | Web（运营商）仅此端 |
| 补差接口 | 平台营销补贴工具 | Web（运营商）仅此端 |

---

## 二、TypeScript 类型定义

新建文件：`web/src/types/wechat-complaint.ts`

```typescript
/**
 * 微信收付通：用户投诉 & 补差类型定义
 * 对齐后端 api/complaint.go + api/subsidy.go
 */

// ==================== 投诉 ====================

export type ComplaintState =
  | 'PENDING_RESPONSE'   // 待商户回应
  | 'PROCESSING'         // 处理中（已回应，待用户确认）
  | 'PROCESSED';         // 已完结

export interface ComplaintItem {
  id: number;
  complaint_id: string;          // 微信投诉单号
  complaint_time: string;        // 投诉时间 (RFC3339)
  payer_openid?: string;         // 投诉用户 openid（脱敏展示）
  complaint_detail: string;      // 投诉内容
  complaint_state: ComplaintState;
  transaction_id?: string;       // 微信支付订单号
  out_trade_no?: string;         // 商户订单号
  amount: number;                // 涉及金额（分）
  response_content?: string;     // 商户回应内容
  responded_at?: string;         // 回应时间 (RFC3339)
  completed_at?: string;         // 完结时间 (RFC3339)
  last_synced_at: string;        // 最后同步时间 (RFC3339)
  wxpay_update_time?: string;    // 微信侧最后更新时间 (RFC3339)
  created_at: string;
  updated_at: string;
}

export interface ComplaintListResponse {
  complaints: ComplaintItem[];
  total: number;
  page: number;
  page_size: number;
}

// ==================== 补差 ====================

export type SubsidyStatus = 'pending' | 'success' | 'failed' | 'canceled';

export interface SubsidyOrder {
  id: number;
  payment_order_id: number;
  sub_mch_id: string;
  transaction_id?: string;
  out_subsidy_no: string;
  payer_amount: number;      // 用户实付金额（分）
  amount: number;            // 补差金额（分）
  description: string;
  status: SubsidyStatus;
  wxpay_subsidy_id?: string;
  fail_reason?: string;
  out_return_no?: string;
  return_amount?: number;
  return_status?: string;
  return_wxpay_id?: string;
  created_at: string;
  updated_at: string;
}

// ==================== 辅助常量 ====================

export const COMPLAINT_STATE_LABELS: Record<ComplaintState, string> = {
  PENDING_RESPONSE: '待回应',
  PROCESSING:       '处理中',
  PROCESSED:        '已完结',
};

export const COMPLAINT_STATE_COLORS: Record<ComplaintState, string> = {
  PENDING_RESPONSE: 'bg-rose-100 text-rose-700',
  PROCESSING:       'bg-amber-100 text-amber-700',
  PROCESSED:        'bg-emerald-100 text-emerald-700',
};

export const SUBSIDY_STATUS_LABELS: Record<SubsidyStatus, string> = {
  pending:  '处理中',
  success:  '已成功',
  failed:   '已失败',
  canceled: '已取消',
};

export const SUBSIDY_STATUS_COLORS: Record<SubsidyStatus, string> = {
  pending:  'bg-amber-100 text-amber-700',
  success:  'bg-emerald-100 text-emerald-700',
  failed:   'bg-rose-100 text-rose-700',
  canceled: 'bg-slate-100 text-slate-600',
};
```

---

## 三、API 封装

### 3.1 投诉接口

```typescript
// web/src/lib/api/complaint.ts

const BASE = '/v1';

// 运营商：查看所有待处理投诉
export const listPendingComplaints = (page = 1, pageSize = 20) =>
  fetch(`${BASE}/operator/complaints?page=${page}&page_size=${pageSize}`);

// 运营商：完结投诉
export const completeComplaintByOperator = (id: number) =>
  fetch(`${BASE}/operator/complaints/${id}/complete`, { method: 'POST' });

// 商户：查看自己的投诉列表
export const listMerchantComplaints = (state?: string, page = 1) =>
  fetch(`${BASE}/merchant/complaints?${state ? `state=${state}&` : ''}page=${page}`);

// 商户：回应投诉
export const respondToComplaint = (id: number, content: string) =>
  fetch(`${BASE}/merchant/complaints/${id}/response`, {
    method: 'POST',
    body: JSON.stringify({ response_content: content }),
  });

// 商户：完结投诉
export const completeComplaintByMerchant = (id: number) =>
  fetch(`${BASE}/merchant/complaints/${id}/complete`, { method: 'POST' });
```

### 3.2 补差接口

```typescript
// web/src/lib/api/subsidy.ts

// 运营商：为收付通支付订单发起补差
export const createSubsidy = (paymentOrderId: number, body: {
  merchant_id: number;
  payer_amount: number;
  amount: number;
  description: string;
}) =>
  fetch(`/v1/operator/payment-orders/${paymentOrderId}/subsidies`, {
    method: 'POST',
    body: JSON.stringify(body),
  });

// 运营商：退回补差
export const returnSubsidy = (paymentOrderId: number, body: {
  out_subsidy_no: string;
  amount: number;
  description: string;
}) =>
  fetch(`/v1/operator/payment-orders/${paymentOrderId}/subsidies/return`, {
    method: 'POST',
    body: JSON.stringify(body),
  });

// 运营商：取消补差
export const cancelSubsidy = (paymentOrderId: number, subsidyId: number) =>
  fetch(`/v1/operator/payment-orders/${paymentOrderId}/subsidies/cancel`, {
    method: 'POST',
    body: JSON.stringify({ subsidy_order_id: subsidyId }),
  });
```

---

## 四、Web 运营商后台

### 4.1 投诉管理页面

**路由**：`/operator/complaints`（新建页面）

**文件**：`web/src/app/operator/complaints/page.tsx`

**页面结构**：

```
PageHeader（标题：用户投诉管理）
  └─ 右侧：状态筛选 Select（全部 / 待回应 / 处理中 / 已完结）
PageContent
  └─ Table（见下方字段）
  └─ Pagination
```

**表格字段**：

| 列名 | 字段 | 备注 |
|------|------|------|
| 投诉单号 | `complaint_id` | 可复制 |
| 投诉内容 | `complaint_detail` | 截断 50 字，悬浮展示全文 |
| 涉及金额 | `amount` | 分→元格式化 |
| 状态 | `complaint_state` | `Badge` 用 `COMPLAINT_STATE_COLORS` |
| 投诉时间 | `complaint_time` | 格式：`YYYY-MM-DD HH:mm` |
| 剩余时限 | 计算字段 | 见 4.1.1 |
| 操作 | — | 「查看详情」按钮 |

#### 4.1.1 48h 回应时限可视化（重要）

微信规则：投诉创建后 **48 小时内**必须回应，否则将影响商户评级。

- 计算：`deadline = complaint_time + 48h`，`remaining = deadline - now`
- `remaining > 12h`：绿色文字 `"剩余 Xh"`
- `6h < remaining ≤ 12h`：橙色文字 `"剩余 Xh"`
- `remaining ≤ 6h`：红色加粗 + ⚠️ 图标 `"紧急！剩余 Xh"`
- `remaining ≤ 0` 且 `state == PENDING_RESPONSE`：红色 `"已超时"`
- `state != PENDING_RESPONSE`：不显示倒计时

```typescript
function getRemainingTime(complaintTime: string, state: ComplaintState) {
  if (state !== 'PENDING_RESPONSE') return null;
  const deadline = new Date(complaintTime).getTime() + 48 * 3600 * 1000;
  const remaining = deadline - Date.now();
  if (remaining <= 0) return { label: '已超时', urgency: 'overdue' };
  const hours = Math.floor(remaining / 3600000);
  const urgency = hours <= 6 ? 'critical' : hours <= 12 ? 'warning' : 'normal';
  return { label: `剩余 ${hours}h`, urgency };
}
```

#### 4.1.2 详情抽屉（Sheet 组件）

点击「查看详情」弹出右侧抽屉，内容：

- 基本信息（投诉单号、用户 openid 后 6 位、关联订单号）
- 完整投诉内容（全文展示）
- 商户回应内容（若有）
- 时间轴：投诉时间 → 回应时间 → 完结时间
- **操作按钮**：
  - `state == PROCESSED`：无操作
  - `state == PROCESSING`：「标记完结」按钮（`AlertDialog` 二次确认）
  - `state == PENDING_RESPONSE`：「标记完结」按钮（运营商可代为跳过，但不建议；建议提示商户回应）

---

### 4.2 对账页面扩展

**路由**：`/operator/finance`（现有页面，**仅扩展 Select 选项**）

在「账单类型」下拉菜单中增加一项：

```typescript
{ value: 'ecommerce_refund', label: '收付通退款对账' }
```

无需新建页面，对账结果展示逻辑完全复用现有 `ecommerce_refund` bill_type 数据。

---

### 4.3 补差功能（集成在订单详情页）

**路由**：`/operator/finance` 或订单详情页（根据现有跳转逻辑）

在支付订单详情页末尾增加「补差」卡片区块：

#### 无补差记录时：

```
Card（标题：平台补差）
  └─ 空状态文字："暂未发起补差"
  └─ Button：「发起补差」→ 打开 Dialog
```

**「发起补差」Dialog 表单字段**：

| 字段 | 组件 | 说明 |
|------|------|------|
| 受益商户 | 自动填充（只读） | 从支付订单关联的 merchant_id 取 |
| 用户实付金额 | `Input` (number) | 单位：元，提交时 ×100 转分 |
| 补差金额 | `Input` (number) | 单位：元，需 ≤ 用户实付金额 |
| 说明 | `Textarea` (max 80) | 显示字数计数 |

提交后乐观更新卡片状态，显示 loading；失败时 Toast 展示微信返回的错误原因。

#### 有补差记录时：

```
Card（标题：平台补差）
  └─ 信息行：用户实付 X 元 | 补差金额 X 元 | 状态 Badge
  └─ 微信补差单号（若有）
  └─ 失败原因（若 status == failed）
  └─ 操作区（条件渲染）：
       status == success  → 「申请退回」Button + 「取消」Button
       status == pending  → 「取消」Button
       status == failed/canceled → 仅展示，无操作
```

**「申请退回」Dialog**：

| 字段 | 组件 | 说明 |
|------|------|------|
| 退回金额 | `Input` (number) | ≤ 原补差金额 |
| 说明 | `Textarea` (max 80) | |

---

## 五、WeApp 商户端

> **范围说明**：根据微信收付通文档，商户（二级商户）负责回应投诉；补差为服务商专属操作，商户不在调用链上，后端也未开放商户侧补差查询接口，因此 WeApp **只实现投诉模块**。

### 5.1 投诉管理页

**路由**：`/pages/complaints/index`（新建页面）

**入口**：商户后台首页「待处理」区域增加「消费者投诉」入口卡片，附带未读红点数量（`PENDING_RESPONSE` 的投诉数）。

**页面结构**：

```
顶部 Tabs（待回应 / 处理中 / 已完结）
  └─ 投诉卡片列表（scroll-view）
       └─ [投诉内容截断] [金额] [时间] [状态角标] [倒计时标记]
       └─ 点击进入详情页
```

**详情页** `/pages/complaints/detail`：

- 展示完整投诉内容、关联订单号（可跳转）
- **待回应状态**：显示回应文本框（maxlength 200）+ 「提交回应」按钮
  - 提交成功后状态变为 `PROCESSING`，并 Toast 提示
- **处理中状态**：显示已提交的回应内容 + 「标记完结」按钮
- **已完结状态**：只读展示

#### 5.1.1 48h 提醒

- 进入列表页时，若存在 `PENDING_RESPONSE` 且剩余时间 ≤ 12h 的投诉，在页面顶部展示红色横幅："您有 X 条投诉需在 Xh 内回复，否则将影响店铺评级"

#### 5.1.2 消息订阅（可选，需后端配合）

新投诉创建后，后端通过微信订阅消息向商户主推送通知（需商户授权"商户投诉提醒"消息模板）。小程序端只需在进入关键页面时调用 `wx.requestSubscribeMessage`，无需额外前端开发。

---

### 5.2 补差（不实现）

补差为服务商（运营商）专属，微信 API 端点 `/v3/ecommerce/subsidies/` 调用方为服务商。后端未开放商户侧补差查询接口，WeApp 本次不实现。若未来需要商户查看补差记录，需后端先开放 `GET /merchant/payment-orders/:id/subsidies` 接口。

---

## 六、完整 API 端点速查

### 投诉

| 调用方 | 方法 | 路径 | 说明 |
|--------|------|------|------|
| 运营商 | GET | `/v1/operator/complaints?state=&page=&page_size=` | 查看所有待处理投诉 |
| 运营商 | POST | `/v1/operator/complaints/:id/complete` | 完结投诉 |
| 商户 | GET | `/v1/merchant/complaints?state=&page=` | 查看自己的投诉列表 |
| 商户 | GET | `/v1/merchant/complaints/:id` | 查看投诉详情 |
| 商户 | POST | `/v1/merchant/complaints/:id/response` | 回应投诉 `{response_content: string}` |
| 商户 | POST | `/v1/merchant/complaints/:id/complete` | 完结投诉 |

### 补差

| 调用方 | 方法 | 路径 | 说明 |
|--------|------|------|------|
| 运营商 | POST | `/v1/operator/payment-orders/:id/subsidies` | 发起补差 |
| 运营商 | POST | `/v1/operator/payment-orders/:id/subsidies/return` | 退回补差 |
| 运营商 | POST | `/v1/operator/payment-orders/:id/subsidies/cancel` | 取消补差 |

### 请求体示例

**发起补差**
```json
{
  "merchant_id": 123,
  "payer_amount": 5000,
  "amount": 200,
  "description": "满减活动平台补贴"
}
```

**回应投诉**
```json
{
  "response_content": "您好，我们已核实该情况，将在24小时内为您处理。"
}
```

---

## 七、任务拆解建议

| 任务 | 端 | 优先级 | 估时 |
|------|----|--------|------|
| 新增 `wechat-complaint.ts` 类型文件 | Web | P0 | 0.5h |
| 运营商投诉列表页（含 48h 倒计时） | Web | P0 | 1.5d |
| 运营商投诉详情抽屉 | Web | P0 | 0.5d |
| 对账页增加 ecommerce_refund 选项 | Web | P1 | 0.5h |
| 订单详情页补差卡片 | Web | P1 | 1d |
| WeApp 投诉列表 + 详情 + 回应 | WeApp | P0 | 1.5d |
| ~~WeApp 订单详情页补差只读展示~~ | WeApp | 不实现（缺后端接口） | — |
