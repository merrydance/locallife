<div style="page-break-after: always; text-align: center; padding-top: 160px;">

# 乐客来福商户

## 软件说明书

<br><br><br>

| | |
|:---|:---|
| **软件名称** | 乐客来福商户 |
| **版本号** | V1.0 |
| **著作权人** | 来富网络（宁晋）有限公司 |
| **开发语言** | Dart |
| **运行平台** | Android |
| **编制日期** | 2026年4月 |

</div>

---

## 目 录

1. 软件概述
2. 运行环境
3. 系统架构
4. 功能模块详细说明
   - 4.1 启动与初始化模块
   - 4.2 账号绑定与身份认证模块
   - 4.3 订单实时推送与接收模块
   - 4.4 消息去重模块
   - 4.5 订单管理模块
   - 4.6 订单提醒与告警模块
   - 4.7 语音播报与铃声模块
   - 4.8 小票打印模块
   - 4.9 设备保活管理模块
   - 4.10 系统设置模块
   - 4.11 应用更新模块
5. 数据模型说明
6. 接口说明
7. 安全设计
8. 用户操作说明
9. 错误处理与异常恢复
10. 性能指标

---

## 第一章 软件概述

### 1.1 软件简介

乐客来福商户（以下简称"本软件"）是一款面向乐客来福外卖及到店服务平台入驻商家的移动端管理工具，基于 Flutter 框架开发，运行于 Android 操作系统。

本软件的核心职能是：在商家收银台或后厨场景下，实时接收来自乐客来福平台的新订单通知，驱动音频告警唤醒商家注意，并支持一键确认接单与蓝牙小票打印。软件面向中国大陆 Android 市场。

### 1.2 软件背景

随着本地生活服务平台的快速发展，商家端实时响应订单的能力直接影响用户体验与平台 SLA。传统的被动刷新或人工查单方式已无法满足"分钟级响应"的业务要求。本软件通过 WebSocket 实时推送、厂商通道推送、定时轮询三路并行的架构，配合前台保活服务，在商家设备息屏、应用退入后台乃至进程被系统回收等极端场景下，依然能保证订单到达通知送达率。

### 1.3 适用范围

本软件适用于以下使用场景：

- 乐客来福平台已完成商家入驻并获得绑定码的餐饮、零售、服务类商家
- 运行于 Android 6.0（API 级别 23）及以上版本的智能手机或平板设备
- 国内主流 Android 定制系统，包括华为 EMUI/HarmonyOS、小米 MIUI/HyperOS、OPPO ColorOS、vivo OriginOS 等

### 1.4 主要功能概览

| 功能模块 | 功能描述 |
|---|---|
| 账号绑定与认证 | 通过微信小程序生成的六位绑定码完成首次设备绑定，后续自动维持登录态 |
| 三通道订单推送 | WebSocket 实时通道、厂商原生推送通道、30 秒定时轮询同步运行，确保消息不漏收 |
| 消息去重 | 基于 message_id 的本地去重机制，防止三通道重复消息触发重复提醒 |
| 订单管理 | 订单列表按"待接单 / 进行中 / 已完成"三状态分 Tab 展示，支持查看详情 |
| 订单提醒 | 全屏弹窗唤醒锁屏、系统通知双路告警，确保商家不漏单，提升客户满意度 |
| 语音与铃声 | 预录 MP3 铃声 + TTS 合成播报订单号与金额 |
| 蓝牙打印 | 自动扫描并连接蓝牙热敏打印机，完成接单后自动打印小票 |
| 保活管理 | 前台常驻服务配合主流品牌自启动权限引导，保证后台持续运行 |
| 应用更新 | 启动时在线检测版本，支持强制和非强制 APK 升级 |

---

## 第二章 运行环境

### 2.1 硬件环境

| 项目 | 最低要求 | 推荐配置 |
|---|---|---|
| CPU | 四核 1.6 GHz | 八核 2.0 GHz 以上 |
| 内存（RAM） | 2 GB | 4 GB 以上 |
| 存储空间 | 剩余 200 MB | 剩余 1 GB 以上 |
| 网络 | 支持 Wi-Fi 或移动数据（4G/5G） | Wi-Fi 优先 |
| 蓝牙 | 支持 BLE 4.0（可选，打印功能需要） | BLE 5.0 |

### 2.2 软件运行环境

