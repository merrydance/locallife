import 'dart:io';
import 'package:device_info_plus/device_info_plus.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:uuid/uuid.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';
import 'package:merchant_app/core/network/api_client.dart';

class AuthService {
  final ApiClient _apiClient;
  final FlutterSecureStorage _storage;

  static const _accessTokenKey = 'access_token';
  static const _refreshTokenKey = 'refresh_token';
  static const _merchantNameKey = 'merchant_name';
  static const _deviceIdKey = 'device_uuid';

  AuthService(this._apiClient) : _storage = const FlutterSecureStorage();

  Future<Map<String, String?>> getTokens() async {
    final access = await _storage.read(key: _accessTokenKey);
    final refresh = await _storage.read(key: _refreshTokenKey);
    final merchantName = await _storage.read(key: _merchantNameKey);
    return {
      'accessToken': access,
      'refreshToken': refresh,
      'merchantName': merchantName,
    };
  }

  Future<void> saveTokens(
    String access,
    String refresh, {
    String? merchantName,
  }) async {
    await _storage.write(key: _accessTokenKey, value: access);
    await _storage.write(key: _refreshTokenKey, value: refresh);
    if (merchantName != null && merchantName.trim().isNotEmpty) {
      await _storage.write(key: _merchantNameKey, value: merchantName.trim());
    }
  }

  Future<void> clearTokens() async {
    await _storage.delete(key: _accessTokenKey);
    await _storage.delete(key: _refreshTokenKey);
    await _storage.delete(key: _merchantNameKey);
  }

  Future<String> _getDeviceId() async {
    String? id = await _storage.read(key: _deviceIdKey);
    if (id == null) {
      id = const Uuid().v4();
      await _storage.write(key: _deviceIdKey, value: id);
    }
    return id;
  }

  /// Verify binding code and return tokens
  Future<Map<String, dynamic>> verifyBindingCode(String code) async {
    final deviceInfo = DeviceInfoPlugin();
    String deviceId = await _getDeviceId();
    String deviceModel = 'Unknown';
    String osVersion = 'Unknown';

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
    } catch (e) {
      // Ignored
    }

    String appVersion = '1.0.0';
    try {
      final packageInfo = await PackageInfo.fromPlatform();
      appVersion = packageInfo.version;
    } catch (e) {
      // Ignored
    }

    final response = await _apiClient.post(
      '/auth/app-bind/verify',
      requiresAuth: false,
      data: {
        'code': code,
        'device_id': deviceId,
        'device_model': deviceModel,
        'os_version': osVersion,
        'app_version': appVersion,
      },
    );

    final envelope = response.data;
    if (envelope is Map && envelope.containsKey('data')) {
      return envelope['data'] as Map<String, dynamic>;
    }
    return envelope;
  }

  /// Refresh access token using refresh token
  Future<Map<String, dynamic>> refreshToken(String refreshToken) async {
    final response = await _apiClient.post(
      '/auth/refresh',
      requiresAuth: false,
      data: {'refresh_token': refreshToken},
    );
    final envelope = response.data;
    if (envelope is Map && envelope.containsKey('data')) {
      return envelope['data'] as Map<String, dynamic>;
    }
    return envelope;
  }

  Future<Map<String, String?>?> tryAutoLogin() async {
    final tokens = await getTokens();
    final storedRefreshToken = tokens['refreshToken'];
    if (storedRefreshToken == null || storedRefreshToken.isEmpty) {
      return null;
    }

    try {
      final data = await refreshToken(storedRefreshToken);
      final accessToken = data['access_token']?.toString();
      final newRefreshToken = data['refresh_token']?.toString();

      if (accessToken == null || newRefreshToken == null) {
        await clearTokens();
        return null;
      }

      await saveTokens(
        accessToken,
        newRefreshToken,
        merchantName: tokens['merchantName'],
      );

      return {
        'accessToken': accessToken,
        'refreshToken': newRefreshToken,
        'merchantName': tokens['merchantName'],
      };
    } catch (_) {
      await clearTokens();
      return null;
    }
  }
}
