import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/utils/error_handler.dart';
import 'package:merchant_app/features/table/models/table_model.dart';
import 'package:merchant_app/features/table/providers/table_detail_provider.dart';
import 'package:merchant_app/features/table/providers/table_provider.dart';
import 'package:merchant_app/features/table/repositories/table_repository.dart';

class TableTagState {
  final List<TableTagInfo> availableTags;
  final bool isLoading;
  final String? error;

  TableTagState({
    this.availableTags = const [],
    this.isLoading = false,
    this.error,
  });

  TableTagState copyWith({
    List<TableTagInfo>? availableTags,
    bool? isLoading,
    String? error,
  }) {
    return TableTagState(
      availableTags: availableTags ?? this.availableTags,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class TableTagNotifier extends StateNotifier<TableTagState> {
  final TableRepository _repository;
  final Ref _ref;

  TableTagNotifier(this._repository, this._ref) : super(TableTagState());

  Future<void> fetchAvailableTags() async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      final tags = await _repository.listAvailableTableTags();
      state = state.copyWith(availableTags: tags, isLoading: false);
    } catch (e) {
      state = state.copyWith(
        error: ErrorHandler.getErrorMessage(e),
        isLoading: false,
      );
    }
  }

  Future<bool> addTagToTable(int tableId, int tagId) async {
    try {
      await _repository.addTableTag(tableId, tagId);
      _ref.read(tableDetailProvider.notifier).loadDetail(tableId);
      return true;
    } catch (e) {
      return false;
    }
  }

  Future<bool> removeTagFromTable(int tableId, int tagId) async {
    try {
      await _repository.removeTableTag(tableId, tagId);
      _ref.read(tableDetailProvider.notifier).loadDetail(tableId);
      return true;
    } catch (e) {
      return false;
    }
  }
}

final tableTagProvider = StateNotifierProvider<TableTagNotifier, TableTagState>(
  (ref) {
    final repository = ref.watch(tableRepositoryProvider);
    return TableTagNotifier(repository, ref);
  },
);