| 项目 | 要求 |
|---|---|
| 操作系统 | Android 6.0（API 23）及以上 |
| 推荐操作系统 | Android 10（API 29）及以上 |
| 目标 SDK 版本 | Android 14（API 34） |
| 网络协议 | HTTP/HTTPS、WebSocket（WSS） |
| 推送支持 | 华为 HMS、小米 MiPush、OPPO Push、vivo Push 各厂商原生推送 SDK |

### 2.3 开发环境

| 项目 | 版本/说明 |
|---|---|
| 开发框架 | Flutter SDK 3.x |
| 开发语言 | Dart 3.x |
| IDE | Android Studio / VS Code |
| 编译工具 | Gradle 8.x，Android Gradle Plugin 8.x |
| 最小 SDK 版本 | Android 6.0（minSdkVersion 23） |
| 目标 SDK 版本 | Android 14（targetSdkVersion 34） |

### 2.4 依赖的第三方组件库

| 组件库 | 版本 | 用途 |
|---|---|---|
| flutter_riverpod | ^2.x | 响应式状态管理 |
| go_router | ^14.x | 声明式路由导航 |
| dio | ^5.x | HTTP 网络请求，含拦截器 |
| web_socket_channel | ^3.x | WebSocket 长连接通信 |
| 各厂商原生推送 SDK | — | 华为 HMS / 小米 MiPush / OPPO Push / vivo Push |
| flutter_foreground_task | ^8.x | Android 前台 Service 封装 |
| flutter_local_notifications | ^18.x | 本地通知（系统级通知栏） |
| flutter_blue_plus | ^1.x | 蓝牙 BLE 设备扫描与通信 |
| flutter_tts | ^4.x | 文字转语音（TTS）播报 |
| audioplayers | ^6.x | 预录音频文件播放 |
| flutter_secure_storage | ^9.x | Android Keystore 加密存储 |
| sqflite | ^2.x | 本地 SQLite 数据库（消息去重） |
| connectivity_plus | ^6.x | 实时网络状态监测 |
| device_info_plus | ^10.x | 获取设备型号与系统版本 |
| package_info_plus | ^8.x | 读取应用版本信息 |
| permission_handler | ^11.x | 运行时权限申请 |
| install_plugin | ^2.x | 应用内安装 APK 升级包 |
| r_upgrade | ^0.x | APK 下载与安装 |

---

## 第三章 系统架构

### 3.1 整体架构概述

本软件采用"功能优先（Feature-First）"的目录分层架构，结合 Riverpod 依赖注入实现各层解耦。整体可分为四层：

```
┌───────────────────────────────────────────┐
│               UI层（Pages & Widgets）       │
│  展示业务界面，监听 Provider 变化驱动重绘     │
├───────────────────────────────────────────┤
│           Application层（Providers）        │
│  封装业务逻辑，协调各 Service 完成用例       │
├───────────────────────────────────────────┤
│            Core 服务层（Services）          │
│  独立可测的基础能力：网络、推送、音频、打印   │
├───────────────────────────────────────────┤
│         平台/基础设施层（Infrastructure）    │
│  Android 原生 API、第三方 SDK、文件系统      │
└───────────────────────────────────────────┘
```

### 3.2 目录结构

```
lib/
├── main.dart                    # 应用入口，完成各服务初始化
├── app.dart                     # 根 Widget，GoRouter 路由注册
├── config/
│   ├── env.dart                 # 环境变量（API 地址、推送 Key 等）
│   └── theme.dart               # 全局设计系统（颜色、间距、圆角）
├── core/
│   ├── audio/                   # 铃声与 TTS 播报服务
│   │   ├── sound_player.dart
│   │   └── tts_service.dart
│   ├── network/                 # HTTP 客户端与 WebSocket 客户端
│   │   ├── api_client.dart
│   │   ├── api_provider.dart
│   │   ├── connectivity_provider.dart
│   │   ├── ws_client.dart
│   │   └── ws_provider.dart
│   ├── push/                    # 厂商推送初始化与消息分发
│   ├── print/                   # 蓝牙打印协议与指令生成
│   ├── service/
│   │   ├── auth_session_controller.dart   # Token 生命周期管理
│   │   ├── foreground_service.dart        # 前台 Service 封装
│   │   ├── message_dedup.dart             # 消息去重（SQLite）
│   │   ├── navigation_service.dart        # 全局导航服务
│   │   └── order_poller.dart              # 30 秒定时轮询
│   └── utils/                   # 通用工具函数
├── features/
│   ├── auth/                    # 绑定码登录、身份状态管理
│   ├── order/                   # 订单列表、详情、告警页
│   ├── printer/                 # 蓝牙打印机扫描与连接
│   ├── settings/                # 系统设置、权限引导
│   └── update/                  # 版本检测与 APK 更新
├── models/
│   ├── order.dart               # 订单数据模型
│   └── push_message.dart        # 推送消息数据模型
└── widgets/                     # 全局公共 Widget 组件库
```

