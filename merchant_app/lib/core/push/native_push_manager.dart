import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:device_info_plus/device_info_plus.dart';
import 'package:merchant_app/models/push_message.dart';

typedef TokenCallback = Future<void> Function(String token, String provider);
typedef PushInitializationFailedCallback =
    Future<void> Function(String provider, String message);

typedef PushOrderCallback =
    Future<void> Function(
      PushMessage message, {
      required bool showLocalNotification,
    });

class NativePushManager {
  static const MethodChannel _channel = MethodChannel(
    'com.locallife.merchant/push',
  );

  // Callback when a valid new order arrives
  PushOrderCallback? onNewOrder;
  Future<void> Function(PushMessage message)? onNotificationOpened;
  TokenCallback? onTokenRegistered;
  PushInitializationFailedCallback? onInitializationFailed;
  String? _registrationId;
  String? _registrationProvider;
  String? _initializationFailure;

  NativePushManager();

  Future<void> init() async {
    if (kIsWeb) return;
    _channel.setMethodCallHandler(_handleMethodCall);

    // Initial native setup
    try {
      final String manufacturer = await _getManufacturer();
      await _channel.invokeMethod('initialize', {'manufacturer': manufacturer});
      debugPrint('NativePushManager initialized for $manufacturer');
    } on PlatformException catch (e) {
      _initializationFailure = e.message ?? '厂商推送初始化失败';
      debugPrint('Failed to initialize native push: ${e.message}');
      await onInitializationFailed?.call('unknown', _initializationFailure!);
    }
  }

  Future<String> _getManufacturer() async {
    final deviceInfo = DeviceInfoPlugin();
    if (defaultTargetPlatform == TargetPlatform.android) {
      final androidInfo = await deviceInfo.androidInfo;
      return androidInfo.manufacturer.toLowerCase();
    }
    return 'unknown';
  }

  Future<void> _handleMethodCall(MethodCall call) async {
    switch (call.method) {
      case 'onReceiveMessage':
        final Map<String, dynamic> message = Map<String, dynamic>.from(
          call.arguments,
        );
        await _handleIncomingMessage(message, showLocalNotification: true);
        break;
      case 'onNotificationOpened':
        final Map<String, dynamic> message = Map<String, dynamic>.from(
          call.arguments,
        );
        final parsedMessage = _extractPushMessage(message);
        if (parsedMessage != null) {
          await onNotificationOpened?.call(parsedMessage);
        }
        break;
      case 'onTokenRegistered':
        final String token = call.arguments['token'];
        final String provider = call.arguments['provider'];
        _registrationId = token;
        _registrationProvider = provider;
        _initializationFailure = null;
        debugPrint('Push token registered (Provider: $provider)');
        await onTokenRegistered?.call(token, provider);
        break;
      case 'onInitializationFailed':
        final arguments = Map<String, dynamic>.from(call.arguments);
        final provider = arguments['provider']?.toString() ?? 'unknown';
        final message = arguments['message']?.toString() ?? '厂商推送初始化失败';
        _registrationProvider = provider;
        _initializationFailure = message;
        debugPrint('Native push initialization failed ($provider): $message');
        await onInitializationFailed?.call(provider, message);
        break;
    }
  }

  Future<void> _handleIncomingMessage(
    Map<String, dynamic> message, {
    required bool showLocalNotification,
  }) async {
    final pushMessage = _extractPushMessage(message);
    if (pushMessage == null) return;

    await onNewOrder?.call(
      pushMessage,
      showLocalNotification: showLocalNotification,
    );
  }

  PushMessage? _extractPushMessage(Map<String, dynamic> message) {
    // Standardize payload extraction from different vendors if needed
    // Assuming native side sends a clean map
    try {
      return PushMessage.fromJson(message);
    } catch (e) {
      debugPrint('Error parsing push message: $e');
      return null;
    }
  }

  Future<String?> getRegistrationID() async {
    if (kIsWeb) return null;
    return _registrationId ??
        await _channel.invokeMethod<String>('getRegistrationId');
  }

  Future<String?> getRegistrationProvider() async {
    if (kIsWeb) return null;
    return _registrationProvider ??
        await _channel.invokeMethod<String>('getRegistrationProvider');
  }

  Future<String?> getInitializationFailure() async {
    if (kIsWeb) return null;
    return _initializationFailure ??
        await _channel.invokeMethod<String>('getInitializationFailure');
  }
}
