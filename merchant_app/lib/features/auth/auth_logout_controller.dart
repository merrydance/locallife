import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/push/push_provider.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';

final authLogoutControllerProvider = Provider<AuthLogoutController>((ref) {
  return AuthLogoutController(
    unregisterCurrentDevice: ref
        .watch(deviceSyncServiceProvider)
        .unregisterCurrentDevice,
    clearAuthSession: ref.read(authProvider.notifier).logout,
  );
});

class AuthLogoutController {
  AuthLogoutController({
    required Future<void> Function() unregisterCurrentDevice,
    required Future<void> Function() clearAuthSession,
  }) : _unregisterCurrentDevice = unregisterCurrentDevice,
       _clearAuthSession = clearAuthSession;

  final Future<void> Function() _unregisterCurrentDevice;
  final Future<void> Function() _clearAuthSession;

  Future<void> logout() async {
    try {
      await _unregisterCurrentDevice();
    } catch (error) {
      if (kDebugMode) {
        debugPrint(
          'Failed to unregister merchant app device before logout: $error',
        );
      }
    } finally {
      await _clearAuthSession();
    }
  }
}
