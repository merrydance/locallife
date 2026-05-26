import 'order.dart';

class PushMessage {
  final String messageId;
  final String orderId;
  final String orderNumber;
  final String? pickupCode;
  final String? pickupCodeMasked;
  final String title;
  final String content;
  final double amount;
  final OrderFeeBreakdown? feeBreakdown;
  final String shopName;
  final String? note;
  final List<OrderItem> items;
  final bool itemsLoadFailed;
  final DateTime timestamp;

  PushMessage({
    required this.messageId,
    required this.orderId,
    this.orderNumber = '',
    this.pickupCode,
    this.pickupCodeMasked,
    required this.title,
    required this.content,
    required this.amount,
    this.feeBreakdown,
    required this.shopName,
    this.note,
    this.items = const <OrderItem>[],
    this.itemsLoadFailed = false,
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  factory PushMessage.fromJson(Map<String, dynamic> json) {
    final normalized = _normalizePushPayload(json);
    final order = OrderModel.fromJson(normalized);
    return PushMessage(
      messageId: normalized['message_id'] ?? '',
      orderId: normalized['order_id']?.toString() ?? order.id,
      orderNumber: order.orderNum.isNotEmpty
          ? order.orderNum
          : normalized['order_id']?.toString() ?? '',
      pickupCode: order.pickupCode,
      pickupCodeMasked: order.pickupCodeMasked,
      title: normalized['title'] ?? '',
      content: normalized['content'] ?? '',
      amount: normalized.containsKey('total_amount')
          ? order.amount
          : _amountFromPush(normalized),
      feeBreakdown: order.feeBreakdown,
      shopName: normalized['shop_name'] ?? '',
      note: order.note,
      items: order.items,
      itemsLoadFailed: order.itemsLoadFailed,
    );
  }

  factory PushMessage.fromOrder(dynamic order, {required String shopName}) {
    return PushMessage(
      messageId: 'polled-${order.id}',
      orderId: order.id,
      orderNumber: order.orderNum,
      pickupCode: order.pickupCode,
      pickupCodeMasked: order.pickupCodeMasked,
      title: '您有新的订单',
      content: '订单号 ${order.displayOrderNumber}，请及时处理',
      amount: order.amount,
      feeBreakdown: order.feeBreakdown,
      shopName: shopName,
      note: order.note,
      items: order.items,
      itemsLoadFailed: order.itemsLoadFailed,
      timestamp: order.createdAt,
    );
  }

  PushMessage withOrderNumber(String nextOrderNumber) {
    return PushMessage(
      messageId: messageId,
      orderId: orderId,
      orderNumber: nextOrderNumber,
      pickupCode: pickupCode,
      pickupCodeMasked: pickupCodeMasked,
      title: title,
      content: content,
      amount: amount,
      feeBreakdown: feeBreakdown,
      shopName: shopName,
      note: note,
      items: items,
      itemsLoadFailed: itemsLoadFailed,
      timestamp: timestamp,
    );
  }

  PushMessage withOrderSnapshot(OrderModel order) {
    return PushMessage(
      messageId: messageId,
      orderId: order.id.isNotEmpty ? order.id : orderId,
      orderNumber: order.orderNum.isNotEmpty ? order.orderNum : orderNumber,
      pickupCode: order.pickupCode ?? pickupCode,
      pickupCodeMasked: order.pickupCodeMasked ?? pickupCodeMasked,
      title: title,
      content: content,
      amount: order.amount > 0 ? order.amount : amount,
      feeBreakdown: order.feeBreakdown ?? feeBreakdown,
      shopName: shopName,
      note: order.note ?? note,
      items: order.items,
      itemsLoadFailed: order.itemsLoadFailed,
      timestamp: timestamp,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'message_id': messageId,
      'order_id': orderId,
      'order_num': orderNumber,
      'pickup_code': pickupCode,
      'pickup_code_masked': pickupCodeMasked,
      'title': title,
      'content': content,
      'amount': amount,
      'fee_breakdown': feeBreakdown?.toJson(),
      'shop_name': shopName,
      'notes': note,
      'items': items
          .map(
            (item) => {
              'name': item.name,
              'quantity': item.quantity,
              'unit_price': item.unitPriceCents,
              'subtotal': item.subtotalCents,
              'specs_text': item.specsText,
            },
          )
          .toList(),
      'items_load_failed': itemsLoadFailed,
    };
  }

  String get displayOrderNumber {
    final pickup = pickupCode?.trim() ?? '';
    if (pickup.isNotEmpty) {
      return pickup;
    }
    final masked = pickupCodeMasked?.trim() ?? '';
    if (masked.isNotEmpty) {
      return masked;
    }
    return orderNumber.isNotEmpty ? orderNumber : orderId;
  }

  int get notificationId => Object.hash(orderId, messageId);
}

double _amountFromPush(Map<String, dynamic> json) {
  final raw = json['amount'];
  if (raw is int) return raw / 100.0;
  if (raw is num) return raw.toDouble();
  final parsed = double.tryParse(raw?.toString() ?? '');
  if (parsed == null) return 0.0;
  return parsed >= 100 ? parsed / 100.0 : parsed;
}

Map<String, dynamic> _normalizePushPayload(Map<String, dynamic> json) {
  final extraData = _mapFromValue(json['extra_data']);
  if (extraData == null) {
    return json;
  }

  final relatedType = json['related_type']?.toString().toLowerCase();
  final relatedId = json['related_id'];
  if (relatedType != 'order' || relatedId == null) {
    return json;
  }

  final normalized = <String, dynamic>{
    ...extraData,
    'order_id': extraData['order_id'] ?? relatedId,
    'id': extraData['order_id'] ?? relatedId,
  };
  for (final key in const ['message_id', 'title', 'content']) {
    normalized[key] ??= json[key];
  }
  return normalized;
}

Map<String, dynamic>? _mapFromValue(dynamic value) {
  if (value is Map) {
    return Map<String, dynamic>.from(value);
  }
  return null;
}
