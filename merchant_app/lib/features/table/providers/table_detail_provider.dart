import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/core/utils/error_handler.dart';
import 'package:merchant_app/features/table/models/table_model.dart';
import 'package:merchant_app/features/table/models/table_image_model.dart';
import 'package:merchant_app/features/table/providers/table_provider.dart';
import 'package:merchant_app/features/table/repositories/table_repository.dart';

/// 桌台详情页状态
class TableDetailState {
  final TableModel? table;
  final List<TableTagInfo> tags;
  final List<TableImageModel> images;
  final bool isLoading;
  final String? error;

  TableDetailState({
    this.table,
    this.tags = const [],
    this.images = const [],
    this.isLoading = false,
    this.error,
  });

  TableDetailState copyWith({
    TableModel? table,
    List<TableTagInfo>? tags,
    List<TableImageModel>? images,
    bool? isLoading,
    String? error,
  }) {
    return TableDetailState(
      table: table ?? this.table,
      tags: tags ?? this.tags,
      images: images ?? this.images,
      isLoading: isLoading ?? this.isLoading,
      error: error,
    );
  }
}

class TableDetailNotifier extends StateNotifier<TableDetailState> {
  final TableRepository _repository;

  TableDetailNotifier(this._repository) : super(TableDetailState());

  /// 加载桌台完整详情（基础信息 + 标签 + 图片）
  Future<void> loadDetail(int tableId) async {
    state = state.copyWith(isLoading: true, error: null);
    try {
      // 并行加载详情、标签、图片
      final results = await Future.wait([
        _repository.getTable(tableId),
        _repository.listTableTags(tableId),
        _repository.listTableImages(tableId),
      ]);

      state = state.copyWith(
        table: results[0] as TableModel,
        tags: results[1] as List<TableTagInfo>,
        images: results[2] as List<TableImageModel>,
        isLoading: false,
      );
    } catch (e) {
      state = state.copyWith(
        error: ErrorHandler.getErrorMessage(e),
        isLoading: false,
      );
    }
  }

  /// 生成二维码
  Future<TableQRCodeResponse?> generateQRCode(int tableId) async {
    try {
      final qrCode = await _repository.generateTableQRCode(tableId);
      // 更新本地桌台的 qrCodeUrl
      if (state.table != null) {
        state = state.copyWith(
          table: state.table!.copyWith(qrCodeUrl: qrCode.qrCodeUrl),
        );
      }
      return qrCode;
    } catch (e) {
      state = state.copyWith(error: ErrorHandler.getErrorMessage(e));
      return null;
    }
  }
}

/// 桌台详情 provider（autoDispose，页面关闭自动释放）
final tableDetailProvider =
    StateNotifierProvider.autoDispose<TableDetailNotifier, TableDetailState>((
      ref,
    ) {
      final repository = ref.watch(tableRepositoryProvider);
      return TableDetailNotifier(repository);
    });
