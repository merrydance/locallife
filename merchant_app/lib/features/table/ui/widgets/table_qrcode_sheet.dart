import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/table/providers/table_detail_provider.dart';

/// 桌台二维码展示组件
class TableQRCodeSheet extends ConsumerStatefulWidget {
  final int tableId;
  final String tableNo;

  const TableQRCodeSheet({
    super.key,
    required this.tableId,
    required this.tableNo,
  });

  @override
  ConsumerState<TableQRCodeSheet> createState() => _TableQRCodeSheetState();
}

class _TableQRCodeSheetState extends ConsumerState<TableQRCodeSheet> {
  String? _qrCodeUrl;
  bool _isLoading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadQRCode();
  }

  Future<void> _loadQRCode() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    final result = await ref
        .read(tableDetailProvider.notifier)
        .generateQRCode(widget.tableId);

    if (!mounted) return;

    if (result != null) {
      setState(() {
        _qrCodeUrl = result.qrCodeUrl;
        _isLoading = false;
      });
    } else {
      setState(() {
        _error = '二维码生成失败';
        _isLoading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
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
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Text(
                  '桌台 ${widget.tableNo} · 二维码',
                  style: const TextStyle(
                    fontSize: 18,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                IconButton(
                  icon: const Icon(Icons.close),
                  onPressed: () => Navigator.pop(context),
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.xl),
            _buildQRCodeContent(),
            const SizedBox(height: AppSpacing.lg),
            const Text(
              '顾客扫码即可进入点餐页面',
              style: TextStyle(fontSize: 13, color: AppColors.onSurfaceVariant),
            ),
            const SizedBox(height: AppSpacing.lg),
          ],
        ),
      ),
    );
  }

  Widget _buildQRCodeContent() {
    if (_isLoading) {
      return const SizedBox(
        height: 240,
        child: Center(child: CircularProgressIndicator()),
      );
    }

    if (_error != null) {
      return SizedBox(
        height: 240,
        child: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Text(_error!, style: const TextStyle(color: AppColors.danger)),
              const SizedBox(height: AppSpacing.md),
              TextButton(onPressed: _loadQRCode, child: const Text('重试')),
            ],
          ),
        ),
      );
    }

    return Container(
      padding: const EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: Colors.white,
        border: Border.all(color: AppColors.outlineVariant),
        borderRadius: BorderRadius.circular(AppRadius.xl),
      ),
      child: Image.network(
        _qrCodeUrl!,
        width: 240,
        height: 240,
        fit: BoxFit.contain,
        loadingBuilder: (_, child, progress) {
          if (progress == null) return child;
          return const SizedBox(
            width: 240,
            height: 240,
            child: Center(child: CircularProgressIndicator()),
          );
        },
        errorBuilder: (context, error, stackTrace) => const SizedBox(
          width: 240,
          height: 240,
          child: Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(
                  Icons.broken_image,
                  size: 40,
                  color: AppColors.onSurfaceVariant,
                ),
                SizedBox(height: AppSpacing.sm),
                Text(
                  '图片加载失败',
                  style: TextStyle(color: AppColors.onSurfaceVariant),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
