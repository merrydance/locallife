## LocalLife API 契约规范（V1）

发布时间：2026-03-15
适用范围：`/v1/**` 全部后端 HTTP 接口（包括 merchant/operator/platform/admin）

### 1. 路径规范

1. 所有对外 API 路径统一以 `/v1` 作为前缀。
2. 路径命名优先资源化，使用复数名词：`/orders`、`/merchants`、`/tags`。
3. 允许动作型子路径，但仅限以下白名单场景：
	- 状态机推进（如 `:id/approve`、`:id/reject`、`:id/confirm`）
	- 外部系统协议动作（如支付、签约、回调）
	- 无法自然映射到 CRUD 的领域动作（需在注释中说明理由）
4. Swagger 注释的 `@Router` 必须与实际注册路由完全一致（路径、method、前缀）。
5. 以下为既存动作型路径白名单（冻结于 2026-03-15，新增须 PR 审批并注明理由）：

   | 路径 | Method | 说明 |
   |------|--------|------|
   | `/v1/auth/refresh` | POST | token 续期 |
   | `/v1/auth/web-login/confirm` | POST | Web 登录二次确认 |
   | `/v1/bind-merchant` | POST | 账号绑定商户 |
   | `/v1/merchant/application/submit` | POST | 提交商户入驻申请 |
   | `/v1/merchant/application/reset` | POST | 重置商户申请 |
   | `/v1/merchant/bindbank` | POST | 商户绑定银行卡 |
   | `/v1/merchant/orders/{id}/accept` | POST | 商户接单 |
   | `/v1/merchant/orders/{id}/reject` | POST | 商户拒单 |
   | `/v1/merchant/orders/{id}/complete` | POST | 商户标记完成 |
   | `/v1/operator/application/submit` | POST | 提交运营商申请 |
   | `/v1/operator/application/reset` | POST | 重置运营商申请 |
   | `/v1/operator/applyment/bindbank` | POST | 运营商绑定银行卡 |
   | `/v1/operators/me/rules/{id}/disable` | POST | 禁用运营商规则 |
   | `/v1/rider/application/submit` | POST | 提交骑手申请 |
   | `/v1/rider/application/reset` | POST | 重置骑手申请 |
   | `/v1/groups/applications/submit` | POST | 提交团购申请 |
   | `/v1/groups/{id}/join-requests/{request_id}/approve` | POST | 审批加群请求 |
   | `/v1/groups/{id}/join-requests/{request_id}/reject` | POST | 拒绝加群请求 |
   | `/v1/groups/{id}/join-requests/{request_id}/cancel` | POST | 取消加群申请 |
   | `/v1/orders/{id}/confirm` | POST | 用户确认订单 |
   | `/v1/orders/{id}/cancel` | POST | 取消订单 |
   | `/v1/reservations/{id}/confirm` | POST | 确认预订 |
   | `/v1/reservations/{id}/cancel` | POST | 取消预订 |
   | `/v1/reservations/{id}/complete` | POST | 标记预订完成 |
   | `/v1/delivery/:delivery_id/confirm-pickup` | POST | 骑手确认取货 |
   | `/v1/delivery/:delivery_id/confirm-delivery` | POST | 骑手确认送达 |
   | `/v1/dining-sessions/{id}/transfer-table` | POST | 换桌 |
   | `/v1/admin/riders/{rider_id}/approve` | POST | 管理员审批骑手 |
   | `/v1/admin/riders/{rider_id}/reject` | POST | 管理员拒绝骑手 |
   | `/v1/admin/operators/region-applications/{id}/approve` | POST | 审批运营商区域扩张 |
   | `/v1/admin/operators/region-applications/{id}/reject` | POST | 拒绝运营商区域扩张 |
   | `/v1/platform/profit-sharing/configs/{id}/disable` | POST | 禁用分润配置 |

### 2. Method 语义

1. `GET`：只读，无副作用。
2. `POST`：创建资源或执行不可幂等动作。
3. `PUT`：整资源替换，幂等。
4. `PATCH`：局部更新，幂等。
5. `DELETE`：删除资源或关系，幂等。

### 3. 状态码语义

1. `200/201/204`：成功。
2. `400`：参数不合法、业务前置条件不满足（但资源存在）。
3. `401`：未认证或 token 无效。
4. `403`：已认证但无权限。
5. `404`：目标资源不存在（路径资源或明确对象不存在）。
6. `409`：并发冲突、重复提交、状态冲突。
7. `422`：语义校验失败（可选，后续可逐步引入）。
8. `5xx`：服务端错误或下游依赖异常。

### 4. 空态语义（重点）

1. “尚未开通/尚未申请/尚未配置”属于业务状态，不是资源缺失。
2. 此类场景统一返回 `200`，并通过状态字段表达（如 `status`、`account_status`、`status_desc`）。
3. 只有“明确按 id 查询目标对象，且对象不存在”才返回 `404`。

### 5. 响应结构规范

1. 优先使用强类型响应结构体，避免大量 `gin.H` 的临时 key。
2. 列表返回统一包含：
	- `items`（或领域名数组，如 `withdrawals`）
	- `total`
	- `page`
	- `limit`
	- `total_pages`
3. 同一领域接口返回字段命名必须一致（例如 `total` 与 `total_count` 不应长期并存）。

### 6. 错误结构规范

1. 统一 `ErrorResponse` 结构。
2. `message` 保持可读且稳定，避免把内部错误栈直接透出到客户端。
3. 业务可判定错误建议配套稳定 `code`（后续可扩展分层错误码）。

### 7. 变更兼容策略

1. 会影响前端分支逻辑的契约变更，必须先兼容后切换。
2. 若将 `404` 改为 `200 + status`，需先让前端兼容新旧返回，再移除旧逻辑。
3. PR 必须包含：
	- 变更前后契约说明
	- 受影响接口清单
	- 回归测试说明

### 8. 审查清单（PR 必填）

1. `@Router` 与实际路由是否一致
2. method 是否符合语义
3. 状态码是否符合规范
4. 空态是否误用 `404`
5. 响应结构是否稳定且可预测
6. 是否补充或更新测试

