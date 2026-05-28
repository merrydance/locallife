import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/push/device_sync_service.dart';
import 'package:merchant_app/core/push/native_push_manager.dart';

void main() {
  test('device registration payload includes native push provider', () async {
    final apiClient = _FakeApiClient();
    final pushManager = _FakeNativePushManager(
      registrationId: 'push-token-1',
      provider: 'xiaomi',
    );
    final service = DeviceSyncService(
      apiClient,
      pushManager,
      deviceIdProvider: () async => 'device-1',
      registeredTokenReader: () async => null,
      registeredTokenWriter: (_) async {},
    );

    await service.ensureRegistered();

    expect(apiClient.lastPostPath, '/merchant/device/register');
    expect(apiClient.lastPostData, containsPair('device_id', 'device-1'));
    expect(apiClient.lastPostData, containsPair('push_token', 'push-token-1'));
    expect(apiClient.lastPostData, containsPair('provider', 'xiaomi'));
    expect(service.state.nativePushStatus, NativePushStatus.registered);
    expect(
      service.state.deviceRegistrationStatus,
      DeviceRegistrationStatus.success,
    );
    expect(service.state.degradationMessages, isEmpty);
  });

  test('missing push token is visible as degraded state', () async {
    final apiClient = _FakeApiClient();
    final pushManager = _FakeNativePushManager(
      registrationId: null,
      provider: 'xiaomi',
    );
    final service = DeviceSyncService(
      apiClient,
      pushManager,
      deviceIdProvider: () async => 'device-1',
      registeredTokenReader: () async => null,
      registeredTokenWriter: (_) async {},
    );

    await service.ensureRegistered();

    expect(apiClient.lastPostPath, isNull);
    expect(service.state.nativePushStatus, NativePushStatus.missingToken);
    expect(
      service.state.deviceRegistrationStatus,
      DeviceRegistrationStatus.missingToken,
    );
    expect(
      service.state.degradationMessages,
      contains('厂商推送 Token 未获取，杀进程后可能收不到新订单'),
    );
  });

  test('device registration failure is visible as degraded state', () async {
    final apiClient = _FakeApiClient(failPost: true);
    final pushManager = _FakeNativePushManager(
      registrationId: 'push-token-1',
      provider: 'oppo',
    );
    final service = DeviceSyncService(
      apiClient,
      pushManager,
      deviceIdProvider: () async => 'device-1',
      registeredTokenReader: () async => null,
      registeredTokenWriter: (_) async {},
    );

    await service.ensureRegistered();

    expect(service.state.nativePushStatus, NativePushStatus.registered);
    expect(
      service.state.deviceRegistrationStatus,
      DeviceRegistrationStatus.failure,
    );
    expect(service.state.degradationMessages, contains('设备注册失败，后台推送路由可能不可用'));
  });

  test(
    'native push initialization failure is visible as degraded state',
    () async {
      final apiClient = _FakeApiClient();
      final pushManager = _FakeNativePushManager(
        registrationId: null,
        provider: 'xiaomi',
        initializationFailure: '小米推送初始化失败',
      );
      final service = DeviceSyncService(
        apiClient,
        pushManager,
        deviceIdProvider: () async => 'device-1',
        registeredTokenReader: () async => null,
        registeredTokenWriter: (_) async {},
      );

      await service.ensureRegistered();

      expect(apiClient.lastPostPath, isNull);
      expect(service.state.nativePushStatus, NativePushStatus.failed);
      expect(
        service.state.deviceRegistrationStatus,
        DeviceRegistrationStatus.failure,
      );
      expect(
        service.state.degradationMessages,
        contains('厂商推送初始化失败，请检查系统通知和保活设置'),
      );
    },
  );

  test('heartbeat failure and recovery update visible state', () async {
    final apiClient = _FakeApiClient(failPut: true);
    final pushManager = _FakeNativePushManager(
      registrationId: 'push-token-1',
      provider: 'vivo',
    );
    final service = DeviceSyncService(
      apiClient,
      pushManager,
      deviceIdProvider: () async => 'device-1',
      registeredTokenReader: () async => null,
      registeredTokenWriter: (_) async {},
    );

    await service.sendHeartbeat();

    expect(service.state.heartbeatStatus, DeviceHeartbeatStatus.failure);
    expect(service.state.degradationMessages, contains('设备心跳失败，后台可能误判商户离线'));

    apiClient.failPut = false;
    await service.sendHeartbeat();

    expect(service.state.heartbeatStatus, DeviceHeartbeatStatus.success);
    expect(service.state.degradationMessages, isEmpty);
  });
}

class _FakeNativePushManager extends NativePushManager {
  _FakeNativePushManager({
    required this.registrationId,
    required this.provider,
    this.initializationFailure,
  }) : super();

  final String? registrationId;
  final String? provider;
  final String? initializationFailure;

  @override
  Future<String?> getRegistrationID() async => registrationId;

  @override
  Future<String?> getRegistrationProvider() async => provider;

  @override
  Future<String?> getInitializationFailure() async => initializationFailure;
}

class _FakeApiClient implements ApiClient {
  _FakeApiClient({this.failPost = false, this.failPut = false});

  bool failPost;
  bool failPut;
  String? lastPostPath;
  Map<String, dynamic>? lastPostData;

  @override
  Future<Response<dynamic>> post(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) async {
    lastPostPath = path;
    lastPostData = Map<String, dynamic>.from(data as Map);
    if (failPost) {
      throw DioException(
        requestOptions: RequestOptions(path: path),
        type: DioExceptionType.connectionError,
      );
    }
    return Response<dynamic>(
      requestOptions: RequestOptions(path: path),
      data: const <String, dynamic>{'code': 0, 'message': 'ok'},
    );
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
    throw UnimplementedError();
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
  Future<Response<dynamic>> put(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) {
    if (failPut) {
      throw DioException(
        requestOptions: RequestOptions(path: path),
        type: DioExceptionType.connectionError,
      );
    }
    return Future<Response<dynamic>>.value(
      Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: const <String, dynamic>{'code': 0, 'message': 'ok'},
      ),
    );
  }

  @override
  Future<Map<String, String?>?> refreshSessionTokens() {
    throw UnimplementedError();
  }
}
