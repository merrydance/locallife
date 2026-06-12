import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/service/order_poller.dart';
import 'package:merchant_app/features/order/order_provider.dart';
import 'package:merchant_app/models/order.dart';

void main() {
  test(
    'fetchOrders includes backend-required pagination query params',
    () async {
      final apiClient = _FakeApiClient();
      final notifier = OrderNotifier(apiClient);

      await notifier.fetchOrders();

      expect(apiClient.lastPath, '/merchant/orders');
      expect(apiClient.lastQueryParameters, <String, dynamic>{
        'page_id': 1,
        'page_size': 20,
      });
    },
  );

  test(
    'fetchAwaitingAcceptanceOrders requests only backend paid orders',
    () async {
      final apiClient = _FakeApiClient(
        orders: [_orderJson(id: 'paid-1', status: 'paid')],
      );
      final notifier = OrderNotifier(apiClient);

      final orders = await notifier.fetchAwaitingAcceptanceOrders();

      expect(apiClient.lastPath, '/merchant/orders');
      expect(apiClient.lastQueryParameters, <String, dynamic>{
        'page_id': 1,
        'page_size': 20,
        'status': 'paid',
      });
      expect(orders.single.status, OrderStatus.paid);
      expect(notifier.state.orders, isEmpty);
    },
  );

  test('order poller fetches awaiting acceptance orders only', () async {
    final apiClient = _FakeApiClient(
      orders: [_orderJson(id: 'paid-1', status: 'paid')],
    );
    final notifier = OrderNotifier(apiClient);

    await OrderPoller.fetchLatestAwaitingAcceptanceOrders(notifier);

    expect(apiClient.lastQueryParameters, containsPair('status', 'paid'));
  });

  test('markOrderReady posts ready endpoint and updates local state', () async {
    final apiClient = _FakeApiClient(
      orders: [_orderJson(id: '501', status: 'preparing')],
      postResponseOrder: _orderJson(id: '501', status: 'ready'),
    );
    final notifier = OrderNotifier(apiClient);

    await notifier.fetchOrders();
    final success = await notifier.markOrderReady('501');

    expect(success, isTrue);
    expect(apiClient.lastPostPath, '/merchant/orders/501/ready');
    expect(notifier.state.orders.single.status, OrderStatus.ready);
  });

  test(
    'acceptOrder reads detail when action response has no order snapshot',
    () async {
      final apiClient = _FakeApiClient(
        orders: [_orderJson(id: '501', status: 'paid')],
        detailOrders: {'501': _orderJson(id: '501', status: 'preparing')},
      );
      final notifier = OrderNotifier(apiClient);

      await notifier.fetchOrders();
      final success = await notifier.acceptOrder('501');

      expect(success, isTrue);
      expect(apiClient.lastPostPath, '/merchant/orders/501/accept');
      expect(apiClient.detailGetCalls['501'], 1);
      expect(notifier.state.orders.single.status, OrderStatus.preparing);
      expect(notifier.state.orders.single.canMarkReady, isTrue);
    },
  );

  test('addOrUpdateOrder preserves and updates order type', () {
    final notifier = OrderNotifier(_FakeApiClient());

    notifier.addOrUpdateOrder(
      OrderModel.fromJson(
        _orderJson(id: '501', status: 'paid', orderType: 'takeout'),
      ),
    );
    notifier.addOrUpdateOrder(
      OrderModel.fromJson(_orderJson(id: '501', status: 'preparing')),
    );
    expect(notifier.state.orders.single.orderType, 'takeout');

    notifier.addOrUpdateOrder(
      OrderModel.fromJson(
        _orderJson(id: '501', status: 'preparing', orderType: 'dine_in'),
      ),
    );
    expect(notifier.state.orders.single.orderType, 'dine_in');
  });

  test(
    'acceptOrder does not optimistically succeed when readback is unresolved',
    () async {
      final apiClient = _FakeApiClient(
        orders: [_orderJson(id: '501', status: 'paid')],
        detailOrders: {'501': _orderJson(id: '501', status: 'paid')},
      );
      final notifier = OrderNotifier(apiClient);

      await notifier.fetchOrders();
      final success = await notifier.acceptOrder('501');

      expect(success, isFalse);
      expect(apiClient.lastPostPath, '/merchant/orders/501/accept');
      expect(apiClient.detailGetCalls['501'], 1);
      expect(notifier.state.orders.single.status, OrderStatus.paid);
      expect(notifier.state.error, '结果确认中，请刷新订单');
    },
  );

  test(
    'markOrderReady accepts courier-accepted readback with ready fulfillment',
    () async {
      final apiClient = _FakeApiClient(
        orders: [
          _orderJson(
            id: '501',
            status: 'courier_accepted',
            fulfillmentStatus: 'preparing',
          ),
        ],
        detailOrders: {
          '501': _orderJson(
            id: '501',
            status: 'courier_accepted',
            fulfillmentStatus: 'ready',
          ),
        },
      );
      final notifier = OrderNotifier(apiClient);

      await notifier.fetchOrders();
      final success = await notifier.markOrderReady('501');

      expect(success, isTrue);
      expect(apiClient.detailGetCalls['501'], 1);
      expect(notifier.state.orders.single.status, OrderStatus.courierAccepted);
      expect(
        notifier.state.orders.single.fulfillmentStatus,
        FulfillmentStatus.ready,
      );
      expect(notifier.state.orders.single.canMarkReady, isFalse);
    },
  );
}

