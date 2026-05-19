import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:merchant_app/core/audio/sound_player.dart';
import 'package:merchant_app/core/audio/order_audio_alert.dart';
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

final localNotificationServiceProvider = Provider<LocalNotificationService>((
  ref,
) {
  return LocalNotificationService();
});

final orderAlertCoordinatorProvider = Provider<OrderAlertCoordinator>((ref) {
  return OrderAlertCoordinator(ref);
});

class OrderAlertCoordinator {
  OrderAlertCoordinator(this._ref);

  final OrderAudioAlert _orderAudioAlert = OrderAudioAlert();
  final Ref _ref;
  final Set<String> _presentingOrderIds = <String>{};

  Future<void> handleIncomingOrder(
    PushMessage message, {
    required bool showLocalNotification,
  }) async {
    final hydratedMessage = await _hydrateIncomingOrder(message);

    if (showLocalNotification) {
      await _ref
          .read(localNotificationServiceProvider)
          .showNewOrderNotification(hydratedMessage);
    }

    final notificationSettings = _ref.read(notificationSettingsProvider);
    await _orderAudioAlert.runWithAlertVolume(() async {
      if (notificationSettings.soundEnabled) {
        await SoundPlayer.playNewOrderAlert();
      }
      if (notificationSettings.voiceEnabled) {
        await TtsService.speakOrderAlert(
          hydratedMessage.displayOrderNumber,
          hydratedMessage.amount,
        );
      }
    });

    if (notificationSettings.autoAcceptEnabled) {
      final accepted = await _acceptAndPrint(hydratedMessage);
      if (accepted) {
        return;
      }
    }

    _presentAlert(hydratedMessage);
  }

  Future<PushMessage> _hydrateIncomingOrder(PushMessage message) async {
    if (message.items.isNotEmpty || message.itemsLoadFailed) {
      _ref
          .read(orderProvider.notifier)
          .addOrUpdateOrder(
            OrderModel(
              id: message.orderId,
              orderNum: message.orderNumber,
              amount: message.amount,
              status: OrderStatus.paid,
              createdAt: message.timestamp,
              items: message.items,
              note: message.note,
              itemsLoadFailed: message.itemsLoadFailed,
            ),
          );
    }

    final order = await _ref
        .read(orderProvider.notifier)
        .fetchOrderDetail(message.orderId);
    return order == null ? message : message.withOrderSnapshot(order);
  }

  Future<void> handleNotificationTap(PushMessage message) async {
    final hydratedOrder = await _ref
        .read(orderProvider.notifier)
        .fetchOrderDetail(message.orderId);
    if (hydratedOrder != null) {
      if (hydratedOrder.isAwaitingAcceptance) {
        _presentAlert(message.withOrderSnapshot(hydratedOrder));
        return;
      }

      _presentOrderDetail(hydratedOrder);
      return;
    }

    await _ref.read(orderProvider.notifier).fetchOrders();

    final order = _ref
        .read(orderProvider)
        .orders
        .cast<OrderModel?>()
        .firstWhere(
          (candidate) => candidate?.id == message.orderId,
          orElse: () => null,
        );

    if (order == null) {
      _presentAlert(message);
      return;
    }

    if (order.isAwaitingAcceptance) {
      _presentAlert(message.withOrderSnapshot(order));
      return;
    }

    _presentOrderDetail(order);
  }

  void _presentOrderDetail(OrderModel order) {
    final navigator = rootNavigatorKey.currentState;
    final context = navigator?.context;
    if (navigator == null || context == null) {
      return;
    }

    navigator.push(
      MaterialPageRoute(builder: (_) => OrderDetailPage(order: order)),
    );
  }

  Future<void> handlePolledOrders({
    required List<OrderModel> previousOrders,
    required List<OrderModel> latestOrders,
  }) async {
    final newlyAwaitingAcceptanceOrders = identifyNewAwaitingAcceptanceOrders(
      previousOrders: previousOrders,
      latestOrders: latestOrders,
    );

    for (final order in newlyAwaitingAcceptanceOrders) {
      final message = PushMessage.fromOrder(order, shopName: _merchantName);
      await handleIncomingOrder(message, showLocalNotification: true);
    }
  }

  static List<OrderModel> identifyNewAwaitingAcceptanceOrders({
    required List<OrderModel> previousOrders,
    required List<OrderModel> latestOrders,
  }) {
    final previousAwaitingAcceptanceIds = previousOrders
        .where((order) => order.isAwaitingAcceptance)
        .map((order) => order.id)
        .toSet();

    final latestAwaitingAcceptanceOrders =
        latestOrders.where((order) => order.isAwaitingAcceptance).toList()
          ..sort((left, right) => left.createdAt.compareTo(right.createdAt));

    return latestAwaitingAcceptanceOrders
        .where((order) => !previousAwaitingAcceptanceIds.contains(order.id))
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
