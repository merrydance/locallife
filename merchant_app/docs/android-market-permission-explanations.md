# Android 应用市场权限用途说明

适用目标：小米、华为、荣耀、OPPO、vivo 等应用市场审核表单、工单回复、人工复审说明。

## 1. 可直接复制的整体说明

本应用为商户接单工作台，主要用于接收新订单提醒、在前台或后台保持在线、通过网络同步订单状态、在收到订单后进行语音提醒和蓝牙小票打印，并支持商户主动检查新版本下载链接。相关权限仅用于订单通知、在线服务保活、网络连接保障和打印功能，不用于与核心业务无关的个人信息采集。

## 2. 权限逐项说明

### INTERNET

用途说明：

用于访问商户订单接口、WebSocket 实时消息、轮询兜底接口、协议内容接口和版本检查接口，保证商户可以及时接收新订单并同步订单状态。

对应代码路径：

- `lib/main.dart`
- `lib/core/service/order_poller.dart`
- `lib/features/update/update_service.dart`

### ACCESS_NETWORK_STATE

用途说明：

用于判断当前网络是否可用，避免在断网状态下继续轮询订单接口，并在网络恢复后恢复订单同步。

对应代码路径：

- `lib/core/service/order_poller.dart`

### WAKE_LOCK

用途说明：

用于商户端前台在线服务在熄屏、待机等场景下保持必要的唤醒能力，降低商户错过新订单提醒的概率。

对应代码路径：

- `lib/core/service/foreground_service.dart`

### RECEIVE_BOOT_COMPLETED

用途说明：

用于设备重启后自动恢复商户在线服务，避免商户因为手机重启后未重新打开应用而错过订单提醒。

对应代码路径：

- `lib/core/service/foreground_service.dart`

### VIBRATE

用途说明：

用于新订单到达时通过系统通知提供震动提醒，帮助商户在繁忙场景下及时注意到新订单。

业务说明：

- 仅用于订单消息提醒
- 不用于后台无关震动行为

### FOREGROUND_SERVICE

用途说明：

用于启动商户在线前台服务，在后台维持必要的在线接单能力，并向用户展示常驻通知，明确告知应用正在运行。

对应代码路径：

- `lib/main.dart`
- `lib/core/service/foreground_service.dart`

### USE_FULL_SCREEN_INTENT

用途说明：

用于高优先级订单提醒场景，确保商户在锁屏或后台时也能及时感知新订单，降低漏单风险。

审核答复建议：

- 仅在订单提醒等高优先级业务场景使用
- 不用于广告、营销或无关弹窗唤起

### ACCESS_WIFI_STATE

用途说明：

用于前台在线服务在 Wi-Fi 网络下保持必要的网络连接能力，提高商户接单稳定性。

对应代码路径：

- `lib/core/service/foreground_service.dart`

### CHANGE_WIFI_STATE

用途说明：

用于配合前台在线服务保持网络能力，降低后台运行时因网络休眠导致的订单消息中断风险。

审核答复建议：

- 不用于主动修改用户 Wi-Fi 配置
- 仅用于在线订单通知场景的网络稳定性保障

### 已移除的高风险权限

以下权限已从主 `AndroidManifest.xml` 中移除，因为当前商户端实现没有直接业务依赖：

- `READ_PHONE_STATE`
- `WRITE_EXTERNAL_STORAGE`
- `READ_EXTERNAL_STORAGE`
- `REQUEST_INSTALL_PACKAGES`

移除依据：

- 当前在线更新逻辑仅跳转外部下载链接，不执行应用内安装
- 当前代码未看到读写公共外部存储的实现
- 当前代码未看到读取通话状态、IMEI、SIM 信息的实现

## 3. 与业务强相关的能力说明

### 前台保活与开机恢复

用途说明：

商户端属于接单类工作应用，需要在熄屏、后台、网络切换、设备重启后尽量保持在线，以免错过新订单。当前实现中已存在前台服务、开机恢复和保活引导页面。

对应代码路径：

- `lib/core/service/foreground_service.dart`
- `lib/features/settings/permission_guide_page.dart`

### 推送与订单提醒

用途说明：

应用使用推送、WebSocket 和轮询三条链路兜底接收订单消息，以减少单点失败导致的漏单。

对应代码路径：

- `lib/main.dart`
- `lib/core/push/push_manager.dart`
- `lib/core/service/order_poller.dart`

### 蓝牙打印

用途说明：

商户可以连接蓝牙小票打印机打印订单小票，属于商户经营环节的核心功能之一。

对应代码路径：

- `lib/features/printer/bluetooth_printer_page.dart`
- `lib/features/printer/printer_provider.dart`

备注：

- 当前 `AndroidManifest.xml` 中未看到 Android 12+ 常见的 `BLUETOOTH_SCAN`、`BLUETOOTH_CONNECT` 显式声明，上架前建议结合真机测试再复核一次蓝牙权限链路
- 已明确不接入 JPush，避免聚合 SDK 自动带出未使用的定位或设备权限声明

## 4. 建议提交给审核的简化版表述

如果市场表单只允许填写短描述，建议直接使用以下版本：

### 保活和通知类权限

用于商户端在后台保持在线接收新订单，并通过高优先级通知、震动和前台服务提醒商户及时处理订单，避免漏单。

### 网络类权限

用于访问订单接口、实时消息、轮询兜底和版本检查接口，保障订单同步与网络可用性判断。

### 蓝牙打印相关说明

用于连接蓝牙小票打印机打印商户订单小票，帮助商户完成接单后的打印流程。

### 更新相关说明

当前版本仅支持检查新版本并跳转到外部下载地址，不执行静默安装。

## 5. 当前建议结论

- 强业务相关、可正常解释的权限：`INTERNET`、`ACCESS_NETWORK_STATE`、`WAKE_LOCK`、`RECEIVE_BOOT_COMPLETED`、`VIBRATE`、`FOREGROUND_SERVICE`、`USE_FULL_SCREEN_INTENT`、`ACCESS_WIFI_STATE`、`CHANGE_WIFI_STATE`
- 已完成移除的权限：`READ_PHONE_STATE`、`WRITE_EXTERNAL_STORAGE`、`READ_EXTERNAL_STORAGE`、`REQUEST_INSTALL_PACKAGES`、`QUERY_ALL_PACKAGES`，以及聚合推送 SDK 可能自动注入的定位权限
- 当前剩余权限已聚焦在订单通知、前台在线、网络状态与必要的厂商推送权限