/// 桌台图片模型 — 对齐后端 api.tableImageResponse
class TableImageModel {
  final int id;
  final int tableId;
  final int? mediaAssetId;
  final String imageUrl;
  final int sortOrder;
  final bool isPrimary;

  TableImageModel({
    required this.id,
    required this.tableId,
    this.mediaAssetId,
    required this.imageUrl,
    required this.sortOrder,
    required this.isPrimary,
  });

  factory TableImageModel.fromJson(Map<String, dynamic> json) {
    return TableImageModel(
      id: json['id'] as int,
      tableId: json['table_id'] as int,
      mediaAssetId: json['media_asset_id'] as int?,
      imageUrl: json['image_url'] as String? ?? '',
      sortOrder: json['sort_order'] as int? ?? 0,
      isPrimary: json['is_primary'] as bool? ?? false,
    );
  }
}

/// 桌台二维码响应 — 对齐后端 api.generateTableQRCodeResponse
class TableQRCodeResponse {
  final String qrCodeUrl;
  final String tableNo;
  final int merchantId;

  TableQRCodeResponse({
    required this.qrCodeUrl,
    required this.tableNo,
    required this.merchantId,
  });

  factory TableQRCodeResponse.fromJson(Map<String, dynamic> json) {
    return TableQRCodeResponse(
      qrCodeUrl: json['qr_code_url'] as String,
      tableNo: json['table_no'] as String,
      merchantId: json['merchant_id'] as int,
    );
  }
}
