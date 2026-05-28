import 'dart:io';

import 'package:device_info_plus/device_info_plus.dart';
import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:package_info_plus/package_info_plus.dart';

import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/push/native_push_manager.dart';

enum NativePushStatus { unknown, noProvider, missingToken, registered, failed }

enum DeviceRegistrationStatus { unknown, missingToken, success, failure }

enum DeviceHeartbeatStatus { unknown, skipped, success, failure }

class DeviceSyncState {
  const DeviceSyncState({
    this.nativePushStatus = NativePushStatus.unknown,
    this.provider,
    this.deviceRegistrationStatus = DeviceRegistrationStatus.unknown,
    this.heartbeatStatus = DeviceHeartbeatStatus.unknown,
    this.lastRegistrationError,
    this.lastHeartbeatError,
    this.lastHeartbeatAt,
  });

  final NativePushStatus nativePushStatus;
  final String? provider;
  final DeviceRegistrationStatus deviceRegistrationStatus;
  final DeviceHeartbeatStatus heartbeatStatus;
  final String? lastRegistrationError;
  final String? lastHeartbeatError;
  final DateTime? lastHeartbeatAt;

  bool get isDegraded => degradationMessages.isNotEmpty;

  List<String> get degradationMessages {
    final messages = <String>[];
    if (nativePushStatus == NativePushStatus.noProvider) {
      messages.add('当前机型未启用厂商推送，杀进程后只能依赖系统通知点击和下次打开补单');
    } else if (nativePushStatus == NativePushStatus.missingToken) {
      messages.add('厂商推送 Token 未获取，杀进程后可能收不到新订单');
    } else if (nativePushStatus == NativePushStatus.failed) {
      messages.add('厂商推送初始化失败，请检查系统通知和保活设置');
    }
    if (deviceRegistrationStatus == DeviceRegistrationStatus.failure) {
      messages.add('设备注册失败，后台推送路由可能不可用');
    }
    if (heartbeatStatus == DeviceHeartbeatStatus.failure) {
      messages.add('设备心跳失败，后台可能误判商户离线');
    }
    return messages;
  }

  DeviceSyncState copyWith({
    NativePushStatus? nativePushStatus,
    Object? provider = _unset,
    DeviceRegistrationStatus? deviceRegistrationStatus,
    DeviceHeartbeatStatus? heartbeatStatus,
    Object? lastRegistrationError = _unset,
    Object? lastHeartbeatError = _unset,
    Object? lastHeartbeatAt = _unset,
  }) {
    return DeviceSyncState(
      nativePushStatus: nativePushStatus ?? this.nativePushStatus,
      provider: identical(provider, _unset)
          ? this.provider
          : provider as String?,
      deviceRegistrationStatus:
          deviceRegistrationStatus ?? this.deviceRegistrationStatus,
      heartbeatStatus: heartbeatStatus ?? this.heartbeatStatus,
      lastRegistrationError: identical(lastRegistrationError, _unset)
          ? this.lastRegistrationError
          : lastRegistrationError as String?,
      lastHeartbeatError: identical(lastHeartbeatError, _unset)
          ? this.lastHeartbeatError
          : lastHeartbeatError as String?,
      lastHeartbeatAt: identical(lastHeartbeatAt, _unset)
          ? this.lastHeartbeatAt
          : lastHeartbeatAt as DateTime?,
    );
  }
}

const Object _unset = Object();

class DeviceSyncService {
  DeviceSyncService(
    this._apiClient,
    this._pushManager, {
    Future<String> Function()? deviceIdProvider,
    Future<String?> Function()? registeredTokenReader,
    Future<void> Function(String token)? registeredTokenWriter,
  }) : _deviceIdProvider = deviceIdProvider,
       _registeredTokenReader = registeredTokenReader,
       _registeredTokenWriter = registeredTokenWriter {
    _pushManager.onTokenRegistered = (token, provider) => ensureRegistered();
    _pushManager.onInitializationFailed = (provider, message) async {
      _updateState(
        nativePushStatus: NativePushStatus.failed,
        provider: provider,
        deviceRegistrationStatus: DeviceRegistrationStatus.failure,
        lastRegistrationError: message,
      );
    };
  }

  static const _lastRegisteredPushTokenKey = 'last_registered_push_token';
  static const _deviceIdKey = 'device_uuid';

