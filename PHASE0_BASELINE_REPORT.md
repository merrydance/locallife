# 阶段 0：基线梳理与规范化（执行中）

> 本文档用于落地“阶段 0”交付物。当前内容为**已确认**与**待补全**两部分，后续逐项补齐并勾选。

## 0. 进度
- [x] 统一状态机矩阵与错误码对照表
- [x] 堂食/外卖/预订分场景规则清单（允许/禁止/触发点/责任方）
- [ ] 错误码规范统一（API/前端/日志字段）
- [ ] region_id 强约束策略（API 中间件 + SQL 约束 + 审计）
- [x] API/表/页面映射清单（小程序/商户/运营商/平台）
- [ ] 上线验收 checklist

> 收口提示：剩余工作主要集中在“状态机矩阵全量化”和“错误码统一落地”。

---

## 1. 统一状态机矩阵与错误码对照（已部分完成）

### 1.1 枚举来源（已确认）
- 订单状态/履约状态/订单类型：
  - [locallife/api/order.go](locallife/api/order.go#L17-L66)
- 预订状态：
  - [locallife/api/constants.go](locallife/api/constants.go#L7-L30)
- 支付状态：
  - [locallife/api/payment_order.go](locallife/api/payment_order.go#L29-L60)
- KDS 状态：
  - [locallife/api/kitchen.go](locallife/api/kitchen.go#L15-L35)

### 1.2 错误码与响应封装（已确认）
- 统一错误码：
  - [locallife/api/response_envelope.go](locallife/api/response_envelope.go#L14-L70)
  - 约定映射（基线）：
    - 400 → 40000（参数错误/状态不允许）
    - 401 → 40100（未授权）
    - 403 → 40300（权限不足）
    - 404 → 40400（资源不存在）
    - 409 → 40900（冲突/状态不一致）
    - 422 → 42200（语义错误）
    - 429 → 42900（限流）
    - 5xx → 50000/50200/50300/50400

### 1.2.1 前端错误码消费与日志字段（已确认）
| 维度 | Web（商户/运营） | 小程序（C 端/运营/骑手） |
| --- | --- | --- |
| 成功判定 | 仅判断 HTTP ok；如带 `ApiEnvelope` 则取 `data`，不判断 `code` | 统一响应信封；`response.code === ErrorCode.SUCCESS` 才成功 | 
| 错误字段 | `handleError` 读取 `message` / `error` / `data.error` | HTTP 非 200/201 直接抛 `AppError`；`response.code` 非成功也抛 | 
| 401 处理 | 401 触发 refresh，失败清 token 并跳转登录 | 401 触发 refresh，失败清 token 并抛 `AUTH` | 
| 日志字段 | 无统一 error envelope 日志字段（前端仅抛 `Error`） | `AppError` 包含 `type/message/userMessage/originalError` 并记录 | 
| 统一信封 | 可选（兼容无 envelope） | 强制（请求头带 `X-Response-Envelope: 1`） | 
| 证据 | [web/src/lib/api.ts](web/src/lib/api.ts#L1-L220) | [weapp/miniprogram/utils/request.ts](weapp/miniprogram/utils/request.ts#L180-L380), [weapp/miniprogram/utils/error-handler.ts](weapp/miniprogram/utils/error-handler.ts#L1-L100) |

**错误码枚举差异（已确认）**
- 小程序 `ErrorCode` 仍使用 HTTP 风格（400/401/403/404/500），且成功码为 0：
  - [weapp/miniprogram/api/types.ts](weapp/miniprogram/api/types.ts#L46-L78)
- 后端基线为 40000/40100/40300/40400/40900/42200/42900/5xx：
  - [locallife/api/response_envelope.go](locallife/api/response_envelope.go#L14-L70)
- 结论：前端 `ErrorCode` 与后端基线**不一致**，需要在 Phase 0 统一（映射或升级枚举）。

### 1.2.2 错误码统一策略草案（待确认）
- 后端保持 `code` 为业务码（40000/40100/…），`message` 为人类可读描述，`data` 承载细节。
- 前端（Web/小程序）统一按 `code` 判定成功与失败；仅当 `code` 缺失时回退到 HTTP ok。
- 小程序 `ErrorCode` 更新为业务码枚举，增加对 409/422/429/5xx 的显式分支；Web 补充对 `code` 的判断与映射。

### 1.2.3 小程序 HTTP 错误提示映射（已确认）
| HTTP | 用户提示（默认） | 备注 | 证据 |
| --- | --- | --- | --- |
| 400 | 请求参数错误 | 特例：`merchant is not accepting takeout orders` → “商户休息中～” | [weapp/miniprogram/utils/request.ts](weapp/miniprogram/utils/request.ts#L256-L272) |
| 409 | 操作冲突，请稍后重试 | 业务冲突类 | [weapp/miniprogram/utils/request.ts](weapp/miniprogram/utils/request.ts#L274-L278) |
| 404 | 服务暂时不可用,请稍后重试 | 后端未启动或路由不存在 | [weapp/miniprogram/utils/request.ts](weapp/miniprogram/utils/request.ts#L279-L282) |
| 502/503/504 | 服务暂时不可用,请稍后重试 | 网关错误 | [weapp/miniprogram/utils/request.ts](weapp/miniprogram/utils/request.ts#L282-L284) |
| 5xx | 服务器内部错误,请稍后重试 | 未区分具体错误 | [weapp/miniprogram/utils/request.ts](weapp/miniprogram/utils/request.ts#L285-L287) |

### 1.2.4 错误码统一落地任务（待执行）
- 后端：所有 JSON 响应统一包裹 `APIResponse`（确保 `code/message/data` 一致），并在 4xx/5xx 时带业务码。
- Web：`api.ts` 增加 `code` 判定逻辑（优先 `code`，无 `code` 才回退 HTTP）。
- Web：补充 `X-Response-Envelope: 1` 请求头，减少非信封响应分支。
- 小程序：更新 `ErrorCode` 枚举为业务码（40000/40100/…），保留 `SUCCESS=0`，并补齐 409/422/429 分支。
- 日志：统一打印 `code`、`request_id/trace_id`（如有），并在前端错误上报中携带 `code`。

### 1.2.6 错误码统一落地清单（建议执行顺序）
**后端（API）**
- 将以下非统一响应改为 `APIResponse`：
  - 位置服务：`location.go`（`Code/Message/Data` → `code/message/data`）
  - 超时中间件：`middleware.go`（`gin.H{error: ...}` → `APIResponse`）
  - 外卖风控拒绝：`order.go`（`gin.H{error: ...}` → `APIResponse`）
- 统一错误码来源：禁止 handler 内返回“裸 HTTP 错误”，改为 `ErrorCode` 映射。
- 增补 `payment failed` 的 API 枚举或禁止该状态进入 API 返回。

**Web**
- `api.ts`：优先读取 `response.code`，`code !== 0` 视为失败。
- 统一错误上报字段：`code`、`message`、`request_id/trace_id`。

**小程序**
- `ErrorCode` 枚举与后端业务码对齐，保留 `SUCCESS=0`。
- `request.ts`：HTTP 非 2xx 与 `code != 0` 统一抛错，并标准化 `userMessage`。

**验收标准**
- 100% JSON API 均返回 `{code,message,data}`，无例外。
- 前端（Web/小程序）成功/失败判定仅依赖 `code`，HTTP 仅作兜底。
- 401/403/404/409/422/429/5xx 在前端均有明确 UI 文案分支。

### 1.2.7 错误码统一改造清单（文件级）
**后端**
- 统一响应封装入口：
  - [locallife/api/response_envelope.go](locallife/api/response_envelope.go)
- 非统一响应修复：
  - 位置服务： [locallife/api/location.go](locallife/api/location.go)
  - 超时中间件： [locallife/api/middleware.go](locallife/api/middleware.go)
  - 外卖风控拒绝： [locallife/api/order.go](locallife/api/order.go)
- 统一错误码枚举来源：
  - [locallife/api/response_envelope.go](locallife/api/response_envelope.go)

**Web**
- `code` 判定与 envelope 头： [web/src/lib/api.ts](web/src/lib/api.ts)

**小程序**
- `ErrorCode` 枚举： [weapp/miniprogram/api/types.ts](weapp/miniprogram/api/types.ts)
- 请求封装与错误处理： [weapp/miniprogram/utils/request.ts](weapp/miniprogram/utils/request.ts), [weapp/miniprogram/utils/error-handler.ts](weapp/miniprogram/utils/error-handler.ts)

### 1.2.5 中间件错误码来源（已确认）
- 认证中间件：缺失/无效 token → 401 → 40100
  - [locallife/api/middleware.go](locallife/api/middleware.go#L33-L58)
- 超时中间件：请求超时 → 504 → 50400
  - [locallife/api/middleware.go](locallife/api/middleware.go#L78-L108)
  - 缺口：当前返回 `{error: "request timeout"}` 未包裹 `APIResponse`

**非统一响应信封（需收口）**
- 位置服务接口自定义返回体（`Code/Message/Data`），与 `APIResponse` 不一致：
  - [locallife/api/location.go](locallife/api/location.go#L20-L45)
- 外卖风控拒绝返回 `gin.H`（未包裹 `APIResponse`）：
  - [locallife/api/order.go](locallife/api/order.go#L1386-L1397)

### 1.3 状态矩阵（待补全）
> 先补齐核心状态枚举与动作约束，完整矩阵（含错误码）后续补充。

**待补齐的矩阵项（收口清单）**
- 订单状态 × 角色动作 × 错误码（merchant/user/rider）全量表
- 预订状态 × 动作 × 错误码（user/merchant）全量表
- 支付状态 × 动作 × 错误码（create/close/refund）全量表
- KDS 状态 × 动作 × 错误码全量表

**订单状态（Order）**
- 枚举：`pending` → `paid` → `preparing` → `ready` → `courier_accepted` → `picked` → `delivering` → `rider_delivered` → `user_delivered` → `completed`
  - [locallife/api/order.go](locallife/api/order.go#L28-L50)

**订单状态主路径（基线对照）**
- 主链路（外卖）：`pending` → `paid` → `preparing` → `ready` → `courier_accepted` → `picked` → `delivering` → `rider_delivered` → `user_delivered`
- 主链路（堂食/打包）：`pending` → `paid` → `preparing` → `ready` → `completed`
- 取消：`pending/paid` → `cancelled`（制作/配送中进入售后通道）

**履约状态（Fulfillment）**
- `scheduled` / `pending_kitchen` / `preparing` / `ready` / `completed` / `cancelled`
  - [locallife/api/order.go](locallife/api/order.go#L52-L60)

**履约状态初始化（已确认）**
- 下单创建默认 `FulfillmentStatus = scheduled`
  - [locallife/api/order.go](locallife/api/order.go#L1402-L1416)

**配送动作约束（骑手）**
- `grab`：仅 `paid/preparing/ready`
- `start_pickup`：仅 `courier_accepted`
- `confirm_pickup`：仅 `courier_accepted`
- `start_delivery`：仅 `picked`
- `confirm_delivery`：仅 `delivering`
  - [locallife/api/delivery.go](locallife/api/delivery.go#L736-L764)

**预订状态（Reservation）**
- `pending` / `paid` / `confirmed` / `checked_in` / `completed` / `cancelled` / `expired` / `no_show`
  - [locallife/api/constants.go](locallife/api/constants.go#L13-L24)

**厨房状态（KDS）**
- `new` / `preparing` / `ready`
  - [locallife/api/kitchen.go](locallife/api/kitchen.go#L14-L20)

**支付状态（Payment）**
- `pending` / `paid` / `refunded` / `closed`
  - [locallife/api/payment_order.go](locallife/api/payment_order.go#L33-L40)

### 1.3.1 用户侧订单动作矩阵（已确认）
| 订单状态 | 可用动作 | 证据 |
| --- | --- | --- |
| `pending` | pay, cancel | [locallife/api/order.go](locallife/api/order.go#L616-L621) |
| `paid` | cancel, urge | [locallife/api/order.go](locallife/api/order.go#L622-L625) |
| `preparing` / `ready` | urge | [locallife/api/order.go](locallife/api/order.go#L626-L631) |
| `courier_accepted` / `picked` / `delivering` | urge | [locallife/api/order.go](locallife/api/order.go#L632-L634) |
| `rider_delivered` | confirm, complain | [locallife/api/order.go](locallife/api/order.go#L635-L637) |
| `user_delivered` / `completed` | complain | [locallife/api/order.go](locallife/api/order.go#L638-L639) |

### 1.3.2 履约状态变更与动作（已确认）
| 动作 | 履约状态变化 | 证据 |
| --- | --- | --- |
| 商户接单 | `preparing` | [locallife/api/order.go](locallife/api/order.go#L2871-L2882) |
| 商户拒单 | `cancelled` | [locallife/api/order.go](locallife/api/order.go#L2985-L2994) |
| 商户出餐 | `ready` | [locallife/api/order.go](locallife/api/order.go#L3134-L3142) |

### 1.3.3 订单状态 SQL 更新约束（已确认）
| 目标状态 | 允许的当前状态 | 证据 |
| --- | --- | --- |
| `courier_accepted` | `ready` / `courier_accepted` | [locallife/db/query/order.sql](locallife/db/query/order.sql#L163-L172) |
| `picked` | `ready` / `courier_accepted` / `picked` | [locallife/db/query/order.sql](locallife/db/query/order.sql#L173-L181) |
| `delivering` | `picked` / `delivering` | [locallife/db/query/order.sql](locallife/db/query/order.sql#L182-L188) |
| `rider_delivered` | `delivering` / `rider_delivered` | [locallife/db/query/order.sql](locallife/db/query/order.sql#L189-L196) |
| `user_delivered` | `rider_delivered` / `user_delivered` | [locallife/db/query/order.sql](locallife/db/query/order.sql#L199-L206) |

### 1.3.4 KDS 动作矩阵（已确认）
| 动作 | 允许状态 | 禁止时返回（HTTP→业务码） | 证据 |
| --- | --- | --- | --- |
| 开始制作 | 订单 `paid` | 400 → 40000（order is not in paid status） | [locallife/api/kitchen.go](locallife/api/kitchen.go#L292-L307) |
| 出餐完成 | 订单 `preparing` 或 `paid` | 400 → 40000（order is not in preparing or paid status） | [locallife/api/kitchen.go](locallife/api/kitchen.go#L368-L383) |

### 1.3.5 支付状态 SQL 更新约束（已确认）
| 目标状态 | 允许的当前状态 | 证据 |
| --- | --- | --- |
| `paid` | `pending` | [locallife/db/query/payment_order.sql](locallife/db/query/payment_order.sql#L55-L61) |
| `failed` | `pending` | [locallife/db/query/payment_order.sql](locallife/db/query/payment_order.sql#L63-L67) |
| `closed` | `pending` | [locallife/db/query/payment_order.sql](locallife/db/query/payment_order.sql#L69-L73) |
| `refunded` | 任意（无状态限制） | [locallife/db/query/payment_order.sql](locallife/db/query/payment_order.sql#L75-L79) |

**支付状态缺口（已确认）**
- SQL 存在 `failed`，但 API 枚举未暴露该状态（需补齐或禁止产生）。
  - [locallife/db/query/payment_order.sql](locallife/db/query/payment_order.sql#L63-L67)

### 1.3.6 预订动作-状态矩阵（已确认）
| 动作 | 允许状态 | 证据 |
| --- | --- | --- |
| 确认 | `paid` | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L35-L41) |
| 完成 | `confirmed` | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L40-L41) |
| 取消 | `pending` / `paid` / `confirmed` | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L42-L43) |
| 未到店 | `paid` / `confirmed` | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L43-L44) |
| 签到 | `paid` / `confirmed` | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L45-L46) |
| 起菜 | `confirmed` / `checked_in` | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L47-L48) |

### 1.3.7 配送动作-订单状态矩阵（已确认）
| 动作 | 允许订单状态 | 证据 |
| --- | --- | --- |
| 抢单 | `paid` / `preparing` / `ready` | [locallife/api/delivery.go](locallife/api/delivery.go#L754-L760) |
| 开始取餐 | `courier_accepted` | [locallife/api/delivery.go](locallife/api/delivery.go#L760-L763) |
| 确认取餐 | `courier_accepted` | [locallife/api/delivery.go](locallife/api/delivery.go#L760-L763) |
| 开始配送 | `picked` | [locallife/api/delivery.go](locallife/api/delivery.go#L762-L763) |
| 确认送达 | `delivering` | [locallife/api/delivery.go](locallife/api/delivery.go#L763-L764) |

### 1.3.8 订单状态 × 动作 × 错误码（摘要版，已确认）
| 角色 | 动作 | 允许状态/条件 | 禁止时返回（HTTP→业务码） | 证据 |
| --- | --- | --- | --- | --- |
| 商户 | 接单 | `paid` | 400 → 40000（only paid orders can be accepted） | [locallife/api/order.go](locallife/api/order.go#L2864-L2899) |
| 商户 | 拒单 | `paid` | 400 → 40000（only paid orders can be rejected） | [locallife/api/order.go](locallife/api/order.go#L2976-L3006) |
| 商户 | 出餐 | `preparing` | 400 → 40000（only preparing orders can be marked as ready） | [locallife/api/order.go](locallife/api/order.go#L3125-L3166) |
| 商户 | 完成 | 非外卖 + `ready` | 400 → 40000（takeout not allowed / only ready） | [locallife/api/order.go](locallife/api/order.go#L3219-L3236) |
| 用户 | 取消订单 | `pending/paid` | 400 → 40000（状态不允许取消） | [locallife/api/order.go](locallife/api/order.go#L2027-L2088) |
| 用户 | 催单 | `paid/preparing/ready/courier_accepted/picked/delivering/rider_delivered` | 400 → 40000（order cannot be urged in current status） | [locallife/api/order.go](locallife/api/order.go#L2182-L2194) |
| 用户 | 确认收货 | `takeout` + `rider_delivered` | 400 → 40000（类型或状态不允许） | [locallife/api/order.go](locallife/api/order.go#L2524-L2554) |
| 骑手 | 抢单 | `paid/preparing/ready` | 400 → 40000（状态不允许接单） | [locallife/api/delivery.go](locallife/api/delivery.go#L552-L576) |
| 骑手 | 开始取餐 | 配送单 `assigned` + 订单 `courier_accepted` | 400 → 40000（当前状态不允许） | [locallife/api/delivery.go](locallife/api/delivery.go#L836-L866) |
| 骑手 | 确认取餐 | 配送单 `picking` + 订单 `courier_accepted` | 400 → 40000（当前状态不允许） | [locallife/api/delivery.go](locallife/api/delivery.go#L900-L940) |
| 骑手 | 开始配送 | 配送单 `picked` | 400 → 40000（当前状态不允许） | [locallife/api/delivery.go](locallife/api/delivery.go#L1039-L1047) |
| 骑手 | 确认送达 | 未见配送单状态校验 | ⚠️ 缺口：需限制 `delivering`，否则可越级完成 | [locallife/api/delivery.go](locallife/api/delivery.go#L1115-L1168) |

### 1.3.9 支付状态 × 动作 × 错误码（摘要版，已确认）
| 动作 | 允许状态/条件 | 禁止时返回（HTTP→业务码） | 证据 |
| --- | --- | --- | --- |
| 创建支付单 | 订单 `pending` + 归属用户 | 400 → 40000（order is not in pending status） | [locallife/api/payment_order.go](locallife/api/payment_order.go#L246-L262) |
| 关闭支付单 | 支付单 `pending` + 归属用户 | 400 → 40000（only pending payment orders can be closed） | [locallife/api/payment_order.go](locallife/api/payment_order.go#L608-L616) |
| 发起退款 | 支付单 `paid` + 归属商户 | 400 → 40000（payment order is not paid / amount exceeds） | [locallife/api/payment_order.go](locallife/api/payment_order.go#L736-L766) |

### 1.3.10 预订状态 × 动作 × 错误码（摘要版，已确认）
| 动作 | 允许状态/条件 | 禁止时返回（HTTP→业务码） | 证据 |
| --- | --- | --- | --- |
| 确认 | `paid` | 409 → 40900（reservation is not paid） | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1024-L1030) |
| 完成 | `confirmed` | 409 → 40900（reservation is not confirmed） | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1112-L1116) |
| 取消 | `pending/paid/confirmed` | 409 → 40900（状态不允许或超出退款时限） | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1203-L1218) |
| 签到 | `paid/confirmed` | 409 → 40900（only paid/confirmed） | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L2124-L2129) |
| 起菜 | `confirmed/checked_in` | 409 → 40900（only confirmed/checked-in） | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L2201-L2204) |
| 标记未到店 | `paid/confirmed` | 409 → 40900（only paid/confirmed） | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1360-L1364) |

### 1.3.11 KDS 状态 × 动作 × 错误码（摘要版，已确认）
| 动作 | 允许状态/条件 | 禁止时返回（HTTP→业务码） | 证据 |
| --- | --- | --- | --- |
| 开始制作 | 订单 `paid` | 400 → 40000（order is not in paid status） | [locallife/api/kitchen.go](locallife/api/kitchen.go#L292-L307) |
| 出餐完成 | 订单 `preparing` 或 `paid` | 400 → 40000（order is not in preparing or paid status） | [locallife/api/kitchen.go](locallife/api/kitchen.go#L368-L383) |

### 1.3.12 订单状态机全量校验（按角色，已覆盖核心动作）
> 说明：以下为**已实现校验**的动作清单，覆盖“商户/用户/骑手”的核心动作与错误码。

**商户动作**
| 动作 | 允许状态/条件 | 禁止时返回（HTTP→业务码） | 证据 |
| --- | --- | --- | --- |
| 接单 | `paid` | 400 → 40000（only paid orders can be accepted） | [locallife/api/order.go](locallife/api/order.go#L2864-L2899) |
| 拒单 | `paid` | 400 → 40000（only paid orders can be rejected） | [locallife/api/order.go](locallife/api/order.go#L2976-L3006) |
| 出餐 | `preparing` | 400 → 40000（only preparing orders can be marked as ready） | [locallife/api/order.go](locallife/api/order.go#L3125-L3166) |
| 完成 | 非外卖 + `ready` | 400 → 40000（takeout not allowed / only ready） | [locallife/api/order.go](locallife/api/order.go#L3219-L3236) |

**用户动作**
| 动作 | 允许状态/条件 | 禁止时返回（HTTP→业务码） | 证据 |
| --- | --- | --- | --- |
| 取消订单 | `pending/paid` | 400 → 40000（状态不允许取消） | [locallife/api/order.go](locallife/api/order.go#L2027-L2088) |
| 催单 | `paid/preparing/ready/courier_accepted/picked/delivering/rider_delivered` | 400 → 40000（order cannot be urged in current status） | [locallife/api/order.go](locallife/api/order.go#L2182-L2194) |
| 确认收货 | `takeout` + `rider_delivered` | 400 → 40000（only takeout / not ready for confirmation） | [locallife/api/order.go](locallife/api/order.go#L2545-L2554) |

**骑手动作（配送单）**
| 动作 | 允许状态/条件 | 禁止时返回（HTTP→业务码） | 证据 |
| --- | --- | --- | --- |
| 抢单 | 订单 `paid/preparing/ready` | 400 → 40000（当前订单状态不允许接单） | [locallife/api/delivery.go](locallife/api/delivery.go#L552-L576) |
| 开始取餐 | 配送单 `assigned` + 订单允许 | 400 → 40000（状态不允许） | [locallife/api/delivery.go](locallife/api/delivery.go#L836-L874) |
| 确认取餐 | 配送单 `picking` + 订单允许 | 400 → 40000（状态不允许） | [locallife/api/delivery.go](locallife/api/delivery.go#L900-L940) |
| 开始配送 | 配送单 `picked` | 400 → 40000（状态不允许） | [locallife/api/delivery.go](locallife/api/delivery.go#L1040-L1060) |
| 确认送达 | 期望 `delivering` | ⚠️ 缺口：未校验配送单状态，需补充 | [locallife/api/delivery.go](locallife/api/delivery.go#L1125-L1185) |

**通用权限/资源错误（适用于所有动作）**
- 403 → 40300：资源不归属当前操作者（用户/商户/骑手）。
  - 示例：订单归属校验 [locallife/api/order.go](locallife/api/order.go#L2014-L2038)
  - 示例：配送单归属校验 [locallife/api/delivery.go](locallife/api/delivery.go#L842-L850)
- 404 → 40400：订单/配送单不存在。
  - 示例：订单不存在 [locallife/api/order.go](locallife/api/order.go#L2001-L2010)
  - 示例：配送单不存在 [locallife/api/delivery.go](locallife/api/delivery.go#L906-L914)

### 1.4 统一错误码对照表（摘要版，已确认）
| 动作 | HTTP | 业务码 | 触发原因 | 证据 |
| --- | --- | --- | --- | --- |
| 商户接单 | 400 | 40000 | 仅 `paid` 可接单 | [locallife/api/order.go](locallife/api/order.go#L2864-L2899) |
| 商户拒单 | 400 | 40000 | 仅 `paid` 可拒单 | [locallife/api/order.go](locallife/api/order.go#L2976-L3006) |
| 商户出餐 | 400 | 40000 | 仅 `preparing` 可出餐 | [locallife/api/order.go](locallife/api/order.go#L3125-L3166) |
| 商户完成 | 400 | 40000 | 仅 `ready` 且非外卖 | [locallife/api/order.go](locallife/api/order.go#L3219-L3236) |
| 用户取消 | 400 | 40000 | 状态不允许取消 | [locallife/api/order.go](locallife/api/order.go#L2027-L2088) |
| 用户确认收货 | 400 | 40000 | 非外卖或状态不允许 | [locallife/api/order.go](locallife/api/order.go#L2524-L2554) |
| 预订确认 | 409 | 40900 | 未支付 | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1024-L1030) |
| 预订完成 | 409 | 40900 | 未确认 | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1112-L1116) |
| 预订取消 | 409 | 40900 | 状态不允许或超出退款时限 | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1203-L1218) |
| 骑手动作 | 400 | 40000 | 订单状态不允许 | [locallife/api/delivery.go](locallife/api/delivery.go#L552-L576) |
| 创建支付单 | 400 | 40000 | 订单非 `pending` | [locallife/api/payment_order.go](locallife/api/payment_order.go#L246-L262) |

---

## 2. 堂食/外卖/预订分场景规则清单（已部分完成）

### 2.1 堂食（部分确认）
- 商户接单：仅 `paid`，否则 40000
  - [locallife/api/order.go](locallife/api/order.go#L2829-L2885)
- 商户拒单：仅 `paid`，否则 40000
  - [locallife/api/order.go](locallife/api/order.go#L2950-L3025)
- 商户出餐：仅 `preparing`
  - [locallife/api/order.go](locallife/api/order.go#L3050-L3115)
- 商户完成：仅 `ready` 且非外卖
  - [locallife/api/order.go](locallife/api/order.go#L3180-L3235)
- 用户取消：仅 `pending/paid`，制作/配送中走异常通道
  - [locallife/api/order.go](locallife/api/order.go#L1992-L2088)

### 2.2 外卖（部分确认）
- 用户确认收货：仅 `rider_delivered` 且 `takeout`
  - [locallife/api/order.go](locallife/api/order.go#L2500-L2555)
- 商户完成：外卖不允许商户完成
  - [locallife/api/order.go](locallife/api/order.go#L3199-L3230)
- 骑手动作与订单状态约束：
  - [locallife/api/delivery.go](locallife/api/delivery.go#L742-L770)
- 缺口：骑手确认送达未校验配送单状态（应限制 `delivering`）
  - [locallife/api/delivery.go](locallife/api/delivery.go#L1115-L1168)

### 2.3 预订（部分确认）
- 预订状态允许矩阵：
  - [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L14-L50)
- 预订确认/完成/取消/签到/起菜/未到店：
  - [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L990-L2210)
  - 签到与起菜约束：
    - [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L2077-L2215)
- 退款截止时间：用户取消在退款截止后返回冲突
  - [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1212-L1218)
- 未到店：仅 `paid/confirmed` 可标记为 no-show（通常不退款）
  - [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1360-L1392)

### 2.4 关键动作规则矩阵（含错误码，已部分确认）
> 说明：业务码按“HTTP → 业务码”映射（见 1.2）。

| 场景 | 动作 | 允许状态/条件 | 禁止时返回（HTTP→业务码） | 证据 |
| --- | --- | --- | --- | --- |
| 堂食/商户 | 接单 | 仅 `paid` | 400 → 40000（only paid orders can be accepted） | [locallife/api/order.go](locallife/api/order.go#L2864-L2899) |
| 堂食/商户 | 拒单 | 仅 `paid` | 400 → 40000（only paid orders can be rejected） | [locallife/api/order.go](locallife/api/order.go#L2976-L3006) |
| 堂食/商户 | 出餐 | 仅 `preparing` | 400 → 40000（only preparing orders can be marked as ready） | [locallife/api/order.go](locallife/api/order.go#L3125-L3166) |
| 堂食/商户 | 完成 | 非外卖 + `ready` | 400 → 40000（takeout not allowed / only ready） | [locallife/api/order.go](locallife/api/order.go#L3219-L3236) |
| 用户 | 取消订单 | 仅 `pending/paid` | 400 → 40000（制作/配送中转售后，或状态不允许） | [locallife/api/order.go](locallife/api/order.go#L2027-L2088) |
| 用户 | 催单 | `paid/preparing/ready/courier_accepted/picked/delivering/rider_delivered` | 400 → 40000（order cannot be urged in current status） | [locallife/api/order.go](locallife/api/order.go#L2182-L2194) |
| 预订/用户 | 替换订单 | 仅预订单 + `paid` + 未替换 + 预订 `full` + 预订 `paid/confirmed/checked_in` + 有用餐会话 | 400 → 40000（类型/缺字段/非全款），409 → 40900（状态冲突/无会话） | [locallife/api/order.go](locallife/api/order.go#L2284-L2344) |
| 外卖/用户 | 确认收货 | 仅 `takeout` + `rider_delivered` | 400 → 40000（类型或状态不允许） | [locallife/api/order.go](locallife/api/order.go#L2524-L2554) |
| 外卖/骑手 | 抢单 | `paid/preparing/ready` | 400 → 40000（状态不允许接单） | [locallife/api/delivery.go](locallife/api/delivery.go#L552-L576) |
| 外卖/骑手 | 开始取餐 | 配送单 `assigned` + 订单 `courier_accepted` | 400 → 40000（当前状态不允许） | [locallife/api/delivery.go](locallife/api/delivery.go#L836-L866) |
| 外卖/骑手 | 确认取餐 | 配送单 `picking` + 订单 `courier_accepted` | 400 → 40000（当前状态不允许） | [locallife/api/delivery.go](locallife/api/delivery.go#L900-L940) |
| 外卖/骑手 | 开始配送 | 配送单 `picked` | 400 → 40000（当前状态不允许） | [locallife/api/delivery.go](locallife/api/delivery.go#L1039-L1047) |
| 外卖/骑手 | 确认送达 | 未见配送单状态校验 | ⚠️ 缺口：需限制 `delivering`，否则可越级完成 | [locallife/api/delivery.go](locallife/api/delivery.go#L1115-L1168) |
| 预订/商户 | 确认 | 仅 `paid` | 409 → 40900（reservation is not paid） | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1024-L1030) |
| 预订/商户 | 完成 | 仅 `confirmed` | 409 → 40900（reservation is not confirmed） | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1112-L1116) |
| 预订/用户/商户 | 取消 | `pending/paid/confirmed` | 409 → 40900（状态不允许或超出退款时限） | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1203-L1218) |
| 预订/用户/商户 | 签到 | `paid/confirmed` | 409 → 40900（only paid/confirmed） | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L2124-L2129) |
| 预订/用户/商户 | 起菜 | `confirmed/checked_in` | 409 → 40900（only confirmed/checked-in） | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L2201-L2204) |
| 预订/商户 | 标记未到店 | `paid/confirmed` | 409 → 40900（only paid/confirmed） | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L1360-L1364) |
| KDS/商户 | 开始制作 | 订单 `paid` | 400 → 40000（order is not in paid status） | [locallife/api/kitchen.go](locallife/api/kitchen.go#L292-L307) |
| KDS/商户 | 出餐完成 | 订单 `preparing` 或 `paid` | 400 → 40000（order is not in preparing or paid status） | [locallife/api/kitchen.go](locallife/api/kitchen.go#L368-L383) |

**通用错误码（权限/资源）**
- 403 → 40300：非商户/非本人/无权操作（订单/预订/配送）
- 404 → 40400：资源不存在（订单/预订/配送）

**取消进入售后通道（已确认）**
- 当订单进入 `preparing/ready/courier_accepted/picked/delivering/rider_delivered`，用户取消会被拒绝并记录异常状态 `cancel_requested`（售后通道）。
  - [locallife/api/order.go](locallife/api/order.go#L2046-L2076)

### 2.5 支付/退款关键规则（已确认）
| 场景 | 动作 | 允许状态/条件 | 禁止时返回（HTTP→业务码） | 证据 |
| --- | --- | --- | --- | --- |
| 用户 | 创建支付单 | 订单必须 `pending` 且归属用户 | 400 → 40000（order is not in pending status） | [locallife/api/payment_order.go](locallife/api/payment_order.go#L246-L262) |
| 用户 | 关闭支付单 | 支付单必须 `pending` 且归属用户 | 400 → 40000（only pending payment orders can be closed） | [locallife/api/payment_order.go](locallife/api/payment_order.go#L608-L616) |
| 商户 | 发起退款 | 支付单必须 `paid` 且订单归属商户 | 400 → 40000（payment order is not paid / amount exceeds） | [locallife/api/payment_order.go](locallife/api/payment_order.go#L736-L766) |

### 2.6 触发点/责任方摘要（待补全）
| 场景 | 动作 | 触发方 | 触发点 | 证据 |
| --- | --- | --- | --- | --- |
| 堂食/外卖 | 商户接单/拒单/出餐/完成 | 商户 | 后台操作（订单列表/详情） | [locallife/api/order.go](locallife/api/order.go#L2840-L3236) |
| 外卖 | 骑手抢单/取餐/配送/送达 | 骑手 | 配送端操作（配送单状态流转） | [locallife/api/delivery.go](locallife/api/delivery.go#L552-L1168) |
| 用户 | 取消/催单/确认收货 | 用户 | 订单详情/追踪页 | [locallife/api/order.go](locallife/api/order.go#L1992-L2555) |
| 预订 | 确认/取消/完成/签到/起菜/未到店 | 商户/用户 | 预订详情页 | [locallife/api/table_reservation.go](locallife/api/table_reservation.go#L990-L2210) |
| 支付 | 创建/关闭支付单 | 用户 | 支付入口/订单详情 | [locallife/api/payment_order.go](locallife/api/payment_order.go#L194-L660) |

---

## 3. region_id 强约束策略（待补全）

### 3.1 数据层证据（已确认）
- 商户表含 `region_id`
  - [locallife/db/query/merchant.sql](locallife/db/query/merchant.sql#L25-L60)
- 运营商表含 `region_id`
  - [locallife/db/query/operator.sql](locallife/db/query/operator.sql#L1-L40)
- 运营商多区域迁移（迁移记录）
  - [locallife/db/migration](locallife/db/migration)

### 3.2 API 强制隔离与审计（待补全）
- 已存在的区域隔离证据（运营商侧）：
  - 统一中间件校验 operator 管辖区域：
    - [locallife/api/casbin_enforcer.go](locallife/api/casbin_enforcer.go#L472-L525)
  - handler 内部校验与取默认区域：
    - [locallife/api/delivery_fee.go](locallife/api/delivery_fee.go#L15-L92)
    - 运营商统计接口基于 region_id 权限：
      - [locallife/api/operator_stats.go](locallife/api/operator_stats.go#L40-L78)
      - [locallife/api/operator_stats.go](locallife/api/operator_stats.go#L132-L166)
    - 运营商申诉审核按 region_id 校验：
      - [locallife/api/appeal.go](locallife/api/appeal.go#L1216-L1242)
  - 路由层强制校验（示例：运费配置按 region_id）：
    - [locallife/api/server.go](locallife/api/server.go#L566-L610)
  - 区域关系表与校验 SQL：
    - [locallife/db/query/operator_region.sql](locallife/db/query/operator_region.sql#L1-L60)
- 数据层 region 约束（已存在）：
  - 商户 region 必填：
    - [locallife/db/migration/000034_merchant_region_required.up.sql](locallife/db/migration/000034_merchant_region_required.up.sql#L1-L42)
  - 骑手 region 字段与索引：
    - [locallife/db/migration/000032_add_rider_region_id.up.sql](locallife/db/migration/000032_add_rider_region_id.up.sql#L1-L18)
  - 用户地址绑定 region：
    - [locallife/db/migration/000003_add_regions_and_addresses.up.sql](locallife/db/migration/000003_add_regions_and_addresses.up.sql#L12-L40)
  - 运营商多区域关系表：
    - [locallife/db/migration/000033_operator_multi_region.up.sql](locallife/db/migration/000033_operator_multi_region.up.sql#L1-L36)
  - 运费/峰时/天气按 region 绑定：
    - [locallife/db/migration/000007_add_delivery_fee.up.sql](locallife/db/migration/000007_add_delivery_fee.up.sql#L1-L90)
  - 骑手列表/统计按 region_id 过滤：
    - [locallife/db/query/rider.sql](locallife/db/query/rider.sql#L140-L170)
- 地理位置映射 region（API 侧兜底逻辑）：
  - [locallife/api/location.go](locallife/api/location.go#L186-L239)
- 待补全：
  - 商户侧：主要依赖 `MerchantStaffMiddleware` 绑定商户（以 merchant_id 为天然隔离），未见统一 region 中间件：
    - [locallife/api/rbac_middleware.go](locallife/api/rbac_middleware.go#L140-L205)
  - 骑手侧：已有显式区域校验（推荐单与抢单均要求 rider.region_id 与 merchant.region_id 一致）：
    - [locallife/api/delivery.go](locallife/api/delivery.go#L72-L140)
    - [locallife/api/delivery.go](locallife/api/delivery.go#L488-L540)
    - 推荐单按 region 过滤且要求已分配区域：
      - [locallife/api/delivery.go](locallife/api/delivery.go#L101-L141)
    - 配送池 SQL 按 region_id 过滤：
      - [locallife/db/query/delivery_pool.sql](locallife/db/query/delivery_pool.sql#L66-L83)
  - 顾客侧：搜索接口仅提供 region_id 可选过滤（非强制），未见统一 region 中间件：
    - [locallife/api/search.go](locallife/api/search.go#L20-L52)
    - 搜索请求参数显式为 optional：
      - [locallife/api/search.go](locallife/api/search.go#L21-L38)
  - 审计字段与日志落地位置（是否记录请求 region_id、操作者、被访问资源）。
    - 已发现的审计日志仅覆盖集团管理：
      - [locallife/api/group.go](locallife/api/group.go#L238-L242)
      - [locallife/db/query/group.sql](locallife/db/query/group.sql#L183-L189)
    - 申诉列表/详情 SQL 以 region_id 过滤：
      - [locallife/db/query/appeal.sql](locallife/db/query/appeal.sql#L260-L302)

### 3.3 审计日志规范（待补全）
> 目标：为涉及跨区域、资金、风控、审核类操作提供统一审计记录。

**建议的最小审计字段**
- actor_user_id（操作者）
- actor_role（角色：admin/operator/merchant_owner/merchant_staff/rider/customer）
- action（动作枚举）
- target_type / target_id（被操作资源）
- region_id（资源所属区域，便于隔离与追溯）
- request_id / trace_id（链路追踪）
- ip / user_agent
- metadata（JSON，包含请求参数与关键业务字段）
- created_at

**优先落地点（第一批）**
- 运营商：商户/骑手暂停与恢复、运费/峰时配置变更、申诉审核、规则修改
- 平台：商户/骑手审核、平台统计导出、全局规则与费率变更
- 商户：订单关键状态流转（accept/reject/ready/complete）、退款/索赔处理、菜品/价格/库存变更

**落地方式（建议）**
- 新增通用审计表（或复用现有 merchant_group_audit_logs 扩展为通用表）。
- 提供中间件/拦截器：自动写入 request_id、actor、region_id 与操作入口。

**已存在审计日志（范围受限）**
- 仅覆盖集团相关操作：
  - 写入入口：`createGroupAuditLog`
    - [locallife/api/group.go](locallife/api/group.go#L216-L227)
  - 审计表：`merchant_group_audit_logs`
    - [locallife/db/query/group.sql](locallife/db/query/group.sql#L183-L189)

### 3.4 region_id 强约束统一策略（建议草案）
> 目标：跨商户/运营商/骑手/用户数据访问在 API 与 SQL 层形成“默认隔离”。

**API 层统一策略**
- 运营商：所有 `/v1/operator/*` 路由统一挂载 `LoadOperatorMiddleware` + 区域校验（region_id path 或 operator_regions 绑定）。
- 商户：统一使用 `MerchantStaffMiddleware` 绑定 `merchant_id`，并在涉及 region 的功能（配送、运费、申诉）显式校验 `merchant.region_id`。
- 骑手：所有 `/v1/delivery/*` 路由使用 `RiderMiddleware`，并强制校验 `rider.region_id` 与订单/商户 region 一致。
- 顾客：面向全局搜索/推荐接口，建议强制要求 `region_id` 或基于地理位置推导 region（不允许缺省跨区）。

**SQL 层统一策略**
- 关键查询统一增加 `region_id` 条件（或通过 merchant_id -> region_id 间接约束）。
- 对跨区域查询设置只读白名单（如平台统计）。

**SQL 侧 region_id 可选过滤的证据（风险）**
- 菜品搜索：region_id 允许为空
  - [locallife/db/query/dish.sql](locallife/db/query/dish.sql#L190-L202)
- 套餐搜索：region_id 允许为空
  - [locallife/db/query/combo.sql](locallife/db/query/combo.sql#L339-L344)
- 包间查询：region_id 允许为空
  - [locallife/db/query/table.sql](locallife/db/query/table.sql#L258-L262)
- 商户搜索：region_id 允许为空
  - [locallife/db/query/merchant.sql](locallife/db/query/merchant.sql#L166-L175)

**缺口清单（待逐项核对）**
- 顾客搜索与推荐接口目前 `region_id` 仅可选，存在跨区默认聚合风险：
  - [locallife/api/search.go](locallife/api/search.go#L20-L52)
- 商户侧接口多以 `merchant_id` 隔离，但未见统一 region 中间件，需梳理涉及配送/规则/财务的跨区风险。
- 审计日志尚未覆盖区域敏感操作，需配合 3.3 补齐。

### 3.5 region_id 落地任务（待执行）
- 顾客搜索/推荐：强制 region_id（或基于定位推导），禁止无 region 默认返回。
- SQL 查询：将可选 region_id 改为必填（或在入口处兜底注入 region_id）。
- 商户侧：补充 region 级校验（配送/财务/申诉/运费）并统一中间件。
- 审计：关键操作写入 `region_id` 并落地到通用审计表。

### 3.6 region_id 强约束执行步骤（建议）
**阶段 A：入口强制注入**
- 顾客侧：搜索/推荐类接口必须携带 `region_id`；若缺失则调用定位推导并写入请求上下文。
- 商户侧：`MerchantStaffMiddleware` 后追加 `region_id` 读取与一致性校验（`merchant.region_id`）。
- 运营商侧：统一从 `operator_regions` 计算可访问 region 列表。

**阶段 B：SQL 层收口**
- 将 `dish/combo/table/merchant` 的可选 region 查询改为必填（或在 handler 层补齐）。
- 对跨区域统计接口建立白名单，并记录审计。

**阶段 C：审计与报警**
- 新增通用审计表（或扩展现有审计表）。
- 每次跨区域访问记录 `actor/region_id/target`；出现越权返回 403 并告警。

**验收标准**
- 顾客搜索/推荐无 `region_id` 时不会返回任何跨区数据。
- 运营商/商户/骑手接口的 region 校验覆盖率 100%。
- 审计表可回溯跨区访问与拒绝记录。

### 3.7 region_id 强约束改造清单（文件级）
**顾客侧（强制 region）**
- 搜索入口： [locallife/api/search.go](locallife/api/search.go)
- 位置兜底： [locallife/api/location.go](locallife/api/location.go)

**SQL 侧（可选过滤改为必选）**
- 菜品搜索： [locallife/db/query/dish.sql](locallife/db/query/dish.sql)
- 套餐搜索： [locallife/db/query/combo.sql](locallife/db/query/combo.sql)
- 桌台搜索： [locallife/db/query/table.sql](locallife/db/query/table.sql)
- 商户搜索： [locallife/db/query/merchant.sql](locallife/db/query/merchant.sql)

**运营商/骑手侧（已存在校验，需覆盖率检查）**
- 运营商区域校验： [locallife/api/casbin_enforcer.go](locallife/api/casbin_enforcer.go)
- 运营商区域统计： [locallife/api/operator_stats.go](locallife/api/operator_stats.go)
- 申诉按区域： [locallife/api/appeal.go](locallife/api/appeal.go)
- 骑手区域校验： [locallife/api/delivery.go](locallife/api/delivery.go)

---

## 4. API/表/页面映射清单（待补全）

### 4.1 小程序页面
- 页面总清单：
  - [weapp/miniprogram/app.json](weapp/miniprogram/app.json)

### 4.2 商户 Web 页面
- 页面目录：
  - [web/src/app/merchant](web/src/app/merchant)

### 4.3 后端 API 与表结构
- API 目录：
  - [locallife/api](locallife/api)
- SQL 查询目录：
  - [locallife/db/query](locallife/db/query)
- 迁移目录：
  - [locallife/db/migration](locallife/db/migration)

### 4.4 核心交易链路映射（先行补全）

#### 4.4.1 小程序 API → 后端 API → 表结构

**订单（创建/查询/取消/确认）**
- 小程序：
  - [weapp/miniprogram/api/order.ts](weapp/miniprogram/api/order.ts)
- 后端：
  - 订单路由与动作： [locallife/api/order.go](locallife/api/order.go)
- 主要表：
  - [locallife/db/query/order.sql](locallife/db/query/order.sql)
  - [locallife/db/query/order_item.sql](locallife/db/query/order_item.sql)
  - [locallife/db/query/order_status_log.sql](locallife/db/query/order_status_log.sql)

**支付/退款（创建支付、关闭、退款）**
- 小程序：
  - [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts)
- 后端：
  - 支付/退款 API： [locallife/api/payment_order.go](locallife/api/payment_order.go)
  - 支付回调： [locallife/api/payment_callback.go](locallife/api/payment_callback.go)
- 主要表：
  - [locallife/db/query/payment_order.sql](locallife/db/query/payment_order.sql)
  - [locallife/db/query/refund_order.sql](locallife/db/query/refund_order.sql)
  - [locallife/db/query/wechat_notification.sql](locallife/db/query/wechat_notification.sql)

**配送（骑手接单/取餐/送达、配送查询）**
- 小程序：
  - [weapp/miniprogram/api/delivery.ts](weapp/miniprogram/api/delivery.ts)
- 后端：
  - 配送 API： [locallife/api/delivery.go](locallife/api/delivery.go)
- 主要表：
  - [locallife/db/query/delivery.sql](locallife/db/query/delivery.sql)
  - [locallife/db/query/delivery_pool.sql](locallife/db/query/delivery_pool.sql)
  - [locallife/db/query/delivery_location_event.sql](locallife/db/query/delivery_location_event.sql)

**预订（创建/取消/确认/签到/起菜）**
- 小程序：
  - [weapp/miniprogram/api/reservation.ts](weapp/miniprogram/api/reservation.ts)
- 后端：
  - 预订 API： [locallife/api/table_reservation.go](locallife/api/table_reservation.go)
- 主要表：
  - [locallife/db/query/table_reservation.sql](locallife/db/query/table_reservation.sql)
  - [locallife/db/query/reservation_item.sql](locallife/db/query/reservation_item.sql)
  - [locallife/db/query/reservation_payment.sql](locallife/db/query/reservation_payment.sql)
  - [locallife/db/query/reservation_inventory.sql](locallife/db/query/reservation_inventory.sql)

**用餐会话（堂食扫码/开台/转台）**
- 小程序：
  - [weapp/miniprogram/api/dining-session.ts](weapp/miniprogram/api/dining-session.ts)
- 后端：
  - 会话 API： [locallife/api/dining_session.go](locallife/api/dining_session.go)
- 主要表：
  - [locallife/db/query/dining_session.sql](locallife/db/query/dining_session.sql)
  - [locallife/db/query/billing_group.sql](locallife/db/query/billing_group.sql)

#### 4.4.2 页面 → API（核心路径）

**小程序（顾客侧）核心页面 → API 模块**
- 外卖首页：weapp/miniprogram/pages/takeout/index.ts
  - [weapp/miniprogram/pages/takeout/index.ts](weapp/miniprogram/pages/takeout/index.ts)
  - 关联 API：
    - [weapp/miniprogram/api/dish.ts](weapp/miniprogram/api/dish.ts)
    - [weapp/miniprogram/api/cart.ts](weapp/miniprogram/api/cart.ts)
    - [weapp/miniprogram/api/merchant.ts](weapp/miniprogram/api/merchant.ts)
    - [weapp/miniprogram/api/combo.ts](weapp/miniprogram/api/combo.ts)

- 外卖购物车：weapp/miniprogram/pages/takeout/cart/index.ts
  - [weapp/miniprogram/pages/takeout/cart/index.ts](weapp/miniprogram/pages/takeout/cart/index.ts)
  - 关联 API：
    - [weapp/miniprogram/api/cart.ts](weapp/miniprogram/api/cart.ts)
    - [weapp/miniprogram/api/dish.ts](weapp/miniprogram/api/dish.ts)

- 外卖下单确认：weapp/miniprogram/pages/takeout/order-confirm/index.ts
  - [weapp/miniprogram/pages/takeout/order-confirm/index.ts](weapp/miniprogram/pages/takeout/order-confirm/index.ts)
  - 关联 API：
    - [weapp/miniprogram/api/cart.ts](weapp/miniprogram/api/cart.ts)
    - [weapp/miniprogram/api/address.ts](weapp/miniprogram/api/address.ts)
    - [weapp/miniprogram/api/order.ts](weapp/miniprogram/api/order.ts)
    - [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts)
    - [weapp/miniprogram/api/merchant.ts](weapp/miniprogram/api/merchant.ts)
    - [weapp/miniprogram/api/personal.ts](weapp/miniprogram/api/personal.ts)

- 堂食扫码入口：weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts
  - [weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts](weapp/miniprogram/pages/dine-in/scan-entry/scan-entry.ts)
  - 关联 API：
    - [weapp/miniprogram/api/table.ts](weapp/miniprogram/api/table.ts)
    - [weapp/miniprogram/api/dining-session.ts](weapp/miniprogram/api/dining-session.ts)

- 堂食点餐页：weapp/miniprogram/pages/dine-in/menu/menu.ts
  - [weapp/miniprogram/pages/dine-in/menu/menu.ts](weapp/miniprogram/pages/dine-in/menu/menu.ts)
  - 关联 API：
    - [weapp/miniprogram/api/table.ts](weapp/miniprogram/api/table.ts)
    - [weapp/miniprogram/api/cart.ts](weapp/miniprogram/api/cart.ts)
    - [weapp/miniprogram/api/reservation.ts](weapp/miniprogram/api/reservation.ts)
    - [weapp/miniprogram/api/merchant.ts](weapp/miniprogram/api/merchant.ts)
    - [weapp/miniprogram/api/dish.ts](weapp/miniprogram/api/dish.ts)

- 堂食结算页：weapp/miniprogram/pages/dine-in/checkout/checkout.ts
  - [weapp/miniprogram/pages/dine-in/checkout/checkout.ts](weapp/miniprogram/pages/dine-in/checkout/checkout.ts)
  - 关联 API：
    - [weapp/miniprogram/api/cart.ts](weapp/miniprogram/api/cart.ts)
    - [weapp/miniprogram/api/merchant.ts](weapp/miniprogram/api/merchant.ts)
    - [weapp/miniprogram/api/table.ts](weapp/miniprogram/api/table.ts)
    - [weapp/miniprogram/api/reservation.ts](weapp/miniprogram/api/reservation.ts)
    - [weapp/miniprogram/api/order.ts](weapp/miniprogram/api/order.ts)
    - [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts)
    - [weapp/miniprogram/api/personal.ts](weapp/miniprogram/api/personal.ts)

- 订单列表/详情/追踪：
  - [weapp/miniprogram/pages/orders/list/index.ts](weapp/miniprogram/pages/orders/list/index.ts)
  - [weapp/miniprogram/pages/orders/detail/index.ts](weapp/miniprogram/pages/orders/detail/index.ts)
  - [weapp/miniprogram/pages/orders/tracking/index.ts](weapp/miniprogram/pages/orders/tracking/index.ts)
  - 关联 API：
    - [weapp/miniprogram/api/order.ts](weapp/miniprogram/api/order.ts)
    - [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts)
    - [weapp/miniprogram/api/reservation.ts](weapp/miniprogram/api/reservation.ts)
    - [weapp/miniprogram/api/delivery.ts](weapp/miniprogram/api/delivery.ts)
    - [weapp/miniprogram/api/location.ts](weapp/miniprogram/api/location.ts)

- 预订入口/确认/详情/修改/列表：
  - [weapp/miniprogram/pages/reservation/index.ts](weapp/miniprogram/pages/reservation/index.ts)
  - [weapp/miniprogram/pages/reservation/confirm/index.ts](weapp/miniprogram/pages/reservation/confirm/index.ts)
  - [weapp/miniprogram/pages/reservation/detail/index.ts](weapp/miniprogram/pages/reservation/detail/index.ts)
  - [weapp/miniprogram/pages/reservation/modify/index.ts](weapp/miniprogram/pages/reservation/modify/index.ts)
  - [weapp/miniprogram/pages/reservation/list/index.ts](weapp/miniprogram/pages/reservation/list/index.ts)
  - 关联 API：
    - [weapp/miniprogram/api/reservation.ts](weapp/miniprogram/api/reservation.ts)
    - [weapp/miniprogram/api/room.ts](weapp/miniprogram/api/room.ts)
    - [weapp/miniprogram/api/payment.ts](weapp/miniprogram/api/payment.ts)
    - [weapp/miniprogram/api/cart.ts](weapp/miniprogram/api/cart.ts)
    - [weapp/miniprogram/api/merchant.ts](weapp/miniprogram/api/merchant.ts)

---

> 下一步：补齐商户 Web 页面 → API 的精确映射（按模块分批）。

**商户 Web（核心页面） → API 入口**
- 总体 API 访问封装：
  - [web/src/lib/api.ts](web/src/lib/api.ts)

- 仪表盘：/merchant/dashboard
  - 页面：
    - [web/src/app/merchant/dashboard/page.tsx](web/src/app/merchant/dashboard/page.tsx)
    - [web/src/components/merchant/dashboard-page-client.tsx](web/src/components/merchant/dashboard-page-client.tsx)
  - 主要 API：
    - `/merchants/me`, `/merchants/me/status`, `/merchant/stats/overview`
    - `/merchant/orders`, `/tables`, `/reservations/merchant/today`
    - `/dining-sessions/open`

- 数据分析：/merchant/analytics
  - 页面：
    - [web/src/app/merchant/analytics/page.tsx](web/src/app/merchant/analytics/page.tsx)
    - [web/src/components/merchant/analytics-page-client.tsx](web/src/components/merchant/analytics-page-client.tsx)
  - 主要 API：
    - `/merchant/stats/overview`, `/merchant/stats/daily`, `/merchant/stats/hourly`
    - `/merchant/stats/dishes/top`, `/merchant/stats/sources`, `/merchant/stats/repurchase`
    - `/merchant/stats/categories`, `/merchant/stats/customers`
    - `/merchant/finance/overview`

- 订单列表：/merchant/orders
  - 页面：
    - [web/src/app/merchant/orders/page.tsx](web/src/app/merchant/orders/page.tsx)
    - [web/src/components/merchant/orders-page-client.tsx](web/src/components/merchant/orders-page-client.tsx)
  - 主要 API：
    - `/merchant/orders/stats`, `/merchant/orders`, `/merchant/stats/overview`
    - `/merchant/orders/:id`
    - `/merchant/orders/:id/{accept|reject|ready|complete}`

- 订单详情：/merchant/orders/[id]
  - 页面：
    - [web/src/app/merchant/orders/[id]/page.tsx](web/src/app/merchant/orders/[id]/page.tsx)
    - [web/src/components/merchant/order-actions.tsx](web/src/components/merchant/order-actions.tsx)
  - 主要 API：
    - `/merchant/orders/:id`
    - `/merchant/orders/:id/{accept|reject|ready|complete}`

- 预订管理：/merchant/reservations
  - 页面：
    - [web/src/app/merchant/reservations/page.tsx](web/src/app/merchant/reservations/page.tsx)
    - [web/src/components/merchant/reservations-page-client.tsx](web/src/components/merchant/reservations-page-client.tsx)
  - 主要 API：
    - `/reservations/merchant`, `/reservations/merchant/stats`, `/reservations/merchant/today`
    - `/reservations/merchant/create`, `/reservations/:id/update`, `/reservations/:id/cancel`
    - `/dining-sessions/open`

- 预订详情：/merchant/reservations/[id]
  - 页面：
    - [web/src/app/merchant/reservations/[id]/page.tsx](web/src/app/merchant/reservations/[id]/page.tsx)
  - 主要 API：
    - `/reservations/:id`, `/tables`

- 员工管理：/merchant/staff
  - 页面：
    - [web/src/app/merchant/staff/page.tsx](web/src/app/merchant/staff/page.tsx)
    - [web/src/components/merchant/staff-page-client.tsx](web/src/components/merchant/staff-page-client.tsx)
  - 主要 API：
    - `/merchant/staff`
    - `/merchant/staff/invite-code`
    - `/merchant/staff/:id/role`
    - `/merchant/staff/:id` (DELETE)

- 财务中心：/merchant/finance
  - 页面：
    - [web/src/app/merchant/finance/page.tsx](web/src/app/merchant/finance/page.tsx)
    - [web/src/components/merchant/finance-page-client.tsx](web/src/components/merchant/finance-page-client.tsx)
  - 主要 API：
    - `/merchant/finance/overview`
    - `/merchant/finance/daily`
    - `/merchant/finance/orders`
    - `/merchant/finance/service-fees`
    - `/merchant/finance/promotions`
    - `/merchant/finance/settlements`

- 菜品管理：/merchant/dishes
  - 页面：
    - [web/src/app/merchant/dishes/page.tsx](web/src/app/merchant/dishes/page.tsx)
    - [web/src/components/merchant/dishes-page-client.tsx](web/src/components/merchant/dishes-page-client.tsx)
  - 主要 API：
    - `/dishes`, `/dishes/:id`, `/dishes/:id/customizations`
    - `/dishes/images/upload`
    - `/dishes/categories`
    - `/dishes/batch/status`
    - `/tags?type=dish`, `/tags?type=customization`

- 库存管理：/merchant/inventory
  - 页面：
    - [web/src/app/merchant/inventory/page.tsx](web/src/app/merchant/inventory/page.tsx)
    - [web/src/components/merchant/inventory-page-client.tsx](web/src/components/merchant/inventory-page-client.tsx)
  - 主要 API：
    - `/inventory` (POST/PUT)

- 桌台管理：/merchant/tables
  - 页面：
    - [web/src/app/merchant/tables/page.tsx](web/src/app/merchant/tables/page.tsx)
    - [web/src/components/merchant/tables-page-client.tsx](web/src/components/merchant/tables-page-client.tsx)
  - 主要 API：
    - `/tables`, `/tables/:id`
    - `/tables/:id/images`, `/tables/:id/images/:image_id`, `/tables/:id/images/:image_id/primary`
    - `/tables/images/upload`
    - `/tags?type=table`, `/tags/:id`

- 评价管理：/merchant/reviews
  - 页面：
    - [web/src/app/merchant/reviews/page.tsx](web/src/app/merchant/reviews/page.tsx)
    - [web/src/components/merchant/reviews-page-client.tsx](web/src/components/merchant/reviews-page-client.tsx)
  - 主要 API：
    - `/reviews/:id/reply`

- 套餐管理：/merchant/combos
  - 页面：
    - [web/src/app/merchant/combos/page.tsx](web/src/app/merchant/combos/page.tsx)
    - [web/src/components/merchant/combos-page-client.tsx](web/src/components/merchant/combos-page-client.tsx)
  - 主要 API：
    - `/combos`, `/combos/:id`

- 营销折扣：/merchant/marketing/discounts
  - 页面：
    - [web/src/components/merchant/discounts-page-client.tsx](web/src/components/merchant/discounts-page-client.tsx)
  - 主要 API：
    - `/merchants/:id/discounts`, `/merchants/:id/discounts/:id`

- 优惠券：/merchant/marketing/vouchers
  - 页面：
    - [web/src/components/merchant/vouchers-page-client.tsx](web/src/components/merchant/vouchers-page-client.tsx)
  - 主要 API：
    - `/merchants/:id/vouchers`, `/merchants/:id/vouchers/:id`

- 会员管理：/merchant/members
  - 页面：
    - [web/src/app/merchant/members/page.tsx](web/src/app/merchant/members/page.tsx)
    - [web/src/components/merchant/members-page-client.tsx](web/src/components/merchant/members-page-client.tsx)
  - 主要 API：
    - `/merchants/:id/members/:user_id/balance`
    - `/merchants/:id/recharge-rules`, `/merchants/:id/recharge-rules/:id`
    - `/merchants/me/membership-settings`

- 配送促销：/merchant/marketing/delivery
  - 页面：
    - [web/src/components/merchant/delivery-page-client.tsx](web/src/components/merchant/delivery-page-client.tsx)
  - 主要 API：
    - `/delivery-fee/merchants/:id/promotions`, `/delivery-fee/merchants/:id/promotions/:id`

- 设置/设备：/merchant/settings
  - 页面：
    - [web/src/app/merchant/settings/page.tsx](web/src/app/merchant/settings/page.tsx)
    - [web/src/components/merchant/settings-page-client.tsx](web/src/components/merchant/settings-page-client.tsx)
  - 主要 API：
    - `/merchants/me/status`
    - `/merchant/display-config`
    - `/merchant/devices`, `/merchant/devices/:id`

- 集团管理：/merchant/group
  - 页面：
    - [web/src/app/merchant/group/page.tsx](web/src/app/merchant/group/page.tsx)
    - [web/src/components/merchant/group-page-client.tsx](web/src/components/merchant/group-page-client.tsx)
  - 主要 API：
    - `/groups/:id/join-requests/:id/{approve|reject}`
    - `/groups/:id/policies`
    - `/groups/:id/join-requests`

- 堂食会话：/merchant/dinein
  - 页面：
    - [web/src/components/merchant/dinein-page-client.tsx](web/src/components/merchant/dinein-page-client.tsx)
  - 主要 API：
    - `/tables`, `/merchant/orders`, `/reservations/merchant/today`
    - `/dining-sessions/open`, `/dining-sessions/:id/transfer-table`

- 厨房/KDS：/merchant/kds
  - 页面：
    - [web/src/app/merchant/kds/page.tsx](web/src/app/merchant/kds/page.tsx)
    - [web/src/components/merchant/kds-page-client.tsx](web/src/components/merchant/kds-page-client.tsx)
    - [web/src/components/merchant/kitchen-actions.tsx](web/src/components/merchant/kitchen-actions.tsx)
  - 主要 API：
    - `/kitchen/orders`
    - `/kitchen/orders/:id/{preparing|ready}`

- 桌台管理：/merchant/tables
  - 页面：
    - [web/src/components/merchant/tables-page-client.tsx](web/src/components/merchant/tables-page-client.tsx)
    - [web/src/app/merchant/tables/[id]/page.tsx](web/src/app/merchant/tables/[id]/page.tsx)
  - 主要 API：
    - `/tables`, `/tables/:id`, `/tables/:id/images`, `/tables/:id/images/:image_id/primary`
    - `/tables/:id/images/:image_id`, `/tables/:id/tags`, `/tags?type=table`

- 菜品管理：/merchant/dishes
  - 页面：
    - [web/src/components/merchant/dishes-page-client.tsx](web/src/components/merchant/dishes-page-client.tsx)
  - 主要 API：
    - `/dishes`, `/dishes/:id`, `/dishes/categories`, `/dishes/:id/customizations`
    - `/tags?type=dish`, `/tags?type=customization`

- 套餐管理：/merchant/combos
  - 页面：
    - [web/src/components/merchant/combos-page-client.tsx](web/src/components/merchant/combos-page-client.tsx)
  - 主要 API：
    - `/combos`, `/combos/:id`, `/combos/:id/dishes`, `/dishes`, `/tags?type=dish`

- 库存管理：/merchant/inventory
  - 页面：
    - [web/src/components/merchant/inventory-page-client.tsx](web/src/components/merchant/inventory-page-client.tsx)
  - 主要 API：
    - `/inventory`, `/inventory/stats`, `/dishes`, `/dishes/categories`

- 配送设置：/merchant/marketing/delivery
  - 页面：
    - [web/src/components/merchant/delivery-page-client.tsx](web/src/components/merchant/delivery-page-client.tsx)
  - 主要 API：
    - `/delivery-fee/merchants/:id/promotions`

- 折扣/优惠券：/merchant/marketing/discounts, /merchant/marketing/vouchers
  - 页面：
    - [web/src/components/merchant/discounts-page-client.tsx](web/src/components/merchant/discounts-page-client.tsx)
    - [web/src/components/merchant/vouchers-page-client.tsx](web/src/components/merchant/vouchers-page-client.tsx)
  - 主要 API：
    - `/merchants/:id/discounts`
    - `/merchants/:id/vouchers`

- 会员管理：/merchant/members
  - 页面：
    - [web/src/components/merchant/members-page-client.tsx](web/src/components/merchant/members-page-client.tsx)
  - 主要 API：
    - `/merchants/:id/members`, `/merchants/:id/members/:user_id/balance`
    - `/merchants/:id/recharge-rules`, `/merchants/me/membership-settings`

- 评价管理：/merchant/reviews
  - 页面：
    - [web/src/components/merchant/reviews-page-client.tsx](web/src/components/merchant/reviews-page-client.tsx)
  - 主要 API：
    - `/reviews/merchants/:id/all`, `/reviews/:id/reply`

- 商户设置：/merchant/settings
  - 页面：
    - [web/src/components/merchant/settings-page-client.tsx](web/src/components/merchant/settings-page-client.tsx)
  - 主要 API：
    - `/merchants/me`, `/merchants/me/business-hours`
    - `/merchant/devices`, `/merchant/display-config`
    - `/groups`, `/groups/:id`, `/groups/:id/join-requests`, `/groups/:id/policies`

- 数据分析：/merchant/analytics
  - 页面：
    - [web/src/components/merchant/analytics-page-client.tsx](web/src/components/merchant/analytics-page-client.tsx)
  - 主要 API：
    - `/merchant/stats/overview`, `/merchant/stats/daily`, `/merchant/stats/dishes/top`
    - `/merchant/stats/hourly`, `/merchant/stats/sources`, `/merchant/stats/repurchase`
    - `/merchant/stats/categories`, `/merchant/stats/customers`
    - `/merchant/finance/overview`

- 财务概览：/merchant/finance
  - 页面：
    - [web/src/components/merchant/finance-page-client.tsx](web/src/components/merchant/finance-page-client.tsx)
  - 主要 API：
    - `/merchant/finance/overview`, `/merchant/finance/daily`
    - `/merchant/finance/orders`, `/merchant/finance/service-fees`
    - `/merchant/finance/promotions`, `/merchant/finance/settlements`

---

### 4.4.3 商户 Web 页面 → API → 表结构（关键模块，已补全）

**商户统计（/merchant/analytics）**
- API：
  - `/merchant/stats/overview`, `/merchant/stats/daily`, `/merchant/stats/dishes/top`
  - `/merchant/stats/hourly`, `/merchant/stats/sources`, `/merchant/stats/repurchase`
  - `/merchant/stats/categories`, `/merchant/stats/customers`
  - 入口实现： [locallife/api/merchant_stats.go](locallife/api/merchant_stats.go)
- 主要表：
  - 订单聚合： [locallife/db/query/merchant_stats.sql](locallife/db/query/merchant_stats.sql)
  - 订单明细/状态： [locallife/db/query/order.sql](locallife/db/query/order.sql)
  - 订单项： [locallife/db/query/order_item.sql](locallife/db/query/order_item.sql)
  - 菜品： [locallife/db/query/dish.sql](locallife/db/query/dish.sql)
  - 用户： [locallife/db/query/user.sql](locallife/db/query/user.sql)

**商户财务（/merchant/finance）**
- API：
  - `/merchant/finance/overview`, `/merchant/finance/orders`, `/merchant/finance/service-fees`
  - `/merchant/finance/promotions`, `/merchant/finance/daily`, `/merchant/finance/settlements`
  - 入口实现： [locallife/api/merchant_finance.go](locallife/api/merchant_finance.go)
- 主要表：
  - 分账订单： [locallife/db/query/profit_sharing_order.sql](locallife/db/query/profit_sharing_order.sql)
  - 支付订单： [locallife/db/query/payment_order.sql](locallife/db/query/payment_order.sql)
  - 满返支出（运费优惠）： [locallife/db/query/order.sql](locallife/db/query/order.sql)

**预订管理（/merchant/reservations）**
- API：
  - `/reservations/merchant`, `/reservations/merchant/stats`, `/reservations/merchant/today`
  - `/reservations/merchant/create`, `/reservations/:id/update`, `/reservations/:id/cancel`
  - 入口实现： [locallife/api/table_reservation.go](locallife/api/table_reservation.go)
- 主要表：
  - 预订： [locallife/db/query/table_reservation.sql](locallife/db/query/table_reservation.sql)
  - 桌台： [locallife/db/query/table.sql](locallife/db/query/table.sql)
  - 预订菜品： [locallife/db/query/reservation_item.sql](locallife/db/query/reservation_item.sql)
  - 预订支付： [locallife/db/query/reservation_payment.sql](locallife/db/query/reservation_payment.sql)

**堂食会话（/merchant/dinein）**
- API：
  - `/dining-sessions/open`, `/dining-sessions/:id/transfer-table`
  - 入口实现： [locallife/api/dining_session.go](locallife/api/dining_session.go)
- 主要表：
  - 会话： [locallife/db/query/dining_session.sql](locallife/db/query/dining_session.sql)
  - 桌台： [locallife/db/query/table.sql](locallife/db/query/table.sql)

**厨房/KDS（/merchant/kds）**
- API：
  - `/kitchen/orders`, `/kitchen/orders/:id/{preparing|ready}`
  - 入口实现： [locallife/api/kitchen.go](locallife/api/kitchen.go)
- 主要表：
  - 订单（KDS 视图）： [locallife/db/query/order.sql](locallife/db/query/order.sql)
  - 订单状态日志： [locallife/db/query/order_status_log.sql](locallife/db/query/order_status_log.sql)

**桌台管理（/merchant/tables）**
- API：
  - `/tables`, `/tables/:id`, `/tables/:id/images`, `/tables/:id/tags`
  - 入口实现： [locallife/api/table.go](locallife/api/table.go)
- 主要表：
  - 桌台与图片/标签： [locallife/db/query/table.sql](locallife/db/query/table.sql)
  - 标签： [locallife/db/query/tag.sql](locallife/db/query/tag.sql)

**菜品管理（/merchant/dishes）**
- API：
  - `/dishes`, `/dishes/:id`, `/dishes/categories`, `/dishes/:id/customizations`
  - 入口实现： [locallife/api/dish.go](locallife/api/dish.go)
- 主要表：
  - 菜品/分类/配料/标签/自定义： [locallife/db/query/dish.sql](locallife/db/query/dish.sql)
  - 标签： [locallife/db/query/tag.sql](locallife/db/query/tag.sql)

**套餐管理（/merchant/combos）**
- API：
  - `/combos`, `/combos/:id`, `/combos/:id/dishes`
  - 入口实现： [locallife/api/combo.go](locallife/api/combo.go)
- 主要表：
  - 套餐： [locallife/db/query/combo.sql](locallife/db/query/combo.sql)
  - 菜品： [locallife/db/query/dish.sql](locallife/db/query/dish.sql)

**库存管理（/merchant/inventory）**
- API：
  - `/inventory`, `/inventory/stats`
  - 入口实现： [locallife/api/inventory.go](locallife/api/inventory.go)
- 主要表：
  - 日库存： [locallife/db/query/inventory.sql](locallife/db/query/inventory.sql)
  - 菜品： [locallife/db/query/dish.sql](locallife/db/query/dish.sql)

**营销-满减/代金券（/merchant/marketing）**
- API：
  - `/merchants/:id/discounts`, `/merchants/:id/vouchers`
  - 入口实现： [locallife/api/discount.go](locallife/api/discount.go), [locallife/api/voucher.go](locallife/api/voucher.go)
- 主要表：
  - 满减规则： [locallife/db/query/discount.sql](locallife/db/query/discount.sql)
  - 代金券模板/用户券： [locallife/db/query/voucher.sql](locallife/db/query/voucher.sql)

**评价管理（/merchant/reviews）**
- API：
  - `/reviews/merchants/:id/all`, `/reviews/:id/reply`
  - 入口实现： [locallife/api/review.go](locallife/api/review.go)
- 主要表：
  - 评价： [locallife/db/query/review.sql](locallife/db/query/review.sql)

### 4.4.4 运营商/平台（小程序）页面 → API → 表结构（已补全）

> 说明：Web 端目前仅有商户后台（[web/src/app/merchant](web/src/app/merchant)），运营商/平台侧暂为小程序承载。

**运营商工作台（/pages/operator/dashboard）**
- 页面：
  - [weapp/miniprogram/pages/operator/dashboard/dashboard.ts](weapp/miniprogram/pages/operator/dashboard/dashboard.ts)
- 主要 API：
  - `/v1/operator/regions`, `/v1/operator/regions/:region_id/stats`
  - `/v1/operator/stats/realtime`, `/v1/operator/merchants/ranking`, `/v1/operator/riders/ranking`, `/v1/operator/trend/daily`
  - `/v1/operators/me/finance/overview`, `/v1/operators/me/commission`
  - 入口实现：
    - [locallife/api/operator_stats.go](locallife/api/operator_stats.go)
    - [locallife/api/operator_finance.go](locallife/api/operator_finance.go)
  - 小程序 API：
    - [weapp/miniprogram/api/operator-basic-management.ts](weapp/miniprogram/api/operator-basic-management.ts)
    - [weapp/miniprogram/api/operator-analytics.ts](weapp/miniprogram/api/operator-analytics.ts)
    - [weapp/miniprogram/api/operator-merchant-management.ts](weapp/miniprogram/api/operator-merchant-management.ts)
    - [weapp/miniprogram/api/operator-rider-management.ts](weapp/miniprogram/api/operator-rider-management.ts)
- 主要表：
  - 运营商与区域： [locallife/db/query/operator.sql](locallife/db/query/operator.sql), [locallife/db/query/operator_region.sql](locallife/db/query/operator_region.sql), [locallife/db/query/region.sql](locallife/db/query/region.sql)
  - 分账订单统计： [locallife/db/query/operator_stats.sql](locallife/db/query/operator_stats.sql), [locallife/db/query/profit_sharing_order.sql](locallife/db/query/profit_sharing_order.sql)

**运营商商户管理（/pages/operator/merchants）**
- 小程序 API： [weapp/miniprogram/api/operator-merchant-management.ts](weapp/miniprogram/api/operator-merchant-management.ts)
- 后端 API：
  - `/v1/operator/merchants`, `/v1/operator/merchants/:id`, `/v1/operator/merchants/:id/{suspend|resume}`
  - 入口实现： [locallife/api/operator_merchant_rider.go](locallife/api/operator_merchant_rider.go)
- 主要表：
  - 商户： [locallife/db/query/merchant.sql](locallife/db/query/merchant.sql)
  - 区域关系： [locallife/db/query/operator_region.sql](locallife/db/query/operator_region.sql)

**运营商骑手管理（/pages/operator/riders）**
- 小程序 API： [weapp/miniprogram/api/operator-rider-management.ts](weapp/miniprogram/api/operator-rider-management.ts)
- 后端 API：
  - `/v1/operator/riders`, `/v1/operator/riders/:id`, `/v1/operator/riders/:id/{suspend|resume}`
  - 入口实现： [locallife/api/operator_merchant_rider.go](locallife/api/operator_merchant_rider.go)
- 主要表：
  - 骑手： [locallife/db/query/rider.sql](locallife/db/query/rider.sql)
  - 配送： [locallife/db/query/delivery.sql](locallife/db/query/delivery.sql)

**运营商客诉/申诉处理（/pages/operator/appeal）**
- 小程序 API： [weapp/miniprogram/api/appeals-customer-service.ts](weapp/miniprogram/api/appeals-customer-service.ts)
- 后端 API：
  - `/v1/operator/appeals`, `/v1/operator/appeals/:id`, `/v1/operator/appeals/:id/review`
  - 入口实现： [locallife/api/appeal.go](locallife/api/appeal.go)
- 主要表：
  - 申诉/索赔： [locallife/db/query/appeal.sql](locallife/db/query/appeal.sql), [locallife/db/query/trust_score.sql](locallife/db/query/trust_score.sql)

**运营商运费与高峰时段（/pages/operator/delivery-fee, /pages/operator/region）**
- 小程序 API： [weapp/miniprogram/api/delivery-fee.ts](weapp/miniprogram/api/delivery-fee.ts)
- 后端 API：
  - `/v1/delivery-fee/regions/:region_id/config`
  - `/v1/operator/regions/:region_id/peak-hours`, `/v1/operator/peak-hours/:id`
  - 入口实现： [locallife/api/delivery_fee.go](locallife/api/delivery_fee.go)
- 主要表：
  - 运费配置/峰时配置： [locallife/db/query/delivery_fee_config.sql](locallife/db/query/delivery_fee_config.sql)

**运营商提现（/pages/operator/finance/withdraw）**
- 小程序 API： [weapp/miniprogram/api/operator-finance.ts](weapp/miniprogram/api/operator-finance.ts)
- 后端 API：
  - `/v1/operators/me/finance/withdraw`
  - 入口实现： [locallife/api/operator_finance.go](locallife/api/operator_finance.go)
- 主要表：
  - 运营商： [locallife/db/query/operator.sql](locallife/db/query/operator.sql)

**平台大屏（/pages/platform/dashboard）**
- 页面：
  - [weapp/miniprogram/pages/platform/dashboard/dashboard.ts](weapp/miniprogram/pages/platform/dashboard/dashboard.ts)
- 小程序 API：
  - [weapp/miniprogram/api/platform-dashboard.ts](weapp/miniprogram/api/platform-dashboard.ts)
  - [weapp/miniprogram/api/platform-management.ts](weapp/miniprogram/api/platform-management.ts)
- 后端 API：
  - `/v1/platform/stats/{overview|daily|regions/compare|merchants/ranking|categories|growth/users|growth/merchants|riders/ranking|hourly|realtime}`
  - 入口实现： [locallife/api/platform_stats.go](locallife/api/platform_stats.go)
- 主要表：
  - 平台统计： [locallife/db/query/platform_stats.sql](locallife/db/query/platform_stats.sql)
  - 订单/配送/用户/商户： [locallife/db/query/order.sql](locallife/db/query/order.sql), [locallife/db/query/delivery.sql](locallife/db/query/delivery.sql), [locallife/db/query/user.sql](locallife/db/query/user.sql), [locallife/db/query/merchant.sql](locallife/db/query/merchant.sql)

> 下一步：补齐“region_id 强约束策略”的全域覆盖情况，并整理审计日志规范。

---

## 5. 上线验收 checklist（待补全）
### 5.1 监控/告警
- [ ] API 延迟/错误率（核心链路：下单/支付/配送/预订）
- [ ] 支付回调与分账失败告警（支付单/分账单状态异常）
- [ ] 配送异常（超时、无人接单、异常高取消率）
- [ ] 数据库/队列/缓存基础设施监控（连接数、慢查询、积压）
- [ ] 订单状态异常迁移告警（跳跃/逆序/重复）
- [ ] KDS 超时告警（预设 SLA）
- [ ] 退款超时告警（退款发起后超过 T+X 未完成）

### 5.2 压测/容量
- [ ] 核心接口压测基线：/orders, /payment, /delivery, /reservations
- [ ] 峰值容量估算（GMV、订单、配送单峰值）
- [ ] 关键慢查询优化清单与索引确认
- [ ] 关键缓存命中率基线（商品/商户/配置类）
- [ ] 支付回调与异步任务队列容量验证

### 5.3 合规/隐私
- [ ] PII 脱敏与日志规范（手机号、身份证、地址）
- [ ] 数据保留策略与删除/匿名化流程
- [ ] 文件上传安全与内容安全检测覆盖（商户/骑手/用户）
- [ ] 操作日志与用户授权合规（隐私政策/服务协议）

### 5.4 灾备/回滚
- [ ] 数据库备份与恢复演练（RPO/RTO）
- [ ] 支付/分账失败补偿流程（重试/补单）
- [ ] 发布回滚流程与开关策略
- [ ] 关键配置变更灰度与回滚验证

### 5.5 审计/追溯
- [ ] 关键操作审计落地（商户/运营商/平台）
- [ ] 订单全链路追踪（状态日志 + 事件日志）
- [ ] region_id 访问审计与越权监测

### 5.6 安全/风控
- [ ] 接口鉴权与权限校验覆盖率（角色/资源级别）
- [ ] 风控策略开关可配置（外卖拒单、异常退款）
- [ ] 防刷/限流基线（IP/用户/设备维度）

### 5.7 业务验收（最小闭环）
- [ ] 外卖闭环：下单→支付→商户接单→出餐→骑手配送→确认收货
- [ ] 堂食闭环：扫码→点餐→支付→出餐→完成
- [ ] 预订闭环：创建→支付→确认→签到/起菜→完成/取消
- [ ] 售后闭环：取消/退款/申诉→审核→结果落地

---

## 6. 已确认的规则驱动能力（补充）
- 索赔自动裁决与行为回溯：
  - [locallife/algorithm/claim_auto_approval.go](locallife/algorithm/claim_auto_approval.go)
- 团伙欺诈检测：
  - [locallife/algorithm/fraud_detector.go](locallife/algorithm/fraud_detector.go)
- 运营商规则读取：
  - [locallife/api/operator_rules.go](locallife/api/operator_rules.go)
- 分账数据模型：
  - [locallife/db/query/profit_sharing_order.sql](locallife/db/query/profit_sharing_order.sql)

---

## 7. Phase 0 可执行任务拆解（建议）
> 目标：将“错误码统一 + region_id 强约束”拆为可交付任务，并在上线前完成。

### 7.1 错误码统一（P0）
| 任务 | 负责人建议 | 产出 | 验收 |
| --- | --- | --- | --- |
| 修复非统一响应（location/middleware/order） | 后端 | 全量返回 `{code,message,data}` | 代码扫描无裸返回 | 
| Web `code` 判定与 envelope 头 | Web | `api.ts` 统一判定 | 线上错误码分布可追踪 |
| 小程序 `ErrorCode` 迁移 | 小程序 | 业务码枚举 + UI 分支 | 409/422/429 有明确文案 |
| 错误上报字段统一 | 前端 | 带 `code/trace_id` | 监控可聚合按 code |

### 7.2 region_id 强约束（P0）
| 任务 | 负责人建议 | 产出 | 验收 |
| --- | --- | --- | --- |
| 搜索/推荐强制 region | 后端 | search 强制注入 | 无 region 不返回 | 
| SQL 可选 region 收口 | 后端/DB | dish/combo/table/merchant 改必选 | 越权查询为 0 |
| 商户侧 region 校验 | 后端 | 中间件或 handler 校验 | 测试覆盖关键接口 |
| 审计落地 | 后端 | 通用审计表 + 写入 | 可回溯跨区访问 |

### 7.3 上线验收（P1）
| 任务 | 负责人建议 | 产出 | 验收 |
| --- | --- | --- | --- |
| 监控告警基线 | 运维/后端 | 核心链路告警 | 触发演练通过 |
| 压测与容量评估 | 后端 | 压测报告 | 峰值指标达标 |
| 合规与隐私检查 | 产品/法务/后端 | 合规清单 | 审计记录齐全 |
