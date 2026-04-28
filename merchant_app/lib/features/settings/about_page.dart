import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';
import 'package:package_info_plus/package_info_plus.dart';
import 'package:merchant_app/config/theme.dart';
import 'package:merchant_app/features/update/update_service.dart';
import 'package:merchant_app/widgets/merchant_content_shell.dart';

final packageInfoProvider = FutureProvider<PackageInfo>((ref) async {
  return await PackageInfo.fromPlatform();
});

class AboutPage extends ConsumerWidget {
  const AboutPage({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final packageInfoAsync = ref.watch(packageInfoProvider);

    return Scaffold(
      appBar: AppBar(
        title: const Text('关于乐客来福商户'),
      ),
      body: SingleChildScrollView(
        child: MerchantContentShell(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const SizedBox(height: AppSpacing.xl),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(AppSpacing.xxl),
                  child: Column(
                    children: [
                      Container(
                        width: 96,
                        height: 96,
                        decoration: BoxDecoration(
                          color: AppColors.surfaceLow,
                          borderRadius: BorderRadius.circular(AppRadius.xl),
                        ),
                        child: const Icon(
                          Icons.storefront_rounded,
                          size: 56,
                          color: AppColors.primary,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.lg),
                      const Text(
                        '乐客来福商户',
                        style: TextStyle(fontSize: 24, fontWeight: FontWeight.w700),
                      ),
                      const SizedBox(height: AppSpacing.xs),
                      packageInfoAsync.when(
                        data: (info) => Text(
                          '版本 ${info.version}',
                          style: const TextStyle(
                            fontSize: 15,
                            color: AppColors.onSurfaceVariant,
                          ),
                        ),
                        loading: () => const SizedBox(
                          height: 20,
                          width: 20,
                          child: CircularProgressIndicator(strokeWidth: 2),
                        ),
                        error: (error, stackTrace) => const Text('获取版本失败'),
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.lg),
              Card(
                child: Padding(
                  padding: const EdgeInsets.all(AppSpacing.xl),
                  child: Column(
                    children: [
                      _AboutActionTile(
                        icon: Icons.system_update_alt,
                        title: '检查更新',
                        subtitle: '检查当前版本是否有可用新版本。',
                        onTap: () {
                          ref
                              .read(updateServiceProvider)
                              .checkForUpdates(context, showToastIfLatest: true);
                        },
                      ),
                      const SizedBox(height: AppSpacing.md),
                      const _AboutActionTile(
                        icon: Icons.privacy_tip_outlined,
                        title: '隐私政策',
                        subtitle: '查看当前生效的隐私政策与个人信息处理规则。',
                        route: '/settings/agreement/PRIVACY_POLICY',
                      ),
                      const SizedBox(height: AppSpacing.md),
                      const _AboutActionTile(
                        icon: Icons.description_outlined,
                        title: '用户协议',
                        subtitle: '查看当前生效的用户协议。',
                        route: '/settings/agreement/USER_AGREEMENT',
                      ),
                    ],
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.xl),
              const Center(
                child: Text(
                  '© 2026 乐客来福科技 版权所有',
                  style: TextStyle(
                    color: AppColors.onSurfaceVariant,
                    fontSize: 12,
                  ),
                ),
              ),
              const SizedBox(height: AppSpacing.xl),
            ],
          ),
        ),
      ),
    );
  }
}

class _AboutActionTile extends StatelessWidget {
  const _AboutActionTile({
    required this.icon,
    required this.title,
    required this.subtitle,
    this.onTap,
    this.route,
  });

  final IconData icon;
  final String title;
  final String subtitle;
  final VoidCallback? onTap;
  final String? route;

  @override
  Widget build(BuildContext context) {
    final effectiveTap = onTap ?? (route == null ? null : () => context.push(route!));
    final content = Container(
      padding: const EdgeInsets.all(AppSpacing.lg),
      decoration: BoxDecoration(
        color: AppColors.surfaceLow,
        borderRadius: BorderRadius.circular(AppRadius.lg),
      ),
      child: Row(
        children: [
          Icon(icon, color: AppColors.primary),
          const SizedBox(width: AppSpacing.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  style: const TextStyle(
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: AppSpacing.xs),
                Text(
                  subtitle,
                  style: const TextStyle(
                    color: AppColors.onSurfaceVariant,
                    height: 1.45,
                  ),
                ),
              ],
            ),
          ),
          const SizedBox(width: AppSpacing.md),
          Icon(
              effectiveTap == null ? Icons.schedule_outlined : Icons.chevron_right,
            color: AppColors.onSurfaceVariant,
          ),
        ],
      ),
    );

    if (effectiveTap == null) {
      return content;
    }

    return InkWell(
      onTap: effectiveTap,
      borderRadius: BorderRadius.circular(AppRadius.lg),
      child: content,
    );
  }
}
