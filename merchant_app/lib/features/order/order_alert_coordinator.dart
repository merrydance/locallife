import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import 'package:merchant_app/core/audio/sound_player.dart';
import 'package:merchant_app/core/audio/order_audio_alert.dart';
import 'package:merchant_app/core/audio/tts_service.dart';
import 'package:merchant_app/core/push/push_provider.dart';
import 'package:merchant_app/core/service/navigation_service.dart';
import 'package:merchant_app/core/service/message_dedup.dart';
import 'package:merchant_app/core/service/order_alert_checkpoint_store.dart';
import 'package:merchant_app/core/service/pending_order_alert_store.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/display_config/display_config_provider.dart';
import 'package:merchant_app/features/order/order_alert_page.dart';
import 'package:merchant_app/features/order/order_detail_page.dart';
import 'package:merchant_app/features/order/order_provider.dart';
import 'package:merchant_app/features/printer/printer_provider.dart';
import 'package:merchant_app/features/settings/notification_settings_provider.dart';
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/models/push_message.dart';

final orderAlertCoordinatorProvider = Provider<OrderAlertCoordinator>((ref) {
  return OrderAlertCoordinator(ref);
});

final orderAlertDisplayConfigTimeoutProvider = Provider<Duration>((ref) {
  return const Duration(seconds: 2);
});

final pendingOrderAlertDrainManagerProvider = Provider<void>((ref) {
  final manager = PendingOrderAlertDrainManager(ref);
  manager.start();
  ref.onDispose(manager.dispose);
});

class PendingOrderAlertDrainManager with WidgetsBindingObserver {
  PendingOrderAlertDrainManager(this._ref);

  final Ref _ref;
  bool _started = false;
  bool _disposed = false;

  void start() {
    if (_started) {
      return;
    }
    _started = true;
    WidgetsBinding.instance.addObserver(this);
    WidgetsBinding.instance.addPostFrameCallback((_) => _drain());
  }

  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.resumed) {
      _drain();
    }
  }

  void _drain() {
    if (_disposed) {
      return;
    }
    unawaited(_ref.read(orderAlertCoordinatorProvider).drainPendingAlerts());
    unawaited(
      _ref.read(orderAlertCoordinatorProvider).drainPendingNotificationTaps(),
    );
  }

  void dispose() {
    _disposed = true;
    WidgetsBinding.instance.removeObserver(this);
  }
}

class OrderAlertCoordinator {
  OrderAlertCoordinator(this._ref);

  final OrderAudioAlert _orderAudioAlert = OrderAudioAlert();
  final Ref _ref;
  final Set<String> _presentingOrderIds = <String>{};
  final Set<String> _processingOrderIds = <String>{};
  final List<PushMessage> _pendingNotificationTaps = <PushMessage>[];
  Future<void>? _pendingDrainFuture;
  Future<void>? _pendingTapDrainFuture;

  Future<void> handleIncomingOrder(
    PushMessage message, {
    required bool showLocalNotification,
  }) async {
    if (!_processingOrderIds.add(message.orderId)) {
      return;
    }

    try {
      await _handleIncomingOrder(
        message,
        showLocalNotification: showLocalNotification,
      );
    } finally {
      _processingOrderIds.remove(message.orderId);
    }
  }

