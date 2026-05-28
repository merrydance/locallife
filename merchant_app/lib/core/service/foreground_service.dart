import 'package:flutter/foundation.dart';
import 'package:flutter_foreground_task/flutter_foreground_task.dart';

enum ForegroundServiceStatus {
  stopped,
  waitingForOrders,
  networkUnavailable,
  reconnecting,
  notificationPermissionMissing,
}

class ForegroundServiceDecision {
  const ForegroundServiceDecision({
    required this.shouldRun,
    required this.status,
    required this.notificationText,
  });

  final bool shouldRun;
  final ForegroundServiceStatus status;
  final String notificationText;
}

ForegroundServiceDecision decideForegroundServiceStatus({
  required bool isAuthenticated,
  required bool isWorking,
  required bool hasNetwork,
  required bool isWebSocketConnected,
  required bool isNotificationPermissionGranted,
}) {
  if (!isAuthenticated || !isWorking) {
    return const ForegroundServiceDecision(
      shouldRun: false,
      status: ForegroundServiceStatus.stopped,
      notificationText: '商户未上线营业',
    );
  }

  if (!isNotificationPermissionGranted) {
    return const ForegroundServiceDecision(
      shouldRun: true,
      status: ForegroundServiceStatus.notificationPermissionMissing,
      notificationText: '通知权限未开启 · 请到设置中允许通知',
    );
  }

  if (!hasNetwork) {
    return const ForegroundServiceDecision(
      shouldRun: true,
      status: ForegroundServiceStatus.networkUnavailable,
      notificationText: '网络不可用 · 正在等待恢复并补单',
    );
  }

  if (!isWebSocketConnected) {
    return const ForegroundServiceDecision(
      shouldRun: true,
      status: ForegroundServiceStatus.reconnecting,
      notificationText: '连接中断 · 正在重连并启用轮询兜底',
    );
  }

  return const ForegroundServiceDecision(
    shouldRun: true,
    status: ForegroundServiceStatus.waitingForOrders,
    notificationText: '后台运行中 · 等待新订单...',
  );
}

class MerchantForegroundService {
  static Future<void> init() async {
    FlutterForegroundTask.init(
      androidNotificationOptions: AndroidNotificationOptions(
        channelId: 'merchant_fg_service',
        channelName: '商户在线服务',
        channelDescription: '保持商户端在线接收订单',
        channelImportance: NotificationChannelImportance.LOW,
        priority: NotificationPriority.LOW,
      ),
      iosNotificationOptions: const IOSNotificationOptions(),
      foregroundTaskOptions: ForegroundTaskOptions(
        eventAction: ForegroundTaskEventAction.repeat(15000),
        autoRunOnBoot: true,
        autoRunOnMyPackageReplaced: true,
        allowWakeLock: true,
        allowWifiLock: true,
      ),
    );
  }

  static Future<void> start() async {
    if (await FlutterForegroundTask.isRunningService) {
      await FlutterForegroundTask.updateService(
        notificationTitle: '乐客来福商户端',
        notificationText: '后台运行中 · 等待新订单...',
      );
    } else {
      await FlutterForegroundTask.startService(
        serviceTypes: const [
          ForegroundServiceTypes.dataSync,
          ForegroundServiceTypes.remoteMessaging,
        ],
        notificationTitle: '乐客来福商户端',
        notificationText: '后台运行中 · 等待新订单...',
        callback: startCallback,
      );
    }
  }

  static Future<void> applyDecision(ForegroundServiceDecision decision) async {
    if (!decision.shouldRun) {
      await stop();
      return;
    }

    await start();
    await updateStatus(decision);
  }

  static Future<void> updateStatus(ForegroundServiceDecision decision) async {
    if (!await FlutterForegroundTask.isRunningService) {
      return;
    }
    await FlutterForegroundTask.updateService(
      notificationTitle: '乐客来福商户端',
      notificationText: decision.notificationText,
    );
  }

  static Future<void> stop() async {
    await FlutterForegroundTask.stopService();
  }
}

@pragma('vm:entry-point')
void startCallback() {
  FlutterForegroundTask.setTaskHandler(MerchantTaskHandler());
}

class MerchantTaskHandler extends TaskHandler {
  @override
  Future<void> onStart(DateTime timestamp, TaskStarter starter) async {
    if (kDebugMode) {
      debugPrint('Foreground task started at $timestamp by $starter');
    }
  }

  @override
  void onRepeatEvent(DateTime timestamp) {
    // Heartbeat logic or polling fallback can be added here
    if (kDebugMode) {
      debugPrint('Foreground service heartbeat at $timestamp');
    }
  }

  @override
  Future<void> onDestroy(DateTime timestamp, bool isTimeout) async {
    if (kDebugMode) {
      debugPrint('Foreground service destroyed (isTimeout: $isTimeout)');
    }
  }
}