### 3.3 状态管理架构

本软件使用 Riverpod 作为唯一状态管理方案，禁止使用单例或全局变量。

- `FutureProvider`：用于一次性异步请求，如登录验证、版本检测
- `StreamProvider`：用于 WebSocket 消息流、网络状态流
- `StateNotifierProvider`：用于可变 UI 状态，如订单列表、打印机连接状态、提醒设置
- `Provider`：用于只读服务实例的依赖注入，如各 Service 类

### 3.4 消息传递流程

```
厂商推送 SDK         WebSocket 连接         定时轮询（30s）
      │                     │                      │
      ▼                     ▼                      ▼
 pushManager.onNewOrder  WsClient.onNewOrder   orderPoller._pollOrders
      │                     │                      │
      └──────────┬──────────┘                      │
                 ▼                                  │
     OrderAlertCoordinator.handleIncomingOrder ◄───┘
                 │
        MessageDeduplicator（去重）
                 │
     ┌───────────┴────────────┐
     │                        │
SoundPlayer / TtsService    LocalNotificationService
  (铃声 + 语音播报)          (系统通知/全屏弹窗)
     │                        │
     └───────────┬────────────┘
                 ▼
          OrderAlertPage（全屏接单弹窗）
                 │
         商家点击"接单"
                 │
    POST /v1/merchant/orders/:id/accept
                 │
          PrinterProvider（自动打印小票）
```

---

## 第四章 功能模块详细说明

### 4.1 启动与初始化模块

#### 4.1.1 功能描述

应用启动时，`main()` 函数按序完成以下初始化工作：

1. **TTS 引擎初始化**：调用 `TtsService.init()`，预加载文字转语音引擎，设置中文语言包（zh-CN），配置语速和音量参数。
2. **前台服务初始化**：调用 `MerchantForegroundService.init()`，注册 Android 前台 Service 所需的通知通道和回调。
3. **消息去重数据库初始化**：调用 `MessageDeduplicator.ensureInitialized()`，创建或打开本地 SQLite 数据库，建立去重记录表。
4. **本地通知服务初始化**：创建 `order_alert`（高优先级）、`merchant_fg_service`（低优先级）、`update_channel`（默认优先级）三个通知渠道；申请通知权限。
5. **厂商推送初始化**：分别初始化华为 HMS、小米 MiPush、OPPO Push、vivo Push 各厂商原生 SDK，注册设备 Token，挂载 `onNewOrder` 和 `onNotificationOpened` 回调。
6. **WebSocket 客户端初始化**：创建 WsClient 实例，连接管理器监听登录状态变化，登录成功后自动建立 WSS 长连接。

#### 4.1.2 初始化流程图

```
应用启动
  │
  ├─ WidgetsFlutterBinding.ensureInitialized()
  ├─ TtsService.init()
  ├─ MerchantForegroundService.init()
  ├─ MessageDeduplicator.ensureInitialized()
  ├─ LocalNotificationService.init()
  │     ├─ 创建通知渠道（order_alert / merchant_fg_service / update_channel）
  │     └─ 申请 POST_NOTIFICATIONS 权限
  ├─ PushManager.init()
  │     ├─ 各厂商推送 SDK 初始化（华为/小米/OPPO/vivo）
  │     ├─ 注册厂商推送通道（华为/小米/OPPO/vivo）
  │     └─ 绑定 onNewOrder 回调
  └─ runApp(ProviderScope → MerchantAppBootstrap)
        ├─ wsConnectionManagerProvider（监听登录态，按需连接 WebSocket）
        └─ orderPollerManagerProvider（监听营业状态，按需启动轮询）
```

#### 4.1.3 关键配置

通过 Dart 编译期环境变量（`--dart-define`）注入，避免硬编码：

| 配置项 | 环境变量名 | 默认值 |
|---|---|---|
| API 基础地址 | `API_BASE_URL` | `https://llapi.merrydance.cn/v1` |
| WebSocket 地址 | `WS_URL` | `wss://llapi.merrydance.cn/v1/ws` |


---

### 4.2 账号绑定与身份认证模块

#### 4.2.1 功能描述

本软件采用"绑定码"认证方案，无需传统用户名/密码。流程如下：

**首次绑定：**

