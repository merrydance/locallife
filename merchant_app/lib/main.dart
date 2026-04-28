import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/app.dart';
import 'package:merchant_app/core/audio/tts_service.dart';
import 'package:merchant_app/core/push/push_provider.dart';
import 'package:merchant_app/core/network/ws_provider.dart';
import 'package:merchant_app/core/service/order_poller.dart';
import 'package:merchant_app/core/service/foreground_service.dart';
import 'package:merchant_app/features/order/order_alert_coordinator.dart';

// Removed global key from here

class MerchantAppBootstrap extends ConsumerWidget {
  const MerchantAppBootstrap({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    ref.watch(wsConnectionManagerProvider);
    ref.watch(orderPollerManagerProvider);
    return const MerchantApp();
  }
}

void main() async {
  WidgetsFlutterBinding.ensureInitialized();

  // Initialize Services
  await TtsService.init();
  await MerchantForegroundService.init();

  final container = ProviderContainer();
  await container.read(messageDeduplicatorProvider).ensureInitialized();
  final localNotificationService = container.read(localNotificationServiceProvider);
  await localNotificationService.init();
  await localNotificationService.ensureNotificationPermission();

  // Initialize Push Manager
  final pushManager = container.read(pushManagerProvider);
  await pushManager.init();

  // Initialize WebSocket Manager
  final wsClient = container.read(wsClientProvider);
  final orderAlertCoordinator = container.read(orderAlertCoordinatorProvider);

  localNotificationService.onNotificationTap = orderAlertCoordinator.handleNotificationTap;

  // Handle new order push
  pushManager.onNewOrder = orderAlertCoordinator.handleIncomingOrder;
  pushManager.onNotificationOpened = orderAlertCoordinator.handleNotificationTap;

  // Handle new order WebSocket
  wsClient.onNewOrder = (message) {
    orderAlertCoordinator.handleIncomingOrder(
      message,
      showLocalNotification: true,
    );
  };

  runApp(
    UncontrolledProviderScope(
      container: container,
      child: const MerchantAppBootstrap(),
    ),
  );
}
