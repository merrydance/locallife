import 'package:flutter/foundation.dart';
import 'package:jpush_flutter/jpush_flutter.dart';
import 'package:jpush_flutter/jpush_interface.dart';
import 'package:merchant_app/config/env.dart';
import 'package:merchant_app/core/service/message_dedup.dart';
import 'package:merchant_app/models/push_message.dart';

typedef PushOrderCallback = Future<void> Function(
  PushMessage message, {
  required bool showLocalNotification,
});

class PushManager {
  final JPushFlutterInterface _jpush = JPush.newJPush();
  final MessageDeduplicator _deduplicator;
  
  // Callback when a valid new order arrives
  PushOrderCallback? onNewOrder;
  Future<void> Function(PushMessage message)? onNotificationOpened;

  PushManager(this._deduplicator);

  Future<void> init() async {
    _jpush.setup(
      appKey: Env.jpushAppKey,
      channel: Env.jpushChannel,
      production: !Env.isDebug,
      debug: Env.isDebug,
    );

    _jpush.applyPushAuthority(
      const NotificationSettingsIOS(sound: true, alert: true, badge: true),
    );

    _jpush.addEventHandler(
      onReceiveNotification: (Map<String, dynamic> message) async {
        if (kDebugMode) {
          debugPrint('JPush onReceiveNotification messageId=${message['extras']?['message_id']}');
        }
        await _handleIncomingMessage(message, showLocalNotification: false);
      },
      onOpenNotification: (Map<String, dynamic> message) async {
        if (kDebugMode) {
          debugPrint('JPush onOpenNotification messageId=${message['extras']?['message_id']}');
        }
        final parsedMessage = _extractPushMessage(message);
        if (parsedMessage != null) {
          await onNotificationOpened?.call(parsedMessage);
        }
      },
      onReceiveMessage: (Map<String, dynamic> message) async {
        if (kDebugMode) {
          debugPrint('JPush onReceiveMessage messageId=${message['extras']?['message_id']}');
        }
        await _handleIncomingMessage(message, showLocalNotification: true);
      },
    );
  }

  Future<void> _handleIncomingMessage(
    Map<String, dynamic> message, {
    required bool showLocalNotification,
  }) async {
    final pushMessage = _extractPushMessage(message);
    if (pushMessage == null) {
      return;
    }

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
    final extras = message['extras'];
    if (extras is! Map) {
      return null;
    }

    return PushMessage.fromJson(Map<String, dynamic>.from(extras));
  }

  Future<String?> getRegistrationID() async {
    return _jpush.getRegistrationID();
  }
}
