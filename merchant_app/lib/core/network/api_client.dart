import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:merchant_app/config/env.dart';
import 'package:merchant_app/core/service/auth_session_controller.dart';

Map<String, dynamic> extractApiResponseData(dynamic payload) {
  if (payload is Map<String, dynamic>) {
    final data = payload['data'];
    if (data is Map<String, dynamic>) {
      return data;
    }
    return payload;
  }
  return <String, dynamic>{};
}

class ApiClient {
  final Dio _dio;
  final Dio _refreshDio;
  final _storage = const FlutterSecureStorage();
  final AuthSessionController _sessionController;
  Future<Map<String, String?>?>? _refreshFuture;

  ApiClient(this._sessionController)
    : _dio = Dio(
        BaseOptions(
          baseUrl: Env.apiBaseUrl,
          connectTimeout: const Duration(seconds: 10),
          receiveTimeout: const Duration(seconds: 10),
          contentType: 'application/json',
        ),
      ),
      _refreshDio = Dio(
        BaseOptions(
          baseUrl: Env.apiBaseUrl,
          connectTimeout: const Duration(seconds: 10),
          receiveTimeout: const Duration(seconds: 10),
          contentType: 'application/json',
        ),
      ) {
    if (kDebugMode) {
      _dio.interceptors.add(
        LogInterceptor(requestBody: false, responseBody: false),
      );
    }

    _dio.interceptors.add(
      QueuedInterceptorsWrapper(
        onRequest: (options, handler) async {
          final requiresAuth = options.extra['requiresAuth'] != false;
          if (!requiresAuth) {
            return handler.next(options);
          }

          final accessToken = await _storage.read(key: 'access_token');
          if (accessToken != null) {
            options.headers['Authorization'] = 'Bearer $accessToken';
          }
          return handler.next(options);
        },
        onError: (e, handler) async {
          final statusCode = e.response?.statusCode;
          final requestOptions = e.requestOptions;
          final isRefreshRequest = requestOptions.path == '/auth/refresh';
          final alreadyRetried =
              requestOptions.extra['retriedAfterRefresh'] == true;
          final requiresAuth = requestOptions.extra['requiresAuth'] != false;

          if (statusCode == 401 &&
              requiresAuth &&
              !isRefreshRequest &&
              !alreadyRetried) {
            final refreshedTokens = await refreshSessionTokens();
            final refreshedAccessToken = refreshedTokens?['accessToken'];
            if (refreshedAccessToken != null) {
              requestOptions.headers['Authorization'] =
                  'Bearer $refreshedAccessToken';
              requestOptions.extra['retriedAfterRefresh'] = true;

              try {
                final retryResponse = await _dio.fetch(requestOptions);
                return handler.resolve(retryResponse);
              } on DioException catch (retryError) {
                return handler.next(retryError);
              }
            }
          }

          return handler.next(e);
        },
      ),
    );
  }

  Future<Map<String, String?>?> refreshSessionTokens() {
    _refreshFuture ??= _performRefresh();
    return _refreshFuture!.whenComplete(() => _refreshFuture = null);
  }

  Future<Map<String, String?>?> _performRefresh() async {
    final refreshToken = await _storage.read(key: 'refresh_token');
    if (refreshToken == null || refreshToken.isEmpty) {
      await _clearSession('缺少刷新令牌');
      return null;
    }

    try {
      final response = await _refreshDio.post(
        '/auth/refresh',
        data: {'refresh_token': refreshToken},
      );

      final data = extractApiResponseData(response.data);
      final accessToken = data['access_token']?.toString();
      final newRefreshToken = data['refresh_token']?.toString();

      if (accessToken == null || newRefreshToken == null) {
        await _clearSession('刷新登录状态失败');
        return null;
      }

      await _storage.write(key: 'access_token', value: accessToken);
      await _storage.write(key: 'refresh_token', value: newRefreshToken);
      _sessionController.updateTokens(
        accessToken: accessToken,
        refreshToken: newRefreshToken,
      );
      return {'accessToken': accessToken, 'refreshToken': newRefreshToken};
    } on DioException {
      await _clearSession('登录状态已失效，请重新绑定');
      return null;
    }
  }

  Future<void> _clearSession(String reason) async {
    await _storage.delete(key: 'access_token');
    await _storage.delete(key: 'refresh_token');
    await _storage.delete(key: 'merchant_name');
    _sessionController.invalidate(reason);
  }

  Future<Response> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) {
    return _dio.get(
      path,
      queryParameters: queryParameters,
      options: Options(extra: {'requiresAuth': requiresAuth}),
    );
  }

  Future<Response> post(String path, {dynamic data, bool requiresAuth = true}) {
    return _dio.post(
      path,
      data: data,
      options: Options(extra: {'requiresAuth': requiresAuth}),
    );
  }

  Future<Response> put(String path, {dynamic data, bool requiresAuth = true}) {
    return _dio.put(
      path,
      data: data,
      options: Options(extra: {'requiresAuth': requiresAuth}),
    );
  }

  Future<Response> patch(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    return _dio.patch(
      path,
      data: data,
      options: Options(extra: {'requiresAuth': requiresAuth}),
    );
  }

  Future<Response> delete(String path, {bool requiresAuth = true}) {
    return _dio.delete(
      path,
      options: Options(extra: {'requiresAuth': requiresAuth}),
    );
  }
}
