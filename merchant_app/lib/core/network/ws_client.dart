import 'dart:async';
import 'dart:convert';
import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:web_socket_channel/io.dart';
import 'package:merchant_app/config/env.dart';
import 'package:merchant_app/core/service/message_dedup.dart';
import 'package:merchant_app/models/push_message.dart';

class WsClient {
  final MessageDeduplicator _deduplicator;
  IOWebSocketChannel? _channel;
  Timer? _reconnectTimer;
  bool _isDisposed = false;
  bool _isConnecting = false;
  int _reconnectAttempts = 0;
  String? _currentToken;

  void Function(PushMessage)? onNewOrder;
  void Function(Map<String, dynamic>)? onTableStatusChange;
  void Function(bool)? onStatusChange;

  WsClient(this._deduplicator);

  Future<void> connect(String token) async {
    if (_isDisposed) return;
    if (_isConnecting) return;
    if (_channel != null && _currentToken == token) return;

    _isConnecting = true;
    _currentToken = token;
    final uri = Uri.parse('${Env.wsUrl}?token=$token');
    if (kDebugMode) {
      debugPrint('Connecting to WebSocket: ${uri.replace(query: 'token=***')}');
    }

    try {
      final socket = await WebSocket.connect(uri.toString());
      socket.pingInterval = const Duration(seconds: 20);
      _channel = IOWebSocketChannel(socket);
      _reconnectAttempts = 0;
      onStatusChange?.call(true);

      _channel!.stream.listen(
        (message) async {
          await _handleMessage(message);
        },
        onDone: () {
          if (kDebugMode) {
            debugPrint('WebSocket closed');
          }
          _channel = null;
          onStatusChange?.call(false);
          _scheduleReconnect(token);
        },
        onError: (error) {
          if (kDebugMode) {
            debugPrint('WebSocket error: $error');
          }
          _channel = null;
          onStatusChange?.call(false);
          _scheduleReconnect(token);
        },
      );
    } catch (e) {
      if (kDebugMode) {
        debugPrint('WebSocket connect error: $e');
      }
      _channel = null;
      onStatusChange?.call(false);
      _scheduleReconnect(token);
    } finally {
      _isConnecting = false;
    }
  }

  Future<void> _handleMessage(dynamic message) async {
    try {
      final data = jsonDecode(message);
      // Assuming the format from Go backend
      if (data['type'] == 'order_notification' || data['type'] == 'notification') {
        final payload = data['data'];
        if (payload is! Map) {
          return;
        }
        final messageId = payload['message_id']?.toString();
        final orderId = payload['order_id']?.toString();

        if (messageId != null &&
            orderId != null &&
            await _deduplicator.tryAcceptGroup([
              MessageDeduplicator.messageKey(messageId),
              MessageDeduplicator.orderKey(orderId),
            ])) {
          final pushMsg = PushMessage.fromJson(Map<String, dynamic>.from(payload));
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

  void disconnect() {
    _reconnectTimer?.cancel();
    _reconnectAttempts = 0;
    _currentToken = null;
    _channel?.sink.close();
    _channel = null;
    onStatusChange?.call(false);
  }

  void dispose() {
    _isDisposed = true;
    disconnect();
  }
}
