import 'package:dio/dio.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/features/display_config/display_config_provider.dart';

void main() {
  test('display config repository reads backend auto-accept truth', () async {
    final apiClient = _FakeApiClient(<String, dynamic>{
      'enable_print': true,
      'auto_accept_paid_orders': true,
    });
    final repository = ApiOrderDisplayConfigRepository(apiClient);

    final config = await repository.fetchDisplayConfig();

    expect(apiClient.lastPath, '/merchant/display-config');
    expect(config.enablePrint, isTrue);
    expect(config.autoAcceptPaidOrders, isTrue);
    expect(config.allowsAutoAcceptPaidOrders, isTrue);
  });

  test('display config disables auto-accept when printing is disabled', () {
    final config = OrderDisplayConfig.fromJson(<String, dynamic>{
      'enable_print': false,
      'auto_accept_paid_orders': true,
    });

    expect(config.allowsAutoAcceptPaidOrders, isFalse);
  });
}

class _FakeApiClient implements ApiClient {
  _FakeApiClient(this.displayConfig);

  final Map<String, dynamic> displayConfig;
  String? lastPath;

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) async {
    lastPath = path;
    return Response<dynamic>(
      requestOptions: RequestOptions(path: path),
      data: <String, dynamic>{
        'code': 0,
        'message': 'ok',
        'data': displayConfig,
      },
    );
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
  Future<Map<String, String?>?> refreshSessionTokens() {
    throw UnimplementedError();
  }
}
