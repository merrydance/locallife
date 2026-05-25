import 'dart:typed_data';
import 'package:crypto/crypto.dart';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:mime/mime.dart';
import 'package:merchant_app/core/network/api_client.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/models/media_asset_model.dart';

class MediaUploadService {
  final ApiClient _apiClient;

  MediaUploadService(this._apiClient);

  /// Helper to calculate SHA256 of bytes
  String _calculateSha256(Uint8List bytes) {
    return sha256.convert(bytes).toString();
  }

  /// 1. Create Upload Session
  Future<UploadSessionResponse> createUploadSession({
    required String businessType,
    required String mediaCategory,
    required String contentType,
    required int contentLength,
    required String checksumSha256,
  }) async {
    final request = CreateUploadSessionRequest(
      businessType: businessType,
      mediaCategory: mediaCategory,
      contentType: contentType,
      contentLength: contentLength,
      checksumSha256: checksumSha256,
    );

    final response = await _apiClient.post(
      '/media/upload-sessions',
      data: request.toJson(),
    );

    return UploadSessionResponse.fromJson(response.data);
  }

  /// 2. Transfer file direct to OSS (or dev proxy)
  Future<void> transferFile({
    required Uint8List fileBytes,
    required String filename,
    required UploadSessionResponse session,
  }) async {
    final dio = Dio();
    final formDataMap = Map<String, dynamic>.from(session.form);

    // Add the file as the last field as required by most OSS like AWS S3
    formDataMap['file'] = MultipartFile.fromBytes(
      fileBytes,
      filename: filename,
    );

    final formData = FormData.fromMap(formDataMap);

    // Some OSS endpoints might require specific headers, but typically FormData is enough.
    await dio.post(
      session.uploadHost,
      data: formData,
      options: Options(
        validateStatus: (status) =>
            status != null && status >= 200 && status < 300,
      ),
    );
  }

  /// 3. Complete Upload
  Future<MediaAssetResponse> completeUpload({
    required String uploadId,
    required String objectKey,
  }) async {
    final request = CompleteUploadRequest(
      uploadId: uploadId,
      objectKey: objectKey,
    );

    final response = await _apiClient.post(
      '/media/complete',
      data: request.toJson(),
    );

    return MediaAssetResponse.fromJson(response.data);
  }

  /// Unified 3-step upload method
  Future<MediaAssetResponse> uploadMedia({
    required Uint8List fileBytes,
    required String filename,
    required String businessType,
    required String mediaCategory,
  }) async {
    final contentType = lookupMimeType(filename) ?? 'application/octet-stream';
    final contentLength = fileBytes.length;
    final checksumSha256 = _calculateSha256(fileBytes);

    // Step 1: Request session
    final session = await createUploadSession(
      businessType: businessType,
      mediaCategory: mediaCategory,
      contentType: contentType,
      contentLength: contentLength,
      checksumSha256: checksumSha256,
    );

    // Step 2: Transfer to OSS
    await transferFile(
      fileBytes: fileBytes,
      filename: filename,
      session: session,
    );

    // Step 3: Complete upload
    final assetResponse = await completeUpload(
      uploadId: session.uploadId,
      objectKey: session.objectKey,
    );

    return assetResponse;
  }
}

final mediaUploadServiceProvider = Provider<MediaUploadService>((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return MediaUploadService(apiClient);
});
