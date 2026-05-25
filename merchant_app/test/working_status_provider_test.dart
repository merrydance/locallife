import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/features/order/working_status_provider.dart';

void main() {
  test(
    'syncFromBackend uses merchant open status from backend response',
    () async {
      final apiClient = _FakeApiClient(initialIsOpen: false);
      final notifier = WorkingStatusNotifier(apiClient);

      await notifier.syncFromBackend();

      expect(apiClient.lastGetPath, '/merchants/me/status');
      expect(notifier.state.isOnline, isFalse);
      expect(notifier.state.isLoading, isFalse);
      expect(notifier.state.hasConfirmedState, isTrue);
    },
  );

  test(
    'setStatus patches backend and applies returned merchant open status',
    () async {
      final apiClient = _FakeApiClient(
        initialIsOpen: false,
        patchResponseIsOpen: false,
      );
      final notifier = WorkingStatusNotifier(apiClient);

      final updated = await notifier.setStatus(true);

      expect(updated, isFalse);
      expect(apiClient.lastPatchPath, '/merchants/me/status');
      expect(apiClient.lastPatchData, <String, dynamic>{'is_open': true});
      expect(notifier.state.isOnline, isFalse);
      expect(notifier.state.hasConfirmedState, isTrue);
    },
  );

  test('sync failure keeps merchant open status unconfirmed', () async {
    final apiClient = _ThrowingApiClient();
    final notifier = WorkingStatusNotifier(apiClient);

    await notifier.syncFromBackend();

    expect(notifier.state.hasConfirmedState, isFalse);
    expect(notifier.state.error, isNotNull);
    expect(notifier.state.isOnline, isFalse);
  });

  test('resetLocal drops stale in-flight status responses', () async {
    final apiClient = _DelayedApiClient();
    final notifier = WorkingStatusNotifier(apiClient);

    final sync = notifier.syncFromBackend();
    notifier.resetLocal();
    apiClient.completeGet(true);
    await sync;

    expect(notifier.state.hasConfirmedState, isFalse);
    expect(notifier.state.isOnline, isFalse);
  });
}

class _FakeApiClient implements ApiClient {
  _FakeApiClient({required this.initialIsOpen, bool? patchResponseIsOpen})
    : patchResponseIsOpen = patchResponseIsOpen ?? initialIsOpen;

  final bool initialIsOpen;
  final bool patchResponseIsOpen;
  String? lastGetPath;
  String? lastPatchPath;
  dynamic lastPatchData;

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    lastGetPath = path;
    return Response<dynamic>(
      requestOptions: RequestOptions(path: path),
      data: <String, dynamic>{
        'code': 0,
        'message': 'ok',
        'data': <String, dynamic>{
          'is_open': initialIsOpen,
          'message': initialIsOpen ? '店铺营业中' : '店铺已打烊',
        },
      },
    );
  }

  @override
  Future<Response<dynamic>> patch(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) async {
    lastPatchPath = path;
    lastPatchData = data;
    return Response<dynamic>(
      requestOptions: RequestOptions(path: path),
      data: <String, dynamic>{
        'code': 0,
        'message': 'ok',
        'data': <String, dynamic>{
          'is_open': patchResponseIsOpen,
          'message': patchResponseIsOpen ? '店铺已开始营业' : '店铺已打烊',
        },
      },
    );
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

class _ThrowingApiClient implements ApiClient {
  @override
  Future<Response<dynamic>> delete(String path, {bool requiresAuth = true}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    throw DioException(
      requestOptions: RequestOptions(path: path),
      error: 'boom',
      type: DioExceptionType.connectionError,
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
  Future<Map<String, String?>?> refreshSessionTokens() {
    throw UnimplementedError();
  }
}

class _DelayedApiClient implements ApiClient {
  final Completer<Response<dynamic>> _getCompleter =
      Completer<Response<dynamic>>();

  void completeGet(bool isOpen) {
    if (!_getCompleter.isCompleted) {
      _getCompleter.complete(
        Response<dynamic>(
          requestOptions: RequestOptions(path: '/merchants/me/status'),
          data: <String, dynamic>{
            'code': 0,
            'message': 'ok',
            'data': <String, dynamic>{
              'is_open': isOpen,
              'message': isOpen ? '店铺营业中' : '店铺已打烊',
            },
          },
        ),
      );
    }
  }

  @override
  Future<Response<dynamic>> delete(String path, {bool requiresAuth = true}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) {
    return _getCompleter.future;
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
  Future<Map<String, String?>?> refreshSessionTokens() {
    throw UnimplementedError();
  }
}
