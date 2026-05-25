import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/network/ws_client.dart';
import 'package:merchant_app/core/push/push_provider.dart';
import 'package:merchant_app/core/service/foreground_service.dart';
import 'package:merchant_app/core/network/connectivity_provider.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/order/working_status_provider.dart';
import 'package:merchant_app/features/table/providers/table_provider.dart';

final wsStatusProvider = StateProvider<bool>((ref) => false);

final wsClientProvider = Provider((ref) {
  final deduplicator = ref.watch(messageDeduplicatorProvider);
  final client = WsClient(deduplicator);

  client.onStatusChange = (isConnected) {
    ref.read(wsStatusProvider.notifier).state = isConnected;
  };

  client.onTableStatusChange = (data) {
    ref.read(tableProvider.notifier).updateTableFromWebSocket(data);
  };

  client.onAuthenticationFailure = () async {
    final tokens = await ref.read(authProvider.notifier).refreshSession();
    return tokens?['accessToken'];
  };

  // Clean up on dispose
  ref.onDispose(() => client.dispose());

  return client;
});

// Watcher to manage WS connection based on Auth state and Working Status
final wsConnectionManagerProvider = Provider((ref) {
  final authState = ref.watch(authProvider);
  final wsClient = ref.watch(wsClientProvider);
  final isWorking = ref.watch(
    workingStatusProvider.select((state) => state.isOnline),
  );
  final hasNetwork = ref.watch(connectivityProvider).value ?? false;
  final deviceSyncService = ref.watch(deviceSyncServiceProvider);

  if (authState.isAuthenticated &&
      authState.accessToken != null &&
      isWorking &&
      hasNetwork) {
    wsClient.connect(authState.accessToken!);
    MerchantForegroundService.start();
    deviceSyncService.ensureRegistered();
  } else {
    wsClient.disconnect();
    MerchantForegroundService.stop();
  }

  return null;
});
