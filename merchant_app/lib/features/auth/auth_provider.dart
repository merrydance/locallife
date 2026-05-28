import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/service/auth_session_controller.dart';
import 'package:merchant_app/features/auth/auth_service.dart';
import 'package:merchant_app/features/auth/auth_state.dart';
import 'package:merchant_app/core/utils/error_handler.dart';

final authServiceProvider = Provider((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return AuthService(apiClient);
});

final authProvider = StateNotifierProvider<AuthNotifier, AuthState>((ref) {
  final authService = ref.watch(authServiceProvider);
  final sessionController = ref.watch(authSessionControllerProvider);
  return AuthNotifier(authService, sessionController);
});

class AuthNotifier extends StateNotifier<AuthState> {
  final AuthService _authService;
  final AuthSessionController _sessionController;
  Future<void>? _bindingLoginFuture;
  Future<Map<String, String?>?>? _refreshSessionFuture;
  bool _manualLoginStarted = false;

  AuthNotifier(this._authService, this._sessionController)
    : super(AuthState()) {
    _sessionController.addListener(_handleSessionInvalidation);
    _checkAuth();
  }

  Future<void> _checkAuth() async {
    state = state.copyWith(isLoading: true);
    try {
      final tokens = await _authService.tryAutoLogin();
      if (_manualLoginStarted || state.isAuthenticated) {
        return;
      }
      if (tokens != null && tokens['accessToken'] != null) {
        state = state.copyWith(
          accessToken: tokens['accessToken'],
          refreshToken: tokens['refreshToken'],
          merchantName: tokens['merchantName'],
          isAuthenticated: true,
          isLoading: false,
          isSessionDegraded: false,
          error: null,
        );
      } else {
        state = AuthState(isLoading: false, isAuthenticated: false);
      }
    } on AuthRefreshRecoverableException catch (e) {
      if (_manualLoginStarted || state.isAuthenticated) {
        return;
      }
      final tokens = await _authService.getTokens();
      final accessToken = tokens['accessToken'];
      final refreshToken = tokens['refreshToken'];
      if (accessToken != null &&
          accessToken.isNotEmpty &&
          refreshToken != null &&
          refreshToken.isNotEmpty) {
        state = state.copyWith(
          accessToken: accessToken,
          refreshToken: refreshToken,
          merchantName: tokens['merchantName'],
          isAuthenticated: true,
          isLoading: false,
          isSessionDegraded: true,
          error: e.message,
        );
      } else {
        state = AuthState(
          isAuthenticated: false,
          isLoading: false,
          error: e.message,
        );
      }
    } catch (e) {
      if (_manualLoginStarted || state.isAuthenticated) {
        return;
      }
      state = state.copyWith(
        isLoading: false,
        error: ErrorHandler.getErrorMessage(e),
      );
    }
  }

  void _handleSessionInvalidation() {
    final latestAccessToken = _sessionController.latestAccessToken;
    final latestRefreshToken = _sessionController.latestRefreshToken;
    if (latestAccessToken != null && latestRefreshToken != null) {
      state = state.copyWith(
        accessToken: latestAccessToken,
        refreshToken: latestRefreshToken,
        isAuthenticated: true,
        isLoading: false,
        isSessionDegraded: false,
        error: null,
      );
      _sessionController.clearTokenUpdate();
      return;
    }

    final reason = _sessionController.lastInvalidationReason;
    if (reason == null) {
      return;
    }

    state = AuthState(isAuthenticated: false, isLoading: false, error: reason);
    _sessionController.clearInvalidation();
  }

  Future<Map<String, String?>?> refreshSession() async {
    final existingRefresh = _refreshSessionFuture;
    if (existingRefresh != null) {
      return existingRefresh;
    }

    _refreshSessionFuture = _performRefreshSession();
    return _refreshSessionFuture!.whenComplete(
      () => _refreshSessionFuture = null,
    );
  }

