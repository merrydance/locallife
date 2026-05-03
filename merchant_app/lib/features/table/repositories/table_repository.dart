import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/features/table/models/table_model.dart';
import 'package:merchant_app/features/table/models/table_image_model.dart';

/// 桌台数据仓库 — App → 后端 API 的唯一通道
///
/// 所有路由契约（HTTP 方法、路径、请求/响应结构）在此层统一管理，
/// Provider 层不直接持有 ApiClient，仅通过本类访问后端。
class TableRepository {
  final ApiClient _apiClient;

  TableRepository(this._apiClient);

  // ==================== 桌台 CRUD ====================

  /// 获取桌台详情 — GET /tables/:id
  Future<TableModel> getTable(int tableId) async {
    final response = await _apiClient.get('/tables/$tableId');
    return TableModel.fromJson(_extractData(response.data));
  }

  /// 获取桌台列表 — GET /tables?table_type=
  Future<List<TableModel>> listTables({String? tableType}) async {
    final queryParams = <String, dynamic>{};
    if (tableType != null) {
      queryParams['table_type'] = tableType;
    }
    final response = await _apiClient.get(
      '/tables',
      queryParameters: queryParams,
    );
    final dynamic actualData = _extractData(response.data);
    final List data = (actualData is Map && actualData.containsKey('tables'))
        ? actualData['tables']
        : (actualData is List ? actualData : []);
    return data.map((json) => TableModel.fromJson(json)).toList();
  }

  /// 创建桌台 — POST /tables
  Future<TableModel> createTable({
    required String tableNo,
    required String tableType,
    required int capacity,
    String? description,
    int? minimumSpend,
    String? accessCode,
    List<int>? tagIds,
  }) async {
    final body = <String, dynamic>{
      'table_no': tableNo,
      'table_type': tableType,
      'capacity': capacity,
    };
    if (description != null) body['description'] = description;
    if (minimumSpend != null) body['minimum_spend'] = minimumSpend;
    if (accessCode != null) body['access_code'] = accessCode;
    if (tagIds != null && tagIds.isNotEmpty) body['tag_ids'] = tagIds;

    final response = await _apiClient.post('/tables', data: body);
    return TableModel.fromJson(_extractData(response.data));
  }

  /// 更新桌台 — PATCH /tables/:id  ⚠️ 后端是 PATCH，不是 PUT
  Future<TableModel> updateTable(
    int tableId, {
    String? tableNo,
    String? tableType,
    int? capacity,
    String? description,
    int? minimumSpend,
    String? accessCode,
    String? status,
    List<int>? tagIds,
  }) async {
    final body = <String, dynamic>{};
    if (tableNo != null) body['table_no'] = tableNo;
    if (tableType != null) body['table_type'] = tableType;
    if (capacity != null) body['capacity'] = capacity;
    if (description != null) body['description'] = description;
    if (minimumSpend != null) body['minimum_spend'] = minimumSpend;
    if (accessCode != null) body['access_code'] = accessCode;
    if (status != null) body['status'] = status;
    if (tagIds != null) body['tag_ids'] = tagIds;

    final response = await _apiClient.patch('/tables/$tableId', data: body);
    return TableModel.fromJson(_extractData(response.data));
  }

  /// 删除桌台 — DELETE /tables/:id
  Future<void> deleteTable(int tableId) async {
    await _apiClient.delete('/tables/$tableId');
  }

  /// 更新桌台状态 — PATCH /tables/:id/status
  Future<TableModel> updateTableStatus(int tableId, String status) async {
    final response = await _apiClient.patch(
      '/tables/$tableId/status',
      data: {'status': status},
    );
    return TableModel.fromJson(_extractData(response.data));
  }

  // ==================== 标签管理 ====================

  /// 获取桌台标签列表 — GET /tables/:id/tags
  Future<List<TableTagInfo>> listTableTags(int tableId) async {
    final response = await _apiClient.get('/tables/$tableId/tags');
    final dynamic actualData = _extractData(response.data);
    final List data = (actualData is Map && actualData.containsKey('tags'))
        ? actualData['tags']
        : (actualData is List ? actualData : []);
    return data.map((json) => TableTagInfo.fromJson(json)).toList();
  }

  /// 添加桌台标签 — POST /tables/:id/tags
  Future<void> addTableTag(int tableId, int tagId) async {
    await _apiClient.post('/tables/$tableId/tags', data: {'tag_id': tagId});
  }

  /// 移除桌台标签 — DELETE /tables/:id/tags/:tag_id
  Future<void> removeTableTag(int tableId, int tagId) async {
    await _apiClient.delete('/tables/$tableId/tags/$tagId');
  }

  /// 获取可用的桌台标签（全局） — GET /tags?type=table
  Future<List<TableTagInfo>> listAvailableTableTags() async {
    final response = await _apiClient.get(
      '/tags',
      queryParameters: {'type': 'table'},
    );
    final dynamic actualData = _extractData(response.data);
    final List data = (actualData is Map && actualData.containsKey('tags'))
        ? actualData['tags']
        : (actualData is List ? actualData : []);
    return data.map((json) => TableTagInfo.fromJson(json)).toList();
  }

  // ==================== 图片管理 ====================

  /// 获取桌台图片列表 — GET /tables/:id/images
  Future<List<TableImageModel>> listTableImages(int tableId) async {
    final response = await _apiClient.get('/tables/$tableId/images');
    final dynamic actualData = _extractData(response.data);
    final List data = (actualData is Map && actualData.containsKey('images'))
        ? actualData['images']
        : (actualData is List ? actualData : []);
    return data.map((json) => TableImageModel.fromJson(json)).toList();
  }

  /// 添加桌台图片 — POST /tables/:id/images
  Future<TableImageModel> addTableImage(
    int tableId, {
    required int mediaAssetId,
    int sortOrder = 0,
    bool isPrimary = false,
  }) async {
    final response = await _apiClient.post(
      '/tables/$tableId/images',
      data: {
        'media_asset_id': mediaAssetId,
        'sort_order': sortOrder,
        'is_primary': isPrimary,
      },
    );
    return TableImageModel.fromJson(_extractData(response.data));
  }

  /// 设置桌台主图 — PUT /tables/:id/images/:image_id/primary
  Future<void> setTablePrimaryImage(int tableId, int imageId) async {
    await _apiClient.put('/tables/$tableId/images/$imageId/primary');
  }

  /// 删除桌台图片 — DELETE /tables/:id/images/:image_id
  Future<void> deleteTableImage(int tableId, int imageId) async {
    await _apiClient.delete('/tables/$tableId/images/$imageId');
  }

  // ==================== 二维码 ====================

  /// 生成桌台二维码 — GET /tables/:id/qrcode
  Future<TableQRCodeResponse> generateTableQRCode(int tableId) async {
    final response = await _apiClient.get('/tables/$tableId/qrcode');
    return TableQRCodeResponse.fromJson(_extractData(response.data));
  }

  /// 辅助方法：从统一的 API 响应信封中提取数据
  /// 后端统一格式为: {"code": 0, "message": "ok", "data": ...}
  dynamic _extractData(dynamic responseData) {
    if (responseData is Map<String, dynamic>) {
      if (responseData.containsKey('code') &&
          responseData.containsKey('data')) {
        return responseData['data'];
      }
    }
    return responseData;
  }
}
