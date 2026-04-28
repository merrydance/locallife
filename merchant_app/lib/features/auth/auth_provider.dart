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

  AuthNotifier(this._authService, this._sessionController) : super(AuthState()) {
    _sessionController.addListener(_handleSessionInvalidation);
    _checkAuth();
  }

  Future<void> _checkAuth() async {
    state = state.copyWith(isLoading: true);
    try {
      final tokens = await _authService.tryAutoLogin();
      if (tokens != null && tokens['accessToken'] != null) {
        state = state.copyWith(
          accessToken: tokens['accessToken'],
          refreshToken: tokens['refreshToken'],
          merchantName: tokens['merchantName'],
          isAuthenticated: true,
          isLoading: false,
        );
      } else {
        state = AuthState(isLoading: false, isAuthenticated: false);
      }
    } catch (e) {
      state = state.copyWith(isLoading: false, error: ErrorHandler.getErrorMessage(e));
    }
  }

  void _handleSessionInvalidation() {
    final reason = _sessionController.lastInvalidationReason;
    state = AuthState(
      isAuthenticated: false,
      isLoading: false,
      error: reason,
    );
    _sessionController.clearInvalidation();
  }

  Future<void> loginWithBindingCode(String code) async {
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
        );
      } else {
        state = state.copyWith(isLoading: false, error: '获取 Token 失败');
      }
    } catch (e) {
      state = state.copyWith(isLoading: false, error: ErrorHandler.getErrorMessage(e));
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
      final hasMerchantBinding = workbench['merchant_id'] != null || workbenchId == 'merchant';
      if (hasMerchantBinding && merchantName != null && merchantName.isNotEmpty) {
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
