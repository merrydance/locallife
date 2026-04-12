# Merchant App Backend Interface Requirements

本文档面向 `merchant_app/` 对应后端能力排期与接口对接，不只描述“缺什么”，也明确“当前客户端实际依赖什么协议”。

目标是让后端能直接据此拆分任务、补齐契约、确认字段与错误语义，而不是再回头读 Flutter 代码反推需求。

## 1. 客户端范围

- 客户端：Android 商户端 `merchant_app`
- 包名：`com.merrydance.locallife.merchant`
- 当前版本：`1.0.0+1`
- 接口基路径：`/v1`
- 认证方式：`Authorization: Bearer <access_token>`

## 2. 优先级结论

### 2.1 P0 必须具备

以下能力会直接影响商户端主链路可用性：

1. 设备绑定登录
2. Token 刷新
3. Push 设备注册与解绑
4. 商户订单列表
5. 商户接单
6. 商户拒单
7. 实时新订单通知（WS / Push payload 契约）
8. 协议详情查询

### 2.2 P1 建议补齐

以下能力当前前端可降级运行，但影响完整交付质量：

1. 在线升级版本查询接口
2. 订单详情返回中补充拒单原因 / 取消原因 / 退款状态
3. 协议接口补充纯文本内容，避免移动端自己从 HTML 降级

## 3. 接口总表

| 优先级 | 状态 | Method | Path | 能力 |
| --- | --- | --- | --- | --- |
| P0 | 已有 | `POST` | `/v1/auth/app-bind/verify` | 绑定码登录并下发 token |
| P0 | 已有 | `POST` | `/v1/auth/refresh` | 刷新 access token |
| P0 | 缺失 | `POST` | `/v1/push/devices/register` | 上报 JPush registration_id 并绑定商户设备 |
| P1 | 缺失 | `POST` | `/v1/push/devices/unregister` | 设备登出或失效时解绑推送目标 |
| P0 | 已有 | `GET` | `/v1/merchant/orders` | 获取商户订单列表 |
| P0 | 已有 | `POST` | `/v1/merchant/orders/{id}/accept` | 商户接单 |
| P0 | 已有 | `POST` | `/v1/merchant/orders/{id}/reject` | 商户拒单 |
| P0 | 已有 | `GET` | `/v1/agreements/{type}` | 获取协议详情 |
| P0 | 需锁定契约 | `WS / Push` | 新订单通知消息 | 新订单实时提醒 |
| P1 | 缺失 | `GET` | `/v1/app/version/latest` | 检查新版本并返回 APK 下载信息 |

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

### 4.3 商户订单列表

- Method: `GET`
- Path: `/v1/merchant/orders`
- Auth: Bearer

#### 当前前端依赖的订单 DTO

前端当前最少依赖：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `id` | `string | number` | 是 | 订单主键，前端统一转字符串 |
| `order_num` | `string` | 是 | 订单号 |
    "refresh_token": "jwt-refresh-token",
    "user": {
      "id": 1001,
      "full_name": "张店长",
      "workbenches": [
        {
          "id": "merchant",
          "merchant_id": 2001,
          "merchant_name": "示例门店"
        }
      ]
    }
| `status` | `string` | 是 | 见订单状态枚举 |
| `created_at` | `string(datetime)` | 是 | 订单创建时间 |
| `user_name` | `string | null` | 否 | 顾客姓名 |
| `user_phone` | `string | null` | 否 | 顾客电话 |
| `note` | `string | null` | 否 | 顾客备注 |
| `items` | `OrderItem[]` | 是 | 商品列表 |

#### OrderItem DTO
- 返回 `user.workbenches[].merchant_name`，作为商户端首页主标题与当前绑定商户展示来源。


### 4.3 Push 设备注册

JPush key 已配置，不代表端到端已经打通。后端仍需要显式承接设备注册与推送目标绑定。

- Method: `POST`
- Recommended path: `/v1/push/devices/register`
- Auth: Bearer

#### Request body

| 字段 | 类型 | 必填 | 示例 | 说明 |
| --- | --- | --- | --- | --- |
| `provider` | `string` | 是 | `jpush` | 推送厂商标识 |
| `registration_id` | `string` | 是 | `190e35f7e0f...` | JPush 设备注册 ID |
| `device_id` | `string` | 是 | `550e8400-e29b-41d4-a716-446655440000` | 客户端持久化设备 ID |
| `platform` | `string` | 是 | `android` | 平台 |
| `app_version` | `string` | 否 | `1.0.0` | 当前应用版本 |
| `device_model` | `string` | 否 | `Redmi K70` | 设备型号 |