1. 商家在乐客来福微信小程序的商家后台生成一个 6 位数字绑定码，有效期约 5 分钟。
2. 商家在本 App 的绑定页面输入该绑定码并点击"绑定"。
3. App 调用 `POST /v1/auth/app-bind/verify` 接口，携带绑定码及设备信息（设备 UUID、型号、系统版本、App 版本），换取 JWT AccessToken 与 RefreshToken。
4. Token 通过 `flutter_secure_storage` 写入 Android Keystore 加密存储区。
5. 绑定成功后跳转至订单列表主界面。

**后续自动登录：**

App 每次启动时从安全存储中读取 Token，通过 `POST /v1/auth/refresh` 换取新 Token 对。若 RefreshToken 过期（有效期 365 天）则回退绑定页面，要求重新绑定。

**Token 自动刷新：**

`ApiClient` 的 Dio 拦截器在每次请求时自动附加 `Authorization: Bearer {accessToken}`。若收到 HTTP 401 响应，自动触发 Token 刷新流程，刷新成功后重试原请求，对业务层透明。

#### 4.2.2 设备注册

绑定成功后，App 调用 `POST /v1/merchant/device/register` 向服务端注册设备推送 Token，确保后续厂商推送消息可以准确路由到当前设备。设备注册信息包括：

- 设备唯一 UUID（本地生成并持久化）
- 厂商推送 Token（各厂商 RegistrationID）
- 设备型号与操作系统版本
- App 版本号

#### 4.2.3 数据安全

- AccessToken 和 RefreshToken 严格存储于 Android Keystore 加密区，不写入 SharedPreferences、数据库或任何明文文件。
- 设备 UUID 同样使用 `flutter_secure_storage` 存储，防止被第三方应用读取。
- 绑定码仅用于一次性换取 Token，不在本地持久化。

#### 4.2.4 登出与解绑

- 商家可在设置页面手动登出，App 调用 `DELETE /v1/merchant/device/:device_id` 注销设备推送通道，并清除本地所有 Token。
- 解绑后 App 回到绑定页面，需重新输入绑定码完成绑定。

---

### 4.3 订单实时推送与接收模块

#### 4.3.1 功能描述

为最大化订单消息到达率，本软件同时维护三条消息接收通道。三通道并行运行，任意一条通道成功接收消息均可触发后续告警流程。

#### 4.3.2 通道一：WebSocket 实时通道

- 连接地址：`wss://llapi.merrydance.cn/v1/ws?token={accessToken}`
- 本软件在用户完成绑定且开启营业状态后，`WsConnectionManager` 自动建立 WSS 长连接。
- 心跳机制：通过 WebSocket PingFrame（间隔 20 秒）维持连接活性，避免被中间网关断开。
- 自动重连：连接断开后采用指数退避策略（最长等待约 60 秒）自动重连，并在主界面状态栏实时反映当前连接状态。
- 消息格式：JSON；收到消息后调用 `MessageDeduplicator` 去重，去重通过后转发给 `OrderAlertCoordinator`。

#### 4.3.3 通道二：厂商原生推送通道

- 直接集成华为 HMS、小米 MiPush、OPPO Push、vivo Push 四大厂商原生推送 SDK，各 SDK 由对应厂商系统级 Daemon 托管。
- 厂商通道在 App 进程被系统回收后依然可以将消息推送到设备，由系统级 Daemon 唤醒 App 处理。
- App 收到厂商推送消息后，在 `PushManager.onNewOrder` 回调中经消息去重后转发给协调器。

#### 4.3.4 通道三：定时轮询

- `OrderPoller` 在以下条件同时满足时启动：用户已绑定 + 营业状态开启 + 网络可用。
- 每隔 30 秒调用 `GET /v1/merchant/orders/pending` 获取待接单订单列表。
- 将服务端返回的订单与本地已知订单集合对比，对新出现的 `pending` 订单触发告警流程。

#### 4.3.5 通道对比

| 通道 | 延迟 | 说明 |
|---|---|---|
| WebSocket | <1 秒 | 实时性最高；断网时不可用，需 App 进程存活 |
| 厂商原生推送 | 1~3 秒 | 可靠性高；App 进程被回收后仍可送达 |
| 定时轮询 | 最长 30 秒 | 兜底补漏；需 App 进程存活 |

---

### 4.4 消息去重模块

#### 4.4.1 功能描述

由于三通道可能同时或先后接收到同一条订单推送（相同 `message_id`），若不去重将导致同一张订单触发多次告警，极大干扰商家操作体验。`MessageDeduplicator` 负责拦截重复消息。

#### 4.4.2 实现机制

