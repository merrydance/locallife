import 'package:intl/intl.dart';

enum OrderStatus {
  pending('pending', '待支付'),
  paid('paid', '待接单'),
  accepted('accepted', '已接单'),
  preparing('preparing', '制作中'),
  ready('ready', '待配送/待取餐'),
  courierAccepted('courier_accepted', '骑手已接单'),
  picked('picked', '已取餐'),
  delivering('delivering', '配送中'),
  riderDelivered('rider_delivered', '骑手已送达'),
  userDelivered('user_delivered', '用户已确认送达'),
  completed('completed', '已完成'),
  cancelled('cancelled', '已取消'),
  unknown('unknown', '未知状态');

  final String backendValue;
  final String label;
  const OrderStatus(this.backendValue, this.label);

  static OrderStatus fromString(String value) {
    final normalized = value.trim().toLowerCase();
    return OrderStatus.values.firstWhere(
      (e) => e.backendValue == normalized || e.name == normalized,
      orElse: () => OrderStatus.unknown,
    );
  }
}

class OrderModel {
  final String id;
  final String orderNum;
  final double amount;
  final OrderFeeBreakdown? feeBreakdown;
  final OrderStatus status;
  final DateTime createdAt;
  final String? userName;
  final String? userPhone;
  final List<OrderItem> items;
  final String? note;
  final bool itemsLoadFailed;
  final FeeBreakdown? feeBreakdown;

  OrderModel({
    required this.id,
    required this.orderNum,
    required this.amount,
    this.feeBreakdown,
    required this.status,
    required this.createdAt,
    this.userName,
    this.userPhone,
    required this.items,
    this.note,
    this.itemsLoadFailed = false,
    this.feeBreakdown,
  });

  factory OrderModel.fromJson(Map<String, dynamic> json) {
    return OrderModel(
      id: _firstString(json, const ['id', 'order_id']),
      orderNum: _firstString(json, const [
        'order_no',
        'order_num',
        'order_number',
      ]),
      amount: _moneyYuan(
        json,
        centsKeys: const ['total_amount'],
        yuanKeys: const ['amount'],
      ),
      feeBreakdown: _feeBreakdownFromJson(json['fee_breakdown']),
      status: OrderStatus.fromString(json['status']?.toString() ?? 'unknown'),
      createdAt:
          DateTime.tryParse(json['created_at']?.toString() ?? '') ??
          DateTime.now(),
      userName: json['user_name']?.toString(),
      userPhone: json['user_phone']?.toString(),
      items:
          (json['items'] as List?)
              ?.whereType<Map>()
              .map(
                (item) => OrderItem.fromJson(Map<String, dynamic>.from(item)),
              )
              .toList() ??
          [],
      note: _firstNullableString(json, const ['notes', 'note']),
      itemsLoadFailed: json['items_load_failed'] == true,
      feeBreakdown: FeeBreakdown.fromJsonOrNull(json['fee_breakdown']),
    );
  }

  String get formattedDate => DateFormat('MM-dd HH:mm').format(createdAt);

  bool get hasReliableItems => !itemsLoadFailed;

  bool get isAwaitingAcceptance => status == OrderStatus.paid;

  double get merchantFoodDisplayAmount =>
      feeBreakdown?.foodPayableAmount ?? amount;

  double get deliveryFeeDisplayAmount =>
      feeBreakdown?.deliveryPayableAmount ?? 0.0;

  bool get hasFeeBreakdown => feeBreakdown != null;
}

class OrderFeeBreakdown {
  final double foodAmount;
  final double merchantDiscountAmount;
  final double voucherDiscountAmount;
  final double foodPayableAmount;
  final double deliveryFeeAmount;
  final double deliveryFeeDiscountAmount;
  final double deliveryPayableAmount;
  final double customerPayableAmount;
  final double platformServiceFeeAmount;
  final double paymentChannelFeeAmount;
  final double merchantReceivableAmount;