  final ApiClient _apiClient;
  final NativePushManager _pushManager;
  final Future<String> Function()? _deviceIdProvider;
  final Future<String?> Function()? _registeredTokenReader;
  final Future<void> Function(String token)? _registeredTokenWriter;
  final FlutterSecureStorage _storage = const FlutterSecureStorage();

  Future<void>? _registrationFuture;
  String? _lastRegisteredPushToken;
  final ValueNotifier<DeviceSyncState> stateNotifier =
      ValueNotifier<DeviceSyncState>(const DeviceSyncState());

  DeviceSyncState get state => stateNotifier.value;

  Future<void> ensureRegistered() {
    _registrationFuture ??= _performEnsureRegistered().whenComplete(
      () => _registrationFuture = null,
    );
    return _registrationFuture!;
  }

  Future<void> _performEnsureRegistered() async {
    final registrationId = await _pushManager.getRegistrationID();
    final initializationFailure = await _pushManager.getInitializationFailure();
    if (initializationFailure != null && initializationFailure.isNotEmpty) {
      final provider = await _pushManager.getRegistrationProvider();
      _updateState(
        nativePushStatus: NativePushStatus.failed,
        provider: provider,
        deviceRegistrationStatus: DeviceRegistrationStatus.failure,
        lastRegistrationError: initializationFailure,
      );
      return;
    }
    if (registrationId == null || registrationId.isEmpty) {
      final provider = await _pushManager.getRegistrationProvider();
      _updateState(
        nativePushStatus: provider == null || provider.trim().isEmpty
            ? NativePushStatus.noProvider
            : NativePushStatus.missingToken,
        provider: provider,
        deviceRegistrationStatus: DeviceRegistrationStatus.missingToken,
      );
      return;
    }
    final provider = await _pushManager.getRegistrationProvider();
    _updateState(
      nativePushStatus: NativePushStatus.registered,
      provider: provider,
    );

    _lastRegisteredPushToken ??= await _readLastRegisteredPushToken();
    if (_lastRegisteredPushToken == registrationId) {
      _updateState(
        deviceRegistrationStatus: DeviceRegistrationStatus.success,
        lastRegistrationError: null,
      );
      return;
    }

    final payload = await _buildDevicePayload(registrationId, provider);

    try {
      await _apiClient.post('/merchant/device/register', data: payload);
      await _writeLastRegisteredPushToken(registrationId);
      _lastRegisteredPushToken = registrationId;
      _updateState(
        deviceRegistrationStatus: DeviceRegistrationStatus.success,
        lastRegistrationError: null,
      );
    } on DioException catch (error) {
      if (_isEndpointNotReady(error)) {
        if (kDebugMode) {
          debugPrint('Device register endpoint not ready yet.');
        }
        _updateState(
          deviceRegistrationStatus: DeviceRegistrationStatus.failure,
          lastRegistrationError: '设备注册接口暂不可用',
        );
        return;
      }

      if (kDebugMode) {
        debugPrint('Failed to register device: ${error.message}');
      }
      _updateState(
        deviceRegistrationStatus: DeviceRegistrationStatus.failure,
        lastRegistrationError: error.message ?? '设备注册失败',
      );
    }
  }

  Future<void> sendHeartbeat() async {
    final registrationId = await _pushManager.getRegistrationID();
    final initializationFailure = await _pushManager.getInitializationFailure();
    if (initializationFailure != null && initializationFailure.isNotEmpty) {
      final provider = await _pushManager.getRegistrationProvider();
      _updateState(
        nativePushStatus: NativePushStatus.failed,
        provider: provider,
        heartbeatStatus: DeviceHeartbeatStatus.skipped,
        lastRegistrationError: initializationFailure,
      );
      return;
    }
    if (registrationId == null || registrationId.isEmpty) {
      final provider = await _pushManager.getRegistrationProvider();
      _updateState(
        nativePushStatus: provider == null || provider.trim().isEmpty
            ? NativePushStatus.noProvider
            : NativePushStatus.missingToken,
        provider: provider,
        heartbeatStatus: DeviceHeartbeatStatus.skipped,
      );
      return;
    }
    final provider = await _pushManager.getRegistrationProvider();
    _updateState(
      nativePushStatus: NativePushStatus.registered,
      provider: provider,
    );

    final payload = await _buildDevicePayload(registrationId, provider);
    try {
      await _apiClient.put('/merchant/device/heartbeat', data: payload);
      _updateState(
        heartbeatStatus: DeviceHeartbeatStatus.success,
        lastHeartbeatError: null,
        lastHeartbeatAt: DateTime.now(),
      );
    } on DioException catch (error) {
      if (_isEndpointNotReady(error)) {
        if (kDebugMode) {
          debugPrint('Device heartbeat endpoint not ready yet.');
        }
        _updateState(
          heartbeatStatus: DeviceHeartbeatStatus.failure,
          lastHeartbeatError: '设备心跳接口暂不可用',
        );
        return;
      }

      if (kDebugMode) {
        debugPrint('Failed to send device heartbeat: ${error.message}');
      }
      _updateState(
        heartbeatStatus: DeviceHeartbeatStatus.failure,
        lastHeartbeatError: error.message ?? '设备心跳失败',
      );
    }
  }