  Future<Map<String, String?>?> _performRefreshSession() async {
    final refreshToken = state.refreshToken;
    if (refreshToken == null || refreshToken.isEmpty) {
      return null;
    }

    try {
      final data = await _authService.refreshToken(refreshToken);
      final accessToken = data['access_token']?.toString();
      final newRefreshToken = data['refresh_token']?.toString();

      if (accessToken == null || newRefreshToken == null) {
        await _authService.clearTokens();
        state = AuthState(
          isAuthenticated: false,
          isLoading: false,
          error: '登录状态已失效，请重新绑定',
        );
        return null;
      }

      await _authService.saveTokens(
        accessToken,
        newRefreshToken,
        merchantName: state.merchantName,
      );
      state = state.copyWith(
        accessToken: accessToken,
        refreshToken: newRefreshToken,
        isAuthenticated: true,
        isSessionDegraded: false,
        error: null,
      );

      return {
        'accessToken': accessToken,
        'refreshToken': newRefreshToken,
        'merchantName': state.merchantName,
      };
    } on AuthRefreshRecoverableException catch (e) {
      state = state.copyWith(
        isAuthenticated: true,
        isLoading: false,
        isSessionDegraded: true,
        error: e.message,
      );
      return null;
    } catch (_) {
      await _authService.clearTokens();
      state = AuthState(
        isAuthenticated: false,
        isLoading: false,
        error: '登录状态已失效，请重新绑定',
      );
      return null;
    }
  }

  Future<void> loginWithBindingCode(String code) async {
    if (state.isAuthenticated) {
      return;
    }

    final existingLogin = _bindingLoginFuture;
    if (existingLogin != null) {
      return existingLogin;
    }

    _manualLoginStarted = true;
    _bindingLoginFuture = _performLoginWithBindingCode(code);
    return _bindingLoginFuture!.whenComplete(() => _bindingLoginFuture = null);
  }

  Future<void> _performLoginWithBindingCode(String code) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final data = await _authService.verifyBindingCode(code);
      final accessToken = data['access_token'];
      final refreshToken = data['refresh_token'];
      final merchantName = _extractMerchantName(data['user']);

      if (accessToken != null && refreshToken != null) {
        await _authService.saveTokens(
          accessToken,
          refreshToken,
          merchantName: merchantName,
        );
        state = state.copyWith(
          accessToken: accessToken,
          refreshToken: refreshToken,
          merchantName: merchantName,
          isAuthenticated: true,
          isLoading: false,
          isSessionDegraded: false,
          error: null,
        );
      } else {
        state = state.copyWith(isLoading: false, error: '获取 Token 失败');
      }
    } catch (e) {
      state = state.copyWith(
        isLoading: false,
        error: ErrorHandler.getErrorMessage(e),
      );
    }
  }

  Future<void> logout() async {
    await _authService.clearTokens();
    state = AuthState();
  }

  @override
  void dispose() {
    _sessionController.removeListener(_handleSessionInvalidation);
    super.dispose();
  }

  String? _extractMerchantName(dynamic userData) {
    if (userData is! Map) {
      return null;
    }

    final workbenches = userData['workbenches'];
    if (workbenches is! List) {
      return null;
    }

    for (final workbench in workbenches) {
      if (workbench is! Map) {
        continue;
      }
      final merchantName = workbench['merchant_name']?.toString().trim();
      final workbenchId = workbench['id']?.toString();
      final hasMerchantBinding =
          workbench['merchant_id'] != null || workbenchId == 'merchant';
      if (hasMerchantBinding &&
          merchantName != null &&
          merchantName.isNotEmpty) {
        return merchantName;
      }
    }

    for (final workbench in workbenches) {
      if (workbench is! Map) {
        continue;
      }
      final merchantName = workbench['merchant_name']?.toString().trim();
      if (merchantName != null && merchantName.isNotEmpty) {
        return merchantName;
      }
    }

    return null;
  }
}
