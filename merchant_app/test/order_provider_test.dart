import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/features/order/order_provider.dart';

void main() {
  test('fetchOrders includes backend-required pagination query params', () async {
    final apiClient = _FakeApiClient();
    final notifier = OrderNotifier(apiClient);

    await notifier.fetchOrders();

    expect(apiClient.lastPath, '/merchant/orders');
    expect(apiClient.lastQueryParameters, <String, dynamic>{
      'page_id': 1,
      'page_size': 20,
    });
  });
}

class _FakeApiClient implements ApiClient {
  String? lastPath;
  Map<String, dynamic>? lastQueryParameters;

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    lastPath = path;
    lastQueryParameters = queryParameters;
    return Response<dynamic>(
      requestOptions: RequestOptions(path: path),
      data: <String, dynamic>{
        'code': 0,
        'message': 'ok',
        'data': <String, dynamic>{
          'orders': <Map<String, dynamic>>[],
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
}
