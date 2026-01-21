# 商户 Web Catalog 草案（json-render 前置准备）

> 目标：为 json-render 提供可执行的 catalog 结构蓝图；以最新后端/Swagger 为准。
> 约定：数据源统一包裹 `{code,message,data}`；所有接口返回值需 unwrap 到 data。
> 参考：
> - [locallife/docs/swagger.yaml](locallife/docs/swagger.yaml)
> - [locallife/docs/merchant_web_execution_plan_20260121.md](locallife/docs/merchant_web_execution_plan_20260121.md)

---

## 1. Catalog 顶层结构（建议）
- app
  - id, name, version
  - auth: { type: "web-login-qr" }
  - routes: 页面路由表
  - datasources: 统一数据源声明（可复用）
  - pages: 页面定义
  - components: 通用组件定义
  - enums: 枚举/字典表（状态/类型）

---

## 2. 通用数据源（可复用）
> 下面为建议的 datasource 命名与请求契约，具体按 json-render 语法落地。

- ds.auth.webLoginSessionCreate
  - method: POST
  - url: /v1/auth/web-login/sessions
  - response: webLoginSessionStatusResponse
- ds.auth.webLoginSessionStatus
  - method: GET
  - url: /v1/auth/web-login/sessions/{code}
  - response: webLoginSessionStatusResponse
- ds.auth.webLoginConfirm
  - method: POST
  - url: /v1/auth/web-login/confirm
  - request: { code }
  - auth: bearer
- ds.auth.webLoginConsume
  - method: POST
  - url: /v1/auth/web-login/consume
  - request: { code }
  - response: webLoginConsumeResponse

- ds.merchant.me
  - method: GET
  - url: /v1/merchants/me
  - response: merchantResponse
- ds.merchant.update
  - method: PATCH
  - url: /v1/merchants/me
  - request: updateMerchantRequest

- ds.orders.list
  - method: GET
  - url: /v1/merchant/orders
  - request: { page_id, page_size, status? }
  - response: orderResponse[]
- ds.orders.detail
  - method: GET
  - url: /v1/merchant/orders/{id}
- ds.orders.accept
  - method: POST
  - url: /v1/merchant/orders/{id}/accept
- ds.orders.reject
  - method: POST
  - url: /v1/merchant/orders/{id}/reject
  - request: { reason }
- ds.orders.ready
  - method: POST
  - url: /v1/merchant/orders/{id}/ready
- ds.orders.complete
  - method: POST
  - url: /v1/merchant/orders/{id}/complete

- ds.stats.overview
  - method: GET
  - url: /v1/merchant/stats/overview
  - request: { start_date, end_date }
- ds.stats.daily
  - method: GET
  - url: /v1/merchant/stats/daily
  - request: { start_date, end_date }
- ds.stats.hourly
  - method: GET
  - url: /v1/merchant/stats/hourly
  - request: { date }
- ds.stats.dishesTop
  - method: GET
  - url: /v1/merchant/stats/dishes/top
  - request: { start_date, end_date, limit }
- ds.stats.categories
  - method: GET
  - url: /v1/merchant/stats/categories
  - request: { start_date, end_date }
- ds.stats.customers
  - method: GET
  - url: /v1/merchant/stats/customers
  - request: { start_date, end_date, page_id, page_size }
- ds.stats.repurchase
  - method: GET
  - url: /v1/merchant/stats/repurchase
  - request: { start_date, end_date }

- ds.tables.list
  - method: GET
  - url: /v1/tables
- ds.tables.updateStatus
  - method: PATCH
  - url: /v1/tables/{id}/status

---

## 3. 页面 Catalog 草案（示例）

### 3.1 登录页（Web）
- route: /login
- datasources: ds.auth.webLoginSessionCreate, ds.auth.webLoginSessionStatus, ds.auth.webLoginConsume
- states:
  - session: { code, status, expires_at }
  - token: { access_token, refresh_token }
- flow:
  1) create session -> display QR
  2) poll status -> confirmed
  3) consume -> token

### 3.2 工作台（Dashboard）
- route: /dashboard
- datasources: ds.merchant.me, ds.stats.overview, ds.orders.list, ds.tables.list
- fields:
  - merchant: id, name, is_open
  - stats: total_revenue, total_orders
  - orders: id, order_no, status, order_type, total_amount, created_at
  - tables: id, table_no, status, table_type

### 3.3 订单管理
- route: /orders
- datasources: ds.orders.list, ds.orders.detail, ds.orders.accept, ds.orders.reject, ds.orders.ready, ds.orders.complete
- fields:
  - list: orderResponse[]
  - detail: orderResponse

---

## 4. 下一步
1) 按页面补全 datasource 与字段字典（逐页）。
2) 明确每个页面的状态机与交互事件（action -> datasource）。
3) 输出 json-render 真实 catalog JSON（由此草案生成）。
