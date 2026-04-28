# Merchant App Backend Interface Requirements

本文档面向 `merchant_app/` 对应后端能力排期与接口对接，记录当前 Android 商户端真实依赖的后端契约。

重要技术路线：JPush 已彻底废弃。商户端推送改为直连各手机厂商原生通道，包括华为 HMS、小米 MiPush、OPPO Push、vivo Push、荣耀 Push 等。后端不得再按 JPush registration_id、JPush REST API、JPush third_party_channel 或 `/v1/push/devices/*` 设计新能力。

## 1. 客户端范围

- 客户端：Android 商户端 `merchant_app`
- 包名：`com.merrydance.locallife.merchant`
- 当前版本：`1.0.0+1`
- 接口基路径：`/v1`
- 客户端约定：Flutter 代码通过 `Env.apiBaseUrl` 统一注入 `/v1` 前缀，业务调用只写相对路径，例如 `/merchant/orders`、`/tables`，不得在 feature 代码里硬编码 `/v1/...`
- 认证方式：`Authorization: Bearer <access_token>`

## 2. 优先级结论

### 2.1 P0 必须具备

以下能力直接影响商户端主链路可用性：

1. 设备绑定登录
2. Token 刷新
3. 厂商原生推送设备注册、解绑、心跳
4. 商户订单列表
5. 商户接单
6. 商户拒单
7. 实时新订单通知（WebSocket / 厂商 Push payload 契约）
8. 协议详情查询

### 2.2 P1 建议补齐

以下能力当前前端可降级运行，但影响完整交付质量：

1. 在线升级版本查询接口
2. 订单详情返回中补充拒单原因、取消原因、退款状态
3. 协议接口补充纯文本内容，避免移动端自己从 HTML 降级
4. 桌台管理接口契约纳入商户 App 后续同步范围

## 3. 接口总表

| 优先级 | 状态 | Method | Path | 能力 |
| --- | --- | --- | --- | --- |
| P0 | 已有 | `POST` | `/v1/auth/app-bind/verify` | 绑定码登录并下发 token |
| P0 | 已有 | `POST` | `/v1/auth/refresh` | 刷新 access token |
| P0 | 已实现 | `POST` | `/v1/merchant/device/register` | 注册或更新厂商原生推送 token |
| P0 | 已实现 | `DELETE` | `/v1/merchant/device/{device_id}` | 设备登出或失效时解绑推送目标 |
| P0 | 已实现 | `PUT` | `/v1/merchant/device/heartbeat` | 上报设备在线状态、App 版本和推送 token 状态 |
| P0 | 已有 | `GET` | `/v1/merchant/orders` | 获取商户订单列表 |
| P0 | 已有 | `POST` | `/v1/merchant/orders/{id}/accept` | 商户接单 |
| P0 | 已有 | `POST` | `/v1/merchant/orders/{id}/reject` | 商户拒单 |
| P0 | 已有 | `GET` | `/v1/agreements/{type}` | 获取协议详情 |
| P0 | 契约已锁定 | `WS / Push` | 新订单通知消息 | 新订单实时提醒 |
| P1 | 已实现 | `GET` | `/v1/app/version/latest` | 检查新版本并返回 APK 下载信息 |
| P1 | 已有/需 App 对齐 | `GET` | `/v1/tables` | 商户桌台列表 |
| P1 | 已有/需 App 对齐 | `POST` | `/v1/tables` | 新增桌台 |
| P1 | 已有/需 App 对齐 | `PUT` | `/v1/tables/{id}` | 编辑桌台 |
| P1 | 已有/需 App 对齐 | `PATCH` | `/v1/tables/{id}/status` | 修改桌台状态 |
| P1 | 已有/需 App 对齐 | `DELETE` | `/v1/tables/{id}` | 删除桌台 |

## 4. 请求与响应契约

### 4.1 设备绑定登录

- Method: `POST`
- Path: `/v1/auth/app-bind/verify`
- Auth: 否

#### Request body

