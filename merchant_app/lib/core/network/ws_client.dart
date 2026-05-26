import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:web_socket_channel/io.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:merchant_app/config/env.dart';
import 'package:merchant_app/core/service/message_dedup.dart';
import 'package:merchant_app/models/push_message.dart';

typedef WsConnector = Future<WebSocketChannel> Function(Uri uri);

class WsClient {
  final MessageDeduplicator _deduplicator;
  final WsConnector _connector;
  WebSocketChannel? _channel;
  Timer? _reconnectTimer;
  bool _isDisposed = false;
  bool _isConnecting = false;
  int _reconnectAttempts = 0;
  String? _currentToken;
  int _connectionGeneration = 0;

  void Function(PushMessage)? onNewOrder;
  void Function(Map<String, dynamic>)? onTableStatusChange;
  void Function(bool)? onStatusChange;
  Future<String?> Function()? onAuthenticationFailure;

  WsClient(this._deduplicator, {WsConnector? connector})
    : _connector = connector ?? _connectWebSocket;

  Future<void> connect(String token) async {
    if (_isDisposed) return;
    if (_isConnecting) return;
    if (_channel != null && _currentToken == token) return;

    _isConnecting = true;
    _reconnectTimer?.cancel();
    if (_channel != null) {
      _connectionGeneration += 1;
      await _channel!.sink.close();
      _channel = null;
    }
    final generation = _connectionGeneration;
    _currentToken = token;
    final uri = Uri.parse('${Env.wsUrl}?token=$token');
    if (kDebugMode) {
      debugPrint('Connecting to WebSocket: ${uri.replace(query: 'token=***')}');
    }

    try {
      final channel = await _connector(uri);
      if (_isDisposed || generation != _connectionGeneration) {
        await channel.sink.close();
        return;
      }

      _channel = channel;
      _reconnectAttempts = 0;
      onStatusChange?.call(true);

      channel.stream.listen(
        (message) async {
          await _handleMessage(message);
        },
        onDone: () {
          if (generation != _connectionGeneration) {
            return;
          }
          if (kDebugMode) {
            debugPrint('WebSocket closed');
          }
          _channel = null;
          onStatusChange?.call(false);
          _scheduleReconnect(token);
        },
        onError: (error) {
          if (generation != _connectionGeneration) {
            return;
          }
          if (kDebugMode) {
            debugPrint('WebSocket error: $error');
          }
          _channel = null;
          onStatusChange?.call(false);
          _scheduleReconnect(token);
        },
      );
    } catch (e) {
      if (generation != _connectionGeneration) {
        return;
      }
      if (kDebugMode) {
        debugPrint('WebSocket connect error: $e');
      }
      _channel = null;
      onStatusChange?.call(false);
      if (isWebSocketAuthenticationFailure(e)) {
        _scheduleAuthenticatedReconnect(token);
      } else {
        _scheduleReconnect(token);
      }
    } finally {
      _isConnecting = false;
    }
  }

  Future<void> _handleMessage(dynamic message) async {
    try {
      final data = jsonDecode(message);
      if (data is! Map) {
        return;
      }

      final payload = extractMerchantNewOrderPayload(
        Map<String, dynamic>.from(data),
      );
      if (payload != null) {
        final messageId = payload['message_id']?.toString();
        final orderId =
            payload['order_id']?.toString() ?? payload['id']?.toString();

        if (messageId != null &&
            orderId != null &&
            await _deduplicator.tryAcceptGroup([
              MessageDeduplicator.messageKey(messageId),
              MessageDeduplicator.orderKey(orderId),
            ])) {
          final pushMsg = PushMessage.fromJson(
            Map<String, dynamic>.from(payload),
          );
          onNewOrder?.call(pushMsg);
        }
      } else if (data['type'] == 'table_status_change') {
        final payload = data['data'];
        if (payload is Map<String, dynamic>) {
          onTableStatusChange?.call(payload);
        }
      }
    } catch (e) {
      if (kDebugMode) {
        debugPrint('Error decoding WS message: $e');
      }
    }
  }

  void _scheduleReconnect(String token) {
    _reconnectTimer?.cancel();
    if (_isDisposed) return;

    _reconnectAttempts += 1;
    final delaySeconds = (_reconnectAttempts * 5).clamp(5, 30);

    _reconnectTimer = Timer(Duration(seconds: delaySeconds), () {
      connect(token);
    });
  }

