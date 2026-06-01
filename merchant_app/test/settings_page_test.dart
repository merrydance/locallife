import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/push/device_sync_service.dart';
import 'package:merchant_app/features/settings/settings_page.dart';

void main() {
  test('device sync subtitle hides operator-only push diagnostics', () {
    const state = DeviceSyncState(
      nativePushStatus: NativePushStatus.missingToken,
      provider: 'xiaomi',
      deviceRegistrationStatus: DeviceRegistrationStatus.missingToken,
    );

    expect(state.operatorDiagnosticMessages, isNotEmpty);
    expect(state.userVisibleDegradationMessages, isEmpty);
    expect(deviceSyncSubtitle(state), '正在检测设备连接和心跳状态。');
  });

  test('device sync subtitle keeps heartbeat failure visible', () {
    const state = DeviceSyncState(
      heartbeatStatus: DeviceHeartbeatStatus.failure,
    );

    expect(deviceSyncSubtitle(state), '设备心跳失败，后台可能误判商户离线');
  });
}