| 字段 | 类型 | 必填 | 示例 | 说明 |
| --- | --- | --- | --- | --- |
| `code` | `string` | 是 | `123456` | 小程序端生成的绑定码 |
| `device_id` | `string` | 是 | `550e8400-e29b-41d4-a716-446655440000` | 设备唯一标识，客户端持久化 |
| `device_model` | `string` | 是 | `Redmi K70` | 设备型号 |
| `os_version` | `string` | 是 | `Android 15` | 系统版本 |
| `app_version` | `string` | 是 | `1.0.0` | 当前客户端版本 |

#### Response body

前端当前最少依赖：

```json
{
  "data": {
    "access_token": "jwt-access-token",
    "refresh_token": "jwt-refresh-token"
  }
}
```

建议返回 `merchant_name` 或 `user.workbenches[].merchant_name`，作为商户端首页主标题与当前绑定商户展示来源。

#### 后端能力要求

- 校验绑定码是否可用、是否过期、是否属于正确商户。
- 绑定成功后返回有效 access / refresh token。
- 如设备重复绑定，需要明确定义是否顶替旧设备。

### 4.2 Token 刷新

- Method: `POST`
- Path: `/v1/auth/refresh`
- Auth: 否，使用 refresh token

#### Request body

```json
{
  "refresh_token": "jwt-refresh-token"
}
```

#### Response body

```json
{
  "data": {
    "access_token": "new-access-token",
    "refresh_token": "new-refresh-token"
  }
}
```

### 4.3 厂商原生推送设备注册

- Method: `POST`
- Path: `/v1/merchant/device/register`
- Auth: Bearer

#### Request body

| 字段 | 类型 | 必填 | 示例 | 说明 |
| --- | --- | --- | --- | --- |
| `device_id` | `string` | 是 | `550e8400-e29b-41d4-a716-446655440000` | 客户端持久化设备 ID |
| `push_token` | `string` | 是 | `vendor-token` | 当前厂商通道返回的设备 token |
| `platform` | `string` | 是 | `android` | 平台 |
| `provider` | `string` | 否 | `xiaomi` | 取值 `huawei`、`honor`、`xiaomi`、`oppo`、`vivo`、`unknown`；省略时后端记为 `unknown` |
| `app_version` | `string` | 否 | `1.0.0` | 当前应用版本 |
| `device_model` | `string` | 否 | `Redmi K70` | 设备型号 |
| `os_version` | `string` | 否 | `Android 15` | 系统版本 |

#### Response body

```json
{
  "data": {
    "device_id": "550e8400-e29b-41d4-a716-446655440000",
    "provider": "xiaomi",
    "registered": true,
    "merchant_id": 2001,
    "merchant_name": "示例门店"
  }
}
```

#### 后端能力要求

- 将厂商 `push_token` 与当前登录商户、设备、平台、厂商建立映射。
- 同一设备重复注册时保持幂等。
- Token 变化时覆盖旧 token，避免向失效 token 持续投递。
- 如果商户切换账号，应覆盖旧商户绑定，避免错发。
- 响应体不返回原始 `push_token`。
- 后端推送网关应按 `provider` 选择对应厂商 REST API，而不是走 JPush 聚合通道。

### 4.4 厂商原生推送设备解绑

- Method: `DELETE`
- Path: `/v1/merchant/device/{device_id}`
- Auth: Bearer

#### Response body

```json
{
  "data": {
    "device_id": "550e8400-e29b-41d4-a716-446655440000",
    "unregistered": true
  }
}
```

`unregistered=false` 表示当前登录商户下该设备没有活动绑定或已经解绑；接口仍返回 200，方便前端登出流程幂等收尾。

#### 后端能力要求

- 登出、设备失效、商户切换时支持解绑。
- 解绑失败不能影响前端退出登录，但服务端应保证最终一致回收。
- 按当前登录商户和 `device_id` 失效活动绑定，不接受客户端传入 `merchant_id`。

### 4.5 设备心跳

- Method: `PUT`
- Path: `/v1/merchant/device/heartbeat`
- Auth: Bearer

