import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/core/utils/error_handler.dart';
import 'update_dialog.dart';

final updateServiceProvider = Provider((ref) {
  final apiClient = ref.watch(apiClientProvider);
  return UpdateService(apiClient);
});

class UpdateService {
  UpdateService(this._apiClient);

  final dynamic _apiClient;

  Future<void> checkForUpdates(BuildContext context, {bool showToastIfLatest = false}) async {
    try {
      final packageInfo = await PackageInfo.fromPlatform();
      final currentCode = int.tryParse(packageInfo.buildNumber) ?? 1;

      final response = await _apiClient.get(
        '/app/version/latest',
        queryParameters: {
          'platform': 'android',
          'channel': 'merchant_app',
          'package_name': 'com.merrydance.locallife.merchant',
          'version_code': currentCode,
          'version_name': packageInfo.version,
        },
      );

      final responseData = response.data;
      final data = responseData is Map<String, dynamic>
          ? (responseData['data'] as Map<String, dynamic>?) ?? responseData
          : <String, dynamic>{};

      final serverCode = (data['version_code'] as num?)?.toInt() ?? currentCode;

      if (serverCode > currentCode) {
        if (!context.mounted) return;

        showDialog(
          context: context,
          barrierDismissible: !(data['is_force'] as bool? ?? false),
          builder: (context) => UpdateDialog(
            versionName: data['version_name']?.toString() ?? packageInfo.version,
            changelog: data['changelog']?.toString() ?? '本次版本包含若干稳定性修复。',
            downloadUrl: data['download_url']?.toString() ?? '',
            isForce: data['is_force'] as bool? ?? false,
          ),
        );
      } else {
        if (showToastIfLatest && context.mounted) {
          ScaffoldMessenger.of(context).showSnackBar(
            const SnackBar(content: Text('已经是最新版本')),
          );
        }
      }
    } catch (e) {
      if (!context.mounted) {
        return;
      }

      final message = _isUpdateApiMissing(e)
          ? '当前版本暂未接入在线升级，请等待后端接口上线'
          : ErrorHandler.getErrorMessage(e);

      if (showToastIfLatest) {
        ScaffoldMessenger.of(context).showSnackBar(
          SnackBar(content: Text(message)),
        );
      }
    }
  }

  bool _isUpdateApiMissing(Object error) {
    final message = error.toString();
    return message.contains('404');
  }
}
