import 'dart:async';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:merchant_app/core/network/ws_client.dart';
import 'package:merchant_app/core/service/message_dedup.dart';
import 'package:web_socket_channel/web_socket_channel.dart';

void main() {
  group('WsClient connection lifecycle', () {
    test(
      'closes stale socket when reconnecting with a refreshed token',
      () async {
        final channels = <_FakeWebSocketChannel>[];
        final statusChanges = <bool>[];
        final client = WsClient(
          connector: (uri) async {
            final channel = _FakeWebSocketChannel();
            channels.add(channel);
            return channel;
          },
        )..onStatusChange = statusChanges.add;

        await client.connect('old-access-token');
        await client.connect('fresh-access-token');
        await Future<void>.delayed(Duration.zero);

        expect(channels, hasLength(2));
        expect(channels.first.fakeSink.isClosed, isTrue);
        expect(channels.last.fakeSink.isClosed, isFalse);
        expect(statusChanges, [true, true]);

        client.dispose();
      },
    );
  });

  group('WsClient message delivery', () {
    test('routes backend table status changes to the table callback', () async {
      final channel = _FakeWebSocketChannel();
      final delivered = <Map<String, dynamic>>[];
      final client = WsClient(connector: (_) async => channel)
        ..onTableStatusChange = delivered.add;

      await client.connect('access-token');
      channel.addMessage(
        '{"type":"table_status_change","data":{"id":11,"table_no":"A01","status":"occupied"}}',
      );
      await Future<void>.delayed(Duration.zero);

      expect(delivered, [
        {'id': 11, 'table_no': 'A01', 'status': 'occupied'},
      ]);

      client.dispose();
    });

    test(
      'does not permanently deduplicate before alert owner succeeds',
      () async {
        final deduplicator = MessageDeduplicator.memoryOnly();
        final channel = _FakeWebSocketChannel();
        final delivered = <String>[];
        final client = WsClient(connector: (_) async => channel)
          ..onNewOrder = (message) {
            delivered.add(message.orderId);
          };

        await client.connect('access-token');
        channel.addMessage(
          '{"id":"merchant:new_order:501","type":"new_order","data":{"order_id":501,"order_no":"ORD501","event":"new_order"}}',
        );
        await Future<void>.delayed(Duration.zero);

        expect(delivered, <String>['501']);
        expect(
          await deduplicator.tryAcceptGroup([
            MessageDeduplicator.messageKey('merchant:new_order:501'),
            MessageDeduplicator.orderKey('501'),
          ]),
          isTrue,
        );

        client.dispose();
      },
    );
  });

  group('isWebSocketAuthenticationFailure', () {
    test('detects expired token websocket handshake failures', () {
      final error = WebSocketException(
        'Connection to "https://example.test/v1/ws?token=***" was not upgraded '
        'to websocket, HTTP status code: 401',
      );

      expect(isWebSocketAuthenticationFailure(error), isTrue);
    });

    test('ignores non-authentication websocket failures', () {
      final error = WebSocketException(
        'Connection to "https://example.test/v1/ws?token=***" failed',
      );

      expect(isWebSocketAuthenticationFailure(error), isFalse);
    });
  });

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

    test(
      'unwraps backend notification rows without using notification id as order id',
      () {
        final payload = extractMerchantNewOrderPayload({
          'id': 9001,
          'type': 'notification',
          'data': {
            'id': 9001,
            'type': 'order',
            'title': '新订单',
            'content': '您有一笔新订单 ORD501，请及时处理',
            'related_type': 'order',
            'related_id': 501,
            'extra_data': {
              'message_id': 'merchant:new_order:501',
              'order_no': 'ORD501',
              'total_amount': 10300,
              'shop_name': '测试门店',
              'fee_breakdown': {'food_payable_amount': 9500},
            },
          },
        });

        expect(payload, isNotNull);
        expect(payload!['message_id'], 'merchant:new_order:501');
        expect(payload['order_id'], 501);
        expect(payload['order_no'], 'ORD501');
        expect(payload['total_amount'], 10300);
        expect(payload['shop_name'], '测试门店');
        expect(payload['fee_breakdown'], isA<Map>());
        expect(payload['id'], isNot(9001));
      },
    );

    test('ignores unrelated notification rows', () {
      final payload = extractMerchantNewOrderPayload({
        'id': 9002,
        'type': 'notification',
        'data': {
          'id': 9002,
          'type': 'system',
          'title': '系统通知',
          'related_type': 'merchant',
          'related_id': 7,
          'extra_data': {'message_id': 'merchant:system:7'},
        },
      });

      expect(payload, isNull);
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

class _FakeWebSocketChannel implements WebSocketChannel {
  final _controller = StreamController<dynamic>();
  late final _FakeWebSocketSink fakeSink = _FakeWebSocketSink(_controller);

  @override
  int? get closeCode => null;

  @override
  String? get closeReason => null;

  @override
  String? get protocol => null;

  @override
  Future<void> get ready => Future<void>.value();

  @override
  WebSocketSink get sink => fakeSink;

  @override
  Stream get stream => _controller.stream;

  void addMessage(Object? message) {
    _controller.add(message);
  }

  @override
  dynamic noSuchMethod(Invocation invocation) => super.noSuchMethod(invocation);
}

class _FakeWebSocketSink implements WebSocketSink {
  final StreamController<dynamic> _controller;
  final Completer<void> _done = Completer<void>();
  bool isClosed = false;

  _FakeWebSocketSink(this._controller);

  @override
  Future get done => _done.future;

  @override
  void add(Object? event) {}

  @override
  void addError(Object error, [StackTrace? stackTrace]) {}

  @override
  Future addStream(Stream stream) async {}

  @override
  Future close([int? closeCode, String? closeReason]) async {
    if (isClosed) {
      return done;
    }

    isClosed = true;
    await _controller.close();
    if (!_done.isCompleted) {
      _done.complete();
    }
    return done;
  }
}