心跳用于更新 `last_active`、当前 App 版本、设备型号、系统版本和推送 token 状态，方便后端判断商户端是否在线、是否具备推送兜底能力。

#### Request body

| 字段 | 类型 | 必填 | 示例 | 说明 |
| --- | --- | --- | --- | --- |
| `device_id` | `string` | 是 | `550e8400-e29b-41d4-a716-446655440000` | 客户端持久化设备 ID |
| `provider` | `string` | 否 | `vivo` | 省略时保持原 provider 不变 |
| `push_token` | `string` | 否 | `vendor-token` | token 刷新后可随心跳上报；省略时保持原 token 不变 |
| `app_version` | `string` | 否 | `1.0.1` | 当前应用版本 |
| `device_model` | `string` | 否 | `Redmi K70` | 设备型号 |
| `os_version` | `string` | 否 | `Android 15` | 系统版本 |

#### Response body

```json
{
  "data": {
    "device_id": "550e8400-e29b-41d4-a716-446655440000",
    "heartbeat": true
  }
}
```

如果当前登录商户下不存在该 `device_id` 的活动绑定，返回 404；前端应重新注册设备后再继续心跳。

### 4.6 商户订单列表

- Method: `GET`
- Path: `/v1/merchant/orders`
- Auth: Bearer

#### 当前前端依赖的订单 DTO

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `id` | `string 或 number` | 是 | 订单主键，前端统一转字符串 |
| `order_num` | `string` | 是 | 订单号 |
| `amount` | `number` | 是 | 订单金额 |
| `status` | `string` | 是 | 见订单状态枚举 |
| `created_at` | `string(datetime)` | 是 | 订单创建时间 |
| `user_name` | `string 或 null` | 否 | 顾客姓名 |
| `user_phone` | `string 或 null` | 否 | 顾客电话 |
| `note` | `string 或 null` | 否 | 顾客备注 |
| `items` | `OrderItem[]` | 是 | 商品列表 |

#### OrderItem DTO

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `name` | `string` | 是 | 商品名 |
| `quantity` | `int` | 是 | 数量 |
| `price` | `number` | 是 | 单价 |

#### 前端当前支持的订单状态

| 状态值 | 含义 |
| --- | --- |
| `pending` | 待接单 |
| `accepted` | 已接单 |
| `preparing` | 制作中 |
| `delivering` | 配送中 |
| `completed` | 已完成 |
| `cancelled` | 已取消 |

#### 后端能力要求

- 返回结果必须至少覆盖待接单、进行中、已完成三类订单所需字段。
- 建议支持筛选参数，例如 `status`、`updated_after`、`page`、`page_size`，便于移动端做增量拉取。

### 4.7 商户接单

- Method: `POST`
- Path: `/v1/merchant/orders/{id}/accept`
- Auth: Bearer

#### 后端能力要求

- 当前前端未传 body，可兼容空 body。
- 返回更新后的订单对象，结构应与订单列表中的单个订单 DTO 一致。
- 只允许 `pending` 订单接单。
- 若订单不属于当前商户，返回 `403`。
- 若并发下已被接单或状态已变化，返回明确业务错误。

### 4.8 商户拒单

- Method: `POST`
- Path: `/v1/merchant/orders/{id}/reject`
- Auth: Bearer

#### Request body

| 字段 | 类型 | 必填 | 示例 | 说明 |
| --- | --- | --- | --- | --- |
| `reason` | `string` | 是 | `菜品已售罄` | 拒单原因 |

#### 后端能力要求

- 返回更新后的订单对象，当前前端至少要求返回 `status = cancelled`。
- 只允许 `pending` 订单拒单。
- 拒单后的退款由后端内部编排完成，前端不参与退款工作流。

### 4.9 协议详情查询

- Method: `GET`
- Path: `/v1/agreements/{type}`
- Auth: Bearer

当前前端使用的 `type`：

- `PRIVACY_POLICY`
- `USER_AGREEMENT`

建议后续为商户端统一使用 `MERCHANT_AGREEMENT` 或明确商户端协议类型，避免把消费者协议作为商户端协议展示。

