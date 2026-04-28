import 'dart:io';

import 'package:device_info_plus/device_info_plus.dart';
import 'package:dio/dio.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:package_info_plus/package_info_plus.dart';

import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/push/native_push_manager.dart';

class DeviceSyncService {
  DeviceSyncService(this._apiClient, this._pushManager) {
    _pushManager.onTokenRegistered = (token, provider) => ensureRegistered();
  }

  static const _lastRegisteredPushTokenKey = 'last_registered_push_token';
  static const _deviceIdKey = 'device_uuid';

  final ApiClient _apiClient;
  final NativePushManager _pushManager;
  final FlutterSecureStorage _storage = const FlutterSecureStorage();

  Future<void>? _registrationFuture;
  String? _lastRegisteredPushToken;

  Future<void> ensureRegistered() {
    _registrationFuture ??= _performEnsureRegistered()
        .whenComplete(() => _registrationFuture = null);
    return _registrationFuture!;
  }

  Future<void> _performEnsureRegistered() async {
    final registrationId = await _pushManager.getRegistrationID();
    if (registrationId == null || registrationId.isEmpty) {
      return;
    }

    _lastRegisteredPushToken ??=
        await _storage.read(key: _lastRegisteredPushTokenKey);
    if (_lastRegisteredPushToken == registrationId) {
      return;
    }

    final payload = await _buildDevicePayload(registrationId);

    try {
      await _apiClient.post('/merchant/device/register', data: payload);
      await _storage.write(
        key: _lastRegisteredPushTokenKey,
        value: registrationId,
      );
      _lastRegisteredPushToken = registrationId;
    } on DioException catch (error) {
      if (_isEndpointNotReady(error)) {
        if (kDebugMode) {
          debugPrint('Device register endpoint not ready yet.');
        }
        return;
      }

      if (kDebugMode) {
        debugPrint('Failed to register device: ${error.message}');
      }
    }
  }

  Future<void> sendHeartbeat() async {
    final registrationId = await _pushManager.getRegistrationID();
    if (registrationId == null || registrationId.isEmpty) {
      return;
    }

    final payload = await _buildDevicePayload(registrationId);
    try {
      await _apiClient.put('/merchant/device/heartbeat', data: payload);
    } on DioException catch (error) {
      if (_isEndpointNotReady(error)) {
        if (kDebugMode) {
          debugPrint('Device heartbeat endpoint not ready yet.');
        }
        return;
      }

      if (kDebugMode) {
        debugPrint('Failed to send device heartbeat: ${error.message}');
      }
    }
  }

  Future<Map<String, dynamic>> _buildDevicePayload(String registrationId) async {
    final deviceInfo = DeviceInfoPlugin();
    var deviceModel = 'Unknown';
    var osVersion = 'Unknown';

    try {
      if (Platform.isAndroid) {
        final androidInfo = await deviceInfo.androidInfo;
        deviceModel = androidInfo.model;
        osVersion = 'Android ${androidInfo.version.release}';
      } else if (Platform.isIOS) {
        final iosInfo = await deviceInfo.iosInfo;
        deviceModel = iosInfo.model;
        osVersion = 'iOS ${iosInfo.systemVersion}';
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
      'device_model': deviceModel,
      'os_version': osVersion,
      'app_version': appVersion,
      'platform': Platform.isIOS ? 'ios' : 'android',
    };
  }

  Future<String> _getOrCreateDeviceId() async {
    final existing = await _storage.read(key: _deviceIdKey);
    if (existing != null && existing.isNotEmpty) {
      return existing;
    }

    final fallback = DateTime.now().millisecondsSinceEpoch.toString();
    await _storage.write(key: _deviceIdKey, value: fallback);
    return fallback;
  }

  bool _isEndpointNotReady(DioException error) {
    final statusCode = error.response?.statusCode;
    return statusCode == 404 || statusCode == 405 || statusCode == 501;
  }
}