- 使用 SQLite 本地数据库（通过 `sqflite` 包）持久化已处理的 `message_id` 集合。
- 每条消息到达时，先查询数据库中是否存在相同 `message_id`。若存在，丢弃该消息，终止后续处理流程。若不存在，写入数据库后继续处理。
- 去重记录保留 24 小时，超期记录由定时清理任务自动删除，避免数据库无限增长。
- `ensureInitialized()` 在启动阶段完成数据库开启与表结构创建，后续操作均为同步写入 + 异步查询，不阻塞主线程。

---

### 4.5 订单管理模块

#### 4.5.1 功能描述

订单管理是本软件的核心展示界面，入口为 `OrderListPage`（订单列表页）。商家可通过标签页切换查看以下三类订单：

| 标签页 | 对应状态 | 说明 |
|---|---|---|
| 待接单 | pending | 已下单、尚未确认 |
| 进行中 | accepted / preparing / delivering | 已接单、处理中 |
| 已完成 | completed / cancelled | 已完结或已取消 |

#### 4.5.2 标题栏营业状态切换

主界面标题栏右侧提供"在线营业 / 离线打烊"开关（`Switch`），商家可随时切换工作状态：

- **开启营业**：WebSocket 连接建立，轮询启动，界面标题显示"[商户名称] (在线营业)"
- **关闭营业**：WebSocket 断开，轮询停止，界面标题显示"[商户名称] (离线打烊)"，不再接收任何订单推送

#### 4.5.3 订单列表

- 每条订单显示：订单号、金额、商品明细摘要、规格、顾客备注、下单时间、当前状态标签
- 待接单列表中，订单按下单时间倒序排列，最新订单置顶
- 支持手动刷新，触发 `GET /v1/merchant/orders` 同步最新数据；订单明细由后端随订单结构一并返回，异常时由系统自动重试，不把空列表当作真实商品清单

#### 4.5.4 订单详情页

点击任意订单进入 `OrderDetailPage`，展示完整订单信息：

- **基本信息**：订单号、当前状态、下单时间
- **顾客信息**：顾客姓名（若有）、联系电话（若有）
- **商品明细**：商品名称、数量、规格、单价、小计，以及订单总金额
- **备注**：顾客填写的特殊要求（若有）
- **明细恢复**：商品明细由后端派发前校验并自动重试，商户无需手动同步；页面收到完整明细后自动刷新展示
- **操作按钮**：针对"待接单"订单显示"接单"按钮；针对已接单订单显示"重新打印小票"按钮

#### 4.5.5 接单操作

商家点击"接单"后：

1. 按钮进入加载状态，防止二次点击
2. 调用 `POST /v1/merchant/orders/:id/accept` 通知服务端
3. 成功后，订单状态更新为"已接单"，若打印机已连接则自动打印小票
4. 失败时显示错误提示，订单保留在"待接单"列表

---

### 4.6 订单提醒与告警模块

#### 4.6.1 功能描述

当 `OrderAlertCoordinator` 收到经去重验证的新订单消息时，触发以下告警流程：

**应用在前台时（App 可见）：**

1. 播放铃声提醒（若用户已开启"铃声提醒"）
2. 进行语音播报（若用户已开启"语音播报"）
3. 若"自动接单"已开启，直接调用接单接口并打印小票；否则弹出全屏接单弹窗

**应用在后台或设备息屏时：**

1. 通过 `flutter_local_notifications` 在 `order_alert` 通道发出高优先级系统通知
2. 利用 Android `fullScreenIntent` 在锁屏界面直接展示全屏弹窗，唤醒商家注意
3. 同步播放铃声与语音（需相关音频权限）

#### 4.6.2 全屏接单弹窗（OrderAlertPage）

弹窗界面采用红色渐变背景（`tertiary` → 深红），强视觉冲击，内容包括：

- 图标（铃铛动画）
- 提示文字："您有新的订单"
- 订单金额（超大字号显示）
- 来源店铺名称
- 商品摘要、规格和顾客备注；系统恢复期间显示"订单明细同步中"，不要求商户手动补同步
- 操作按钮：**接单**（主操作，绿色）、**查看详情**（次操作）

商家必须在 60 秒内点击接单，否则服务端将重新推送并通知运营人员介入。

#### 4.6.3 自动接单模式

在设置页面开启"自动接单"后，新订单到达时系统自动调用接单接口，无需商家手动确认。若自动接单 API 调用失败，订单退回待接单列表，弹出手动接单弹窗。

---

### 4.7 语音播报与铃声模块

