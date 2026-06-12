import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/models/order.dart';

abstract interface class LocalPrintEventRepository {
  Future<void> recordAcceptedReceiptEvent(
    OrderModel order, {
    required String status,
    String? printerName,
    String? errorMessage,
  });
}

class ApiLocalPrintEventRepository implements LocalPrintEventRepository {
  ApiLocalPrintEventRepository(this._apiClient);

  final ApiClient _apiClient;

  @override
  Future<void> recordAcceptedReceiptEvent(
    OrderModel order, {
    required String status,
    String? printerName,
    String? errorMessage,
  }) async {
    await _apiClient.post(
      '/merchant/orders/${order.id}/local-print-events',
      data: <String, dynamic>{
        'event_key': 'accepted-receipt:${order.id}',
        'status': status,
        if (printerName?.trim().isNotEmpty == true)
          'printer_name': printerName!.trim(),
        if (errorMessage?.trim().isNotEmpty == true)
          'error_message': errorMessage!.trim(),
      },
    );
  }
}

final localPrintEventRepositoryProvider = Provider<LocalPrintEventRepository>((
  ref,
) {
  final apiClient = ref.watch(apiClientProvider);
  return ApiLocalPrintEventRepository(apiClient);
});
