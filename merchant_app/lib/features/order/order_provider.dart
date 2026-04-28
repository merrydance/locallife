import 'dart:async';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter/foundation.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/utils/error_handler.dart';
import 'package:merchant_app/models/order.dart';

class OrderState {
  static const Object _unset = Object();

  final List<OrderModel> orders;
  final bool isLoading;
  final String? error;
  final Set<String> actionInFlightOrderIds;

  OrderState({
    this.orders = const [],
    this.isLoading = false,
    this.error,
    this.actionInFlightOrderIds = const <String>{},
  });

  OrderState copyWith({
    List<OrderModel>? orders,
    bool? isLoading,
    Object? error = _unset,
    Set<String>? actionInFlightOrderIds,
  }) {
    return OrderState(
      orders: orders ?? this.orders,
      isLoading: isLoading ?? this.isLoading,
      error: identical(error, _unset) ? this.error : error as String?,
      actionInFlightOrderIds:
          actionInFlightOrderIds ?? this.actionInFlightOrderIds,
    );
  }
}

class OrderNotifier extends StateNotifier<OrderState> {
  final ApiClient _apiClient;
  final Map<String, Future<bool>> _pendingOrderActions = {};

  OrderNotifier(this._apiClient) : super(OrderState());

  Future<List<OrderModel>> fetchOrders() async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final response = await _apiClient.get('/merchant/orders');
      final orders = _extractOrdersFromResponse(response.data);
      state = state.copyWith(orders: orders, isLoading: false);
      return List<OrderModel>.from(orders);
    } catch (e) {
      state = state.copyWith(
        error: ErrorHandler.getErrorMessage(e),
        isLoading: false,
      );
      return const <OrderModel>[];
    }
  }

  Future<OrderModel?> fetchOrderDetail(String orderId) async {
    try {
      final response = await _apiClient.get('/merchant/orders/$orderId');
      final order = _extractOrderFromResponse(response.data);
      if (order != null) {
        addOrUpdateOrder(order);
      }
      return order;
    } catch (e) {
      state = state.copyWith(error: ErrorHandler.getErrorMessage(e));
      return null;
    }
  }

  Future<bool> acceptOrder(String orderId) async {
    return _runSingleFlightAction(orderId, () async {
      try {
        final response = await _apiClient.post(
          '/merchant/orders/$orderId/accept',
        );
        final updatedOrder = _extractOrderFromResponse(response.data);
        if (updatedOrder != null) {
          addOrUpdateOrder(updatedOrder);
          return true;
        }

        state = state.copyWith(
          orders: state.orders.map((o) {
            if (o.id == orderId) {
              return OrderModel(
                id: o.id,
                orderNum: o.orderNum,
                amount: o.amount,
                status: OrderStatus.accepted,
                createdAt: o.createdAt,
                userName: o.userName,
                userPhone: o.userPhone,
                items: o.items,
                note: o.note,
                itemsLoadFailed: o.itemsLoadFailed,
              );
            }
            return o;
          }).toList(),
        );
        return true;
      } catch (e) {
        state = state.copyWith(error: ErrorHandler.getErrorMessage(e));
        return false;
      }
    });
  }

  Future<bool> rejectOrder(String orderId, {required String reason}) async {
    return _runSingleFlightAction(orderId, () async {
      try {
        final response = await _apiClient.post(
          '/merchant/orders/$orderId/reject',
          data: {'reason': reason},
        );
        final updatedOrder = _extractOrderFromResponse(response.data);
        if (updatedOrder != null) {
          addOrUpdateOrder(updatedOrder);
          return true;
        }

        state = state.copyWith(
          orders: state.orders.map((o) {
            if (o.id == orderId) {
              return OrderModel(
                id: o.id,
                orderNum: o.orderNum,
                amount: o.amount,
                status: OrderStatus.cancelled,
                createdAt: o.createdAt,
                userName: o.userName,
                userPhone: o.userPhone,
                items: o.items,
                note: o.note,
                itemsLoadFailed: o.itemsLoadFailed,
              );
            }
            return o;
          }).toList(),
        );
        return true;
      } catch (e) {
        state = state.copyWith(error: ErrorHandler.getErrorMessage(e));
        return false;
      }
    });
  }

  Future<bool> _runSingleFlightAction(
    String orderId,
    Future<bool> Function() action,
  ) {
    final existing = _pendingOrderActions[orderId];
    if (existing != null) {
      return existing;
    }

    final nextInFlight = {...state.actionInFlightOrderIds, orderId};
    state = state.copyWith(actionInFlightOrderIds: nextInFlight, error: null);

    final future = () async {
      try {
        return await action();
      } finally {
        _pendingOrderActions.remove(orderId);
        final remaining = {...state.actionInFlightOrderIds}..remove(orderId);
        state = state.copyWith(actionInFlightOrderIds: remaining);
      }
    }();

    _pendingOrderActions[orderId] = future;
    return future;
  }

  void clearOrders() {
    state = OrderState();
  }

  void addOrUpdateOrder(OrderModel newOrder) {
    final index = state.orders.indexWhere((o) => o.id == newOrder.id);
    if (index >= 0) {
      final newOrders = List<OrderModel>.from(state.orders);
      newOrders[index] = _mergeOrder(newOrders[index], newOrder);
      state = state.copyWith(orders: newOrders);
    } else {
      state = state.copyWith(orders: [newOrder, ...state.orders]);
    }
  }

  OrderModel _mergeOrder(OrderModel existing, OrderModel incoming) {
    final preserveItems =
        incoming.items.isEmpty &&
        existing.items.isNotEmpty &&
        !incoming.itemsLoadFailed;
    return OrderModel(
      id: incoming.id.isNotEmpty ? incoming.id : existing.id,
      orderNum: incoming.orderNum.isNotEmpty
          ? incoming.orderNum
          : existing.orderNum,
      amount: incoming.amount > 0 ? incoming.amount : existing.amount,
      status: incoming.status,
      createdAt: incoming.createdAt,
      userName: incoming.userName ?? existing.userName,
      userPhone: incoming.userPhone ?? existing.userPhone,
      items: preserveItems ? existing.items : incoming.items,
      note: incoming.note ?? existing.note,
      itemsLoadFailed: preserveItems
          ? existing.itemsLoadFailed
          : incoming.itemsLoadFailed,
    );
  }

  List<OrderModel> _extractOrdersFromResponse(dynamic payload) {
    try {
      if (payload is Map<String, dynamic>) {
        final dynamic data = payload['data'];
        final rawOrders = data is List
            ? data
            : data is Map<String, dynamic>
            ? data['orders']
            : null;
        if (rawOrders is List) {
          return rawOrders
              .whereType<Map>()
              .map(
                (json) => OrderModel.fromJson(Map<String, dynamic>.from(json)),
              )
              .toList();
        }
      }
    } catch (error) {
      debugPrint('Failed to parse orders response: $error');
    }
    return const <OrderModel>[];
  }

  OrderModel? _extractOrderFromResponse(dynamic payload) {
    try {
      if (payload is Map<String, dynamic>) {
        final dynamic data = payload['data'];
        if (data is Map<String, dynamic>) {
          return OrderModel.fromJson(data);
        }
        if (payload.containsKey('id') && payload.containsKey('status')) {
          return OrderModel.fromJson(payload);
        }
      }
    } catch (error) {
      debugPrint('Failed to parse order response: $error');
    }
    return null;
  }

  @override
  void dispose() {
    _pendingOrderActions.clear();
    super.dispose();
  }
}

final orderProvider = StateNotifierProvider<OrderNotifier, OrderState>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return OrderNotifier(apiClient);
});