class _FakeApiClient implements ApiClient {
  _FakeApiClient({
    this.orders = const <Map<String, dynamic>>[],
    this.postResponseOrder,
    this.detailOrders = const <String, Map<String, dynamic>>{},
  });

  final List<Map<String, dynamic>> orders;
  final Map<String, dynamic>? postResponseOrder;
  final Map<String, Map<String, dynamic>> detailOrders;
  String? lastPath;
  String? lastPostPath;
  Map<String, dynamic>? lastQueryParameters;
  final Map<String, int> detailGetCalls = <String, int>{};

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    lastPath = path;
    lastQueryParameters = queryParameters;
    if (path.startsWith('/merchant/orders/')) {
      final orderId = path.split('/').last;
      detailGetCalls[orderId] = (detailGetCalls[orderId] ?? 0) + 1;
      final detailOrder = detailOrders[orderId];
      if (detailOrder == null) {
        throw DioException(
          requestOptions: RequestOptions(path: path),
          type: DioExceptionType.badResponse,
          response: Response<void>(
            requestOptions: RequestOptions(path: path),
            statusCode: 404,
          ),
        );
      }
      return Response<dynamic>(
        requestOptions: RequestOptions(path: path),
        data: <String, dynamic>{
          'code': 0,
          'message': 'ok',
          'data': detailOrder,
        },
      );
    }
    return Response<dynamic>(
      requestOptions: RequestOptions(path: path),
      data: <String, dynamic>{
        'code': 0,
        'message': 'ok',
        'data': <String, dynamic>{
          'orders': orders,
          'total': 0,
          'page_id': 1,
          'page_size': 20,
        },
      },
    );
  }

  @override
  Future<Response<dynamic>> post(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) async {
    lastPostPath = path;
    return Response<dynamic>(
      requestOptions: RequestOptions(path: path),
      data: <String, dynamic>{
        'code': 0,
        'message': 'ok',
        'data': postResponseOrder,
      },
    );
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
  Future<Response<dynamic>> patch(
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

Map<String, dynamic> _orderJson({
  required String id,
  required String status,
  String orderType = '',
  String? fulfillmentStatus,
}) {
  return <String, dynamic>{
    'id': id,
    'order_no': 'ORD-$id',
    'order_type': orderType,
    'total_amount': 1800,
    'status': status,
    'fulfillment_status': ?fulfillmentStatus,
    'created_at': '2026-04-12T08:00:00Z',
    'items': <Map<String, dynamic>>[],
  };
}