#### Response body

```json
{
  "data": {
    "provider": "jpush",
    "registration_id": "190e35f7e0f...",
    "registered": true,
    "merchant_id": 2001,
    "merchant_name": "示例门店"
  }
}
```

#### 后端能力要求

- 将 `registration_id` 与当前登录商户、设备、平台建立映射。
- 同一设备重复注册时保持幂等。
- 如果商户切换账号，应覆盖旧商户绑定，避免错发。
- 推送侧建议使用 merchant 维度 alias/tag，但服务端仍应保留 registration_id 直连能力，避免只靠控制台配置。

### 4.4 Push 设备解绑

- Method: `POST`
- Recommended path: `/v1/push/devices/unregister`
- Auth: Bearer

#### Request body

| 字段 | 类型 | 必填 | 示例 | 说明 |
| --- | --- | --- | --- | --- |
| `provider` | `string` | 是 | `jpush` | 推送厂商标识 |
| `registration_id` | `string` | 是 | `190e35f7e0f...` | JPush 设备注册 ID |
| `device_id` | `string` | 是 | `550e8400-e29b-41d4-a716-446655440000` | 客户端设备 ID |

#### 后端能力要求

- 登出、设备失效、商户切换时支持解绑。
- 解绑失败不能影响前端退出登录，但服务端应保证最终一致回收。
| 字段 | 类型 | 必填 | 说明 |
### 4.5 商户订单列表
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

#### 推荐响应

前端当前已兼容：

```json
{
  "data": [
    {
      "id": 10001,
      "order_num": "LL202604120001",
      "amount": 48.5,
      "status": "pending",
      "created_at": "2026-04-12T11:23:45Z",
      "user_name": "张三",
      "user_phone": "13800000000",
      "note": "少辣，不要香菜",
      "items": [
        {
          "name": "鱼香肉丝盖饭",
          "quantity": 1,
          "price": 22.0
        }
      ]
    }
  ]
}
```

#### 后端能力要求

- 返回结果必须至少覆盖待接单、进行中、已完成三类订单所需字段。
- 若字段类型不是前端当前约定，需提前锁定变更。
- 建议后续支持筛选参数，例如 `status`、`updated_after`、`page`、`page_size`，便于移动端做增量拉取。

### 4.6 商户接单

- Method: `POST`
- Path: `/v1/merchant/orders/{id}/accept`
- Auth: Bearer

#### Request body

当前前端未传 body，可兼容空 body。

#### Response body

- 返回更新后的订单对象。
- 返回结构应与订单列表中的单个订单 DTO 一致。

#### 后端能力要求

- 只允许 `pending` 订单接单。
- 若订单不属于当前商户，返回 `403`。
- 若并发下已被接单或状态已变化，返回明确业务错误。

### 4.7 商户拒单

- Method: `POST`
- Path: `/v1/merchant/orders/{id}/reject`
- Auth: Bearer

#### Request body

| 字段 | 类型 | 必填 | 示例 | 说明 |
| --- | --- | --- | --- | --- |
| `reason` | `string` | 是 | `菜品已售罄` | 拒单原因 |

#### Request example

```json
{
  "reason": "菜品已售罄，建议改天再下单"
}
```

#### Response body

- 返回更新后的订单对象。
- 当前前端至少要求返回 `status = cancelled`。

#### 后端能力要求

- 只允许 `pending` 订单拒单。
- 拒单后的退款由后端内部编排完成，前端不参与退款工作流。
- 若拒单后还存在退款处理中间态，建议后续新增退款字段，而不是让前端猜测。

#### 建议新增字段

为便于详情页展示，建议后续在订单 DTO 中补充：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `reject_reason` | `string | null` | 商户拒单原因 |
| `refund_status` | `string | null` | 退款状态，例如 `processing` / `succeeded` / `failed` |

### 4.8 协议详情查询

- Method: `GET`
- Path: `/v1/agreements/{type}`
- Auth: Bearer

#### 当前前端使用的 `type`

- `PRIVACY_POLICY`
- `USER_AGREEMENT`

