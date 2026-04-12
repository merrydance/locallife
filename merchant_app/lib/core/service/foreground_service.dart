import 'package:flutter/foundation.dart';
import 'package:flutter_foreground_task/flutter_foreground_task.dart';

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
        allowWakeLock: true,
        allowWifiLock: true,
      ),
    );
  }

  static Future<void> start() async {
    if (await FlutterForegroundTask.isRunningService) {
      await FlutterForegroundTask.restartService();
    } else {
      await FlutterForegroundTask.startService(
        notificationTitle: '乐客来福商户端',
        notificationText: '后台运行中 · 等待新订单...',
        callback: startCallback,
      );
    }
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
