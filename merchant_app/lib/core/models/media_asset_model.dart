class CreateUploadSessionRequest {
  final String businessType;
  final String mediaCategory;
  final String contentType;
  final int contentLength;
  final String checksumSha256;

  CreateUploadSessionRequest({
    required this.businessType,
    required this.mediaCategory,
    required this.contentType,
    required this.contentLength,
    required this.checksumSha256,
  });

  Map<String, dynamic> toJson() {
    return {
      'business_type': businessType,
      'media_category': mediaCategory,
      'content_type': contentType,
      'content_length': contentLength,
      'checksum_sha256': checksumSha256,
    };
  }
}

class UploadSessionResponse {
  final String uploadId;
  final String objectKey;
  final String visibility;
  final String uploadHost;
  final Map<String, String> form;
  final DateTime expireAt;

  UploadSessionResponse({
    required this.uploadId,
    required this.objectKey,
    required this.visibility,
    required this.uploadHost,
    required this.form,
    required this.expireAt,
  });

  factory UploadSessionResponse.fromJson(Map<String, dynamic> json) {
    return UploadSessionResponse(
      uploadId: json['upload_id'] as String,
      objectKey: json['object_key'] as String,
      visibility: json['visibility'] as String,
      uploadHost: json['upload_host'] as String,
      form: Map<String, String>.from(json['form'] as Map),
      expireAt: DateTime.parse(json['expire_at'] as String),
    );
  }
}

class CompleteUploadRequest {
  final String uploadId;
  final String objectKey;
  final String? etag;

  CompleteUploadRequest({
    required this.uploadId,
    required this.objectKey,
    this.etag,
  });

  Map<String, dynamic> toJson() {
    return {
      'upload_id': uploadId,
      'object_key': objectKey,
      if (etag != null) 'etag': etag,
    };
  }
}

class MediaAssetResponse {
  final int mediaId;
  final Map<String, String>? urls;
  final String status;

  MediaAssetResponse({required this.mediaId, this.urls, required this.status});

  factory MediaAssetResponse.fromJson(Map<String, dynamic> json) {
    return MediaAssetResponse(
      mediaId: json['media_id'] as int,
      urls: json['urls'] != null
          ? Map<String, String>.from(json['urls'] as Map)
          : null,
      status: json['status'] as String,
    );
  }
}
