import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/service/foreground_service.dart';

void main() {
  test(
    'keeps foreground service running when working merchant loses network',
    () {
      final decision = decideForegroundServiceStatus(
        isAuthenticated: true,
        isWorking: true,
        hasNetwork: false,
        isWebSocketConnected: false,
        isNotificationPermissionGranted: true,
      );

      expect(decision.shouldRun, isTrue);
      expect(decision.status, ForegroundServiceStatus.networkUnavailable);
      expect(decision.notificationText, contains('网络不可用'));
    },
  );

  test('updates foreground service as reconnecting when websocket is down', () {
    final decision = decideForegroundServiceStatus(
      isAuthenticated: true,
      isWorking: true,
      hasNetwork: true,
      isWebSocketConnected: false,
      isNotificationPermissionGranted: true,
    );

    expect(decision.shouldRun, isTrue);
    expect(decision.status, ForegroundServiceStatus.reconnecting);
    expect(decision.notificationText, contains('正在重连'));
  });

  test('stops foreground service only when merchant is not active', () {
    final decision = decideForegroundServiceStatus(
      isAuthenticated: true,
      isWorking: false,
      hasNetwork: true,
      isWebSocketConnected: false,
      isNotificationPermissionGranted: true,
    );

    expect(decision.shouldRun, isFalse);
    expect(decision.status, ForegroundServiceStatus.stopped);
  });
}
