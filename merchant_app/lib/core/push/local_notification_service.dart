import 'dart:convert';

import 'package:flutter/foundation.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:merchant_app/models/push_message.dart';
import 'package:permission_handler/permission_handler.dart';

class LocalNotificationService {
  LocalNotificationService();

  final FlutterLocalNotificationsPlugin _plugin =
      FlutterLocalNotificationsPlugin();

  bool _initialized = false;
  final List<PushMessage> _pendingTapMessages = <PushMessage>[];

  void Function(PushMessage message)? _onNotificationTap;

  set onNotificationTap(void Function(PushMessage message)? callback) {
    _onNotificationTap = callback;
    _drainPendingTapMessages();
  }

  void Function(PushMessage message)? get onNotificationTap =>
      _onNotificationTap;

  Future<void> init() async {
    if (_initialized) {
      return;
    }

    const initializationSettings = InitializationSettings(
      android: AndroidInitializationSettings('@mipmap/launcher_icon'),
      iOS: DarwinInitializationSettings(),
    );

    await _plugin.initialize(
      settings: initializationSettings,
      onDidReceiveNotificationResponse: _handleNotificationResponse,
      onDidReceiveBackgroundNotificationResponse:
          _handleBackgroundNotificationResponse,
    );

    final launchDetails = await _plugin.getNotificationAppLaunchDetails();
    final response = launchDetails?.notificationResponse;
    if (launchDetails?.didNotificationLaunchApp == true && response != null) {
      await _handleNotificationResponse(response);
    }

    final androidPlugin = _plugin
        .resolvePlatformSpecificImplementation<
          AndroidFlutterLocalNotificationsPlugin
        >();
    if (androidPlugin != null) {
      await androidPlugin.createNotificationChannel(
        const AndroidNotificationChannel(
          'order_alert',
          '订单提醒',
          description: '新订单与超时提醒',
          importance: Importance.max,
          playSound: true,
        ),
      );
      await androidPlugin.createNotificationChannel(
        const AndroidNotificationChannel(
          'update_channel',
          '应用更新',
          description: '版本更新通知',
          importance: Importance.defaultImportance,
        ),
      );
    }

    _initialized = true;
  }

  Future<bool> ensureNotificationPermission() async {
    final status = await Permission.notification.status;
    if (status.isGranted) {
      return true;
    }

    final result = await Permission.notification.request();
    return result.isGranted;
  }

  Future<void> showNewOrderNotification(PushMessage message) async {
    await init();

    const androidDetails = AndroidNotificationDetails(
      'order_alert',
      '订单提醒',
      channelDescription: '新订单与超时提醒',
      importance: Importance.max,
      priority: Priority.high,
      category: AndroidNotificationCategory.call,
      visibility: NotificationVisibility.public,
      fullScreenIntent: true,
      ticker: '新订单提醒',
    );

    const notificationDetails = NotificationDetails(
      android: androidDetails,
      iOS: DarwinNotificationDetails(
        presentAlert: true,
        presentBadge: true,
        presentSound: true,
      ),
    );

    await _plugin.show(
      id: message.notificationId,
      title: message.title.isNotEmpty ? message.title : '您有新的订单',
      body: message.content.isNotEmpty
          ? message.content
          : '订单号 ${message.displayOrderNumber}，请及时处理',
      notificationDetails: notificationDetails,
      payload: jsonEncode(message.toJson()),
    );
  }

  Future<void> _handleNotificationResponse(
    NotificationResponse response,
  ) async {
    final payload = response.payload;
    if (payload == null || payload.isEmpty) {
      return;
    }

    try {
      final decoded = jsonDecode(payload);
      if (decoded is Map<String, dynamic>) {
        _dispatchNotificationTap(PushMessage.fromJson(decoded));
      }
    } catch (error) {
      if (kDebugMode) {
        debugPrint('Failed to parse local notification payload: $error');
      }
    }
  }

  void _dispatchNotificationTap(PushMessage message) {
    final callback = _onNotificationTap;
    if (callback == null) {
      if (!_pendingTapMessages.any(
        (pending) => pending.orderId == message.orderId,
      )) {
        _pendingTapMessages.add(message);
      }
      return;
    }
    callback(message);
  }

  void _drainPendingTapMessages() {
    final callback = _onNotificationTap;
    if (callback == null || _pendingTapMessages.isEmpty) {
      return;
    }
    final pending = List<PushMessage>.from(_pendingTapMessages);
    _pendingTapMessages.clear();
    for (final message in pending) {
      callback(message);
    }
  }
}

@pragma('vm:entry-point')
void _handleBackgroundNotificationResponse(NotificationResponse response) {}
