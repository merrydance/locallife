import 'package:flutter/foundation.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/utils/error_handler.dart';
import 'package:merchant_app/features/table/models/table_model.dart';
import 'package:merchant_app/features/table/repositories/table_repository.dart';

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
      actionInFlightTableIds:
          actionInFlightTableIds ?? this.actionInFlightTableIds,
    );
  }
}

class TableNotifier extends StateNotifier<TableState> {
  final TableRepository _repository;
  final Map<int, Future<bool>> _pendingTableActions = {};

  TableNotifier(this._repository) : super(TableState());

  Future<void> fetchTables() async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final tables = await _repository.listTables();
      state = state.copyWith(tables: tables, isLoading: false);
    } catch (e, stackTrace) {
      debugPrint('fetchTables error: $e\n$stackTrace');
      state = state.copyWith(
        error: ErrorHandler.getErrorMessage(e),
        isLoading: false,
      );
    }
  }

  Future<bool> updateTableStatus(int tableId, String status) async {
    return _runSingleFlightAction(tableId, () async {
      try {
        final updatedTable = await _repository.updateTableStatus(
          tableId,
          status,
        );
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

  /// 创建桌台 — 支持后端全部字段
  Future<bool> createTable({
    required String tableNo,
    required String tableType,
    required int capacity,
    String? description,
    int? minimumSpend,
    String? accessCode,
    List<int>? tagIds,
  }) async {
    final id = await createTableAndReturnId(
      tableNo: tableNo,
      tableType: tableType,
      capacity: capacity,
      description: description,
      minimumSpend: minimumSpend,
      accessCode: accessCode,
      tagIds: tagIds,
    );
    return id != null;
  }

  /// 创建桌台并返回新建桌台的 ID（用于后续关联图片等操作）
  Future<int?> createTableAndReturnId({
    required String tableNo,
    required String tableType,
    required int capacity,
    String? description,
    int? minimumSpend,
    String? accessCode,
    List<int>? tagIds,
  }) async {
    try {
      final newTable = await _repository.createTable(
        tableNo: tableNo,
        tableType: tableType,
        capacity: capacity,
        description: description,
        minimumSpend: minimumSpend,
        accessCode: accessCode,
        tagIds: tagIds,
      );

      state = state.copyWith(tables: [...state.tables, newTable]);
      return newTable.id;
    } catch (e) {
      state = state.copyWith(error: ErrorHandler.getErrorMessage(e));
      return null;
    }
  }

  /// 更新桌台 — 使用 PATCH，支持后端全部字段
  Future<bool> updateTable({
    required int tableId,
    String? tableNo,
    String? tableType,
    int? capacity,
    String? description,
    int? minimumSpend,
    String? accessCode,
    List<int>? tagIds,
  }) async {
    try {
      final updatedTable = await _repository.updateTable(
        tableId,
        tableNo: tableNo,
        tableType: tableType,
        capacity: capacity,
        description: description,
        minimumSpend: minimumSpend,
        accessCode: accessCode,
        tagIds: tagIds,
      );

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
      await _repository.deleteTable(tableId);

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

/// Repository provider — 桌台数据层的依赖注入
final tableRepositoryProvider = Provider<TableRepository>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return TableRepository(apiClient);
});

/// 桌台列表状态 provider
final tableProvider = StateNotifierProvider<TableNotifier, TableState>((ref) {
  final repository = ref.watch(tableRepositoryProvider);
  return TableNotifier(repository);
});
