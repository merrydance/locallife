import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:dio/dio.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/features/order/order_alert_page.dart';
import 'package:merchant_app/features/order/order_alert_coordinator.dart';
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/models/push_message.dart';

void main() {
  group('OrderAlertCoordinator.identifyNewAwaitingAcceptanceOrders', () {
    test(
      'only returns paid orders that were not awaiting acceptance before',
      () {
        final previousOrders = [
          _buildOrder(id: 'order-1', status: OrderStatus.paid),
          _buildOrder(id: 'order-2', status: OrderStatus.preparing),
        ];

        final latestOrders = [
          _buildOrder(id: 'order-1', status: OrderStatus.paid),
          _buildOrder(id: 'order-2', status: OrderStatus.paid),
          _buildOrder(id: 'order-3', status: OrderStatus.paid),
          _buildOrder(id: 'order-4', status: OrderStatus.pending),
          _buildOrder(id: 'order-5', status: OrderStatus.preparing),
        ];

        final result =
            OrderAlertCoordinator.identifyNewAwaitingAcceptanceOrders(
              previousOrders: previousOrders,
              latestOrders: latestOrders,
            );

        expect(result.map((order) => order.id).toList(), [
          'order-2',
          'order-3',
        ]);
      },
    );
  });

  group('PushMessage.fromOrder', () {
    test('keeps real id for actions and order number for display', () {
      final order = _buildOrder(
        id: 'internal-id-1',
        orderNum: 'LKLF-20260412-001',
        status: OrderStatus.paid,
      );

      final message = PushMessage.fromOrder(order, shopName: '测试门店');

      expect(message.orderId, 'internal-id-1');
      expect(message.displayOrderNumber, 'LKLF-20260412-001');
      expect(message.shopName, '测试门店');
    });
  });

  group('OrderModel.fromJson', () {
    test('normalizes backend order fields with item specs and notes', () {
      final order = OrderModel.fromJson({
        'id': 501,
        'order_no': 'ORD501',
        'total_amount': 8800,
        'status': 'paid',
        'created_at': '2026-04-12T08:00:00Z',
        'delivery_contact_name': '张三',
        'delivery_contact_phone': '13800138000',
        'notes': '少放葱',
        'fee_breakdown': {
          'customer_payable_amount': 8800,
          'platform_service_fee_amount': 475,
          'payment_channel_fee_amount': 31,
          'merchant_receivable_amount': 8294,
          'rider_gross_amount': 800,
          'rider_payment_fee_amount': 5,
          'rider_net_earnings_amount': 795,
        },
        'items': [
          {
            'id': 801,
            'name': '测试菜品',
            'quantity': 2,
            'unit_price': 2800,
            'subtotal': 5600,
            'specs_text': '大份 / 少辣',
          },
        ],
      });

      expect(order.id, '501');
      expect(order.orderNum, 'ORD501');
      expect(order.amount, 88.0);
      expect(order.status, OrderStatus.paid);
      expect(order.isAwaitingAcceptance, isTrue);
      expect(order.userName, '张三');
      expect(order.userPhone, '13800138000');
      expect(order.note, '少放葱');
      expect(order.hasReliableItems, isTrue);
      expect(order.feeBreakdown, isNotNull);
      expect(order.feeBreakdown!.customerPayableAmountCents, 8800);
      expect(order.feeBreakdown!.platformServiceFeeAmountCents, 475);
      expect(order.feeBreakdown!.paymentChannelFeeAmountCents, 31);
      expect(order.feeBreakdown!.merchantReceivableAmountCents, 8294);
      expect(order.feeBreakdown!.riderGrossAmountCents, 800);
      expect(order.feeBreakdown!.riderPaymentFeeCents, 5);
      expect(order.feeBreakdown!.riderNetEarningsCents, 795);
      expect(order.items.single.price, 28.0);
      expect(order.items.single.subtotal, 56.0);
      expect(order.items.single.specsText, '大份 / 少辣');
    });

    test('parses merchant fee breakdown from backend cents', () {
      final order = OrderModel.fromJson({
        'id': 501,
        'order_no': 'ORD501',
        'total_amount': 10300,
        'status': 'paid',
        'created_at': '2026-04-12T08:00:00Z',
        'fee_breakdown': {
          'food_amount': 10000,
          'merchant_discount_amount': 300,
          'voucher_discount_amount': 200,
          'food_payable_amount': 9500,
          'delivery_fee_amount': 800,
          'delivery_fee_discount_amount': 0,
          'delivery_payable_amount': 800,
          'customer_payable_amount': 10300,
          'platform_service_fee_amount': 475,
          'payment_channel_fee_amount': 57,
          'merchant_receivable_amount': 8968,
        },
      });

      expect(order.amount, 103.0);
      expect(order.feeBreakdown, isNotNull);
      expect(order.feeBreakdown!.foodPayableAmount, 95.0);
      expect(order.feeBreakdown!.deliveryPayableAmount, 8.0);
      expect(order.feeBreakdown!.platformServiceFeeAmount, 4.75);
      expect(order.feeBreakdown!.paymentChannelFeeAmount, 0.57);
      expect(order.feeBreakdown!.merchantReceivableAmount, 89.68);
    });

    test('keeps legacy aliases compatible', () {
      final order = OrderModel.fromJson({
        'order_id': 'legacy-1',
        'order_num': 'LEGACY001',
        'amount': 18.5,
        'status': 'pending',
        'created_at': '2026-04-12T08:00:00Z',
        'note': '不要香菜',
        'items_load_failed': true,
        'items': [
          {'name': '旧菜品', 'quantity': 1, 'price': 18.5},
        ],
      });

      expect(order.id, 'legacy-1');
      expect(order.orderNum, 'LEGACY001');
      expect(order.amount, 18.5);
      expect(order.status, OrderStatus.pending);
      expect(order.isAwaitingAcceptance, isFalse);
      expect(order.note, '不要香菜');
      expect(order.hasReliableItems, isFalse);
      expect(order.items.single.price, 18.5);
    });

    test(
      'does not default missing or unknown backend status to accept action',
      () {
        final missingStatusOrder = OrderModel.fromJson({
          'id': 'missing-status',
          'order_no': 'ORD-MISSING',
          'total_amount': 1800,
          'created_at': '2026-04-12T08:00:00Z',
        });
        final unknownStatusOrder = OrderModel.fromJson({
          'id': 'unknown-status',
          'order_no': 'ORD-UNKNOWN',
          'total_amount': 1800,
          'status': 'reviewing',
          'created_at': '2026-04-12T08:00:00Z',
        });

        expect(missingStatusOrder.status, OrderStatus.unknown);
        expect(missingStatusOrder.isAwaitingAcceptance, isFalse);
        expect(unknownStatusOrder.status, OrderStatus.unknown);
        expect(unknownStatusOrder.isAwaitingAcceptance, isFalse);
      },
    );
  });

  group('PushMessage.fromJson', () {
    test('carries backend snapshot items and specs', () {
      final message = PushMessage.fromJson({
        'message_id': 'merchant:new_order:501',
        'order_id': 501,
        'order_no': 'ORD501',
        'title': '新订单',
        'content': '您有一笔新订单 ORD501，请及时处理',
        'amount': 8800,
        'shop_name': '测试门店',
        'notes': '少放葱',
        'fee_breakdown': {
          'customer_payable_amount': 8800,
          'platform_service_fee_amount': 475,
          'payment_channel_fee_amount': 31,
          'merchant_receivable_amount': 8294,
          'rider_gross_amount': 800,
          'rider_payment_fee_amount': 5,
          'rider_net_earnings_amount': 795,
        },
        'items': [
          {
            'name': '测试菜品',
            'quantity': 2,
            'unit_price': 2800,
            'subtotal': 5600,
            'specs_text': '大份 / 少辣',
          },
        ],
      });

      expect(message.messageId, 'merchant:new_order:501');
      expect(message.orderId, '501');
      expect(message.displayOrderNumber, 'ORD501');
      expect(message.amount, 88.0);
      expect(message.note, '少放葱');
      expect(message.feeBreakdown, isNotNull);
      expect(message.feeBreakdown!.merchantReceivableAmountCents, 8294);
      expect(message.feeBreakdown!.riderGrossAmountCents, 800);
      expect(message.feeBreakdown!.riderPaymentFeeCents, 5);
      expect(message.feeBreakdown!.riderNetEarningsCents, 795);
      expect(message.items.single.specsText, '大份 / 少辣');
      expect(message.itemsLoadFailed, isFalse);
    });

    test('carries fee breakdown through push hydration', () {
      final message = PushMessage.fromJson({
        'message_id': 'merchant:new_order:501',
        'order_id': 501,
        'order_no': 'ORD501',
        'amount': 10300,
        'shop_name': '测试门店',
        'fee_breakdown': {
          'food_payable_amount': 9500,
          'delivery_payable_amount': 800,
          'platform_service_fee_amount': 475,
          'payment_channel_fee_amount': 57,
          'merchant_receivable_amount': 8968,
        },
      });

      expect(message.feeBreakdown, isNotNull);
      expect(message.feeBreakdown!.foodPayableAmount, 95.0);

      final hydrated = message.withOrderSnapshot(
        OrderModel.fromJson({
          'id': 501,
          'order_no': 'ORD501',
          'total_amount': 10300,
          'fee_breakdown': {
            'food_payable_amount': 9600,
            'delivery_payable_amount': 700,
            'platform_service_fee_amount': 480,
            'payment_channel_fee_amount': 58,
            'merchant_receivable_amount': 9062,
          },
        }),
      );

      expect(hydrated.feeBreakdown!.foodPayableAmount, 96.0);
      expect(hydrated.feeBreakdown!.deliveryPayableAmount, 7.0);
    });

    test(
      'keeps message identity when replacing snapshot from detail order',
      () {
        final message = PushMessage.fromJson({
          'message_id': 'merchant:new_order:501',
          'order_id': 501,
          'amount': 8800,
          'shop_name': '测试门店',
        });
        final order = OrderModel.fromJson({
          'id': 501,
          'order_no': 'ORD501',
          'total_amount': 8800,
          'notes': '少放葱',
          'fee_breakdown': {
            'customer_payable_amount': 8800,
            'platform_service_fee_amount': 475,
            'payment_channel_fee_amount': 31,
            'merchant_receivable_amount': 8294,
            'rider_gross_amount': 800,
            'rider_payment_fee_amount': 5,
            'rider_net_earnings_amount': 795,
          },
          'items': [
            {
              'name': '测试菜品',
              'quantity': 2,
              'unit_price': 2800,
              'subtotal': 5600,
              'specs_text': '大份 / 少辣',
            },
          ],
        });

        final hydrated = message.withOrderSnapshot(order);

        expect(hydrated.messageId, 'merchant:new_order:501');
        expect(hydrated.displayOrderNumber, 'ORD501');
        expect(hydrated.amount, 88.0);
        expect(hydrated.note, '少放葱');
        expect(hydrated.feeBreakdown, isNotNull);
        expect(hydrated.feeBreakdown!.merchantReceivableAmountCents, 8294);
        expect(hydrated.feeBreakdown!.riderGrossAmountCents, 800);
        expect(hydrated.feeBreakdown!.riderPaymentFeeCents, 5);
        expect(hydrated.feeBreakdown!.riderNetEarningsCents, 795);
        expect(hydrated.items.single.specsText, '大份 / 少辣');
      },
    );
  });

  group('OrderAlertPage', () {
    testWidgets('shows accept reject and defer actions together', (
      tester,
    ) async {
      final message = PushMessage(
        messageId: 'merchant:new_order:501',
        orderId: '501',
        orderNumber: 'ORD501',
        title: '新订单',
        content: '您有一笔新订单',
        amount: 103.0,
        feeBreakdown: const OrderFeeBreakdown(
          foodAmountCents: 10000,
          merchantDiscountAmountCents: 300,
          voucherDiscountAmountCents: 200,
          foodPayableAmountCents: 9500,
          deliveryFeeAmountCents: 800,
          deliveryFeeDiscountAmountCents: 0,
          deliveryPayableAmountCents: 800,
          customerPayableAmountCents: 10300,
          platformServiceFeeAmountCents: 475,
          paymentChannelFeeAmountCents: 57,
          merchantReceivableAmountCents: 8968,
        ),
        shopName: '测试门店',
      );

      await tester.pumpWidget(
        ProviderScope(
          overrides: [apiClientProvider.overrideWithValue(_FakeApiClient())],
          child: MaterialApp(home: OrderAlertPage(message: message)),
        ),
      );

      expect(find.text('立即接单'), findsOneWidget);
      expect(find.text('拒单'), findsOneWidget);
      expect(find.text('稍后处理'), findsOneWidget);
      expect(find.text('¥ 95.00'), findsOneWidget);
      expect(find.text('代取费另列 ¥8.00'), findsOneWidget);
    });
  });
}

class _FakeApiClient implements ApiClient {
  @override
  Future<Response<dynamic>> delete(String path, {bool requiresAuth = true}) {
    throw UnimplementedError();
  }

  @override
  Future<Response<dynamic>> get(
    String path, {
    Map<String, dynamic>? queryParameters,
    bool requiresAuth = true,
  }) {
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

OrderModel _buildOrder({
  required String id,
  required OrderStatus status,
  String orderNum = '',
}) {
  return OrderModel(
    id: id,
    orderNum: orderNum,
    amount: 18.5,
    status: status,
    createdAt: DateTime.parse('2026-04-12T08:00:00Z'),
    items: const <OrderItem>[],
  );
}
