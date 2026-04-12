import 'package:flutter/material.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:url_launcher/url_launcher.dart';

class UpdateDialog extends StatelessWidget {
  final String versionName;
  final String changelog;
  final String downloadUrl;
  final bool isForce;

  const UpdateDialog({
    super.key,
    required this.versionName,
    required this.changelog,
    required this.downloadUrl,
    required this.isForce,
  });

  Future<void> _launchUrl(BuildContext context) async {
    if (downloadUrl.trim().isEmpty) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('当前未提供下载地址')),
      );
      return;
    }
    final url = Uri.parse(downloadUrl);
    if (await canLaunchUrl(url)) {
      await launchUrl(url, mode: LaunchMode.externalApplication);
      return;
    }

    if (context.mounted) {
      ScaffoldMessenger.of(context).showSnackBar(
        const SnackBar(content: Text('无法打开下载地址')),
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: !isForce,
      child: AlertDialog(
        title: Text('发现新版本 V$versionName'),
        content: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(AppSpacing.lg),
              decoration: BoxDecoration(
                color: AppColors.surfaceLow,
                borderRadius: BorderRadius.circular(AppRadius.lg),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text(
                    '新版内容',
                    style: TextStyle(fontWeight: FontWeight.w700),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Text(
                    changelog,
                    style: const TextStyle(height: 1.5),
                  ),
                ],
              ),
            ),
          ],
        ),
        actions: [
          if (!isForce)
            TextButton(
              onPressed: () => Navigator.pop(context),
              child: const Text('稍后提醒', style: TextStyle(color: Colors.grey)),
            ),
          ElevatedButton(
            onPressed: () {
              _launchUrl(context);
              if (!isForce) {
                 Navigator.pop(context);
              }
            },
            child: const Text('前往下载'),
          ),
        ],
      ),
    );
  }
}

