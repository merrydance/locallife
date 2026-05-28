import 'dart:convert';

import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/models/push_message.dart';
import 'package:shared_preferences/shared_preferences.dart';

final pendingOrderAlertStoreProvider = Provider<PendingOrderAlertStore>((ref) {
  return SharedPreferencesPendingOrderAlertStore();
});

class PendingOrderAlert {
  final PushMessage message;
  final String source;
  final DateTime createdAt;

  PendingOrderAlert({
    required this.message,
    required this.source,
    DateTime? createdAt,
  }) : createdAt = createdAt ?? DateTime.now();

  String get orderId => message.orderId;

  String get messageId => message.messageId;

  Map<String, dynamic> toJson() {
    return <String, dynamic>{
      'message': message.toJson(),
      'source': source,
      'created_at': createdAt.toIso8601String(),
    };
  }

  factory PendingOrderAlert.fromJson(Map<String, dynamic> json) {
    final rawMessage = json['message'];
    return PendingOrderAlert(
      message: PushMessage.fromJson(
        rawMessage is Map<String, dynamic>
            ? rawMessage
            : Map<String, dynamic>.from(rawMessage as Map),
      ),
      source: json['source']?.toString() ?? 'unknown',
      createdAt: DateTime.tryParse(json['created_at']?.toString() ?? ''),
    );
  }
}

abstract class PendingOrderAlertStore {
  Future<void> save(PushMessage message, {required String source});

  Future<List<PendingOrderAlert>> loadPendingAlerts();

  Future<bool> hasPending(PushMessage message);

  Future<void> remove(PushMessage message);
}

class SharedPreferencesPendingOrderAlertStore
    implements PendingOrderAlertStore {
  static const _prefix = 'pending_order_alert.';

  @override
  Future<void> save(PushMessage message, {required String source}) async {
    final preferences = await SharedPreferences.getInstance();
    final alert = PendingOrderAlert(message: message, source: source);
    await preferences.setString(_keyFor(message), jsonEncode(alert.toJson()));
  }

  @override
  Future<List<PendingOrderAlert>> loadPendingAlerts() async {
    final preferences = await SharedPreferences.getInstance();
    final alerts = <PendingOrderAlert>[];
    for (final key in preferences.getKeys()) {
      if (!key.startsWith(_prefix)) {
        continue;
      }
      final raw = preferences.getString(key);
      if (raw == null || raw.isEmpty) {
        continue;
      }
      final decoded = jsonDecode(raw);
      if (decoded is! Map) {
        await preferences.remove(key);
        continue;
      }
      alerts.add(
        PendingOrderAlert.fromJson(Map<String, dynamic>.from(decoded)),
      );
    }
    alerts.sort((left, right) => left.createdAt.compareTo(right.createdAt));
    return alerts;
  }

  @override
  Future<bool> hasPending(PushMessage message) async {
    final preferences = await SharedPreferences.getInstance();
    if (preferences.containsKey(_keyFor(message))) {
      return true;
    }
    final orderId = message.orderId.trim();
    if (orderId.isEmpty) {
      return false;
    }
    for (final alert in await loadPendingAlerts()) {
      if (alert.orderId == orderId) {
        return true;
      }
    }
    return false;
  }

  @override
  Future<void> remove(PushMessage message) async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.remove(_keyFor(message));
    final orderId = message.orderId.trim();
    if (orderId.isEmpty) {
      return;
    }
    for (final alert in await loadPendingAlerts()) {
      if (alert.orderId == orderId) {
        await preferences.remove(_keyFor(alert.message));
      }
    }
  }

  String _keyFor(PushMessage message) {
    final messageId = message.messageId.trim();
    if (messageId.isNotEmpty) {
      return '$_prefix$messageId';
    }
    return '${_prefix}order:${message.orderId}';
  }
}

class MemoryPendingOrderAlertStore implements PendingOrderAlertStore {
  final Map<String, PendingOrderAlert> _alerts = <String, PendingOrderAlert>{};

  @override
  Future<void> save(PushMessage message, {required String source}) async {
    _alerts[_keyFor(message)] = PendingOrderAlert(
      message: message,
      source: source,
    );
  }

  @override
  Future<List<PendingOrderAlert>> loadPendingAlerts() async {
    final alerts = _alerts.values.toList();
    alerts.sort((left, right) => left.createdAt.compareTo(right.createdAt));
    return alerts;
  }

  @override
  Future<bool> hasPending(PushMessage message) async {
    if (_alerts.containsKey(_keyFor(message))) {
      return true;
    }
    final orderId = message.orderId.trim();
    if (orderId.isEmpty) {
      return false;
    }
    return _alerts.values.any((alert) => alert.orderId == orderId);
  }

  @override
  Future<void> remove(PushMessage message) async {
    _alerts.remove(_keyFor(message));
    final orderId = message.orderId.trim();
    if (orderId.isEmpty) {
      return;
    }
    _alerts.removeWhere((_, alert) => alert.orderId == orderId);
  }

  String _keyFor(PushMessage message) {
    final messageId = message.messageId.trim();
    if (messageId.isNotEmpty) {
      return messageId;
    }
    return 'order:${message.orderId}';
  }
}
