import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/app.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/network/connectivity_provider.dart';
import 'package:merchant_app/core/push/push_provider.dart';
import 'package:merchant_app/core/service/auth_session_controller.dart';
import 'package:merchant_app/core/service/order_poller.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/auth/auth_service.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:dio/dio.dart';

void main() {
  testWidgets('successful bind redirects from bind page to home', (
    tester,
  ) async {
    final authService = _FakeAuthService();
    final sessionController = AuthSessionController();

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          authServiceProvider.overrideWithValue(authService),
          authSessionControllerProvider.overrideWithValue(sessionController),
          apiClientProvider.overrideWithValue(_FakeApiClient()),
          connectivityProvider.overrideWith((ref) => Stream<bool>.value(true)),
          deviceSyncManagerProvider.overrideWith((ref) {}),
          orderPollerManagerProvider.overrideWith((ref) {}),
        ],
        child: const _RouterHost(),
      ),
    );

    authService.completeAutoLogin();
    await tester.pumpAndSettle();

    expect(find.text('尚未绑定商户'), findsOneWidget);
    await tester.tap(find.text('立即绑定商户'));
    await tester.pumpAndSettle();
    expect(find.text('商户应用绑定'), findsOneWidget);

    await tester.enterText(find.byType(TextField), '123456');
    await tester.tap(find.text('立即绑定'));
    await tester.pump();
    authService.completeVerify();
    await tester.pumpAndSettle();

    expect(find.text('商户应用绑定'), findsNothing);
    expect(find.text('尚未绑定商户'), findsNothing);
  });
}

class _RouterHost extends ConsumerWidget {
  const _RouterHost();

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final router = ref.watch(routerProvider);
    return MaterialApp.router(routerConfig: router);
  }
}

class _FakeApiClient implements ApiClient {
  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    return Response<dynamic>(
      requestOptions: RequestOptions(path: path),
      data: const <String, dynamic>{
        'data': <String, dynamic>{'is_open': false, 'message': '店铺已打烊'},
      },
    );
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
  final Completer<void> _autoLoginCompleter = Completer<void>();
  final Completer<Map<String, String?>?> _autoLoginResultCompleter =
      Completer<Map<String, String?>?>();
  final Completer<Map<String, dynamic>> _verifyCompleter =
      Completer<Map<String, dynamic>>();

  void completeAutoLogin([Map<String, String?>? tokens]) {
    if (!_autoLoginResultCompleter.isCompleted) {
      _autoLoginResultCompleter.complete(tokens);
    }
  }

  void completeVerify() {
    if (!_verifyCompleter.isCompleted) {
      _verifyCompleter.complete({
        'access_token': 'access-token',
        'refresh_token': 'refresh-token',
        'user': const <String, dynamic>{'workbenches': <dynamic>[]},
      });
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
    if (!_autoLoginCompleter.isCompleted) {
      _autoLoginCompleter.complete();
    }
    return _autoLoginResultCompleter.future;
  }

  @override
  Future<Map<String, dynamic>> verifyBindingCode(String code) {
    return _verifyCompleter.future;
  }
}