  void _scheduleAuthenticatedReconnect(String expiredToken) {
    _reconnectTimer?.cancel();
    if (_isDisposed) return;

    _reconnectAttempts += 1;
    final delaySeconds = (_reconnectAttempts * 5).clamp(5, 30);

    _reconnectTimer = Timer(Duration(seconds: delaySeconds), () async {
      final refresh = onAuthenticationFailure;
      if (refresh == null) {
        connect(expiredToken);
        return;
      }

      final freshToken = await refresh();
      if (_isDisposed) return;

      if (freshToken == null || freshToken.isEmpty) {
        return;
      }
      connect(freshToken);
    });
  }

  void disconnect() {
    _reconnectTimer?.cancel();
    _reconnectAttempts = 0;
    _currentToken = null;
    _connectionGeneration += 1;
    _channel?.sink.close();
    _channel = null;
    onStatusChange?.call(false);
  }

  void dispose() {
    _isDisposed = true;
    disconnect();
  }
}

Future<WebSocketChannel> _connectWebSocket(Uri uri) async {
  final socket = await WebSocket.connect(uri.toString());
  socket.pingInterval = const Duration(seconds: 20);
  return IOWebSocketChannel(socket);
}

@visibleForTesting
bool isWebSocketAuthenticationFailure(Object error) {
  if (error is WebSocketException && error.message.contains('401')) {
    return true;
  }

  return error.toString().contains('401');
}

@visibleForTesting
Map<String, dynamic>? extractMerchantNewOrderPayload(
  Map<String, dynamic> data,
) {
  final outerType = data['type']?.toString();
  final rawPayload = data['data'];
  final payload = rawPayload is Map
      ? Map<String, dynamic>.from(rawPayload)
      : Map<String, dynamic>.from(data);
  final innerType = payload['type']?.toString();
  final event = payload['event']?.toString();

  if (outerType == 'notification') {
    return _extractNewOrderFromNotification(data, payload);
  }

  final isNewOrder =
      outerType == 'new_order' ||
      innerType == 'new_order' ||
      event == 'new_order' ||
      outerType == 'order_notification';
  if (!isNewOrder) {
    return null;
  }

  final messageId = payload['message_id']?.toString() ?? data['id']?.toString();
  if (messageId != null && messageId.isNotEmpty) {
    payload['message_id'] = messageId;
  }
  payload['order_id'] ??= payload['id'];
  return payload;
}

Map<String, dynamic>? _extractNewOrderFromNotification(
  Map<String, dynamic> envelope,
  Map<String, dynamic> payload,
) {
  final extraData = _mapFromValue(payload['extra_data']);
  final relatedType = payload['related_type']?.toString().toLowerCase();
  final relatedId = payload['related_id'];

  if (extraData != null && relatedType == 'order' && relatedId != null) {
    final messageId = extraData['message_id']?.toString();
    final event = extraData['event']?.toString();
    final isMerchantNewOrder =
        event == 'new_order' ||
        messageId?.startsWith('merchant:new_order:') == true ||
        extraData.containsKey('order_id') ||
        extraData.containsKey('order_no') ||
        extraData.containsKey('total_amount');
    if (!isMerchantNewOrder) {
      return null;
    }

    final normalized = <String, dynamic>{
      ...extraData,
      'order_id': extraData['order_id'] ?? relatedId,
      'id': extraData['order_id'] ?? relatedId,
    };
    for (final key in const ['message_id', 'title', 'content']) {
      normalized[key] ??= payload[key] ?? envelope[key];
    }
    return normalized;
  }

  final hasLegacyOrderIdentity =
      payload['message_id'] != null && payload['order_id'] != null;
  final legacyEvent = payload['event']?.toString();
  final legacyType = payload['type']?.toString();
  final hasNoLegacyType = legacyEvent == null && legacyType == null;
  if (hasLegacyOrderIdentity &&
      (hasNoLegacyType ||
          legacyEvent == 'new_order' ||
          legacyType == 'new_order' ||
          legacyType == 'order')) {
    return payload;
  }

  return null;
}

Map<String, dynamic>? _mapFromValue(dynamic value) {
  if (value is Map) {
    return Map<String, dynamic>.from(value);
  }
  return null;
}