#### 4.7.1 铃声提醒

使用 `audioplayers` 播放预录 MP3 音频文件（位于 `assets/audio/` 目录），内容为预录的"您有新的乐客来福订单"提示音。相较 TTS 合成，预录音频响应更快（<100ms），适合高频告警场景。

#### 4.7.2 TTS 语音播报

使用 `flutter_tts` 引擎（底层依赖 Android TextToSpeech API）合成订单详情语音，播报内容格式为：

> "您有新的订单，订单号 [订单号后四位]，金额 [金额] 元"

语音参数配置：
- 语言：中文（zh-CN）
- 语速（speechRate）：适中，保证清晰度
- 音量（volume）：1.0（最大），确保嘈杂厨房环境下可听见

#### 4.7.3 设置项

用户可在设置页面独立开启/关闭铃声提醒和语音播报，两者互相独立，互不影响。

---

### 4.8 小票打印模块

#### 4.8.1 功能描述

本软件支持通过蓝牙连接热敏打印机，在接单后自动打印订单小票。

#### 4.8.2 蓝牙打印机管理（BluetoothPrinterPage）

商家进入"小票打印机"页面后，软件自动触发 BLE 扫描：

1. **扫描**：使用 `flutter_blue_plus` 扫描周边蓝牙设备，过滤仅显示设备名不为空的设备（排除匿名广播）
2. **连接**：商家从列表中点击目标打印机设备，App 发起配对与连接请求
3. **状态展示**：连接成功后，设置页面的打印机区域显示"已连接 [设备名]"状态徽章

#### 4.8.3 自动打印逻辑

当"接单"操作成功后，若蓝牙打印机已连接，`PrinterProvider` 自动获取订单详情并构建打印指令（基于 ESC/POS 协议），通过蓝牙 GATT 特征写入打印机执行打印。

打印内容包括：
- 店铺名称
- 订单号
- 商品明细（名称、数量、规格、单价、小计）
- 订单总金额
- 顾客备注（若有）

#### 4.8.4 打印异常处理

蓝牙连接断开或打印失败时，系统显示错误提示，订单不会受影响（已完成接单操作）。商家可在订单详情页手动点击"重新打印小票"再次尝试。

---

### 4.9 设备保活管理模块

#### 4.9.1 功能描述

国内 Android 定制系统（EMUI、MIUI 等）对后台应用有严格的电池优化与内存回收策略，容易将本应用进程挂起或杀死，导致 WebSocket 断开、轮询停止、厂商推送通道无法唤醒 App。为此，本软件采用多层保活策略。

#### 4.9.2 前台 Service（Foreground Service）

- 使用 `flutter_foreground_task` 启动 Android 前台 Service
- 常驻通知文字："乐客来福正在运行"（通知渠道 `merchant_fg_service`，低优先级，折叠显示）
- 前台 Service 可有效阻止系统进入省电时回收应用进程

#### 4.9.3 权限引导页（PermissionGuidePage）

首次使用时，软件引导商家完成各品牌机型的保活权限设置，支持以下品牌：

| 品牌 / 系统 | 关键设置步骤 |
|---|---|
| 华为 / 荣耀（EMUI/HarmonyOS） | 关闭自动管理，手动开启自启动、关联启动、后台活动；加入电池优化白名单 |
| 小米 / Redmi（MIUI/HyperOS） | 开启自启动；省电设置改为"无限制"；任务管理界面锁定应用卡片 |
| OPPO（ColorOS） | 在电池管理中关闭应用耗电监控；在权限管理中开启后台运行 |
| vivo（OriginOS） | 开启高后台运行权限；加入内存清理白名单 |

此页面同时提供"前往系统设置"按钮，点击后跳转至对应系统设置页面，减少操作步骤。

---

### 4.10 系统设置模块

#### 4.10.1 功能描述

系统设置页（`SettingsPage`）集中管理所有可配置项，分为以下功能区：

**硬件与连接区：**
- 蓝牙打印机：显示当前连接状态，点击进入蓝牙打印机管理页

**提醒与保活区：**
- 铃声提醒：开关控制来单铃声播放
- 语音播报：开关控制 TTS 订单语音播报
- 自动接单：开关控制是否自动确认新订单
- 接单后自动打印：开关控制接单成功后是否自动打印小票
- 自启动与保活设置：跳转至权限引导页

**账号与应用区：**
- 当前商户名称与绑定状态
- 检查更新：手动触发版本检测
- 服务协议：查看用户协议
- 关于软件：显示版本号、联系方式等

#### 4.10.2 设置持久化

