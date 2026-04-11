# App 绑定码认证方案

> 作用：定义 Flutter 商户端的身份认证机制。因 App 与微信安装在同一设备上，无法使用扫码登录，采用"小程序生成绑定码 + App 输入验证"方案。

## 1. 认证流程

### 1.1 首次绑定

```
微信小程序 (已登录)                      Flutter App (未登录)
      │                                        │
      │ ① 进入"商户工具"→"绑定商户端App"          │
      │                                        │
      │ ② POST /v1/auth/app-bind/code           │
      │    → 返回 6 位数字码 (5 分钟有效)          │
      │                                        │
      │ ③ 屏幕显示: "绑定码: 839471"              │
      │                                        │
      │           ④ 商户切换到 App                │
      │                                        │
      │                          ⑤ 输入 839471   │
      │                                        │
      │           ⑥ POST /v1/auth/app-bind/verify
      │                          → JWT tokens    │
      │                                        │
      │                          ⑦ 登录完成 ✅     │
```

### 1.2 后续使用 (自动续期)

```
App 启动
  → 检查本地 refresh_token
    ├── 有 → POST /v1/auth/refresh → 新 token pair → 正常使用
    │         ├── 成功 → 续期成功
    │         └── 失败 (401) → 跳转绑定码登录页
    └── 没有 → 首次使用 → 跳转绑定码登录页
```

### 1.3 何时需要重新绑定

| 场景 | 需要重新绑定 | 说明 |
|---|---|---|
| 正常使用 | ❌ | App 自动续期 |
| 关机几天 | ❌ | refresh_token 有效期内自动续 |
| App 卸载重装 | ✅ | Token 存在本地，卸载即丢失 |
| 换手机 | ✅ | 新设备无 token |
| 清除 App 数据 | ✅ | 本地存储被清空 |
| 连续 365 天不打开 App | ✅ | refresh_token 过期 |
| 管理员强制下线 | ✅ | Session 被吊销 |

## 2. 后端 API

### 2.1 生成绑定码 (小程序侧调用)

```
POST /v1/auth/app-bind/code
Authorization: Bearer <小程序的 access_token>

Response 200:
{
  "code": "839471",
  "expires_in": 300
}
```

实现要点：

- **调用方必须是商户角色**：校验 user 的 roles 中包含 `merchant`，普通顾客不允许操作。
- **6 位数字码**：使用 `crypto/rand` 生成，非顺序。
- **存储**：Redis，key = `app_bind:{code}`，value = `{user_id, merchant_id}`，TTL = 5 分钟。
- **频率限制**：每用户每分钟最多 3 次。
- **同一用户在有效期内重复调用**：返回同一个码（幂等），不生成新码。

### 2.2 验证绑定码 (App 侧调用)

```
POST /v1/auth/app-bind/verify
(无需 Authorization，公开端点)

Request:
{
  "code": "839471",
  "device_id": "a1b2c3d4-...",
  "device_model": "Xiaomi Redmi Note 12",
  "os_version": "Android 13",
  "app_version": "1.0.0"
}

Response 200:
{
  "session_id": 12345,
  "access_token": "eyJ...",
  "access_token_expires_at": "2026-04-11T11:00:00Z",
  "refresh_token": "eyJ...",
  "refresh_token_expires_at": "2027-04-11T10:00:00Z",
  "user": { ... }
}
```

实现要点：

- **验证码**：从 Redis 读取并校验。
- **一次性**：verify 成功后立即从 Redis 删除。
- **角色二次校验**：确保 user 仍然是 merchant 角色。
- **设备记录**：将 device 信息写入 `merchant_devices` 表。
- **Token 有效期**：
  - access_token: 与现有配置一致（通常 15 分钟 - 1 小时）
  - refresh_token: **365 天**（App 场景专用，长于小程序/Web 的 refresh_token）
- **创建 Session**：复用现有 `store.CreateSession()`，与小程序/Web 的 session 机制完全一致。
- **频率限制**：每 IP 每分钟最多 10 次（防暴力猜码）。

### 2.3 Token 刷新 (复用现有)

```
POST /v1/auth/refresh
(复用现有端点，无需改动)
```

- App 启动时、access_token 过期前主动调用。
- 返回新的 access_token + refresh_token pair。
- refresh_token 旋转续期：每次刷新都产生新的 365 天有效期 refresh_token。

## 3. 安全设计

### 3.1 绑定码安全

| 措施 | 说明 |
|---|---|
| 随机性 | `crypto/rand` 生成，100 万种组合 |
| 时效性 | 5 分钟有效期 |
| 一次性 | 使用即销毁 |
| 角色校验 | 仅 merchant 角色可生成 |
| 频率限制 | 生成 3 次/分钟/用户，验证 10 次/分钟/IP |

### 3.2 Token 存储 (Flutter 端)

- 使用 `flutter_secure_storage` 存储 token（Android Keystore 加密）。
- 不使用 `shared_preferences`（明文存储不安全）。
- App 被卸载时 Android Keystore 条目自动清除。

### 3.3 Session 吊销

- 管理员可通过后台吊销特定 session（`store.RevokeSession()`）。
- 吊销后 refresh 请求返回 401，App 跳转到绑定码登录页。

## 4. Go 后端实现位置

```
locallife/
├── api/
│   └── app_bind.go              # [NEW] 两个端点: generateAppBindCode, verifyAppBindCode
├── db/query/
│   └── (无需新表，用 Redis)
└── (Redis key: "app_bind:{code}")
```

核心代码复用：

- Token 生成: 复用 `server.tokenMaker.CreateToken()`
- Session 创建: 复用 `server.store.CreateSession()`
- 设备记录: 复用 `merchant_devices` 表（推送网关同一张表）
- 用户角色查询: 复用 `server.store.ListUserRoles()`

## 5. Flutter 端实现

```dart
// lib/features/auth/
├── bind_code_page.dart          # 输入绑定码 UI
├── auth_provider.dart           # 认证状态管理
└── auth_service.dart            # API 调用 + Token 存储

// 关键逻辑:
class AuthService {
  // 验证绑定码 → 获取 token → 安全存储
  Future<AuthResult> verifyBindCode(String code);

  // 启动时检查 → 自动刷新 token
  Future<bool> tryAutoLogin();

  // Token 刷新（Dio 拦截器自动调用）
  Future<TokenPair> refreshToken();

  // 登出（清除本地 token）
  Future<void> logout();
}
```

## 6. 小程序端改动

在小程序商户工具页面新增一个入口：

```
商户工具
  ├── ... (现有功能)
  └── 绑定商户端 App   ← [NEW]
       → 调用 POST /v1/auth/app-bind/code
       → 展示 6 位数字码 + 倒计时
       → 提示"请在商户端 App 中输入此绑定码"
```
