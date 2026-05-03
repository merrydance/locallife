import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/core/service/media_upload_service.dart';
import 'package:merchant_app/core/utils/error_handler.dart';

typedef OnUploadSuccess = void Function(int mediaAssetId, String url);
typedef OnUploadError = void Function(String error);

class MerchantImagePicker extends ConsumerStatefulWidget {
  final String businessType;
  final String mediaCategory;
  final Widget child;
  final OnUploadSuccess onSuccess;
  final OnUploadError onError;

  const MerchantImagePicker({
    super.key,
    required this.businessType,
    required this.mediaCategory,
    required this.child,
    required this.onSuccess,
    required this.onError,
  });

  @override
  ConsumerState<MerchantImagePicker> createState() =>
      _MerchantImagePickerState();
}

class _MerchantImagePickerState extends ConsumerState<MerchantImagePicker> {
  final ImagePicker _picker = ImagePicker();
  bool _isUploading = false;

  Future<void> _pickAndUploadImage(ImageSource source) async {
    try {
      final XFile? image = await _picker.pickImage(source: source);
      if (image == null) return;

      setState(() => _isUploading = true);

      final bytes = await image.readAsBytes();
      final filename = image.name;

      final uploadService = ref.read(mediaUploadServiceProvider);
      final response = await uploadService.uploadMedia(
        fileBytes: bytes,
        filename: filename,
        businessType: widget.businessType,
        mediaCategory: widget.mediaCategory,
      );

      // Extract one of the URLs (or fallback to empty if unavailable)
      final url = response.urls?.values.firstOrNull ?? '';

      widget.onSuccess(response.mediaId, url);
    } catch (e) {
      widget.onError(ErrorHandler.getErrorMessage(e));
    } finally {
      if (mounted) {
        setState(() => _isUploading = false);
      }
    }
  }

  void _showPickerOptions() {
    showModalBottomSheet(
      context: context,
      backgroundColor: Colors.transparent,
      builder: (context) => Container(
        padding: const EdgeInsets.all(AppSpacing.xl),
        decoration: const BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.vertical(
            top: Radius.circular(AppRadius.xxl),
          ),
        ),
        child: SafeArea(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              ListTile(
                leading: const Icon(
                  Icons.photo_library,
                  color: AppColors.primary,
                ),
                title: const Text('从相册选择'),
                onTap: () {
                  Navigator.pop(context);
                  _pickAndUploadImage(ImageSource.gallery);
                },
              ),
              ListTile(
                leading: const Icon(Icons.camera_alt, color: AppColors.primary),
                title: const Text('拍照'),
                onTap: () {
                  Navigator.pop(context);
                  _pickAndUploadImage(ImageSource.camera);
                },
              ),
            ],
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: _isUploading ? null : _showPickerOptions,
      child: Stack(
        alignment: Alignment.center,
        children: [
          Opacity(opacity: _isUploading ? 0.5 : 1.0, child: widget.child),
          if (_isUploading)
            const CircularProgressIndicator(
              valueColor: AlwaysStoppedAnimation<Color>(AppColors.primary),
            ),
        ],
      ),
    );
  }
}
