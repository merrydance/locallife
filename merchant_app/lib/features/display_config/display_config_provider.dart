import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';

class OrderDisplayConfig {
  const OrderDisplayConfig({
    required this.enablePrint,
    required this.autoAcceptPaidOrders,
  });

  final bool enablePrint;
  final bool autoAcceptPaidOrders;

  bool get allowsAutoAcceptPaidOrders => enablePrint && autoAcceptPaidOrders;

  factory OrderDisplayConfig.fromJson(Map<String, dynamic> json) {
    return OrderDisplayConfig(
      enablePrint: json['enable_print'] == true,
      autoAcceptPaidOrders: json['auto_accept_paid_orders'] == true,
    );
  }
}

abstract interface class OrderDisplayConfigRepository {
  Future<OrderDisplayConfig> fetchDisplayConfig();
}

class ApiOrderDisplayConfigRepository implements OrderDisplayConfigRepository {
  ApiOrderDisplayConfigRepository(this._apiClient);

  final ApiClient _apiClient;

  @override
  Future<OrderDisplayConfig> fetchDisplayConfig() async {
    final response = await _apiClient.get('/merchant/display-config');
    return OrderDisplayConfig.fromJson(extractApiResponseData(response.data));
  }
}

final orderDisplayConfigRepositoryProvider =
    Provider<OrderDisplayConfigRepository>((ref) {
      final apiClient = ref.watch(apiClientProvider);
      return ApiOrderDisplayConfigRepository(apiClient);
    });
