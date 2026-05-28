import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

final orderAlertCheckpointStoreProvider = Provider<OrderAlertCheckpointStore>((
  ref,
) {
  return SharedPreferencesOrderAlertCheckpointStore();
});

abstract class OrderAlertCheckpointStore {
  Future<bool> hasAlerted(String orderId);

  Future<void> markAlerted(String orderId);

  Future<void> clear(String orderId);
}

class SharedPreferencesOrderAlertCheckpointStore
    implements OrderAlertCheckpointStore {
  static const _prefix = 'order_alert_checkpoint.';

  @override
  Future<bool> hasAlerted(String orderId) async {
    final preferences = await SharedPreferences.getInstance();
    return preferences.containsKey(_keyFor(orderId));
  }

  @override
  Future<void> markAlerted(String orderId) async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.setInt(
      _keyFor(orderId),
      DateTime.now().millisecondsSinceEpoch,
    );
  }

  @override
  Future<void> clear(String orderId) async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.remove(_keyFor(orderId));
  }

  String _keyFor(String orderId) => '$_prefix$orderId';
}

class MemoryOrderAlertCheckpointStore implements OrderAlertCheckpointStore {
  final Set<String> _alertedOrderIds = <String>{};

  @override
  Future<bool> hasAlerted(String orderId) async {
    return _alertedOrderIds.contains(orderId);
  }

  @override
  Future<void> markAlerted(String orderId) async {
    _alertedOrderIds.add(orderId);
  }

  @override
  Future<void> clear(String orderId) async {
    _alertedOrderIds.remove(orderId);
  }
}
