import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/push/device_sync_service.dart';
import 'package:merchant_app/core/push/local_notification_service.dart';
import 'package:merchant_app/core/service/message_dedup.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/core/push/native_push_manager.dart';

final messageDeduplicatorProvider = Provider((ref) => MessageDeduplicator());

final localNotificationServiceProvider = Provider<LocalNotificationService>((
  ref,
) {
  return LocalNotificationService();
});

final pushManagerProvider = Provider((ref) {
  return NativePushManager();
});

final deviceSyncServiceProvider = Provider<DeviceSyncService>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final pushManager = ref.watch(pushManagerProvider);
  final service = DeviceSyncService(apiClient, pushManager);
  ref.onDispose(service.stateNotifier.dispose);
  return service;
});

final deviceSyncStateProvider = Provider<DeviceSyncState>((ref) {
  final service = ref.watch(deviceSyncServiceProvider);
  final notifier = service.stateNotifier;
  void listener() => ref.invalidateSelf();
  notifier.addListener(listener);
  ref.onDispose(() => notifier.removeListener(listener));
  return notifier.value;
});

final notificationPermissionProvider = StateProvider<bool?>((ref) => null);

final deviceSyncManagerProvider = Provider<void>((ref) {
  final authState = ref.watch(authProvider);
  if (authState.isAuthenticated) {
    ref.read(deviceSyncServiceProvider).ensureRegistered();
  }
});

final notificationPermissionManagerProvider = Provider<void>((ref) {
  Future<void>(() async {
    final granted = await ref
        .read(localNotificationServiceProvider)
        .ensureNotificationPermission();
    ref.read(notificationPermissionProvider.notifier).state = granted;
  });
});