### 4.10 在线升级版本查询

当前后端已提供该接口。无可用新版本时返回 200 和 `has_update=false`，不使用 404 表达“无更新”。

- Method: `GET`
- Path: `/v1/app/version/latest`
- Auth: 公开访问；客户端带 token 访问也可正常请求。

#### Query parameters

| 参数 | 类型 | 必填 | 示例 | 说明 |
| --- | --- | --- | --- | --- |
| `platform` | `string` | 是 | `android` | 当前只接 Android |
| `channel` | `string` | 是 | `merchant_app` | 客户端渠道 |
| `package_name` | `string` | 是 | `com.merrydance.locallife.merchant` | 用于校验应用身份 |
| `version_code` | `int` | 是 | `1` | 当前安装 build number |
| `version_name` | `string` | 否 | `1.0.0` | 当前显示版本 |

#### Response body

```json
{
  "data": {
    "has_update": true,
    "version_code": 2,
    "version_name": "1.0.1",
    "download_url": "https://example.com/merchant-app-1.0.1.apk",
    "changelog": "1. 修复蓝牙连接稳定性\n2. 优化订单提醒表现",
    "is_force": false,
    "published_at": "2026-04-12T12:00:00Z",
    "file_size_bytes": 48392120,
    "sha256": "optional checksum"
  }
}
```

无更新时返回：

```json
{
  "data": {
    "has_update": false,
    "version_code": 1,
    "version_name": "1.0.0",
    "download_url": "",
    "changelog": "",
    "is_force": false
  }
}
```

### 4.11 桌台管理

桌台管理是后加能力。后续会逐步把小程序端的商户功能同步到 Android 商户端，因此桌台接口需要被纳入 App 合同，而不再视为小程序专属能力。

客户端调用时必须依赖 `Env.apiBaseUrl`，在 feature 代码里使用 `/tables`、`/tables/{id}` 等相对路径。

#### 当前 App 依赖字段

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `id` | `int` | 是 | 桌台 ID |
| `merchant_id` | `int` | 是 | 商户 ID |
| `table_no` | `string` | 是 | 桌台编号 |
| `table_type` | `string` | 是 | `table` 或 `room` |
| `capacity` | `int` | 是 | 容量 |
| `status` | `string` | 是 | `available`、`occupied`、`disabled` |
| `current_reservation_id` | `int 或 null` | 否 | 当前预订 ID |
| `current_reservation` | `object 或 null` | 否 | 当前预订信息 |
| `primary_image_url` | `string 或 null` | 否 | 主图 |
| `tags` | `array` | 否 | 桌台标签 |

#### 当前 App 使用接口

| Method | Path | 用途 |
| --- | --- | --- |
| `GET` | `/v1/tables` | 获取桌台列表 |
| `POST` | `/v1/tables` | 新增桌台 |
| `PUT` | `/v1/tables/{id}` | 编辑桌台 |
| `PATCH` | `/v1/tables/{id}/status` | 开台、清台、停用等状态变更 |
| `DELETE` | `/v1/tables/{id}` | 删除桌台 |

#### 后端能力要求

- 所有桌台接口必须按当前登录商户做对象级权限校验。
- 状态变更需要明确非法状态迁移语义，例如已占用桌台能否删除、停用。
- WebSocket 如推送桌台状态变化，payload 至少包含 `id` 和 `status`。

## 5. 实时新订单通知契约

商户端当前同时消费 Push 与 WebSocket，新订单通知 payload 需要字段一致。后端已为支付成功后的 WebSocket `new_order` 消息补齐稳定业务字段；厂商 Push 投递网关仍在后续阶段实现，但必须复用同一 payload 契约。

### 5.1 投递要求

- 新订单创建后，后端应同时具备 WebSocket 和厂商原生 Push 投递能力。
- 当商户 App 不在线或 WebSocket 断开时，厂商原生 Push 是关键兜底，不是可选优化。
- 后端已具备原生 Push provider 边界、no-op/test provider、活动设备查询和分发编排；真实厂商 REST 客户端仍需后续接入。
- 同一订单通过多个通道到达时，payload 中 `message_id` 必须一致。
- 后端不得再通过 JPush 聚合通道发商户 App 新订单消息。

