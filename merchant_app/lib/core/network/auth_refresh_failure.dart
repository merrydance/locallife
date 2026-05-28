import 'package:dio/dio.dart';

enum AuthRefreshFailureKind { recoverable, invalidSession }

class AuthRefreshRecoverableException implements Exception {
  const AuthRefreshRecoverableException([this.message = '登录状态暂未确认，网络恢复后会自动重试']);

  final String message;

  @override
  String toString() => message;
}

class AuthRefreshInvalidSessionException implements Exception {
  const AuthRefreshInvalidSessionException([this.message = '登录状态已失效，请重新绑定']);

  final String message;

  @override
  String toString() => message;
}

AuthRefreshFailureKind classifyRefreshFailure(DioException error) {
  if (error.type == DioExceptionType.badResponse) {
    final statusCode = error.response?.statusCode;
    if (statusCode == 401 || statusCode == 403) {
      return AuthRefreshFailureKind.invalidSession;
    }
    if (statusCode != null && statusCode >= 500) {
      return AuthRefreshFailureKind.recoverable;
    }
    return AuthRefreshFailureKind.invalidSession;
  }

  return AuthRefreshFailureKind.recoverable;
}
