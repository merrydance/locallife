import 'order.dart';

class PushMessage {
  final String messageId;
  final String orderId;
  final String orderNumber;
  final String title;
  final String content;
  final double amount;
  final OrderFeeBreakdown? feeBreakdown;
  final String shopName;
  final String? note;
  final List<OrderItem> items;
  final bool itemsLoadFailed;
  final FeeBreakdown? feeBreakdown;
  final DateTime timestamp;

  PushMessage({
    required this.messageId,
    required this.orderId,
    this.orderNumber = '',
    required this.title,
    required this.content,
    required this.amount,
    this.feeBreakdown,
    required this.shopName,
    this.note,
    this.items = const <OrderItem>[],
    this.itemsLoadFailed = false,
    this.feeBreakdown,
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  factory PushMessage.fromJson(Map<String, dynamic> json) {
    final order = OrderModel.fromJson(json);
    return PushMessage(
      messageId: json['message_id'] ?? '',
      orderId: json['order_id']?.toString() ?? order.id,
      orderNumber: order.orderNum.isNotEmpty
          ? order.orderNum
          : json['order_id']?.toString() ?? '',
      title: json['title'] ?? '',
      content: json['content'] ?? '',
      amount: json.containsKey('total_amount')
          ? order.amount
          : _amountFromPush(json),
      feeBreakdown: order.feeBreakdown,
      shopName: json['shop_name'] ?? '',
      note: order.note,
      items: order.items,
      itemsLoadFailed: order.itemsLoadFailed,
      feeBreakdown: order.feeBreakdown,
    );
  }

  factory PushMessage.fromOrder(dynamic order, {required String shopName}) {
    return PushMessage(
      messageId: 'polled-${order.id}',
      orderId: order.id,
      orderNumber: order.orderNum,
      title: '您有新的订单',
      content:
          '订单号 ${order.orderNum.isNotEmpty ? order.orderNum : order.id}，请及时处理',
      amount: order.amount,
      feeBreakdown: order.feeBreakdown,
      shopName: shopName,
      note: order.note,
      items: order.items,
      itemsLoadFailed: order.itemsLoadFailed,
      feeBreakdown: order.feeBreakdown,
      timestamp: order.createdAt,
    );
  }

  PushMessage withOrderNumber(String nextOrderNumber) {
    return PushMessage(
      messageId: messageId,
      orderId: orderId,
      orderNumber: nextOrderNumber,
      title: title,
      content: content,
      amount: amount,
      feeBreakdown: feeBreakdown,
      shopName: shopName,
      note: note,
      items: items,
      itemsLoadFailed: itemsLoadFailed,
      feeBreakdown: feeBreakdown,
      timestamp: timestamp,
    );
  }

  PushMessage withOrderSnapshot(OrderModel order) {
    return PushMessage(
      messageId: messageId,
      orderId: order.id.isNotEmpty ? order.id : orderId,
      orderNumber: order.orderNum.isNotEmpty ? order.orderNum : orderNumber,
      title: title,
      content: content,
      amount: order.amount > 0 ? order.amount : amount,
      feeBreakdown: order.feeBreakdown ?? feeBreakdown,
      shopName: shopName,
      note: order.note ?? note,
      items: order.items,
      itemsLoadFailed: order.itemsLoadFailed,
      feeBreakdown: order.feeBreakdown ?? feeBreakdown,
      timestamp: timestamp,
    );
  }

  Map<String, dynamic> toJson() {
    return {
      'message_id': messageId,
      'order_id': orderId,
      'order_num': orderNumber,
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
      if (feeBreakdown != null) 'fee_breakdown': feeBreakdown!.toJson(),
    };
  }

  String get displayOrderNumber =>
      orderNumber.isNotEmpty ? orderNumber : orderId;

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
