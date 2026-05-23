import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/service/auth_session_controller.dart';
import 'package:merchant_app/core/service/order_poller.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/auth/auth_service.dart';
import 'package:merchant_app/features/order/working_status_provider.dart';
import 'package:merchant_app/features/auth/auth_state.dart';

void main() {
  test('pollOnce stops after syncing merchant status to closed', () async {
    final apiClient = _FakeApiClient();
    final sessionController = AuthSessionController();
    final authService = _FakeAuthService();
    final container = ProviderContainer(
      overrides: [
        apiClientProvider.overrideWithValue(apiClient),
        authServiceProvider.overrideWithValue(authService),
        authSessionControllerProvider.overrideWithValue(sessionController),
        authProvider.overrideWith(
          (ref) =>
              AuthNotifier(authService, sessionController)
                ..state = AuthState(
                  accessToken: 'access-token',
                  refreshToken: 'refresh-token',
                  merchantName: '测试商户',
                  isAuthenticated: true,
                ),
        ),
      ],
    );
    addTearDown(container.dispose);

    final poller = container.read(orderPollerProvider);
    await poller.pollOnce();

    expect(apiClient.calls, <String>['GET /merchants/me/status']);
    expect(container.read(workingStatusProvider).isOnline, isFalse);
    expect(container.read(workingStatusProvider).hasConfirmedState, isTrue);
  });
}

class _FakeApiClient implements ApiClient {
  final List<String> calls = <String>[];

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    calls.add('GET $path');
    if (path == '/merchants/me/status') {
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: const <String, dynamic>{
          'code': 0,
          'message': 'ok',
          'data': <String, dynamic>{'is_open': false, 'message': '店铺已打烊'},
        },
      );
    }

    throw UnimplementedError('Unexpected GET $path');
  }

  @override
  Future<Response<dynamic>> patch(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> post(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> put(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> delete(String path, {bool requiresAuth = true}) {
    throw UnimplementedError();
  }

  @override
  Future<Map<String, String?>?> refreshSessionTokens() {
    throw UnimplementedError();
  }
}

class _FakeAuthService implements AuthService {
  @override
  Future<void> clearTokens() async {}

  @override
  Future<Map<String, String?>> getTokens() async => {
    'accessToken': null,
    'refreshToken': null,
    'merchantName': null,
  };

  @override
  Future<Map<String, dynamic>> refreshToken(String refreshToken) {
    throw UnimplementedError();
  }

  @override
  Future<void> saveTokens(
    String access,
    String refresh, {
    String? merchantName,
  }) async {}

  @override
  Future<Map<String, String?>?> tryAutoLogin() async {
    return <String, String?>{
      'accessToken': 'access-token',
      'refreshToken': 'refresh-token',
      'merchantName': '测试商户',
    };
  }

  @override
  Future<Map<String, dynamic>> verifyBindingCode(String code) {
    throw UnimplementedError();
  }
}
