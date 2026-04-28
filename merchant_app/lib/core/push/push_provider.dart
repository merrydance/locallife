import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/push/device_sync_service.dart';
import 'package:merchant_app/core/service/message_dedup.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/core/push/native_push_manager.dart';

final messageDeduplicatorProvider = Provider((ref) => MessageDeduplicator());

final pushManagerProvider = Provider((ref) {
  final deduplicator = ref.watch(messageDeduplicatorProvider);
  return NativePushManager(deduplicator);
});

final deviceSyncServiceProvider = Provider<DeviceSyncService>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final pushManager = ref.watch(pushManagerProvider);
  return DeviceSyncService(apiClient, pushManager);
});

final deviceSyncManagerProvider = Provider<void>((ref) {
  final authState = ref.watch(authProvider);
  if (authState.isAuthenticated) {
    ref.read(deviceSyncServiceProvider).ensureRegistered();
  }
});
