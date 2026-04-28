import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/utils/error_handler.dart';
import 'package:merchant_app/features/table/models/table_model.dart';

class TableState {
  final List<TableModel> tables;
  final bool isLoading;
  final String? error;
  final Set<int> actionInFlightTableIds;

  TableState({
    this.tables = const [],
    this.isLoading = false,
    this.error,
    this.actionInFlightTableIds = const <int>{},
  });

  TableState copyWith({
    List<TableModel>? tables,
    bool? isLoading,
    String? error,
    Set<int>? actionInFlightTableIds,
  }) {
    return TableState(
      tables: tables ?? this.tables,
      isLoading: isLoading ?? this.isLoading,
      error: error,
      actionInFlightTableIds: actionInFlightTableIds ?? this.actionInFlightTableIds,
    );
  }
}

class TableNotifier extends StateNotifier<TableState> {
  final ApiClient _apiClient;
  final Map<int, Future<bool>> _pendingTableActions = {};

  TableNotifier(this._apiClient) : super(TableState());

  Future<void> fetchTables() async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final response = await _apiClient.get('/tables');
      final List data = response.data['tables'] ?? [];
      final tables = data.map((json) => TableModel.fromJson(json)).toList();
      state = state.copyWith(tables: tables, isLoading: false);
    } catch (e) {
      state = state.copyWith(
          error: ErrorHandler.getErrorMessage(e), isLoading: false);
    }
  }

  Future<bool> updateTableStatus(int tableId, String status) async {
    return _runSingleFlightAction(tableId, () async {
      try {
        final response = await _apiClient.patch(
          '/tables/$tableId/status',
          data: {'status': status},
        );

        final updatedTable = TableModel.fromJson(response.data);
        final newTables = state.tables.map((t) {
          if (t.id == tableId) return updatedTable;
          return t;
        }).toList();

        state = state.copyWith(tables: newTables);
        return true;
      } catch (e) {
        state = state.copyWith(error: ErrorHandler.getErrorMessage(e));
        return false;
      }
    });
  }

  Future<bool> createTable({
    required String tableNo,
    required String tableType,
    required int capacity,
  }) async {
    try {
      final response = await _apiClient.post(
        '/tables',
        data: {
          'table_no': tableNo,
          'table_type': tableType,
          'capacity': capacity,
        },
      );

      final newTable = TableModel.fromJson(response.data);
      state = state.copyWith(tables: [...state.tables, newTable]);
      return true;
    } catch (e) {
      state = state.copyWith(error: ErrorHandler.getErrorMessage(e));
      return false;
    }
  }

  Future<bool> updateTable({
    required int tableId,
    required String tableNo,
    required String tableType,
    required int capacity,
  }) async {
    try {
      final response = await _apiClient.put(
        '/tables/$tableId',
        data: {
          'table_no': tableNo,
          'table_type': tableType,
          'capacity': capacity,
        },
      );

      final updatedTable = TableModel.fromJson(response.data);
      final newTables = state.tables.map((t) {
        if (t.id == tableId) return updatedTable;
        return t;
      }).toList();

      state = state.copyWith(tables: newTables);
      return true;
    } catch (e) {
      state = state.copyWith(error: ErrorHandler.getErrorMessage(e));
      return false;
    }
  }

  Future<bool> deleteTable(int tableId) async {
    try {
      await _apiClient.delete('/tables/$tableId');

      final newTables = state.tables.where((t) => t.id != tableId).toList();
      state = state.copyWith(tables: newTables);
      return true;
    } catch (e) {
      state = state.copyWith(error: ErrorHandler.getErrorMessage(e));
      return false;
    }
  }

  Future<bool> _runSingleFlightAction(
    int tableId,
    Future<bool> Function() action,
  ) {
    final existing = _pendingTableActions[tableId];
    if (existing != null) {
      return existing;
    }

    final nextInFlight = {...state.actionInFlightTableIds, tableId};
    state = state.copyWith(actionInFlightTableIds: nextInFlight, error: null);

    final future = () async {
      try {
        return await action();
      } finally {
        _pendingTableActions.remove(tableId);
        final remaining = {...state.actionInFlightTableIds}..remove(tableId);
        state = state.copyWith(actionInFlightTableIds: remaining);
      }
    }();

    _pendingTableActions[tableId] = future;
    return future;
  }

  void updateTableFromWebSocket(Map<String, dynamic> data) {
     final tableId = data['id'];
     final status = data['status'];
     if (tableId != null && status != null) {
       final newTables = state.tables.map((t) {
         if (t.id == tableId) {
           return t.copyWith(status: TableStatus.fromString(status as String?));
         }
         return t;
       }).toList();
       state = state.copyWith(tables: newTables);
     }
  }
}

final tableProvider = StateNotifierProvider<TableNotifier, TableState>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return TableNotifier(apiClient);
});