  Future<void> _handleIncomingOrder(
    PushMessage message, {
    required bool showLocalNotification,
  }) async {
    final dedupKeys = _dedupKeysFor(message);
    final deduplicator = _ref.read(messageDeduplicatorProvider);
    if (!await deduplicator.isAccepted(dedupKeys)) {
      return;
    }
    final pendingStore = _ref.read(pendingOrderAlertStoreProvider);
    if (await pendingStore.hasPending(message)) {
      return;
    }

    final hydratedMessage = await _hydrateIncomingOrder(message);

    if (showLocalNotification) {
      try {
        await _ref
            .read(localNotificationServiceProvider)
            .showNewOrderNotification(hydratedMessage);
      } catch (error) {
        debugPrint('Failed to show new order notification: $error');
      }
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

    if (await _isBackendAutoAcceptEnabled()) {
      final accepted = await _acceptAndPrint(hydratedMessage);
      if (accepted) {
        await deduplicator.markAccepted(dedupKeys);
        await _markAlerted(hydratedMessage.orderId);
        return;
      }
    }

    final presented = await _presentAlert(hydratedMessage);
    if (presented) {
      await deduplicator.markAccepted(dedupKeys);
      await _markAlerted(hydratedMessage.orderId);
    } else {
      await pendingStore.save(hydratedMessage, source: 'incoming');
    }
  }

  Future<void> drainPendingAlerts() {
    _pendingDrainFuture ??= _drainPendingAlerts().whenComplete(() {
      _pendingDrainFuture = null;
    });
    return _pendingDrainFuture!;
  }

  Future<void> _drainPendingAlerts() async {
    final pendingStore = _ref.read(pendingOrderAlertStoreProvider);
    final pendingAlerts = await pendingStore.loadPendingAlerts();

    for (final pending in pendingAlerts) {
      final message = pending.message;
      if (await _hasAlerted(message.orderId)) {
        await pendingStore.remove(message);
        continue;
      }

      final hydratedMessage = await _hydrateIncomingOrder(message);
      final hydratedOrder = _ref
          .read(orderProvider)
          .orders
          .cast<OrderModel?>()
          .firstWhere(
            (candidate) => candidate?.id == hydratedMessage.orderId,
            orElse: () => null,
          );
      if (hydratedOrder != null && !hydratedOrder.isAwaitingAcceptance) {
        await pendingStore.remove(message);
        continue;
      }

      final presented = await _presentAlert(hydratedMessage);
      if (!presented) {
        return;
      }

      await _ref
          .read(messageDeduplicatorProvider)
          .markAccepted(_dedupKeysFor(hydratedMessage));
      await _markAlerted(hydratedMessage.orderId);
      await pendingStore.remove(message);
    }
  }

  Future<PushMessage> _hydrateIncomingOrder(PushMessage message) async {
    if (message.items.isNotEmpty || message.itemsLoadFailed) {
      _ref
          .read(orderProvider.notifier)
          .addOrUpdateOrder(
            OrderModel(
              id: message.orderId,
              orderNum: message.orderNumber,
              pickupCode: message.pickupCode,
              pickupCodeMasked: message.pickupCodeMasked,
              amount: message.amount,
              status: OrderStatus.paid,
              fulfillmentStatus: FulfillmentStatus.pendingKitchen,
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

  Future<bool> _isBackendAutoAcceptEnabled() async {
    try {
      final config = await _ref
          .read(orderDisplayConfigRepositoryProvider)
          .fetchDisplayConfig()
          .timeout(_ref.read(orderAlertDisplayConfigTimeoutProvider));
      return config.allowsAutoAcceptPaidOrders;
    } catch (error) {
      debugPrint(
        'Failed to load backend display config for auto accept: $error',
      );
      return false;
    }
  }

  Future<void> handleNotificationTap(PushMessage message) async {
    final hydratedOrder = await _ref
        .read(orderProvider.notifier)
        .fetchOrderDetail(message.orderId);
    if (hydratedOrder != null) {
      if (hydratedOrder.isAwaitingAcceptance) {
        final presented = await _presentAlert(
          message.withOrderSnapshot(hydratedOrder),
        );
        if (!presented) {
          _queueNotificationTap(message);
        }
        return;
      }

      if (!_presentOrderDetail(hydratedOrder)) {
        _queueNotificationTap(message);
      }
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
      final presented = await _presentAlert(message);
      if (!presented) {
        _queueNotificationTap(message);
      }
      return;
    }

    if (order.isAwaitingAcceptance) {
      final presented = await _presentAlert(message.withOrderSnapshot(order));
      if (!presented) {
        _queueNotificationTap(message);
      }
      return;
    }

    if (!_presentOrderDetail(order)) {
      _queueNotificationTap(message);
    }
  }

  Future<void> drainPendingNotificationTaps() {
    _pendingTapDrainFuture ??= _drainPendingNotificationTaps().whenComplete(() {
      _pendingTapDrainFuture = null;
    });
    return _pendingTapDrainFuture!;
  }

  Future<void> _drainPendingNotificationTaps() async {
    final pendingTaps = List<PushMessage>.from(_pendingNotificationTaps);
    _pendingNotificationTaps.clear();

    for (final message in pendingTaps) {
      await handleNotificationTap(message);
      if (_pendingNotificationTaps.isNotEmpty) {
        break;
      }
    }
  }

  void _queueNotificationTap(PushMessage message) {
    final orderId = message.orderId.trim();
    if (orderId.isEmpty) {
      return;
    }
    if (_pendingNotificationTaps.any((pending) => pending.orderId == orderId)) {
      return;
    }
    _pendingNotificationTaps.add(message);
  }

  bool _presentOrderDetail(OrderModel order) {
    final navigator = rootNavigatorKey.currentState;
    final context = navigator?.context;
    if (navigator == null || context == null) {
      return false;
    }

    navigator.push(
      MaterialPageRoute(builder: (_) => OrderDetailPage(order: order)),
    );
    return true;
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
      if (await _hasAlerted(order.id)) {
        continue;
      }
      final message = PushMessage.fromOrder(order, shopName: _merchantName);
      await handleIncomingOrder(message, showLocalNotification: true);
    }
  }

  Future<void> handleAwaitingAcceptanceBackfill(
    List<OrderModel> latestOrders,
  ) async {
    final latestAwaitingAcceptanceOrders =
        latestOrders.where((order) => order.isAwaitingAcceptance).toList()
          ..sort((left, right) => left.createdAt.compareTo(right.createdAt));

    for (final order in latestAwaitingAcceptanceOrders) {
      if (await _hasAlerted(order.id)) {
        continue;
      }
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
    if (!notificationSettings.autoPrintAfterAcceptEnabled) {
      return true;
    }

    final printerState = _ref.read(printerProvider);
    if (printerState.connectedDevice != null) {
      await _ref.read(printerProvider.notifier).printReceipt(message);
    }

    return true;
  }

  Future<bool> _presentAlert(PushMessage message) async {
    if (_presentingOrderIds.contains(message.orderId)) {
      return true;
    }

    final navigator = rootNavigatorKey.currentState;
    if (navigator == null) {
      return false;
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
    return true;
  }

  List<String> _dedupKeysFor(PushMessage message) {
    return <String>[
      if (message.messageId.trim().isNotEmpty)
        MessageDeduplicator.messageKey(message.messageId),
      if (message.orderId.trim().isNotEmpty)
        MessageDeduplicator.orderKey(message.orderId),
    ];
  }

  Future<bool> _hasAlerted(String orderId) {
    return _ref.read(orderAlertCheckpointStoreProvider).hasAlerted(orderId);
  }

  Future<void> _markAlerted(String orderId) {
    return _ref.read(orderAlertCheckpointStoreProvider).markAlerted(orderId);
  }

  String get _merchantName {
    final merchantName = _ref.read(authProvider).merchantName;
    if (merchantName == null || merchantName.trim().isEmpty) {
      return '商户工作台';
    }
    return merchantName.trim();
  }
}
