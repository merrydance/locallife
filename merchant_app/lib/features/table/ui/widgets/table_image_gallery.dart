import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/table/models/table_image_model.dart';
import 'package:merchant_app/features/table/providers/table_image_provider.dart';
import 'package:merchant_app/widgets/merchant_image_picker.dart';

class TableImageGallery extends ConsumerWidget {
  final int tableId;
  final List<TableImageModel> images;

  const TableImageGallery({
    super.key,
    required this.tableId,
    required this.images,
  });

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    if (images.isEmpty) {
      return Container(
        height: 180,
        decoration: BoxDecoration(
          color: AppColors.surfaceLow,
          borderRadius: BorderRadius.circular(AppRadius.xl),
        ),
        child: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Icon(
                Icons.image_outlined,
                size: 40,
                color: AppColors.onSurfaceVariant,
              ),
              const SizedBox(height: AppSpacing.sm),
              const Text(
                '暂无图片',
                style: TextStyle(
                  color: AppColors.onSurfaceVariant,
                  fontSize: 14,
                ),
              ),
              const SizedBox(height: AppSpacing.md),
              MerchantImagePicker(
                businessType: 'table',
                mediaCategory: 'table_cover',
                onSuccess: (mediaAssetId, url) async {
                  await ref
                      .read(tableImageProvider.notifier)
                      .addImage(tableId, mediaAssetId);
                  if (context.mounted) {
                    ScaffoldMessenger.of(
                      context,
                    ).showSnackBar(const SnackBar(content: Text('图片添加成功')));
                  }
                },
                onError: (error) {
                  ScaffoldMessenger.of(
                    context,
                  ).showSnackBar(SnackBar(content: Text('上传失败: $error')));
                },
                child: OutlinedButton.icon(
                  onPressed: null, // Gesture handled by picker
                  icon: const Icon(Icons.add_photo_alternate_outlined),
                  label: const Text('添加图片'),
                ),
              ),
            ],
          ),
        ),
      );
    }

    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          mainAxisAlignment: MainAxisAlignment.spaceBetween,
          children: [
            const Text(
              '桌台图片',
              style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
            ),
            MerchantImagePicker(
              businessType: 'table',
              mediaCategory: 'table_cover',
              onSuccess: (mediaAssetId, url) async {
                await ref
                    .read(tableImageProvider.notifier)
                    .addImage(tableId, mediaAssetId);
                if (context.mounted) {
                  ScaffoldMessenger.of(
                    context,
                  ).showSnackBar(const SnackBar(content: Text('图片添加成功')));
                }
              },
              onError: (error) {
                ScaffoldMessenger.of(
                  context,
                ).showSnackBar(SnackBar(content: Text('上传失败: $error')));
              },
              child: TextButton.icon(
                onPressed: null, // Gesture handled by picker
                icon: const Icon(Icons.add, size: 18),
                label: const Text('添加'),
                style: TextButton.styleFrom(
                  visualDensity: VisualDensity.compact,
                ),
              ),
            ),
          ],
        ),
        const SizedBox(height: AppSpacing.xs),
        SizedBox(
          height: 200,
          child: ListView.separated(
            scrollDirection: Axis.horizontal,
            itemCount: images.length,
            separatorBuilder: (context, index) =>
                const SizedBox(width: AppSpacing.sm),
            itemBuilder: (context, index) {
              final image = images[index];
              return GestureDetector(
                onLongPress: () => _showImageOptions(context, ref, image),
                child: Stack(
                  children: [
                    ClipRRect(
                      borderRadius: BorderRadius.circular(AppRadius.lg),
                      child: Image.network(
                        image.imageUrl,
                        width: 280,
                        height: 200,
                        fit: BoxFit.cover,
                        errorBuilder: (context, error, stackTrace) => Container(
                          width: 280,
                          height: 200,
                          color: AppColors.surfaceLow,
                          child: const Icon(
                            Icons.broken_image,
                            color: AppColors.onSurfaceVariant,
                          ),
                        ),
                      ),
                    ),
                    if (image.isPrimary)
                      Positioned(
                        top: 8,
                        left: 8,
                        child: Container(
                          padding: const EdgeInsets.symmetric(
                            horizontal: 8,
                            vertical: 2,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.primary,
                            borderRadius: BorderRadius.circular(AppRadius.pill),
                          ),
                          child: const Text(
                            '主图',
                            style: TextStyle(
                              color: Colors.white,
                              fontSize: 10,
                              fontWeight: FontWeight.w600,
                            ),
                          ),
                        ),
                      ),
                  ],
                ),
              );
            },
          ),
        ),
        const SizedBox(height: AppSpacing.xs),
        const Text(
          '长按图片可设为主图或删除',
          style: TextStyle(fontSize: 12, color: AppColors.onSurfaceVariant),
        ),
      ],
    );
  }

  void _showImageOptions(
    BuildContext context,
    WidgetRef ref,
    TableImageModel image,
  ) {
    showModalBottomSheet(
      context: context,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(AppRadius.xl)),
      ),
      builder: (ctx) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const Padding(
              padding: EdgeInsets.symmetric(vertical: AppSpacing.lg),
              child: Text(
                '图片操作',
                style: TextStyle(fontSize: 16, fontWeight: FontWeight.w600),
              ),
            ),
            if (!image.isPrimary)
              ListTile(
                leading: const Icon(Icons.star_outline),
                title: const Text('设为主图'),
                onTap: () async {
                  Navigator.pop(ctx);
                  final success = await ref
                      .read(tableImageProvider.notifier)
                      .setPrimaryImage(tableId, image.id);
                  if (success && context.mounted) {
                    ScaffoldMessenger.of(
                      context,
                    ).showSnackBar(const SnackBar(content: Text('已设为主图')));
                  }
                },
              ),
            ListTile(
              leading: const Icon(
                Icons.delete_outline,
                color: AppColors.danger,
              ),
              title: const Text(
                '删除图片',
                style: TextStyle(color: AppColors.danger),
              ),
              onTap: () async {
                Navigator.pop(ctx);
                final success = await ref
                    .read(tableImageProvider.notifier)
                    .deleteImage(tableId, image.id);
                if (success && context.mounted) {
                  ScaffoldMessenger.of(
                    context,
                  ).showSnackBar(const SnackBar(content: Text('图片已删除')));
                }
              },
            ),
            const SizedBox(height: AppSpacing.md),
          ],
        ),
      ),
    );
  }
}
