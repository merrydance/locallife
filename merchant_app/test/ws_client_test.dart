import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/ws_client.dart';

void main() {
  group('extractMerchantNewOrderPayload', () {
    test('accepts backend new_order envelope', () {
      final payload = extractMerchantNewOrderPayload({
        'id': 'merchant:new_order:501',
        'type': 'new_order',
        'data': {
          'order_id': 501,
          'order_no': 'ORD501',
          'event': 'new_order',
          'items': [
            {
              'name': '测试菜品',
              'quantity': 2,
              'unit_price': 2800,
              'subtotal': 5600,
              'specs_text': '大份 / 少辣',
            },
          ],
        },
      });

      expect(payload, isNotNull);
      expect(payload!['message_id'], 'merchant:new_order:501');
      expect(payload['order_id'], 501);
      expect((payload['items'] as List).single['specs_text'], '大份 / 少辣');
    });

    test('keeps legacy notification payload compatible', () {
      final payload = extractMerchantNewOrderPayload({
        'type': 'notification',
        'data': {
          'message_id': 'legacy-message-1',
          'order_id': 'legacy-order-1',
          'title': '新订单',
        },
      });

      expect(payload, isNotNull);
      expect(payload!['message_id'], 'legacy-message-1');
      expect(payload['order_id'], 'legacy-order-1');
    });

    test('ignores unrelated websocket messages', () {
      final payload = extractMerchantNewOrderPayload({
        'type': 'table_status_change',
        'data': {'table_id': 1},
      });

      expect(payload, isNull);
    });
  });
}