  void _updateState({
    NativePushStatus? nativePushStatus,
    Object? provider = _unset,
    DeviceRegistrationStatus? deviceRegistrationStatus,
    DeviceHeartbeatStatus? heartbeatStatus,
    Object? lastRegistrationError = _unset,
    Object? lastHeartbeatError = _unset,
    Object? lastHeartbeatAt = _unset,
  }) {
    stateNotifier.value = state.copyWith(
      nativePushStatus: nativePushStatus,
      provider: provider,
      deviceRegistrationStatus: deviceRegistrationStatus,
      heartbeatStatus: heartbeatStatus,
      lastRegistrationError: lastRegistrationError,
      lastHeartbeatError: lastHeartbeatError,
      lastHeartbeatAt: lastHeartbeatAt,
    );
  }

  Future<Map<String, dynamic>> _buildDevicePayload(
    String registrationId,
    String? provider,
  ) async {
    final deviceInfo = DeviceInfoPlugin();
    var deviceModel = 'Unknown';
    var osVersion = 'Unknown';

    try {
      if (!kIsWeb) {
        if (Platform.isAndroid) {
          final androidInfo = await deviceInfo.androidInfo;
          deviceModel = androidInfo.model;
          osVersion = 'Android ${androidInfo.version.release}';
        } else if (Platform.isIOS) {
          final iosInfo = await deviceInfo.iosInfo;
          deviceModel = iosInfo.model;
          osVersion = 'iOS ${iosInfo.systemVersion}';
        }
      } else {
        deviceModel = 'Web Browser';
        osVersion = 'Web';
      }
    } catch (_) {
      if (kDebugMode) {
        debugPrint('Failed to read device info for sync.');
      }
    }

    var appVersion = '1.0.0';
    try {
      final packageInfo = await PackageInfo.fromPlatform();
      appVersion = packageInfo.version;
    } catch (_) {
      if (kDebugMode) {
        debugPrint('Failed to read package info for device sync.');
      }
    }

    final deviceId = await _getOrCreateDeviceId();

    return {
      'device_id': deviceId,
      'push_token': registrationId,
      'provider': provider?.trim().isNotEmpty == true ? provider!.trim() : null,
      'device_model': deviceModel,
      'os_version': osVersion,
      'app_version': appVersion,
      'platform': kIsWeb ? 'web' : (Platform.isIOS ? 'ios' : 'android'),
    };
  }

  Future<String> _getOrCreateDeviceId() async {
    final provider = _deviceIdProvider;
    if (provider != null) {
      return provider();
    }
    final existing = await _storage.read(key: _deviceIdKey);
    if (existing != null && existing.isNotEmpty) {
      return existing;
    }

    final fallback = DateTime.now().millisecondsSinceEpoch.toString();
    await _storage.write(key: _deviceIdKey, value: fallback);
    return fallback;
  }

  Future<String?> _readLastRegisteredPushToken() {
    final reader = _registeredTokenReader;
    if (reader != null) {
      return reader();
    }
    return _storage.read(key: _lastRegisteredPushTokenKey);
  }

  Future<void> _writeLastRegisteredPushToken(String token) {
    final writer = _registeredTokenWriter;
    if (writer != null) {
      return writer(token);
    }
    return _storage.write(key: _lastRegisteredPushTokenKey, value: token);
  }

  bool _isEndpointNotReady(DioException error) {
    final statusCode = error.response?.statusCode;
    return statusCode == 404 || statusCode == 405 || statusCode == 501;
  }
}
