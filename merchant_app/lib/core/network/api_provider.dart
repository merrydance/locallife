import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/service/auth_session_controller.dart';

final authSessionControllerProvider = Provider<AuthSessionController>((ref) {
  return AuthSessionController();
});

final apiClientProvider = Provider<ApiClient>((ref) {
  final sessionController = ref.watch(authSessionControllerProvider);
  return ApiClient(sessionController);
});
