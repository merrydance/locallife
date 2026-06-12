import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';

class OrderDisplayConfig {
  const OrderDisplayConfig({
    required this.enablePrint,
    this.printTakeout = true,
    this.printDineIn = true,
    this.printReservation = true,
    this.printTriggerMode = 'accepted',
    required this.autoAcceptPaidOrders,
  });

  final bool enablePrint;
  final bool printTakeout;
  final bool printDineIn;
  final bool printReservation;
  final String printTriggerMode;
  final bool autoAcceptPaidOrders;

  bool get allowsAutoAcceptPaidOrders => enablePrint && autoAcceptPaidOrders;

  bool allowsAcceptedReceiptPrint(String orderType) {
    if (!enablePrint || !_isAcceptedTrigger(printTriggerMode)) {
      return false;
    }
    switch (orderType.trim()) {
      case 'takeout':
      case 'takeaway':
        return printTakeout;
      case 'dine_in':
        return printDineIn;
      case 'reservation':
        return printReservation;
      default:
        return false;
    }
  }

  factory OrderDisplayConfig.fromJson(Map<String, dynamic> json) {
    return OrderDisplayConfig(
      enablePrint: json['enable_print'] == true,
      printTakeout: json['print_takeout'] != false,
      printDineIn: json['print_dine_in'] != false,
      printReservation: json['print_reservation'] != false,
      printTriggerMode: json['print_trigger_mode']?.toString() ?? 'accepted',
      autoAcceptPaidOrders: json['auto_accept_paid_orders'] == true,
    );
  }

  static bool _isAcceptedTrigger(String triggerMode) {
    final normalized = triggerMode.trim();
    return normalized.isEmpty || normalized == 'accepted';
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
