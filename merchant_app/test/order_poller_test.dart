import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/push/device_sync_service.dart';
import 'package:merchant_app/core/push/local_notification_service.dart';
import 'package:merchant_app/core/push/native_push_manager.dart';
import 'package:merchant_app/core/push/push_provider.dart';
import 'package:merchant_app/core/service/auth_session_controller.dart';
import 'package:merchant_app/core/service/message_dedup.dart';
import 'package:merchant_app/core/service/order_alert_checkpoint_store.dart';
import 'package:merchant_app/core/service/order_poller.dart';
import 'package:merchant_app/core/service/pending_order_alert_store.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/auth/auth_service.dart';
import 'package:merchant_app/features/order/working_status_provider.dart';
import 'package:merchant_app/features/auth/auth_state.dart';
import 'package:merchant_app/features/order/order_provider.dart';
import 'package:merchant_app/features/settings/notification_settings_provider.dart';
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/models/push_message.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  test('pollOnce stops after syncing merchant status to closed', () async {
    final apiClient = _FakeApiClient();
    final sessionController = AuthSessionController();
    final authService = _FakeAuthService();
    final container = ProviderContainer(
      overrides: [
        apiClientProvider.overrideWithValue(apiClient),
        authServiceProvider.overrideWithValue(authService),
        authSessionControllerProvider.overrideWithValue(sessionController),
        authProvider.overrideWith(
          (ref) =>
              AuthNotifier(authService, sessionController)
                ..state = AuthState(
                  accessToken: 'access-token',
                  refreshToken: 'refresh-token',
                  merchantName: '测试商户',
                  isAuthenticated: true,
                ),
        ),
      ],
    );
    addTearDown(container.dispose);

    final poller = container.read(orderPollerProvider);
    await poller.pollOnce();

    expect(apiClient.calls, <String>['GET /merchants/me/status']);
    expect(container.read(workingStatusProvider).isOnline, isFalse);
    expect(container.read(workingStatusProvider).hasConfirmedState, isTrue);
  });

  test(
    'pollOnce treats paid orders already in memory as alertable without checkpoint',
    () async {
      SharedPreferences.setMockInitialValues(<String, Object>{});
      final apiClient = _FakeApiClient(
        isOpen: true,
        awaitingOrders: [_orderJson(id: 'paid-1', status: 'paid')],
      );
      final sessionController = AuthSessionController();
      final authService = _FakeAuthService();
      final checkpointStore = MemoryOrderAlertCheckpointStore();
      final pendingStore = MemoryPendingOrderAlertStore();
      final container = ProviderContainer(
        overrides: [
          apiClientProvider.overrideWithValue(apiClient),
          deviceSyncServiceProvider.overrideWithValue(_NoopDeviceSyncService()),
          localNotificationServiceProvider.overrideWithValue(
            _NoopLocalNotificationService(),
          ),
          messageDeduplicatorProvider.overrideWithValue(
            MessageDeduplicator.memoryOnly(),
          ),
          authServiceProvider.overrideWithValue(authService),
          authSessionControllerProvider.overrideWithValue(sessionController),
          orderAlertCheckpointStoreProvider.overrideWithValue(checkpointStore),
          pendingOrderAlertStoreProvider.overrideWithValue(pendingStore),
          notificationSettingsProvider.overrideWith(
            (ref) => _FakeNotificationSettingsNotifier(),
          ),
          authProvider.overrideWith(
            (ref) =>
                AuthNotifier(authService, sessionController)
                  ..state = AuthState(
                    accessToken: 'access-token',
                    refreshToken: 'refresh-token',
                    merchantName: '测试商户',
                    isAuthenticated: true,
                  ),
          ),
        ],
      );
      addTearDown(container.dispose);
      container.read(workingStatusProvider.notifier).state =
          const WorkingStatusState(isOnline: true, hasConfirmedState: true);
      container
          .read(orderProvider.notifier)
          .addOrUpdateOrder(
            _buildOrder(id: 'paid-1', status: OrderStatus.paid),
          );

      final poller = container.read(orderPollerProvider);
      await poller.pollOnce();

      final pendingAlerts = await pendingStore.loadPendingAlerts();
      expect(pendingAlerts.map((alert) => alert.orderId), contains('paid-1'));
      expect(await checkpointStore.hasAlerted('paid-1'), isFalse);
    },
  );
}

