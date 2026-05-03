import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/core/network/api_provider.dart';
import 'package:merchant_app/widgets/merchant_content_shell.dart';

class AgreementDetail {
  const AgreementDetail({
    required this.type,
    required this.title,
    required this.version,
    required this.publishedOn,
    required this.content,
  });

  final String type;
  final String title;
  final String version;
  final String publishedOn;
  final String content;

  factory AgreementDetail.fromJson(Map<String, dynamic> json) {
    return AgreementDetail(
      type: json['type']?.toString() ?? '',
      title: json['title']?.toString() ?? '',
      version: json['version']?.toString() ?? '',
      publishedOn: json['published_on']?.toString() ?? '',
      content: json['content']?.toString() ?? '',
    );
  }
}

final agreementProvider = FutureProvider.family<AgreementDetail, String>((
  ref,
  type,
) async {
  final apiClient = ref.watch(apiClientProvider);
  final response = await apiClient.get('/agreements/$type');

  final responseData = response.data;
  final dynamic data = (responseData is Map && responseData.containsKey('data'))
      ? responseData['data']
      : responseData;

  return AgreementDetail.fromJson(Map<String, dynamic>.from(data));
});

class AgreementPage extends ConsumerWidget {
  const AgreementPage({required this.agreementType, super.key});

  final String agreementType;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final agreementAsync = ref.watch(agreementProvider(agreementType));

    return Scaffold(
      appBar: AppBar(title: const Text('协议详情')),
      body: agreementAsync.when(
        data: (agreement) => SingleChildScrollView(
          child: MerchantContentShell(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(AppSpacing.xl),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          agreement.title,
                          style: const TextStyle(
                            fontSize: 22,
                            fontWeight: FontWeight.w700,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        Text(
                          '版本 ${agreement.version} · 发布于 ${agreement.publishedOn}',
                          style: const TextStyle(
                            color: AppColors.onSurfaceVariant,
                            height: 1.45,
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
                const SizedBox(height: AppSpacing.lg),
                Card(
                  child: Padding(
                    padding: const EdgeInsets.all(AppSpacing.xl),
                    child: SelectableText(
                      _htmlToReadableText(agreement.content),
                      style: const TextStyle(
                        fontSize: 15,
                        height: 1.7,
                        color: AppColors.onSurface,
                      ),
                    ),
                  ),
                ),
                const SizedBox(height: AppSpacing.xl),
              ],
            ),
          ),
        ),
        loading: () => const Center(child: CircularProgressIndicator()),
        error: (error, stackTrace) => Center(
          child: Padding(
            padding: const EdgeInsets.all(AppSpacing.xl),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                const Icon(
                  Icons.description_outlined,
                  size: 48,
                  color: AppColors.onSurfaceVariant,
                ),
                const SizedBox(height: AppSpacing.lg),
                const Text(
                  '协议内容加载失败',
                  style: TextStyle(fontSize: 18, fontWeight: FontWeight.w700),
                ),
                const SizedBox(height: AppSpacing.sm),
                Text(
                  error.toString(),
                  textAlign: TextAlign.center,
                  style: const TextStyle(
                    color: AppColors.onSurfaceVariant,
                    height: 1.45,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  static String _htmlToReadableText(String html) {
    var text = html
        .replaceAll(RegExp(r'<style[\s\S]*?</style>', caseSensitive: false), '')
        .replaceAll(
          RegExp(r'<script[\s\S]*?</script>', caseSensitive: false),
          '',
        )
        .replaceAll(RegExp(r'<br\s*/?>', caseSensitive: false), '\n')
        .replaceAll(
          RegExp(r'</p>|</div>|</h1>|</h2>|</h3>|</li>', caseSensitive: false),
          '\n',
        )
        .replaceAll(RegExp(r'<li[^>]*>', caseSensitive: false), '• ')
        .replaceAll(RegExp(r'<[^>]+>'), '')
        .replaceAll('&nbsp;', ' ')
        .replaceAll('&amp;', '&')
        .replaceAll('&lt;', '<')
        .replaceAll('&gt;', '>')
        .replaceAll('&#39;', "'")
        .replaceAll('&quot;', '"');

    text = text.replaceAll(RegExp(r'\n\s*\n\s*\n+'), '\n\n');
    return text.trim();
  }
}
