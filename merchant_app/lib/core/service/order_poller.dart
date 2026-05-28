import 'dart:async';
import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/network/connectivity_provider.dart';
import 'package:merchant_app/core/push/push_provider.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/order/order_alert_coordinator.dart';
import 'package:merchant_app/features/order/order_provider.dart';
import 'package:merchant_app/features/order/working_status_provider.dart';
import 'package:merchant_app/models/order.dart';

final orderPollerProvider = Provider<OrderPoller>((ref) {
  final poller = OrderPoller(ref);
  ref.onDispose(() => poller.dispose());
  return poller;
});

// A provider that just watches the necessary states and keeps the poller running
final orderPollerManagerProvider = Provider<void>((ref) {
  final authState = ref.watch(authProvider);
  final isWorking = ref.watch(
    workingStatusProvider.select((state) => state.isOnline),
  );
  final hasNetwork = ref.watch(connectivityProvider).value ?? false;

  final poller = ref.watch(orderPollerProvider);

  if (authState.isAuthenticated && isWorking && hasNetwork) {
    poller.start();
  } else {
    poller.stop();
  }
});

class OrderPoller {
  final Ref _ref;
  Timer? _timer;
  bool _isRunning = false;

  OrderPoller(this._ref);

  void start() {
    if (_isRunning) return;
    _isRunning = true;
    debugPrint('OrderPoller started. Polling every 30 seconds...');

    _timer = Timer.periodic(const Duration(seconds: 30), (_) {
      unawaited(pollOnce());
    });
  }

  void stop() {
    if (!_isRunning) return;
    _isRunning = false;
    debugPrint('OrderPoller stopped.');
    _timer?.cancel();
    _timer = null;
  }

  Future<void> pollOnce() async {
    final authState = _ref.read(authProvider);
    if (!authState.isAuthenticated || authState.accessToken == null) {
      debugPrint('OrderPoller: Skipping poll, not authenticated.');
      return;
    }

    try {
      await _ref.read(workingStatusProvider.notifier).syncFromBackend();

      final latestAuthState = _ref.read(authProvider);
      if (!latestAuthState.isAuthenticated ||
          latestAuthState.accessToken == null) {
        debugPrint(
          'OrderPoller: Skipping poll, authentication expired during status sync.',
        );
        return;
      }

      final workingStatusState = _ref.read(workingStatusProvider);
      if (!workingStatusState.isOnline) {
        debugPrint(
          'OrderPoller: Skipping order poll because merchant is closed.',
        );
        return;
      }

      await _ref.read(deviceSyncServiceProvider).sendHeartbeat();
      debugPrint('OrderPoller: Fetching latest orders (fallback).');
      final latestOrders = await fetchLatestAwaitingAcceptanceOrders(
        _ref.read(orderProvider.notifier),
      );
      await _ref
          .read(orderAlertCoordinatorProvider)
          .handleAwaitingAcceptanceBackfill(latestOrders);
    } catch (e) {
      debugPrint('OrderPoller error: $e');
    }
  }

  @visibleForTesting
  static Future<List<OrderModel>> fetchLatestAwaitingAcceptanceOrders(
    OrderNotifier orderNotifier,
  ) {
    return orderNotifier.fetchAwaitingAcceptanceOrders();
  }

  void dispose() {
    stop();
  }
}
