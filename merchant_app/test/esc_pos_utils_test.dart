import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/print/esc_pos_utils.dart';
import 'package:merchant_app/models/order.dart';
import 'package:merchant_app/models/push_message.dart';

void main() {
  test('generateOrderReceipt prints items specs notes and total', () {
    final bytes = EscPosUtils.generateOrderReceipt(
      PushMessage(
        messageId: 'merchant:new_order:501',
        orderId: '501',
        orderNumber: 'ORD501',
        title: '新订单',
        content: '您有一笔新订单 ORD501，请及时处理',
        amount: 88,
        shopName: '测试门店',
        note: '少放葱',
        items: [
          OrderItem(
            name: '测试菜品',
            quantity: 2,
            price: 28,
            subtotal: 56,
            unitPriceCents: 2800,
            subtotalCents: 5600,
            specsText: '大份 / 少辣',
          ),
        ],
      ),
    );

    final text = utf8.decode(bytes, allowMalformed: true);

    expect(text, contains('订单编号: ORD501'));
    expect(text, contains('测试菜品 x2'));
    expect(text, contains('规格: 大份 / 少辣'));
    expect(text, contains('备注: 少放葱'));
    expect(text, contains('订单总额: ¥88.00'));
  });

  test('generateOrderReceipt refuses incomplete item details', () {
    expect(
      () => EscPosUtils.generateOrderReceipt(
        PushMessage(
          messageId: 'merchant:new_order:502',
          orderId: '502',
          title: '新订单',
          content: '您有一笔新订单，请及时处理',
          amount: 99,
          shopName: '测试门店',
          itemsLoadFailed: true,
        ),
      ),
      throwsA(isA<StateError>()),
    );
  });
}