提醒相关设置（铃声、语音、自动接单、自动打印）通过 `StateNotifierProvider` + `SharedPreferences` 持久化，重启后恢复用户设置。

---

### 4.11 应用更新模块

#### 4.11.1 功能描述

本软件在每次启动时自动检查是否有新版本，也支持在设置页面手动触发检测。

#### 4.11.2 检测流程

1. 读取当前 App 版本号（versionCode 和 versionName）
2. 调用 `GET /v1/app/version/latest`，携带平台（android）、渠道（merchant_app）、包名（com.merrydance.locallife.merchant）及当前版本码
3. 服务端返回最新版本信息（version_code、version_name、changelog、download_url、is_force）
4. 若 `version_code` > 当前版本码，弹出更新对话框

#### 4.11.3 更新对话框（UpdateDialog）

- 展示新版本号与更新日志
- 提供"立即更新"按钮，点击后后台下载 APK 并启动系统安装流程
- **强制更新**（`is_force: true`）：对话框不可通过点击外部关闭，商家必须更新才能继续使用
- **非强制更新**：提供"稍后再说"按钮，商家可暂时跳过

---

## 第五章 数据模型说明

### 5.1 订单模型（OrderModel）

| 字段名 | 类型 | 说明 |
|---|---|---|
| id | String | 订单唯一标识（服务端生成） |
| orderNum | String | 订单号（展示用，如 LL20260415001） |
| amount | double | 订单金额（元） |
| status | OrderStatus | 订单状态枚举 |
| createdAt | DateTime | 下单时间 |
| userName | String? | 顾客姓名（可选） |
| userPhone | String? | 顾客电话（可选） |
| items | List\<OrderItem\> | 商品明细列表 |
| note | String? | 顾客备注（可选） |

### 5.2 订单状态枚举（OrderStatus）

| 枚举值 | 中文标签 | 说明 |
|---|---|---|
| pending | 待接单 | 订单已生成，商家尚未确认 |
| accepted | 已接单 | 商家已确认，准备处理 |
| preparing | 制作中 | 订单正在准备（餐品制作等） |
| delivering | 配送中 | 订单已交付配送 |
| completed | 已完成 | 订单全流程完结 |
| cancelled | 已取消 | 订单已被取消 |

### 5.3 推送消息模型（PushMessage）

| 字段名 | 类型 | 说明 |
|---|---|---|
| messageId | String | 全局唯一消息 ID（用于去重） |
| orderId | String | 关联订单 ID |
| orderNumber | String | 订单号（展示用） |
| amount | double | 订单金额 |
| shopName | String | 店铺名称 |

---

## 第六章 接口说明

### 6.1 认证接口

#### 绑定码验证

- **路径：** POST /v1/auth/app-bind/verify
- **鉴权：** 无（公开接口）
- **请求体：** `{ code, device_id, device_model, os_version, app_version }`
- **响应：** `{ access_token, refresh_token, merchant_name }`

#### Token 刷新

- **路径：** POST /v1/auth/refresh
- **鉴权：** 携带 RefreshToken
- **响应：** `{ access_token, refresh_token }`

### 6.2 设备接口

#### 注册设备

- **路径：** POST /v1/merchant/device/register
- **鉴权：** Bearer AccessToken
- **请求体：** `{ device_id, vendor_push_token, device_model, os_version, app_version }`

#### 注销设备

- **路径：** DELETE /v1/merchant/device/:device_id
- **鉴权：** Bearer AccessToken

#### 设备心跳

- **路径：** PUT /v1/merchant/device/heartbeat
- **鉴权：** Bearer AccessToken

### 6.3 订单接口

#### 获取待接单列表

- **路径：** GET /v1/merchant/orders/pending
- **鉴权：** Bearer AccessToken
- **响应：** 订单数组（OrderModel 列表）

#### 确认接单

- **路径：** POST /v1/merchant/orders/:id/accept
- **鉴权：** Bearer AccessToken
- **响应：** 更新后的订单状态

### 6.4 更新接口

#### 检查最新版本

- **路径：** GET /v1/app/version/latest
- **鉴权：** 无
- **查询参数：** `platform, channel, package_name, version_code, version_name`
- **响应：** `{ version_code, version_name, changelog, download_url, is_force }`

---

## 第七章 安全设计

### 7.1 数据存储安全

- 所有鉴权凭证（AccessToken、RefreshToken、设备 UUID）均通过 `flutter_secure_storage` 存储于 Android Keystore 加密区，密钥由硬件安全模块（HSM）保护，无法通过 Root 提权直接读取文件系统获得。
- 不将任何敏感信息写入 SharedPreferences、数据库明文字段或应用日志。