### 5.2 Payload DTO

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `message_id` | `string` | 是 | 幂等去重 ID，当前格式为 `merchant_app:new_order:{order_id}` |
| `order_id` | `string 或 number` | 是 | 订单主键 |
| `title` | `string` | 是 | 通知标题 |
| `content` | `string` | 是 | 通知正文 |
| `amount` | `number 或 string` | 是 | 订单金额 |
| `shop_name` | `string` | 是 | 门店名称 |

### 5.3 WebSocket message envelope

WebSocket 新订单消息当前使用以下 envelope：

```json
{
  "id": "merchant_app:new_order:10001",
  "type": "new_order",
  "data": {
    "message_id": "merchant_app:new_order:10001",
    "event": "new_order",
    "order_id": 10001,
    "title": "新订单",
    "content": "您有一笔新订单 ORD10001，请及时处理",
    "amount": 4850,
    "shop_name": "乐客来福示例门店"
  }
}
```

`data` 中仍保留后端现有订单快照字段，例如 `id`、`order_no`、`merchant_id`、`order_type`、`total_amount`、`status`、`created_at` 和 `items`。

### 5.4 厂商 Push payload 要求

- 各厂商通知或透传消息必须携带上述业务字段，不能只把订单信息放在通知文案里。
- `message_id` 必须稳定，前端已用它做去重。
- `amount` 如果不是数字，至少要是可解析的数字字符串。
- 厂商通道差异只能在后端推送网关或 Android 原生接入层消化，不应泄漏到 Flutter 业务层。
- 后端推送网关按设备注册时持久化的 `provider` 选择 provider，不接受通知发送时临时覆盖 provider。
- 分发结果只返回安全分类，不透传厂商原始错误、原始 token 或原始 provider payload。

## 6. 通用错误语义

建议所有商户端接口保持以下最小语义一致：

| HTTP 状态码 | 语义 | 前端处理 |
| --- | --- | --- |
| `400` | 参数错误 / 状态不允许 | 直接提示用户 |
| `401` | token 无效或过期 | 触发重新登录或刷新 token |
| `403` | 资源不属于当前商户 / 无权限 | 直接提示用户，无重试 |
| `404` | 资源不存在 | 提示数据已失效 |
| `409` | 并发冲突 / 状态已变化 | 提示用户刷新订单 |
| `5xx` | 服务端异常 | 提示稍后重试 |

## 7. 后端建议补充字段

| 领域 | 字段 | 类型 | 说明 |
| --- | --- | --- | --- |
| 订单 | `reject_reason` | `string 或 null` | 商户拒单原因 |
| 订单 | `cancel_reason` | `string 或 null` | 系统取消 / 商户取消原因 |
| 订单 | `refund_status` | `string 或 null` | 退款进度 |
| 订单 | `updated_at` | `string(datetime)` | 增量同步基准 |
| 协议 | `content_text` | `string` | 便于移动端阅读展示 |
| 版本 | `min_supported_version_code` | `int` | 可选，用于强更判定扩展 |
| 桌台 | `updated_at` | `string(datetime)` | 支持桌台增量同步 |

## 8. 当前结论

### 8.1 已真实接通

- 绑定码登录
- 订单列表
- 商户接单
- 商户拒单
- 协议详情查询
- 桌台管理后端 Swagger 已存在，App 侧正在接入

### 8.2 仍需后端新增或确认

- 厂商原生推送设备注册、解绑、心跳接口的字段与幂等语义
- 在线升级版本查询接口 `/v1/app/version/latest`
- 新订单通知 payload 是否长期稳定维持当前字段
- 桌台状态 WebSocket payload 是否稳定包含 `id` 和 `status`

### 8.3 明确废弃

- JPush SDK
- JPush registration_id
- `/v1/push/devices/register`
- `/v1/push/devices/unregister`
- 后端 JPush REST API 与 JPush third_party_channel 聚合通道