#### Response body

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `type` | `string` | 是 | 协议类型 |
| `title` | `string` | 是 | 协议标题 |
| `version` | `string` | 是 | 协议版本 |
| `published_on` | `string` | 是 | 发布时间 |
| `content` | `string` | 是 | 当前返回 HTML 字符串 |

#### 后端能力要求

- 保证协议类型可长期稳定查询。
- 当前返回 HTML 可用，但推荐后续补：
  - `content_html`
  - `content_text`

### 4.9 在线升级版本查询

当前后端未提供，属于本轮明确新增需求。

- Method: `GET`
- Recommended path: `/v1/app/version/latest`
- Auth: Bearer 或公开访问均可，建议允许带 token 访问。

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

#### 字段要求

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `version_code` | `int` | 是 | 用于比较是否更新 |
| `version_name` | `string` | 是 | 展示版本名 |
| `download_url` | `string` | 是 | APK 下载地址 |
| `changelog` | `string` | 是 | 更新说明 |
| `is_force` | `bool` | 是 | 是否强更 |
| `published_at` | `string(datetime)` | 否 | 发布时间 |
| `file_size_bytes` | `int` | 否 | 文件大小 |
| `sha256` | `string` | 否 | 校验值 |

#### 错误语义要求

- `404`: 当前客户端渠道暂未接入在线升级
- `200 + data = null`: 当前已是最新版本
- `5xx`: 服务异常，前端展示“检查更新失败，请稍后再试”

## 5. 实时新订单通知契约

商户端当前同时消费 Push 与 WebSocket，新订单通知 payload 需要字段一致。

### 5.1 投递要求

- 新订单创建后，后端应同时具备 WebSocket 和 JPush 投递能力。
- 当商户 App 不在线或 WebSocket 断开时，JPush 是关键兜底，不建议只把 JPush 视为“可选优化”。
- JPush 通知与自定义消息至少应有一种可达；如果两种都发，payload 中 `message_id` 必须一致。

### 5.2 Payload DTO

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `message_id` | `string` | 是 | 幂等去重 ID |
| `order_id` | `string | number` | 是 | 订单主键 |
| `title` | `string` | 是 | 通知标题 |
| `content` | `string` | 是 | 通知正文 |
| `amount` | `number | string` | 是 | 订单金额 |
| `shop_name` | `string` | 是 | 门店名称 |

### 5.3 WebSocket message envelope

前端当前兼容以下 envelope：

```json
{
  "type": "order_notification",
  "data": {
    "message_id": "msg_123",
    "order_id": "10001",
    "title": "您有新的订单",
    "content": "请及时接单",
    "amount": 48.5,
    "shop_name": "乐客来福示例门店"
  }
}
```

其中 `type` 当前至少兼容：

- `order_notification`
- `notification`

### 5.4 契约要求

- `message_id` 必须稳定，前端已用它做去重。
- Push 和 WS 同时到达时，后端不必保证只发一路，但 payload 必须可去重。
- `amount` 如果不是数字，至少要是可解析的数字字符串。
- JPush 通知 `extras` 中必须包含上述字段，不能只把业务信息放在文案里。

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

以下不是当前前端必需字段，但补齐后能显著减少移动端猜测：

| 领域 | 字段 | 类型 | 说明 |
| --- | --- | --- | --- |
| 订单 | `reject_reason` | `string | null` | 拒单原因 |
| 订单 | `cancel_reason` | `string | null` | 系统取消 / 商户取消原因 |
| 订单 | `refund_status` | `string | null` | 退款进度 |
| 订单 | `updated_at` | `string(datetime)` | 增量同步基准 |
| 协议 | `content_text` | `string` | 便于移动端阅读展示 |
| 版本 | `min_supported_version_code` | `int` | 可选，用于强更判定扩展 |

## 8. 当前结论

### 8.1 已真实接通

- 绑定码登录
- 订单列表
- 商户接单
- 商户拒单
- 协议详情查询

### 8.2 仍需后端新增

- Push 设备注册接口 `/v1/push/devices/register`
- Push 设备解绑接口 `/v1/push/devices/unregister`
- 在线升级版本查询接口 `/v1/app/version/latest`

### 8.3 仍需后端确认但不阻塞当前 APK

- 新订单通知 payload 是否长期稳定维持当前字段
- 订单 DTO 中是否会补充 `reject_reason` / `refund_status`
