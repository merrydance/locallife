import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/app.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/network/connectivity_provider.dart';
import 'package:merchant_app/core/push/push_provider.dart';
import 'package:merchant_app/core/service/auth_session_controller.dart';
import 'package:merchant_app/core/service/order_poller.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/auth/auth_service.dart';

void main() {
  testWidgets('merchant app syncs open status from backend on login', (
    tester,
  ) async {
    final authService = _FakeAuthService();
    final sessionController = AuthSessionController();
    final apiClient = _FakeApiClient();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authServiceProvider.overrideWithValue(authService),
          authSessionControllerProvider.overrideWithValue(sessionController),
          apiClientProvider.overrideWithValue(apiClient),
          connectivityProvider.overrideWith((ref) => Stream<bool>.value(true)),
          deviceSyncManagerProvider.overrideWith((ref) {}),
          orderPollerManagerProvider.overrideWith((ref) {}),
        ],
        child: const MerchantApp(),
      ),
    );

    authService.completeAutoLogin({
      'accessToken': 'access-token',
      'refreshToken': 'refresh-token',
      'merchantName': '测试商户',
    });
    await tester.pumpAndSettle();

    expect(apiClient.lastMerchantStatusPath, '/merchants/me/status');
    expect(find.textContaining('在线营业'), findsOneWidget);
  });
}

class _FakeApiClient implements ApiClient {
  String? lastMerchantStatusPath;
  String? lastOrdersPath;

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    if (path == '/merchants/me/status') {
      lastMerchantStatusPath = path;
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: const <String, dynamic>{
          'code': 0,
          'message': 'ok',
          'data': <String, dynamic>{'is_open': true, 'message': '店铺营业中'},
        },
      );
    }

    if (path == '/merchant/orders') {
      lastOrdersPath = path;
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: const <String, dynamic>{
          'code': 0,
          'message': 'ok',
          'data': <String, dynamic>{
            'orders': <Map<String, dynamic>>[],
            'total': 0,
            'page_id': 1,
            'page_size': 20,
          },
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
  final Completer<Map<String, String?>?> _autoLoginResultCompleter =
      Completer<Map<String, String?>?>();

  void completeAutoLogin([Map<String, String?>? tokens]) {
    if (!_autoLoginResultCompleter.isCompleted) {
      _autoLoginResultCompleter.complete(tokens);
    }
  }

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
    return _autoLoginResultCompleter.future;
  }

  @override
  Future<Map<String, dynamic>> verifyBindingCode(String code) {
    throw UnimplementedError();
  }
}