### 7.2 网络传输安全

- 所有 HTTP 接口均使用 HTTPS（TLS 1.2+），WebSocket 使用 WSS，不存在明文传输。
- API AccessToken 通过 HTTP 请求头 `Authorization: Bearer` 传递，不拼入 URL 查询参数（避免日志泄露），仅 WebSocket 连接时作为 URL 参数传递，且仅在调试模式下打印脱敏日志（`token=***`）。

### 7.3 Token 生命周期管理

- AccessToken 短期有效，到期自动刷新，RefreshToken 365 天有效期；Token 过期后强制重新绑定。
- 登出时主动注销服务端设备记录，避免废弃 Token 被滥用。

---

## 第八章 用户操作说明

### 8.1 首次使用流程

```
第一步：下载并安装乐客来福商户 APK
   │
第二步：打开微信小程序"乐客来福商家后台"，进入"设备管理"页面生成绑定码
   │
第三步：打开 App，在绑定码输入框中输入 6 位绑定码，点击"绑定"
   │
第四步：绑定成功后，按照"自启动与保活设置"页面的指引，
        根据手机品牌完成保活权限配置
   │
第五步：返回主界面，打开"在线营业"开关，开始接收订单
```

### 8.2 日常使用操作

**开启营业：**
在主界面右上角打开开关至"在线营业"状态，App 即开始监听新订单。

**接单方式：**
- 手动接单：新订单到达时，全屏弹窗展示订单信息，商家点击"接单"按钮确认
- 自动接单：在设置页面开启"自动接单"后，系统无需商家操作，自动完成接单

**查看历史订单：**
在主界面通过"进行中"和"已完成"标签页查看历史订单及详情。

**连接打印机：**
进入"系统设置 → 小票打印机"，等待扫描完成后选择目标蓝牙打印机点击连接。连接成功后每次接单将自动打印小票。

**解绑设备：**
在"系统设置 → 账号"区域点击"退出登录"，解除当前设备与商户账号的绑定关系。

### 8.3 网络异常处理说明

| 异常情况 | 软件行为 |
|---|---|
| 网络断开 | 主界面状态栏显示"网络不可用"提示；WebSocket 断开；轮询暂停 |
| 仅 WebSocket 中断 | 状态栏显示"实时连接已断开"（黄色警告），每隔 30 秒轮询仍正常运行 |
| 网络恢复 | WebSocket 自动重连，轮询自动恢复 |
| Token 失效（401） | Dio 拦截器自动刷新 Token 并重试请求，对商家透明 |

---

## 第九章 错误处理与异常恢复

### 9.1 启动阶段异常

- TTS 初始化失败：记录错误日志，语音功能降级为仅铃声，不影响其他功能
- 推送 SDK 初始化失败：记录错误日志，厂商推送通道不可用，WebSocket 与轮询通道正常运行
- 数据库初始化失败：App 退出并弹出错误提示，建议重启设备后重试

### 9.2 运行阶段异常

- 接单 API 调用失败：显示 Toast 错误提示，订单保留在"待接单"列表
- 打印失败：显示错误提示，不影响接单流程，可手动重试
- 版本检测超时：静默跳过，不影响其他功能；手动检查时显示"检测失败，请重试"

### 9.3 异常上报

在生产模式（`dart.vm.product = true`）下，捕获到的非预期异常通过全局 `ErrorHandler` 统一处理，格式化错误信息后展示给用户，并可通过日志机制上报至监控系统（具体接入由运维侧配置）。

---

## 第十章 性能指标

| 指标项 | 目标值 |
|---|---|
| 冷启动时间（从点击图标到主界面可交互） | ≤ 3 秒 |
| 订单推送到全屏弹窗弹出的端到端延迟 | ≤ 2 秒（前台）/ ≤ 5 秒（后台，厂商推送通道） |
| WebSocket 断线重连时间 | ≤ 60 秒（指数退避上限） |
| 轮询兜底最大延迟 | 30 秒 |
| 消息去重查询时间 | < 10 毫秒（SQLite 本地查询） |
| 蓝牙打印机连接时间 | ≤ 5 秒（设备已配对状态下） |
| APK 安装包大小 | ≤ 50 MB |
| 运行时内存占用（常态） | ≤ 150 MB |

---

*本文档共 10 章，描述了乐客来福商户软件 V1.0 的完整功能设计与操作说明，用于软件著作权登记申请。*
