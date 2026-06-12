import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/features/auth/auth_provider.dart';
import 'package:merchant_app/features/order/order_provider.dart';
import 'package:merchant_app/features/printer/printer_provider.dart';
import 'package:merchant_app/features/settings/notification_settings_provider.dart';
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/models/push_message.dart';

final orderAcceptanceCoordinatorProvider = Provider<OrderAcceptanceCoordinator>(
  (ref) {
    return OrderAcceptanceCoordinator(ref);
  },
);

final acceptedOrderReceiptPrinterProvider =
    Provider<AcceptedOrderReceiptPrinter>((ref) {
      return BleAcceptedOrderReceiptPrinter(ref);
    });

abstract class AcceptedOrderReceiptPrinter {
  bool get hasConnectedPrinter;

  Future<void> printAcceptedOrder(OrderModel order, {required String shopName});
}

class BleAcceptedOrderReceiptPrinter implements AcceptedOrderReceiptPrinter {
  BleAcceptedOrderReceiptPrinter(this._ref);

  final Ref _ref;

  @override
  bool get hasConnectedPrinter =>
      _ref.read(printerProvider).connectedDevice != null;

  @override
  Future<void> printAcceptedOrder(
    OrderModel order, {
    required String shopName,
  }) {
    return _ref
        .read(printerProvider.notifier)
        .printAcceptedOrder(order, shopName: shopName);
  }
}

class OrderAcceptanceCoordinator {
  OrderAcceptanceCoordinator(this._ref);

  final Ref _ref;
  final Map<String, Future<bool>> _acceptAndPrintFutures =
      <String, Future<bool>>{};

  Future<bool> acceptOrder(
    String orderId, {
    OrderModel? orderSnapshot,
    PushMessage? messageSnapshot,
    String? shopName,
  }) {
    final normalizedOrderId = orderId.trim();
    if (normalizedOrderId.isEmpty) {
      return Future<bool>.value(false);
    }

    final existing = _acceptAndPrintFutures[normalizedOrderId];
    if (existing != null) {
      return existing;
    }

    late final Future<bool> future;
    future =
        _acceptAndMaybePrint(
          normalizedOrderId,
          orderSnapshot: orderSnapshot,
          messageSnapshot: messageSnapshot,
          shopName: shopName,
        ).whenComplete(() {
          if (identical(_acceptAndPrintFutures[normalizedOrderId], future)) {
            _acceptAndPrintFutures.remove(normalizedOrderId);
          }
        });
    _acceptAndPrintFutures[normalizedOrderId] = future;
    return future;
  }

  Future<bool> _acceptAndMaybePrint(
    String orderId, {
    OrderModel? orderSnapshot,
    PushMessage? messageSnapshot,
    String? shopName,
  }) async {
    final orderNotifier = _ref.read(orderProvider.notifier);
    final accepted = await orderNotifier.acceptOrder(orderId);
    if (!accepted) {
      return false;
    }

    final detailOrder = await orderNotifier.fetchOrderDetail(orderId);
    await orderNotifier.fetchOrders();

    final orderForPrint =
        detailOrder ??
        _findCurrentOrder(orderId) ??
        orderSnapshot ??
        _orderFromMessage(messageSnapshot);
    if (orderForPrint == null) {
      return true;
    }

    await _printAcceptedOrderIfEnabled(
      orderForPrint,
      shopName: _resolveShopName(shopName ?? messageSnapshot?.shopName),
    );
    return true;
  }

  Future<void> _printAcceptedOrderIfEnabled(
    OrderModel order, {
    required String shopName,
  }) async {
    final notificationSettings = _ref.read(notificationSettingsProvider);
    if (!notificationSettings.autoPrintAfterAcceptEnabled) {
      return;
    }

    final receiptPrinter = _ref.read(acceptedOrderReceiptPrinterProvider);
    if (!receiptPrinter.hasConnectedPrinter) {
      return;
    }

    try {
      await receiptPrinter.printAcceptedOrder(order, shopName: shopName);
    } catch (error) {
      debugPrint('Failed to print accepted order receipt: $error');
    }
  }

  OrderModel? _findCurrentOrder(String orderId) {
    return _ref
        .read(orderProvider)
        .orders
        .cast<OrderModel?>()
        .firstWhere(
          (candidate) => candidate?.id == orderId,
          orElse: () => null,
        );
  }

  OrderModel? _orderFromMessage(PushMessage? message) {
    if (message == null) {
      return null;
    }
    return OrderModel(
      id: message.orderId,
      orderNum: message.orderNumber,
      pickupCode: message.pickupCode,
      pickupCodeMasked: message.pickupCodeMasked,
      amount: message.amount,
      feeBreakdown: message.feeBreakdown,
      status: OrderStatus.preparing,
      fulfillmentStatus: FulfillmentStatus.preparing,
      createdAt: message.timestamp,
      items: message.items,
      note: message.note,
      itemsLoadFailed: message.itemsLoadFailed,
    );
  }

  String _resolveShopName(String? requestedShopName) {
    final requested = requestedShopName?.trim() ?? '';
    if (requested.isNotEmpty) {
      return requested;
    }
    final authMerchantName = _ref.read(authProvider).merchantName?.trim() ?? '';
    if (authMerchantName.isNotEmpty) {
      return authMerchantName;
    }
    return '商户工作台';
  }
}
