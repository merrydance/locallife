# Locallife 项目概览（后端为主）

- 日期：2026-01-16
- 目标读者：新加入的工程师、需要快速理解系统的协作方
- 范围：`locallife/` 后端 Go 服务（含 API / DB / worker / websocket / 外部集成）及与小程序端的边界

> 本文是“介绍性概览”。更细的代码取证与风险清单见：
> - `docs/backend_review_tasks.md`（模块级证据链/结论，作为事实来源）
> - `docs/backend_review_report.md`（跨模块汇总、风险分级与优先级建议）

---

## 1. 这是什么系统

Locallife 是一个本地生活/外卖 + 堂食的综合业务系统，核心参与方包括：
- C 端用户：浏览商户与菜品/套餐、下单、支付、评价
- 商户：入驻/进件、门店与商品管理、接单与后厨出餐
- 骑手：入驻、接单配送、位置上报与履约状态推进
- 运营商/平台：区域运营、审核、统计、部分风控与治理

系统形态为“单体后端服务 + DB + Redis + 异步任务 + WebSocket”，并集成微信支付/进件、地图距离计算等外部能力。

---

## 2. 核心技术栈与关键约定

### 2.1 技术栈
- 语言/框架：Go + Gin
- 数据库访问：sqlc + pgx（查询 SQL 位于 `db/query/*.sql`，生成代码位于 `db/sqlc/*`）
- 异步任务：Asynq（`worker/`）
- 实时通信：WebSocket Hub（`websocket/`），可选 Redis Pub/Sub 做跨进程投递
- 外部集成：微信（支付回调、分账、进件、内容安全）、地图/天气等
- 前端：小程序工程在 `weapp/`

### 2.2 重要约定（读代码时最常见的“系统性假设”）
- 路由聚合入口：`api/server.go`（区分 public 路由、登录态路由组、operator/admin 路由组等）
- Store/事务：业务写链路大多通过 `store.*Tx(...)` 组织（`db/sqlc` 里有多种 Tx 封装）
- 上传与资源访问：以本地 `uploads/` 目录为落盘，配合签名 URL 与“公共目录直出”策略

---

## 3. 目录结构快速地图（从哪里开始看）

- `main.go`：进程启动入口
- `api/`：HTTP API（handler、中间件、路由分组）
  - `api/server.go`：路由装配与依赖初始化（DB/Redis/微信/地图等）
  - `api/middleware*.go`：认证、限流、安全头、Tracing/Prometheus、统一响应封装等
- `db/query/`：sqlc 的 SQL 源文件（事实上的“数据访问契约”）
- `db/sqlc/`：sqlc 生成代码 + 手写 Tx 逻辑（`tx_*.go`）
- `worker/`：异步任务处理（支付成功后处理、通知投递、进件状态处理等）
- `websocket/`：WS Hub、消息回放、队列与 Pub/Sub
- `util/`：上传、本地文件、通用工具等
- `wechat/`：微信相关客户端/封装
- `maps/`、`weather/`：地图/天气相关能力
- `weapp/`：小程序端

---

## 4. 运行时架构（高层视图）

### 4.1 同步链路：HTTP API
典型路径：
1) Gin 路由匹配（`api/server.go`）
2) 中间件链：CORS/安全头/日志/Tracing/指标/限流/超时/认证/（可选授权）
3) Handler：参数校验 → 权限/归属校验 → 调用 store/Tx → 组装响应
4) DB：sqlc 查询/事务落库

### 4.2 异步链路：worker
典型路径：
- webhooks/接口写库后，把“重计算/通知/状态推进”等交给 Asynq worker
- worker 处理完成后可能：
  - 写 DB 状态
  - 通过 WS/PubSub 推送通知

### 4.3 实时链路：WebSocket
- 连接入口在 API 层（`/v1/ws`、`/v1/platform/ws` 等）
- Hub 维护不同实体的连接集合（骑手/商户/平台）
- 支持一定程度的消息回放（基于 `last_sequence`）
- 多实例部署时可选用 Redis Pub/Sub 做跨进程推送

---

## 5. 关键业务链路（读代码时的“主干路径”）

### 5.1 登录/会话
- 以微信登录为主：code → openid → 创建/查找用户 → 生成 token/session
- handler 与测试集中在 token/user/wechat 相关模块

### 5.2 下单/支付/履约
- 订单创建：购物车/价格计算/库存与优惠（见 order/cart/inventory/voucher/discount）
- 支付回调：webhooks 验签/解密 → 更新支付单 → 分发后处理任务
- 配送：配送池、抢单、状态机推进、位置/围栏事件

### 5.3 商户入驻与进件
- 商户申请与微信二级商户进件链路相互耦合：需重点关注状态机与幂等

### 5.4 上传与资源访问
- 上传落盘到 `uploads/`，并通过“公共目录直出 + 私有目录签名 URL”提供访问
- 多处业务会把上传的相对路径写入 DB，响应时再规范化为客户端 URL

### 5.5 通知与运营治理
- 通知支持偏好/免打扰/未读统计；可通过 WS 实时推送
- 运营侧按区域做审核与统计，需要关注权限边界与 PII 最小化

---

## 6. 本地开发与常用入口

- 配置：参考 `app.env.example`，本地可用 `app.env`
- 容器：`Dockerfile` / `docker-compose.yaml`
- 常用命令入口：`Makefile`
- API 契约：`docs/swagger.yaml` / `docs/swagger.json`

> 说明：具体启动命令/依赖准备以仓库 README/Makefile 为准（若后续需要，我可以补一份“快速启动”文档）。

---

## 7. 已知关注点（给维护者的“阅读提示”）

本文不展开审查细节，只列出“理解系统时最常遇到的风险主题”，便于你知道去哪里找证据：
- 错误映射一致性：NotFound/冲突/幂等等场景在不同模块可能存在语义漂移
- 权限体系多来源：路由分组 + RBAC/Casbin + 角色元数据可能出现不一致
- 资金/状态机：支付回调、worker 后处理与幂等门闩是高风险区域
- 上传/图片链路：目录公私策略、路径规范化、外部 URL 拉取与资源限制
- 实时推送：多实例时 Pub/Sub 协议与消息封装需保持一致

---

## 8. 进一步阅读

- 详细审查任务与证据链：`docs/backend_review_tasks.md`
- 总体风险汇总与优先级建议：`docs/backend_review_report.md`
- WebSocket 推送方案（若存在专项方案）：`docs/websocket_push_plan.md`
- 业务专项（外卖/围栏/订单生命周期等）：`docs/takeout_*` 系列文档
