import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/features/order/order_acceptance_coordinator.dart';
import 'package:merchant_app/features/settings/notification_settings_provider.dart';
import 'package:merchant_app/models/order.dart';

void main() {
  test(
    'coalesces concurrent accept-and-print attempts for the same order',
    () async {
      final apiClient = _AcceptApiClient(
        detailOrders: {'501': _orderJson(id: '501', status: 'preparing')},
      );
      final receiptPrinter = _CountingAcceptedOrderReceiptPrinter();
      final container = ProviderContainer(
        overrides: [
          apiClientProvider.overrideWithValue(apiClient),
          acceptedOrderReceiptPrinterProvider.overrideWithValue(receiptPrinter),
          notificationSettingsProvider.overrideWith(
            (ref) => _AutoPrintNotificationSettingsNotifier(),
          ),
        ],
      );
      addTearDown(container.dispose);

      final coordinator = container.read(orderAcceptanceCoordinatorProvider);
      final paidOrder = OrderModel.fromJson(
        _orderJson(id: '501', status: 'paid'),
      );

      final first = coordinator.acceptOrder(
        paidOrder.id,
        orderSnapshot: paidOrder,
        shopName: '测试门店',
      );
      final second = coordinator.acceptOrder(
        paidOrder.id,
        orderSnapshot: paidOrder,
        shopName: '测试门店',
      );

      expect(await Future.wait([first, second]), [true, true]);
      expect(apiClient.acceptPostCalls, 1);
      expect(receiptPrinter.printedOrderIds, ['501']);
    },
  );
}

class _CountingAcceptedOrderReceiptPrinter
    implements AcceptedOrderReceiptPrinter {
  final List<String> printedOrderIds = <String>[];

  @override
  bool get hasConnectedPrinter => true;

  @override
  Future<void> printAcceptedOrder(
    OrderModel order, {
    required String shopName,
  }) async {
    printedOrderIds.add(order.id);
  }
}

class _AutoPrintNotificationSettingsNotifier
    extends StateNotifier<NotificationSettingsState>
    implements NotificationSettingsNotifier {
  _AutoPrintNotificationSettingsNotifier()
    : super(
        const NotificationSettingsState(
          soundEnabled: false,
          voiceEnabled: false,
          autoPrintAfterAcceptEnabled: true,
        ),
      );

  @override
  Future<void> setAutoPrintAfterAcceptEnabled(bool enabled) async {}

  @override
  Future<void> setSoundEnabled(bool enabled) async {}

  @override
  Future<void> setVoiceEnabled(bool enabled) async {}
}

class _AcceptApiClient implements ApiClient {
  _AcceptApiClient({required this.detailOrders});

  final Map<String, Map<String, dynamic>> detailOrders;
  int acceptPostCalls = 0;

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    if (path.startsWith('/merchant/orders/')) {
      final orderId = path.split('/').last;
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: <String, dynamic>{
          'code': 0,
          'message': 'ok',
          'data': detailOrders[orderId],
        },
      );
    }
    if (path == '/merchant/orders') {
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: <String, dynamic>{
          'code': 0,
          'message': 'ok',
          'data': <String, dynamic>{
            'orders': detailOrders.values.toList(),
            'total': detailOrders.length,
            'page_id': 1,
            'page_size': 20,
          },
        },
      );
    }
    throw UnimplementedError('Unexpected GET $path');
  }

  @override
  Future<Response<dynamic>> post(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) async {
    if (path.startsWith('/merchant/orders/') && path.endsWith('/accept')) {
      acceptPostCalls += 1;
      final orderId = path.split('/')[3];
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: <String, dynamic>{
          'code': 0,
          'message': 'ok',
          'data': detailOrders[orderId],
        },
      );
    }
    throw UnimplementedError('Unexpected POST $path');
  }

  @override
  Future<Response<dynamic>> delete(String path, {bool requiresAuth = true}) {
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
    'order_no': 'ORD$id',
    'total_amount': 1850,
    'status': status,
    'created_at': '2026-04-12T08:00:00Z',
    'items': <Map<String, dynamic>>[
      {'name': '测试菜品', 'quantity': 1, 'unit_price': 1850, 'subtotal': 1850},
    ],
    'fee_breakdown': <String, dynamic>{
      'food_payable_amount': 1850,
      'customer_payable_amount': 1850,
      'merchant_receivable_amount': 1800,
    },
  };
}
