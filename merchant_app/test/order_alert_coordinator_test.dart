import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/features/order/order_alert_coordinator.dart';
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/models/push_message.dart';

void main() {
  group('OrderAlertCoordinator.identifyNewPendingOrders', () {
    test('only returns pending orders that were not pending before', () {
      final previousOrders = [
        _buildOrder(id: 'order-1', status: OrderStatus.pending),
        _buildOrder(id: 'order-2', status: OrderStatus.accepted),
      ];

      final latestOrders = [
        _buildOrder(id: 'order-1', status: OrderStatus.pending),
        _buildOrder(id: 'order-2', status: OrderStatus.pending),
        _buildOrder(id: 'order-3', status: OrderStatus.pending),
        _buildOrder(id: 'order-4', status: OrderStatus.accepted),
      ];

      final result = OrderAlertCoordinator.identifyNewPendingOrders(
        previousOrders: previousOrders,
        latestOrders: latestOrders,
      );

      expect(result.map((order) => order.id).toList(), ['order-2', 'order-3']);
    });
  });

  group('PushMessage.fromOrder', () {
    test('keeps real id for actions and order number for display', () {
      final order = _buildOrder(
        id: 'internal-id-1',
        orderNum: 'LKLF-20260412-001',
        status: OrderStatus.pending,
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
      expect(order.status, OrderStatus.pending);
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
      expect(order.note, '不要香菜');
      expect(order.hasReliableItems, isFalse);
      expect(order.items.single.price, 18.5);
    });
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
