import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/push/device_sync_service.dart';
import 'package:merchant_app/core/service/message_dedup.dart';
import 'package:merchant_app/core/push/push_manager.dart';

final messageDeduplicatorProvider = Provider((ref) => MessageDeduplicator());

final pushManagerProvider = Provider((ref) {
  final deduplicator = ref.watch(messageDeduplicatorProvider);
  return PushManager(deduplicator);
});

final deviceSyncServiceProvider = Provider<DeviceSyncService>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  final pushManager = ref.watch(pushManagerProvider);
  return DeviceSyncService(apiClient, pushManager);
});