  const OrderFeeBreakdown({
    required this.foodAmount,
    required this.merchantDiscountAmount,
    required this.voucherDiscountAmount,
    required this.foodPayableAmount,
    required this.deliveryFeeAmount,
    required this.deliveryFeeDiscountAmount,
    required this.deliveryPayableAmount,
    required this.customerPayableAmount,
    required this.platformServiceFeeAmount,
    required this.paymentChannelFeeAmount,
    required this.merchantReceivableAmount,
  });

  factory OrderFeeBreakdown.fromJson(Map<String, dynamic> json) {
    return OrderFeeBreakdown(
      foodAmount: _moneyYuan(json, centsKeys: const ['food_amount']),
      merchantDiscountAmount: _moneyYuan(
        json,
        centsKeys: const ['merchant_discount_amount'],
      ),
      voucherDiscountAmount: _moneyYuan(
        json,
        centsKeys: const ['voucher_discount_amount'],
      ),
      foodPayableAmount: _moneyYuan(
        json,
        centsKeys: const ['food_payable_amount'],
      ),
      deliveryFeeAmount: _moneyYuan(
        json,
        centsKeys: const ['delivery_fee_amount'],
      ),
      deliveryFeeDiscountAmount: _moneyYuan(
        json,
        centsKeys: const ['delivery_fee_discount_amount'],
      ),
      deliveryPayableAmount: _moneyYuan(
        json,
        centsKeys: const ['delivery_payable_amount'],
      ),
      customerPayableAmount: _moneyYuan(
        json,
        centsKeys: const ['customer_payable_amount'],
      ),
      platformServiceFeeAmount: _moneyYuan(
        json,
        centsKeys: const ['platform_service_fee_amount'],
      ),
      paymentChannelFeeAmount: _moneyYuan(
        json,
        centsKeys: const ['payment_channel_fee_amount'],
      ),
      merchantReceivableAmount: _moneyYuan(
        json,
        centsKeys: const ['merchant_receivable_amount'],
      ),
    );
  }

  Map<String, int> toJson() {
    return {
      'food_amount': _yuanToCents(foodAmount),
      'merchant_discount_amount': _yuanToCents(merchantDiscountAmount),
      'voucher_discount_amount': _yuanToCents(voucherDiscountAmount),
      'food_payable_amount': _yuanToCents(foodPayableAmount),
      'delivery_fee_amount': _yuanToCents(deliveryFeeAmount),
      'delivery_fee_discount_amount': _yuanToCents(deliveryFeeDiscountAmount),
      'delivery_payable_amount': _yuanToCents(deliveryPayableAmount),
      'customer_payable_amount': _yuanToCents(customerPayableAmount),
      'platform_service_fee_amount': _yuanToCents(platformServiceFeeAmount),
      'payment_channel_fee_amount': _yuanToCents(paymentChannelFeeAmount),
      'merchant_receivable_amount': _yuanToCents(merchantReceivableAmount),
    };
  }
}

class FeeBreakdown {
  final int customerPayableAmountCents;
  final int platformServiceFeeAmountCents;
  final int paymentChannelFeeAmountCents;
  final int merchantReceivableAmountCents;
  final int deliveryFeeAmountCents;
  final int riderGrossAmountCents;
  final int riderPaymentFeeCents;
  final int riderNetEarningsCents;

  const FeeBreakdown({
    required this.customerPayableAmountCents,
    required this.platformServiceFeeAmountCents,
    required this.paymentChannelFeeAmountCents,
    required this.merchantReceivableAmountCents,
    this.deliveryFeeAmountCents = 0,
    this.riderGrossAmountCents = 0,
    this.riderPaymentFeeCents = 0,
    this.riderNetEarningsCents = 0,
  });

  factory FeeBreakdown.fromJson(Map<String, dynamic> json) {
    return FeeBreakdown(
      customerPayableAmountCents: _firstInt(json, const [
        'customer_payable_amount',
      ]),
      platformServiceFeeAmountCents: _firstInt(json, const [
        'platform_service_fee_amount',
      ]),
      paymentChannelFeeAmountCents: _firstInt(json, const [
        'payment_channel_fee_amount',
      ]),
      merchantReceivableAmountCents: _firstInt(json, const [
        'merchant_receivable_amount',
      ]),
      deliveryFeeAmountCents: _firstInt(json, const ['delivery_fee_amount']),
      riderGrossAmountCents: _firstInt(json, const [
        'rider_gross_amount',
        'delivery_fee_amount',
      ]),
      riderPaymentFeeCents: _firstInt(json, const [
        'rider_payment_fee_amount',
        'rider_payment_fee',
      ]),
      riderNetEarningsCents: _firstInt(json, const [
        'rider_net_earnings_amount',
        'rider_amount',
        'rider_net_earnings',
      ]),
    );
  }

