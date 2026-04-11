# Android Keep-Alive Guide (国产厂商保活适配)

> 作用：定义 LocalLife 商户端在国产 Android 厂商系统上的后台保活策略、前台服务配置，以及地推装机时的权限引导要求。

## 1. 前台服务 (Foreground Service)

### 1.1 配置要求

| 配置项 | 值 | 说明 |
|---|---|---|
| 通知渠道 ID | `merchant_fg_service` | Android 8.0+ 必须 |
| 渠道重要性 | LOW | 不打扰用户但持续显示 |
| 通知标题 | "乐客来福商户端" | |
| 通知内容 | "正在运行 · 等待新订单..." | |
| 不可划掉 | ✅ `isSticky: true` | |
| 开机自启 | ✅ `autoRunOnBoot: true` | |
| Wake Lock | ✅ | 防止 CPU 休眠 |
| WiFi Lock | ✅ | 保持 WiFi 连接 |
| 心跳间隔 | 15 秒 | 在前台服务回调中检查 WebSocket 连接 |

### 1.2 AndroidManifest.xml 权限

```xml
<uses-permission android:name="android.permission.FOREGROUND_SERVICE" />
<uses-permission android:name="android.permission.FOREGROUND_SERVICE_DATA_SYNC" />
<uses-permission android:name="android.permission.WAKE_LOCK" />
<uses-permission android:name="android.permission.RECEIVE_BOOT_COMPLETED" />
<uses-permission android:name="android.permission.REQUEST_IGNORE_BATTERY_OPTIMIZATIONS" />
<uses-permission android:name="android.permission.SYSTEM_ALERT_WINDOW" />
<uses-permission android:name="android.permission.USE_FULL_SCREEN_INTENT" />
<uses-permission android:name="android.permission.VIBRATE" />
<uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />
<uses-permission android:name="android.permission.ACCESS_WIFI_STATE" />
<uses-permission android:name="android.permission.INTERNET" />
```

## 2. 厂商保活适配

### 2.1 华为 / 荣耀 (EMUI 10+ / HarmonyOS)

| 步骤 | 位置 |
|---|---|
| 关闭自动管理 | 设置 → 应用和服务 → 应用启动管理 → 乐客来福 → 关闭"自动管理" |
| 开启自启动 | 上一步中手动开启：自启动 ✅、关联启动 ✅、后台活动 ✅ |
| 保持网络连接 | 设置 → 电池 → 更多电池设置 → 休眠时始终保持网络连接 ✅ |
| 忽略电池优化 | 设置 → 电池 → 乐客来福 → 不允许 |

> ⚠️ HarmonyOS NEXT（纯鸿蒙）不支持 APK。如发现商户使用 HarmonyOS NEXT 设备，需记录并上报，暂无法支持。

### 2.2 小米 / Redmi (MIUI 12+ / HyperOS)

| 步骤 | 位置 |
|---|---|
| 开启自启动 | 设置 → 应用设置 → 应用管理 → 乐客来福 → 自启动 ✅ |
| 无限制模式 | 设置 → 省电与电池 → 乐客来福 → 无限制 ✅ |
| 锁定后台 | 最近任务 → 长按乐客来福卡片 → 锁定 🔒 |

### 2.3 OPPO / Realme (ColorOS 11+)

| 步骤 | 位置 |
|---|---|
| 允许后台运行 | 设置 → 应用管理 → 乐客来福 → 耗电保护 → 允许后台运行 ✅ |
| 开启自启动 | 设置 → 电池 → 自启动管理 → 乐客来福 ✅ |
| 允许悬浮通知 | 设置 → 通知与状态栏 → 通知管理 → 乐客来福 → 允许悬浮通知 ✅ |

### 2.4 vivo (OriginOS / Funtouch)

| 步骤 | 位置 |
|---|---|
| 允许高耗电 | 设置 → 电池 → 后台耗电管理 → 乐客来福 → 允许后台高耗电 ✅ |
| 开启自启动 | i管家 → 应用管理 → 权限管理 → 自启动 → 乐客来福 ✅ |

## 3. App 内权限检测与引导

### 3.1 自动检测

App 应在启动时自动检测以下权限状态：

```dart
class PermissionChecker {
  /// 检测电池优化是否已忽略
  Future<bool> isBatteryOptimizationIgnored();

  /// 检测通知权限是否已开启
  Future<bool> isNotificationPermissionGranted();

  /// 检测是否在后台限制列表中
  Future<bool> isBackgroundRestricted();
}
```

### 3.2 引导页面 (PermissionGuidePage)

当检测到关键权限未开启时，展示引导页面：

- 用图文步骤引导用户开启权限
- 按当前手机品牌自动展示对应的设置路径
- 提供"打开系统设置"快捷按钮
- 用户可选择"稍后设置"跳过，但主页需要持续显示警告标记

### 3.3 权限检测频率

- App 启动时检测一次
- 每次从后台恢复前台时检测一次
- 检测结果缓存 5 分钟，避免频繁弹出

## 4. 地推装机检查单

地推人员在安装 App 后必须完成以下操作：

- [ ] APK 安装成功，App 能正常启动
- [ ] 商户账号登录成功
- [ ] 通知栏显示"乐客来福正在运行"前台服务通知
- [ ] 已关闭电池优化（按品牌操作）
- [ ] 已开启自启动权限（按品牌操作）
- [ ] 已锁定后台卡片（小米/OPPO）
- [ ] 发送测试订单，确认：
  - [ ] 语音播报正常
  - [ ] 全屏弹窗弹出
  - [ ] 锁屏状态下也能弹出
- [ ] 打印机已配对（如有）
- [ ] 告知商户不要手动清理后台
- [ ] 留下售后联系方式

## 5. 网络异常处理

| 场景 | 检测方式 | 处理 |
|---|---|---|
| WiFi 断开 | `connectivity_plus` | 立即语音提醒"网络连接异常" |
| WebSocket 断线 | 心跳超时 (15s) | 自动重连，指数退避 (1s, 2s, 4s, 8s, max 30s) |
| 重连失败 >30s | 重试计数器 | 通知栏更新为"⚠️ 连接中断，正在重连..." |
| 重连失败 >5min | 重试计数器 | 语音播报"网络持续异常，请检查WiFi连接" |

## 6. 注意事项

- Android 12+ 对前台服务有新的限制，需要声明 `foregroundServiceType`。
- Android 13+ 需要运行时请求通知权限 (`POST_NOTIFICATIONS`)。
- 国产 ROM 的后台管理策略会随系统更新而变化，需要定期在主流机型上回归测试。
- 部分极端省电模式（如小米的"超级省电"）会强制停止所有后台进程，此时只有厂商推送通道可用。
