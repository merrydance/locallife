import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/features/printer/local_print_event_repository.dart';
import 'package:merchant_app/models/order.dart';

void main() {
  test(
    'records accepted receipt event to merchant order local-print endpoint',
    () async {
      final apiClient = _FakeApiClient();
      final repository = ApiLocalPrintEventRepository(apiClient);

      await repository.recordAcceptedReceiptEvent(
        OrderModel(
          id: '501',
          orderNum: 'ORD501',
          orderType: 'takeout',
          amount: 18.5,
          status: OrderStatus.preparing,
          createdAt: DateTime.parse('2026-04-12T08:00:00Z'),
          items: const <OrderItem>[],
        ),
        status: 'success',
        printerName: ' 前台蓝牙打印机 ',
      );

      expect(apiClient.lastPath, '/merchant/orders/501/local-print-events');
      expect(apiClient.lastData, <String, dynamic>{
        'event_key': 'accepted-receipt:501',
        'status': 'success',
        'printer_name': '前台蓝牙打印机',
      });
    },
  );
}

class _FakeApiClient implements ApiClient {
  String? lastPath;
  dynamic lastData;

  @override
  Future<Response<dynamic>> post(
    String path, {
    dynamic data,
    bool requiresAuth = true,
  }) async {
    lastPath = path;
    lastData = data;
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
    throw UnimplementedError();
  }

  @override
  Future<Map<String, String?>?> refreshSessionTokens() {
    throw UnimplementedError();
  }
}
