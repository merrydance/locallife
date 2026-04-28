import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:merchant_app/core/audio/sound_player.dart';
import 'package:merchant_app/core/audio/tts_service.dart';
import 'package:merchant_app/core/push/local_notification_service.dart';
import 'package:merchant_app/core/service/navigation_service.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/order/order_alert_page.dart';
import 'package:merchant_app/features/order/order_detail_page.dart';
import 'package:merchant_app/features/order/order_provider.dart';
import 'package:merchant_app/features/printer/printer_provider.dart';
import 'package:merchant_app/features/settings/notification_settings_provider.dart';
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/models/push_message.dart';

final localNotificationServiceProvider = Provider<LocalNotificationService>((ref) {
  return LocalNotificationService();
});

final orderAlertCoordinatorProvider = Provider<OrderAlertCoordinator>((ref) {
  return OrderAlertCoordinator(ref);
});

class OrderAlertCoordinator {
  OrderAlertCoordinator(this._ref);

  final Ref _ref;
  final Set<String> _presentingOrderIds = <String>{};

  Future<void> handleIncomingOrder(
    PushMessage message, {
    required bool showLocalNotification,
  }) async {
    if (showLocalNotification) {
      await _ref
          .read(localNotificationServiceProvider)
          .showNewOrderNotification(message);
    }

    final notificationSettings = _ref.read(notificationSettingsProvider);
    if (notificationSettings.soundEnabled) {
      await SoundPlayer.playNewOrderAlert();
    }
    if (notificationSettings.voiceEnabled) {
      await TtsService.speakOrderAlert(
        message.displayOrderNumber,
        message.amount,
      );
    }

    if (notificationSettings.autoAcceptEnabled) {
      final accepted = await _acceptAndPrint(message);
      if (accepted) {
        return;
      }
    }

    _presentAlert(message);
  }

  Future<void> handleNotificationTap(PushMessage message) async {
    await _ref.read(orderProvider.notifier).fetchOrders();

    final order = _ref.read(orderProvider).orders.cast<OrderModel?>().firstWhere(
          (candidate) => candidate?.id == message.orderId,
          orElse: () => null,
        );

    if (order == null) {
      _presentAlert(message);
      return;
    }

    if (order.status == OrderStatus.pending) {
      final pendingMessage = message.withOrderNumber(
        order.orderNum.isNotEmpty ? order.orderNum : message.orderNumber,
      );
      _presentAlert(pendingMessage);
      return;
    }

    final navigator = rootNavigatorKey.currentState;
    final context = navigator?.context;
    if (navigator == null || context == null) {
      return;
    }

    navigator.push(
      MaterialPageRoute(
        builder: (_) => OrderDetailPage(order: order),
      ),
    );
  }

  Future<void> handlePolledOrders({
    required List<OrderModel> previousOrders,
    required List<OrderModel> latestOrders,
  }) async {
    final newlyPendingOrders = identifyNewPendingOrders(
      previousOrders: previousOrders,
      latestOrders: latestOrders,
    );

    for (final order in newlyPendingOrders) {
      final message = PushMessage.fromOrder(
        order,
        shopName: _merchantName,
      );
      await handleIncomingOrder(message, showLocalNotification: true);
    }
  }

  static List<OrderModel> identifyNewPendingOrders({
    required List<OrderModel> previousOrders,
    required List<OrderModel> latestOrders,
  }) {
    final previousPendingIds = previousOrders
        .where((order) => order.status == OrderStatus.pending)
        .map((order) => order.id)
        .toSet();

    final latestPendingOrders = latestOrders
        .where((order) => order.status == OrderStatus.pending)
        .toList()
      ..sort((left, right) => left.createdAt.compareTo(right.createdAt));

    return latestPendingOrders
        .where((order) => !previousPendingIds.contains(order.id))
        .toList(growable: false);
  }

  Future<bool> _acceptAndPrint(PushMessage message) async {
    final accepted = await _ref
        .read(orderProvider.notifier)
        .acceptOrder(message.orderId);
    if (!accepted) {
      return false;
    }

    await _ref.read(orderProvider.notifier).fetchOrders();

    final notificationSettings = _ref.read(notificationSettingsProvider);
    final printerState = _ref.read(printerProvider);
    if (notificationSettings.autoPrintAfterAcceptEnabled &&
        printerState.connectedDevice != null) {
      await _ref.read(printerProvider.notifier).printReceipt(message);
    }

    return true;
  }

  void _presentAlert(PushMessage message) {
    if (_presentingOrderIds.contains(message.orderId)) {
      return;
    }

    final navigator = rootNavigatorKey.currentState;
    if (navigator == null) {
      return;
    }

    _presentingOrderIds.add(message.orderId);
    unawaited(
      navigator
          .push(
            MaterialPageRoute(
              builder: (_) => OrderAlertPage(message: message),
              fullscreenDialog: true,
            ),
          )
          .whenComplete(() => _presentingOrderIds.remove(message.orderId)),
    );
  }

  String get _merchantName {
    final merchantName = _ref.read(authProvider).merchantName;
    if (merchantName == null || merchantName.trim().isEmpty) {
      return '商户工作台';
    }
    return merchantName.trim();
  }
}