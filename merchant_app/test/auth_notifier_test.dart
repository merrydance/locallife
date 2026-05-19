import 'dart:async';

import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/service/auth_session_controller.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/auth/auth_service.dart';

void main() {
  test(
    'loginWithBindingCode ignores duplicate submits while binding is in flight',
    () async {
      final authService = _FakeAuthService();
      final notifier = AuthNotifier(authService, AuthSessionController());

      authService.completeAutoLogin();
      await authService.autoLoginCompleted;
      await Future<void>.delayed(Duration.zero);

      final firstLogin = notifier.loginWithBindingCode('123456');
      final secondLogin = notifier.loginWithBindingCode('123456');

      expect(authService.verifyCalls, 1);

      authService.completeVerify();
      await Future.wait([firstLogin, secondLogin]);
    },
  );

  test(
    'loginWithBindingCode submits even while startup auth check is in flight',
    () async {
      final authService = _FakeAuthService();
      final notifier = AuthNotifier(authService, AuthSessionController());

      await authService.autoLoginCompleted;

      final login = notifier.loginWithBindingCode('123456');

      expect(authService.verifyCalls, 1);

      authService.completeAutoLogin();
      authService.completeVerify();
      await login;
    },
  );

  test('updates in-memory tokens when api refresh succeeds', () async {
    final sessionController = AuthSessionController();
    final authService = _FakeAuthService();
    final notifier = AuthNotifier(authService, sessionController);

    authService.completeAutoLogin({
      'accessToken': 'old-access-token',
      'refreshToken': 'old-refresh-token',
      'merchantName': '测试商户',
    });
    await authService.autoLoginCompleted;
    await Future<void>.delayed(Duration.zero);

    sessionController.updateTokens(
      accessToken: 'fresh-access-token',
      refreshToken: 'fresh-refresh-token',
    );

    expect(notifier.state.accessToken, 'fresh-access-token');
    expect(notifier.state.refreshToken, 'fresh-refresh-token');
    expect(notifier.state.merchantName, '测试商户');
    expect(notifier.state.isAuthenticated, isTrue);
  });

  test('shares in-flight manual refresh requests', () async {
    final authService = _FakeAuthService();
    final notifier = AuthNotifier(authService, AuthSessionController());

    authService.completeAutoLogin({
      'accessToken': 'old-access-token',
      'refreshToken': 'old-refresh-token',
      'merchantName': '测试商户',
    });
    await authService.autoLoginCompleted;
    await Future<void>.delayed(Duration.zero);

    final firstRefresh = notifier.refreshSession();
    final secondRefresh = notifier.refreshSession();

    expect(authService.refreshCalls, 1);

    authService.completeRefresh();
    final results = await Future.wait([firstRefresh, secondRefresh]);

    expect(results[0]?['accessToken'], 'fresh-access-token');
    expect(results[1]?['accessToken'], 'fresh-access-token');
  });
}

class _FakeAuthService implements AuthService {
  final Completer<void> _autoLoginCompleter = Completer<void>();
  final Completer<Map<String, String?>?> _autoLoginResultCompleter =
      Completer<Map<String, String?>?>();
  final Completer<Map<String, dynamic>> _verifyCompleter =
      Completer<Map<String, dynamic>>();
  final Completer<Map<String, dynamic>> _refreshCompleter =
      Completer<Map<String, dynamic>>();
  int verifyCalls = 0;
  int refreshCalls = 0;

  Future<void> get autoLoginCompleted => _autoLoginCompleter.future;

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

  void completeRefresh() {
    if (!_refreshCompleter.isCompleted) {
      _refreshCompleter.complete({
        'access_token': 'fresh-access-token',
        'refresh_token': 'fresh-refresh-token',
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
    refreshCalls++;
    return _refreshCompleter.future;
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
    verifyCalls++;
    return _verifyCompleter.future;
  }
}
