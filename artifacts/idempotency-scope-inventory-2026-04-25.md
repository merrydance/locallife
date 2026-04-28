# 幂等 Scope Inventory

日期：2026-04-25

## 1. 盘点结论

当前系统的幂等语义分为五类：外部契约键、领域资源自然键、请求重放键、异步重复投递、自然去重查询。

本阶段只完成分类和标准沉淀，不实现新的中心幂等表，不改造任何入口路径。

核心结论：

- 支付、退款、分账、补差等外部单号必须继续保留在领域表中，不能迁出到统一中心实体。
- OCR job 和媒体上传 session 属于领域资源自然键或自然去重查询，暂不作为 request-level guard 首批候选。
- 会员代充值是当前最明确的 request-level guard 候选，但本阶段不改造。
- 微信 callback、worker、scheduler 的重复执行属于异步重复投递，应由事实记录、状态机条件更新、任务记录或领域幂等兜底，不应降级成普通 HTTP `Idempotency-Key`。

## 2. 分类矩阵

| 类型 | 判断标准 | 默认落点 | 是否接入 request-level guard |
| --- | --- | --- | --- |
| 外部契约键 | 第三方接口、回调、查询、对账依赖该字段 | 领域表唯一约束 | 否；最多在入口 guard 中记录资源绑定 |
| 领域资源自然键 | key 本身定义一个业务资源或任务 | 领域表唯一约束 | 通常否 |
| 请求重放键 | 客户端或上游重试同一写请求 | request-level guard + 业务表兜底 | 候选 |
| 异步重复投递 | webhook、worker、scheduler 重复触发 | callback/任务/事实记录 + 条件更新 | 否 |
| 自然去重查询 | 无显式 key，但业务条件定义同一未完成资源 | 领域查询 + 唯一/条件索引 | 通常否 |

## 3. 当前 Scope Inventory

| Scope | 当前键/约束 | 类型 | 风险 | 当前落点 | request-level guard 判断 | 依据 |
| --- | --- | --- | --- | --- | --- | --- |
| 单笔支付订单 | `payment_orders.out_trade_no` unique | 外部契约键 | G3 | `payment_orders` | 不迁入；入口可后续记录请求绑定 | 微信下单、查单、关单、回调、对账都依赖 `out_trade_no` |
| 合单主单 | `combined_payment_orders.combine_out_trade_no` unique | 外部契约键 | G3 | `combined_payment_orders` | 不迁入 | 微信合单接口和查询依赖 `combine_out_trade_no` |
| 合单子单 | `combined_payment_sub_orders.out_trade_no` unique | 外部契约键 | G3 | `combined_payment_sub_orders` | 不迁入 | 微信合单子单 `sub_out_trade_no` 语义 |
| 退款单 | `refund_orders.out_refund_no` unique | 外部契约键 | G3 | `refund_orders` | 不迁入；入口可后续记录请求绑定 | 直连/收付通退款创建、查询、回调依赖退款单号 |
| 分账单 | `profit_sharing_orders.out_order_no` unique | 外部契约键 | G3 | `profit_sharing_orders` | 不迁入 | 微信分账请求和查询依赖分账商户单号 |
| 分账回退 | `profit_sharing_returns.out_return_no` unique | 外部契约键 | G3 | `profit_sharing_returns` | 不迁入 | 微信分账回退查询和结果处理依赖回退单号 |
| 商户补差 | `subsidy_orders.out_subsidy_no` unique | 外部契约键 | G3 | `subsidy_orders` | 不迁入 | 补差接口以商户补差单号作为幂等键 |
| 补差退回 | `subsidy_orders.out_return_no` unique | 外部契约键 | G3 | `subsidy_orders` | 不迁入 | 补差退回以退回单号作为外部契约键 |
| 微信支付 callback inbox | `wechat_notifications.id` primary key | 异步重复投递 | G3 | `wechat_notifications` | 不接入 request-level guard | 微信 notification id 是回调投递去重键，不是客户端请求重放键 |
| 商户违规通知 | `latest_notification_id` 等通知字段 | 异步重复投递 | G2/G3 | 违规/通知领域表 | 不接入 request-level guard | 属于外部通知事实去重和状态同步 |
| OCR job | `ocr_jobs.idempotency_key` unique | 领域资源自然键 | G3 | `ocr_jobs` | 暂不接入 | key 由 `media_asset_id + document_type + owner_type + owner_id + side` 定义同一 OCR 任务 |
| 媒体上传 session | partial unique `(user_id, media_category, checksum_sha256) where status='pending'` | 自然去重查询 | G3 | `media_upload_sessions` | 暂不接入 | 同用户同类别同 checksum 的 pending session 直接复用 |
| 会员代充值 | `Idempotency-Key` + `membership_transactions(membership_id, idempotency_key)` partial unique | 请求重放键 | G2/G3 | `membership_transactions` | 候选；本阶段不改造 | header 语义是商户重复提交同一充值请求；当前已校验同 key 不同金额/备注为 409 |
| 骑手押金充值/追偿支付入口 | pending payment/order 复用、`out_trade_no`、业务类型/金额条件 | 混合：外部契约键 + 业务去重 | G3 | `payment_orders` 与 logic 编排 | 入口候选；本阶段不改造 | 创建支付请求和外部支付事实强耦合，不能只靠入口 guard 解决 |
| 退款创建入口 | `out_refund_no`、refund/order 状态 | 混合：外部契约键 + 状态机幂等 | G3 | `refund_orders`、worker/recovery | 入口候选；本阶段不改造 | 请求提交、外部终态、业务结算是不同层次 |
| 赔付/追偿动作 | behavior action、outbox、外部转账单号 | 异步重复投递 + 外部契约键 | G3 | `behavior_actions`、worker/recovery、转账单号 | 入口候选；本阶段不改造 | 必须保留 action/outbox 的状态机幂等和转账单号语义 |
| worker timeout/recovery | task payload id、状态条件、next retry | 异步重复投递 | G2/G3 | worker/scheduler + 领域表状态 | 不接入 request-level guard | 重复执行语义应由任务 claim、状态条件和业务幂等保证 |

## 4. 首批候选与非候选

### 明确非候选

- `out_trade_no`、`combine_out_trade_no`、`out_refund_no`、`out_order_no`、`out_return_no`、`out_subsidy_no` 等外部契约键。
- `ocr_jobs.idempotency_key`。
- `media_upload_sessions` pending session 自然去重。
- `wechat_notifications.id` 或任何 callback notification id。

这些 key 可以被日志、指标、审计、后续事实层或 request-level guard 引用，但不能被中心请求幂等表替代。

### 候选但本阶段不改造

- 会员代充值 `Idempotency-Key`。
- 支付创建入口的客户端重复提交治理。
- 退款创建入口的客户端重复提交治理。
- 赔付/追偿入口的客户端或运营重复提交治理。

这些候选需要在后续实现前重新确认：canonical request fields、同 key 不同请求冲突语义、处理中重放语义、TTL、敏感响应策略和领域表最终唯一约束。

## 5. 阶段 0 输出

阶段 0 已完成的判断：

- 当前系统不适合把所有幂等键统一到一个中心实体。
- 幂等治理应先统一分类和 review gate，而不是先建表。
- `idempotency_records` 只能作为 request-level guard 候选能力，不是支付/退款/回调/worker 事实幂等的替代品。
- 后续实现前，必须先根据本 inventory 对目标 scope 再做一次就地确认。