  static FeeBreakdown? fromJsonOrNull(dynamic value) {
    if (value is Map) {
      return FeeBreakdown.fromJson(Map<String, dynamic>.from(value));
    }
    return null;
  }

  Map<String, dynamic> toJson() {
    return {
      'customer_payable_amount': customerPayableAmountCents,
      'platform_service_fee_amount': platformServiceFeeAmountCents,
      'payment_channel_fee_amount': paymentChannelFeeAmountCents,
      'merchant_receivable_amount': merchantReceivableAmountCents,
      'delivery_fee_amount': deliveryFeeAmountCents,
      'rider_gross_amount': riderGrossAmountCents,
      'rider_payment_fee_amount': riderPaymentFeeCents,
      'rider_net_earnings_amount': riderNetEarningsCents,
    };
  }
}

class OrderItem {
  final String name;
  final int quantity;
  final double price;
  final double subtotal;
  final int unitPriceCents;
  final int subtotalCents;
  final String specsText;

  OrderItem({
    required this.name,
    required this.quantity,
    required this.price,
    this.subtotal = 0.0,
    this.unitPriceCents = 0,
    this.subtotalCents = 0,
    this.specsText = '',
  });

  factory OrderItem.fromJson(Map<String, dynamic> json) {
    final unitPriceCents = _firstInt(json, const ['unit_price']);
    final subtotalCents = _firstInt(json, const ['subtotal']);
    return OrderItem(
      name: json['name']?.toString() ?? '',
      quantity: _firstInt(json, const ['quantity'], fallback: 1),
      price: unitPriceCents > 0
          ? unitPriceCents / 100.0
          : _moneyYuan(json, yuanKeys: const ['price']),
      subtotal: subtotalCents > 0
          ? subtotalCents / 100.0
          : _moneyYuan(json, yuanKeys: const ['subtotal_price']),
      unitPriceCents: unitPriceCents,
      subtotalCents: subtotalCents,
      specsText: json['specs_text']?.toString() ?? '',
    );
  }

  double get lineTotal => subtotal > 0 ? subtotal : price * quantity;
}

String _firstString(Map<String, dynamic> json, List<String> keys) {
  return _firstNullableString(json, keys) ?? '';
}

String? _firstNullableString(Map<String, dynamic> json, List<String> keys) {
  for (final key in keys) {
    final value = json[key];
    if (value != null && value.toString().isNotEmpty) return value.toString();
  }
  return null;
}

int _firstInt(
  Map<String, dynamic> json,
  List<String> keys, {
  int fallback = 0,
}) {
  for (final key in keys) {
    final value = json[key];
    if (value is int) return value;
    if (value is num) return value.toInt();
    final parsed = int.tryParse(value?.toString() ?? '');
    if (parsed != null) return parsed;
  }
  return fallback;
}

double _moneyYuan(
  Map<String, dynamic> json, {
  List<String> centsKeys = const [],
  List<String> yuanKeys = const [],
}) {
  for (final key in centsKeys) {
    final cents = _firstInt(json, [key], fallback: -1);
    if (cents >= 0) return cents / 100.0;
  }
  for (final key in yuanKeys) {
    final value = json[key];
    if (value is num) return value.toDouble();
    final parsed = double.tryParse(value?.toString() ?? '');
    if (parsed != null) return parsed;
  }
  return 0.0;
}

OrderFeeBreakdown? _feeBreakdownFromJson(dynamic value) {
  if (value is! Map) {
    return null;
  }
  return OrderFeeBreakdown.fromJson(Map<String, dynamic>.from(value));
}

int _yuanToCents(double value) => (value * 100).round();