class _FakeNotificationSettingsNotifier
    extends StateNotifier<NotificationSettingsState>
    implements NotificationSettingsNotifier {
  _FakeNotificationSettingsNotifier()
    : super(
        const NotificationSettingsState(
          soundEnabled: false,
          voiceEnabled: false,
          autoPrintAfterAcceptEnabled: false,
        ),
      );

  @override
  Future<void> setAutoPrintAfterAcceptEnabled(bool enabled) async {}

  @override
  Future<void> setSoundEnabled(bool enabled) async {}

  @override
  Future<void> setVoiceEnabled(bool enabled) async {}
}

class _NoopDeviceSyncService extends DeviceSyncService {
  _NoopDeviceSyncService() : super(_FakeApiClient(), NativePushManager());

  @override
  Future<void> sendHeartbeat() async {}
}

class _NoopLocalNotificationService extends LocalNotificationService {
  @override
  Future<void> showNewOrderNotification(PushMessage message) async {}
}

class _FakeApiClient implements ApiClient {
  final List<String> calls = <String>[];

  _FakeApiClient({
    this.isOpen = false,
    this.awaitingOrders = const <Map<String, dynamic>>[],
  });

  final bool isOpen;
  final List<Map<String, dynamic>> awaitingOrders;

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    calls.add('GET $path');
    if (path == '/merchants/me/status') {
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: <String, dynamic>{
          'code': 0,
          'message': 'ok',
          'data': <String, dynamic>{'is_open': isOpen, 'message': '店铺状态已同步'},
        },
      );
    }
    if (path == '/merchant/orders') {
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: <String, dynamic>{
          'code': 0,
          'message': 'ok',
          'data': <String, dynamic>{'orders': awaitingOrders},
        },
      );
    }

    throw UnimplementedError('Unexpected GET $path');
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
  }) async {
    calls.add('PUT $path');
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
  Future<Map<String, String?>?> refreshSessionTokens() {
    throw UnimplementedError();
  }
}

Map<String, dynamic> _orderJson({required String id, required String status}) {
  return <String, dynamic>{
    'id': id,
    'order_no': 'ORD-$id',
    'total_amount': 1800,
    'status': status,
    'created_at': '2026-04-12T08:00:00Z',
    'items': <Map<String, dynamic>>[],
  };
}

OrderModel _buildOrder({required String id, required OrderStatus status}) {
  return OrderModel(
    id: id,
    orderNum: 'ORD-$id',
    amount: 18,
    status: status,
    createdAt: DateTime.parse('2026-04-12T08:00:00Z'),
    items: const <OrderItem>[],
  );
}

class _FakeAuthService implements AuthService {
  @override
  Future<void> clearTokens() async {}

  @override
  Future<Map<String, String?>> getTokens() async => {
    'accessToken': null,
    'refreshToken': null,
    'merchantName': null,
  };

  @override
  Future<Map<String, dynamic>> refreshToken(String refreshToken) {
    throw UnimplementedError();
  }

  @override
  Future<void> saveTokens(
    String access,
    String refresh, {
    String? merchantName,
  }) async {}

  @override
  Future<Map<String, String?>?> tryAutoLogin() async {
    return <String, String?>{
      'accessToken': 'access-token',
      'refreshToken': 'refresh-token',
      'merchantName': '测试商户',
    };
  }

  @override
  Future<Map<String, dynamic>> verifyBindingCode(String code) {
    throw UnimplementedError();
  }
}
