import 'package:intl/intl.dart';

enum OrderStatus {
  pending('待接单'),
  accepted('已接单'),
  preparing('制作中'),
  delivering('配送中'),
  completed('已完成'),
  cancelled('已取消');

  final String label;
  const OrderStatus(this.label);

  static OrderStatus fromString(String value) {
    return OrderStatus.values.firstWhere(
      (e) => e.name == value.toLowerCase(),
      orElse: () => OrderStatus.pending,
    );
  }
}

class OrderModel {
  final String id;
  final String orderNum;
  final double amount;
  final OrderStatus status;
  final DateTime createdAt;
  final String? userName;
  final String? userPhone;
  final List<OrderItem> items;
  final String? note;

  OrderModel({
    required this.id,
    required this.orderNum,
    required this.amount,
    required this.status,
    required this.createdAt,
    this.userName,
    this.userPhone,
    required this.items,
    this.note,
  });

  factory OrderModel.fromJson(Map<String, dynamic> json) {
    return OrderModel(
      id: json['id']?.toString() ?? '',
      orderNum: json['order_num']?.toString() ?? '',
      amount: (json['amount'] as num?)?.toDouble() ?? 0.0,
      status: OrderStatus.fromString(json['status']?.toString() ?? 'pending'),
      createdAt: DateTime.tryParse(json['created_at']?.toString() ?? '') ?? DateTime.now(),
      userName: json['user_name'],
      userPhone: json['user_phone'],
      items: (json['items'] as List?)?.map((i) => OrderItem.fromJson(i)).toList() ?? [],
      note: json['note'],
    );
  }

  String get formattedDate => DateFormat('MM-dd HH:mm').format(createdAt);
}

class OrderItem {
  final String name;
  final int quantity;
  final double price;

  OrderItem({
    required this.name,
    required this.quantity,
    required this.price,
  });

  factory OrderItem.fromJson(Map<String, dynamic> json) {
    return OrderItem(
      name: json['name']?.toString() ?? '',
      quantity: json['quantity'] as int? ?? 1,
      price: (json['price'] as num?)?.toDouble() ?? 0.0,
    );
  }
}
