class PushMessage {
  final String messageId;
  final String orderId;
  final String orderNumber;
  final String title;
  final String content;
  final double amount;
  final String shopName;
  final DateTime timestamp;

  PushMessage({
    required this.messageId,
    required this.orderId,
    this.orderNumber = '',
    required this.title,
    required this.content,
    required this.amount,
    required this.shopName,
    DateTime? timestamp,
  }) : timestamp = timestamp ?? DateTime.now();

  factory PushMessage.fromJson(Map<String, dynamic> json) {
    return PushMessage(
      messageId: json['message_id'] ?? '',
      orderId: json['order_id'] ?? '',
      orderNumber: json['order_num'] ?? json['order_number'] ?? json['order_id'] ?? '',
      title: json['title'] ?? '',
      content: json['content'] ?? '',
      amount: double.tryParse(json['amount']?.toString() ?? '0') ?? 0.0,
      shopName: json['shop_name'] ?? '',
    );
  }

  factory PushMessage.fromOrder(
    dynamic order, {
    required String shopName,
  }) {
    return PushMessage(
      messageId: 'polled-${order.id}',
      orderId: order.id,
      orderNumber: order.orderNum,
      title: '您有新的订单',
      content: '订单号 ${order.orderNum.isNotEmpty ? order.orderNum : order.id}，请及时处理',
      amount: order.amount,
      shopName: shopName,
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
      shopName: shopName,
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
      'shop_name': shopName,
    };
  }

  String get displayOrderNumber => orderNumber.isNotEmpty ? orderNumber : orderId;

  int get notificationId => Object.hash(orderId, messageId);
}
