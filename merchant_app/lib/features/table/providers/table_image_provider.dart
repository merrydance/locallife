import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/utils/error_handler.dart';
import 'package:merchant_app/features/table/providers/table_detail_provider.dart';
import 'package:merchant_app/features/table/providers/table_provider.dart';
import 'package:merchant_app/features/table/repositories/table_repository.dart';

class TableImageNotifier extends StateNotifier<AsyncValue<void>> {
  final TableRepository _repository;
  final Ref _ref;

  TableImageNotifier(this._repository, this._ref)
    : super(const AsyncData(null));

  Future<bool> addImage(int tableId, int mediaAssetId) async {
    state = const AsyncLoading();
    try {
      await _repository.addTableImage(tableId, mediaAssetId: mediaAssetId);
      state = const AsyncData(null);
      // Reload detail to refresh the image list
      _ref.read(tableDetailProvider.notifier).loadDetail(tableId);
      return true;
    } catch (e) {
      state = AsyncError(ErrorHandler.getErrorMessage(e), StackTrace.current);
      return false;
    }
  }

  Future<bool> setPrimaryImage(int tableId, int imageId) async {
    state = const AsyncLoading();
    try {
      await _repository.setTablePrimaryImage(tableId, imageId);
      state = const AsyncData(null);
      _ref.read(tableDetailProvider.notifier).loadDetail(tableId);
      _ref
          .read(tableProvider.notifier)
          .fetchTables(); // Refresh list to update grid primary image
      return true;
    } catch (e) {
      state = AsyncError(ErrorHandler.getErrorMessage(e), StackTrace.current);
      return false;
    }
  }

  Future<bool> deleteImage(int tableId, int imageId) async {
    state = const AsyncLoading();
    try {
      await _repository.deleteTableImage(tableId, imageId);
      state = const AsyncData(null);
      _ref.read(tableDetailProvider.notifier).loadDetail(tableId);
      return true;
    } catch (e) {
      state = AsyncError(ErrorHandler.getErrorMessage(e), StackTrace.current);
      return false;
    }
  }
}

final tableImageProvider =
    StateNotifierProvider<TableImageNotifier, AsyncValue<void>>((ref) {
      final repository = ref.watch(tableRepositoryProvider);
      return TableImageNotifier(repository, ref);
    });
