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