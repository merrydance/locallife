import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:device_info_plus/device_info_plus.dart';
import 'package:merchant_app/core/service/message_dedup.dart';
import 'package:merchant_app/models/push_message.dart';

typedef TokenCallback = Future<void> Function(String token, String provider);

typedef PushOrderCallback =
    Future<void> Function(
      PushMessage message, {
      required bool showLocalNotification,
    });

class NativePushManager {
  static const MethodChannel _channel = MethodChannel(
    'com.locallife.merchant/push',
  );

  final MessageDeduplicator _deduplicator;

  // Callback when a valid new order arrives
  PushOrderCallback? onNewOrder;
  Future<void> Function(PushMessage message)? onNotificationOpened;
  TokenCallback? onTokenRegistered;

  NativePushManager(this._deduplicator);

  Future<void> init() async {
    if (kIsWeb) return;
    _channel.setMethodCallHandler(_handleMethodCall);

    // Initial native setup
    try {
      final String manufacturer = await _getManufacturer();
      await _channel.invokeMethod('initialize', {'manufacturer': manufacturer});
      debugPrint('NativePushManager initialized for $manufacturer');
    } on PlatformException catch (e) {
      debugPrint('Failed to initialize native push: ${e.message}');
    }
  }

  Future<String> _getManufacturer() async {
    final deviceInfo = DeviceInfoPlugin();
    if (defaultTargetPlatform == TargetPlatform.android) {
      final androidInfo = await deviceInfo.androidInfo;
      return androidInfo.manufacturer.toLowerCase();
    }
    return 'unknown';
  }

  Future<void> _handleMethodCall(MethodCall call) async {
    switch (call.method) {
      case 'onReceiveMessage':
        final Map<String, dynamic> message = Map<String, dynamic>.from(
          call.arguments,
        );
        await _handleIncomingMessage(message, showLocalNotification: true);
        break;
      case 'onNotificationOpened':
        final Map<String, dynamic> message = Map<String, dynamic>.from(
          call.arguments,
        );
        final parsedMessage = _extractPushMessage(message);
        if (parsedMessage != null) {
          await onNotificationOpened?.call(parsedMessage);
        }
        break;
      case 'onTokenRegistered':
        final String token = call.arguments['token'];
        final String provider = call.arguments['provider'];
        debugPrint('Push token registered (Provider: $provider)');
        await onTokenRegistered?.call(token, provider);
        break;
    }
  }

  Future<void> _handleIncomingMessage(
    Map<String, dynamic> message, {
    required bool showLocalNotification,
  }) async {
    final pushMessage = _extractPushMessage(message);
    if (pushMessage == null) return;

    final accepted = await _deduplicator.tryAcceptGroup([
      MessageDeduplicator.messageKey(pushMessage.messageId),
      MessageDeduplicator.orderKey(pushMessage.orderId),
    ]);

    if (accepted) {
      await onNewOrder?.call(
        pushMessage,
        showLocalNotification: showLocalNotification,
      );
    }
  }

  PushMessage? _extractPushMessage(Map<String, dynamic> message) {
    // Standardize payload extraction from different vendors if needed
    // Assuming native side sends a clean map
    try {
      return PushMessage.fromJson(message);
    } catch (e) {
      debugPrint('Error parsing push message: $e');
      return null;
    }
  }

  Future<String?> getRegistrationID() async {
    if (kIsWeb) return null;
    return await _channel.invokeMethod<String>('getRegistrationId');
  }
}
